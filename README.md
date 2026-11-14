# Atlas Game Server Platform

Atlas is a game server hosting panel designed for performance, modularity, and security.

## 🚀 Key Features
*   **Infrastructure as Code**: Manage game templates ("Eggs") as JSON files.
*   **Distributed Architecture**: Deploy multiple game nodes globally from a single panel.
*   **Hardened Security**: Internal services (API/Database) are hidden from the public by default.
*   **Ready-made Games**: 26 eggs for Minecraft, Steam and Source servers, plus Pterodactyl egg import.
*   **Backups and Schedules**: Archive, restore and automate each server from the panel.

## 🎮 Games

Atlas ships with eggs for these servers. Each one installs itself, is told
which ports it was given, and shows as *Starting* until it is ready for players.

| Category | Eggs |
| --- | --- |
| Minecraft | Vanilla, Paper, Purpur, Fabric, Forge, NeoForge, Velocity (proxy), Bedrock |
| Survival | Palworld, Valheim, Rust, ARK: Survival Evolved, ARK: Survival Ascended, 7 Days to Die, Satisfactory, Enshrouded, Project Zomboid |
| Source Engine | Garry's Mod, Counter-Strike 2, Team Fortress 2, Left 4 Dead 2 |
| Sandbox & Other | Terraria, tModLoader, Factorio, FiveM |

Anything else can be added under **Admin > Eggs > Import**, which reads
Pterodactyl and Pelican egg files as they are: config-file rules, startup
detection, the stop command and variable rules are all carried over, and
variables that are clearly extra ports (`QUERY_PORT`, `RCON_PORT`, ...) become
ports Atlas allocates. The egg format, and how to write your own, is described
in [`eggs/README.md`](eggs/README.md).

Some games are large (Counter-Strike 2 needs about 60 GB, ARK about 30 GB) and
some need a lot of memory; each egg's description says what to allow for.

## 💾 Backups and Schedules

**Backups** are made from a server's **Backups** tab: an archive of all of its
files, which can be downloaded, deleted or restored. Restoring needs the server
to be stopped, and can either overwrite matching files or replace everything
with the backup's contents.

*   Archives are kept on the server's node under
    `DATA_PATH/.atlas/backups/<server id>/` and do not count against the
    server's disk limit, so leave room for them.
*   Each server may keep 3 backups by default. An admin can change that under
    **Admin > All servers > Edit** (0 turns backups off for that server).
*   Deleting a server deletes its backups.

**Schedules** run one thing at set times from a server's **Schedules** tab: a
power action (start, stop, restart, kill), a console command, or a backup.

*   Times are read in Core's time zone, which is UTC unless you set
    `ATLAS_TIMEZONE` in `.env` (for example `ATLAS_TIMEZONE=Europe/London`).
*   Scheduled backups rotate: once the limit is reached, the oldest scheduled
    backup makes room for the new one. Backups made by hand are never removed
    automatically.
*   A sub-user needs the *Manage backups* or *Manage schedules* permission, and
    can only schedule actions they are allowed to do by hand.
*   A schedule that was due while Atlas was down runs once when it comes back.

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

## Security Model

*   **Daemon API (port 8081)** is for Core only. Every route requires the node
    token. Compose publishes it on `127.0.0.1` and Core reaches the local
    daemon over the Compose network. Never forward this port on your router.
*   **Node internal address.** In **Admin > Nodes**, set *Internal Address* to
    `daemon` for the node that runs in the same Compose stack. The public
    address is still what players and SFTP clients use. A remote node sets
    `ATLAS_NODE_BIND` to a private or VPN address that only Core can reach and
    leaves *Internal Address* empty or set to that address.
*   **Console** traffic is relayed by Core; browsers never connect to a node.
    Your reverse proxy must pass websocket upgrades on `/api/v1/` (see the
    example below).
*   **Signing secret.** Leave `JWT_SECRET` unset and Atlas generates one and
    stores it in the database. If you set it, it must be at least 32 characters.
*   **Registration** is closed by default. Create accounts in **Admin > Users**,
    or set `REGISTRATION_ENABLED=true` to allow public sign-up.
*   **SFTP (port 2022)** is the only daemon port meant to be public. Repeated
    failed logins from an address are blocked for 30 minutes.

Optional `.env` settings added by this release:

```dotenv
# JWT_SECRET=                 # leave unset to auto-generate
REGISTRATION_ENABLED=false
ATLAS_NODE_BIND=127.0.0.1     # address the daemon API is published on
INSTALL_TIMEOUT_MINUTES=120   # a single installation is stopped after this long
```

### Upgrading from a release before the security update

After deploying, do these once:

1.  Open **Admin > Nodes**, edit the local node and set *Internal Address* to
    `daemon`. Until this is set Core will try to reach the daemon through the
    node's public address, which no longer accepts connections.
2.  Everyone must log in again: sessions signed with the old built-in secret
    are rejected.
3.  For each existing server, open **Settings > Rebuild Container**. Containers
    created by older releases carry the node token in their environment;
    rebuilding removes it. Files are kept and a running server is restarted.

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
