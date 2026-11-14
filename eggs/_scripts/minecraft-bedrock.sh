#!/bin/ash
# Bedrock Dedicated Server installation script for Atlas.
#
# Runs in ghcr.io/pterodactyl/installers:alpine as root with the server
# directory mounted at /mnt/server.
#
# Variables it reads:
#   BEDROCK_VERSION  version to install (for example 1.26.63.1), "latest"
#                    (default) or "preview"
set -eu

LINKS_API="https://net-secondary.web.minecraft-services.net/api/v1.0/download/links"
DOWNLOAD_BASE="https://www.minecraft.net/bedrockdedicatedserver/bin-linux"
# minecraft.net never answers curl's own user agent, so send a browser one.
UA="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
ARCHIVE="bedrock-server.zip"
# Files a reinstall or update must not overwrite.
KEEP="server.properties permissions.json allowlist.json"
cd /mnt/server

BEDROCK_VERSION="${BEDROCK_VERSION:-latest}"

fail() {
    echo "[Atlas] Error: $1" >&2
    exit 1
}

case "$(uname -m)" in
    x86_64 | amd64) ;;
    *) fail "the Bedrock Dedicated Server only runs on x86-64 hosts." ;;
esac

case "${BEDROCK_VERSION}" in
    latest | preview)
        if [ "${BEDROCK_VERSION}" = "preview" ]; then
            DOWNLOAD_TYPE="serverBedrockPreviewLinux"
        else
            DOWNLOAD_TYPE="serverBedrockLinux"
        fi
        echo "[Atlas] Looking up the ${BEDROCK_VERSION} Bedrock server download"
        LINKS=$(curl -fsSL --retry 3 --connect-timeout 30 --max-time 120 -A "${UA}" -H "Accept-Language: en" "${LINKS_API}") ||
            fail "could not reach the Minecraft download service."
        DOWNLOAD_URL=$(echo "${LINKS}" | jq -r --arg t "${DOWNLOAD_TYPE}" '.result.links[] | select(.downloadType == $t) | .downloadUrl') || DOWNLOAD_URL=""
        if [ -z "${DOWNLOAD_URL}" ] || [ "${DOWNLOAD_URL}" = "null" ]; then
            fail "the Minecraft download service did not list a ${BEDROCK_VERSION} Linux server."
        fi
        ;;
    *)
        DOWNLOAD_URL="${DOWNLOAD_BASE}/bedrock-server-${BEDROCK_VERSION}.zip"
        ;;
esac

echo "[Atlas] Downloading ${DOWNLOAD_URL}"
rm -f "${ARCHIVE}"
if ! curl -fSL --retry 3 --retry-delay 2 --connect-timeout 30 --max-time 3600 -A "${UA}" -H "Accept-Language: en" -o "${ARCHIVE}" "${DOWNLOAD_URL}"; then
    rm -f "${ARCHIVE}"
    fail "the download failed. Check that Bedrock version ${BEDROCK_VERSION} exists."
fi
if [ "$(head -c 2 "${ARCHIVE}")" != "PK" ]; then
    rm -f "${ARCHIVE}"
    fail "the downloaded file is not a valid zip archive."
fi

for file in ${KEEP}; do
    if [ -f "${file}" ]; then
        cp -f "${file}" "${file}.atlas-keep"
    fi
done
restore_kept() {
    for file in ${KEEP}; do
        if [ -f "${file}.atlas-keep" ]; then
            mv -f "${file}.atlas-keep" "${file}"
        fi
    done
}

echo "[Atlas] Unpacking the server files"
if ! unzip -o -q "${ARCHIVE}"; then
    restore_kept
    rm -f "${ARCHIVE}"
    fail "could not unpack the server archive."
fi
restore_kept
rm -f "${ARCHIVE}"

if [ ! -f bedrock_server ]; then
    fail "the archive did not contain bedrock_server."
fi
chmod +x bedrock_server

echo "[Atlas] Finalizing installation"
