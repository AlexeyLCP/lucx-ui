#!/bin/bash
set -e

# =============================================================================
# LucX-UI: Удаление LucX Patch — возврат к чистому 3x-ui
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
STATE_FILE="/etc/lucx-ui/patch-state.env"

[[ $EUID -ne 0 ]] && { echo -e "${RED}Запустите с правами root${NC}"; exit 1; }

echo -e "${YELLOW}=== Удаление LucX Patch ===${NC}"
echo -e "${RED}Это удалит все AWG/Telemt компоненты.${NC}"

if [[ ! -f "$STATE_FILE" ]]; then
    echo -e "${YELLOW}Патч не найден (нет ${STATE_FILE}). Нечего удалять.${NC}"
    exit 0
fi
source "$STATE_FILE"
echo -e "Версия патча: ${LUCX_PATCH_VERSION}"

read -rp "Продолжить? (y/N): " confirm
[[ "$confirm" != "y" && "$confirm" != "Y" ]] && { echo "Отмена."; exit 0; }

# 1. Remove systemd service
systemctl disable lucx-restore.service 2>/dev/null || true
rm -f /etc/systemd/system/lucx-restore.service
systemctl daemon-reload

# 2. Remove LucX binaries
rm -f /usr/local/bin/lucx-restore /usr/local/bin/lucx-health-check

# 3. Remove AWG configs (but keep backups)
echo -e "${YELLOW}Сохранение конфигов AWG в /etc/lucx-ui/backups/...${NC}"
mkdir -p /etc/lucx-ui/backups
cp -a /etc/amnezia/amneziawg /etc/lucx-ui/backups/awg-configs-$(date +%Y%m%d) 2>/dev/null || true

# 4. Remove state file
rm -f "$STATE_FILE"

# 5. Restart x-ui
echo -e "${GREEN}Перезапуск x-ui...${NC}"
systemctl restart x-ui 2>/dev/null || true

echo -e "${GREEN}LucX Patch удалён. 3x-ui работает в оригинальном режиме.${NC}"
echo -e "${YELLOW}Конфиги AWG сохранены в /etc/lucx-ui/backups/${NC}"
