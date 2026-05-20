#!/bin/bash
set -e
# =============================================================================
# LucX-UI: Восстановление всех AWG/Telemt интерфейсов после ребута
# Вызывается systemd (lucx-restore.service) или вручную
# =============================================================================

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; NC='\033[0m'
XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
CONF_DIR="/etc/amnezia/amneziawg"

echo -e "${GREEN}=== LucX Restore: восстановление интерфейсов ===${NC}"

# 1. Load AWG module if not loaded
if ! lsmod | grep -q amneziawg; then
    echo -e "${YELLOW}Загрузка модуля amneziawg...${NC}"
    modprobe amneziawg 2>/dev/null || {
        echo -e "${RED}Не удалось загрузить модуль amneziawg. Установи: bash bin/install-awg-module.sh${NC}"
        exit 1
    }
fi
echo -e "${GREEN}Модуль amneziawg загружен.${NC}"

# 2. Enable forwarding
echo 1 > /proc/sys/net/ipv4/ip_forward 2>/dev/null || true

# 3. Restore AWG interfaces from configs
RESTORED=0
if [[ -d "$CONF_DIR" ]]; then
    for conf in "$CONF_DIR"/awg*.conf; do
        [[ -f "$conf" ]] || continue
        AWG_ID=$(basename "$conf" .conf | sed 's/awg//')
        IFACE="awg${AWG_ID}"

        # Check if interface exists
        if ip link show "$IFACE" &>/dev/null; then
            echo -e "${GREEN}  ${IFACE}: уже существует${NC}"
        else
            echo -e "${GREEN}  ${IFACE}: поднимаю...${NC}"
            awg-quick up "$conf" 2>/dev/null && {
                echo -e "${GREEN}    ✓ поднят${NC}"
                ((RESTORED++))
            } || echo -e "${RED}    ✗ ошибка${NC}"
        fi

        # Restore peers from config
        PEERS=$(grep -c '^\[Peer\]' "$conf" 2>/dev/null || echo 0)
        echo -e "    пиров: ${PEERS}"

        # Start tun2socks if configured
        HIDDEN_PORT=$(grep -oP 'hiddenSOCKSPort.*?(\d+)' /etc/x-ui/x-ui.db 2>/dev/null | grep -oP '\d+' | head -1 || true)
        TUN_DEV="tun${AWG_ID}"
        if [[ -n "$HIDDEN_PORT" ]] && ! pgrep -f "tun2socks.*-device ${TUN_DEV}" &>/dev/null; then
            echo -e "${GREEN}  ${TUN_DEV}: запуск tun2socks (port ${HIDDEN_PORT})...${NC}"
            nohup tun2socks -device "$TUN_DEV" -proxy "socks5://127.0.0.1:${HIDDEN_PORT}" -loglevel silent &>/dev/null &
            sleep 1
            pgrep -f "tun2socks.*-device ${TUN_DEV}" &>/dev/null && \
                echo -e "${GREEN}    ✓ tun2socks запущен${NC}" || \
                echo -e "${YELLOW}    ⚠ tun2socks не запустился${NC}"
        fi
    done
fi

echo -e "${GREEN}=== Восстановлено интерфейсов: ${RESTORED} ===${NC}"
