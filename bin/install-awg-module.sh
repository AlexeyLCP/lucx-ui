#!/bin/bash
# Copyright (c) 2025 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

set -e

# =============================================================================
# LucX-UI: Установка модуля ядра AmneziaWG (DKMS + update-initramfs)
#
# Универсальный скрипт — работает на Debian/Ubuntu/Armbian с любым ядром.
# Обходит проблему "linux-headers-$(uname -r) не найден" через fallback на
# meta-package + предложение reboot если ядро обновилось но не загружено.
# Подход перенят из pumbaX/awg-multi-script (do_install).
#
# Флаги:
#   --force-rebuild       пересобрать даже если SHA уже целевой / модуль загружен
#   --no-kernel-upgrade   не трогать linux-image meta-package (панель / Cores)
#   --uninstall           снять модуль + awg/awg-quick; .conf не трогает
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

# Panel / systemd-run has no TTY. apt + needrestart otherwise hang forever
# on "Restart services?" / conffile prompts — Cores → Rebuild then looks dead.
export DEBIAN_FRONTEND="${DEBIAN_FRONTEND:-noninteractive}"
export NEEDRESTART_MODE="${NEEDRESTART_MODE:-a}"
export NEEDRESTART_SUSPEND="${NEEDRESTART_SUSPEND:-1}"
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

FORCE_REBUILD=0
NO_KERNEL_UPGRADE=0
DO_UNINSTALL=0
for arg in "$@"; do
    case "$arg" in
        --force-rebuild) FORCE_REBUILD=1 ;;
        --no-kernel-upgrade) NO_KERNEL_UPGRADE=1 ;;
        --uninstall) DO_UNINSTALL=1 ;;
        -h|--help)
            echo "Usage: $0 [--force-rebuild] [--no-kernel-upgrade] [--uninstall]"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown argument: ${arg}${NC}"
            echo "Usage: $0 [--force-rebuild] [--no-kernel-upgrade] [--uninstall]"
            exit 1
            ;;
    esac
done

[[ $EUID -ne 0 ]] && { echo -e "${RED}Запустите с правами root${NC}"; exit 1; }

# Marker file written after a successful DKMS install: the commit SHA the
# module was built from. Compared against AWG_KMOD_PIN (not floating master) —
# unpinned master made every panel update a rebuild and could swap a v3
# netlink ABI under pre-v3 host tools. Bump the pins only after a stand test.
# PACKAGE_VERSION is always "1.0.0" (v1 and v3 alike), so a version string
# cannot discriminate builds.
AWG_MODULE_MARKER="/etc/x-ui/.awg-module-version"
AWG_KMOD_PIN="46803204e7ec3b068199cd671143bec661d3fe21"
AWG_TOOLS_PIN="ee0f0a9aa34ff0a0da4b3433b9512781cfe02843"

git_clone_sha() {
    local url="$1" sha="$2" dest="$3"
    rm -rf "$dest"
    mkdir -p "$dest"
    git -C "$dest" init -q
    git -C "$dest" remote add origin "$url"
    git -C "$dest" fetch --depth 1 origin "$sha" && git -C "$dest" checkout -q FETCH_HEAD
}

uninstall_awg_module() {
    echo -e "${YELLOW}=== Удаление модуля ядра AmneziaWG ===${NC}"
    local conf iface ver
    if command -v awg-quick >/dev/null 2>&1; then
        shopt -s nullglob
        for conf in /etc/amnezia/amneziawg/awg*.conf /etc/amnezia/amneziawg/awgo-*.conf; do
            if grep -qF "# Managed by x-ui - do not edit" "$conf" 2>/dev/null; then
                echo -e "${YELLOW}awg-quick down $(basename "$conf")${NC}"
                awg-quick down "$conf" >/dev/null 2>&1 || true
            fi
        done
        shopt -u nullglob
    fi
    if command -v ip >/dev/null 2>&1; then
        while read -r iface; do
            [[ -n "$iface" ]] || continue
            echo -e "${YELLOW}ip link delete ${iface}${NC}"
            ip link delete "$iface" >/dev/null 2>&1 || true
        done < <(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | cut -d'@' -f1 | grep -E '^(awg[0-9]+|awgo-[0-9]+)$' || true)
    fi
    rmmod amneziawg >/dev/null 2>&1 || true
    if command -v dkms >/dev/null 2>&1; then
        while read -r ver; do
            [[ -n "$ver" ]] || continue
            echo -e "${YELLOW}dkms remove amneziawg/${ver}${NC}"
            dkms remove -m amneziawg -v "$ver" --all >/dev/null 2>&1 || true
        done < <(dkms status amneziawg 2>/dev/null | grep -oP 'amneziawg[,/] ?\K[^,]+' | sort -u || true)
    fi
    rm -rf /usr/src/amneziawg-* /var/lib/dkms/amneziawg
    rm -f /usr/bin/awg /usr/bin/awg-quick \
        /usr/local/bin/awg /usr/local/bin/awg-quick \
        /usr/sbin/awg /usr/sbin/awg-quick \
        /usr/local/sbin/awg /usr/local/sbin/awg-quick
    rm -f /usr/share/man/man8/awg.8 /usr/share/man/man8/awg-quick.8 \
        /usr/local/share/man/man8/awg.8 /usr/local/share/man/man8/awg-quick.8
    rm -f /etc/modules-load.d/amneziawg.conf
    rm -f "$AWG_MODULE_MARKER" /etc/x-ui/.awg-reboot-needed
    rm -f /etc/sysctl.d/99-awg-performance.conf
    sysctl --system >/dev/null 2>&1 || true
    update-initramfs -u -k all >/dev/null 2>&1 || update-initramfs -u >/dev/null 2>&1 || true
    echo -e "${GREEN}=== AWG модуль удалён ===${NC}"
    echo -e "${YELLOW}Конфиги в /etc/amnezia/amneziawg/ не тронуты.${NC}"
    echo -e "${YELLOW}Вернуть: x-ui install-awg   или   bash $0${NC}"
}

if [[ $DO_UNINSTALL -eq 1 ]]; then
    uninstall_awg_module
    exit 0
fi

echo -e "${GREEN}=== Установка модуля ядра AmneziaWG ===${NC}"

# Detect OS
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    OS_ID=$ID
else
    echo -e "${RED}Не удалось определить ОС (/etc/os-release отсутствует)${NC}"
    exit 1
fi

# awg_tools_stale reports whether the installed awg tools predate AWG 3.1 and
# must be rebuilt. Tools < v3.1 do not parse RandomTrailers / DisableCookies
# and abort awg-quick with "Line unrecognized"; `awg version` prints
# "amneziawg-tools v3.1.20260812 - https://amnezia.org", so anything under 3.1
# (or unparsable, or awg-quick missing) is stale. v3.0 tools still parse HPK.
# Extracted so both the early-exit and the rebuild section share one rule.
awg_tools_stale() {
    if ! command -v awg-quick &>/dev/null; then
        return 0
    fi
    local TOOLS_MAJ TOOLS_MIN
    TOOLS_MAJ=$(awg version 2>/dev/null | grep -oP 'v\K[0-9]+' | head -1 || true)
    TOOLS_MIN=$(awg version 2>/dev/null | grep -oP 'v[0-9]+\.\K[0-9]+' | head -1 || true)
    if [[ "${TOOLS_MAJ:-0}" -lt 3 ]] || { [[ "${TOOLS_MAJ:-0}" -eq 3 ]] && [[ "${TOOLS_MIN:-0}" -lt 1 ]]; }; then
        return 0
    fi
    return 1
}

# Compat for Linux ≥ 7.1.5 (and distro backports): udp_tunnel_sock_release /
# setup_udp_tunnel_sock take struct sock * instead of struct socket *.
# Upstream PR amnezia-vpn/amneziawg-linux-kernel-module#218. No-op when
# master already has the wrappers — drop this after that merge.
apply_udp_tunnel_abi_compat() {
    local f="${1:-socket.c}"
    if grep -qF 'wg_udp_tunnel_sock_release' "$f" 2>/dev/null; then
        echo -e "${GREEN}udp_tunnel ABI wrappers already in tree — skip.${NC}"
        return 0
    fi
    if ! command -v python3 >/dev/null 2>&1; then
        echo -e "${YELLOW}python3 нет — патч udp_tunnel ABI пропущен (ядра 7.1.5+ не соберутся).${NC}"
        return 0
    fi
    echo -e "${YELLOW}Патч udp_tunnel ABI (ядро 7.1.5+, PR #218)...${NC}"
    python3 - "$f" <<'PY'
import sys
path = sys.argv[1]
text = open(path, encoding="utf-8", errors="surrogateescape").read()
repls = [
    ("udp_tunnel_sock_release(sock->sk_socket)",
     "wg_udp_tunnel_sock_release(sock, sock->sk_socket)"),
    ("setup_udp_tunnel_sock(net, new4, &cfg)",
     "wg_setup_udp_tunnel_sock(net, new4, &cfg)"),
    ("udp_tunnel_sock_release(new4)",
     "wg_udp_tunnel_sock_release(new4->sk, new4)"),
    ("setup_udp_tunnel_sock(net, new6, &cfg)",
     "wg_setup_udp_tunnel_sock(net, new6, &cfg)"),
]
new = text
for old, repl in repls:
    if old not in new:
        sys.stderr.write("udp_tunnel ABI: no match for %s\n" % old)
        sys.exit(1)
    new = new.replace(old, repl, 1)
needle = "static void sock_free(struct sock *sock)"
if needle not in new:
    sys.stderr.write("udp_tunnel ABI: sock_free not found\n")
    sys.exit(1)
wrapper = """\
/*
 * Linux 7.1.5+ (and some distro backports) changed
 * udp_tunnel_sock_release()/setup_udp_tunnel_sock() to take struct sock *
 * instead of struct socket *. Detect the real signature at compile time.
 * From amneziawg-linux-kernel-module PR #218. Remove once upstream merges.
 */
static inline void wg_udp_tunnel_sock_release(struct sock *sk, struct socket *sock)
{
	if (__builtin_types_compatible_p(typeof(&udp_tunnel_sock_release), void (*)(struct sock *)))
		((void (*)(struct sock *))udp_tunnel_sock_release)(sk);
	else
		((void (*)(struct socket *))udp_tunnel_sock_release)(sock);
}

static inline void wg_setup_udp_tunnel_sock(struct net *net, struct socket *sock,
					     struct udp_tunnel_sock_cfg *cfg)
{
	if (__builtin_types_compatible_p(typeof(&setup_udp_tunnel_sock),
					  void (*)(struct net *, struct sock *, struct udp_tunnel_sock_cfg *)))
		((void (*)(struct net *, struct sock *, struct udp_tunnel_sock_cfg *))setup_udp_tunnel_sock)(net, sock->sk, cfg);
	else
		((void (*)(struct net *, struct socket *, struct udp_tunnel_sock_cfg *))setup_udp_tunnel_sock)(net, sock, cfg);
}

"""
new = new.replace(needle, wrapper + needle, 1)
open(path, "w", encoding="utf-8", errors="surrogateescape").write(new)
PY
}

# Skip DKMS/kernel when the installed module SHA already matches AWG_KMOD_PIN
# (lucx.145/153). --force-rebuild (Cores / x-ui install-awg) bypasses.
# No network + module already present → do not force a reinstall.
AWG_NEED_MODULE=1
if [[ $FORCE_REBUILD -eq 0 ]]; then
    INSTALLED_AWG_SHA=""
    [[ -f "$AWG_MODULE_MARKER" ]] && INSTALLED_AWG_SHA=$(tr -d '[:space:]' < "$AWG_MODULE_MARKER" 2>/dev/null)
    if [[ -n "$INSTALLED_AWG_SHA" && "$INSTALLED_AWG_SHA" == "$AWG_KMOD_PIN" ]]; then
        AWG_NEED_MODULE=0
    fi
    if [[ $AWG_NEED_MODULE -eq 0 ]] && ! awg_tools_stale; then
        echo -e "${GREEN}Модуль amneziawg уже целевой (${INSTALLED_AWG_SHA:0:12}) — пропуск.${NC}"
        modprobe amneziawg 2>/dev/null || true
        exit 0
    fi
    if [[ $AWG_NEED_MODULE -eq 0 ]]; then
        echo -e "${YELLOW}Тулзы awg устарели (< v3.1) — обновляем (модуль не пересобираем).${NC}"
    fi
fi

# Kernel meta-packages only when a module rebuild is due. lucx.58 used to
# upgrade the kernel on every call (even a no-op); that forced apt + reboot
# on every panel update. --no-kernel-upgrade: Cores-button must not pull a
# newer linux-image. Never fatal: the panel keeps running on the booted kernel.
if [[ $NO_KERNEL_UPGRADE -eq 0 && $AWG_NEED_MODULE -eq 1 ]]; then
    case "$OS_ID" in
        ubuntu|debian|linuxmint|raspbian)
            apt-get update -qq || true
            apt-get install -y -q linux-image-amd64 linux-headers-amd64 2>/dev/null || \
            apt-get install -y -q linux-image-generic linux-headers-generic 2>/dev/null || \
            echo -e "${YELLOW}Не удалось обновить ядро (meta-package) — работаем на текущем${NC}"
            ;;
        *)
            echo -e "${YELLOW}Авто-апгрейд ядра для ${OS_ID} не поддерживается — пропущен${NC}"
            ;;
    esac
elif [[ $NO_KERNEL_UPGRADE -eq 1 ]]; then
    echo -e "${YELLOW}Авто-апгрейд ядра пропущен (--no-kernel-upgrade)${NC}"
fi

# 1. Install build dependencies
echo -e "${GREEN}Установка сборочных зависимостей...${NC}"
apt-get update -qq

# Core build tools + DKMS + git
# Core build tools + DKMS + git. List mirrors pumbaX/awg-multi-script so a
# fresh bare-metal install gets every package `make` for amneziawg-tools
# needs — libmnl-dev and pkg-config are NOT in build-essential and their
# absence is the most common reason awg-quick fails to build on a clean
# server (reported by tester VladufQa: "awg-quick пришлось через тулзу
# устанавливать зависимости"). qrencode, bc, net-tools and ca-certificates
# are utilities pumbaX ships; ufw is intentionally omitted — it is a
# firewall that can conflict with our iptables PostUp rules + fail2ban.
# CRITICAL deps go in their own apt call (lucx.70): apt is all-or-nothing per
# invocation, so one unavailable optional package used to abort the whole
# transaction and leave dkms missing → "dkms: command not found" at `dkms
# build` (line ~232). Optional utilities stay best-effort and never block.
apt-get install -y -q \
    build-essential dkms git libmnl-dev pkg-config python3 \
    2>/dev/null || true
apt-get install -y -q \
    unzip curl python3 net-tools qrencode bc ca-certificates gnupg \
    2>/dev/null || true

# Fail early with a clear message when the critical toolchain is genuinely
# absent (no network / no repo) instead of dying later at `dkms build` with a
# bare "dkms: command not found". If dkms/make/gcc already exist from a
# previous run we proceed even when apt itself failed.
if ! command -v dkms >/dev/null 2>&1 || ! command -v make >/dev/null 2>&1 || ! command -v gcc >/dev/null 2>&1; then
    echo -e "${RED}dkms/build-essential не установились (нет сети или репозитория).${NC}"
    echo -e "${RED}Вручную: apt-get update && apt-get install -y build-essential dkms libmnl-dev pkg-config${NC}"
    echo -e "${RED}затем:  bash bin/install-awg-module.sh${NC}"
    exit 1
fi

# openresolv — awg-quick вызывает resolvconf при наличии DNS= в .conf.
# Без него awg-quick up падает с "resolvconf: command not found".
apt-get install -y -q openresolv 2>/dev/null || echo -e "${YELLOW}openresolv не установлен — awg-quick может падать на DNS=${NC}"

# systemd-resolved — на Debian 13+ resolvconf symlink указывает на
# systemd-resolved backend, но сам сервис может быть не enabled. Тогда
# awg-quick up с DNS= падает с "Failed to set DNS configuration: Unit
# dbus-org.freedesktop.resolve1.service not found" → интерфейс откатывается
# → reconcile failed every 10s (поймано тестером VladufQa на awgo-3).
# Enable сервис если он установлен; если нет — openresolv выше уже покрыл.
if systemctl list-unit-files 2>/dev/null | grep -q systemd-resolved; then
    systemctl enable --now systemd-resolved 2>/dev/null && \
        echo -e "${GREEN}systemd-resolved enabled (resolvconf backend)${NC}" || true
fi

# iptables — PostUp панели ставит MASQUERADE/FORWARD через iptables.
# На Debian 13+ iptables отсутствует из коробки (только nftables), и
# awg-quick up падает с "iptables: command not found" (exit 127) — интерфейс
# вообще не поднимается. Пакет iptables ставит shim над nf_tables, наши
# правила работают через него прозрачно.
apt-get install -y -q iptables 2>/dev/null || echo -e "${YELLOW}iptables не установлен — kernel NAT (PostUp) будет падать${NC}"

# 1b. Network performance tuning — VPN tunnels (AWG + WireGuard) suffer from
# asymmetric upload throughput when the kernel TCP buffers are left at the
# Linux default (208 KB). A 100 Mbps residential uplink can only push ~1-2
# Mbps through an AWG tunnel with default rmem/wmem because the TCP send
# window never grows large enough to keep the pipe full under the per-packet
# crypto + obfuscation overhead. Raising the buffers to 64 MB lets TCP scale
# the window to the BDP and recover the full uplink rate (measured: upload
# went from 1 Mbps to 9 Mbps on a 100/10 Mbps link after this tuning).
# BBR + fq qdisc are also required (BBR for congestion control that doesn't
# back off on packet loss from obfuscation padding; fq for flow isolation).
# This mirrors the recommendation in amneziawg-go issue #112 and the
# WireGuard performance community consensus.
SYSCTL_FILE="/etc/sysctl.d/99-awg-performance.conf"
cat > "$SYSCTL_FILE" <<'SYSCTL'
# AWG / WireGuard performance tuning — large TCP buffers for VPN throughput.
# Default Linux rmem/wmem (208 KB) caps upload at ~1-2 Mbps through AWG.
# See amneziawg-go issue #112 and LucX-UI lucx.45.
net.core.rmem_max = 67108864
net.core.wmem_max = 67108864
net.core.rmem_default = 262144
net.core.wmem_default = 262144
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
# BBR congestion control + fq queue discipline — BBR doesn't back off on
# obfuscation-induced packet loss; fq isolates flows so one speedtest doesn't
# starve the AWG keepalive. These are no-ops if already set (idempotent).
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
SYSCTL
sysctl --system >/dev/null 2>&1 || sysctl -p "$SYSCTL_FILE" >/dev/null 2>&1 || true
echo -e "${GREEN}Network performance tuning applied (BBR + fq + 64MB TCP buffers)${NC}"

# 2. Install kernel headers — универсальная логика с fallback
RUNNING_KERNEL=$(uname -r)
echo -e "${GREEN}Ядро: ${RUNNING_KERNEL}${NC}"

# Сначала пробуем точный пакет headers для текущего ядра
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    echo -e "${GREEN}Установка linux-headers для ${RUNNING_KERNEL}...${NC}"
    apt-get install -y -q "linux-headers-${RUNNING_KERNEL}" 2>/dev/null || true
fi

# Если точный пакет не найден — fallback на meta-package
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    echo -e "${YELLOW}Точные headers не найдены, пробуем meta-package...${NC}"
    case "$OS_ID" in
        ubuntu|debian|linuxmint|raspbian)
            apt-get install -y -q linux-headers-amd64 2>/dev/null || \
            apt-get install -y -q linux-headers-generic 2>/dev/null || \
            apt-get install -y -q linux-headers-generic-hwe-22.04 2>/dev/null || true
            ;;
        armbian)
            apt-get install -y -q linux-headers-current-sunxi 2>/dev/null || \
            apt-get install -y -q linux-headers-current-rockchip 2>/dev/null || \
            apt-get install -y -q linux-headers-current-arm64 2>/dev/null || true
            ;;
        *)
            apt-get install -y -q linux-headers-amd64 2>/dev/null || \
            apt-get install -y -q linux-headers-generic 2>/dev/null || true
            ;;
    esac
fi

# If headers for the RUNNING kernel are still missing, DO NOT reboot here
# (lucx.122). Mid-script reboot aborted install.sh and forced a second run.
# Build for every kernel that already has headers; mark reboot-needed so
# install.sh/update.sh can finish the panel and reboot once at the end.
AWG_REBOOT_FLAG="/etc/x-ui/.awg-reboot-needed"
rm -f "$AWG_REBOOT_FLAG" 2>/dev/null || true
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    NEWEST_HEADERS=$(ls -d /lib/modules/*/build 2>/dev/null | head -1)
    if [[ -n "$NEWEST_HEADERS" ]]; then
        NEWEST_KERNEL=$(basename "$(dirname "$NEWEST_HEADERS")")
        echo -e "${YELLOW}Headers for running kernel ${RUNNING_KERNEL} missing; found ${NEWEST_KERNEL}.${NC}"
        echo -e "${YELLOW}Building AWG for installed kernels with headers; reboot deferred to end of install/update.${NC}"
        mkdir -p "$(dirname "$AWG_REBOOT_FLAG")"
        echo "${NEWEST_KERNEL}" > "$AWG_REBOOT_FLAG"
    else
        echo -e "${RED}Заголовки ядра для ${RUNNING_KERNEL} не найдены.${NC}"
        echo -e "${YELLOW}Попробуй: apt-get install linux-headers-${RUNNING_KERNEL}${NC}"
        echo -e "${YELLOW}Или обнови ядро: apt-get install linux-image-amd64 && reboot${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}Заголовки ядра: OK${NC}"
fi

# 3. Build and install kernel module via DKMS.
#    Version = the git tag/SHA of the exact commit built. Upstream hardcodes
#    PACKAGE_VERSION="1.0.0" / WIREGUARD_VERSION=1.0.0 in EVERY release (the
#    GitHub tags v1.0.20260611…v3.0.20260731 never reach the module build),
#    so the git-describe string stamped into DKMS here is the only record of
#    what the tree contains; the full commit SHA goes to the marker file for
#    update.sh's rebuild gate (a version string cannot discriminate v1 from
#    v3 — the module version literally starts with 1.0 on both).
#    Build-first-safe: the NEW tree is compiled while the running module is
#    still loaded; only a successful build swaps (unload old, retire the old
#    DKMS tree, install new). A failed build leaves the host on its existing
#    module — never module-less (the old rmmod-first order could strand a
#    host without amneziawg when the new build failed).
if [[ $AWG_NEED_MODULE -eq 1 ]]; then
    echo -e "${GREEN}Сборка модуля ядра из исходников (pin ${AWG_KMOD_PIN:0:12})...${NC}"
    KERNEL_MOD_DIR="/tmp/amneziawg-kmod-$$"
    git_clone_sha "https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git" "$AWG_KMOD_PIN" "$KERNEL_MOD_DIR" || {
        echo -e "${RED}Не удалось клонировать amneziawg-linux-kernel-module @ ${AWG_KMOD_PIN:0:12}${NC}"
        exit 1
    }
    cd "$KERNEL_MOD_DIR/src"

    MOD_VER=$(git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
    MOD_SHA=$(git rev-parse HEAD 2>/dev/null || echo "")
    OLD_DKMS_VER=$(dkms status amneziawg 2>/dev/null | grep -oP 'amneziawg, \K[^,]+(?=,)' | head -1 || true)

    apply_udp_tunnel_abi_compat socket.c || \
        echo -e "${YELLOW}Патч udp_tunnel ABI не применился — продолжаем (ядра 7.1.5+ могут не собраться).${NC}"

    # Stage the sources under the real version and compile for the booted kernel.
    sed -i "s/^PACKAGE_VERSION=.*/PACKAGE_VERSION=\"${MOD_VER}\"/" dkms.conf
    rm -rf "/usr/src/amneziawg-${MOD_VER}"
    make dkms-install WIREGUARD_VERSION="${MOD_VER}" 2>/dev/null || true
    dkms add -m amneziawg -v "${MOD_VER}" 2>/dev/null || true
    # Prefer the running kernel; if its headers are missing (meta-upgrade
    # already pulled a newer image), build for the first kernel that has
    # headers so install can finish without a mid-script reboot (lucx.122).
    BUILD_K="$(uname -r)"
    if [[ ! -d "/lib/modules/${BUILD_K}/build" ]]; then
        for KERN in $(ls -1 /lib/modules 2>/dev/null); do
            if [[ -d "/lib/modules/$KERN/build" ]]; then
                BUILD_K="$KERN"
                break
            fi
        done
    fi
    dkms build -m amneziawg -v "${MOD_VER}" -k "${BUILD_K}" || {
        echo -e "${RED}Ошибка сборки DKMS — текущий модуль не тронут.${NC}"
        mklog="/var/lib/dkms/amneziawg/${MOD_VER}/build/make.log"
        if [[ -f "$mklog" ]]; then
            echo -e "${YELLOW}--- ${mklog} (хвост) ---${NC}"
            tail -n 50 "$mklog" || true
        else
            echo -e "${YELLOW}make.log не найден. Заголовки: /lib/modules/${BUILD_K}/build${NC}"
        fi
        cd /tmp; rm -rf "$KERNEL_MOD_DIR"
        exit 1
    }

    # Build succeeded → swap: unload the old module and retire its DKMS tree,
    # then install the new one. Skip rmmod when a foreign (unmanaged) awgN is
    # up — those clients must stay online until the operator imports them.
    foreign_awg=0
    if command -v ip >/dev/null 2>&1; then
        while read -r iface; do
            [[ -n "$iface" ]] || continue
            conf="/etc/amnezia/amneziawg/${iface}.conf"
            if [[ -f "$conf" ]] && grep -qF "# Managed by x-ui - do not edit" "$conf" 2>/dev/null; then
                continue
            fi
            echo -e "${YELLOW}Чужой интерфейс ${iface} оставлен (не x-ui) — rmmod пропущен.${NC}"
            foreign_awg=1
            break
        done < <(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | cut -d'@' -f1 | grep -E '^awg[0-9]+$' || true)
    fi
    if [[ -n "$OLD_DKMS_VER" && "$OLD_DKMS_VER" != "$MOD_VER" ]]; then
        if [[ "$foreign_awg" -eq 1 ]]; then
            mkdir -p "$(dirname "$AWG_REBOOT_FLAG")"
            uname -r > "$AWG_REBOOT_FLAG" 2>/dev/null || true
            echo -e "${YELLOW}Новый модуль загрузится после reboot, текущие AWG-клиенты не сброшены.${NC}"
        elif ! rmmod amneziawg 2>/dev/null; then
            echo -e "${YELLOW}Не удалось выгрузить amneziawg (занят?) — новый модуль подхватится после перезагрузки.${NC}"
            mkdir -p "$(dirname "$AWG_REBOOT_FLAG")"
            uname -r > "$AWG_REBOOT_FLAG" 2>/dev/null || true
        fi
        if [[ "$foreign_awg" -eq 0 ]]; then
            dkms remove -m amneziawg -v "$OLD_DKMS_VER" --all 2>/dev/null || true
        fi
    fi
    dkms install -m amneziawg -v "${MOD_VER}" -k "${BUILD_K}" || {
        echo -e "${RED}dkms install не удалась — проверь /var/lib/dkms/amneziawg${NC}"
        cd /tmp; rm -rf "$KERNEL_MOD_DIR"
        exit 1
    }
    # Compile for every other installed kernel that has headers, so a reboot
    # into a freshly upgraded kernel boots straight onto the new module.
    for KERN in $(ls -1 /lib/modules 2>/dev/null); do
        [[ "$KERN" == "$BUILD_K" ]] && continue
        [[ -d "/lib/modules/$KERN/build" ]] || continue
        dkms build -m amneziawg -v "${MOD_VER}" -k "$KERN" 2>/dev/null && \
            dkms install -m amneziawg -v "${MOD_VER}" -k "$KERN" 2>/dev/null || \
            echo -e "${YELLOW}Модуль для ядра $KERN не собран — соберётся при следующем dkms autoinstall${NC}"
    done

    cd /tmp; rm -rf "$KERNEL_MOD_DIR"
    # Write the commit SHA marker so update.sh's rebuild gate compares like
    # with like (git ls-remote of upstream master).
    mkdir -p "$(dirname "$AWG_MODULE_MARKER")"
    echo "${MOD_SHA:-$MOD_VER}" > "$AWG_MODULE_MARKER"
    echo -e "${GREEN}Модуль ядра собран и установлен (${MOD_VER}).${NC}"
fi

# 4. Build and install userspace tools (awg + awg-quick, both from src/).
#    Rebuild not only when missing but also when they predate AWG 3.1 (tools
#    < v3.1 reject RandomTrailers / DisableCookies with "Line unrecognized").
#    See awg_tools_stale above for the version rule.
if awg_tools_stale; then
    echo -e "${GREEN}Сборка утилит awg (pin ${AWG_TOOLS_PIN:0:12})...${NC}"
    TOOLS_DIR="/tmp/amneziawg-tools-$$"
    if git_clone_sha "https://github.com/amnezia-vpn/amneziawg-tools.git" "$AWG_TOOLS_PIN" "$TOOLS_DIR"; then
        ( cd "$TOOLS_DIR/src" && make && make install ) \
            && echo -e "${GREEN}Утилиты awg установлены.${NC}" \
            || echo -e "${RED}Сборка утилит awg упала — проверь build-essential (apt install build-essential). AWG не стартует без awg-quick.${NC}"
        cd /tmp; rm -rf "$TOOLS_DIR"
    else
        echo -e "${RED}Не удалось клонировать amneziawg-tools (сеть/GitHub?). AWG не стартует без awg-quick.${NC}"
    fi
fi

# Sanity: both binaries must exist now — a silent miss here is how panels end up
# with a running kernel module but no awg-quick (reconcile fails every 10s).
if ! command -v awg-quick &>/dev/null; then
    echo -e "${RED}ВНИМАНИЕ: awg-quick не найден после установки. AWG-инбаунды не поднимутся.${NC}"
    echo -e "${RED}Дособрать вручную: apt install build-essential && cd /tmp && git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-tools.git && cd amneziawg-tools/src && make && make install${NC}"
fi

# 5. Load module and enable autostart (only if the running kernel has the module)
modprobe amneziawg 2>/dev/null || {
    if [[ -f "$AWG_REBOOT_FLAG" ]]; then
        echo -e "${YELLOW}Модуль собран для нового ядра — загрузится после reboot (в конце install/update).${NC}"
    else
        echo -e "${YELLOW}Не удалось загрузить модуль. Возможно, нужен ребут.${NC}"
        mkdir -p "$(dirname "$AWG_REBOOT_FLAG")"
        uname -r > "$AWG_REBOOT_FLAG" 2>/dev/null || true
    fi
}
echo "amneziawg" > /etc/modules-load.d/amneziawg.conf

# 6. Update initramfs (critical for reboot survival)
echo -e "${GREEN}Обновление initramfs...${NC}"
update-initramfs -u -k all 2>/dev/null || update-initramfs -u 2>/dev/null || {
    echo -e "${YELLOW}Предупреждение: update-initramfs не сработал. Модуль может не загрузиться после ребута.${NC}"
}

# 7. Secure Boot check
if [[ -d /sys/firmware/efi ]]; then
    if mokutil --sb-state 2>/dev/null | grep -q "SecureBoot enabled"; then
        echo -e "${YELLOW}┌──────────────────────────────────────────────────────┐${NC}"
        echo -e "${YELLOW}│ ОБНАРУЖЕН SECURE BOOT!                              │${NC}"
        echo -e "${YELLOW}│ Модуль amneziawg не подписан — может не загрузиться. │${NC}"
        echo -e "${YELLOW}│ Отключи Secure Boot в BIOS или подпиши модуль.       │${NC}"
        echo -e "${YELLOW}└──────────────────────────────────────────────────────┘${NC}"
    fi
fi

# 8. Verify
echo ""
if lsmod | grep -q amneziawg; then
    echo -e "${GREEN}✓ Модуль amneziawg загружен${NC}"
else
    echo -e "${YELLOW}⚠ Модуль не загружен — нужен ребут${NC}"
fi
command -v awg &>/dev/null && echo -e "${GREEN}✓ awg установлен ($(awg version 2>&1 | head -1))${NC}"
command -v awg-quick &>/dev/null && echo -e "${GREEN}✓ awg-quick установлен${NC}"
command -v resolvconf &>/dev/null && echo -e "${GREEN}✓ resolvconf (openresolv) установлен${NC}"
# Fallback marker write: if the build block was skipped (module already loaded
# from a prior install) the marker may still be absent on pre-lucx.51 systems.
# Backfill it from modinfo so update.sh's version gate works next time.
if [[ ! -f "$AWG_MODULE_MARKER" ]]; then
    MOD_VER_NOW=$(modinfo -F version amneziawg 2>/dev/null | head -1 || true)
    if [[ -n "$MOD_VER_NOW" ]]; then
        mkdir -p "$(dirname "$AWG_MODULE_MARKER")"
        echo "$MOD_VER_NOW" > "$AWG_MODULE_MARKER"
    fi
fi
echo -e "${GREEN}=== Установка AWG завершена ===${NC}"
