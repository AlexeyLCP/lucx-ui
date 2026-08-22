#!/bin/bash
# Copyright (c) 2026 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

set -euo pipefail

ARCH="${ARCH:-amd64}"
TAG="${SOURCECRAFT_COMMIT_REF_NAME:-${GITHUB_REF_NAME:-}}"
SHA="${SOURCECRAFT_COMMIT_SHA:-${GITHUB_SHA:-}}"
OUT="${OUT:-x-ui-linux-${ARCH}.tar.gz}"
CURL_RETRY="--retry 5 --retry-all-errors --retry-delay 3"

fetch() {
    wget -q --tries=5 --waitretry=10 --retry-on-http-error=429,500,502,503 "$@"
}

if [[ ! -f main.go || ! -d frontend ]]; then
    echo "run from the repo root" >&2
    exit 1
fi

echo "Building frontend"
(
    cd frontend
    npm ci
    npm run build
)
if [[ ! -d internal/web/dist ]]; then
    echo "frontend dist missing" >&2
    exit 1
fi

export CGO_ENABLED=1
export GOOS=linux
export GOARCH="$ARCH"

if ! command -v gcc >/dev/null 2>&1; then
    sudo apt-get update -qq
    sudo apt-get install -y -qq gcc libc6-dev
fi

LDFLAGS="-w -s"
if [[ "$TAG" == v* && "$TAG" != "dev-latest" ]]; then
    TAG_LUCX="${TAG##*-}"
    SRC_LUCX=$(grep -oP 'var lucxVersion = "\Klucx\.\d+(?=")' internal/config/config.go)
    if [[ -z "$TAG_LUCX" || "$TAG_LUCX" != "$SRC_LUCX" ]]; then
        echo "Tag suffix '${TAG_LUCX}' != lucxVersion '${SRC_LUCX}'" >&2
        exit 1
    fi
    LDFLAGS="$LDFLAGS -X github.com/mhsanaei/3x-ui/v3/internal/config.lucxVersion=${TAG_LUCX}"
elif [[ -n "$SHA" ]]; then
    LDFLAGS="$LDFLAGS -X github.com/mhsanaei/3x-ui/v3/internal/config.buildCommit=${SHA::8} -X github.com/mhsanaei/3x-ui/v3/internal/config.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

echo "Building panel"
go build -ldflags "$LDFLAGS" -o xui-release -v main.go

rm -rf x-ui
mkdir -p x-ui/bin
cp xui-release x-ui/x-ui
chmod +x x-ui/x-ui
cp x-ui.service.debian x-ui.service.arch x-ui.service.rhel x-ui.sh x-ui/
cp bin/install-awg-module.sh x-ui/bin/

cd x-ui/bin
Xray_URL="https://github.com/XTLS/Xray-core/releases/download/v26.7.28/"
fetch "${Xray_URL}Xray-linux-64.zip"
unzip -qo Xray-linux-64.zip
rm -f Xray-linux-64.zip geoip.dat geosite.dat
mv xray "xray-linux-${ARCH}"
fetch https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
fetch https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
fetch -O geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
fetch -O geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
fetch -O geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
fetch -O geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
fetch -O geoip_ROSCOM.dat https://github.com/hydraponique/roscomvpn-geoip/releases/latest/download/geoip.dat
fetch -O geosite_ROSCOM.dat https://github.com/hydraponique/roscomvpn-geosite/releases/latest/download/geosite.dat

MTG_MULTI_VER=$(curl -sf $CURL_RETRY -o /dev/null -w '%{redirect_url}' "https://github.com/mhsanaei/mtg-multi/releases/latest" | sed -n 's#.*/releases/tag/##p')
if [[ -z "$MTG_MULTI_VER" ]]; then
    echo "could not resolve mtg-multi tag" >&2
    exit 1
fi
MTG_PKG="mtg-multi-${MTG_MULTI_VER#v}-linux-${ARCH}"
curl -sfLRO $CURL_RETRY "https://github.com/mhsanaei/mtg-multi/releases/download/${MTG_MULTI_VER}/${MTG_PKG}.tar.gz"
tar -xzf "${MTG_PKG}.tar.gz"
mv "${MTG_PKG}/mtg-multi" "mtg-linux-${ARCH}"
rm -rf "${MTG_PKG}" "${MTG_PKG}.tar.gz"

CADDY_NAIVE_TAG="v2.11.2-naive"
fetch -O caddy-naive.tar.xz "https://github.com/klzgrad/forwardproxy/releases/download/${CADDY_NAIVE_TAG}/caddy-forwardproxy-naive.tar.xz"
tar -xJf caddy-naive.tar.xz
mv caddy-forwardproxy-naive/caddy caddy-naive-linux-amd64
rm -rf caddy-forwardproxy-naive caddy-naive.tar.xz

NAIVE_CLIENT_TAG="v150.0.7871.63-1"
NAIVE_CLIENT_XZ="naiveproxy-${NAIVE_CLIENT_TAG}-linux-x64.tar.xz"
fetch -O "${NAIVE_CLIENT_XZ}" "https://github.com/klzgrad/naiveproxy/releases/download/${NAIVE_CLIENT_TAG}/${NAIVE_CLIENT_XZ}"
mkdir -p /tmp/naiveclient
tar -xJf "${NAIVE_CLIENT_XZ}" -C /tmp/naiveclient
NAIVE_CLIENT_BIN=$(find /tmp/naiveclient -type f -name naive | head -n1)
mv "$NAIVE_CLIENT_BIN" "naive-client-linux-${ARCH}"
chmod +x "naive-client-linux-${ARCH}"
rm -rf /tmp/naiveclient "${NAIVE_CLIENT_XZ}"

OLCRTC_REF="3339cd36716885e583429f97e73462cde4984e2e"
git init -q /tmp/olcrtc
git -C /tmp/olcrtc remote add origin https://github.com/openlibrecommunity/olcrtc.git
git -C /tmp/olcrtc fetch -q --depth 1 origin "${OLCRTC_REF}"
git -C /tmp/olcrtc checkout -q FETCH_HEAD
(
    cd /tmp/olcrtc
    GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "/tmp/olcrtc-linux-${ARCH}" ./cmd/olcrtc
)
mv "/tmp/olcrtc-linux-${ARCH}" "olcrtc-linux-${ARCH}"
rm -rf /tmp/olcrtc

EX3_TAG="v1.0"
EX3_TGZ="x-ui-linux-${ARCH}.tar.gz"
fetch -O "${EX3_TGZ}" "https://github.com/Bebrik2283555/Ex3-ui/releases/download/${EX3_TAG}/${EX3_TGZ}"
tar -xzf "${EX3_TGZ}" "x-ui/bin/extra-qwdtt"
mv x-ui/bin/extra-qwdtt "qwdtt-linux-${ARCH}"
chmod +x "qwdtt-linux-${ARCH}"
rm -rf x-ui "${EX3_TGZ}"

MIERU_REF="v3.35.0"
git clone --depth 1 --branch "${MIERU_REF}" https://github.com/enfein/mieru.git /tmp/mieru
(
    cd /tmp/mieru
    GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "/tmp/mieru-linux-${ARCH}" ./cmd/mita
    GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags="-s -w" -o "/tmp/mieru-client-linux-${ARCH}" ./cmd/mieru
)
mv "/tmp/mieru-linux-${ARCH}" "mieru-linux-${ARCH}"
mv "/tmp/mieru-client-linux-${ARCH}" "mieru-client-linux-${ARCH}"
chmod +x "mieru-linux-${ARCH}" "mieru-client-linux-${ARCH}"
rm -rf /tmp/mieru

TT_VER="v1.0.33"
TT_CLIENT_VER="v1.0.49"
TT_TGZ="trusttunnel-${TT_VER}-linux-x86_64.tar.gz"
TT_CLIENT_TGZ="trusttunnel_client-${TT_CLIENT_VER}-linux-x86_64.tar.gz"
fetch -O "${TT_TGZ}" "https://github.com/TrustTunnel/TrustTunnel/releases/download/${TT_VER}/${TT_TGZ}"
mkdir -p /tmp/trusttunnel
tar -xzf "${TT_TGZ}" -C /tmp/trusttunnel
TT_BIN=$(find /tmp/trusttunnel -type f -name trusttunnel_endpoint | head -n1)
mv "$TT_BIN" "trusttunnel-linux-${ARCH}"
chmod +x "trusttunnel-linux-${ARCH}"
rm -rf /tmp/trusttunnel "${TT_TGZ}"
fetch -O "${TT_CLIENT_TGZ}" "https://github.com/TrustTunnel/TrustTunnelClient/releases/download/${TT_CLIENT_VER}/${TT_CLIENT_TGZ}"
mkdir -p /tmp/ttclient
tar -xzf "${TT_CLIENT_TGZ}" -C /tmp/ttclient
TT_CLIENT_BIN=$(find /tmp/ttclient -type f -name trusttunnel_client | head -n1)
mv "$TT_CLIENT_BIN" "trusttunnel-client-linux-${ARCH}"
chmod +x "trusttunnel-client-linux-${ARCH}"
rm -rf /tmp/ttclient "${TT_CLIENT_TGZ}"

cd ../..
tar -zcvf "$OUT" x-ui
echo "Wrote $OUT"
