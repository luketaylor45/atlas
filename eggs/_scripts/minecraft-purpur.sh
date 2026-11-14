#!/bin/ash
# Purpur installation script for Atlas.
#
# Runs in ghcr.io/pterodactyl/installers:alpine as root with the server
# directory mounted at /mnt/server.
#
# Variables it reads:
#   MINECRAFT_VERSION  Minecraft version, or "latest" (default)
#   BUILD_NUMBER       Purpur build number, or "latest" (default)
#   SERVER_JARFILE     name of the jar to write (default server.jar)
#   DL_PATH            optional direct download URL; skips the version lookup
set -eu

API="https://api.purpurmc.org/v2/purpur"
cd /mnt/server

SERVER_JARFILE="${SERVER_JARFILE:-server.jar}"
MINECRAFT_VERSION="${MINECRAFT_VERSION:-latest}"
BUILD_NUMBER="${BUILD_NUMBER:-latest}"

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

if [ -n "${DL_PATH:-}" ]; then
    echo "[Atlas] Using the supplied download URL"
    download_jar "${DL_PATH}" "${SERVER_JARFILE}"
else
    if [ "${MINECRAFT_VERSION}" = "latest" ]; then
        echo "[Atlas] Looking up Purpur versions"
        PROJECT=$(curl -fsSL --retry 3 "${API}") || {
            echo "[Atlas] Error: could not reach the Purpur API." >&2
            exit 1
        }
        # The list is ordered oldest to newest.
        VERSION=$(echo "${PROJECT}" | jq -r '.versions[-1]') || VERSION=""
        if [ -z "${VERSION}" ] || [ "${VERSION}" = "null" ]; then
            echo "[Atlas] Error: could not determine the newest Purpur version." >&2
            exit 1
        fi
    else
        VERSION="${MINECRAFT_VERSION}"
    fi

    BUILDS=$(curl -fsSL --retry 3 "${API}/${VERSION}") || {
        echo "[Atlas] Error: Purpur has no version ${VERSION}." >&2
        exit 1
    }

    if [ "${BUILD_NUMBER}" = "latest" ]; then
        BUILD=$(echo "${BUILDS}" | jq -r '.builds.latest') || BUILD=""
    elif echo "${BUILDS}" | jq -e --arg b "${BUILD_NUMBER}" '.builds.all | index($b) != null' > /dev/null; then
        BUILD="${BUILD_NUMBER}"
    else
        echo "[Atlas] Error: Purpur ${VERSION} has no build ${BUILD_NUMBER}." >&2
        exit 1
    fi
    if [ -z "${BUILD}" ] || [ "${BUILD}" = "null" ]; then
        echo "[Atlas] Error: Purpur ${VERSION} has no builds." >&2
        exit 1
    fi

    echo "[Atlas] Installing Purpur ${VERSION} (build ${BUILD})"
    download_jar "${API}/${VERSION}/${BUILD}/download" "${SERVER_JARFILE}"
fi

echo "eula=true" > eula.txt
echo "[Atlas] Finalizing installation"
