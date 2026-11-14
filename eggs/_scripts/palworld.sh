#!/bin/bash
# Palworld post-install steps. Listed after _scripts/steamcmd.sh, which has
# already downloaded the dedicated server into /mnt/server.
#
# - checks that the server binary arrived
# - creates PalWorldSettings.ini from the shipped defaults (never overwrites)
# - writes .atlas/palworld.sh, the helper the startup command runs to apply
#   the panel's variables to PalWorldSettings.ini and to turn console input
#   into RCON / REST API calls (Palworld has no console of its own)
set -euo pipefail

cd /mnt/server

PAL_BINARY=Pal/Binaries/Linux/PalServer-Linux-Shipping
if [ ! -f "$PAL_BINARY" ]; then
    echo "[Atlas] Error: SteamCMD finished without installing ${PAL_BINARY}." >&2
    exit 1
fi
chmod u+x "$PAL_BINARY"

PAL_CONFIG=Pal/Saved/Config/LinuxServer/PalWorldSettings.ini
if grep -q '^OptionSettings=' "$PAL_CONFIG" 2>/dev/null; then
    echo "[Atlas] Keeping the existing PalWorldSettings.ini"
elif [ -f DefaultPalWorldSettings.ini ]; then
    echo "[Atlas] Creating PalWorldSettings.ini from the default settings"
    install -d -m 0770 "$(dirname "$PAL_CONFIG")"
    cp DefaultPalWorldSettings.ini "$PAL_CONFIG"
else
    echo "[Atlas] Warning: DefaultPalWorldSettings.ini is missing; the server will start with built-in settings."
fi

echo "[Atlas] Writing the Palworld start helper"
install -d -m 0770 .atlas
cat > .atlas/palworld.sh <<'ATLAS_HELPER'
#!/bin/bash
# Written by the Atlas Palworld egg when the server is installed; reinstalling
# replaces it. The startup command runs it from /home/container:
#
#   bash .atlas/palworld.sh config    apply the panel's variables to PalWorldSettings.ini
#   bash .atlas/palworld.sh console   forward console lines to the server
set -u

CONFIG=Pal/Saved/Config/LinuxServer/PalWorldSettings.ini
DEFAULTS=DefaultPalWorldSettings.ini
RCON_PORT=${RCON_PORT:-25575}
RESTAPI_PORT=${RESTAPI_PORT:-8212}

# The settings live on one line: OptionSettings=(Key=Value,Key="Text",...)
LINE=

set_option() {
    local key=$1 value=$2
    local pattern="^(.*[(,]${key}=)(\"[^\"]*\"|[^,)]*)(.*)\$"
    if [[ $LINE =~ $pattern ]]; then
        LINE="${BASH_REMATCH[1]}${value}${BASH_REMATCH[3]}"
    else
        LINE="${LINE%)},${key}=${value})"
    fi
}

# Text values are quoted; a quote inside one would end it early.
text() {
    printf '"%s"' "${1//\"/}"
}

configure() {
    if ! grep -q '^OptionSettings=' "$CONFIG" 2>/dev/null; then
        if [ ! -f "$DEFAULTS" ]; then
            echo "[Atlas] Warning: ${DEFAULTS} is missing, so the panel's settings cannot be applied."
            return 0
        fi
        echo "[Atlas] Creating PalWorldSettings.ini from the default settings"
        mkdir -p "$(dirname "$CONFIG")"
        cp "$DEFAULTS" "$CONFIG"
    fi

    LINE=$(grep -m1 '^OptionSettings=' "$CONFIG")
    LINE=${LINE%$'\r'}

    set_option ServerName "$(text "${SERVER_NAME:-Palworld Server}")"
    set_option ServerDescription "$(text "${SERVER_DESCRIPTION:-}")"
    set_option ServerPassword "$(text "${SERVER_PASSWORD:-}")"
    set_option AdminPassword "$(text "${ADMIN_PASSWORD:-}")"
    set_option ServerPlayerMaxNum "${MAX_PLAYERS:-32}"
    set_option PublicPort "${SERVER_PORT:-8211}"
    set_option PublicIP "$(text "${PUBLIC_IP:-}")"
    set_option RCONEnabled True
    set_option RCONPort "$RCON_PORT"
    set_option RESTAPIEnabled True
    set_option RESTAPIPort "$RESTAPI_PORT"

    local temporary="${CONFIG}.atlas"
    local current
    while IFS= read -r current || [ -n "$current" ]; do
        if [[ $current == OptionSettings=* ]]; then
            printf '%s\n' "$LINE"
        else
            printf '%s\n' "${current%$'\r'}"
        fi
    done < "$CONFIG" > "$temporary"
    mv -f "$temporary" "$CONFIG"
    echo "[Atlas] Applied the panel's settings to PalWorldSettings.ini"
}

port_open() {
    (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

rcon_command() {
    rcon -s -a "127.0.0.1:${RCON_PORT}" -p "${ADMIN_PASSWORD:-}" "$@"
}

rest_call() {
    curl -fsS -m 30 -o /dev/null -u "admin:${ADMIN_PASSWORD:-}" -X POST \
        -H 'Content-Type: application/json' -d "$2" \
        "http://127.0.0.1:${RESTAPI_PORT}/v1/api/$1"
}

stop_server() {
    echo "[Atlas] Saving the world and shutting Palworld down..."
    if port_open "$RESTAPI_PORT" && rest_call save '' \
        && rest_call shutdown '{"waittime":1,"message":"The server is shutting down"}'; then
        return 0
    fi
    if port_open "$RCON_PORT"; then
        rcon_command Save
        rcon_command "Shutdown 1"
        return 0
    fi
    echo "[Atlas] The server is not accepting commands yet; interrupting it instead."
    kill -INT 0
}

console() {
    local line
    while IFS= read -r line; do
        line=${line%$'\r'}
        case "$line" in
            '') ;;
            stop|quit|exit) stop_server ;;
            *)
                if port_open "$RCON_PORT"; then
                    rcon_command "$line"
                else
                    echo "[Atlas] The server is not accepting commands yet."
                fi
                ;;
        esac
    done
}

case "${1:-}" in
    config) configure ;;
    console) console ;;
    *) echo "Usage: bash .atlas/palworld.sh config|console" >&2; exit 2 ;;
esac
ATLAS_HELPER
chmod 0750 .atlas/palworld.sh

echo "[Atlas] Palworld is installed"
