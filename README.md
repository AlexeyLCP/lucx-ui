[English](/README.md) | [Русский](/README.ru_RU.md)

# LucX-UI

Long-term maintainable fork of [3x-ui](https://github.com/MHSanaei/3x-ui) with native integration of AmneziaWG, Telemt (MTProto), smart cluster management, and DPI bypass presets for Russia (May 2026).

## Disclaimer

This software is provided "as is", without warranty of any kind, express or implied. It is created solely for educational, research, and network optimization purposes to study the behavior of encrypted protocols under tight network constraints.

The author:
1. Does not provide any commercial proxy, VPN, or data transmission services.
2. Does not maintain, control, or monitor any servers deployed by third parties using this software.
3. Shall not be held liable for any misuse, illegal redistribution, or compliance violations of local laws by end-users.

All responsibility for the deployment, use, and compliance of this software lies entirely with the individual running the installation script.

## Install

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install-lucx.sh)
```

Access: `http://<server_ip>:<port>/<basepath>` — реквизиты выводятся установщиком в конце.

## Added Protocols

| Protocol | Transport | Obfuscation | Traffic Path |
|----------|-----------|-------------|-------------|
| **AWG** (AmneziaWG) | UDP kernel module | Jc/Jmin/Jmax, S1-S4, H1-H4, CPS I1-I5 | Client → awgN → Xray TUN → Routing → Outbound |
| **Telemt** (MTProto) | TCP 443/8443 | FakeTLS (`ee` secret), SOCKS5 upstream via Xray | Client → Telemt → 127.0.0.1:SOCKS5 → Xray Routing |

Both protocols create invisible child Xray inbounds (TUN for AWG, SOCKS5 for Telemt). Traffic accounting uses Xray's native gRPC API — tagged children are polled for total bytes. Per-user breakdown is delegated to protocol-native tools (`awg show`, Telemt REST API). No custom parsers, no bash hacks.

## Cluster (Multi-Node)

- **SSH Smart Import:** paste install script output into the Add Node form — fields auto-filled
- **Node Type Detection:** each node probed via `GET /panel/api/lucx/hello` — LucX vs vanilla 3x-ui badges in UI
- **Vanilla Guard:** AWG/Telemt protocols blocked at API level for vanilla 3x-ui nodes
- **Inbound → Outbound:** one-click copy of remote inbound as local outbound config

## DPI Presets (Russia, May 2026)

All presets avoid Cloudflare, Fastly, and Akamai domains. Use Russian critical infrastructure (`gosuslugi.ru`, `online.sberbank.ru`) and system update domains (`update.microsoft.com`, `releases.ubuntu.com`) that cannot be blocked without breaking essential services.

- **VLESS Reality:** Ghost Mode (gosuslugi.ru + randomized fingerprint), Best Speed (XHTTP + update.microsoft.com), RF Critical (Sberbank), Stealth QUIC, Anti-DPI
- **Trojan Reality:** Ghost Mode, Best Speed (TLS 1.3), Stealth (WS)
- **Hysteria2:** Salamander obfs + Masquerade + port hopping (1000 ports)
- **AWG:** Jumbo Random (Jc 3-10, Jmin 50-100, Jmax 150-250)
- **Telemt:** FakeTLS Neutral (ee + hex domain encoding)
- **Shadowsocks:** 2022 blake3-aes-128-gcm

Port 443 is monitored — presets use ports 47000+ for maximum security. Split tunneling (`geosite:category-ru` → direct) is auto-configured on panel start.

## Telegram Bot

`/lang` — language selection (EN/RU/FA/ZH). AWG clients receive `.conf` files via Telegram Document. Telemt clients receive `tg://proxy` deep links with inline "Connect" button. Language preferences persist across restarts (`/etc/lucx-ui/lucx_tg_langs.json`).

## Project Structure

```
internal/lucx/
├── parser/              SSH output parser (smart import)
├── nodetype/            LucX vs vanilla detection
├── outbound_link/       Inbound → outbound generator
├── awg/                 AWG params, CPS, templates, service
├── telemt/              Telemt config, process manager, service
├── telegram/            Bot helpers (lang, AWG/Telemt links)
├── controller/          HTTP API handlers
├── integration/         End-to-end integration tests
└── stress_test.go       Chaos engineering suite

frontend/src/lucx/
├── presets.js           Obfuscation presets for all protocols
├── PresetButtons.vue    One-click preset application
├── AWGForm.vue          AWG inbound creation form
├── TelemtForm.vue       Telemt inbound creation form
├── SshParser.vue        SSH output paste & parse
├── NodeBadge.vue        LucX/Vanilla badge
├── OutboundLinkButton.vue  Inbound → outbound button
├── awg-config-gen.js    AWG .conf file generator
└── client-generators.js AWG/Telemt client key generators
```

## Tests

```bash
# Unit + integration (requires Go 1.24+)
cd frontend && npm install && npm run build && cd ..
go test ./internal/lucx/... ./internal/lucx/integration/... -v -count=1

# Chaos engineering (skip with -short for CI)
go test ./internal/lucx/ -v -run "Vector" -count=1

# Run all tests
go test ./internal/lucx/... ./internal/lucx/integration/... ./database/model/... -count=1
```

Test categories:
- **parser:** 7 tests (SSH output, ANSI handling, edge cases)
- **awg:** 13 tests (params, CPS, templates, config validation)
- **telemt:** 11 tests (config, secrets, TOML, proxy links, table-driven)
- **nodetype:** 3 tests (LucX, vanilla, timeout)
- **outbound_link:** 4 tests (VLESS, rejection, edge cases)
- **telegram:** 9 tests (AWG config, Telemt links, validation)
- **integration:** 3 tests (CRUD lifecycle, traffic accounting, parallel clients)
- **stress:** 6 tests (concurrency 5000 ops, fuzzing, resource leaks, crash recovery)

## Architecture Rules

All new code lives in `internal/lucx/` (Go) and `frontend/src/lucx/` (Vue). Changes to original 3x-ui files are wrapped in:

```
// LUCX-HOOK:
// ... new code call ...
// END LUCX-HOOK
```

Run `grep -rn "LUCX-HOOK"` to list all integration points. This keeps upstream merges clean — conflicts are limited to marked blocks.

## Credits

- **3x-ui** — [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) (AGPL-3.0)
- **AWG obfuscation logic** — [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) (MIT)
- **AmneziaWG** — kernel module and userspace tools (MIT)
- **Telemt** — MTProto proxy in Rust, [telemt/telemt](https://github.com/telemt/telemt)
- **Xray-core** — [XTLS/Xray-core](https://github.com/XTLS/Xray-core) (MPL-2.0)
- **GeoIP/GeoSite** — [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)

## License

LucX-UI components (`internal/lucx/`, `frontend/src/lucx/`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use — including VPN resale, paid proxy/VPN hosting, managed services — requires explicit written permission from the author.

Original 3x-ui code remains under AGPL-3.0.

See `LICENSE-LucX.md` for full terms.
