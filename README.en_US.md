<!-- LUCX-HOOK: LucX-UI fork README — Streamlined EN README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Advanced Xray control panel** — AmneziaWG (kernel + native, up to 3.1), import existing AWG, supervised tunnels & sidecar outbounds (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel), Clash / Amnezia `vpn://` / Happ subscriptions, geodata browser & RoscomVPN routing.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
  <a href="https://boosty.to/alexeylcp"><img src="https://img.shields.io/badge/boosty-subscribe-orange" alt="Boosty"></a>
</p>

<p align="center">
  <b>English</b> |
  <a href="README.md">Русский</a> |
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

### Docker
Images are published on every release tag (`ghcr.io/alexeylcp/lucx-ui`):

```bash
docker run -d \
  --name lucx-ui \
  --restart unless-stopped \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  -p 2053:2053 \
  -v $PWD/db/:/etc/x-ui/ \
  ghcr.io/alexeylcp/lucx-ui:latest
```

Or `docker compose up -d` (pulls the same image; `docker compose build` builds locally).

PostgreSQL: uncomment the `XUI_DB_*` lines in `docker-compose.yml` and:

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

[3x-ui](https://github.com/MHSanaei/3x-ui) is an excellent multi-protocol panel with a modern React 19 + Ant Design 6 frontend. LucX-UI keeps everything 3x-ui offers and adds what upstream does not: **kernel AmneziaWG** (alongside upstream's native `amneziawg`), **import of existing AWG**, **tunnel sidecars** (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel), **richer subscriptions** (Clash Meta AWG, Amnezia `vpn://`, Happ), and **geodata tools** (in-panel browser + RoscomVPN packs):

| Feature | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (kernel sidecar via `awg-quick`) | ✗ | ✓ |
| Native AmneziaWG inbound (`amneziawg`, userspace) | ✓ | ✓ |
| Import existing host AWG (awg-multi / toolza3 / Docker) | ✗ | ✓ |
| Kernel AWG without module → embedded amneziawg-go | ✗ | ✓ |
| Live AWG client/inbound speed in the panel | ✗ | ✓ |
| AWG CPS obfuscation (TLS / DNS / SIP / QUIC + browser fingerprints) | ✗ | ✓ |
| AWG outbound — VPN chaining to upstream AWG servers (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1 (`RandomTrailers` / `DisableCookies` anti-DPI) | ✗ | ✓ |
| Client config version presets (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| In-panel AWG diagnostics (routing / NAT / peers / handshakes) | ✗ | ✓ |
| AWG in Clash Meta + Amnezia subscription `/awg/` (`.conf` / `vpn://`) | ✗ | ✓ |
| NaiveProxy tunnel sidecar (Caddy + forward_proxy, supervised) | ✗ | ✓ |
| Per-client NaiveProxy credentials + `naive+https://` in subscriptions | ✗ | ✓ |
| NaiveProxy → Xray routing (SOCKS loopback bridge, optional) | ✗ | ✓ |
| olcRTC tunnel sidecar (WebRTC via meet rooms, supervised) | ✗ | ✓ |
| qWDTT tunnel sidecar (WireGuard over VK TURN, supervised) | ✗ | ✓ |
| mieru tunnel sidecar (`mita`, per-client traffic, supervised) | ✗ | ✓ |
| TrustTunnel sidecar (AdGuard VPN protocol, HTTPS-like, supervised) | ✗ | ✓ |
| Sidecar outbounds (Naive / mieru / TrustTunnel client → SOCKS, routing & balancers) | ✗ | ✓ |
| Geodata browser — pick geosite/geoip categories from panel | ✗* | ✓ |
| RoscomVPN geo pack (`geoip/geosite_ROSCOM.dat`, RKN geoblock lists) | ✗ | ✓ |
| Happ routing profiles (RoscomVPN deeplink + custom) | ✗ | ✓ |
| Smart Cluster outbound links | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 frontend | ✓ | ✓ (inherited) |
| All Xray protocols (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Frictionless upstream sync (LUCX-HOOK isolation, 49 files) | — | ✓ |

\* Upstream [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) (not merged yet) — ported into LucX-UI.

A kernel sidecar (like 3x-ui's MTProto `mtg`) means AWG runs as a real kernel interface — not a userspace shim — so Xray routes decrypted traffic through its own TUN inbound, giving you the full routing, sniffing and domain-rule power of Xray on AWG traffic. No module — the same LucX `awg` inbound runs on embedded amneziawg-go. Upstream's native `amneziawg` protocol stays in the panel next to it.

---

## 🌟 About LucX-UI

**LucX-UI** is an enhanced fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (currently synced to upstream **v3.7.0**). Beyond stock Xray protocols it adds **AmneziaWG** in two modes — kernel sidecar `awg` (same idea as MTProto/`mtg`) and upstream's native `amneziawg`, up to **AWG 3.1**; **import** of awg-multi / toolza3 / Docker; panel-supervised **tunnel sidecars** (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel), extended **subscriptions** (Clash Meta AWG, Amnezia `/awg/` + `vpn://`, Happ routing), and a **geodata browser** with stock RoscomVPN lists. 100% upstream compatibility via strict `LUCX-HOOK` isolation.

### 🛡️ AmneziaWG (AWG) Features
- **AWG Inbounds & Outbounds** — Kernel sidecar (`awg-quick`), client mode dial-out to upstream AWG servers (`awgo-{id}`), 10-second automatic reconcile loop, and DKMS kernel module builder.
- **Two engines** — both `AmneziaWG (kernel)` (`awg-quick` when the module is present) and upstream's native `amneziawg`. No module — LucX `awg` inbounds run on embedded amneziawg-go (SOCKS into Xray); the kernel path is unchanged when the module is there.
- **Import existing AWG** — Inbounds banner: awg-multi / toolza3 / Docker Amnezia. Keys, IPs, port and obfuscation copied as-is; kernel iface renamed in place (handshakes stay).
- **Live speed** — Clients / Inbounds speed columns for AWG (Xray stats never see it).
- **Advanced Obfuscation** — Lite/Standard/Pro presets (Jc/Jmin/Jmax/S1–S4/H1–H4), CPS packet mimicry (TLS, DNS, SIP, QUIC), and browser TLS fingerprints (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — AmneziaWG 3 header protection with auto-generated 32-byte keys; server-side version ceiling gates feature emission per client.
- **AWG 3.1** — `RandomTrailers` (random packet tail, size-based anti-DPI) and `DisableCookies`; kernel module + tools auto-upgrade to v3.1 on panel update.
- **Client Version Presets** — Generate client configs for AWG 1.5 / 2 / 3 / 3.1 from a single inbound — pick the format your client app understands.
- **Live Signature Capture** — Convert real QUIC handshakes from front domains into I1–I5 obfuscation parameters.
- **Routing & Diagnostics** — Dual routing modes (Kernel NAT and Route through Xray with policy routing & sniffing) + one-click in-panel diagnostics.

### 🚇 Tunnel Sidecars (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel)
- **NaiveProxy** — Caddy with the `forward_proxy` plugin ([klzgrad](https://github.com/klzgrad/forwardproxy) fork, HTTP/2 padding) runs as a panel-supervised sidecar: rendered Caddyfile, start/stop/restart with crash-reviving reconcile, and a three-level health probe (process → TCP → TLS).
- **Per-client credentials** — every enabled panel client automatically gets a personal `basic_auth` pair (derived from the panel secret, nothing stored); disabling a client revokes it on the next reconcile.
- **Subscriptions** — each client's subscription carries their personal `naive+https://` link alongside their Xray/AWG links (standard format for NekoBox / husi / Exclave), plus a QR code and a strong-password generator in the panel.
- **Panel UX** — Auto TLS (Let's Encrypt) or your own cert/key, raw-Caddyfile mode with `caddy adapt` validation, Caddyfile preview, process logs, binary upload/download.
- **Route through Xray (optional)** — toggle makes Caddy dial destinations via a hidden loopback SOCKS bridge (`upstream socks5://127.0.0.1:…`, native forward_proxy — no binary patch) tagged `lucx-tunnel-naive`, so NaiveProxy traffic gets full Xray routing / sniffing / domain rules (same pattern as MTProto). Default stays direct egress.
- **olcRTC** — TCP-over-WebRTC tunnel via a legal video-call room ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc), WTFPL): Jitsi / Yandex Telemost / WB Stream. No public ports on the VPS — the binary joins the room as a silent participant. Panel renders server YAML, supervises the process, and exposes a copyable `olcrtc://` connect URI for owenclave / olcbox clients.
- **qWDTT** — WireGuard tunnelled through VK Calls TURN relays ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android), GPL-3.0 server). Requires root (TUN + NAT). Panel supervises the process, exposes `qwdtt://` / `wdtt://` URIs and subscription JSON for the Android client. Operator supplies live VK call hashes.
- **mieru** — censorship-resistant proxy over a custom protocol instead of TLS ([enfein/mieru](https://github.com/enfein/mieru) `mita`, GPL-3.0). Multi-client with per-panel-client HMAC credentials, per-client traffic & online accounting, and a `mierus://` share link. Clients: mieru CLI, mihomo, Clash Verge Rev, husi, Exclave.
- **TrustTunnel** — the AdGuard VPN protocol ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel), Apache-2.0): traffic indistinguishable from HTTPS (HTTP/1.1 + HTTP/2 + QUIC). Reuses the panel's ACME certificate (needs a domain with an issued cert), emits a `tt://?` deep link for the Flutter / CLI clients.
- **Sidecar outbounds** — client-mode Naive / mieru / TrustTunnel: paste a share link (`naive+https://` / `mierus://` / `tt://`), the tag shows up in routing rules and balancer pools (same as AWG outbound). Disable = blackhole (fail-closed, does not leak to `direct`). Client binaries ship in the tarball (`naive-client`, `mieru-client`, `trusttunnel-client`).

### 📦 Subscriptions, Geodata & Client Routing
- **Amnezia subscription** — dedicated `/awg/{subId}` endpoint returns a pure AmneziaWG `.conf` (or `?format=vpn` → `vpn://…` body) for AmneziaVPN / Happ; also listed next to Clash / JSON / base64 links in the panel and Telegram bot.
- **AWG in Clash Meta** — subscription emits AmneziaWG peers via `amnezia-wg-option` so Clash Meta clients can consume AWG alongside VLESS/Trojan.
- **Geodata browser** — open any `geoip*.dat` / `geosite*.dat` from the routing UI, search categories, multi-select into a rule (ported from upstream [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) by [STRENCH0](https://github.com/STRENCH0)).
- **RoscomVPN geo pack** — stock `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([hydraponique/roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)): RKN geoblock lists (`category-geoblock-ru`, `category-ru`, ads, YouTube / Telegram / Steam, …). Update from panel Version → Geofiles or `x-ui` menu.
- **Happ routing profiles** — Settings → Happ: built-in RoscomVPN deeplink profile plus free-text custom (from [hydraponique/roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

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

## 🔄 Migration from 3x-ui and existing AWG

LucX-UI shares the same Xray-core / SQLite (or PostgreSQL) database schema base as 3x-ui, and AWG tables are created automatically on first run. To install over an existing 3x-ui setup, back up your database first and run the standard install command:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

The AWG kernel module is built automatically by the installer (`bin/install-awg-module.sh`, DKMS). After install, run `x-ui` in the console to confirm AWG kernel module version and start adding AWG inbounds from the panel.

### AWG keys in the panel

There is no separate “keys” screen. A key is a client on an AmneziaWG inbound:

1. **Inbounds → Add inbound**, protocol **AmneziaWG (kernel)** (or native `amneziawg`).
2. **Clients → Add client**, attach to that inbound.
3. QR / download `.conf` / subscription `/awg/{subId}`.

Default inbound subnet is `/24` — up to ~253 clients.

### From existing AWG on the host

If the server already runs **awg-multi**, **toolza3** or **Docker Amnezia**, the panel **does not tear down** foreign `awg0`/`awg1`. Inbounds shows an **Import existing AWG** banner: preview peers → one inbound per interface. Keys / IPs / port / obfuscation are copied as-is. A kernel iface is renamed in place (`awg{id}`) — handshakes stay. Userspace/Docker: stop the old manager; those clients reconnect once.

Without a kernel module, LucX `awg` inbounds still come up on embedded amneziawg-go. Upstream's native `amneziawg` protocol is available in the panel next to it.

---

## 📜 License & Terms

This project is published under **two licenses** for first-party code, plus third-party binaries/data kept under their upstream terms (full matrix in [LICENSING.md](LICENSING.md)):

| Component | License |
|---|---|
| Original 3x-ui codebase | **GPL-3.0** |
| LucX-UI components (`internal/awg/`, `internal/lucx/`, LucX frontend pages) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| `forward_proxy` plugin ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy / `bin/naive-client-*` ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/mieru-*` (`mita`, [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| AmneziaWG kernel module & tools ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (module; installed on host) |
| Stock geo `.dat` (Loyalsoldier / IR / RU / ROSCOM) | Upstream of each dataset (see LICENSING.md) |

Tunnel binaries are **child processes** — the panel does not link them. qWDTT GPL applies to that binary and its sources, not to LucX PolyForm code.

---

## 🤝 Acknowledgements & Credits

LucX-UI stands on the shoulders of many open-source projects and people. Thank you.

### Testers & contributors
- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13) — AWG `routeThroughXray`), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, the **[3x-ui](https://github.com/MHSanaei/3x-ui)** team ([MHSanaei](https://github.com/MHSanaei) and contributors).

### Upstream PRs we ported / built on
- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) *feat(xray): browse geosite/geoip categories from routing rules* (geodata browser).

### Projects & inspiration
| Project | What we use | License |
|---|---|---|
| [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) | Base panel | GPL-3.0 |
| [amnezia-vpn](https://github.com/amnezia-vpn) — kernel module & tools | AmneziaWG protocol / AWG3 | GPL-2.0 (module) |
| [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) | NaiveProxy protocol / client ref | BSD-3-Clause |
| [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) + Caddy | NaiveProxy sidecar binary | MIT + Apache-2.0 |
| [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) | olcRTC core binary | WTFPL |
| [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) | qWDTT server binary | GPL-3.0 |
| [enfein/mieru](https://github.com/enfein/mieru) | mieru `mita` server binary | GPL-3.0 |
| [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) | TrustTunnel endpoint binary | Apache-2.0 |
| [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) | Caddyfile integration design reference | — |
| [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) | Tunnel-sidecar panel concept (qWDTT / olcRTC) | — |
| [hydraponique/3x-ui](https://github.com/hydraponique/3x-ui), [roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite), [roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing) | RoscomVPN geo pack + Happ routing profiles | Upstream |
| [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat), [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules), [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) | Stock geoip/geosite datasets | Upstream |
| [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) | AWG ops inspiration | — |
| [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls) | TLS fingerprint references for CPS | — |

---

## ☕ Support the project

LucX-UI is free for personal use. **Liked it? Star the repo ⭐** — it helps others find the project and keeps development going. Donations are optional and always appreciated:

| Method | Details |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🟠 **Boosty** (subscription) | [boosty.to/alexeylcp](https://boosty.to/alexeylcp) |
| 🟠 **Boosty** (one-time) | [boosty.to/alexeylcp/donate](https://boosty.to/alexeylcp/donate) |
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

<!-- END LUCX-HOOK -->