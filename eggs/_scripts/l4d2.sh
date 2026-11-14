#!/bin/bash
# Left 4 Dead 2 dedicated server installer for Atlas.
#
# Runs in ghcr.io/pterodactyl/installers:debian as root with the server
# directory mounted at /mnt/server.
#
# This egg does not use _scripts/steamcmd.sh. An anonymous Steam login cannot
# download app 222860 for Linux into an empty directory (SteamCMD reports an
# invalid platform). The known workaround, which the upstream egg relies on
# too, is to download the Windows build first and then the Linux build over
# it, so this script runs SteamCMD once per platform.
#
# Variables it reads:
#   SRCDS_APPID  Steam app id of the dedicated server (222860)
set -euo pipefail

APPID="${SRCDS_APPID:-222860}"

export HOME=/mnt/server
export LANG=C.UTF-8
export LC_ALL=C.UTF-8

fail() {
    echo "[Atlas] Error: $1" >&2
    exit 1
}

install -d -m 0770 /mnt/server/steamcmd /mnt/server/steamapps

echo "[Atlas] SteamCMD phase 1/4: Downloading SteamCMD"
curl -fsSL --retry 3 --retry-delay 2 -o /tmp/steamcmd.tar.gz \
    https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz ||
    fail "could not download SteamCMD."
tar -xzf /tmp/steamcmd.tar.gz -C /mnt/server/steamcmd
rm -f /tmp/steamcmd.tar.gz

STEAMCMD=/mnt/server/steamcmd/steamcmd.sh

# The first run only updates SteamCMD itself and may exit non-zero.
"$STEAMCMD" +login anonymous +quit || true

# steam_update <platform> downloads the app for one platform, retrying twice.
steam_update() {
    local platform="$1" attempt=1
    until "$STEAMCMD" +@ShutdownOnFailedCommand 1 +@NoPromptForPassword 1 \
        +@sSteamCmdForcePlatformType "$platform" +force_install_dir /mnt/server \
        +login anonymous +app_update "$APPID" validate +quit; do
        if [ "$attempt" -ge 3 ]; then
            fail "SteamCMD could not download the ${platform} files of app ${APPID} after ${attempt} attempts."
        fi
        attempt=$((attempt + 1))
        echo "[Atlas] SteamCMD failed; retrying (attempt ${attempt} of 3)"
        sleep 5
    done
}

echo "[Atlas] SteamCMD phase 2/4: Downloading the Windows files of app ${APPID}"
steam_update windows

echo "[Atlas] SteamCMD phase 3/4: Downloading the Linux files of app ${APPID}"
steam_update linux

if [ ! -f /mnt/server/srcds_run ] || [ ! -f /mnt/server/srcds_linux ]; then
    fail "SteamCMD finished without installing the Linux server (srcds_run)."
fi
chmod u+x /mnt/server/srcds_run /mnt/server/srcds_linux

echo "[Atlas] SteamCMD phase 4/4: Installing Steam client libraries"
install -D -m 0644 /mnt/server/steamcmd/linux32/steamclient.so /mnt/server/.steam/sdk32/steamclient.so
install -D -m 0644 /mnt/server/steamcmd/linux64/steamclient.so /mnt/server/.steam/sdk64/steamclient.so

echo "[Atlas] Left 4 Dead 2 server installed"
