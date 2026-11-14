#!/bin/bash
# Factorio headless server installer for Atlas.
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root with the server
# directory mounted at /mnt/server. The map is created by the startup command
# on first start, so the game is never run as root here.
#
# Variables it reads:
#   FACTORIO_VERSION  "latest" (default, newest stable), "experimental" or an
#                     exact version such as 2.0.72
set -euo pipefail

RELEASES_API="https://factorio.com/api/latest-releases"
UA="atlas-panel/1.0"
FACTORIO_VERSION="${FACTORIO_VERSION:-latest}"

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
    *) fail "the Factorio headless server only runs on x86-64 hosts." ;;
esac

need_tools curl jq tar xz:xz-utils
cd /mnt/server

case "${FACTORIO_VERSION}" in
    latest | stable | experimental)
        CHANNEL="${FACTORIO_VERSION}"
        if [ "${CHANNEL}" = "latest" ]; then
            CHANNEL="stable"
        fi
        echo "[Atlas] Looking up the ${CHANNEL} Factorio release"
        RELEASES=$(curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" "${RELEASES_API}") ||
            fail "could not reach factorio.com."
        VERSION=$(echo "${RELEASES}" | jq -r --arg c "${CHANNEL}" '.[$c].headless // empty' 2> /dev/null) || VERSION=""
        ;;
    *)
        VERSION="${FACTORIO_VERSION}"
        ;;
esac
if ! [[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "'${VERSION:-${FACTORIO_VERSION}}' is not a Factorio version. Use 'latest', 'experimental' or a version such as 2.0.72."
fi

WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"' EXIT

URL="https://factorio.com/get-download/${VERSION}/headless/linux64"
echo "[Atlas] Downloading Factorio ${VERSION} from ${URL}"
if ! curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" -o "${WORK}/factorio.tar.xz" "${URL}"; then
    fail "the download failed. Check that Factorio ${VERSION} has a headless Linux build."
fi
# An .xz file starts with the bytes FD 37 7A 58 5A 00.
if [ "$(head -c 6 "${WORK}/factorio.tar.xz" | od -An -tx1 | tr -d ' \n')" != "fd377a585a00" ]; then
    fail "the downloaded file is not a .tar.xz archive."
fi

echo "[Atlas] Unpacking Factorio ${VERSION}"
# Remove the old program files so an update does not leave stale ones behind.
# Saves, mods, config and the settings files in data/ are kept.
rm -rf bin data/base data/core data/elevated-rails data/quality data/space-age
tar -xJf "${WORK}/factorio.tar.xz" --no-same-owner --strip-components=1 -C /mnt/server || fail "the archive could not be unpacked."
if [ ! -x bin/x64/factorio ]; then
    fail "the archive does not contain bin/x64/factorio."
fi

for SETTINGS in server-settings map-gen-settings map-settings; do
    if [ -f "data/${SETTINGS}.json" ]; then
        echo "[Atlas] Keeping the existing data/${SETTINGS}.json"
    elif [ -f "data/${SETTINGS}.example.json" ]; then
        echo "[Atlas] Creating data/${SETTINGS}.json from the example"
        cp "data/${SETTINGS}.example.json" "data/${SETTINGS}.json"
    else
        fail "the archive does not contain data/${SETTINGS}.example.json."
    fi
done

mkdir -p saves mods

echo "[Atlas] Factorio ${VERSION} installed"
