#!/bin/bash
set -e

# =============================================================================
# LucX-UI: Этап 2 — LucX Patch (AWG + Telemt + обфускация) поверх 3x-ui
# =============================================================================
# Idempotent: можно запускать повторно, бэкапит перед патчем, проверяет версии.
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; BLUE='\033[0;34m'; NC='\033[0m'

XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
PATCH_VERSION="0.2.0-pre"
BACKUP_DIR="/etc/lucx-ui/backups/$(date +%Y%m%d-%H%M%S)"
STATE_FILE="/etc/lucx-ui/patch-state.env"

usage() {
    echo "Usage: bash install-lucx-patch.sh [--force] [--skip-build] [--skip-awg]"
    echo "  --force        Применить патч даже если уже установлен"
    echo "  --skip-build   Пропустить пересборку (при разработке)"
    echo "  --skip-awg     Пропустить установку модуля AWG"
    exit 1
}

FORCE=false; SKIP_BUILD=false; SKIP_AWG=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --force) FORCE=true; shift ;;
        --skip-build) SKIP_BUILD=true; shift ;;
        --skip-awg) SKIP_AWG=true; shift ;;
        --help|-h) usage ;;
        *) echo -e "${RED}Unknown: $1${NC}"; usage ;;
    esac
done

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  LucX-UI: Этап 2 — LucX Patch v${PATCH_VERSION}${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""

[[ $EUID -ne 0 ]] && { echo -e "${RED}Запустите с правами root${NC}"; exit 1; }

# --- Determine source: cloned repo or standalone download ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IS_CLONED_REPO=false
if [[ -f "${SCRIPT_DIR}/main.go" ]] && [[ -d "${SCRIPT_DIR}/internal/lucx" ]]; then
    IS_CLONED_REPO=true
    echo -e "${GREEN}Режим: клонированный репозиторий${NC}"
else
    echo -e "${GREEN}Режим: standalone (скачиваю релизный тарболл)${NC}"
    RELEASE_URL="https://github.com/AlexeyLCP/lucx-ui/releases/download/v0.2.0-pre/x-ui-linux-amd64.tar.gz"
    TMP_DIR="/tmp/lucx-patch-$$"
    mkdir -p "$TMP_DIR"
    echo -e "${GREEN}Загрузка релиза...${NC}"
    curl -fsSL "$RELEASE_URL" -o "$TMP_DIR/x-ui.tar.gz" || {
        echo -e "${RED}Не удалось скачать релиз. Скачай вручную:${NC}"
        echo -e "  git clone https://github.com/AlexeyLCP/lucx-ui && cd lucx-ui && bash install-lucx-patch.sh"
        exit 1
    }
    tar xzf "$TMP_DIR/x-ui.tar.gz" -C "$TMP_DIR"
    SCRIPT_DIR="$TMP_DIR/x-ui"
fi

# --- Pre-checks ---
if [[ ! -f "${XUI_DIR}/x-ui" ]]; then
    echo -e "${RED}3x-ui не найден в ${XUI_DIR}. Сначала выполни этап 1:${NC}"
    echo -e "  ${BLUE}bash install-3xui-mirror.sh${NC}"
    exit 1
fi

XUI_VERSION=$(${XUI_DIR}/x-ui version 2>/dev/null || echo "unknown")
echo -e "${GREEN}Найден 3x-ui: ${XUI_VERSION}${NC}"

# --- Idempotency check ---
if [[ -f "$STATE_FILE" ]] && [[ "$FORCE" != "true" ]]; then
    source "$STATE_FILE"
    if [[ "$LUCX_PATCH_VERSION" == "$PATCH_VERSION" ]]; then
        echo -e "${GREEN}LucX Patch v${PATCH_VERSION} уже применён.${NC}"
        echo -e "${YELLOW}Для переустановки: bash install-lucx-patch.sh --force${NC}"
        exit 0
    fi
    echo -e "${YELLOW}Обнаружена предыдущая версия патча ($LUCX_PATCH_VERSION). Обновление до ${PATCH_VERSION}.${NC}"
fi

# --- Backup ---
echo -e "${GREEN}Создание бэкапа в ${BACKUP_DIR}...${NC}"
mkdir -p "$BACKUP_DIR"
cp -a "${XUI_DIR}/x-ui" "${BACKUP_DIR}/x-ui.bak" 2>/dev/null || true
cp -a "${XUI_DIR}/web" "${BACKUP_DIR}/web.bak" 2>/dev/null || true
cp -a "${XUI_DIR}/frontend" "${BACKUP_DIR}/frontend.bak" 2>/dev/null || true
echo -e "${GREEN}Бэкап создан.${NC}"

# --- Install AWG kernel module ---
if [[ "$SKIP_AWG" != "true" ]]; then
    if [[ -f "${SCRIPT_DIR}/bin/install-awg-module.sh" ]]; then
        bash "${SCRIPT_DIR}/bin/install-awg-module.sh"
    else
        echo -e "${YELLOW}bin/install-awg-module.sh не найден — пропускаю установку модуля${NC}"
    fi
fi

# --- Install nftables ---
if ! command -v nft &>/dev/null; then
    echo -e "${GREEN}Установка nftables...${NC}"
    apt-get update -qq && apt-get install -y -q nftables
fi

# --- Install tun2socks ---
if ! command -v tun2socks &>/dev/null; then
    if [[ -f "${SCRIPT_DIR}/bin/tun2socks-linux-amd64" ]]; then
        cp "${SCRIPT_DIR}/bin/tun2socks-linux-amd64" /usr/local/bin/tun2socks
        chmod +x /usr/local/bin/tun2socks
        echo -e "${GREEN}tun2socks установлен из bin/.${NC}"
    else
        echo -e "${YELLOW}tun2socks не найден — установи вручную${NC}"
    fi
fi

# --- Apply LucX hooks ---
if [[ -f "${SCRIPT_DIR}/bin/apply-lucx-hooks.sh" ]]; then
    echo -e "${GREEN}Применение LUCX-HOOK патчей...${NC}"
    bash "${SCRIPT_DIR}/bin/apply-lucx-hooks.sh"
fi

# --- Install binary ---
if [[ "$SKIP_BUILD" != "true" ]]; then
    if [[ "$IS_CLONED_REPO" == "true" ]]; then
        echo -e "${GREEN}Сборка из исходников...${NC}"
        if [[ -d "${SCRIPT_DIR}/frontend" ]]; then
            cd "${SCRIPT_DIR}/frontend"
            npm install --silent 2>/dev/null || true
            npm run build 2>&1 | tail -3
            cd "${SCRIPT_DIR}"
        fi
        go build -o "${XUI_DIR}/x-ui" -ldflags="-s -w" . 2>&1 || {
            echo -e "${RED}Ошибка сборки Go. Проверь зависимости.${NC}"
            exit 1
        }
    else
        echo -e "${GREEN}Установка бинарника из релиза...${NC}"
        if [[ -f "${SCRIPT_DIR}/x-ui" ]]; then
            cp "${SCRIPT_DIR}/x-ui" "${XUI_DIR}/x-ui"
            chmod +x "${XUI_DIR}/x-ui"
            echo -e "${GREEN}Бинарник установлен.${NC}"
        else
            echo -e "${RED}Бинарник не найден в релизе!${NC}"
            exit 1
        fi
    fi
fi

# --- Configure systemd ---
if [[ -f "${SCRIPT_DIR}/bin/lucx-restore.sh" ]]; then
    cp "${SCRIPT_DIR}/bin/lucx-restore.sh" /usr/local/bin/lucx-restore
    chmod +x /usr/local/bin/lucx-restore
fi

if [[ -f "${SCRIPT_DIR}/bin/lucx-health-check.sh" ]]; then
    cp "${SCRIPT_DIR}/bin/lucx-health-check.sh" /usr/local/bin/lucx-health-check
    chmod +x /usr/local/bin/lucx-health-check
fi

# Systemd oneshot for auto-restore after reboot
cat > /etc/systemd/system/lucx-restore.service << 'SYSTEMD'
[Unit]
Description=LucX-UI AWG Restore
After=x-ui.service network-online.target
Wants=x-ui.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/lucx-restore
RemainAfterExit=yes
TimeoutSec=60

[Install]
WantedBy=multi-user.target
SYSTEMD
systemctl daemon-reload
systemctl enable lucx-restore.service 2>/dev/null || true

# --- Set permissions ---
chmod +x "${XUI_DIR}/x-ui"

# --- Restart ---
echo -e "${GREEN}Перезапуск x-ui...${NC}"
systemctl restart x-ui 2>/dev/null || service x-ui restart 2>/dev/null || true
sleep 3

# --- Restore AWG ---
echo -e "${GREEN}Восстановление AWG интерфейсов...${NC}"
if [[ -f /usr/local/bin/lucx-restore ]]; then
    /usr/local/bin/lucx-restore
elif [[ -f "${XUI_DIR}/x-ui" ]]; then
    "${XUI_DIR}/x-ui" lucx-restore 2>/dev/null || true
fi

# --- Save state ---
mkdir -p /etc/lucx-ui
cat > "$STATE_FILE" << STATEFILE
LUCX_PATCH_VERSION=${PATCH_VERSION}
PATCH_DATE=$(date -Iseconds)
XUI_VERSION=${XUI_VERSION}
STATEFILE

# --- Health check ---
echo ""
if [[ -f /usr/local/bin/lucx-health-check ]]; then
    /usr/local/bin/lucx-health-check
else
    echo -e "${GREEN}============================================${NC}"
    echo -e "${GREEN}  LucX Patch v${PATCH_VERSION} установлен!   ${NC}"
    echo -e "${GREEN}============================================${NC}"
fi

echo ""
echo -e "${BLUE}Полезные команды:${NC}"
echo -e "  ${GREEN}lucx-health-check${NC}  — проверка здоровья AWG"
echo -e "  ${GREEN}lucx-restore${NC}       — восстановить интерфейсы AWG"
echo -e "  ${GREEN}bash uninstall-lucx-patch.sh${NC} — удалить патч"
