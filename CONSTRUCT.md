# LucX-UI Construct — Session Snapshot (May 17, 2026 ~10:00 MSK)

## Quick Resume

```
Продолжи работу над lucx-ui. Прочитай CONSTRUCT.md, изучи историю коммитов, проверь состояние тестов и сервера.
```

## Repository

- **GitHub:** https://github.com/AlexeyLCP/lucx-ui (public)
- **Branch:** `lucx-ui-phase1` → `main`
- **Commits:** 85+
- **Release:** v0.1.5-pre-MVP (prerelease, 43MB tarball)
- **Local path (WSL):** `/home/lcp/3x-ui`
- **Local path (Windows):** `C:\Users\dante\projects\3x-ui` → WSL: `/mnt/c/Users/dante/projects/3x-ui`
- **Test server:** vps-finland-lucx (GCP, 34.88.118.168, Debian 12)
- **Server panel path:** `/usr/local/x-ui/`
- **Server CLI:** `/usr/bin/x-ui`
- **Server systemd:** `x-ui.service`
- **Server web port:** 33976 (HTTPS)
- **Server web base path:** `/apAjopDQ2DsWREievL/`
- **Server DB:** `/etc/x-ui/x-ui.db`
- **Server API token:** `3HseHV5D3NRYXz8Lr9obmm3gPsNyXLEqRifnSimwiSo4qBfi`

## Architecture

All new code isolated in `internal/lucx/` (Go, 31 files) and `frontend/src/lucx/` (Vue/JS, 9 files). Changes to 3x-ui files wrapped in `// LUCX-HOOK` / `// END LUCX-HOOK` markers. 153+ markers total.

### Key Files

| File | Purpose |
|------|---------|
| `internal/lucx/awg/params.go` | AWGParams struct, GenerateAWGParams, ValidateAWGParams, MergeParamsToSettings, validateHRange |
| `internal/lucx/awg/cps.go` | GenerateCPS (QUIC/SIP/DNS), DomainPoolByRegion, PickRandomDomain |
| `internal/lucx/awg/service.go` | AWGService: Create/Delete/AddClient/DeleteClient, EnsureAWGParams, RepairAllAWGInbounds, RepairAWGOnGet, buildAWGConfig, logAWG |
| `internal/lucx/awg/importer.go` | ScanAWGConfigs, ParseAWGConfig, ImportAllAWGConfigs |
| `internal/lucx/awg/templates.go` | PostUp/PostDown bash scripts |
| `internal/lucx/awg/firewall.go` | CheckPrerequisites, DetectFirewall |
| `internal/lucx/telemt/service.go` | TelemtService: Create/Delete, AddClient/DeleteClient, RepairAllTelemtInbounds, logTelemt |
| `internal/lucx/telemt/config.go` | GenerateConfig (TOML template) |
| `internal/lucx/telemt/manager.go` | TelemtManager: EnsureBinary, Start, Stop, Healthcheck |
| `internal/lucx/controller/lucx_controller.go` | All HTTP handlers: AWG CRUD, Telemt CRUD, client ops, Smart Import, repair |
| `internal/lucx/telegram/awg.go` | buildAWGConfigText (bot config generation) |
| `frontend/src/lucx/awg-config-gen.js` | generateAWGConfig — client .conf generator |
| `frontend/src/lucx/client-generators.js` | generateAWGClient, generateTelemtClient, buildClientObject (URL-safe base64) |
| `frontend/src/lucx/presets.js` | All protocol presets (VLESS, Trojan, Hysteria2, AWG, Telemt, Shadowsocks) |
| `frontend/src/lucx/AWGForm.vue` | AWG creation form |
| `frontend/src/lucx/TelemtForm.vue` | Telemt creation form |
| `frontend/src/api/lucx-api.js` | postLucx, getLucx — safe JSON-aware wrappers with transformResponse |
| `frontend/src/models/inbound.js` | AWGSettings, TelemtSettings — full passthrough model classes |
| `web/web.go` | Startup hooks: AWG/Telemt repair at boot |
| `web/controller/api.go` | LucX route registration |
| `database/model/model.go` | AWG/Telemt/TUN protocols, IsSpecialInbound, GenSpecialConfig |
| `AGENTS.md` | Agent operating manual with best practices |

## Critical Bug Fixes Applied (May 16-17 sessions)

### 1. AWG Obfuscation Flow
- **Root cause:** `CreateAWGInbound` hardcoded `GenerateAWGParams(1, "quic", "ru")`, params not saved to Settings
- **Fix:** Read obfLevel/profile/region from request, generate CPS, call MergeParamsToSettings BEFORE AddInbound
- **Files:** params.go, service.go, cps.go

### 2. Frontend Model Passthrough
- **Root cause:** `AWGSettings.toJson()` used `if (this.jc)` — 0 is falsy, field dropped
- **Fix:** Changed to `if (this.jc !== undefined)` for all numeric fields
- **Root cause:** `Inbound.protocol` setter ALWAYS replaced settings with empty AWGSettings
- **Fix:** Guard: `if (this._protocol === protocol) return`
- **Files:** inbound.js (AWGSettings.toJson, protocol setter)

### 3. Client Toggle 404
- **Root cause:** `lucx-api.js` used raw `axios` without Content-Type: application/json, no transformResponse
- **Root cause:** webBasePath `/apAjopDQ2DsWREievL/` not in hardcoded `LUCX_BASE` path
- **Fix:** Content-Type header, transformResponse for safe JSON parsing, axios.defaults.baseURL handles path
- **Files:** lucx-api.js, InboundsPage.vue (postLucxSafe)

### 4. Double Error Toasts
- **Root cause:** `confirmDelete` had TWO `del/${id}` calls; `_handleMsg` shows toast for every non-empty msg
- **Fix:** Single del call, postLucxSafe no longer shows toasts for structured errors
- **Files:** InboundsPage.vue (confirmDelete, onToggleEnableClient, onDeleteClient)

### 5. JS Falsy Trap
- **Root cause:** `s.jc || 8` — when jc is 0, falls through to default 8
- **Fix:** Changed to `s.jc ?? 8` (nullish coalescing) for ALL numeric params
- **Files:** awg-config-gen.js

### 6. Telemt Missing Endpoints
- **Root cause:** No `add-client`/`del-client` routes for Telemt → 404 → HTML → "invalid character 'o'"
- **Fix:** Added AddTelemtClient, DeleteTelemtClient handlers and routes
- **Files:** lucx_controller.go

### 7. DeleteAWG/DeleteTelemt HTTP Status
- **Root cause:** HTTP 200 with success:false on error — client can't distinguish
- **Fix:** Changed to HTTP 500 on error
- **Files:** lucx_controller.go

### 8. AWG DeleteClient Error Silencing
- **Root cause:** `exec.Command("awg", ...).Run()` ignored errors
- **Fix:** Check CombinedOutput, log warning
- **Files:** awg/service.go

### 9. Flaky Test (jc=8)
- **Root cause:** Test checked `jc != 8` but 8 is valid random outcome in [4,16]
- **Fix:** Check `jc != 0` + verify I1-I5 keys present
- **Files:** lifecycle_test.go

### 10. Database Migration
- **Fix:** RepairAWGOnGet (standalone), RepairAllAWGInbounds (startup), RepairAllTelemtInbounds (startup)
- **Files:** awg/service.go, telemt/service.go, web.go

## Known Gotchas

1. **Import cycle:** `lucx/awg` → `web/service` → can't reverse. Use frontend-side LucX calls for kernel operations.
2. **webBasePath:** `/apAjopDQ2DsWREievL/` — axios prepends it, curl tests must include it.
3. **Auth middleware:** Returns 404 (not 401) for unauthenticated non-AJAX requests.
4. **S1+56 ≠ S2:** DPI detection risk. Also S2+56 ≠ S3, S3+56 ≠ S4.
5. **H-quadrants:** Non-overlapping. H1 [5, 536870911], H2 [536870912, 1073741823], H3 [1073741824, 1610612735], H4 [1610612736, 2147483647].
6. **I1-I5 format:** Raw hex strings in Settings. `<b 0x...>` prefix added during config generation.
7. **Client base64:** URL-safe (`-` for `+`, `_` for `/`, no `=`).
8. **Telemt AddClient:** Stops ALL users (TOML modify requires process restart).
9. **JS numeric falsy:** NEVER `if (x)` for numbers — 0 is falsy. Use `??` or `!== undefined`.
10. **Protocol setter:** Destroys settings. Guard with same-protocol check.

## AWG Parameter Ranges (from pumbaX/awg-multi-script)

| Param | Range | Constraint |
|-------|-------|------------|
| Jc | 4–16 | Junk packets count |
| Jmin | 50–256 | Min junk size (bytes) |
| Jmax | 300–1000 | Max junk size, Jmax > Jmin |
| S1 | 15–150 | Handshake init padding |
| S2 | 15–150 | Handshake response padding, S1+56 ≠ S2 |
| S3 | 8–64 | Cookie reply padding |
| S4 | 6–31 | Transport data padding |
| H1 | [5, 536870911] | Quadrant 1 |
| H2 | [536870912, 1073741823] | Quadrant 2 |
| H3 | [1073741824, 1610612735] | Quadrant 3 |
| H4 | [1610612736, 2147483647] | Quadrant 4 |

## API Endpoints

```
POST /panel/api/lucx/awg/create
POST /panel/api/lucx/awg/delete
POST /panel/api/lucx/awg/add-client
POST /panel/api/lucx/awg/del-client
GET  /panel/api/lucx/awg/prerequisites
GET  /panel/api/lucx/awg/import
GET  /panel/api/lucx/awg/import/*path
POST /panel/api/lucx/awg/repair
POST /panel/api/lucx/awg/ensure       # Repair single inbound params, returns settings

POST /panel/api/lucx/telemt/create
POST /panel/api/lucx/telemt/delete
POST /panel/api/lucx/telemt/add-client
POST /panel/api/lucx/telemt/del-client
GET  /panel/api/lucx/telemt/status/:id
GET  /panel/api/lucx/telemt/link/:id
GET  /panel/api/lucx/telemt/version
POST /panel/api/lucx/telemt/repair
```

## Test Commands

```bash
cd /mnt/c/Users/dante/projects/3x-ui  # or /home/lcp/3x-ui
export PATH=/home/lcp/.local/go/bin:$PATH
export GOTOOLCHAIN=auto
go test ./internal/lucx/... -count=1 -v
```

9 test packages: internal/lucx, awg, integration, telemt, telegram, nodetype, outbound_link, parser.

## Deploy to Server

```bash
export PATH=/home/lcp/.local/go/bin:$PATH
export GOTOOLCHAIN=auto
npm --prefix /home/lcp/3x-ui/frontend run build
go build -C /home/lcp/3x-ui -o /tmp/x-ui -ldflags="-s -w" .
gcloud compute scp /tmp/x-ui vps-finland-lucx:/tmp/x-ui --zone=europe-north1-a
gcloud compute ssh vps-finland-lucx --zone=europe-north1-a --command="
  sudo systemctl stop x-ui &&
  sudo cp /tmp/x-ui /usr/local/x-ui/x-ui &&
  sudo chmod +x /usr/local/x-ui/x-ui &&
  sudo systemctl start x-ui &&
  sleep 2 && echo DEPLOYED
"
```

## Testing API directly

```bash
TOKEN="3HseHV5D3NRYXz8Lr9obmm3gPsNyXLEqRifnSimwiSo4qBfi"
BASE="https://127.0.0.1:33976/apAjopDQ2DsWREievL/panel/api/lucx"
curl -sk -H "Authorization: Bearer $TOKEN" -H "X-Requested-With: XMLHttpRequest" "$BASE/hello"
curl -sk -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"id":11}' "$BASE/awg/ensure"
```
