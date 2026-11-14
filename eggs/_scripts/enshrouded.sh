#!/bin/bash
# Enshrouded post-install steps. Listed after _scripts/steamcmd.sh, which has
# already downloaded the Windows build of the dedicated server
# (WINDOWS_INSTALL=1) into /mnt/server. It runs under Proton.
#
# - checks that the server executable arrived
# - creates the save and log directories
# - writes a first enshrouded_server.json (never overwrites one). The name,
#   port and player slots in it are rewritten from the panel's variables
#   before every start; the passwords are only written here.
#
# Variables it reads:
#   SERVER_PASSWORD  password of the "Player" group; generated when empty
#   ADMIN_PASSWORD   password of the "Admin" group; generated when empty
set -euo pipefail

cd /mnt/server

if [ ! -f enshrouded_server.exe ]; then
    echo "[Atlas] Error: SteamCMD finished without installing enshrouded_server.exe." >&2
    echo "[Atlas] The Windows build is required; check that WINDOWS_INSTALL is 1." >&2
    exit 1
fi

install -d -m 0770 logs savegame

# Escapes a value for use inside a JSON string.
json_text() {
    local value=$1
    value=${value//\\/\\\\}
    value=${value//\"/\\\"}
    printf '%s' "$value"
}

random_password() {
    head -c 9 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

if [ -f enshrouded_server.json ]; then
    echo "[Atlas] Keeping the existing enshrouded_server.json"
else
    echo "[Atlas] Writing enshrouded_server.json"
    PLAYER_PASSWORD=${SERVER_PASSWORD:-}
    GROUP_ADMIN_PASSWORD=${ADMIN_PASSWORD:-}
    if [ -z "$PLAYER_PASSWORD" ]; then
        PLAYER_PASSWORD=$(random_password)
        echo "[Atlas] No server password was set; a random one was generated."
    fi
    if [ -z "$GROUP_ADMIN_PASSWORD" ]; then
        GROUP_ADMIN_PASSWORD=$(random_password)
        echo "[Atlas] No admin password was set; a random one was generated."
    fi
    if [ "$PLAYER_PASSWORD" = "$GROUP_ADMIN_PASSWORD" ]; then
        echo "[Atlas] Warning: the server and admin passwords are the same, so every player joins as an admin."
    fi

    cat > enshrouded_server.json <<EOF
{
	"name": "Enshrouded Server",
	"saveDirectory": "./savegame",
	"logDirectory": "./logs",
	"ip": "0.0.0.0",
	"queryPort": 15637,
	"slotCount": 16,
	"gameSettingsPreset": "Default",
	"userGroups": [
		{
			"name": "Admin",
			"password": "$(json_text "$GROUP_ADMIN_PASSWORD")",
			"canKickBan": true,
			"canAccessInventories": true,
			"canEditBase": true,
			"canExtendBase": true,
			"reservedSlots": 0
		},
		{
			"name": "Player",
			"password": "$(json_text "$PLAYER_PASSWORD")",
			"canKickBan": false,
			"canAccessInventories": true,
			"canEditBase": true,
			"canExtendBase": false,
			"reservedSlots": 0
		}
	]
}
EOF
    echo "[Atlas] The passwords are in enshrouded_server.json under userGroups; edit them there to change them."
fi

echo "[Atlas] Enshrouded is installed"
