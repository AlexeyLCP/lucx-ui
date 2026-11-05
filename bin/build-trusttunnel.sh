#!/usr/bin/env bash
# Copyright (c) 2026 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

# LUCX: build the TrustTunnel endpoint with the LucX PROXY v1 patch
# (third_party/patches/trusttunnel-proxy-protocol.patch). The stock release
# can't read the SNI gateway's PROXY header, so behind the gateway every
# client shows as 127.0.0.1.
#
# usage: bin/build-trusttunnel.sh <amd64|arm64> <dest dir>
#
# Cross-compiles a static musl binary with cargo-zigbuild from Linux or macOS.
# Needs git, curl, python3 (for cmake/ninja/zig wheels) and a libclang for
# bindgen (Linux: apt install libclang-dev; macOS: Xcode command line tools).
# A missing Rust toolchain, cmake, ninja, zig or cargo-zigbuild is installed
# into a throwaway directory, never system-wide.
set -euo pipefail

ARCH="${1:?arch}"
DEST="${2:?dest dir}"
TT_REF="v1.3.0-beta.2"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PATCH="${ROOT}/third_party/patches/trusttunnel-proxy-protocol.patch"

case "$ARCH" in
    amd64) TARGET=x86_64-unknown-linux-musl ZT=x86_64-linux ;;
    arm64) TARGET=aarch64-unknown-linux-musl ZT=aarch64-linux ;;
    *) echo "no trusttunnel for ${ARCH}" >&2; exit 1 ;;
esac

WORK="$(mktemp -d "${TMPDIR:-/tmp}/lucx-tt.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
TOOLS="${LUCX_TT_TOOLS:-$WORK/tools}"
mkdir -p "$TOOLS" "$DEST"

# Toolchain, isolated unless already present.
if ! command -v cargo >/dev/null 2>&1; then
    export RUSTUP_HOME="$TOOLS/rustup" CARGO_HOME="$TOOLS/cargo"
    curl -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path --profile minimal --default-toolchain none
    export PATH="$CARGO_HOME/bin:$PATH"
fi
if ! command -v cmake >/dev/null 2>&1 || ! command -v ninja >/dev/null 2>&1 || ! command -v zig >/dev/null 2>&1; then
    python3 -m venv "$TOOLS/venv"
    "$TOOLS/venv/bin/pip" -q install cmake ninja ziglang
    mkdir -p "$TOOLS/venv/zigbin"
    # The ziglang wheel ships `python -m ziglang`; cargo-zigbuild wants `zig`.
    printf '#!/bin/sh\nexec "%s" -m ziglang "$@"\n' "$TOOLS/venv/bin/python3" > "$TOOLS/venv/zigbin/zig"
    chmod +x "$TOOLS/venv/zigbin/zig"
    export PATH="$TOOLS/venv/bin:$PATH"
    command -v zig >/dev/null 2>&1 || export PATH="$TOOLS/venv/zigbin:$PATH"
fi
git clone -q --depth 1 --branch "$TT_REF" https://github.com/TrustTunnel/TrustTunnel.git "$WORK/src"
git -C "$WORK/src" apply "$PATCH"
# rust-toolchain.toml in the source pins the compiler. Installed before any
# cargo call: with --default-toolchain none rustup has no default outside the
# source tree, so even `cargo install` would fail on a fresh machine.
(cd "$WORK/src" && rustup toolchain install && rustup target add "$TARGET")
if ! command -v cargo-zigbuild >/dev/null 2>&1; then
    (cd "$WORK/src" && cargo install -q --root "$TOOLS/zigbuild" cargo-zigbuild)
fi
export PATH="$TOOLS/zigbuild/bin:$PATH"

# bindgen (boring-sys) parses BoringSSL headers for the target with the
# host libclang; point it at zig's musl headers.
ZIG_INC="$(zig env 2>/dev/null | sed -n 's/.*lib_dir = "\(.*\)".*/\1/p; s/.*"lib_dir": *"\(.*\)".*/\1/p' | head -n1)/libc/include"
if [[ -z "${LIBCLANG_PATH:-}" && "$(uname)" == Darwin ]]; then
    export LIBCLANG_PATH=/Library/Developer/CommandLineTools/usr/lib
fi
target_env="$(echo "$TARGET" | tr '-' '_')"
export "BINDGEN_EXTRA_CLANG_ARGS_${target_env}=--target=${TARGET} -isystem ${ZIG_INC}/${ZT}-musl -isystem ${ZIG_INC}/generic-musl -isystem ${ZIG_INC}/${ZT}-any -isystem ${ZIG_INC}/any-linux-any"
export TRUSTTUNNEL_VERSION="${TT_REF#v}" CARGO_PROFILE_RELEASE_DEBUG=false

(cd "$WORK/src" && cargo zigbuild --release --target "$TARGET" -p trusttunnel_endpoint)
install -m 755 "$WORK/src/target/${TARGET}/release/trusttunnel_endpoint" "${DEST}/trusttunnel-linux-${ARCH}"
grep -aq '+lucx.proxy' "${DEST}/trusttunnel-linux-${ARCH}" || { echo "build lacks the LucX patch" >&2; exit 1; }
echo "built ${DEST}/trusttunnel-linux-${ARCH}"
