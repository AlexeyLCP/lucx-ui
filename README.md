<!-- LUCX-HOOK: LucX-UI fork README — Streamlined EN README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Advanced Xray & AmneziaWG control panel** — with unified subscriptions, multi-server management and native AWG support.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
</p>

<p align="center">
  <b>English</b> |
  <a href="README.ru_RU.md">Русский</a> |
  <a href="README.fa_IR.md">فارسی</a> |
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **For personal, non-commercial, scientific, research, and educational use only.** Commercial use — including VPN resale or paid panels — requires explicit written permission under PolyForm Noncommercial 1.0.0.

---

## ⚡ Quick Start

One-line installation on **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch, etc.)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ Advanced Installation & Configuration (Cloud-Init, Docker, PostgreSQL, Env Vars)</b></summary>

### Non-Interactive Install (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
Credentials are saved to `/etc/x-ui/install-result.env`.

### Docker with PostgreSQL
```bash
docker compose --profile postgres up -d
```

### Key Environment Variables (`/etc/default/x-ui`)
| Variable | Description | Default |
| --- | --- | --- |
| `XUI_DB_TYPE` | Database engine (`sqlite` or `postgres`) | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL DSN | — |
| `XUI_ENABLE_FAIL2BAN` | Enable Fail2ban IP-limiting | `true` |
| `XUI_LOG_LEVEL` | Log level (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ Why LucX-UI?

[3x-ui](https://github.com/MHSanaei/3x-ui) is an excellent multi-protocol panel with a modern React 19 + Ant Design 6 frontend. LucX-UI keeps everything 3x-ui offers and adds **native AmneziaWG (AWG)** — a censorship-resistant WireGuard fork — which 3x-ui does not have:

| Feature | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (kernel sidecar via `awg-quick`) | ✗ | ✓ |
| AWG CPS obfuscation (TLS / DNS / SIP / QUIC + browser fingerprints) | ✗ | ✓ |
| AWG outbound — VPN chaining to upstream AWG servers (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| Client config version presets (1.5 / 2 / 3) | ✗ | ✓ |
| In-panel AWG diagnostics (routing / NAT / peers / handshakes) | ✗ | ✓ |
| NaiveProxy tunnel sidecar (Caddy + forward_proxy, supervised) | ✗ | ✓ |
| Per-client NaiveProxy credentials + `naive+https://` in subscriptions | ✗ | ✓ |
| NaiveProxy → Xray routing (SOCKS loopback bridge, optional) | ✗ | ✓ |
| olcRTC tunnel sidecar (WebRTC via meet rooms, supervised) | ✗ | ✓ |
| Smart Cluster outbound links | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 frontend | ✓ | ✓ (inherited) |
| All Xray protocols (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Frictionless upstream sync (LUCX-HOOK isolation, 49 files) | — | ✓ |

A kernel sidecar (like 3x-ui's MTProto `mtg`) means AWG runs as a real kernel interface — not a userspace shim — so Xray routes decrypted traffic through its own TUN inbound, giving you the full routing, sniffing and domain-rule power of Xray on AWG traffic.

---

## 🌟 About LucX-UI

**LucX-UI** is an enhanced fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (currently synced to upstream **v3.6.0**) that adds native **AmneziaWG (AWG)** support as a kernel-interface sidecar, mirroring upstream's MTProto architecture. It keeps 100% upstream compatibility through strict `LUCX-HOOK` code isolation.

### 🛡️ AmneziaWG (AWG) Features
- **AWG Inbounds & Outbounds** — Kernel sidecar (`awg-quick`), client mode dial-out to upstream AWG servers (`awgo-{id}`), 10-second automatic reconcile loop, and DKMS kernel module builder.
- **Advanced Obfuscation** — Lite/Standard/Pro presets (Jc/Jmin/Jmax/S1–S4/H1–H4), CPS packet mimicry (TLS, DNS, SIP, QUIC), and browser TLS fingerprints (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — AmneziaWG 3 header protection with auto-generated 32-byte keys; server-side version ceiling gates feature emission per client.
- **Client Version Presets** — Generate client configs for AWG 1.5 / 2 / 3 from a single inbound — pick the format your client app understands.
- **Live Signature Capture** — Convert real QUIC handshakes from front domains into I1–I5 obfuscation parameters.
- **Routing & Diagnostics** — Dual routing modes (Kernel NAT and Route through Xray with policy routing & sniffing) + one-click in-panel diagnostics.

### 🚇 Tunnel Sidecars (NaiveProxy, olcRTC)
- **NaiveProxy** — Caddy with the `forward_proxy` plugin ([klzgrad](https://github.com/klzgrad/forwardproxy) fork, HTTP/2 padding) runs as a panel-supervised sidecar: rendered Caddyfile, start/stop/restart with crash-reviving reconcile, and a three-level health probe (process → TCP → TLS).
- **Per-client credentials** — every enabled panel client automatically gets a personal `basic_auth` pair (derived from the panel secret, nothing stored); disabling a client revokes it on the next reconcile.
- **Subscriptions** — each client's subscription carries their personal `naive+https://` link alongside their Xray/AWG links (standard format for NekoBox / husi / Exclave), plus a QR code and a strong-password generator in the panel.
- **Panel UX** — Auto TLS (Let's Encrypt) or your own cert/key, raw-Caddyfile mode with `caddy adapt` validation, Caddyfile preview, process logs, binary upload/download.
- **Route through Xray (optional)** — toggle makes Caddy dial destinations via a hidden loopback SOCKS bridge (`upstream socks5://127.0.0.1:…`, native forward_proxy — no binary patch) tagged `lucx-tunnel-naive`, so NaiveProxy traffic gets full Xray routing / sniffing / domain rules (same pattern as MTProto). Default stays direct egress.
- **olcRTC** — TCP-over-WebRTC tunnel via a legal video-call room ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc), WTFPL): Jitsi / Yandex Telemost / WB Stream. No public ports on the VPS — the binary joins the room as a silent participant. Panel renders server YAML, supervises the process, and exposes a copyable `olcrtc://` connect URI for owenclave / olcbox clients.

### 🚀 Core 3x-ui Features
- **Protocols:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Transports & Security:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Management:** Traffic quotas, IP limits (Fail2ban), live online status, subscriptions, Telegram bot, REST API, Multi-node support, SQLite / PostgreSQL.

<details>
<summary><b>📸 Screenshots</b></summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

</details>

---

## 🔄 Migration from 3x-ui

LucX-UI shares the same Xray-core / SQLite (or PostgreSQL) database schema base as 3x-ui, and AWG tables are created automatically on first run. To install over an existing 3x-ui setup, back up your database first and run the standard install command:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

The AWG kernel module is built automatically by the installer (`bin/install-awg-module.sh`, DKMS). After install, run `x-ui` in the console to confirm AMW kernel module version and start adding AWG inbounds from the panel.

---

## 📜 License & Terms

This project is published under **two licenses** (details in [LICENSING.md](LICENSING.md)):

| Component | License |
|---|---|
| Original 3x-ui codebase | **GPL-3.0** |
| LucX-UI components (`internal/awg/`, `internal/lucx/`, frontend) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Acknowledgements & Credits

- **Testers & Contributors:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, the **3x-ui team**.
- **Projects & Inspiration:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) & [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) (NaiveProxy tunnel sidecar), [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) (olcRTC core), [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) (Caddyfile integration design reference), [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) (tunnel-sidecar concept reference: qWDTT / olcRTC panel integration), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Support the project

LucX-UI is free for personal use. You can support ongoing development:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ For Developers

<details>
<summary><b>Architecture, build & upstream sync (click to expand)</b></summary>

**Architecture & isolation rule.** All LucX code lives in isolated packages (`internal/awg/`, `internal/lucx/`); changes to upstream 3x-ui files go only inside `// LUCX-HOOK` / `// END LUCX-HOOK` markers so that every upstream release is a near-trivial port. See [AGENTS.md](AGENTS.md) for the full architecture map, the 10 rules, known issues and debug patterns.

**Build from source** (requires Go 1.23+, Node.js 20+, gcc — Linux only, CGO for SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# pre-push hygiene: bin/check-lucx.sh  (gofumpt on the 49 LucX-owned files)
```

**Upstream sync procedure** (validated v3.5.0→v3.6.0, 103 commits / 432 files / 7 conflicts):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolve block by block (see AGENTS.md Rule 8) — never blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # compare marker counts before/after to detect lost blocks
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

---

## ⭐ Stargazers over Time

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->