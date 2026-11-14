#!/bin/bash
# Shared SteamCMD installer for Atlas catalog eggs.
#
# Runs in ghcr.io/pterodactyl/installers:debian as root with the server
# directory mounted at /mnt/server. Eggs list this file first in
# "script_install_files" and may follow it with their own steps, so it must
# not exit on success.
#
# Variables it reads:
#   SRCDS_APPID      (required) Steam app id of the dedicated server
#   SRCDS_BETAID     optional beta branch
#   SRCDS_BETAPASS   optional beta branch password
#   WINDOWS_INSTALL  "1" to download the Windows build (for Wine/Proton eggs)
#   INSTALL_FLAGS    extra arguments appended to app_update
#   STEAM_USER / STEAM_PASS / STEAM_AUTH   account login; anonymous when unset
set -euo pipefail

if [ -z "${SRCDS_APPID:-}" ]; then
    echo "[Atlas] Error: this egg does not set SRCDS_APPID." >&2
    exit 1
fi

export HOME=/mnt/server
export LANG=C.UTF-8
export LC_ALL=C.UTF-8

install -d -m 0770 /mnt/server/steamcmd /mnt/server/steamapps

echo "[Atlas] SteamCMD phase 1/3: Downloading SteamCMD"
curl -fsSL --retry 3 --retry-delay 2 -o /tmp/steamcmd.tar.gz \
    https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz
tar -xzf /tmp/steamcmd.tar.gz -C /mnt/server/steamcmd
rm -f /tmp/steamcmd.tar.gz

STEAMCMD=/mnt/server/steamcmd/steamcmd.sh

if [ -z "${STEAM_USER:-}" ]; then
    STEAM_LOGIN=(+login anonymous)
else
    STEAM_LOGIN=(+login "${STEAM_USER}")
    if [ -n "${STEAM_PASS:-}" ]; then
        STEAM_LOGIN+=("${STEAM_PASS}")
    fi
    if [ -n "${STEAM_AUTH:-}" ]; then
        STEAM_LOGIN+=("${STEAM_AUTH}")
    fi
fi

STEAM_PLATFORM=()
if [ "${WINDOWS_INSTALL:-0}" = "1" ]; then
    STEAM_PLATFORM=(+@sSteamCmdForcePlatformType windows)
fi

APP_UPDATE=(+app_update "${SRCDS_APPID}")
if [ -n "${SRCDS_BETAID:-}" ]; then
    APP_UPDATE+=(-beta "${SRCDS_BETAID}")
    if [ -n "${SRCDS_BETAPASS:-}" ]; then
        APP_UPDATE+=(-betapassword "${SRCDS_BETAPASS}")
    fi
fi
if [ -n "${INSTALL_FLAGS:-}" ]; then
    # Intentionally split on spaces: this is a list of extra arguments.
    # shellcheck disable=SC2206
    APP_UPDATE+=(${INSTALL_FLAGS})
fi
APP_UPDATE+=(validate)

# The first run only updates SteamCMD itself and may exit non-zero.
"$STEAMCMD" +login anonymous +quit || true

echo "[Atlas] SteamCMD phase 2/3: Downloading app ${SRCDS_APPID}"
attempt=1
until "$STEAMCMD" +@ShutdownOnFailedCommand 1 +@NoPromptForPassword 1 "${STEAM_PLATFORM[@]}" \
    +force_install_dir /mnt/server "${STEAM_LOGIN[@]}" "${APP_UPDATE[@]}" +quit; do
    if [ "$attempt" -ge 3 ]; then
        echo "[Atlas] Error: SteamCMD could not download app ${SRCDS_APPID} after ${attempt} attempts." >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    echo "[Atlas] SteamCMD failed; retrying (attempt ${attempt} of 3)"
    sleep 5
done

echo "[Atlas] SteamCMD phase 3/3: Installing Steam client libraries"
install -D -m 0644 /mnt/server/steamcmd/linux32/steamclient.so /mnt/server/.steam/sdk32/steamclient.so
install -D -m 0644 /mnt/server/steamcmd/linux64/steamclient.so /mnt/server/.steam/sdk64/steamclient.so
