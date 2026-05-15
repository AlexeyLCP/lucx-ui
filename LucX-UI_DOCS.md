# LucX-UI Documentation

## Overview

LucX-UI is a long-term maintainable fork of [3x-ui](https://github.com/MHSanaei/3x-ui) with native integration of AmneziaWG (obfuscated WireGuard), Telemt (MTProto proxy), smart cluster management, and obfuscation presets.

**Current version:** 1.0.0 (Phase 1: Smart Cluster)

## Architecture

### Isolation Principle

All new LucX code lives in isolated packages:

- `internal/lucx/` — Go backend packages (parser, nodetype, outbound_link, controller)
- `frontend/src/lucx/` — Vue frontend components (SshParser, NodeBadge, OutboundLinkButton)

Changes to original 3x-ui files are minimal and wrapped in `LUCX-HOOK` / `END LUCX-HOOK` comment blocks. Run `grep -rn "LUCX-HOOK"` from the repo root to find all integration points.

### Package Map

| Package | Purpose |
|---------|---------|
| `internal/lucx/parser` | SSH output parsing → NodeCreds (scheme, host, port, basePath, username, password, apiToken) |
| `internal/lucx/nodetype` | LucX vs Vanilla detection via HTTP probe to `/panel/api/lucx/hello` |
| `internal/lucx/outbound_link` | Inbound → Outbound config generator (Xray protocols only) |
| `internal/lucx/controller` | HTTP handlers for `/panel/api/lucx/*` endpoints |

### Database Extensions

Two new fields in `model.Node`:
- `NodeType` — `""` (unchecked), `"lucx"`, or `"vanilla"`
- `NodeFeatures` — JSON string: `{"features":["awg","telemt"],"awgVersion":"2.0.1","telemtVersion":"3.4.11"}`

GORM AutoMigrate handles the schema change automatically.

### API Endpoints

| Method | Endpoint | Auth | Purpose |
|--------|----------|------|---------|
| GET | `/panel/api/lucx/hello` | Session/CSRF | Node identity (LucX detection) |
| POST | `/panel/api/lucx/parse-ssh` | Session/CSRF | Parse SSH install output → NodeCreds |
| POST | `/panel/api/lucx/inbound-to-outbound` | Session/CSRF | Generate outbound config from inbound |

### Smart Cluster Flow

1. **Smart Import:** User pastes SSH console output into textarea → SshParser.vue calls POST `/parse-ssh` → form fields (address, port, scheme, basePath, apiToken) auto-filled
2. **Node Type Detection:** After node save, heartbeat probe calls GET `/lucx/hello` → 200 = "lucx" (with features/versions), 404 = "vanilla" → stored in NodeType/NodeFeatures
3. **UI Badges:** NodeList table shows "LucX-UI" (blue, with AWG/MT/Pr feature tags) or "Vanilla 3x-ui" (gray) based on NodeType
4. **Protocol Guard:** Creating AWG/Telemt inbound on vanilla node returns 400 — "protocol requires a LucX-UI node"
5. **Outbound Link:** "Copy as Outbound" button on remote inbound → POST `/inbound-to-outbound` → JSON preview with copy-to-clipboard

### Modified Original Files

| File | Changes |
|------|---------|
| `database/model/model.go` | NodeType, NodeFeatures fields in Node struct |
| `web/controller/api.go` | LucX route group registration + import |
| `web/service/node.go` | Node type detection after probe |
| `web/service/inbound.go` | Vanilla guard in AddInbound + UpdateInbound |
| `frontend/src/pages/nodes/NodeFormModal.vue` | SshParser component in add mode |
| `frontend/src/pages/nodes/NodeList.vue` | NodeBadge component in table rows |

## Upstream Merge Guide

When pulling from upstream ([MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)):

```bash
git fetch upstream
git merge upstream/main
```

Resolve conflicts in files with LUCX-HOOK markers:
- `grep -rn "LUCX-HOOK"` — lists all integration points
- Keep LUCX-HOOK blocks intact
- Verify: `go build ./... && cd frontend && npm run build`
- Run tests: `go test ./... -short`

## Testing

```bash
# Run all LucX tests
go test ./internal/lucx/... -v

# Run all tests (including 3x-ui)
go test ./... -short

# Build frontend
cd frontend && npm run build
```

## License

- **LucX-UI components** (`internal/lucx/`, `frontend/src/lucx/`): PolyForm Noncommercial 1.0.0
  - Free for personal and educational use
  - Commercial use requires explicit written permission
- **Original 3x-ui code**: AGPL-3.0 (remains under upstream license)
- See `LICENSE-LucX.md` for full details

## Future Phases

| Phase | Features |
|-------|----------|
| 2 | AWG TUN injection with dynamic PostUp/PostDown scripts |
| 3 | Telemt process manager + SOCKS5 bridge to Xray routing |
| 4 | Obfuscation presets (VLESS Reality, Hysteria2) |
| 5 | Telegram bot: full Russian localization, AWG/Telemt client management |
