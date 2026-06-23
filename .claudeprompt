# LucX-UI — Agent Operating Manual

This file is the law for every agent working on this project. Read it completely before touching any code.

---

## Workflow: How an Agent Executes a Task

```
1. READ    → Read CONSTRUCT.md, git log --oneline -10, check latest state
2. AUDIT   → Read all relevant files, trace data flow end-to-end
3. PLAN    → Write a short plan: which files, what changes, what tests
4. BRANCH  → Work on `feature/awg-integration` (current active branch)
5. CODE    → Implement changes inside LUCX-HOOK blocks only
6. TEST    → Run tests: `go test ./internal/lucx/... -count=1 -v`
7. BUILD   → Frontend: `npm run build`, Backend: `go build -o /tmp/x-ui`
8. DEPLOY  → SCP to vps-finland-lucx, restart x-ui.service
9. VERIFY  → Check `sudo systemctl status x-ui`, check server logs
10. COMMIT → `git add` specific files, `git commit` with descriptive message
11. STATUS → Output `git status` and `git log --oneline -15` after commits
12. DOCS   → Update AWG_CHANGES.md with all changes, update AGENTS.md if needed
```

---

## The 10 Rules

### 1. LUCX-HOOK Isolation

ALL new code goes inside `// LUCX-HOOK` / `// END LUCX-HOOK` markers. Never modify 3x-ui core code outside these markers without explicit instruction.

```go
// LUCX-HOOK: Description of what this does
// ... your code ...
// END LUCX-HOOK
```

```js
// LUCX-HOOK: Description
// ... your code ...
// END LUCX-HOOK
```

Stat: 153+ LUCX-HOOK blocks across the codebase. Keep this discipline.

### 2. Isolated Modules

New functionality lives ONLY in:
- **Go:** `internal/lucx/` — subdirectories: `awg/`, `telemt/`, `telegram/`, `controller/`, `nodetype/`, `parser/`, `outbound_link/`, `integration/`
- **Frontend:** `frontend/src/lucx/` — components, generators, presets
- **API:** `frontend/src/api/lucx-api.js`

Integration points (model.go, inbound.js, web.go, etc.) get LUCX-HOOK blocks only when unavoidable.

### 3. Paranoid Logging

Every critical operation logs with a prefix:
```
[LUCX-AWG]            — AWG service operations
[LUCX-AWG-CLIENT]     — AWG client operations
[LUCX-TELEMT]         — Telemt service operations
[LUCX-TELEMT-CLIENT]  — Telemt client operations
[AWG DEBUG]           — Frontend config generation
```

Go: `fmt.Printf("[LUCX-AWG] message\n", args...)`
JS: `console.warn('[AWG DEBUG] message', data)`

### 4. Test Before Deploy

```bash
cd /home/lcp/projects/lucx-ui
export PATH=/home/lcp/.local/go/bin:$PATH
export GOTOOLCHAIN=auto
go test ./internal/lucx/... -count=1 -v
```

9 test packages must pass:
- `internal/lucx` — stress tests
- `internal/lucx/awg` — params, CPS, templates, firewall, importer
- `internal/lucx/integration` — lifecycle, traffic accounting, parallel ops, obfuscation persistence
- `internal/lucx/telemt` — config, client
- `internal/lucx/telegram` — AWG config, Telemt link
- `internal/lucx/nodetype`, `outbound_link`, `parser`

### 5. Deploy Flow

```bash
# 1. Build frontend
npm --prefix /home/lcp/projects/lucx-ui/frontend run build

# 2. Build Go binary
go build -C /home/lcp/projects/lucx-ui -o /tmp/x-ui -ldflags="-s -w" .

# 3. Deploy to server
gcloud compute scp /tmp/x-ui vps-finland-lucx:/tmp/x-ui --zone=europe-north1-a
gcloud compute ssh vps-finland-lucx --zone=europe-north1-a --command="
  sudo systemctl stop x-ui &&
  sudo cp /tmp/x-ui /usr/local/x-ui/x-ui &&
  sudo chmod +x /usr/local/x-ui/x-ui &&
  sudo systemctl start x-ui
"

# 4. Verify
gcloud compute ssh vps-finland-lucx --zone=europe-north1-a --command="
  sudo systemctl status x-ui --no-pager -l | head -10
"
```

### 6. Server Reference

| Item | Value |
|------|-------|
| Name | vps-finland-lucx |
| IP | 34.88.118.168 |
| Zone | europe-north1-a |
| OS | Debian 12 |
| Panel path | `/usr/local/x-ui/` |
| Binary | `/usr/local/x-ui/x-ui` |
| DB | `/etc/x-ui/x-ui.db` |
| Service | `x-ui.service` |
| Web port | 33976 (HTTPS) |
| Web base path | `/apAjopDQ2DsWREievL/` |

### 7. Commit Message Format

```
type(scope): short description

# Types: feat, fix, refactor, test, docs, chore, perf
# Scope: awg, telemt, telegram, controller, frontend, presets, import, config

# Examples:
feat(awg): add CPS I1-I5 generation with domain pools from pumbaX
fix(frontend): url-encode AWG client IDs to prevent 404 on toggle
test(awg): add ValidateAWGParams table-driven tests
refactor(controller): unify JSON error responses across all handlers
```

### 8. No Silent Failures

- Go: never ignore errors with `_` without a comment explaining why
- JS: never `catch (_) {}` — at minimum `console.warn`
- API handlers: always return `gin.H{"success": false, "msg": err.Error()}`
- Frontend LucX calls: use `postLucxSafe` which never throws

### 9. Data Flow Verification

For ANY change to AWG/Telemt params, trace the FULL chain:
```
GenerateAWGParams → MergeParamsToSettings → DB (settings column)
  → API (JSON string) → dbInbound.toInbound() → JSON.parse
  → Inbound constructor → this.settings (raw object)
  → generateAWGConfig → s.jc, s.jmin, ... → .conf output
```

Verify at each step. If a parameter goes missing, find EXACTLY which step drops it.

### 10. Frontend Model Safety

- `AWGSettings.toJson()`: NEVER use `if (this.jc)` — 0 is falsy. Use `if (this.jc !== undefined)`
- `AWGSettings` constructor: use `params.jc ?? 0` not `params.jc || 0`
- `generateAWGConfig`: use `s.jc ?? 8` not `s.jc || 8`
- `Inbound.protocol` setter: don't destroy settings when protocol unchanged

### 11. Database Migrations

All new database fields must be added through explicit migrations. Use GORM AutoMigrate for schema changes, and always verify the migration on a backup of the production database before deploying.

New fields added to `database/model/model.go` must be reflected in `AWG_CHANGES.md`.

### 12. Plugin Compatibility

All existing plugins must continue to work, including Суперсил. When adding new functionality, verify it does not conflict with or disable any existing plugin. Test with a representative set of plugins before deploying.

### 13. PostUp/PostDown Scripts

PostUp/PostDown scripts are strictly temporary — applied at runtime and cleaned up after use. No permanent system modifications. No changes to system network configuration, iptables rules, or kernel parameters that persist after service restart. Templates live in `internal/lucx/awg/templates.go` and are rendered with dynamic parameters at runtime.

### 14. Documentation After Each Stage

After completing a major stage, update:
- **AWG_CHANGES.md** — list all changes, fixes, and known issues
- **AGENTS.md** — if rules or workflows changed
- **README.md** — AWG section if new features were added

---

## Project Structure

```
/home/lcp/projects/lucx-ui/
├── internal/lucx/
│   ├── awg/               # AWG service, params, CPS, templates, firewall, importer
│   │   ├── params.go      # AWGParams, GenerateAWGParams, ValidateAWGParams, MergeParamsToSettings
│   │   ├── cps.go         # GenerateCPS, QUIC/SIP/DNS packet generators, domain pools
│   │   ├── service.go     # AWGService: Create/Delete/AddClient/DeleteClient/Ensure/Repair
│   │   ├── importer.go    # ScanAWGConfigs, ParseAWGConfig, ImportAllAWGConfigs
│   │   ├── templates.go   # PostUp/PostDown bash script templates
│   │   └── firewall.go    # CheckPrerequisites, DetectFirewall
│   ├── telemt/            # Telemt service, config, manager, client
│   ├── telegram/          # Telegram bot: AWG config, Telemt proxy, menu, lang
│   ├── controller/        # LucXController: all HTTP handlers
│   ├── nodetype/          # LucX vs Vanilla detection
│   ├── parser/            # SSH output parser
│   ├── outbound_link/     # Inbound→outbound config generator
│   └── integration/       # Integration tests (SQLite-backed)
├── frontend/src/lucx/
│   ├── awg-config-gen.js  # Client .conf generator
│   ├── client-generators.js # Key generation (URL-safe base64)
│   ├── presets.js         # All protocol presets
│   ├── AWGForm.vue        # AWG creation form
│   ├── TelemtForm.vue     # Telemt creation form
│   ├── PresetButtons.vue  # Quick preset buttons
│   ├── NodeBadge.vue      # LucX/Vanilla badge
│   ├── OutboundLinkButton.vue
│   └── SshParser.vue
├── frontend/src/api/
│   └── lucx-api.js        # postLucx(), getLucx() — safe JSON-aware wrappers
├── web/
│   ├── web.go             # Startup hooks (AWG/Telemt repair)
│   └── controller/
│       └── api.go         # LucX route registration
├── database/model/
│   └── model.go           # AWG/Telemt/TUN protocols, IsSpecialInbound, GenSpecialConfig
├── CONSTRUCT.md           # Session snapshot
└── AGENTS.md              # This file
```

---

## Coding Standards

### Go

| Rule | Requirement |
|------|-------------|
| Version | Go 1.21+ (GOTOOLCHAIN=auto) |
| Format | `gofmt` (standard) |
| Imports | stdlib → third-party → local (`github.com/mhsanaei/3x-ui/v3/...`) |
| Errors | Always wrap: `fmt.Errorf("context: %w", err)` |
| Logging | `fmt.Printf("[LUCX-XXX] ...\n", args...)` |
| JSON | `gin.H{"success": false, "msg": err.Error()}` |
| HTTP status | 400 for bad request, 404 for not found, 500 for server error |
| Panics | NEVER panic. Recover in controller if unavoidable |
| Max func | ~80 lines (signal to split) |

### Vue/JavaScript

| Rule | Requirement |
|------|-------------|
| Version | ES2020+ |
| Components | `<script setup>` Composition API |
| API calls | Use `HttpUtil.post` (auto-toast) or `postLucxSafe` (LucX, no auto-toast) |
| Null checks | `??` for numbers, `\|\|` for strings |
| Debugging | `console.error` for critical, `console.warn` for warnings |
| Errors | Always show `message.error()` for user-facing failures |

---

## Known Issues / Gotchas

1. **Import cycle**: `lucx/awg` imports `web/service` → can't import `lucx/awg` from `web/service`. Use frontend-side calls instead.

2. **webBasePath**: Server has custom base path `/apAjopDQ2DsWREievL/`. All API calls go through axios which auto-prepends it. Direct curl tests must include it.

3. **Auth middleware**: Non-auth API calls get 404 (not 401) from `checkAPIAuth`. Always test with Bearer token.

4. **S1+56 ≠ S2 constraint**: DPI detection risk if S1+56 equals S2. Similarly for S2+56 ≠ S3, S3+56 ≠ S4.

5. **H-quadrants**: H1 [5, 536870911], H2 [536870912, 1073741823], H3 [1073741824, 1610612735], H4 [1610612736, 2147483647].

6. **I1-I5 format**: CPS values are raw hex strings. The `<b 0x...>` prefix is added during config generation, not stored.

7. **Client base64 keys**: URL-safe format (`-` instead of `+`, `_` instead of `/`, no `=` padding).

8. **Telemt AddClient stops all users**: Modifying the TOML config requires stopping and restarting the entire Telemt process.

---

## Debugging Patterns (from real sessions)

### Pattern 1: Full-Chain Data Trace

When a value disappears between backend and frontend:
```
1. Check DB:    sqlite3 /etc/x-ui/x-ui.db "SELECT settings FROM inbounds WHERE id=N"
2. Check API:   curl -sk -H "Authorization: Bearer TOKEN" "https://.../panel/api/lucx/awg/ensure" -d '{"id":N}'
3. Check Model: add console.error("[DEBUG] Raw settings:", JSON.stringify(s, null, 2)) in generateAWGConfig
4. Check toJson: verify AWGSettings.toJson() doesn't drop falsy values (0, "", false)
5. Fix the exact step that drops the value
```

### Pattern 2: JSON Passthrough

When creating model classes that wrap server JSON:
```js
// CORRECT: full passthrough
constructor(params) {
    this.jc = params.jc ?? 0;        // ?? preserves 0
    this.name = params.name || '';   // || fine for strings
}
toJson() {
    if (this.jc !== undefined) out.jc = this.jc;  // NOT if (this.jc)
}
```

### Pattern 3: Request Debugging

When API calls fail silently:
```bash
# 1. Check if routes are registered
curl -sk "https://127.0.0.1:33976/BASE_PATH/panel/api/lucx/hello"

# 2. Check with auth
curl -sk -H "Authorization: Bearer TOKEN" -H "X-Requested-With: XMLHttpRequest" \
  "https://127.0.0.1:33976/BASE_PATH/panel/api/lucx/hello"

# 3. Check server logs
sudo journalctl -u x-ui --no-pager -n 50 | grep -E "LUCX|Error|panic"
```

### Pattern 4: Repeated Bug Classes

| Symptom | Root Cause | Fix |
|---------|-----------|-----|
| Jc=0, Jmin=0 everywhere | `toJson()` drops falsy values | `!== undefined` not `if (x)` |
| "invalid character 'o'" toast | Non-JSON response from LucX API | `lucx-api.js` transformResponse |
| Double delete error | Two `del/${id}` calls in `confirmDelete` | Single call + error check |
| Toggle shows error but works | LucX API called after standard API fails | Check standard API success first |
| Default params after UI edit | `protocol` setter destroys settings | Guard: `if (this._protocol === p) return` |
| 404 on LucX API | `webBasePath` not in hardcoded URL | Use configured axios instance |

---

## Test Patterns

### Go: Table-Driven Tests
```go
func TestValidateAWGParams_TableDriven(t *testing.T) {
    tests := []struct {
        name    string
        mutate  func(p *AWGParams)
        wantErr string
    }{
        {name: "jc zero", mutate: func(p *AWGParams) { p.Jc = 0 }, wantErr: "jc out of range"},
        {name: "jmin >= jmax", mutate: func(p *AWGParams) { p.Jmin = 500; p.Jmax = 300 }, wantErr: "jmin"},
        // ... 10+ cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

### Integration: Full Chain Verification
```go
func TestVector4_AWGObfuscationParamsInSettings(t *testing.T) {
    params, _ := awg.GenerateAWGParams(3, "quic", "ru")
    i1, i2, i3, i4, i5 := awg.GenerateCPS(3, awg.CPSProfileQUIC)
    awg.ValidateAWGParams(params)
    awg.MergeParamsToSettings(inbound, params, i1, i2, i3, i4, i5)
    // Save to DB → read back → verify ALL fields present
    // Verify jc != 0, i1-i5 all present
}
```

---

---

## Community Best Practices (May 2026)

### Go + Gin: Production Error Handling

**Structured errors over bare strings:**
```go
// ✅ Use typed errors
type LucXError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Cause   error  `json:"-"`
}
func (e *LucXError) Error() string { return e.Message }

// In handlers:
ctx.JSON(http.StatusInternalServerError, gin.H{
    "success": false,
    "msg":    err.Error(),  // safe — never expose raw DB errors
})
```

**Middleware ordering matters (Gin):**
```
1. RequestID / Trace
2. Structured logger (slog)
3. Custom Recovery (returns JSON, not blank 500)
4. CORS / Rate limiting
5. Auth
6. Business handlers
7. Global error handler (LAST)
```

**Never in production:**
- `ctx.String()` / `ctx.HTML()` for API responses
- `err.Error()` string matching (use `errors.Is`/`errors.As`)
- Default `gin.Recovery()` (returns empty body, not JSON)
- Opaque panics — always recover and return structured JSON

**Custom recovery middleware (Gin):**
```go
func LucXRecovery() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("[LUCX-PANIC] path=%s panic=%v", c.Request.URL.Path, r)
                c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
                    "success": false,
                    "msg":     "internal server error",
                })
            }
        }()
        c.Next()
    }
}
```

### Vue 3: Composition API Best Practices

**`<script setup>` structure (fixed order):**
```
1. imports
2. defineProps / defineEmits / defineModel
3. ref / reactive / shallowRef
4. computed
5. functions
6. watch / watchEffect
7. onMounted / onUnmounted
```

**Falsy trap (critical for AWG numeric params):**
```js
// ❌ BAD: 0 is falsy — falls through to default
const jc = s.jc || 8

// ✅ GOOD: nullish coalescing preserves 0
const jc = s.jc ?? 8

// ❌ BAD in toJson(): 0 is falsy — field dropped
if (this.jc) out.jc = this.jc

// ✅ GOOD: explicit undefined check
if (this.jc !== undefined) out.jc = this.jc
```

**Pinia stores:**
- Always use Setup syntax (not Options)
- Consume with `storeToRefs()` for state, direct destructure for actions
- One store per file, `defineStore` id matches filename

**Components:**
- Max 150 lines `<script>` — extract into composables
- Props: type generics + `withDefaults()`
- `v-for` key: stable unique ID, never index
- Side effects: clean up with `onScopeDispose`

### AWG Parameters: Current Community Consensus

**Ranges validated by pumbaX/awg-multi-script + community testing:**
| Param | Range | Constraint |
|-------|-------|------------|
| Jc | 4–16 | Junk packet count |
| Jmin | 50–256 | Min junk size (bytes) |
| Jmax | 300–1000 | Max junk size, Jmax > Jmin |
| S1 | 15–150 | Handshake init padding |
| S2 | 15–150 | Handshake response padding, S1+56 ≠ S2 |
| S3 | 8–64 | Cookie reply padding (AWG 2.0) |
| S4 | 6–31 | Transport data padding (AWG 2.0) |
| H1 | [5, 536870911] | Quadrant 1 |
| H2 | [536870912, 1073741823] | Quadrant 2 |
| H3 | [1073741824, 1610612735] | Quadrant 3 |
| H4 | [1610612736, 2147483647] | Quadrant 4 |

**DPI-evasion profiles:**
- **Level 1 (basic):** H1-H4 + S1-S4 + Jc/Jmin/Jmax — max speed, min latency
- **Level 2 (+I1):** Adds 1 CPS packet (QUIC Initial / SIP REGISTER / DNS Query)
- **Level 3 (+I1-I5):** Full CPS chain — max DPI bypass, handles TSPU/Revisor

**Performance:** Kernel module overhead < 12% vs plain WireGuard. CPS adds ~100ms at handshake only.

### MTProto/Telemt: Current Best Practices

**Fake-TLS (`ee` prefix) is mandatory:**
- Plain (`no prefix`) and obfuscated (`dd`) modes are blocked by modern DPI
- Secret format: `ee` + 32+ hex chars
- Domain selection: use same-DC domain (gosuslugi.ru for Russia, update.microsoft.com for global)

**Key evasion techniques (priority order):**
1. Fake TLS 1.3 (Chrome JA3 fingerprint emulation)
2. Dynamic Record Sizing (DRS) — randomize TLS record sizes
3. Port 443 only — indistinguishable from HTTPS
4. per-user secrets — avoid cross-user correlation
5. Anti-replay — reject replayed handshakes
6. Split-TLS / TCPMSS=88 — fragmentation for hardware DPI boxes
7. Zero-RTT Nginx masking — defeat active probes (TSPU Revisor)

**Limitation:** MTProto proxies are Telegram-only. Not for browsers or automation tools.


## What NOT to Do

- DO NOT modify 3x-ui core code without LUCX-HOOK markers
- DO NOT import `lucx/awg` from `web/service` (import cycle)
- DO NOT use `ctx.String()` or `ctx.HTML()` in LucX handlers — always `ctx.JSON()`
- DO NOT silently catch errors with `catch (_) {}`
- DO NOT use `||` for numeric defaults (0 is falsy) — use `??`
- DO NOT use `if (this.field)` in `toJson()` — use `!== undefined`
- DO NOT destroy settings in protocol setters without checking if protocol changed
- DO NOT skip `MergeParamsToSettings` when creating/modifying AWG inbounds
- DO NOT deploy without running the full test suite
- DO NOT commit `.env`, credentials, or API tokens
- DO NOT force-push or skip git hooks
- DO NOT use raw `axios.post` for LucX calls — use `postLucx`/`postLucxSafe`
- DO NOT add DB fields without corresponding migration and AWG_CHANGES.md entry
- DO NOT break existing plugins (including Суперсил)
- DO NOT leave PostUp/PostDown changes in the system after service stop
