#!/bin/bash
# ARK: Survival Ascended post-install steps. Listed after
# _scripts/steamcmd.sh, which has already downloaded the Windows build of the
# dedicated server (WINDOWS_INSTALL=1) into /mnt/server. It runs under Proton.
#
# - checks that the server executable arrived
# - creates the log file the startup command follows, since the server does
#   not print its log to the console
set -euo pipefail

cd /mnt/server

ASA_BINARY=ShooterGame/Binaries/Win64/ArkAscendedServer.exe
if [ ! -f "$ASA_BINARY" ]; then
    echo "[Atlas] Error: SteamCMD finished without installing ${ASA_BINARY}." >&2
    echo "[Atlas] The Windows build is required; check that WINDOWS_INSTALL is 1." >&2
    exit 1
fi

install -d -m 0770 ShooterGame/Saved/Logs
touch ShooterGame/Saved/Logs/ShooterGame.log

echo "[Atlas] ARK: Survival Ascended is installed"
