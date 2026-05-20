#!/bin/bash
set -e

# =============================================================================
# LucX-UI: Этап 1 — установка чистого 3x-ui с зеркала
# =============================================================================
# Использование:
#   bash install-3xui-mirror.sh                 # с зеркала по умолчанию
#   bash install-3xui-mirror.sh --mirror URL     # с указанного зеркала
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; BLUE='\033[0;34m'; NC='\033[0m'

MIRROR="https://github.com/AlexeyLCP/3x-ui"
INSTALL_SCRIPT="install.sh"

usage() {
    echo "Usage: bash install-3xui-mirror.sh [--mirror URL]"
    echo "  --mirror URL   GitHub repo URL (default: https://github.com/AlexeyLCP/3x-ui)"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --mirror) MIRROR="$2"; shift 2 ;;
        --help|-h) usage ;;
        *) echo -e "${RED}Unknown option: $1${NC}"; usage ;;
    esac
done

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  LucX-UI: Этап 1 — Установка 3x-ui        ${NC}"
echo -e "${BLUE}  Зеркало: ${MIRROR}${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

# Check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Ошибка: скрипт нужно запускать с правами root${NC}"
    exit 1
fi

# Check if already installed
if [[ -f /usr/local/x-ui/x-ui ]]; then
    CURRENT_VERSION=$(/usr/local/x-ui/x-ui version 2>/dev/null || echo "unknown")
    echo -e "${YELLOW}3x-ui уже установлен (версия: ${CURRENT_VERSION}).${NC}"
    read -rp "Переустановить? (y/N): " reinstall
    if [[ "$reinstall" != "y" && "$reinstall" != "Y" ]]; then
        echo -e "${GREEN}Пропуск установки 3x-ui.${NC}"
        echo -e "${GREEN}Теперь выполни: bash install-lucx-patch.sh${NC}"
        exit 0
    fi
    echo -e "${YELLOW}Останавливаю x-ui перед переустановкой...${NC}"
    systemctl stop x-ui 2>/dev/null || true
fi

# Download and run the official install script from mirror
echo -e "${GREEN}Загрузка установщика 3x-ui с зеркала...${NC}"
INSTALL_URL="${MIRROR}/raw/main/${INSTALL_SCRIPT}"
TMP_SCRIPT="/tmp/3x-ui-install-$$.sh"

if ! curl -fsSL "$INSTALL_URL" -o "$TMP_SCRIPT"; then
    echo -e "${RED}Ошибка загрузки установщика с ${INSTALL_URL}${NC}"
    exit 1
fi

echo -e "${GREEN}Запуск установки 3x-ui...${NC}"
bash "$TMP_SCRIPT"
rm -f "$TMP_SCRIPT"

# Verify installation
if [[ ! -f /usr/local/x-ui/x-ui ]]; then
    echo -e "${RED}Ошибка: 3x-ui не установлен. Проверь вывод выше.${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  3x-ui успешно установлен!                 ${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo -e "${YELLOW}Теперь выполни этап 2 — установку LucX Patch:${NC}"
echo -e "  ${BLUE}bash install-lucx-patch.sh${NC}"
echo ""
