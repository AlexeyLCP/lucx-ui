#!/bin/bash
# Copyright (c) 2025 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

set -e

# =============================================================================
# LucX-UI: Сборка релиза x-ui-linux-amd64.tar.gz на VPS (Linux/amd64)
# =============================================================================
#
# Зачем: CGO-бинарник панели (mattn/go-sqlite3) нельзя cross-compile с Windows
# на Linux — нужен gcc + linux-заголовки на VPS. Этот скрипт собирает всё на
# месте и упаковывает tarball, совместимый с install.sh (структура как у
# апстрим-релиза 3x-ui).
#
# Запуск (на VPS, Ubuntu/Debian amd64):
#   curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/bin/build-release.sh | bash
# или локально из клона:
#   bash bin/build-release.sh
#
# Результат: /tmp/x-ui-linux-amd64.tar.gz
#
# Затем создать GitHub-релиз (нужен gh CLI с auth):
#   gh release create v3.5.0-lucx.1 /tmp/x-ui-linux-amd64.tar.gz \
#     --repo AlexeyLCP/lucx-ui \
#     --title "v3.5.0-lucx.1" \
#     --notes "LucX-UI v3.5.0 с AWG-сайдкаром"
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

LUCX_REPO="AlexeyLCP/lucx-ui"
LUCX_BRANCH="main"
UPSTREAM_REPO="MHSanaei/3x-ui"
UPSTREAM_TAG="v3.5.0"
ARCH="amd64"

BUILD_DIR="/tmp/lucx-build-$$"
OUT_TARBALL="/tmp/x-ui-linux-${ARCH}.tar.gz"

echo -e "${GREEN}=== LucX-UI: сборка релиза ===${NC}"

# 1. Проверка инструментов
echo -e "${GREEN}Проверка инструментов...${NC}"
for cmd in go node npm git curl tar gcc; do
    if ! command -v "$cmd" &>/dev/null; then
        echo -e "${RED}Не найден: $cmd${NC}"
        echo -e "${YELLOW}Установи: apt-get install -y golang-go nodejs npm git curl gcc tar${NC}"
        exit 1
    fi
done
echo -e "${GREEN}Все инструменты на месте.${NC}"

# Go 1.23+ нужен (go.mod требует). Проверим минимальную версию.
# `go version` выдаёт "go1.26.5 linux/amd64" — парсим major.minor.
GO_VERSION=$(go version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
if [[ -z "$GO_MAJOR" || -z "$GO_MINOR" ]]; then
    echo -e "${RED}Не удалось определить версию Go: $(go version 2>&1)${NC}"
    exit 1
fi
if [[ "$GO_MAJOR" -lt 1 || "$GO_MINOR" -lt 23 ]]; then
    echo -e "${RED}Go >= 1.23 требуется (рекомендуется 1.26+). Текущая: go${GO_VERSION}${NC}"
    exit 1
fi

# 2. Клон нашего форка
echo -e "${GREEN}Клон $LUCX_REPO:$LUCX_BRANCH...${NC}"
rm -rf "$BUILD_DIR"
git clone --depth 1 -b "$LUCX_BRANCH" "https://github.com/${LUCX_REPO}.git" "$BUILD_DIR"
cd "$BUILD_DIR"

# 3. Сборка frontend → internal/web/dist/
echo -e "${GREEN}Сборка frontend...${NC}"
cd frontend
npm install --silent 2>&1 | tail -3 || { echo -e "${RED}npm install failed${NC}"; exit 1; }
npm run build 2>&1 | tail -5 || { echo -e "${RED}npm run build failed${NC}"; exit 1; }
cd ..

if [[ ! -d internal/web/dist ]]; then
    echo -e "${RED}internal/web/dist не собран — frontend-сборка не удалась${NC}"
    exit 1
fi
echo -e "${GREEN}Frontend собран в internal/web/dist/${NC}"

# 4. Сборка бинарника панели (CGO для sqlite)
echo -e "${GREEN}Сборка бинарника x-ui (CGO_ENABLED=1)...${NC}"
CGO_ENABLED=1 go build -o x-ui . 2>&1 | tail -10 || { echo -e "${RED}go build failed${NC}"; exit 1; }
if [[ ! -s x-ui ]]; then
    echo -e "${RED}Бинарник x-ui не собран${NC}"
    exit 1
fi
chmod +x x-ui
echo -e "${GREEN}Бинарник x-ui собран ($(du -h x-ui | cut -f1))${NC}"

# 5. Скачивание Xray + mtg из апстрим-релиза (не наш код — переиспользуем)
echo -e "${GREEN}Скачивание Xray-core + mtg из $UPSTREAM_REPO $UPSTREAM_TAG...${NC}"
UPSTREAM_TARBALL="/tmp/x-ui-upstream-${ARCH}.tar.gz"
curl -fL --retry 3 --max-time 300 -o "$UPSTREAM_TARBALL" \
    "https://github.com/${UPSTREAM_REPO}/releases/download/${UPSTREAM_TAG}/x-ui-linux-${ARCH}.tar.gz" \
    || { echo -e "${RED}Не удалось скачать апстрим-релиз${NC}"; exit 1; }

mkdir -p bin
UPSTREAM_EXTRACT="/tmp/x-ui-upstream-extract-$$"
mkdir -p "$UPSTREAM_EXTRACT"
tar xzf "$UPSTREAM_TARBALL" -C "$UPSTREAM_EXTRACT"
# Апстрим tarball содержит x-ui/bin/xray-linux-amd64 и x-ui/bin/mtg-linux-amd64
if [[ -f "$UPSTREAM_EXTRACT/x-ui/bin/xray-linux-${ARCH}" ]]; then
    cp "$UPSTREAM_EXTRACT/x-ui/bin/xray-linux-${ARCH}" "bin/xray-linux-${ARCH}"
    chmod +x "bin/xray-linux-${ARCH}"
    echo -e "${GREEN}xray-linux-${ARCH} скопирован ($(du -h bin/xray-linux-${ARCH} | cut -f1))${NC}"
else
    echo -e "${RED}xray-linux-${ARCH} не найден в апстрим-релизе${NC}"
    exit 1
fi
if [[ -f "$UPSTREAM_EXTRACT/x-ui/bin/mtg-linux-${ARCH}" ]]; then
    cp "$UPSTREAM_EXTRACT/x-ui/bin/mtg-linux-${ARCH}" "bin/mtg-linux-${ARCH}"
    chmod +x "bin/mtg-linux-${ARCH}"
    echo -e "${GREEN}mtg-linux-${ARCH} скопирован ($(du -h bin/mtg-linux-${ARCH} | cut -f1))${NC}"
else
    echo -e "${YELLOW}⚠ mtg-linux-${ARCH} не найден в апстрим-релизе — MTProto sidecar будет недоступен${NC}"
fi
rm -rf "$UPSTREAM_EXTRACT" "$UPSTREAM_TARBALL"

# 6. Подготовка структуры tarball (как у апстрима)
# Файлы x-ui.sh, x-ui.rc, x-ui.service.* уже в корне клона — оставляем.
# bin/install-awg-module.sh уже в репо — оставляем.
echo -e "${GREEN}Структура tarball:${NC}"
ls -la x-ui x-ui.sh x-ui.rc x-ui.service.* bin/ 2>&1 | head -20

# 7. Упаковка
echo -e "${GREEN}Упаковка $OUT_TARBALL...${NC}"
cd /tmp
rm -f "$OUT_TARBALL"
# Запаковываем содержимое BUILD_DIR как x-ui/ (стандартный путь апстрима)
TARBALL_ROOT="x-ui"
rm -rf "$TARBALL_ROOT"
cp -r "$BUILD_DIR" "$TARBALL_ROOT"
tar czf "$OUT_TARBALL" "$TARBALL_ROOT"
rm -rf "$TARBALL_ROOT"

echo -e "${GREEN}=== Сборка завершена ===${NC}"
echo ""
echo -e "${GREEN}Tarball: ${OUT_TARBALL} ($(du -h "$OUT_TARBALL" | cut -f1))${NC}"
echo ""
echo -e "${YELLOW}Следующие шаги:${NC}"
echo -e "  1. Создай GitHub-релиз (нужен gh CLI с auth):"
echo -e "     gh release create v3.5.0-lucx.1 ${OUT_TARBALL} \\"
echo -e "       --repo ${LUCX_REPO} \\"
echo -e "       --title \"v3.5.0-lucx.1\" \\"
echo -e "       --notes \"LucX-UI v3.5.0 с AWG-сайдкаром\""
echo -e "  2. Установи панель:"
echo -e "     bash <(curl -fL https://raw.githubusercontent.com/${LUCX_REPO}/${LUCX_BRANCH}/install.sh)"
echo ""