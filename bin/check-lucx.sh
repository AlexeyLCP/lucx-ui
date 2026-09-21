#!/bin/bash
# Copyright (c) 2025 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

# =============================================================================
# LucX-UI: gofumpt-проверка LucX-кода перед пушем.
#
# CI (golangci-lint) падает на форматировании, а gofumpt на Windows и Linux
# форматирует по-разному — ловим это локально, а не пятой итерацией CI.
# Проверяет изолированные пакеты (internal/awg, internal/lucx) + все файлы
# с LUCX-HOOK маркерами + LucX-файлы БД/контроллеров.
#
# Использование:
#   bin/check-lucx.sh          — проверка (exit 1 со списком файлов)
#   bin/check-lucx.sh -w       — исправить на месте (gofumpt -w)
# =============================================================================
set -uo pipefail

cd "$(dirname "$0")/.."

GOFUMPT=${GOFUMPT:-gofumpt}
if ! command -v "$GOFUMPT" >/dev/null 2>&1; then
    GOFUMPT="$(go env GOPATH)/bin/gofumpt"
fi
if [ ! -x "$GOFUMPT" ] && ! command -v "$GOFUMPT" >/dev/null 2>&1; then
    echo "gofumpt не найден. Установите: go install mvdan.cc/gofumpt@latest" >&2
    exit 2
fi

mapfile -t HOOK_FILES < <(grep -rl "LUCX-HOOK" internal/ --include='*.go' 2>/dev/null)
mapfile -t PKG_FILES < <(find internal/awg internal/lucx -name '*.go' 2>/dev/null)

# LucX-файлы вне изолированных пакетов и без LUCX-HOOK маркеров (новые файлы
# веб-слоя туннельных сайдкаров — SPDX-заголовок есть, маркера нет).
EXTRA_FILES=(
    internal/web/service/awg_import.go
    internal/web/service/awg_host.go
    internal/web/service/awg_host_tail_test.go
    internal/web/service/tunnel.go
    internal/web/controller/tunnel.go
    internal/web/controller/lucx.go
    internal/web/job/tunnel_job.go
    internal/database/migrate_awg_keepalive.go
    internal/database/migrate_awg_keepalive_test.go
    internal/web/job/awg_speed_buffer.go
    internal/web/job/awg_speed_buffer_test.go
    internal/web/controller/client_subbody_test.go
    internal/web/service/sidecar_outbound.go
    internal/web/service/sidecar_outbound_inject_test.go
    internal/web/service/lucx_online.go
    internal/web/service/lucx_online_test.go
    internal/web/controller/sidecar_outbound.go
)

FILES=$(printf '%s\n' "${HOOK_FILES[@]}" "${PKG_FILES[@]}" "${EXTRA_FILES[@]}" | sort -u | grep -v '^$')

if [ "${1:-}" = "-w" ]; then
    echo "$FILES" | xargs "$GOFUMPT" -w
    echo "gofumpt -w применён к $(echo "$FILES" | wc -l) файлам"
    exit 0
fi

BAD=$(echo "$FILES" | xargs "$GOFUMPT" -l 2>/dev/null)
if [ -n "$BAD" ]; then
    echo "gofumpt: требуют форматирования:"
    echo "$BAD"
    echo
    echo "Исправить: bin/check-lucx.sh -w"
    exit 1
fi
echo "gofumpt: OK ($(echo "$FILES" | wc -l) файлов)"

# AWG must run on fresh install and update (dropped in v3.8 overlay, lucx.248).
for f in install.sh update.sh; do
    if ! grep -q 'bin/install-awg-module.sh' "$f"; then
        echo "missing AWG install call in $f" >&2
        exit 1
    fi
done
echo "AWG install hooks: OK"

# SPDX header on every LucX file. Go: the same set gofumpt checks. Shell:
# bin/*.sh. Frontend: files under LucX-owned dirs/names (awg, lucx, tunnel,
# sidecar, mieru, masking, trusttunnel, tproxy, anytls, cover, gateway,
# csqtt, olcrtc, qwdtt) — upstream files have no header and stay exempt.
NO_SPDX=$(echo "$FILES" | xargs grep -L "SPDX-License-Identifier" 2>/dev/null)
LUCX_FE=$(find frontend/src \( -iname '*awg*' -o -iname '*lucx*' -o -iname '*tunnel*' \
    -o -iname '*sidecar*' -o -iname '*mieru*' -o -iname '*masking*' \
    -o -iname '*trusttunnel*' -o -iname '*tproxy*' -o -iname '*anytls*' \
    -o -iname '*cover*' -o -iname '*gateway*' -o -iname '*csqtt*' \
    -o -iname '*olcrtc*' -o -iname '*qwdtt*' -o -iname '*wireguardConfig*' \
    -o -iname '*VpnConfBlock*' \) \
    \( -name '*.ts' -o -name '*.tsx' \) 2>/dev/null | xargs grep -L "SPDX-License-Identifier" 2>/dev/null || true)
LUCX_SH=$(find bin -name '*.sh' 2>/dev/null | xargs grep -L "SPDX-License-Identifier" 2>/dev/null || true)
NO_SPDX=$(printf '%s\n%s\n%s\n' "$NO_SPDX" "$LUCX_FE" "$LUCX_SH" | grep -v '^$' | sort -u)
if [ -n "$NO_SPDX" ]; then
    echo "missing SPDX-License-Identifier header:" >&2
    echo "$NO_SPDX" >&2
    exit 1
fi
echo "SPDX headers: OK"
