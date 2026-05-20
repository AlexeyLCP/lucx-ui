#!/bin/bash
set -e

# =============================================================================
# LucX-UI: Применение LUCX-HOOK патчей к исходникам 3x-ui
# Централизованная система управления изменениями в оригинальном коде
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PATCH_DIR="${SCRIPT_DIR}/../patch"
SRC_DIR="${SCRIPT_DIR}/.."

echo -e "${GREEN}=== Применение LUCX-HOOK патчей ===${NC}"

# LUCX-HOOK маркеры уже вшиты в исходники через комментарии // LUCX-HOOK / // END LUCX-HOOK.
# Этот скрипт проверяет их целостность и применяет дополнительные .patch файлы.
# Все прямые изменения 3x-ui файлов идут ТОЛЬКО через LUCX-HOOK или .patch файлы.

HOOK_COUNT=0

# Проверка наличия LUCX-HOOK маркеров в ключевых файлах
check_hooks() {
    local file="$1"
    local desc="$2"
    if [[ ! -f "${SRC_DIR}/${file}" ]]; then
        echo -e "  ${YELLOW}⚠${NC} ${file}: файл не найден"
        return
    fi
    local open=$(grep -c '// LUCX-HOOK' "${SRC_DIR}/${file}" 2>/dev/null || echo 0)
    local close=$(grep -c '// END LUCX-HOOK' "${SRC_DIR}/${file}" 2>/dev/null || echo 0)
    if [[ "$open" == "$close" ]]; then
        echo -e "  ${GREEN}✓${NC} ${file}: ${open} LUCX-HOOK блоков (${desc})"
        HOOK_COUNT=$((HOOK_COUNT + open))
    else
        echo -e "  ${RED}✗${NC} ${file}: рассинхрон! open=${open} close=${close}"
    fi
}

echo -e "${GREEN}Проверка целостности LUCX-HOOK:${NC}"
check_hooks "web/service/xray.go"       "Xray config"
check_hooks "web/service/inbound.go"    "Inbound service"
check_hooks "web/controller/inbound.go" "Inbound controller"
check_hooks "web/controller/api.go"     "API controller"
check_hooks "web/web.go"               "Server startup"
check_hooks "database/model/model.go"   "DB model"
check_hooks "internal/lucx/controller/lucx_controller.go" "LucX controller"
check_hooks "frontend/src/models/inbound.js"    "Frontend inbound model"
check_hooks "frontend/src/pages/inbounds/InboundsPage.vue" "Inbounds page"
check_hooks "frontend/src/pages/inbounds/QrCodeModal.vue"  "QR modal"
check_hooks "frontend/src/pages/inbounds/InboundInfoModal.vue" "Info modal"
check_hooks "frontend/src/pages/inbounds/InboundFormModal.vue" "Form modal"
check_hooks "frontend/src/pages/inbounds/useInbounds.js" "useInbounds"
check_hooks "frontend/src/lucx/awg-config-gen.js" "AWG config gen"
check_hooks "frontend/src/api/lucx-api.js" "LucX API"
check_hooks "frontend/src/models/dbinbound.js" "DB inbound"

echo ""
echo -e "Всего LUCX-HOOK блоков: ${HOOK_COUNT}"

# Apply .patch files if any
if [[ -d "$PATCH_DIR" ]] && compgen -G "${PATCH_DIR}/*.patch" &>/dev/null; then
    echo ""
    echo -e "${GREEN}Применение .patch файлов:${NC}"
    for patch in "${PATCH_DIR}"/*.patch; do
        echo -e "  $(basename "$patch")..."
        if patch -p1 -d "$SRC_DIR" < "$patch" 2>&1 | tail -1; then
            echo -e "    ${GREEN}✓${NC}"
        else
            echo -e "    ${YELLOW}⚠ (возможно уже применён)${NC}"
        fi
    done
fi

echo -e "${GREEN}=== LUCX-HOOK проверка завершена ===${NC}"
