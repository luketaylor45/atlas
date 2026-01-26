# Atlas Game Server Platform

Atlas is a next-generation game server hosting panel designed for performance, modularity, and security.

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

## 🆘 Common Issues & Fixes

### Server crashes during `go build`
If your server turns off or resets during installation, it is likely running out of RAM (Out of Memory). We have limited the build to a single CPU core, but you should also **add a Swap file** to your Ubuntu server to handle the build load:

```bash
# Create a 2GB swap file
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
# Make it permanent
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### Port 80/3000 already in use
Check if another service (like default Nginx) is running and stop it:
`sudo systemctl stop apache2; sudo systemctl stop nginx`

## 📄 License
MIT License
