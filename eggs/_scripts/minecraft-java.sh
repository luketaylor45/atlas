#!/bin/bash
# Shared Java helper for Atlas Minecraft eggs whose installer is a Java
# program (Forge, NeoForge).
#
# Runs in ghcr.io/ptero-eggs/installers:debian as root. Eggs list this file
# first in "script_install_files" and follow it with their own script, so it
# only defines functions and must not exit on success.
#
# The installer image has no Java, and no single Java runs every installer:
# old Forge installers need Java 8, current ones need the Java their
# Minecraft version runs on. atlas_install_java downloads a matching Temurin
# runtime into /tmp, which is thrown away with the install container.
#
#   atlas_install_java <minecraft version>
#       Sets JAVA_BIN to a java binary suited to that Minecraft version.
set -euo pipefail

ATLAS_JAVA_DIR=/tmp/atlas-java
JAVA_BIN=""

# Prints the Java major version a Minecraft version runs on.
atlas_java_for_minecraft() {
    local minecraft="$1" manifest url major=""

    # Mojang publishes the Java version each Minecraft version needs.
    if manifest=$(curl -fsSL --retry 3 "https://launchermeta.mojang.com/mc/game/version_manifest.json" 2> /dev/null); then
        url=$(echo "${manifest}" | jq -r --arg v "${minecraft}" '.versions[] | select(.id == $v) | .url' 2> /dev/null) || url=""
        if [ -n "${url}" ] && [ "${url}" != "null" ]; then
            major=$(curl -fsSL --retry 3 "${url}" 2> /dev/null | jq -r '.javaVersion.majorVersion // 8' 2> /dev/null) || major=""
        fi
    fi

    case "${major}" in
        '' | *[!0-9]*)
            # Mojang could not tell us; go by the version number.
            case "${minecraft}" in
                1.[0-9] | 1.[0-9].* | 1.1[0-6] | 1.1[0-6].*) major=8 ;;
                1.17 | 1.17.*) major=16 ;;
                1.18 | 1.18.* | 1.19 | 1.19.* | 1.20 | 1.20.[1-4]) major=17 ;;
                1.*) major=21 ;;
                *) major=25 ;;
            esac
            ;;
    esac
    echo "${major}"
}

atlas_install_java() {
    local minecraft="$1" major arch kind url archive=/tmp/atlas-java.tar.gz

    case "$(uname -m)" in
        x86_64 | amd64) arch=x64 ;;
        aarch64 | arm64) arch=aarch64 ;;
        *)
            echo "[Atlas] Error: no Java download for $(uname -m) hosts." >&2
            exit 1
            ;;
    esac

    major=$(atlas_java_for_minecraft "${minecraft}")
    echo "[Atlas] Minecraft ${minecraft} runs on Java ${major}; downloading it for the installer"

    # Some Java versions (16) are only published as a full JDK.
    rm -f "${archive}"
    for kind in jre jdk; do
        url="https://api.adoptium.net/v3/binary/latest/${major}/ga/linux/${arch}/${kind}/hotspot/normal/eclipse"
        if curl -fsL --retry 3 --retry-delay 2 -o "${archive}" "${url}"; then
            break
        fi
        rm -f "${archive}"
    done
    if [ ! -s "${archive}" ]; then
        echo "[Atlas] Error: could not download Java ${major} from adoptium.net." >&2
        exit 1
    fi

    rm -rf "${ATLAS_JAVA_DIR}"
    mkdir -p "${ATLAS_JAVA_DIR}"
    if ! tar -xzf "${archive}" -C "${ATLAS_JAVA_DIR}" --strip-components=1; then
        echo "[Atlas] Error: the Java ${major} download is not a valid archive." >&2
        exit 1
    fi
    rm -f "${archive}"

    JAVA_BIN="${ATLAS_JAVA_DIR}/bin/java"
    if ! "${JAVA_BIN}" -version > /dev/null 2>&1; then
        echo "[Atlas] Error: the downloaded Java ${major} does not run on this host." >&2
        exit 1
    fi
}
