#!/bin/bash
# Minecraft Forge installation script for Atlas.
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root with the server
# directory mounted at /mnt/server. Listed after _scripts/minecraft-java.sh,
# which provides atlas_install_java.
#
# Forge 1.17 and newer start from an argument file; the installer writes it
# under libraries/ and this script copies it to unix_args.txt. Older versions
# start from a jar, which is renamed to SERVER_JARFILE. The egg's startup
# command uses unix_args.txt when it exists and the jar otherwise.
#
# Variables it reads:
#   MC_VERSION      Minecraft version, or "latest" (default)
#   BUILD_TYPE      "recommended" (default) or "latest"
#   FORGE_VERSION   optional exact version such as 1.20.1-47.4.28; overrides
#                   MC_VERSION and BUILD_TYPE
#   SERVER_JARFILE  jar name for versions before 1.17 (default server.jar)
set -euo pipefail

MAVEN="https://maven.minecraftforge.net/net/minecraftforge/forge"
PROMOTIONS="https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"
cd /mnt/server

SERVER_JARFILE="${SERVER_JARFILE:-server.jar}"
MC_VERSION="$(echo "${MC_VERSION:-latest}" | tr -d '[:space:]')"
BUILD_TYPE="${BUILD_TYPE:-recommended}"
FORGE_VERSION="$(echo "${FORGE_VERSION:-}" | tr -d '[:space:]')"
MC_VERSION="${MC_VERSION:-latest}"

fail() {
    echo "[Atlas] Error: $1" >&2
    exit 1
}

download_jar() {
    local url="$1" target="$2"
    echo "[Atlas] Downloading ${url}"
    rm -f "${target}.part"
    if ! curl -fSL --retry 3 --retry-delay 2 -o "${target}.part" "${url}"; then
        rm -f "${target}.part"
        fail "the download failed."
    fi
    if [ "$(head -c 2 "${target}.part")" != "PK" ]; then
        rm -f "${target}.part"
        fail "the downloaded file is not a valid jar."
    fi
    mv -f "${target}.part" "${target}"
}

# Work out the Forge version, written the way the Forge Maven does:
# <minecraft version>-<forge build>.
if [ -n "${FORGE_VERSION}" ]; then
    case "${FORGE_VERSION}" in
        *-*) VERSION="${FORGE_VERSION}" ;;
        *)
            if [ "${MC_VERSION}" = "latest" ]; then
                fail "Forge Version must include the Minecraft version, for example 1.20.1-47.4.28."
            fi
            VERSION="${MC_VERSION}-${FORGE_VERSION}"
            ;;
    esac
    MC_VERSION="${VERSION%%-*}"
else
    echo "[Atlas] Looking up Forge versions"
    PROMOS=$(curl -fsSL --retry 3 "${PROMOTIONS}") || fail "could not reach the Forge version list."

    if [ "${MC_VERSION}" = "latest" ]; then
        # Keys look like "1.21.11-recommended" and "26.3-latest".
        MC_VERSION=$(echo "${PROMOS}" | jq -r '.promos | keys[]' | sed 's/-[a-z]*$//' | sort -V | tail -n 1) || MC_VERSION=""
        if [ -z "${MC_VERSION}" ]; then
            fail "could not determine the newest Minecraft version Forge supports."
        fi
    fi

    if [ "${BUILD_TYPE}" != "latest" ]; then
        BUILD_TYPE=recommended
    fi
    BUILD=$(echo "${PROMOS}" | jq -r --arg k "${MC_VERSION}-${BUILD_TYPE}" '.promos[$k] // empty') || BUILD=""
    if [ -z "${BUILD}" ] && [ "${BUILD_TYPE}" = "recommended" ]; then
        BUILD=$(echo "${PROMOS}" | jq -r --arg k "${MC_VERSION}-latest" '.promos[$k] // empty') || BUILD=""
        if [ -n "${BUILD}" ]; then
            echo "[Atlas] Forge has no recommended build for Minecraft ${MC_VERSION}; using the latest build"
        fi
    fi
    if [ -z "${BUILD}" ]; then
        fail "Forge has no build for Minecraft ${MC_VERSION}."
    fi
    VERSION="${MC_VERSION}-${BUILD}"
fi

# A few old releases repeat the Minecraft version at the end of their Maven
# version (1.7.10-10.13.4.1614-1.7.10, 1.10-12.18.0.2000-1.10.0).
INSTALLER_URL=""
for candidate in "${VERSION}" "${VERSION}-${MC_VERSION}" "${VERSION}-${MC_VERSION}.0"; do
    url="${MAVEN}/${candidate}/forge-${candidate}-installer.jar"
    if curl -fsIL --retry 3 -o /dev/null "${url}"; then
        INSTALLER_URL="${url}"
        VERSION="${candidate}"
        break
    fi
done
if [ -z "${INSTALLER_URL}" ]; then
    fail "Forge ${VERSION} has no installer download."
fi

echo "[Atlas] Installing Forge ${VERSION}"
download_jar "${INSTALLER_URL}" installer.jar

# The installer knows which Minecraft version it is for.
PROFILE_VERSION=$(unzip -p installer.jar install_profile.json 2> /dev/null | jq -r '.minecraft // .install.minecraft // empty' 2> /dev/null) || PROFILE_VERSION=""
if [ -n "${PROFILE_VERSION}" ]; then
    MC_VERSION="${PROFILE_VERSION}"
fi

atlas_install_java "${MC_VERSION}"

# Remove what an earlier install left behind so a version change is clean.
rm -rf libraries/net/minecraftforge/forge
rm -f unix_args.txt forge-*-shim.jar

echo "[Atlas] Running the Forge installer, this can take several minutes"
if ! "${JAVA_BIN}" -jar installer.jar --installServer; then
    rm -f installer.jar
    fail "the Forge installer failed, see its output above."
fi
rm -f installer.jar installer.jar.log

ARGS_FILE=$(find libraries/net/minecraftforge/forge -name unix_args.txt 2> /dev/null | sort | tail -n 1) || ARGS_FILE=""
if [ -n "${ARGS_FILE}" ]; then
    cp -f "${ARGS_FILE}" unix_args.txt
    echo "[Atlas] Forge ${VERSION} starts from unix_args.txt"
else
    FORGE_JAR=""
    for candidate in "forge-${VERSION}.jar" "forge-${VERSION}-universal.jar"; do
        if [ -f "${candidate}" ]; then
            FORGE_JAR="${candidate}"
            break
        fi
    done
    if [ -z "${FORGE_JAR}" ]; then
        FORGE_JAR=$(find . -maxdepth 1 -name 'forge-*.jar' | sort | tail -n 1) || FORGE_JAR=""
    fi
    if [ -z "${FORGE_JAR}" ]; then
        fail "the Forge installer did not leave a server jar behind."
    fi
    mv -f "${FORGE_JAR}" "${SERVER_JARFILE}"
    echo "[Atlas] Forge ${VERSION} starts from ${SERVER_JARFILE}"
fi

echo "eula=true" > eula.txt
echo "[Atlas] Finalizing installation"
