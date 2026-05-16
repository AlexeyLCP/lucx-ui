# LucX-UI Construct — Session State (May 16, 2026)

## Repository

- **GitHub:** https://github.com/AlexeyLCP/lucx-ui (public)
- **Branch:** `lucx-ui-phase1` → `main`
- **Commits:** 80+
- **Release:** v0.1.5 (43MB tarball with Xray v26.3.27 + GeoIP/GeoSite)
- **Server:** 34.88.118.168:2053 (vps-finland-lucx, Debian 12, GCP)

## Architecture: LUCX-HOOK Isolation

All new code in `internal/lucx/` (Go) and `frontend/src/lucx/` (Vue/JS). Original 3x-ui files get minimal changes wrapped in:

```
// LUCX-HOOK:
// call to isolated package
// END LUCX-HOOK
```

152 markers total. `grep -rn "LUCX-HOOK"` lists all integration points. Upstream merges are safe — conflicts limited to marked blocks.

### Backend packages (31 Go files)

| Package | Purpose |
|---------|---------|
| `internal/lucx/parser` | SSH output parser (smart node import) |
| `internal/lucx/nodetype` | LucX vs vanilla detection via `/lucx/hello` |
| `internal/lucx/outbound_link` | Inbound → outbound config generator |
| `internal/lucx/awg` | AWG params, CPS I1-I5, PostUp/PostDown templates, service |
| `internal/lucx/telemt` | Telemt TOML config, process manager, service |
| `internal/lucx/telegram` | Bot: lang persistence (JSON file), AWG .conf sender, Telemt tg://proxy sender |
| `internal/lucx/controller` | HTTP handlers: `/lucx/hello`, `/lucx/parse-ssh`, `/lucx/inbound-to-outbound`, `/lucx/awg/*`, `/lucx/telemt/*` |
| `internal/lucx/integration` | E2E lifecycle tests on real SQLite |
| `internal/lucx/stress_test.go` | Chaos engineering: 5000 concurrent ops, fuzzing, leak detection |

### Frontend components (9 Vue/JS files)

| File | Purpose |
|------|---------|
| `frontend/src/lucx/presets.js` | 18 presets for 6 protocols (no Cloudflare/Akamai/Fastly) |
| `frontend/src/lucx/PresetButtons.vue` | One-click preset application |
| `frontend/src/lucx/AWGForm.vue` | AWG inbound creation (obfLevel, mimicry, region, DNS, MTU) |
| `frontend/src/lucx/TelemtForm.vue` | Telemt creation + manual ee-secret input with hex validation |
| `frontend/src/lucx/SshParser.vue` | SSH output textarea → auto-fill node form |
| `frontend/src/lucx/NodeBadge.vue` | LucX-UI / Vanilla 3x-ui badge |
| `frontend/src/lucx/OutboundLinkButton.vue` | Inbound → outbound button |
| `frontend/src/lucx/awg-config-gen.js` | AWG .conf generator with obfuscation params |
| `frontend/src/lucx/client-generators.js` | AWG/Telemt client key generators (crypto.getRandomValues) |

## Key Fixes (May 2026)

### AWG — QR disabled in favor of .conf download
- AWG config text is too dense for QR → `noQR: true` flag in QrCodeModal
- Modal shows Download button only, filename: `<client>.conf`
- Config includes all obfuscation params: Jc, Jmin, Jmax, S1-S4, H1-H4, CPS I1-I5
- PrivateKey shows `<CLIENT_PRIVATE_KEY>` (user generates their own key)

### Telemt — manual secret input with validation
- `readonly` removed from Secret field
- `ee` prefix + 32+ hex chars validated in real-time
- Generate button preserved for new users
- Manual input allows importing existing users

### DPI Presets — no Cloudflare/Fastly/Akamai
- Ghost Mode: `gosuslugi.ru` SNI (critical RF infrastructure, cannot be blocked)
- RF Critical: `online.sberbank.ru` SNI (largest bank, blocking breaks banking)
- System update domains: `update.microsoft.com`, `releases.ubuntu.com`
- Ports 47000+ for all high-security presets (443 is monitored by TSPU)
- Empty SNI REMOVED — triggers instant TSPU ban (legitimate TLS always has SNI)

### Traffic Accounting
- AWG traffic: Client → awgN → Xray TUN (tag `awg-tun-{id}`) → Xray gRPC polls by tag
- Telemt traffic: Client → Telemt → SOCKS5 (tag `telemt-in-{id}`) → Xray gRPC polls by tag
- Total bytes tracked natively via Xray API — no custom parsers, no bash hacks
- Per-user breakdown: delegated to protocol-native tools (`awg show`, Telemt REST API)
- Integration tests confirm: lifecycle_test.go (Vector 2 — traffic accounting audit)

### E2E Tests — all passing
- `internal/lucx/integration/lifecycle_test.go`: 3 vectors
  - Vector 1: CRUD lifecycle — VLESS/AWG/Telemt creation, client add, cascade delete
  - Vector 2: Traffic accounting — tag conventions, parent-child linking
  - Vector 3: Parallel client ops — 20+20 concurrent clients
- `internal/lucx/stress_test.go`: 6 chaos tests
  - Vector 1: 5000 concurrent ops, 0 failures, 0 deadlocks
  - Vector 2: Fuzzing (1MB binary, SQL injection, XSS, emoji) — 0 panics
  - Vector 3: 100 create/delete cycles — 0 goroutine leaks
  - Vector 4: Crash recovery — PostDown idempotent, config survives corruption
- Total: 55+ tests across all packages

## Install

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install-lucx.sh)
```

- Supports Debian 12/13, Ubuntu 22.04/24.04, CentOS, Arch, Alpine
- Auto-downloads Xray-core v26.3.27 + GeoIP/GeoSite
- Installs AWG kernel module dependencies (graceful failure if unavailable)
- Creates `/etc/amnezia/amneziawg`, `/etc/telemt`, `/var/lib/telemt`, `/var/run/telemt`
- Binary kept as `x-ui` for full CLI script compatibility (systemctl restart x-ui)
- API-based download with direct URL fallback

## Tests

```bash
cd frontend && npm install && npm run build && cd ..
go test ./internal/lucx/... ./internal/lucx/integration/... -v -count=1
go test ./internal/lucx/ -v -run "Vector" -count=1  # chaos/stress
```

## License & Disclaimer

- LucX-UI components (`internal/lucx/`, `frontend/src/lucx/`): **PolyForm Noncommercial 1.0.0**
  - Free for personal/educational use
  - Commercial use (VPN resale, paid hosting, managed services) requires written permission
- Original 3x-ui code: AGPL-3.0
- Full disclaimer in README.md and README.ru_RU.md
- See `LICENSE-LucX.md` for complete terms

## External Repositories (Credits)

- 3x-ui: MHSanaei/3x-ui (AGPL-3.0)
- AWG obfuscation: pumbaX/awg-multi-script (MIT)
- AmneziaWG kernel module: MIT
- Telemt: telemt/telemt (Rust, MTProto)
- Xray-core: XTLS/Xray-core (MPL-2.0)
- GeoIP/GeoSite: Loyalsoldier/v2ray-rules-dat
