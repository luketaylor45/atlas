# Atlas Game Server Platform

Atlas is a next-generation game server hosting panel designed for performance, modularity, and ease of use. It features a distributed architecture with a central control panel and lightweight daemon nodes.

## 🚀 Key Features
*   **Modern Dashboard**: React-based UI with modern design and real-time updates.
*   **Infrastructure as Code**: Manage game templates ("Eggs") as JSON files.
*   **Distributed Architecture**: Run the panel on one server and deploy game nodes globally.
*   **Docker Containerization**: Every game server runs in an isolated, secure Docker container.

## 📦 Production Deployment (Ubuntu)

Atlas is designed to run entirely within Docker. To avoid port conflicts and enable HTTPS, we recommend using an Nginx Reverse Proxy on the host.

### 1. Prerequisites
*   Ubuntu 20.04/22.04/24.04 LTS
*   Docker & Docker Compose installed.
*   Nginx installed on the host: `sudo apt install nginx`

### 2. Configuration & Security
Before launching, you **must** update the default database credentials in `docker-compose.yml`.

1.  Update `POSTGRES_USER` and `POSTGRES_PASSWORD`.
2.  Update `DATABASE_URL` in the `core` service to match.
3.  Set a secure `NODE_TOKEN` for the `daemon`.

### 3. Launching Atlas
Start the stack in detached mode:
```bash
docker-compose up -d --build
```
*The panel is now running internally on port 3000.*

### 4. Reverse Proxy Setup (Subdomain)
To attach Atlas to a subdomain (e.g., `panel.yourdomain.com`), create a new Nginx configuration:

`sudo nano /etc/nginx/sites-available/atlas`

```nginx
server {
    listen 80;
    server_name panel.yourdomain.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # API Proxy (Required for console/websockets)
    location /api/v1/ {
        proxy_pass http://localhost:8080/api/v1/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }
}
```

Enable the site and restart Nginx:
```bash
sudo ln -s /etc/nginx/sites-available/atlas /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

## 🥚 Managing Game Templates (Eggs)
Add your JSON definitions to the `eggs/` directory on the host:

```text
eggs/
├── games/                  # Parent Category
│   ├── gmod/               # Sub-Category
│   │   └── modern.json     # The Egg Definition
│   └── minecraft/
│       └── paper.json
```

## 🛡️ Multi-Node Deployment
To add additional remote nodes to your Atlas cluster:
1.  On the remote server, only deploy the `daemon` service.
2.  Ensure the `CORE_URL` points back to your main panel's IP or subdomain.
3.  Add the node in the **Admin > Nodes** section of the panel.

## 📄 License
MIT License
