<!-- LUCX-HOOK: LucX-UI fork README — EN lead section, license, credits, sources. Keep in sync with LICENSING.md and AGENTS.md. -->
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

**LucX-UI** is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) with native **AmneziaWG (AWG)** support. AWG runs as a kernel-interface sidecar — mirroring the exact architecture upstream uses for MTProto (mtg): the panel owns the lifecycle and traffic accounting, and Xray can optionally route the traffic.

### What we added — and what works

- ✅ **AWG inbounds** — kernel sidecar on `awg-quick`: creation, 10-second reconcile, orphan sweep, DKMS kernel-module installer.
- ✅ **AWG outbounds (client mode)** — the panel dials out to an upstream AmneziaWG server: its own tab under Xray, paste an existing `.conf`, and a kernel interface `awgo-{id}` kept in step by the reconcile loop. A `freedom` outbound with `sockopt.interface` is injected into the Xray config, so routing rules and balancers can send traffic through the upstream VPN.
- ✅ **Obfuscation** — Lite/Standard/Pro presets (Jc/Jmin/Jmax/S1–S4/H1–H4) and CPS packet mimicry: TLS, DNS, SIP, QUIC.
- ✅ **Browser TLS fingerprints** — Chrome (GREASE), Firefox 120+ (NSS ordering, padding), Safari 16+ (Apple ordering, TLS 1.1). For TLS and QUIC.
- ✅ **Live signature capture** — a real QUIC handshake from a front domain becomes your I1–I5.
- ✅ **Clients** — QR codes, `.conf` download, per-peer traffic accounting (`awg show transfer`).
- ✅ **Two routing modes:**
  - **Kernel NAT** — plain kernel forwarding; NAT rules self-heal via the reconcile loop after iptables flushes.
  - **Route through Xray** — traffic flows through Xray's full routing pipeline (domain/geosite rules, balancers, chained outbounds) via a TUN inbound with policy routing and sniffing.
- ✅ **In-panel diagnostics** — one button in the inbound form: interface, ip_forward, peers/handshakes, NAT/TUN rules — the breakage point is immediately visible.
- ✅ **Battle-tested** on testers' VPSs: handshake, ICMP, HTTPS, traffic accounting, cascades, both routing modes.

### Install

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Installs the panel from the [latest release](https://github.com/AlexeyLCP/lucx-ui/releases/latest), the systemd unit, Xray-core and mtg (from the upstream 3x-ui release), and builds the AmneziaWG kernel module via DKMS (`bin/install-awg-module.sh`).

### License

This project is under **two licenses** (details in [LICENSING.md](LICENSING.md)):

| Part | License |
|---|---|
| Original 3x-ui code | **GPL-3.0** (as required by upstream) |
| LucX components (`internal/awg/`, `internal/lucx/`, AWG frontend, scripts) | **PolyForm Noncommercial 1.0.0** |

In practice: **free** for personal, non-commercial, scientific, research, and educational use — run as many panels as you like. **Commercial use** (VPN resale, paid services built on this code, embedding into a commercial product) requires explicit written permission from the author — open an [issue](https://github.com/AlexeyLCP/lucx-ui/issues) or contact the repository owner. Per-file `SPDX-License-Identifier` headers make the boundary unambiguous: no header means GPL-3.0.

### Acknowledgements

- **VladufQa** — live-server testing (ruvds): first handshakes, traffic, cascades, routing bug reports.
- **Kirill Rudenko** — testing (runode) and **PR #13**: AWG needRestart, iif policy routing, per-inbound tables/gateways, reconcile route-ensure, sniffing — the work that made "Route through Xray" actually function.
- **302ba (Alex)** — **PR #24**: fix for client fields loss during Zod schema parsing.
- The **3x-ui** team — for an excellent base and the sidecar architecture we mirror.

### Credits: ideas and code

- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — fork base (GPL-3.0), the MTProto sidecar architecture we mirror.
- [AmneziaVPN](https://github.com/amnezia-vpn) — the AmneziaWG protocol itself and the kernel module.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — PostUp NAT pattern (MASQUERADE + FORWARD), crypto-lib-free QUIC Initial generators, DKMS install approach.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — the QUIC signature capture we ported (`internal/awg/signature/`), and the TLS-incompatibility warning.
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) and [refraction-networking/utls](https://github.com/refraction-networking/utls) — representative Firefox/Safari TLS profiles behind our ClientHello presets.

### ☕ Support the project

LucX-UI is free for personal and non-commercial use. If the panel saves you time, you can support development:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Donations are a thank-you, not a purchase: they do not grant a commercial license and do not change the terms in [LICENSING.md](LICENSING.md).

---

*Everything below is the upstream **3x-ui** documentation, kept intact for reference. LucX-UI tracks upstream releases via migration (not rebase).*

<!-- END LUCX-HOOK -->

[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
</p>

**3X-UI** is an advanced, open-source web control panel for managing [Xray-core](https://github.com/XTLS/Xray-core) servers. It provides a clean, multi-language interface for deploying, configuring, and monitoring a wide range of proxy and VPN protocols — from a single VPS to multi-node deployments.

Built as an enhanced fork of the original X-UI project, 3X-UI adds broader protocol support, improved stability, per-client traffic accounting, and many quality-of-life features.

> [!IMPORTANT]
> This project is intended for personal use only. Please do not use it for illegal purposes or in a production environment.

## Features

- **Multi-protocol inbounds** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel, and TUN.
- **Modern transports & security** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade, and XHTTP, secured with TLS, XTLS, and REALITY.
- **Fallbacks** — serve multiple protocols on a single port (e.g. VLESS and Trojan on 443) using Xray's fallback support.
- **Per-client management** — traffic quotas, expiry dates, IP limits, live online status, and one-click share links, QR codes, and subscriptions.
- **Traffic statistics** — per inbound, per client, and per outbound, with reset controls.
- **Multi-node support** — manage and scale across multiple servers from a single panel.
- **Outbound & routing** — WARP, NordVPN, custom routing rules, load balancers, and outbound proxy chaining.
- **Built-in subscription server** with multiple output formats and [custom page templates](docs/custom-subscription-templates.md).
- **Telegram bot** for remote monitoring and management.
- **RESTful API** with in-panel Swagger documentation.
- **Flexible storage** — SQLite (default) or PostgreSQL.
- **13 UI languages** with dark and light themes.
- **Fail2ban integration** for enforcing per-client IP limits.

## Screenshots

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
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

To install a specific version, append its tag (e.g. `v3.4.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

To install the rolling **dev** build (latest per-commit pre-release from `main`, not a stable release), pass `dev-latest`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

During installation a random username, password, and access path are generated. After installation, run `x-ui` to open the management menu, where you can start/stop the service, view or reset your login credentials, manage SSL certificates, and more.

For full documentation, please visit the [project Wiki](https://github.com/MHSanaei/3x-ui/wiki).

### Unattended install

The installer also runs **non-interactively** for cloud-init.
Set `XUI_NONINTERACTIVE=1` (or pipe with no TTY) and it installs end-to-end with
zero prompts, generating random credentials and writing them to
`/etc/x-ui/install-result.env`. See [`deploy/`](deploy/) for:

- [Cloud-init user-data](deploy/cloud-init/) — unattended install on any cloud (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Hetzner Cloud notes](deploy/marketplace/hetzner/) — cloud-init deployment on Hetzner

## Supported Platforms

**Operating systems:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine, and Windows.

**Architectures:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Database Options

3X-UI supports two backends, chosen during the install:

- **SQLite** (default) — a single file at `/etc/x-ui/x-ui.db`. Zero setup, ideal for small and medium deployments.
- **PostgreSQL** — recommended for high client counts or multi-node setups. The installer can install PostgreSQL locally for you, or accept a DSN to an existing server.

At runtime the backend is selected via environment variables (the installer writes these to `/etc/default/x-ui` for you):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Migrating an existing SQLite install to PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# then set XUI_DB_TYPE and XUI_DB_DSN in /etc/default/x-ui and restart:
systemctl restart x-ui
```

The source SQLite file is left untouched; remove it manually once you have verified the new backend.

### Docker

The default `docker compose up -d` keeps using SQLite. To run with the bundled PostgreSQL service, uncomment the two `XUI_DB_*` env lines in `docker-compose.yml` and start with the profile:

```bash
docker compose --profile postgres up -d
```

The image bundles Fail2ban (enabled by default) to enforce per-client **IP limits**. Fail2ban bans offenders with `iptables`, which requires the `NET_ADMIN` capability. `docker-compose.yml` already grants it via `cap_add`; if you start the container with `docker run` instead, add the capabilities yourself, otherwise bans are logged but never applied:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `XUI_DB_TYPE` | Database backend: `sqlite` or `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL connection string (when `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Directory for the SQLite database file | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maximum open connections (PostgreSQL pool) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maximum idle connections (PostgreSQL pool) | — |
| `XUI_INIT_WEB_BASE_PATH` | The initial URI path for the web panel | `/` |
| `XUI_ENABLE_FAIL2BAN` | Enable Fail2ban-based IP-limit enforcement | `true` |
| `XUI_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Enable debug mode | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Enable the tunnel health monitor (probes a URL and restarts xray after repeated failures; a restart drops all clients) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Proxy the probe is sent through; point it at a local xray inbound so the probe tests the tunnel (e.g. `socks5://127.0.0.1:1080`). Empty means the probe only checks host connectivity | — |
| `XUI_TUNNEL_HEALTH_URL` | URL probed for tunnel health | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Interval between probes | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Per-probe timeout | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Consecutive failures before a restart is triggered | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Minimum delay between consecutive restarts | `5m` |

## Supported Languages

The panel UI is available in 13 languages:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Contributing

Contributions are welcome. Please read the [Contributing Guide](/CONTRIBUTING.md) before opening an issue or pull request.

## A Special Thanks to

- [alireza0](https://github.com/alireza0/)

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Community Tools

Tools and integrations built by the community around 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (License: **MIT**): _Manage inbounds, clients, panel settings, and Xray configuration as code with Terraform / OpenTofu._

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
