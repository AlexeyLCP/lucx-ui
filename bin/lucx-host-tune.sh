#!/bin/bash
# Copyright (c) 2026 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

# Best-effort host tuning for VPN workloads (issue #215). Idempotent, never
# fatal: every step skips on failure with a printed reason. Opt-out with
# LUCX_HOST_TUNE=0. The BBR block stays owned by x-ui.sh's
# /etc/sysctl.d/99-bbr-x-ui.conf — this file never writes it.

set -u

ok()   { echo "  [ok]    $1"; }
skip() { echo "  [skip]  $1"; }
warn() { echo "  [warn]  $1"; }

if [[ "${LUCX_HOST_TUNE:-1}" == "0" ]]; then
    echo "Host tuning disabled via LUCX_HOST_TUNE=0."
    exit 0
fi
if [[ $EUID -ne 0 ]]; then
    warn "needs root; skipping host tuning"
    exit 0
fi

SYSCTL_CONF="/etc/sysctl.d/99-lucx-host-tune.conf"

# QUIC engines (Hysteria2, TUIC, quic-go) print a loud warning and throttle
# below ~7 MiB buffers; the kernel defaults (212k) are far under that.
sysctls=(
    "net.core.rmem_max = 33554432"
    "net.core.wmem_max = 33554432"
    "net.ipv4.udp_rmem_min = 16384"
    "net.ipv4.udp_wmem_min = 16384"
    "net.ipv4.tcp_fastopen = 3"
)

apply_sysctls=0
if [[ -f "$SYSCTL_CONF" ]]; then
    for kv in "${sysctls[@]}"; do
        if ! grep -qF "$kv" "$SYSCTL_CONF"; then
            warn "$SYSCTL_CONF exists but lacks '$kv' — leaving it as is"
        fi
    done
    skip "$SYSCTL_CONF already present"
else
    {
        echo "# LucX-UI host tuning (issue #215): QUIC/UDP buffers + TCP fastopen."
        echo "# Managed by install.sh; disable with LUCX_HOST_TUNE=0."
        for kv in "${sysctls[@]}"; do
            echo "$kv"
        done
    } > "$SYSCTL_CONF"
    apply_sysctls=1
    ok "wrote $SYSCTL_CONF (QUIC buffers, tcp_fastopen)"
fi

if [[ "$apply_sysctls" == "1" ]]; then
    if sysctl -p "$SYSCTL_CONF" >/dev/null 2>&1; then
        ok "sysctl values applied"
    else
        warn "sysctl refused (container/permission?) — values apply after reboot"
    fi
fi

# The panel keeps many concurrent subscription/Xray sockets: raise the file
# descriptor limit for the unit via a drop-in instead of editing the unit.
DROPIN="/etc/systemd/system/x-ui.service.d/10-lucx-limits.conf"
if [[ ! -d /etc/systemd/system ]]; then
    skip "systemd not present — nofile limit not set"
elif [[ -f "$DROPIN" ]] && grep -q "LimitNOFILE=65535" "$DROPIN" 2>/dev/null; then
    skip "$DROPIN already present"
else
    if mkdir -p /etc/systemd/system/x-ui.service.d &&
        printf '[Service]\nLimitNOFILE=65535\n' > "$DROPIN" &&
        systemctl daemon-reload >/dev/null 2>&1; then
        ok "wrote $DROPIN (LimitNOFILE=65535)"
    else
        warn "could not write the systemd drop-in (alpine/rc-service?)"
    fi
fi

echo "Host tuning finished."
exit 0
