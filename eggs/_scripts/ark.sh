#!/bin/bash
# ARK: Survival Evolved post-install steps. Listed after _scripts/steamcmd.sh,
# which has already downloaded the dedicated server into /mnt/server.
#
# - checks that the server binary arrived
# - gives the server its own SteamCMD, which "-automanagedmods" uses to
#   download and update workshop mods, and links the directory the server
#   reads mods from to the one that SteamCMD downloads them into
set -euo pipefail

cd /mnt/server

ARK_BINARY=ShooterGame/Binaries/Linux/ShooterGameServer
if [ ! -f "$ARK_BINARY" ]; then
    echo "[Atlas] Error: SteamCMD finished without installing ${ARK_BINARY}." >&2
    exit 1
fi
chmod u+x "$ARK_BINARY"

echo "[Atlas] Installing SteamCMD for automatically managed mods"
MOD_STEAMCMD=Engine/Binaries/ThirdParty/SteamCMD/Linux
install -d -m 0770 "$MOD_STEAMCMD" Steam/steamapps
curl -fsSL --retry 3 --retry-delay 2 -o /tmp/steamcmd-mods.tar.gz \
    https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz
tar -xzf /tmp/steamcmd-mods.tar.gz -C "$MOD_STEAMCMD"
rm -f /tmp/steamcmd-mods.tar.gz

# Relative, so the link also resolves at /home/container when the server runs.
if [ -L "$MOD_STEAMCMD/steamapps" ] || [ ! -e "$MOD_STEAMCMD/steamapps" ]; then
    ln -sfn ../../../../../Steam/steamapps "$MOD_STEAMCMD/steamapps"
else
    echo "[Atlas] Keeping the existing ${MOD_STEAMCMD}/steamapps directory"
fi

echo "[Atlas] ARK: Survival Evolved is installed"
