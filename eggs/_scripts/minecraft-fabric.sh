#!/bin/ash
# Fabric installation script for Atlas.
#
# Runs in ghcr.io/pterodactyl/installers:alpine as root with the server
# directory mounted at /mnt/server.
#
# Downloads Fabric's server launcher jar. The launcher fetches the Minecraft
# server and its libraries itself the first time the server starts.
#
# Variables it reads:
#   MC_VERSION      Minecraft version, "latest" (default) or "snapshot"
#   LOADER_VERSION  Fabric loader version, or "latest" (default)
#   FABRIC_VERSION  Fabric installer (launcher) version, or "latest" (default)
#   SERVER_JARFILE  name of the jar to write (default server.jar)
set -eu

META="https://meta.fabricmc.net/v2/versions"
cd /mnt/server

SERVER_JARFILE="${SERVER_JARFILE:-server.jar}"
MC_VERSION="${MC_VERSION:-latest}"
LOADER_VERSION="${LOADER_VERSION:-latest}"
FABRIC_VERSION="${FABRIC_VERSION:-latest}"

fail() {
    echo "[Atlas] Error: $1" >&2
    exit 1
}

download_jar() {
    url="$1"
    target="$2"
    echo "[Atlas] Downloading ${url}"
    rm -f "${target}.part"
    if ! curl -fSL --retry 3 --retry-delay 2 -o "${target}.part" "${url}"; then
        echo "[Atlas] Error: the download failed." >&2
        rm -f "${target}.part"
        exit 1
    fi
    if [ "$(head -c 2 "${target}.part")" != "PK" ]; then
        echo "[Atlas] Error: the downloaded file is not a valid jar." >&2
        rm -f "${target}.part"
        exit 1
    fi
    mv -f "${target}.part" "${target}"
}

# Every list is ordered newest first.
echo "[Atlas] Looking up Minecraft versions supported by Fabric"
GAMES=$(curl -fsSL --retry 3 "${META}/game") || fail "could not reach the Fabric meta API."
case "${MC_VERSION}" in
    latest)
        GAME=$(echo "${GAMES}" | jq -r '[.[] | select(.stable)] | .[0].version') || GAME=""
        ;;
    snapshot)
        GAME=$(echo "${GAMES}" | jq -r '.[0].version') || GAME=""
        ;;
    *)
        if echo "${GAMES}" | jq -e --arg v "${MC_VERSION}" 'map(.version) | index($v) != null' > /dev/null; then
            GAME="${MC_VERSION}"
        else
            fail "Fabric has no support for Minecraft ${MC_VERSION}."
        fi
        ;;
esac
if [ -z "${GAME}" ] || [ "${GAME}" = "null" ]; then
    fail "could not determine a Minecraft version."
fi

if [ "${LOADER_VERSION}" = "latest" ]; then
    LOADERS=$(curl -fsSL --retry 3 "${META}/loader") || fail "could not look up Fabric loader versions."
    LOADER=$(echo "${LOADERS}" | jq -r '[.[] | select(.stable)] | .[0].version') || LOADER=""
else
    LOADER="${LOADER_VERSION}"
fi
if [ -z "${LOADER}" ] || [ "${LOADER}" = "null" ]; then
    fail "could not determine a Fabric loader version."
fi

if [ "${FABRIC_VERSION}" = "latest" ]; then
    INSTALLERS=$(curl -fsSL --retry 3 "${META}/installer") || fail "could not look up Fabric installer versions."
    INSTALLER=$(echo "${INSTALLERS}" | jq -r '[.[] | select(.stable)] | .[0].version') || INSTALLER=""
else
    INSTALLER="${FABRIC_VERSION}"
fi
if [ -z "${INSTALLER}" ] || [ "${INSTALLER}" = "null" ]; then
    fail "could not determine a Fabric installer version."
fi

echo "[Atlas] Installing Fabric for Minecraft ${GAME} (loader ${LOADER}, installer ${INSTALLER})"
download_jar "${META}/loader/${GAME}/${LOADER}/${INSTALLER}/server/jar" "${SERVER_JARFILE}"

echo "eula=true" > eula.txt
echo "[Atlas] Finalizing installation"
