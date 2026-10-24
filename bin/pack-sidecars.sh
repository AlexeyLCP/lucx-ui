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
        fetch -O /tmp/caddy-naive.tar.xz "https://github.com/klzgrad/forwardproxy/releases/download/v2.11.7-naive2/caddy-forwardproxy-naive.tar.xz"
        tar -xJf /tmp/caddy-naive.tar.xz -C /tmp
        mv /tmp/caddy-forwardproxy-naive/caddy "${DEST}/caddy-naive-linux-amd64"
        rm -rf /tmp/caddy-forwardproxy-naive /tmp/caddy-naive.tar.xz
    else
        # xcaddy path builds against the caddy v2.11.7 TAG; forwardproxy
        # e747854e (naive2) calls caddyhttp.IdleTimeoutsFromContext which
        # exists only on caddy main (post-2.11.7), so xcaddy builds use the
        # v2.11.7-naive commit. The amd64 prebuilt tarball above is naive2.
        # Revisit when caddy tags a release carrying that symbol.
        go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.7
        CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" "$(go env GOPATH)/bin/xcaddy" build v2.11.7 \
            --with github.com/caddyserver/forwardproxy=github.com/klzgrad/forwardproxy@c096d6a00cb28e019cc1995b04bdc6a9311d8024 \
            --output "${DEST}/caddy-naive-linux-${ARCH}"
    fi
    chmod +x "${DEST}/caddy-naive-linux-${ARCH}"
fi

# PIN: caddy-naive = v2.11.7-naive2 (eMudhra block + read_body_idle stream fix).
# The naive2 tag points at forwardproxy e747854e; pair the client with -5+.

if ! have "naive-client-linux-${ARCH}"; then
    case "$ARCH" in
        amd64) naive_xz="naiveproxy-v154.0.8037.49-5-linux-x64.tar.xz" ;;
        arm64) naive_xz="naiveproxy-v154.0.8037.49-5-linux-arm64.tar.xz" ;;
        *) echo "no naive-client for ${ARCH}" >&2; exit 1 ;;
    esac
    fetch -O "/tmp/${naive_xz}" "https://github.com/klzgrad/naiveproxy/releases/download/v154.0.8037.49-5/${naive_xz}"
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

# OpenFlux exit (p1neappleXpress/OpenFlux). PIN ba31d0fc — not unpinned main.
if ! have "openflux-linux-${ARCH}"; then
    git init -q /tmp/openflux
    git -C /tmp/openflux remote add origin https://github.com/p1neappleXpress/OpenFlux.git
    git -C /tmp/openflux fetch -q --depth 1 origin ba31d0fc38b9b227231056a5ef845f7eae08f42c
    git -C /tmp/openflux checkout -q FETCH_HEAD
    (
        cd /tmp/openflux
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/openflux-linux-${ARCH}" .
    )
    chmod +x "${DEST}/openflux-linux-${ARCH}"
    rm -rf /tmp/openflux
fi

# CSQTT rust-server (amurcanov/csqtt). PIN v2.5.0 (fd936ef0) — not unpinned master.
# Needs rustc 1.97.1 + zig + cargo-zigbuild (release.yml installs them).
if ! have "csqtt-linux-${ARCH}"; then
    case "$ARCH" in
        amd64) csqtt_tgt="x86_64-unknown-linux-musl" ;;
        arm64) csqtt_tgt="aarch64-unknown-linux-musl" ;;
        *) echo "no csqtt for ${ARCH}" >&2; exit 1 ;;
    esac
    command -v cargo >/dev/null
    command -v rustup >/dev/null
    command -v zig >/dev/null
    cargo zigbuild --help >/dev/null
    git init -q /tmp/csqtt
    git -C /tmp/csqtt remote add origin https://github.com/amurcanov/csqtt.git
    git -C /tmp/csqtt fetch -q --depth 1 origin fd936ef087c34cc46f26111dc2361f43b8b03c07
    git -C /tmp/csqtt checkout -q FETCH_HEAD
    (
        cd /tmp/csqtt/rust-server
        rustup toolchain install 1.97.1 --profile minimal
        rustup target add "${csqtt_tgt}" --toolchain 1.97.1
        cargo +1.97.1 zigbuild --release --target "${csqtt_tgt}"
        cp "target/${csqtt_tgt}/release/csqtt" "${DEST}/csqtt-linux-${ARCH}"
    )
    chmod +x "${DEST}/csqtt-linux-${ARCH}"
    rm -rf /tmp/csqtt
fi

rm -f "${DEST}/qwdtt-linux-${ARCH}"
if ! have "qwdtt-linux-${ARCH}"; then
    git init -q /tmp/qwdtt
    git -C /tmp/qwdtt remote add origin https://github.com/SpaceNeuroX/proxy-turn-vk-android.git
    git -C /tmp/qwdtt fetch -q --depth 1 origin a296c57eaba69bb9479a24f9157856490890e47d
    git -C /tmp/qwdtt checkout -q FETCH_HEAD
    git -C /tmp/qwdtt apply "${ROOT}/third_party/patches/qwdtt-subnet.patch"
    (
        cd /tmp/qwdtt
        GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "${DEST}/qwdtt-linux-${ARCH}" ./server
    )
    chmod +x "${DEST}/qwdtt-linux-${ARCH}"
    rm -rf /tmp/qwdtt
fi

if ! have "mieru-linux-${ARCH}" || ! have "mieru-client-linux-${ARCH}"; then
    git clone --depth 1 --branch v3.39.0 https://github.com/enfein/mieru.git /tmp/mieru
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
        amd64) tt_tgz="trusttunnel-v1.3.0-beta.2-linux-x86_64.tar.gz" ;;
        arm64) tt_tgz="trusttunnel-v1.3.0-beta.2-linux-aarch64.tar.gz" ;;
        *) echo "no trusttunnel for ${ARCH}" >&2; exit 1 ;;
    esac
    fetch -O "/tmp/${tt_tgz}" "https://github.com/TrustTunnel/TrustTunnel/releases/download/v1.3.0-beta.2/${tt_tgz}"
    mkdir -p /tmp/trusttunnel
    tar -xzf "/tmp/${tt_tgz}" -C /tmp/trusttunnel
    tt_bin=$(find /tmp/trusttunnel -type f -name trusttunnel_endpoint | head -n1)
    mv "$tt_bin" "${DEST}/trusttunnel-linux-${ARCH}"
    chmod +x "${DEST}/trusttunnel-linux-${ARCH}"
    rm -rf /tmp/trusttunnel "/tmp/${tt_tgz}"
fi

if ! have "trusttunnel-client-linux-${ARCH}"; then
    case "$ARCH" in
        amd64) ttc_tgz="trusttunnel_client-v1.3.0-rc.2-linux-x86_64.tar.gz" ;;
        arm64) ttc_tgz="trusttunnel_client-v1.3.0-rc.2-linux-aarch64.tar.gz" ;;
        *) echo "no trusttunnel-client for ${ARCH}" >&2; exit 1 ;;
    esac
    fetch -O "/tmp/${ttc_tgz}" "https://github.com/TrustTunnel/TrustTunnelClient/releases/download/v1.3.0-rc.2/${ttc_tgz}"
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

# Caddy L4 SNI mux — merged build: l4 routing + forwardproxy (naive/cover
# sites live inside the gateway Caddyfile) + caddylucx l4chan bridge.
# PIN: caddy v2.11.7 (caddy-l4 4ad6aec1 requires it; #476). l4 post-v0.1.2
# fixes: UDP session goroutine leak on shutdown, hijacked-conn prefetch pool,
# proxy_protocol policy pass-through, half_close_timeout, TLS handshake_timeout.
# forwardproxy stays on c096d6a0 (v2.11.7-naive): e747854e needs the caddy-main
# symbol caddyhttp.IdleTimeoutsFromContext, absent from the v2.11.7 tag.
if ! have "caddy-layer4-linux-${ARCH}"; then
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.7
    CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" "$(go env GOPATH)/bin/xcaddy" build v2.11.7 \
        --with github.com/mholt/caddy-l4@4ad6aec1ccc6711df47ca444d9b5364b79ea3170 \
        --with github.com/caddyserver/forwardproxy=github.com/klzgrad/forwardproxy@c096d6a00cb28e019cc1995b04bdc6a9311d8024 \
        --with "github.com/lucx-ui/caddylucx=${ROOT}/caddylucx" \
        --output "${DEST}/caddy-layer4-linux-${ARCH}"
    chmod +x "${DEST}/caddy-layer4-linux-${ARCH}"
fi
