#!/bin/ash
# Velocity installation script for Atlas.
#
# Runs in ghcr.io/pterodactyl/installers:alpine as root with the server
# directory mounted at /mnt/server.
#
# Variables it reads:
#   VELOCITY_VERSION  Velocity version, or "latest" (default)
#   BUILD_NUMBER      build number, or "latest" (default)
#   SERVER_JARFILE    name of the jar to write (default velocity.jar)
#   DL_PATH           optional direct download URL; skips the version lookup
set -eu

PROJECT=velocity
API="https://fill.papermc.io/v3/projects/${PROJECT}"
UA="atlas-panel/1.0 (https://github.com/luketaylor45/atlas)"
cd /mnt/server

SERVER_JARFILE="${SERVER_JARFILE:-velocity.jar}"
VELOCITY_VERSION="${VELOCITY_VERSION:-latest}"
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
    echo "[Atlas] Looking up Velocity versions"
    VERSIONS=$(curl -fsSL --retry 3 -H "User-Agent: ${UA}" "${API}") || {
        echo "[Atlas] Error: could not reach the PaperMC API." >&2
        exit 1
    }

    if [ "${VELOCITY_VERSION}" = "latest" ]; then
        # Newest release; snapshots are only used when there is no release.
        VERSION=$(echo "${VERSIONS}" | jq -r '[.versions[][] | select(test("-") | not)] | .[0]') || VERSION=""
        if [ -z "${VERSION}" ] || [ "${VERSION}" = "null" ]; then
            VERSION=$(echo "${VERSIONS}" | jq -r '[.versions[][]] | .[0]') || VERSION=""
        fi
    elif echo "${VERSIONS}" | jq -e --arg v "${VELOCITY_VERSION}" '[.versions[][]] | index($v) != null' > /dev/null; then
        VERSION="${VELOCITY_VERSION}"
    else
        echo "[Atlas] Error: Velocity has no version ${VELOCITY_VERSION}." >&2
        exit 1
    fi
    if [ -z "${VERSION}" ] || [ "${VERSION}" = "null" ]; then
        echo "[Atlas] Error: could not determine a Velocity version." >&2
        exit 1
    fi

    echo "[Atlas] Installing Velocity ${VERSION} (build ${BUILD_NUMBER})"
    BUILD=$(curl -fsSL --retry 3 -H "User-Agent: ${UA}" "${API}/versions/${VERSION}/builds/${BUILD_NUMBER}") || {
        echo "[Atlas] Error: Velocity ${VERSION} has no build ${BUILD_NUMBER}." >&2
        exit 1
    }
    DOWNLOAD_URL=$(echo "${BUILD}" | jq -r '.downloads["server:default"].url') || DOWNLOAD_URL=""
    if [ -z "${DOWNLOAD_URL}" ] || [ "${DOWNLOAD_URL}" = "null" ]; then
        echo "[Atlas] Error: that Velocity build has no download." >&2
        exit 1
    fi

    download_jar "${DOWNLOAD_URL}" "${SERVER_JARFILE}"

    # Tell the admin which Java image this version needs. Not fatal.
    if INFO=$(curl -fsSL --retry 3 -H "User-Agent: ${UA}" "${API}/versions/${VERSION}" 2> /dev/null); then
        JAVA_MIN=$(echo "${INFO}" | jq -r '.version.java.version.minimum // empty' 2> /dev/null) || JAVA_MIN=""
        if [ -n "${JAVA_MIN}" ]; then
            echo "[Atlas] Velocity ${VERSION} needs Java ${JAVA_MIN} or newer"
        fi
    fi
fi

# Atlas sets the bind address in velocity.toml before every start, which only
# works when the file exists. Start from the default that ships in the jar.
if [ ! -f velocity.toml ]; then
    if unzip -p "${SERVER_JARFILE}" default-velocity.toml > velocity.toml.part 2> /dev/null && [ -s velocity.toml.part ]; then
        mv -f velocity.toml.part velocity.toml
        echo "[Atlas] Wrote the default velocity.toml"
    else
        rm -f velocity.toml.part
        echo "[Atlas] Warning: could not read the default velocity.toml from the jar." >&2
        echo "[Atlas] Velocity creates it on first start; restart once so the port is applied." >&2
    fi
fi

echo "[Atlas] Finalizing installation"
