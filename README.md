# Atlas Game Server Platform

Atlas is a game server hosting panel designed for performance, modularity, and security.

## 🚀 Key Features
*   **Infrastructure as Code**: Manage game templates ("Eggs") as JSON files.
*   **Distributed Architecture**: Deploy multiple game nodes globally from a single panel.
*   **Hardened Security**: Internal services (API/Database) are hidden from the public by default.

## 📦 Production Deployment (Ubuntu)

Atlas runs entirely within Docker Compose. For the best experience, we recommend using an Nginx Reverse Proxy on the host.

### 1. Prerequisites
*   Ubuntu 20.04/22.04/24.04 LTS
*   Docker & Docker Compose installed.
*   Nginx installed on the host.

### 2. Initial Configuration
Create your environment file:
```bash
cp .env.example .env
nano .env
```
*   **Database**: Update `DB_USER` and `DB_PASS`.
*   **Ports**: Customize `ATLAS_PANEL_PORT` (default 4000) if needed.
*   **Storage**: Customize `DATA_PATH` (default `/var/lib/atlas/data`) to choose where game files are stored on your host. Use an absolute host path.
*   **File ownership**: Keep `SERVER_UID=1000` and `SERVER_GID=1000` unless your game images use another Linux identity. Game containers run with this identity; transient installers run as root and Atlas normalizes their output afterward.
*   **Node Token**: Leave this blank or as default for the very first boot.

### 3. First Boot (Registration)
Start the cluster:
```bash
docker-compose up -d --build
```
Log in to your panel (e.g., `http://your-ip:4000` or your proxy domain).

### 4. Setting up your Local Node
By default, the "Daemon" service needs a specific authentication token to speak to the Core.
1.  Navigate to **Admin > Nodes**.
2.  Click **Create New Node**.
3.  Fill in the details (Address should be your server's public IP).
4.  Once created, click the **Settings icon** on the new node to view the **Node Token**.
5.  Edit your `.env` file and paste this token into `NODE_TOKEN`.
6.  Restart the daemon:
    ```bash
    docker-compose up -d daemon
    ```

## Updating an Existing Installation

These steps preserve the PostgreSQL volume and running game-server data. Run them
from the Atlas directory on the server after uploading the changed source files.

1. Back up the database:

   ```bash
   docker compose exec -T database sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' > "atlas-$(date +%Y%m%d-%H%M%S).sql"
   ```

2. Set the storage identity in `.env`. The defaults match the supported game
   images:

   ```dotenv
   DATA_PATH=/var/lib/atlas/data
   SERVER_UID=1000
   SERVER_GID=1000
   ```

   `DATA_PATH` must be an absolute path on the Docker host. Atlas exposes it to
   the daemon as `/var/lib/atlas/data` while passing the original host path to
   Docker when installer and game containers are created.

3. Repair existing game files once. Set these three shell variables to the same
   values used in `.env`; this deliberately leaves the daemon's SFTP host key
   at the data-root level untouched:

   ```bash
   ATLAS_DATA_PATH=/var/lib/atlas/data
   ATLAS_UID=1000
   ATLAS_GID=1000

   sudo install -d -m 0755 "$ATLAS_DATA_PATH"
   while IFS= read -r -d '' service_dir; do
     sudo chown -R "$ATLAS_UID:$ATLAS_GID" "$service_dir"
     sudo chmod -R u+rwX,g+rwX "$service_dir"
     sudo find "$service_dir" -type d -exec chmod g+s {} +
   done < <(find "$ATLAS_DATA_PATH" -mindepth 1 -maxdepth 1 -type d -name '????????-????-????-????-????????????' -print0)
   ```

   Do not use `chmod -R 777`. Atlas also performs a one-time recursive repair
   for legacy server directories when the rebuilt daemon starts, and then
   normalizes panel writes, installer output, and completed SFTP sessions.

4. Remove catalog directories that were deleted by this release. A normal file
   upload does not remove old server-side files:

   ```bash
   rm -rf eggs/games/hytale eggs/games/palworld
   ```

5. Validate the Compose configuration and build fresh images:

   ```bash
   docker compose config --quiet
   docker compose build --pull core daemon panel
   ```

6. Recreate Core first so its migration and egg seeder finish before the daemon
   reconnects:

   ```bash
   docker compose up -d --no-deps --force-recreate core
   until docker compose exec -T core wget -qO- http://127.0.0.1:8080/api/v1/auth/setup-status >/dev/null 2>&1; do sleep 2; done
   docker compose up -d --no-deps --force-recreate daemon panel
   ```

7. Verify the deployment and storage ownership:

   ```bash
   docker compose ps
   docker compose logs --since=5m core daemon panel
   sudo find "$ATLAS_DATA_PATH" -mindepth 1 -maxdepth 2 -printf '%u:%g %m %p\n' | head -40
   ```

   Service directories and files should show the configured UID/GID. To test a
   previously failed SteamCMD installation, use **Reinstall** in the panel and
   watch `docker compose logs -f daemon`; the installer runs transiently as
   root, then normalizes its output to that UID/GID before the game starts.

The node should return online after its next heartbeat, normally within five
seconds. Open the panel and perform one hard refresh (`Ctrl+Shift+R` or
`Cmd+Shift+R`) so the browser loads the newly hashed frontend assets.

Do not run `docker compose down -v`; the `-v` option deletes the Atlas
database volume. Rebuilding or recreating the daemon does not remove game
containers or `${DATA_PATH}`.

For a remote node, upload the daemon changes on that host and run:

```bash
docker compose build --pull daemon
docker compose up -d --no-deps --force-recreate daemon
docker compose logs --since=5m daemon
```

### Recovering a Failed GMod x86-64 Installation

GMod's x86-64 branch requires an AMD64 host. Check the host before retrying:

```bash
uname -m
docker info --format '{{.Architecture}}'
```

The expected values are `x86_64` and `x86_64` or `amd64`. SteamCMD itself
starts from its `linux32/steamcmd` binary even when downloading a 64-bit game;
that log line is normal. Both GMod eggs first force a Windows app pass, then
force a Linux app pass into the same directory. The final pass supplies the
Linux server executable. The `-beta x86-64` argument selects GMod's 64-bit
branch for both passes. Do not add `@sSteamCmdForcePlatformBitness`; platform
type and platform bitness are separate SteamCMD controls.

After deploying the updated Core and daemon, open the failed service and click
**Retry installation**. Do not delete its UUID data directory. Atlas wipes the
failed game files and stale SteamCMD cache, reruns the installer, and only
creates the UUID-named Docker game container after installation succeeds.

Monitor and verify the retry with:

```bash
ATLAS_DATA_PATH=/var/lib/atlas/data
UUID=replace-with-service-uuid
docker compose logs -f daemon
sudo test -f "$ATLAS_DATA_PATH/$UUID/srcds_run_x64"
sudo docker inspect "$UUID" --format '{{.State.Status}}'
```

During installation, the data directory exists while `docker inspect` still
reports no such container. Success is indicated by `Installation SUCCEEDED`, a
present `srcds_run_x64`, and an inspectable UUID-named container.

## 🛡️ Multi-Node Deployment (Global Scaling)
To add a remote server as a game node:
1.  On the **Remote Server**, you only need the `daemon` service and its own `docker-compose.yml`.
2.  Register the new node in the **Central Panel** (as shown above) to get a **unique token** for that specific server.
3.  Configure the remote daemon with your **Panel's Public URL** and the **Unique Node Token**.
4.  Ensure port `8081` (Daemon API) and `2022` (SFTP) are open on the remote server's firewall.

## 🌐 Reverse Proxy (Subdomain example)
Point `panel.yourdomain.com` to Atlas by creating `/etc/nginx/sites-available/atlas`:

```nginx
server {
    listen 80;
    server_name panel.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:4000; # The ATLAS_PANEL_PORT
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # API Proxy (Required for console/websockets)
    location /api/v1/ {
        proxy_pass http://127.0.0.1:8080/api/v1/; # The ATLAS_CORE_PORT
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }
}
```

## 📄 License
MIT License
