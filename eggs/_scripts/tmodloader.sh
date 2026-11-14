#!/bin/bash
# tModLoader dedicated server installer for Atlas.
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root with the server
# directory mounted at /mnt/server. The server itself runs on the .NET
# runtime image, so nothing is executed here.
#
# Variables it reads:
#   TMODLOADER_VERSION  "latest" (default, the newest stable release) or a
#                       release tag such as v2025.08.3.1
set -euo pipefail

REPO="https://github.com/tModLoader/tModLoader"
UA="atlas-panel/1.0"
TMODLOADER_VERSION="${TMODLOADER_VERSION:-latest}"

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

need_tools curl unzip jq
cd /mnt/server

if [ "${TMODLOADER_VERSION}" = "latest" ]; then
    # GitHub redirects this to the newest release that is not a pre-release.
    URL="${REPO}/releases/latest/download/tModLoader.zip"
else
    TAG="v${TMODLOADER_VERSION#v}"
    if ! [[ "${TAG}" =~ ^v20[0-9]{2}(\.[0-9]+){2,3}$ ]]; then
        fail "'${TMODLOADER_VERSION}' is not a supported tModLoader release. Use 'latest' or a tag such as v2025.08.3.1 (the 1.3-era v0.x releases are not supported)."
    fi
    URL="${REPO}/releases/download/${TAG}/tModLoader.zip"
fi

WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"' EXIT

echo "[Atlas] Downloading ${URL}"
if ! curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" -o "${WORK}/tModLoader.zip" "${URL}"; then
    fail "the download failed. Check that tModLoader release ${TMODLOADER_VERSION} exists."
fi
if [ "$(head -c 2 "${WORK}/tModLoader.zip")" != "PK" ]; then
    fail "the downloaded file is not a zip archive."
fi

echo "[Atlas] Unpacking tModLoader"
# Libraries holds versioned program files only; clearing it keeps an update
# from leaving old copies behind. Worlds, mods and configs are not touched.
rm -rf /mnt/server/Libraries
unzip -qo "${WORK}/tModLoader.zip" -x serverconfig.txt -d /mnt/server || fail "the archive could not be unpacked."
unzip -p "${WORK}/tModLoader.zip" serverconfig.txt > /mnt/server/serverconfig.example.txt 2> /dev/null ||
    rm -f /mnt/server/serverconfig.example.txt

if [ ! -f /mnt/server/tModLoader.dll ] || [ ! -f /mnt/server/tModLoader.runtimeconfig.json ]; then
    fail "the archive does not contain tModLoader.dll."
fi

DOTNET=$(jq -r '.runtimeOptions.framework.version // empty' /mnt/server/tModLoader.runtimeconfig.json 2> /dev/null) || DOTNET=""
if [ -n "${DOTNET}" ]; then
    echo "[Atlas] This release needs the .NET ${DOTNET%%.*} runtime (${DOTNET})"
    if [ "${DOTNET%%.*}" != "8" ]; then
        echo "[Atlas] Warning: pick the matching .NET image for this server; the default image carries .NET 8."
    fi
fi

mkdir -p /mnt/server/saves/Worlds /mnt/server/mods
if [ ! -f /mnt/server/serverconfig.txt ]; then
    # The world, port and player settings are passed on the command line.
    # Atlas rewrites the keys below from the server's variables on every
    # start; serverconfig.example.txt lists everything else that can go here.
    echo "[Atlas] Writing a default serverconfig.txt"
    cat > /mnt/server/serverconfig.txt << 'CONFIG'
difficulty=0
language=en-US
upnp=0
CONFIG
fi

echo "[Atlas] tModLoader installed"
