# Atlas Game Server Platform

Atlas is a next-generation game server hosting panel designed for performance, modularity, and ease of use. It features a distributed architecture with a central control panel and lightweight daemon nodes.

## 🚀 Key Features
*   **Modern Dashboard**: React-based UI with glassmorphism design and real-time updates.
*   **Infrastructure as Code**: Manage game templates ("Eggs") as JSON files with version control.
*   **Distributed Architecture**: Run the panel on one server and deploy game nodes globally.
*   **Docker Containerization**: Every game server runs in an isolated, secure Docker container.

## 📦 Components
*   **Panel**: The web interface (React/Vite).
*   **Core**: The central API and database manager (Go).
*   **Daemon**: The remote agent that runs on game nodes (Go).

## 🛠️ Quick Start (Docker Compose)

To run the entire stack (Panel, API, Database, Daemon) with a single command:

1.  **Clone the Repository**:
    ```bash
    git clone https://github.com/your-repo/atlas.git
    cd atlas
    ```

2.  **Start Services**:
    ```bash
    docker-compose up -d --build
    ```

3.  **Access the Panel**:
    Open your browser and navigate to `http://<your-server-ip>`.

The setup process runs automatically on the first boot, creating the necessary database schema and importing default game templates from the `eggs/` directory.

## 🥚 Adding Game Templates (Eggs)
To add new game types to your platform, simply add their JSON definitions to the `eggs/` directory:

```text
eggs/
├── games/
│   ├── gmod/
│   │   └── modern.json
│   └── minecraft/
│       └── paper.json
```

Then restart the core service or use the import feature in the Admin Panel.

## 💻 Development
To run Atlas locally for contribution:

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

## 📄 License
MIT License
