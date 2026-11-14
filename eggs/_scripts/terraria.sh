#!/bin/bash
# Terraria (vanilla) dedicated server installer for Atlas.
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root with the server
# directory mounted at /mnt/server.
#
# Variables it reads:
#   TERRARIA_VERSION  "latest" (default) or a 1.4+ version, written either as
#                     1.4.4.9 or as the number in the download name (1449)
set -euo pipefail

API="https://terraria.org/api"
UA="atlas-panel/1.0"
TERRARIA_VERSION="${TERRARIA_VERSION:-latest}"

fail() {
    echo "[Atlas] Error: $1" >&2
    exit 1
}

# need_tools <command[:package]>... installs whatever the image is missing.
need_tools() {
    local missing=() tool
    for tool in "$@"; do
        command -v "${tool%%:*}" > /dev/null 2>&1 || missing+=("${tool##*:}")
    done
    if [ "${#missing[@]}" -gt 0 ]; then
        echo "[Atlas] Installing required tools: ${missing[*]}"
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq && apt-get install -y -qq --no-install-recommends "${missing[@]}" ||
            fail "could not install ${missing[*]}."
    fi
}

case "$(uname -m)" in
    x86_64 | amd64) ;;
    *) fail "the Terraria dedicated server only runs on x86-64 hosts." ;;
esac

need_tools curl unzip jq
cd /mnt/server

if [ "${TERRARIA_VERSION}" = "latest" ]; then
    echo "[Atlas] Looking up the latest Terraria server"
    NAMES=$(curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" "${API}/get/dedicated-servers-names") ||
        fail "could not reach terraria.org."
    # The list is newest first: ["terraria-server-1458.zip", ...]
    ARCHIVE=$(echo "${NAMES}" | jq -r 'if type == "array" then .[0] else empty end' 2> /dev/null) || ARCHIVE=""
    if ! [[ "${ARCHIVE}" =~ ^terraria-server-[0-9]+\.zip$ ]]; then
        fail "terraria.org returned an unexpected server list: ${NAMES}"
    fi
else
    DIGITS="${TERRARIA_VERSION//./}"
    if ! [[ "${DIGITS}" =~ ^[0-9]{3,6}$ ]]; then
        fail "'${TERRARIA_VERSION}' is not a Terraria version. Use 'latest' or a version such as 1.4.4.9."
    fi
    ARCHIVE="terraria-server-${DIGITS}.zip"
fi
BUILD="${ARCHIVE#terraria-server-}"
BUILD="${BUILD%.zip}"

WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"' EXIT

URL="${API}/download/pc-dedicated-server/${ARCHIVE}"
echo "[Atlas] Downloading ${URL}"
if ! curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" -o "${WORK}/server.zip" "${URL}"; then
    fail "the download failed. terraria.org only hosts 1.4 and newer servers; check that version ${TERRARIA_VERSION} exists."
fi
if [ "$(head -c 2 "${WORK}/server.zip")" != "PK" ]; then
    fail "the downloaded file is not a zip archive."
fi

echo "[Atlas] Unpacking Terraria server ${BUILD}"
unzip -q "${WORK}/server.zip" -d "${WORK}/unpacked" || fail "the archive could not be unpacked."
SOURCE="${WORK}/unpacked/${BUILD}/Linux"
if [ ! -f "${SOURCE}/TerrariaServer.bin.x86_64" ]; then
    # Fall back to wherever the Linux build sits in this archive.
    FOUND=$(find "${WORK}/unpacked" -type f -name TerrariaServer.bin.x86_64 | head -n 1)
    [ -n "${FOUND}" ] || fail "the archive does not contain a Linux server (TerrariaServer.bin.x86_64)."
    SOURCE=$(dirname "${FOUND}")
fi

cp -a "${SOURCE}/." /mnt/server/
chmod +x /mnt/server/TerrariaServer.bin.x86_64 /mnt/server/TerrariaServer 2> /dev/null || true

mkdir -p /mnt/server/saves/Worlds
if [ ! -f /mnt/server/serverconfig.txt ]; then
    # The Linux build ships without a config file. Atlas rewrites the world,
    # port and player settings from the server's variables on every start.
    echo "[Atlas] Writing a default serverconfig.txt"
    cat > /mnt/server/serverconfig.txt << 'CONFIG'
worldpath=/home/container/saves/Worlds
worldname=world
world=/home/container/saves/Worlds/world.wld
autocreate=1
difficulty=0
port=7777
maxplayers=8
upnp=0
CONFIG
fi

echo "[Atlas] Terraria server ${BUILD} installed"
