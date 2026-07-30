<!-- LUCX-HOOK: LucX-UI fork README — Unified EN README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

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
> **For personal, non-commercial, scientific, research, and educational use only.** Commercial use — including VPN resale, paid panels, or subscription services built on this code — requires explicit written permission from the author. Do not use for illegal purposes.

---

## About LucX-UI

**LucX-UI** is an advanced web control panel for managing [Xray-core](https://github.com/XTLS/Xray-core) servers, built as an enhanced fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) with native **AmneziaWG (AWG)** support. AWG runs as a kernel-interface sidecar — mirroring the exact architecture upstream uses for MTProto (mtg): the panel owns the lifecycle and traffic accounting, and Xray can optionally route the traffic.

### Key Features

#### 🛡️ AmneziaWG (AWG) Enhancements
- **AWG Inbounds** — Kernel sidecar via `awg-quick`: creation, 10-second reconcile, orphan interface cleanup, and DKMS kernel-module installer.
- **AWG Outbounds (Client Mode)** — Dial out to an upstream AmneziaWG server directly from the panel: dedicated tab in Xray settings, paste `.conf` files, and `awgo-{id}` kernel interfaces managed by the reconcile loop. Injects a `freedom` outbound with `sockopt.interface` into Xray config so routing rules and balancers can send traffic through an upstream VPN.
- **Obfuscation Control** — Lite/Standard/Pro presets (Jc/Jmin/Jmax/S1–S4/H1–H4) and CPS packet mimicry: TLS, DNS, SIP, and QUIC.
- **Browser TLS Fingerprints** — Chrome (GREASE), Firefox 120+ (NSS ordering, padding), and Safari 16+ (Apple ordering, TLS 1.1) for TLS and QUIC.
- **Live Signature Capture** — Convert real QUIC handshakes from front domains into I1–I5 obfuscation parameters.
- **Client Management** — QR codes, `.conf` download, and per-peer traffic accounting (`awg show transfer`).
- **Two Routing Modes**:
  - **Kernel NAT** — Direct kernel forwarding; NAT rules self-heal via the reconcile loop after iptables flushes.
  - **Route through Xray** — Traffic flows through Xray's full routing pipeline (domain/geosite rules, balancers, chained outbounds) via a TUN inbound with policy routing and sniffing.
- **In-Panel Diagnostics** — One-button probe in the inbound form checking interface UP, ip_forward, peers/handshakes, and NAT/TUN rules.

#### 🚀 Core 3x-ui Features
- **Multi-protocol inbounds** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door, and TUN.
- **Modern transports & security** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade, and XHTTP, secured with TLS, XTLS, and REALITY.
- **Fallbacks** — Serve multiple protocols on a single port (e.g. VLESS and Trojan on 443) using Xray's fallback support.
- **Per-client management** — Traffic quotas, expiry dates, IP limits, live online status, and one-click share links, QR codes, and subscriptions.
- **Traffic statistics** — Per inbound, per client, and per outbound, with reset controls.
- **Multi-node support** — Manage and scale across multiple servers from a single panel.
- **Outbound & routing** — WARP, NordVPN, custom routing rules, load balancers, and outbound proxy chaining.
- **Subscription server** with multiple output formats and custom page templates.
- **Telegram bot** for remote monitoring and management.
- **RESTful API** with in-panel Swagger documentation.
- **Flexible storage** — SQLite (default) or PostgreSQL.
- **Fail2ban integration** for enforcing per-client IP limits.

### Screenshots

<details>
<summary>Click to expand</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Quick Start

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Installs the panel from the [latest release](https://github.com/AlexeyLCP/lucx-ui/releases/latest), the systemd unit, Xray-core and mtg (from the upstream 3x-ui release), and builds the AmneziaWG kernel module via DKMS (`bin/install-awg-module.sh`).

During installation a random username, password, and access path are generated. After installation, run `x-ui` to open the management menu.

### Unattended install

The installer also runs **non-interactively** for cloud-init. Set `XUI_NONINTERACTIVE=1` and it installs end-to-end without prompts, writing credentials to `/etc/x-ui/install-result.env`. See [`deploy/`](deploy/) for cloud-init guides.

## Supported Platforms

**Operating systems:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine, and Windows.

**Architectures:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Database Options

3X-UI supports two backends, chosen during the install:

- **SQLite** (default) — a single file at `/etc/x-ui/x-ui.db`.
- **PostgreSQL** — recommended for high client counts or multi-node setups.

Environment variables in `/etc/default/x-ui`:
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Docker

To run with PostgreSQL in Docker, uncomment the `XUI_DB_*` env lines in `docker-compose.yml` and run:
```bash
docker compose --profile postgres up -d
```

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `XUI_DB_TYPE` | Database backend: `sqlite` or `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL connection string (when `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Directory for the SQLite database file | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | Enable Fail2ban-based IP-limit enforcement | `true` |
| `XUI_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Enable the tunnel health monitor | `false` |

## License & Terms

This project is published under **two licenses** (details in [LICENSING.md](LICENSING.md)):

| Component | License |
|---|---|
| Original 3x-ui code | **GPL-3.0** (as required by upstream) |
| LucX components (`internal/awg/`, `internal/lucx/`, AWG frontend, scripts) | **PolyForm Noncommercial 1.0.0** |

**Free** for personal, non-commercial, scientific, research, and educational use. **Commercial use** (VPN resale, paid panels, commercial embedding) requires explicit written permission from the author — open an [issue](https://github.com/AlexeyLCP/lucx-ui/issues) or contact the repository owner. Per-file `SPDX-License-Identifier` headers define the exact boundary: no header means GPL-3.0.

## Contributing

Contributions are welcome. Please read the [Contributing Guide](/CONTRIBUTING.md) before opening an issue or pull request.

## Acknowledgements & Credits

### Testers & Contributors
- **VladufQa** — Live VPS testing (ruvds): first handshakes, traffic, cascades, routing bug reports.
- **Kirill Rudenko** — Testing (runode) and **PR #13**: AWG needRestart, iif policy routing, per-inbound tables/gateways, reconcile route-ensure, sniffing.
- **302ba (Alex)** — **PR #24**: Fix for client fields loss during Zod schema parsing.
- **alireza0** — Upstream contributor.
- The **3x-ui team** — For an excellent base and the sidecar architecture we mirror.

### Source Credits & Inspiration
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — Fork base (GPL-3.0), MTProto sidecar architecture reference.
- [AmneziaVPN](https://github.com/amnezia-vpn) — AmneziaWG protocol and kernel module.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — PostUp NAT pattern (MASQUERADE + FORWARD), crypto-lib-free QUIC Initial generators, DKMS install approach.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — QUIC signature capture (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) & [refraction-networking/utls](https://github.com/refraction-networking/utls) — Browser TLS profiles for Firefox/Safari presets.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) & [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) — Routing rules datasets.

### Community Tools
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (License: **MIT**): Manage inbounds, clients, panel settings as code.

## ☕ Support the project

LucX-UI is free for personal and non-commercial use. If the panel saves you time, you can support development:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Donations are a thank-you, not a purchase: they do not grant a commercial license and do not change the terms in [LICENSING.md](LICENSING.md).

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)

<!-- END LUCX-HOOK -->
