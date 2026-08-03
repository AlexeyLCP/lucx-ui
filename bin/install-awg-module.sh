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
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

echo -e "${GREEN}=== Установка модуля ядра AmneziaWG ===${NC}"

# --force-rebuild: tear down the loaded/DKMS-installed module and rebuild from
# a fresh git clone. Used by update.sh when the version gate detects that the
# running module is older than upstream master (e.g. AWG1 → AWG3, lucx.50).
# Without this flag the script keeps the early-exit no-op: module already
# loaded → nothing to do. Call: `bash install-awg-module.sh --force-rebuild`.
FORCE_REBUILD=0
[[ "${1:-}" == "--force-rebuild" ]] && FORCE_REBUILD=1

[[ $EUID -ne 0 ]] && { echo -e "${RED}Запустите с правами root${NC}"; exit 1; }

# Marker file written after a successful DKMS install: the upstream commit
# SHA the module was built from. update.sh compares it against `git ls-remote
# refs/heads/master` to decide whether a rebuild is due — a version string
# cannot work because upstream stamps PACKAGE_VERSION="1.0.0" into every
# module build (v1 and v3 alike).
AWG_MODULE_MARKER="/etc/x-ui/.awg-module-version"

# Detect OS
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    OS_ID=$ID
else
    echo -e "${RED}Не удалось определить ОС (/etc/os-release отсутствует)${NC}"
    exit 1
fi

# Upgrade the kernel to the latest packaged version BEFORE the early-exit:
# update.sh calls this script on every panel update (even when the module
# tree is current and the build below no-ops), and the kernel must advance
# on every one of those calls. The meta-packages pull the newest image +
# headers (a no-op when already current). update.sh reboots into the new
# kernel at the END of `x-ui update`; the DKMS build compiles the module for
# it too, so the reboot lands on a host whose amneziawg is already rebuilt.
# Never fatal: the panel keeps running on the booted kernel.
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

# Check if already loaded. Skip the early-exit when --force-rebuild is set.
if [[ -d /sys/module/amneziawg && $FORCE_REBUILD -eq 0 ]]; then
    echo -e "${GREEN}Модуль amneziawg уже загружен.${NC}"
    command -v awg &>/dev/null && { echo -e "${GREEN}awg уже установлен.${NC}"; exit 0; }
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
apt-get install -y -q \
    build-essential dkms git unzip curl \
    libmnl-dev pkg-config \
    python3 net-tools qrencode bc ca-certificates gnupg \
    2>/dev/null || true

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

# Если headers всё ещё нет — возможно ядро обновилось но не загружено
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    # Проверим — есть ли headers для НОВОГО ядра (не загруженного)
    NEWEST_HEADERS=$(ls -d /lib/modules/*/build 2>/dev/null | head -1)
    if [[ -n "$NEWEST_HEADERS" ]]; then
        NEWEST_KERNEL=$(basename $(dirname "$NEWEST_HEADERS"))
        echo -e "${YELLOW}┌──────────────────────────────────────────────────────────┐${NC}"
        echo -e "${YELLOW}│ Headers найдены для ${NEWEST_KERNEL}, но загружено ${RUNNING_KERNEL}        │${NC}"
        echo -e "${YELLOW}│ Ядро обновилось но не загружено. Нужен REBOOT.           │${NC}"
        echo -e "${YELLOW}│ После reboot запустите этот скрипт снова.                │${NC}"
        echo -e "${YELLOW}└──────────────────────────────────────────────────────────┘${NC}"
        echo -e "${GREEN}Выполняю reboot...${NC}"
        sleep 3
        reboot
        exit 0
    fi
    echo -e "${RED}Заголовки ядра для ${RUNNING_KERNEL} не найдены.${NC}"
    echo -e "${YELLOW}Попробуй: apt-get install linux-headers-${RUNNING_KERNEL}${NC}"
    echo -e "${YELLOW}Или обнови ядро: apt-get install linux-image-amd64 && reboot${NC}"
    exit 1
fi
echo -e "${GREEN}Заголовки ядра: OK${NC}"

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
if [[ ! -d /sys/module/amneziawg || $FORCE_REBUILD -eq 1 ]]; then
    echo -e "${GREEN}Сборка модуля ядра из исходников...${NC}"
    KERNEL_MOD_DIR="/tmp/amneziawg-kmod-$$"
    rm -rf "$KERNEL_MOD_DIR"
    git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git "$KERNEL_MOD_DIR"
    cd "$KERNEL_MOD_DIR/src"

    MOD_VER=$(git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
    MOD_SHA=$(git rev-parse HEAD 2>/dev/null || echo "")
    OLD_DKMS_VER=$(dkms status amneziawg 2>/dev/null | grep -oP 'amneziawg, \K[^,]+(?=,)' | head -1 || true)

    # Stage the sources under the real version and compile for the booted kernel.
    sed -i "s/^PACKAGE_VERSION=.*/PACKAGE_VERSION=\"${MOD_VER}\"/" dkms.conf
    rm -rf "/usr/src/amneziawg-${MOD_VER}"
    make dkms-install WIREGUARD_VERSION="${MOD_VER}" 2>/dev/null || true
    dkms add -m amneziawg -v "${MOD_VER}" 2>/dev/null || true
    dkms build -m amneziawg -v "${MOD_VER}" || {
        echo -e "${RED}Ошибка сборки DKMS — текущий модуль не тронут. Проверь заголовки ядра.${NC}"
        cd /tmp; rm -rf "$KERNEL_MOD_DIR"
        exit 1
    }

    # Build succeeded → swap: unload the old module and retire its DKMS tree,
    # then install the new one for the booted kernel...
    if [[ -n "$OLD_DKMS_VER" && "$OLD_DKMS_VER" != "$MOD_VER" ]]; then
        rmmod amneziawg 2>/dev/null || \
            echo -e "${YELLOW}Не удалось выгрузить amneziawg (занят?). Перезагрузка подберёт новый модуль.${NC}"
        dkms remove -m amneziawg -v "$OLD_DKMS_VER" --all 2>/dev/null || true
    fi
    dkms install -m amneziawg -v "${MOD_VER}" || {
        echo -e "${RED}dkms install не удалась — проверь /var/lib/dkms/amneziawg${NC}"
        cd /tmp; rm -rf "$KERNEL_MOD_DIR"
        exit 1
    }
    # ...and compile for every other installed kernel that has headers, so a
    # reboot into a freshly upgraded kernel boots straight onto the new module
    # (the kernel package's dkms autoinstall may have built the OLD tree for
    # it before this rebuild ran).
    for KERN in $(ls -1 /lib/modules 2>/dev/null); do
        [[ "$KERN" == "$(uname -r)" ]] && continue
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
#    Rebuild not only when missing but also when the installed tools predate
#    AWG3: tools < v3 do not parse the HeaderProtectionKey .conf line and
#    abort awg-quick with "Line unrecognized", so an AWG3 module behind v1/v2
#    tools still cannot serve HPK configs. `awg version` prints
#    "amneziawg-tools v3.0.20260730 - https://amnezia.org"; treat anything
#    under major 3 (or unparsable) as stale.
TOOLS_STALE=0
if ! command -v awg-quick &>/dev/null; then
    TOOLS_STALE=1
else
    TOOLS_MAJOR=$(awg version 2>/dev/null | grep -oP 'v\K[0-9]+' | head -1 || true)
    [[ "${TOOLS_MAJOR:-0}" -ge 3 ]] || TOOLS_STALE=1
fi
if [[ $TOOLS_STALE -eq 1 ]]; then
    echo -e "${GREEN}Сборка утилит awg...${NC}"
    TOOLS_DIR="/tmp/amneziawg-tools-$$"
    rm -rf "$TOOLS_DIR"
    if git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-tools.git "$TOOLS_DIR" 2>&1; then
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

# 5. Load module and enable autostart
modprobe amneziawg 2>/dev/null || {
    echo -e "${YELLOW}Не удалось загрузить модоль. Возможно, нужен ребут.${NC}"
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
