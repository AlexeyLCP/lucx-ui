#!/bin/bash
# Copyright (c) 2026 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

set -euo pipefail

ARCH="${1:?arch}"
DEST="${2:?dest dir}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CURL_RETRY="--retry 5 --retry-all-errors --retry-delay 3"

fetch() {
    wget -q --tries=5 --waitretry=10 --retry-on-http-error=429,500,502,503 "$@"
}

have() {
    [[ -s "${DEST}/$1" ]]
}

mkdir -p "$DEST"

if ! have "caddy-naive-linux-${ARCH}"; then
    if [[ "$ARCH" == amd64 ]]; then
        fetch -O /tmp/caddy-naive.tar.xz "https://github.com/klzgrad/forwardproxy/releases/download/v2.11.2-naive/caddy-forwardproxy-naive.tar.xz"
        tar -xJf /tmp/caddy-naive.tar.xz -C /tmp
        mv /tmp/caddy-forwardproxy-naive/caddy "${DEST}/caddy-naive-linux-amd64"
        rm -rf /tmp/caddy-forwardproxy-naive /tmp/caddy-naive.tar.xz
    else
        go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.7
        CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" "$(go env GOPATH)/bin/xcaddy" build v2.11.2 \
            --with github.com/klzgrad/forwardproxy/v2@v2.11.2-naive \
            --output "${DEST}/caddy-naive-linux-${ARCH}"
    fi
    chmod +x "${DEST}/caddy-naive-linux-${ARCH}"
fi

if ! have "naive-client-linux-${ARCH}"; then
    case "$ARCH" in
        amd64) naive_xz="naiveproxy-v150.0.7871.63-1-linux-x64.tar.xz" ;;
        arm64) naive_xz="naiveproxy-v150.0.7871.63-1-linux-arm64.tar.xz" ;;
        *) echo "no naive-client for ${ARCH}" >&2; exit 1 ;;
    esac
    fetch -O "/tmp/${naive_xz}" "https://github.com/klzgrad/naiveproxy/releases/download/v150.0.7871.63-1/${naive_xz}"
    mkdir -p /tmp/naiveclient
    tar -xJf "/tmp/${naive_xz}" -C /tmp/naiveclient
    naive_bin=$(find /tmp/naiveclient -type f -name naive | head -n1)
    mv "$naive_bin" "${DEST}/naive-client-linux-${ARCH}"
    chmod +x "${DEST}/naive-client-linux-${ARCH}"
    rm -rf /tmp/naiveclient "/tmp/${naive_xz}"
fi

if ! have "olcrtc-linux-${ARCH}"; then
    git init -q /tmp/olcrtc
    git -C /tmp/olcrtc remote add origin https://github.com/openlibrecommunity/olcrtc.git
    git -C /tmp/olcrtc fetch -q --depth 1 origin 54bd269bbc8c1c97c966307e0bca733366694a09
    git -C /tmp/olcrtc checkout -q FETCH_HEAD
    (
        cd /tmp/olcrtc
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/olcrtc-linux-${ARCH}" ./cmd/olcrtc
    )
    chmod +x "${DEST}/olcrtc-linux-${ARCH}"
    rm -rf /tmp/olcrtc
fi

if ! have "qwdtt-linux-${ARCH}"; then
    git init -q /tmp/qwdtt
    git -C /tmp/qwdtt remote add origin https://github.com/SpaceNeuroX/proxy-turn-vk-android.git
    git -C /tmp/qwdtt fetch -q --depth 1 origin fae121efc3ef57b633516601d3c0d6b1be1fde7c
    git -C /tmp/qwdtt checkout -q FETCH_HEAD
    (
        cd /tmp/qwdtt
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/qwdtt-linux-${ARCH}" ./server
    )
    chmod +x "${DEST}/qwdtt-linux-${ARCH}"
    rm -rf /tmp/qwdtt
fi

if ! have "mieru-linux-${ARCH}" || ! have "mieru-client-linux-${ARCH}"; then
    git clone --depth 1 --branch v3.36.0 https://github.com/enfein/mieru.git /tmp/mieru
    (
        cd /tmp/mieru
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/mieru-linux-${ARCH}" ./cmd/mita
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/mieru-client-linux-${ARCH}" ./cmd/mieru
    )
    chmod +x "${DEST}/mieru-linux-${ARCH}" "${DEST}/mieru-client-linux-${ARCH}"
    rm -rf /tmp/mieru
fi

if ! have "trusttunnel-linux-${ARCH}"; then
    case "$ARCH" in
        amd64) tt_tgz="trusttunnel-v1.1.0-linux-x86_64.tar.gz" ;;
        arm64) tt_tgz="trusttunnel-v1.1.0-linux-aarch64.tar.gz" ;;
        *) echo "no trusttunnel for ${ARCH}" >&2; exit 1 ;;
    esac
    fetch -O "/tmp/${tt_tgz}" "https://github.com/TrustTunnel/TrustTunnel/releases/download/v1.1.0/${tt_tgz}"
    mkdir -p /tmp/trusttunnel
    tar -xzf "/tmp/${tt_tgz}" -C /tmp/trusttunnel
    tt_bin=$(find /tmp/trusttunnel -type f -name trusttunnel_endpoint | head -n1)
    mv "$tt_bin" "${DEST}/trusttunnel-linux-${ARCH}"
    chmod +x "${DEST}/trusttunnel-linux-${ARCH}"
    rm -rf /tmp/trusttunnel "/tmp/${tt_tgz}"
fi

if ! have "trusttunnel-client-linux-${ARCH}"; then
    case "$ARCH" in
        amd64) ttc_tgz="trusttunnel_client-v1.1.5-linux-x86_64.tar.gz" ;;
        arm64) ttc_tgz="trusttunnel_client-v1.1.5-linux-aarch64.tar.gz" ;;
        *) echo "no trusttunnel-client for ${ARCH}" >&2; exit 1 ;;
    esac
    fetch -O "/tmp/${ttc_tgz}" "https://github.com/TrustTunnel/TrustTunnelClient/releases/download/v1.1.5/${ttc_tgz}"
    mkdir -p /tmp/ttclient
    tar -xzf "/tmp/${ttc_tgz}" -C /tmp/ttclient
    ttc_bin=$(find /tmp/ttclient -type f -name trusttunnel_client | head -n1)
    mv "$ttc_bin" "${DEST}/trusttunnel-client-linux-${ARCH}"
    chmod +x "${DEST}/trusttunnel-client-linux-${ARCH}"
    rm -rf /tmp/ttclient "/tmp/${ttc_tgz}"
fi

if ! have "anytls-linux-${ARCH}"; then
    overlay="${ROOT}/third_party/patches/anytls-server-main.go.overlay"
    git init -q /tmp/anytls
    git -C /tmp/anytls remote add origin https://github.com/anytls/anytls-go.git
    git -C /tmp/anytls fetch -q --depth 1 origin v0.0.13
    git -C /tmp/anytls checkout -q FETCH_HEAD
    cp "$overlay" /tmp/anytls/cmd/server/main.go
    (
        cd /tmp/anytls
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/anytls-linux-${ARCH}" ./cmd/server
    )
    chmod +x "${DEST}/anytls-linux-${ARCH}"
    rm -rf /tmp/anytls
fi
