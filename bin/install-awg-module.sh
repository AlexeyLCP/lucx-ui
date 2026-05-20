#!/bin/bash
set -e

# =============================================================================
# LucX-UI: Установка модуля ядра AmneziaWG (DKMS + update-initramfs)
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

echo -e "${GREEN}=== Установка модуля ядра AmneziaWG ===${NC}"

[[ $EUID -ne 0 ]] && { echo -e "${RED}Запустите с правами root${NC}"; exit 1; }

# Check if already loaded
if [[ -d /sys/module/amneziawg ]]; then
    echo -e "${GREEN}Модуль amneziawg уже загружен.${NC}"
    command -v awg &>/dev/null && { echo -e "${GREEN}awg уже установлен.${NC}"; exit 0; }
fi

# 1. Install build dependencies
echo -e "${GREEN}Установка сборочных зависимостей...${NC}"
apt-get update -qq
apt-get install -y -q build-essential dkms linux-headers-$(uname -r) git unzip curl \
    || apt-get install -y -q build-essential dkms linux-headers-amd64 git unzip curl \
    || true

# 2. Check kernel headers
if [[ ! -d /lib/modules/$(uname -r)/build ]]; then
    echo -e "${RED}Заголовки ядра для $(uname -r) не найдены.${NC}"
    echo -e "${YELLOW}Попробуй: apt-get install linux-headers-$(uname -r)${NC}"
    exit 1
fi
echo -e "${GREEN}Заголовки ядра: OK${NC}"

# 3. Build and install kernel module via DKMS
if [[ ! -d /sys/module/amneziawg ]]; then
    echo -e "${GREEN}Сборка модуля ядра из исходников...${NC}"
    KERNEL_MOD_DIR="/tmp/amneziawg-kmod-$$"
    rm -rf "$KERNEL_MOD_DIR"
    git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git "$KERNEL_MOD_DIR"
    cd "$KERNEL_MOD_DIR/src"

    make dkms-install 2>/dev/null || true
    MOD_VER=$(grep -oP 'version\s*"\K[^"]+' dkms.conf 2>/dev/null || echo "1.0.0")
    dkms add -m amneziawg -v "$MOD_VER" 2>/dev/null || true
    dkms build -m amneziawg -v "$MOD_VER" || {
        echo -e "${RED}Ошибка сборки DKMS. Проверь заголовки ядра.${NC}"
        exit 1
    }
    dkms install -m amneziawg -v "$MOD_VER"

    cd /tmp; rm -rf "$KERNEL_MOD_DIR"
    echo -e "${GREEN}Модуль ядра собран и установлен.${NC}"
fi

# 4. Build and install userspace tools
if ! command -v awg &>/dev/null; then
    echo -e "${GREEN}Сборка утилит awg...${NC}"
    TOOLS_DIR="/tmp/amneziawg-tools-$$"
    rm -rf "$TOOLS_DIR"
    git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-tools.git "$TOOLS_DIR"
    cd "$TOOLS_DIR/src"
    make && make install
    cd /tmp; rm -rf "$TOOLS_DIR"
    echo -e "${GREEN}Утилиты awg установлены.${NC}"
fi

# 5. Load module and enable autostart
modprobe amneziawg 2>/dev/null || {
    echo -e "${YELLOW}Не удалось загрузить модуль. Возможно, нужен ребут.${NC}"
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
echo -e "${GREEN}=== Установка AWG завершена ===${NC}"
