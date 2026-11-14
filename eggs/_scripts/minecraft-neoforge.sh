#!/bin/bash
# NeoForge installation script for Atlas.
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root with the server
# directory mounted at /mnt/server. Listed after _scripts/minecraft-java.sh,
# which provides atlas_install_java.
#
# NeoForge starts from an argument file. The installer writes it under
# libraries/ and this script copies it to unix_args.txt, which the egg's
# startup command reads.
#
# Variables it reads:
#   MC_VERSION        Minecraft version, or "latest" (default)
#   NEOFORGE_VERSION  optional exact version such as 21.1.266; overrides
#                     MC_VERSION
set -euo pipefail

MAVEN="https://maven.neoforged.net/releases/net/neoforged"
VERSIONS_API="https://maven.neoforged.net/api/maven/versions/releases/net/neoforged"
cd /mnt/server

MC_VERSION="$(echo "${MC_VERSION:-latest}" | tr -d '[:space:]')"
NEOFORGE_VERSION="$(echo "${NEOFORGE_VERSION:-}" | tr -d '[:space:]')"
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

# Reads version numbers on stdin and prints the newest one that starts with
# $1. With $2 = "releases", versions with a suffix (-beta, -alpha...) are
# skipped.
newest_version() {
    awk -v prefix="$1" -v only="$2" '
        prefix == "" || index($0, prefix) == 1 {
            rest = substr($0, length(prefix) + 1)
            if (only != "releases" || index(rest, "-") == 0) print
        }' | sort -V | tail -n 1
}

# NeoForge for Minecraft 1.20.1 is published as "forge" and numbered like
# Forge (1.20.1-47.1.106). Everything newer is "neoforge", numbered after the
# Minecraft version: 1.21.1 -> 21.1.x, 1.21 -> 21.0.x, 26.3 -> 26.3.0.x,
# 26.1.2 -> 26.1.2.x.
ARTIFACT=neoforge
if [ -n "${NEOFORGE_VERSION}" ]; then
    case "${NEOFORGE_VERSION}" in
        1.20.1-*) ARTIFACT=forge ;;
    esac
else
    case "${MC_VERSION}" in
        latest) PREFIX="" ;;
        1.20.1)
            ARTIFACT=forge
            PREFIX="1.20.1-"
            ;;
        1.*.*) PREFIX="${MC_VERSION#1.}." ;;
        1.*) PREFIX="${MC_VERSION#1.}.0." ;;
        *.*.*) PREFIX="${MC_VERSION}." ;;
        *.*) PREFIX="${MC_VERSION}.0." ;;
        *) fail "${MC_VERSION} is not a Minecraft version." ;;
    esac

    echo "[Atlas] Looking up NeoForge versions"
    LISTING=$(curl -fsSL --retry 3 "${VERSIONS_API}/${ARTIFACT}") || fail "could not reach the NeoForge Maven."
    VERSIONS=$(echo "${LISTING}" | jq -r '.versions[]') || VERSIONS=""
    if [ -z "${VERSIONS}" ]; then
        fail "the NeoForge Maven returned no versions."
    fi

    NEOFORGE_VERSION=$(echo "${VERSIONS}" | newest_version "${PREFIX}" releases)
    if [ -z "${NEOFORGE_VERSION}" ]; then
        NEOFORGE_VERSION=$(echo "${VERSIONS}" | newest_version "${PREFIX}" all)
        if [ -n "${NEOFORGE_VERSION}" ]; then
            echo "[Atlas] NeoForge has no stable build for Minecraft ${MC_VERSION}; using ${NEOFORGE_VERSION}"
        fi
    fi
    if [ -z "${NEOFORGE_VERSION}" ]; then
        fail "NeoForge has no build for Minecraft ${MC_VERSION}."
    fi
fi

echo "[Atlas] Installing NeoForge ${NEOFORGE_VERSION}"
download_jar "${MAVEN}/${ARTIFACT}/${NEOFORGE_VERSION}/${ARTIFACT}-${NEOFORGE_VERSION}-installer.jar" installer.jar

# The installer knows which Minecraft version it is for.
PROFILE_VERSION=$(unzip -p installer.jar install_profile.json 2> /dev/null | jq -r '.minecraft // empty' 2> /dev/null) || PROFILE_VERSION=""
if [ -z "${PROFILE_VERSION}" ]; then
    rm -f installer.jar
    fail "the NeoForge installer does not say which Minecraft version it is for."
fi

atlas_install_java "${PROFILE_VERSION}"

# Remove what an earlier install left behind so a version change is clean.
rm -rf libraries/net/neoforged/neoforge libraries/net/neoforged/forge
rm -f unix_args.txt

echo "[Atlas] Running the NeoForge installer, this can take several minutes"
if ! "${JAVA_BIN}" -jar installer.jar --installServer; then
    rm -f installer.jar
    fail "the NeoForge installer failed, see its output above."
fi
rm -f installer.jar installer.jar.log

ARGS_FILE=$(find "libraries/net/neoforged/${ARTIFACT}" -name unix_args.txt 2> /dev/null | sort | tail -n 1) || ARGS_FILE=""
if [ -z "${ARGS_FILE}" ]; then
    fail "the NeoForge installer did not write unix_args.txt."
fi
cp -f "${ARGS_FILE}" unix_args.txt

echo "eula=true" > eula.txt
echo "[Atlas] Finalizing installation"
