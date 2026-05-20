#!/bin/bash
# =============================================================================
# LucX-UI: Проверка здоровья AWG после установки/ребута
# =============================================================================

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; NC='\033[0m'
PASS=0; FAIL=0; WARN=0

check() { if eval "$2" &>/dev/null; then echo -e "  ${GREEN}✓${NC} $1"; ((PASS++)); else echo -e "  ${RED}✗${NC} $1"; ((FAIL++)); fi }
warn()  { if eval "$2" &>/dev/null; then echo -e "  ${GREEN}✓${NC} $1"; ((PASS++)); else echo -e "  ${YELLOW}⚠${NC} $1"; ((WARN++)); fi }

echo -e "${GREEN}=== LucX Health Check ===${NC}"
echo ""

echo -e "${GREEN}[Модуль ядра]${NC}"
check "Модуль amneziawg загружен"            "lsmod | grep -q amneziawg"
check "Утилита awg в PATH"                   "command -v awg"
check "Утилита tun2socks в PATH"             "command -v tun2socks"
warn  "nftables установлен"                  "command -v nft"

echo ""
echo -e "${GREEN}[Сервисы]${NC}"
check "x-ui запущен"                         "systemctl is-active --quiet x-ui"
check "lucx-restore.service активирован"     "systemctl is-enabled --quiet lucx-restore.service 2>/dev/null || [[ -f /etc/systemd/system/lucx-restore.service ]]"

echo ""
echo -e "${GREEN}[Сеть]${NC}"
check "IP forwarding включён"                "[[ \$(cat /proc/sys/net/ipv4/ip_forward) -eq 1 ]]"
check "initramfs обновлён (модуль выживет ребут)" "[[ -f /boot/initrd.img-$(uname -r) ]]"

echo ""
echo -e "${GREEN}[AWG интерфейсы]${NC}"
AWG_COUNT=0; PEER_COUNT=0
for iface in $(ip link show 2>/dev/null | grep -oP 'awg\d+' | sort -u); do
    echo -e "  ${GREEN}✓${NC} $iface up"
    ((AWG_COUNT++))
    PEERS=$(awg show "$iface" 2>/dev/null | grep -c 'peer:' || echo 0)
    echo -e "    пиров: $PEERS"
    ((PEER_COUNT += PEERS))
done
[[ $AWG_COUNT -eq 0 ]] && echo -e "  ${YELLOW}⚠${NC} Нет AWG интерфейсов"

echo ""
echo -e "${GREEN}[tun2socks]${NC}"
TUN_COUNT=$(pgrep -cf 'tun2socks' 2>/dev/null || echo 0)
if [[ $TUN_COUNT -gt 0 ]]; then
    echo -e "  ${GREEN}✓${NC} Процессов tun2socks: $TUN_COUNT"
    ((PASS++))
else
    echo -e "  ${YELLOW}⚠${NC} tun2socks не запущен"
    ((WARN++))
fi

echo ""
echo -e "${GREEN}[SOCKS5 hidden inbounds]${NC}"
SOCKS_COUNT=$(ss -tlnp 2>/dev/null | grep -c '127.0.0.1:108' || echo 0)
if [[ $SOCKS_COUNT -gt 0 ]]; then
    echo -e "  ${GREEN}✓${NC} Скрытых SOCKS5: $SOCKS_COUNT"
    ((PASS++))
else
    echo -e "  ${YELLOW}⚠${NC} Нет скрытых SOCKS5"
    ((WARN++))
fi

echo ""
echo -e "${GREEN}[База данных]${NC}"
check "БД x-ui существует"                   "[[ -f /etc/x-ui/x-ui.db ]]"
DB_AWG=$(sqlite3 /etc/x-ui/x-ui.db "SELECT count(*) FROM inbounds WHERE protocol='awg'" 2>/dev/null || echo 0)
echo -e "  ${GREEN}✓${NC} AWG inbound'ов в БД: $DB_AWG"
((PASS++))

echo ""
echo -e "============================================"
echo -e "  ${GREEN}Пройдено: ${PASS}${NC}  ${RED}Провалено: ${FAIL}${NC}  ${YELLOW}Предупреждений: ${WARN}${NC}"
echo -e "============================================"
