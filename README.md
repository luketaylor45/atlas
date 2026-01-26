# Atlas Game Server Platform

Atlas is a next-generation game server hosting panel designed for performance, modularity, and ease of use. It features a distributed architecture with a central control panel and lightweight daemon nodes.

## 🚀 Key Features
*   **Modern Dashboard**: React-based UI with modern design and real-time updates.
*   **Infrastructure as Code**: Manage game templates ("Eggs") as JSON files with version control.
*   **Distributed Architecture**: Run the panel on one server and deploy game nodes globally.
*   **Docker Containerization**: Every game server runs in an isolated, secure Docker container.

## 📦 Production Deployment (Ubuntu)

Atlas is designed to run entirely within Docker for maximum reliability and continuous operation.

### 1. Prerequisites
*   Ubuntu 20.04/22.04/24.04 LTS
*   Docker and Docker Compose installed:
    ```bash
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    ```

### 2. Installation
Clone the repository and enter the project directory:
```bash
git clone https://github.com/luketaylor45/atlas.git
cd atlas
```

### 3. Configuration & Security
Before launching, you **must** update the default database credentials for security.

1.  Open `docker-compose.yml`.
2.  Update the following environment variables:
    *   **Atlas Database Service**:
        *   `POSTGRES_USER`: Choose a secure username.
        *   `POSTGRES_PASSWORD`: Choose a strong password.
    *   **Atlas Core Service**:
        *   `DATABASE_URL`: Update the string with your new user and password: `host=database user=<USER> password=<PASS> dbname=atlas...`

### 4. Launching the Platform
Run the following command to build and start the entire stack in the background:
```bash
docker-compose up -d --build
```

**Services will be available at:**
*   **Control Panel**: `http://<your-server-ip>`
*   **Core API**: `http://<your-server-ip>:8080`
*   **SFTP Port**: `2022`

All services are configured for `restart: always`, meaning they will automatically restart on server reboots or process failures.

## 🥚 Managing Game Templates (Eggs)
Atlas uses a hierarchical file-based system for game environments. Add your JSON definitions to the `eggs/` directory on the host:

**Directory Structure:**
```text
eggs/
├── games/                  # Parent Category
│   ├── gmod/               # Sub-Category
│   │   └── modern.json     # The Egg Definition
│   └── minecraft/
│       └── paper.json
```

The Core service will automatically import these files into the database on every startup.

## �️ Multi-Node Deployment
To add additional remote nodes to your Atlas cluster:
1.  On the remote server, only deploy the `daemon` service using its own `docker-compose.yml`.
2.  Ensure the `CORE_URL` points back to your main panel's IP.
3.  Add the node in the **Admin > Nodes** section of the panel.

## 📄 License
MIT License
