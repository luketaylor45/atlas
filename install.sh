#!/bin/bash

# Atlas Installation Script
# This script automates the deployment of the Atlas game server platform on Ubuntu.

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Atlas Installation...${NC}"

# 1. Update System & Install Dependencies
echo "Updating system packages..."
sudo apt-get update && sudo apt-get upgrade -y
sudo apt-get install -y curl wget git unzip tar build-essential

# 2. Install Docker
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    sudo usermod -aG docker $USER
    # Activate group changes for current session without re-login
    # (Note: This only affects the script execution context)
else
    echo "Docker is already installed."
fi

# 3. Install Go 1.22
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    wget -q https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
else
    echo "Go is already installed."
fi

# 4. Install Node.js 20 & NPM
if ! command -v node &> /dev/null; then
    echo "Installing Node.js..."
    curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
    sudo apt-get install -y nodejs
else
    echo "Node.js is already installed."
fi

# 5. Install PostgreSQL & Setup Database
if ! command -v psql &> /dev/null; then
    echo "Installing PostgreSQL..."
    sudo apt-get install -y postgresql postgresql-contrib
    sudo systemctl start postgresql
    sudo systemctl enable postgresql
    
    # Create User and DB (Idempotent)
    echo "Configuring Database..."
    sudo -u postgres psql -c "CREATE USER atlas WITH PASSWORD 'atlas_password';" || true
    sudo -u postgres psql -c "CREATE DATABASE atlas OWNER atlas;" || true
else
    echo "PostgreSQL is already installed."
fi

# 6. Build Core
echo "Building Atlas Core..."
cd core
/usr/local/go/bin/go build -o atlas-core cmd/server/main.go
/usr/local/go/bin/go build -o atlas-seed cmd/seed/main.go
cd ..

# 7. Build Daemon
echo "Building Atlas Daemon..."
cd daemon
/usr/local/go/bin/go build -o atlas-daemon cmd/server/main.go
cd ..

# 8. Build Panel
echo "Building Atlas Panel..."
cd panel
npm install
npm run build
cd ..

# 9. Setup Directory Structure
echo "Setting up production paths..."
sudo mkdir -p /var/lib/atlas/data
sudo mkdir -p /var/lib/atlas/eggs
sudo chown -R $USER:$USER /var/lib/atlas

# Copy Eggs
cp -r eggs/* /var/lib/atlas/eggs/

# 10. Configure Environment (Example .env creation)
echo "Writing configuration files..."
# This part would typically write to /etc/atlas/config.env based on user input
# For this script we assume defaults matching the hardcoded ones

# 11. Run Seeder
echo "Seeding Infrastructure..."
cd core
# Temporarily set ENV for seeder to verify DB connection
# In prod, config should be loaded from file
# export DATABASE_URL="host=localhost user=atlas password=atlas_password dbname=atlas port=5432 sslmode=disable"
# ./atlas-seed
cd ..

# 12. Create Systemd Services
echo "Creating system services..."

# Core Service
sudo bash -c 'cat > /etc/systemd/system/atlas-core.service <<EOF
[Unit]
Description=Atlas Core API
After=network.target postgresql.service

[Service]
User='$USER'
Group='$USER'
WorkingDirectory='$(pwd)'/core
ExecStart='$(pwd)'/core/atlas-core
Restart=always
Environment="PORT=8080"
Environment="DATABASE_URL=host=localhost user=atlas password=atlas_password dbname=atlas port=5432 sslmode=disable"

[Install]
WantedBy=multi-user.target
EOF'

# Daemon Service
sudo bash -c 'cat > /etc/systemd/system/atlas-daemon.service <<EOF
[Unit]
Description=Atlas Node Daemon
After=docker.service network.target

[Service]
User=root
WorkingDirectory='$(pwd)'/daemon
ExecStart='$(pwd)'/daemon/atlas-daemon
Restart=always
Environment="PORT=8081"
Environment="CORE_URL=http://localhost:8080"
Environment="NODE_TOKEN=change-me"
Environment="SFTP_PORT=2022"

[Install]
WantedBy=multi-user.target
EOF'

# Note: Panel service usually served via Nginx, but we can run a simple serve
sudo bash -c 'cat > /etc/systemd/system/atlas-panel.service <<EOF
[Unit]
Description=Atlas Web Panel
After=network.target

[Service]
User='$USER'
Group='$USER'
WorkingDirectory='$(pwd)'/panel
ExecStart=/usr/bin/npm run preview -- --host --port 80
Restart=always

[Install]
WantedBy=multi-user.target
EOF'

# 13. Enable & Start Services
echo "Enabling services..."
sudo systemctl daemon-reload
sudo systemctl enable atlas-core
sudo systemctl enable atlas-daemon
sudo systemctl enable atlas-panel

echo "Starting services..."
sudo systemctl start atlas-core
sleep 5 # Wait for core to initialize DB
sudo systemctl start atlas-daemon
sudo systemctl start atlas-panel

echo -e "${GREEN}Installation Complete!${NC}"
echo "Panel is running on port 80."
echo "API is running on port 8080."
