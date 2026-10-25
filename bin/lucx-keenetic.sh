#!/opt/bin/bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Derived from own-open-source/x-ui-keenetic (GPL-3.0), which is based on
# MHSanaei/3x-ui. Keenetic / Netcraze Entware wrapper for LucX-UI.
#
# Installs the LucX-UI panel (GitHub arm64/amd64 tarball) under Entware, with
# an Entware init.d autostart. The AmneziaWG kernel module (DKMS) is never
# built — Keenetic firmware is closed and has no kernel headers. AWG inbounds
# automatically run on the embedded userspace amneziawgnet stack instead, and
# wgobfs (Phobos) presets front the stock KeeneticOS kernel WireGuard.

NAME_APP=xui
PRIORITY="S99"
SHELL_SHORT_NAME="x-ui"
SHELL_FULL_NAME="$SHELL_SHORT_NAME.sh"

DIR_ENTWARE='/opt'
DIR_TMP=$DIR_ENTWARE'/tmp'
DIR_INSTALL_LNK=$DIR_ENTWARE'/bin/'$SHELL_SHORT_NAME
DIR_MAIN=$DIR_ENTWARE'/etc'
DIR_AUTO_START=$DIR_MAIN'/init.d'
DIR_PROJECT=$DIR_MAIN'/'$SHELL_SHORT_NAME
DIR_DB="$DIR_PROJECT/db"
DIR_LOGFILE="$DIR_ENTWARE/var/log/script_$SHELL_SHORT_NAME.log"
DIR_PROJECT_SHELL_SCRIPTS="$DIR_PROJECT/$SHELL_FULL_NAME"
DIR_CERT="$DIR_PROJECT/cert"
DIR_CONFIG="$DIR_PROJECT/config"

DB="$DIR_DB/$SHELL_SHORT_NAME.db"
ARCHITECTURE=""
LUCX_REPO="AlexeyLCP/lucx-ui"
# Pin a fixed tag with LUCX_KEENETIC_TAG=v3.10.2-lucx.314, otherwise latest.
LUCX_TAG="${LUCX_KEENETIC_TAG:-}"

export ZONEINFO=/opt/share/zoneinfo
export PATH=/opt/sbin:/opt/bin:/opt/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export XUI_BIN_FOLDER="$DIR_PROJECT/bin"
export XUI_DB_FOLDER="$DIR_DB"
export XUI_LOG_FOLDER="$DIR_ENTWARE/var/log/$SHELL_SHORT_NAME"
export XUI_LOG_LEVEL="${XUI_LOG_LEVEL:-info}"
export XUI_DEBUG="${XUI_DEBUG:-false}"

logmsg() {
    echo "$1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$DIR_LOGFILE" 2>/dev/null
}

check_os() {
    [[ "$(command -v opkg)" == "/opt/bin/opkg" ]] || {
        logmsg "ERR: this script is for Keenetic / Netcraze with Entware (/opt)."; exit 1
    }
}

check_architecture() {
    case "$(uname -m)" in
        x86_64 | amd64) ARCHITECTURE='amd64' ;;
        aarch64 | armv8* | arm64) ARCHITECTURE='arm64' ;;
        *)
            logmsg "ERR: LucX-UI releases ship amd64 and arm64 only; this router is $(uname -m) (most Keenetic models are mips/armv7 — not supported)."
            exit 1
            ;;
    esac
}

install_base() {
    check_architecture
    logmsg "Installing Entware dependencies..."
    opkg update && opkg install curl tar wget-ssl ca-bundle ca-certificates \
        sqlite3-cli jq openssl-util libopenssl ip-full wireguard-tools iptables
    mkdir -p "$DIR_DB" "$DIR_CONFIG" "$DIR_CERT" "$DIR_ENTWARE/tmp/tar" \
        "$XUI_LOG_FOLDER" "$DIR_ENTWARE/var/log"
}

latest_tag() {
    if [[ -n "$LUCX_TAG" ]]; then
        echo "$LUCX_TAG"; return
    fi
    curl -4 -Ls "https://api.github.com/repos/${LUCX_REPO}/releases/latest" \
        | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

download_panel() {
    local tag=$1
    local url="https://github.com/${LUCX_REPO}/releases/download/${tag}/x-ui-linux-${ARCHITECTURE}.tar.gz"
    logmsg "Downloading LucX-UI ${tag} (${ARCHITECTURE})..."
    curl -fL --retry 2 -o "$DIR_TMP/x-ui-lucx.tar.gz" "$url" || return 1
    if command -v sha256sum >/dev/null 2>&1; then
        if curl -fL --retry 2 -s -o "$DIR_TMP/x-ui-lucx.tar.gz.sha256" \
            "https://github.com/${LUCX_REPO}/releases/download/${tag}/x-ui-linux-${ARCHITECTURE}.tar.gz.sha256"; then
            local got
            got=$(sha256sum "$DIR_TMP/x-ui-lucx.tar.gz" | awk '{print $1}')
            if [[ -s "$DIR_TMP/x-ui-lucx.tar.gz.sha256" ]] && grep -q "^$got" "$DIR_TMP/x-ui-lucx.tar.gz.sha256"; then
                logmsg "Checksum OK."
            else
                logmsg "ERR: checksum mismatch."
                rm -f "$DIR_TMP/x-ui-lucx.tar.gz"
                return 1
            fi
        else
            logmsg "WARN: checksum file unavailable — skipping verification."
        fi
    fi
    return 0
}

install_geodata() {
    logmsg "Fetching Xray geodata..."
    cd "$XUI_BIN_FOLDER" || return
    curl -fL --retry 2 -o geoip.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
    curl -fL --retry 2 -o geosite.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
    echo 1 > "$DIR_PROJECT/.geo"
}

migration() {
    logmsg "Running DB migration..."
    "$DIR_PROJECT/$NAME_APP" migrate
}

exe_set_admin() {
    local user pass
    printf "Username [admin]: "; read -r user; user=${user:-admin}
    printf "Password [12345]: "; read -r pass; pass=${pass:-12345}
    "$DIR_PROJECT/$NAME_APP" setting -username "$user" -password "$pass"
    echo "Username: $user"; echo "Password: $pass"
}

exe_set_panel() {
    local port path lips
    printf "Panel port [2053]: "; read -r port; port=${port:-2053}
    printf "Panel path [/spub/]: "; read -r path; path=${path:-/spub/}
    printf "Listen IP [br0 IP]: "; read -r lips
    local params=""
    [[ -n "$port" ]] && params="$params -port $port"
    [[ -n "$path" ]] && params="$params -webBasePath $path"
    [[ -n "$lips" ]] && params="$params -listenIP $lips"
    logmsg "Applying panel settings..."
    "$DIR_PROJECT/$NAME_APP" setting $params
}

install() {
    local tag
    check_architecture
    install_base
    if [[ $(check_status) == "running" ]]; then
        stop 1
    fi
    tag=$(latest_tag)
    [[ -n "$tag" ]] || { logmsg "ERR: could not resolve the latest LucX-UI release tag."; return 1; }
    logmsg "Installing LucX-UI ${tag}..."
    download_panel "$tag" || { logmsg "ERR: download failed."; return 1; }
    tar zxf "$DIR_TMP/x-ui-lucx.tar.gz" -C "$DIR_TMP/tar" || { logmsg "ERR: unpack failed."; return 1; }
    mv "$DIR_TMP/tar/x-ui/x-ui" "$DIR_TMP/tar/x-ui/$NAME_APP"
    chmod 755 "$DIR_TMP/tar/x-ui/$NAME_APP"
    mkdir -p "$DIR_PROJECT"
    # Persist this wrapper itself so bin/x-ui + init.d symlinks survive.
    if [[ -f "$0" && "$(readlink -f "$0")" != "$DIR_PROJECT_SHELL_SCRIPTS" ]]; then
        cp -f "$0" "$DIR_PROJECT_SHELL_SCRIPTS"
    fi
    cp -rf "$DIR_TMP/tar/x-ui/"* "$DIR_PROJECT/"
    rm -rf "$DIR_TMP/tar" "$DIR_TMP/x-ui-lucx.tar.gz" "$DIR_TMP/x-ui-lucx.tar.gz.sha256"
    if [[ ! -d "$XUI_BIN_FOLDER" ]]; then
        logmsg "ERR: $XUI_BIN_FOLDER not found after unpack."; return 1
    fi
    if [[ ! -f "$DB" ]]; then
        exe_set_panel
        exe_set_admin
        install_geodata
    else
        migration
        [[ -f "$DIR_PROJECT/.geo" ]] && install_geodata
    fi
    chmod 755 "$DIR_PROJECT_SHELL_SCRIPTS"
    if [[ -f "$DB" ]]; then chmod 600 "$DB"; fi
    enable
    start
    logmsg "Install finished. Control: x-ui"
    view_panel
}

stop() {
    pkill -f "$DIR_PROJECT/$NAME_APP" 2>/dev/null && sleep 1
    logmsg "Panel stopped."
}

start() {
    if [[ $(check_status) == "running" ]]; then
        logmsg "Already running."; return 0
    fi
    nohup nice -n -19 "$DIR_PROJECT/$NAME_APP" run >> "$DIR_LOGFILE" 2>&1 &
    sleep 2
    if [[ $(check_status) == "running" ]]; then
        logmsg "Panel started."
    else
        logmsg "ERR: panel failed to start — see $DIR_LOGFILE and $XUI_LOG_FOLDER."
    fi
}

restart() { stop 1; start; }

check_status() {
    if [[ ! -f "$DIR_PROJECT/$NAME_APP" ]]; then
        echo "not-installed"; return
    fi
    if pgrep -f "$DIR_PROJECT/$NAME_APP" >/dev/null; then
        echo "running"
    else
        echo "stopped"
    fi
}

status() { echo "Panel: $(check_status)"; }

view_panel() {
    local server_ip existing_port existing_path
    server_ip=$(curl -4 -s --max-time 3 https://api.ipify.org)
    local show
    show=$("$DIR_PROJECT/$NAME_APP" setting -show)
    existing_port=$(echo "$show" | grep -Eo 'port: [0-9]+' | awk '{print $2}')
    existing_path=$(echo "$show" | grep -Eo 'webBasePath: [^ ]*' | awk '{print $2}')
    local localip
    localip="$(ip -4 addr show br0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1)"
    echo "URL (public):  http://${server_ip:-<WAN IP>}:${existing_port:-?}${existing_path:-/}"
    echo "URL (LAN br0): http://${localip:-<br0 IP>}:${existing_port:-?}${existing_path:-/}"
}

enable() {
    mkdir -p "$DIR_AUTO_START"
    ln -sf "$DIR_PROJECT_SHELL_SCRIPTS" "$DIR_AUTO_START/$PRIORITY$SHELL_SHORT_NAME"
    ln -sf "$DIR_PROJECT_SHELL_SCRIPTS" "$DIR_INSTALL_LNK"
    chmod +x "$DIR_PROJECT_SHELL_SCRIPTS"
    logmsg "Autostart enabled via $DIR_AUTO_START/$PRIORITY$SHELL_SHORT_NAME."
}

disable() {
    rm -f "$DIR_AUTO_START/$PRIORITY$SHELL_SHORT_NAME"
    logmsg "Autostart disabled."
}

update() {
    logmsg "Reinstalling from the latest release (DB is preserved)..."
    install
}

uninstall() {
    printf "Really uninstall the panel (DB at %s is kept)? Type YES: " "$DB"
    read -r answer
    if [[ "$answer" != "YES" ]]; then logmsg "Cancelled."; return; fi
    stop 1
    disable
    rm -f "$DIR_INSTALL_LNK"
    rm -rf "$DIR_PROJECT"
    logmsg "Uninstalled."
}

awg_note() {
    echo "AmneziaWG kernel module: not installed (Keenetic firmware is closed;"
    echo "no kernel headers or DKMS). AWG inbounds run on the embedded userspace"
    echo "amneziawgnet stack — heavier on CPU, no module build."
    echo "wgobfs (Phobos) presets front the standard KeeneticOS kernel WireGuard"
    echo "(wireguard-tools + ip-full are installed by this script)."
    echo "wgosocks inbounds are pure userspace."
}

menu() {
    echo "─────────────────────────────────────────"
    echo " LucX-UI Keenetic wrapper (Entware)"
    echo " Panel: $(check_status)"
    echo "─────────────────────────────────────────"
    echo " (1) Start panel"
    echo " (2) Stop panel"
    echo " (3) Restart panel"
    echo " (4) Status"
    echo " (5) Panel URL"
    echo " (6) Install / reinstall latest release"
    echo " (7) Enable autostart"
    echo " (8) Disable autostart"
    echo " (9) AWG / Phobos notes"
    echo "(10) Uninstall"
    echo "(11) Exit"
    printf "Choice: "
    read -r choice
    case $choice in
        1) start ;;
        2) stop ;;
        3) restart ;;
        4) status ;;
        5) view_panel ;;
        6) install ;;
        7) enable ;;
        8) disable ;;
        9) awg_note ;;
        10) uninstall ;;
        11) exit 0 ;;
        *) logmsg "Unknown choice: $choice" ;;
    esac
}

if [[ $# -eq 0 ]]; then
    check_os
    menu
    exit 0
fi
check_os
case "$1" in
    start) start ;;
    stop) stop ;;
    restart) restart ;;
    status) status ;;
    install) install ;;
    update) update ;;
    enable) enable ;;
    disable) disable ;;
    uninstall) uninstall ;;
    menu) menu ;;
    *)
        echo "Usage: x-ui [start|stop|restart|status|install|update|enable|disable|uninstall|menu]"
        ;;
esac