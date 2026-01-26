# Atlas Game Server Platform

Atlas is a next-generation game server hosting panel designed for performance, modularity, and ease of use. It features a distributed architecture with a central control panel and lightweight daemon nodes.

## 🚀 Key Features
*   **Modern Dashboard**: React-based UI with glassmorphism design and real-time updates.
*   **Infrastructure as Code**: Manage game templates ("Eggs") as JSON files with version control.
*   **Distributed Architecture**: Run the panel on one server and deploy game nodes globally.
*   **Nested Categories**: Organize servers into customizable hierarchies (e.g., Games > Minecraft > Paper).
*   **Docker Containerization**: Every game server runs in an isolated, secure Docker container.

## 📦 Components
*   **Panel**: The web interface (React/Vite).
*   **Core**: The central API and database manager (Go).
*   **Daemon**: The remote agent that runs on game nodes (Go).
*   **Seed**: A utility to hydrate the database from the file system.

## 🛠️ Installation (Ubuntu)

1.  **Clone the Repository** (or upload the files):
    ```bash
    git clone https://github.com/your-repo/atlas.git
    cd atlas
    ```

2.  **Run the Installer**:
    This script will install Docker, Go, Node.js, and PostgreSQL, then build and start all services.
    ```bash
    bash install.sh
    ```

3.  **Access the Panel**:
    Open your browser and navigate to `http://<your-server-ip>`.

## 🥚 Managing Game Templates (Eggs)
Atlas uses a file-based structure for defining game environments. You can add new games by dropping JSON files into the `eggs/` directory.

**Structure:**
```text
eggs/
├── games/                  # Parent Category (Nest)
│   ├── gmod/               # Sub-Category
│   │   └── 64bit.json      # The Egg Definition
│   └── minecraft/
│       └── paper.json
```

**Importing:**
During installation, the `atlas-seed` tool automatically imports this structure. Post-installation, you can manually import eggs via the **Admin > Eggs & Nests** page.

## 💻 Development
To run Atlas locally for development:

1.  **Start the Panel**:
    ```bash
    cd panel
    npm run dev
    ```
2.  **Start the Core**:
    ```bash
    cd core
    go run cmd/server/main.go
    ```
3.  **Start the Daemon**:
    ```bash
    cd daemon
    go run cmd/server/main.go
    ```

> Note: Ensure you have Docker Desktop running on Windows, or Docker Engine on Linux.

## 📄 License
MIT License
