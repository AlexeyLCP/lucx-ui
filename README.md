<!-- LUCX-HOOK: LucX-UI fork README — Streamlined EN README. Keep in sync with LICENSING.md and AGENTS.md. -->
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

## 🌟 About LucX-UI

**LucX-UI** is an advanced multi-protocol web control panel for managing [Xray-core](https://github.com/XTLS/Xray-core) servers, built as an enhanced fork of [3x-ui](https://github.com/MHSanaei/3x-ui) with native **AmneziaWG (AWG)** integration.

The project adds censorship-resistant AmneziaWG support as a kernel-interface sidecar mirroring the upstream MTProto architecture. It provides fine-grained obfuscation presets, browser TLS fingerprint mimicry, client mode (AWG Outbounds), in-panel diagnostics, and dual routing modes (Kernel NAT & Route through Xray) while maintaining full compatibility with upstream 3x-ui updates.

### 🛡️ AmneziaWG (AWG) Features
- **AWG Inbounds & Outbounds** — Kernel sidecar (`awg-quick`), client mode dial-out to upstream AWG servers (`awgo-{id}`), 10-second automatic reconcile loop, and DKMS kernel module builder.
- **Advanced Obfuscation** — Lite/Standard/Pro presets (Jc/Jmin/Jmax/S1–S4/H1–H4), CPS packet mimicry (TLS, DNS, SIP, QUIC), and browser TLS fingerprints (Chrome, Firefox, Safari).
- **Live Signature Capture** — Convert real QUIC handshakes from front domains into I1–I5 obfuscation parameters.
- **Routing & Diagnostics** — Dual routing modes (Kernel NAT and Route through Xray with policy routing & sniffing) + one-click in-panel diagnostics.

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

## 📜 License & Terms

This project is published under **two licenses** (details in [LICENSING.md](LICENSING.md)):

| Component | License |
|---|---|
| Original 3x-ui codebase | **GPL-3.0** |
| LucX-UI components (`internal/awg/`, `internal/lucx/`, frontend) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Acknowledgements & Credits

- **Testers & Contributors:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, the **3x-ui team**.
- **Projects & Inspiration:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Support the project

LucX-UI is free for personal use. You can support ongoing development:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ Stargazers over Time

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
