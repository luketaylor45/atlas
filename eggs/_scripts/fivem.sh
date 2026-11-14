#!/bin/bash
# FiveM (FXServer) installer for Atlas.
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root with the server
# directory mounted at /mnt/server.
#
# Variables it reads:
#   FIVEM_VERSION  "recommended" (default), "latest", or a full artifact id
#                  such as 7290-a654bcc2adfa27c4e020fc915a1a6343c3b4f921
set -euo pipefail

VERSIONS_API="https://changelogs-live.fivem.net/api/changelog/versions/linux/server"
ARTIFACTS="https://runtime.fivem.net/artifacts/fivem/build_proot_linux/master"
SERVER_DATA="https://github.com/citizenfx/cfx-server-data/archive/refs/heads/master.tar.gz"
UA="atlas-panel/1.0"
FIVEM_VERSION="${FIVEM_VERSION:-recommended}"

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
    *) fail "FXServer only runs on x86-64 hosts." ;;
esac

need_tools curl jq tar gzip xz:xz-utils
cd /mnt/server

case "${FIVEM_VERSION}" in
    recommended | latest)
        echo "[Atlas] Looking up the ${FIVEM_VERSION} FXServer artifact"
        VERSIONS=$(curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" "${VERSIONS_API}") ||
            fail "could not reach the FiveM changelog service."
        URL=$(echo "${VERSIONS}" | jq -r --arg k "${FIVEM_VERSION}_download" '.[$k] // empty' 2> /dev/null) || URL=""
        BUILD=$(echo "${VERSIONS}" | jq -r --arg k "${FIVEM_VERSION}" '.[$k] // empty' 2> /dev/null) || BUILD=""
        if ! [[ "${URL}" =~ ^https://runtime\.fivem\.net/.+/fx\.tar\.xz$ ]]; then
            fail "the FiveM changelog service did not list a ${FIVEM_VERSION} Linux artifact."
        fi
        ;;
    *)
        if ! [[ "${FIVEM_VERSION}" =~ ^[0-9]+-[0-9a-f]{40}$ ]]; then
            fail "'${FIVEM_VERSION}' is not an FXServer artifact. Use 'recommended', 'latest' or a full artifact id (build number, a dash and the 40 character hash)."
        fi
        URL="${ARTIFACTS}/${FIVEM_VERSION}/fx.tar.xz"
        BUILD="${FIVEM_VERSION%%-*}"
        ;;
esac

WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"' EXIT

echo "[Atlas] Downloading FXServer ${BUILD:-artifact} from ${URL}"
if ! curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" -o "${WORK}/fx.tar.xz" "${URL}"; then
    fail "the download failed. Check that artifact ${FIVEM_VERSION} exists."
fi
# An .xz file starts with the bytes FD 37 7A 58 5A 00.
if [ "$(head -c 6 "${WORK}/fx.tar.xz" | od -An -tx1 | tr -d ' \n')" != "fd377a585a00" ]; then
    fail "the downloaded file is not a .tar.xz archive."
fi

echo "[Atlas] Unpacking FXServer"
# The artifact is a self-contained Alpine tree; replace the old one whole.
rm -rf /mnt/server/alpine
tar -xJf "${WORK}/fx.tar.xz" --no-same-owner -C /mnt/server || fail "the archive could not be unpacked."
rm -f "${WORK}/fx.tar.xz"
if [ ! -f alpine/opt/cfx-server/FXServer ] || [ ! -f alpine/opt/cfx-server/ld-musl-x86_64.so.1 ]; then
    fail "the archive does not contain alpine/opt/cfx-server/FXServer."
fi

echo "[Atlas] Downloading the default server resources (cfx-server-data)"
if ! curl -fsSL --retry 3 --retry-delay 2 -A "${UA}" -o "${WORK}/server-data.tar.gz" "${SERVER_DATA}"; then
    fail "could not download cfx-server-data from GitHub."
fi
mkdir -p "${WORK}/server-data"
tar -xzf "${WORK}/server-data.tar.gz" --no-same-owner --strip-components=1 -C "${WORK}/server-data" ||
    fail "the cfx-server-data archive could not be unpacked."
if [ ! -d "${WORK}/server-data/resources/[system]" ]; then
    fail "the cfx-server-data archive does not contain the default resources."
fi
# Refreshes the stock resources; anything else in resources/ is left alone.
mkdir -p /mnt/server/resources
cp -Rf "${WORK}/server-data/resources/." /mnt/server/resources/

if [ -f server.cfg ]; then
    echo "[Atlas] Keeping the existing server.cfg"
else
    echo "[Atlas] Writing a default server.cfg"
    cat > server.cfg << 'CONFIG'
# FXServer configuration.
#
# Atlas rewrites the endpoint_add_tcp, endpoint_add_udp, sv_hostname and
# sv_maxclients lines from the server's settings on every start. The license
# key and the Steam Web API key are passed on the command line, so they do
# not belong in this file.

endpoint_add_tcp "0.0.0.0:30120"
endpoint_add_udp "0.0.0.0:30120"

# Resources that start with the server.
ensure mapmanager
ensure chat
ensure spawnmanager
ensure sessionmanager
ensure basic-gamemode
ensure hardcap
ensure rconlog

# Allow players to use ScriptHookV-based plugins (0 = no).
sv_scriptHookAllowed 0

# Remote console password; leave commented out to keep RCON disabled.
#set rcon_password ""

# Comma separated tags and the locale shown in the server list.
sets tags "default"
sets locale "en-US"

sv_hostname "My FXServer"
sets sv_projectName "My FXServer Project"
sets sv_projectDesc "A new FXServer"

# OneSync is required for more than 32 players and by most frameworks.
set onesync on

sv_maxclients 48

# Hide player endpoints in external log output.
sv_endpointprivacy true

# Server administrators, for example:
#add_ace group.admin command allow
#add_principal identifier.fivem:1 group.admin
CONFIG
fi

mkdir -p logs txData

echo "[Atlas] FXServer ${BUILD:-artifact} installed"
