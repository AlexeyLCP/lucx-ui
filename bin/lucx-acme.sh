#!/bin/bash
# Copyright (c) 2025 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

# =============================================================================
# LucX-UI: acme.sh HTTP-01 always listens on 127.0.0.1:<ACME port>, and :80
# is forwarded to it, so a LucX Caddy on :80 never blocks issue or renewal.
#
#   :80 held by a LucX Caddy  -> Caddy relays /.well-known/acme-challenge/*
#   :80 free                  -> the pre-hook starts a temporary socat :80
#   :80 held by anything else -> the pre-hook fails and names the owner
#
# The ACME port lives in ${XUI_DB_FOLDER:-/etc/x-ui}/acme-http-port (default
# 29380). The panel reads the same file when it renders the Caddy :80 block.
#
# Sourced by x-ui.sh / install.sh:
#   lucx_acme_prompt_port WebPort   asks for the port, fills LUCX_ACME_ARGS
# Executed:
#   lucx-acme.sh pre-hook | post-hook   registered on every standalone issue
#   lucx-acme.sh migrate                moves existing :80 certs to this scheme
#   lucx-acme.sh restore                back to plain :80 (panel uninstall)
#   lucx-acme.sh port                   prints the configured ACME port
# =============================================================================

# shellcheck disable=SC2034  # LUCX_ACME_ARGS and the WebPort nameref are read by the sourcing script
LUCX_ACME_DEFAULT_PORT=29380
LUCX_ACME_PORT_FILE="${XUI_DB_FOLDER:-/etc/x-ui}/acme-http-port"
LUCX_ACME_SELF="$(readlink -f "${BASH_SOURCE[0]}" 2> /dev/null || echo "${BASH_SOURCE[0]}")"
LUCX_ACME_BIN_DIR="$(dirname "${LUCX_ACME_SELF}")"
LUCX_ACME_PRE_HOOK="[ ! -f '${LUCX_ACME_SELF}' ] || bash '${LUCX_ACME_SELF}' pre-hook"
LUCX_ACME_POST_HOOK="[ ! -f '${LUCX_ACME_SELF}' ] || bash '${LUCX_ACME_SELF}' post-hook"
LUCX_ACME_FORWARD_TTL=900
LUCX_ACME_ARGS=()

# cron runs with a bare PATH; ss lives in /usr/sbin on RHEL.
case ":${PATH}:" in
    *:/usr/sbin:*) ;;
    *) PATH="${PATH}:/usr/sbin:/sbin" ;;
esac

lucx_acme_valid_port() {
    [[ "$1" =~ ^[0-9]+$ ]] && (($1 >= 1 && $1 <= 65535 && $1 != 80))
}

lucx_acme_port() {
    local p=""
    [[ -r "${LUCX_ACME_PORT_FILE}" ]] && p="$(tr -d '[:space:]' < "${LUCX_ACME_PORT_FILE}")"
    if lucx_acme_valid_port "$p"; then
        echo "$p"
    else
        echo "${LUCX_ACME_DEFAULT_PORT}"
    fi
}

lucx_acme_save_port() {
    mkdir -p "$(dirname "${LUCX_ACME_PORT_FILE}")" && echo "$1" > "${LUCX_ACME_PORT_FILE}"
}

# Name of the process listening on TCP $1 (default 80), "unknown" if hidden, empty if free.
lucx_acme_port_owner() {
    local port="${1:-80}" line=""
    if command -v ss > /dev/null 2>&1; then
        line=$(ss -ltnp 2> /dev/null | awk -v p=":${port}\$" '$4 ~ p {print; exit}')
        [[ -z "$line" ]] && return 0
        if [[ "$line" == *'users:(("'* ]]; then
            line="${line#*users:((\"}"
            echo "${line%%\"*}"
        else
            echo "unknown"
        fi
        return 0
    fi
    if command -v netstat > /dev/null 2>&1; then
        line=$(netstat -ltnp 2> /dev/null | awk -v p=":${port}\$" '$4 ~ p {print $NF; exit}')
        [[ -z "$line" ]] && return 0
        if [[ "$line" == */* ]]; then
            echo "${line#*/}"
        else
            echo "unknown"
        fi
    fi
}

# ss/netstat cut the name to 15 chars: caddy-naive-lin, caddy-layer4-li.
lucx_acme_is_lucx_caddy() {
    case "$1" in
        caddy-naive* | caddy-layer4*) return 0 ;;
    esac
    return 1
}

# Ports the running LucX Caddy relays challenges to, from its rendered Caddyfiles.
lucx_acme_caddy_relay_ports() {
    local f
    for f in "${LUCX_ACME_BIN_DIR}"/tunnel/*.caddyfile; do
        [[ -f "$f" ]] || continue
        awk '/handle \/\.well-known\/acme-challenge\/\*/ {getline; if (match($0, /127\.0\.0\.1:[0-9]+/)) print substr($0, RSTART + 10, RLENGTH - 10)}' "$f"
    done | sort -u
}

# Prints why :80 cannot reach acme.sh; empty when it is free or relayed by a LucX Caddy.
lucx_acme_port80_problem() {
    local owner
    owner="$(lucx_acme_port_owner 80)"
    [[ -z "$owner" ]] && return 0
    if ! lucx_acme_is_lucx_caddy "$owner"; then
        echo "port 80 is held by '${owner}', not by LucX-UI, so it cannot be forwarded to acme.sh"
    elif [[ -z "$(lucx_acme_caddy_relay_ports)" ]]; then
        echo "port 80 is held by a LucX Caddy (${owner}) without an ACME relay, e.g. a masking gateway listening on 80"
    fi
}

lucx_acme_explain() {
    echo "LucX-UI: acme.sh listens on 127.0.0.1:$1. Port 80 is forwarded to it automatically:"
    echo "  the Cover/naive Caddy relays /.well-known/acme-challenge/ while it holds :80,"
    echo "  otherwise a temporary socat forwards :80 only while a certificate is issued or renewed."
    echo "Let's Encrypt still connects to port 80 from outside, so keep TCP 80 open in the firewall."
}

# Port 80 is rejected: LucX forwards it, acme.sh needs its own loopback port.
lucx_acme_prompt_port() {
    local -n lucx_port_ref="$1"
    local current choice owner
    current="$(lucx_acme_port)"
    lucx_acme_explain "$current"
    while true; do
        if [[ "${NONINTERACTIVE:-}" == "1" ]]; then
            choice="${XUI_ACME_HTTP_PORT:-$current}"
            if ! lucx_acme_valid_port "$choice"; then
                echo "XUI_ACME_HTTP_PORT=${choice} is not usable (port 80 is forwarded by LucX); using ${current}."
                choice="$current"
            fi
        else
            read -rp "Loopback port for acme.sh (default ${current}, not 80): " choice
            choice="${choice// /}"
            choice="${choice:-$current}"
        fi
        if [[ "$choice" == "80" ]]; then
            echo "Port 80 is forwarded by LucX-UI itself. Choose another port for acme.sh."
            continue
        fi
        if ! lucx_acme_valid_port "$choice"; then
            echo "Invalid port: ${choice}"
            [[ "${NONINTERACTIVE:-}" == "1" ]] && return 1
            continue
        fi
        owner="$(lucx_acme_port_owner "$choice")"
        if [[ -n "$owner" ]]; then
            echo "Port ${choice} is already used by '${owner}'. Choose another port."
            [[ "${NONINTERACTIVE:-}" == "1" ]] && return 1
            continue
        fi
        break
    done
    if [[ "$choice" != "$current" ]] || [[ ! -f "${LUCX_ACME_PORT_FILE}" ]]; then
        lucx_acme_save_port "$choice" || return 1
        lucx_acme_migrate > /dev/null
        echo "ACME port saved: ${choice}. The panel moves the Caddy relay on its next reconcile."
    fi
    owner="$(lucx_acme_port80_problem)"
    if [[ -n "$owner" ]]; then
        echo "Cannot issue: ${owner}. Free port 80 and try again."
        return 1
    fi
    if ! command -v socat > /dev/null 2>&1; then
        echo "socat is required to forward port 80 to acme.sh. Install it and try again."
        return 1
    fi
    lucx_port_ref="$choice"
    LUCX_ACME_ARGS=(--local-address 127.0.0.1 --listen-v4 --pre-hook "${LUCX_ACME_PRE_HOOK}" --post-hook "${LUCX_ACME_POST_HOOK}")
}

lucx_acme_conf_get() {
    local v
    v=$(grep -m1 "^$2=" "$1" 2> /dev/null) || return 0
    v="${v#*=}"
    v="${v#\'}"
    echo "${v%\'}"
}

lucx_acme_conf_set() {
    local conf="$1" key="$2" value="$3" tmp
    tmp="$(mktemp "${conf}.XXXXXX")" || return 1
    grep -v "^${key}=" "$conf" > "$tmp"
    [[ -n "$value" ]] && echo "${key}='${value}'" >> "$tmp"
    cat "$tmp" > "$conf" && rm -f "$tmp"
}

lucx_acme_b64() {
    echo "__ACME_BASE64__START_$(printf '%s' "$1" | base64 | tr -d '\r\n')__ACME_BASE64__END_"
}

lucx_acme_hook_is_ours() {
    local v="$1"
    [[ "$v" == __ACME_BASE64__START_* ]] || return 1
    v="${v#__ACME_BASE64__START_}"
    v="${v%__ACME_BASE64__END_}"
    [[ "$(printf '%s' "$v" | base64 -d 2> /dev/null)" == *lucx-acme.sh* ]]
}

lucx_acme_home() {
    echo "${LE_WORKING_DIR:-${HOME:-/root}/.acme.sh}"
}

lucx_acme_domain_confs() {
    local home conf dir
    home="$(lucx_acme_home)"
    for conf in "$home"/*/*.conf; do
        [[ -f "$conf" ]] || continue
        dir="$(basename "$(dirname "$conf")")"
        [[ "${dir%_ecc}.conf" == "$(basename "$conf")" ]] || continue
        [[ "$(lucx_acme_conf_get "$conf" Le_Webroot)" == "no" ]] || continue
        echo "$conf"
    done
}

# Ours: carries our hooks. Adoptable: plain :80 standalone with no hooks of its own.
lucx_acme_migrate() {
    local port conf pre post http n=0
    if ! command -v socat > /dev/null 2>&1; then
        echo "LucX: socat not installed; standalone certificates keep binding port 80 directly."
        return 0
    fi
    port="$(lucx_acme_port)"
    while IFS= read -r conf; do
        pre="$(lucx_acme_conf_get "$conf" Le_PreHook)"
        post="$(lucx_acme_conf_get "$conf" Le_PostHook)"
        http="$(lucx_acme_conf_get "$conf" Le_HTTPPort)"
        if lucx_acme_hook_is_ours "$pre"; then
            [[ "$http" == "$port" ]] && [[ -n "$post" ]] && continue
        elif [[ -n "$pre" || -n "$post" ]] || [[ "${http:-80}" != "80" ]]; then
            continue
        fi
        lucx_acme_conf_set "$conf" Le_HTTPPort "$port"
        lucx_acme_conf_set "$conf" Le_LocalAddress 127.0.0.1
        lucx_acme_conf_set "$conf" Le_Listen_V4 1
        lucx_acme_conf_set "$conf" Le_Listen_V6 ""
        lucx_acme_conf_set "$conf" Le_PreHook "$(lucx_acme_b64 "${LUCX_ACME_PRE_HOOK}")"
        lucx_acme_conf_set "$conf" Le_PostHook "$(lucx_acme_b64 "${LUCX_ACME_POST_HOOK}")"
        n=$((n + 1))
    done < <(lucx_acme_domain_confs)
    [[ $n -gt 0 ]] && echo "LucX: ${n} standalone certificate(s) now renew through 127.0.0.1:${port}."
    return 0
}

lucx_acme_restore() {
    local conf n=0
    while IFS= read -r conf; do
        lucx_acme_hook_is_ours "$(lucx_acme_conf_get "$conf" Le_PreHook)" || continue
        lucx_acme_conf_set "$conf" Le_HTTPPort ""
        lucx_acme_conf_set "$conf" Le_LocalAddress ""
        lucx_acme_conf_set "$conf" Le_Listen_V4 ""
        lucx_acme_conf_set "$conf" Le_PreHook ""
        lucx_acme_conf_set "$conf" Le_PostHook ""
        if ! ip -4 addr show scope global 2> /dev/null | grep -q "inet "; then
            lucx_acme_conf_set "$conf" Le_Listen_V6 1
        fi
        n=$((n + 1))
    done < <(lucx_acme_domain_confs)
    [[ $n -gt 0 ]] && echo "LucX: ${n} certificate(s) renew on port 80 again."
    return 0
}

lucx_acme_forward() {
    local listen="$1" target="$2" pids="$3"
    nohup timeout "${LUCX_ACME_FORWARD_TTL}" socat "${listen},reuseaddr,fork" "TCP4:127.0.0.1:${target}" > /dev/null 2>&1 &
    echo "$!" >> "$pids"
}

lucx_acme_stop_forwards() {
    local pids="$1" pid
    [[ -f "$pids" ]] || return 0
    while read -r pid; do
        [[ "$pid" =~ ^[0-9]+$ ]] && kill "$pid" 2> /dev/null
    done < "$pids"
    rm -f "$pids"
}

# acme.sh runs hooks in DOMAIN_PATH; the script passes LUCX_ACME_LISTEN_PORT on issue,
# because Le_HTTPPort is only written to the conf after the pre-hook.
lucx_acme_pre_hook() {
    local domain="${Le_Domain:-}" pids="$PWD/.lucx-acme-forward.pids" port owner relay
    port="${LUCX_ACME_LISTEN_PORT:-}"
    [[ -z "$port" && -n "$domain" ]] && port="$(lucx_acme_conf_get "$PWD/${domain}.conf" Le_HTTPPort)"
    lucx_acme_stop_forwards "$pids"
    lucx_acme_valid_port "$port" || return 0
    if ! command -v socat > /dev/null 2>&1; then
        echo "LucX: socat is required to forward port 80 to acme.sh (127.0.0.1:${port})."
        return 1
    fi
    owner="$(lucx_acme_port_owner 80)"
    if [[ -z "$owner" ]]; then
        lucx_acme_forward "TCP4-LISTEN:80" "$port" "$pids"
        lucx_acme_forward "TCP6-LISTEN:80,ipv6only=1" "$port" "$pids"
        return 0
    fi
    relay="$(lucx_acme_port80_problem)"
    if [[ -n "$relay" ]]; then
        echo "LucX: ${relay}."
        return 1
    fi
    for relay in $(lucx_acme_caddy_relay_ports); do
        [[ "$relay" == "$port" ]] && continue
        [[ -z "$(lucx_acme_port_owner "$relay")" ]] || continue
        lucx_acme_forward "TCP4-LISTEN:${relay},bind=127.0.0.1" "$port" "$pids"
    done
    return 0
}

lucx_acme_post_hook() {
    lucx_acme_stop_forwards "$PWD/.lucx-acme-forward.pids"
    return 0
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    case "${1:-}" in
        pre-hook) lucx_acme_pre_hook ;;
        post-hook) lucx_acme_post_hook ;;
        migrate) lucx_acme_migrate ;;
        restore) lucx_acme_restore ;;
        port) lucx_acme_port ;;
        *)
            echo "usage: $0 pre-hook|post-hook|migrate|restore|port" >&2
            exit 64
            ;;
    esac
fi
