# 05 — Rules

Extracted from AGENTS.md. This file is project law.

Rules 3 and 3b (AWG / tunnel architecture) live in `02-architecture.md`.

---

## The 11 Rules

### 0. Client config is sacred — STRICT (lucx.106+)

**Law:** a panel update (`x-ui update` / web-update / first start after upgrade) **MUST NOT** break already working inbounds or issued client configs.

**Never** change data that ends up in the user client config without an **explicit** operator action/request **on that server**.

This includes (non-exhaustive):
- tunnel **Address / AllowedIPs** (AWG, WireGuard) — changing IP = client must re-download .conf
- **keys** (private/public/PSK), secrets, UUID, password
- endpoint/host/port, obfuscation must-match fields (S/H/HPK), if already issued to the client
- enable/disable peers, peer routes, anything that breaks handshake/traffic without an operator click

**Strictly forbidden (even “only for broken ones” / “idempotent” / “for the greater good”):**
- startup- / InitDB- / migrate-on-boot that **rewrite** peer/client addresses, keys, or settings on a live DB
- auto-fix / re-allocate / “repair” IP on panel start or reconcile (lessons lucx.91→92, lucx.105→106)
- “silent” re-allocate on Update/save inbound/client “while we’re at it”
- any release migration that on foreign prod servers changes issued client params without opt-in

**Allowed:**
- allocate a **new** IP/keys **only** if the field is **empty** (new client / new attach with cleared AllowedIPs)
- change IP on an **explicit** inbound Address change via the operator **UI button** (and document it)
- **read** per-inbound IP for export/QR (do not mutate)
- schema/backfill **without** changing values (add a key with default, leave existing alone)

**Broken peer on the server:** fix **only** on operator request or opt-in button/SQL — **not** automatically on every host at update.

**Before merge/release the agent must ask:** “If a tester runs `x-ui update` on a live panel with a hundred clients — will any of them lose internet or have to re-download .conf without wanting to?” If yes — **do not** merge.

### 0b. Vanilla 3x-ui overlay is sacred — STRICT (lucx.119+)

**Law:** installing LucX-UI **on top of** a live vanilla 3x-ui DB (`/etc/x-ui/x-ui.db` without reinstall, `x-ui update` / install over) **MUST** bring the panel up and show existing clients/inbounds without a crash.

Typical tester path: MHSanaei/3x-ui is installed → installs LucX → opens Clients / Inbounds. Any regression here = “the fork won’t install”.

**Before merge/release the agent must ask:** “If we take a fresh SQLite from upstream 3.6.x with WireGuard clients and `wg_keep_alive INTEGER` and run our binary — do the Clients page and Xray start work?” If no — **do not** merge.

**Strictly forbidden:**
- Changing the Go field type mapped to an **existing** upstream column (`clients.*`, `inbounds.*`, …) **without** `sql.Scanner` / `driver.Valuer` (or equivalent) that accepts the driver’s **legacy form**. Lesson lucx.119: `KeepAlive int` → `KeepAliveValue string` without `Scan` → `unsupported Scan, storing driver.Value type int64 into type *model.KeepAliveValue` on every `Find(&[]ClientRecord)` — panel “Something went wrong”.
- Assuming GORM AutoMigrate will “widen” INTEGER→TEXT on SQLite by itself. On SQLite affinity is per-value: the column **stays** INTEGER, the driver returns `int64` for old rows. Need a Scanner, not “type:text in the tag”.
- Zod/frontend schemas that accept **only** the new type (e.g. `keepAlive: z.number()`) when the backend after a LucX write may return a string (`"25"` / `"15-25"`) — the Edit inbound form won’t save. Accept number|string (preprocess), like `normalizeAwgTimer` / `optionalKeepAlive`.
- Startup migrations that on a vanilla DB **mutate** client data (see Rule 0). Schema widen / backfill defaults / new empty tables — OK.

**Required when changing a column type / custom type:**
1. `Scan` (+ `Value` on write) for **all** forms the driver returns on a legacy DB: `nil`, `int64`, `[]byte`, `string`, and `float64` if needed.
2. Postgres: explicit `ALTER … TYPE text USING …` **before** AutoMigrate if a strict type won’t allow writing the new format (`migrate_awg_keepalive.go`).
3. Regression: unit Scan + (if CGO) `Find` on a table with an INTEGER column; JSON unmarshal vanilla `{"keepAlive":25}`; frontend parse number **and** string.
4. Do not rely on “fresh LucX DB is green” — test the overlay path separately.

**Allowed / fine on overlay:**
- New tables (`awg_outbounds`) empty, new optional settings keys with default.
- New `Protocol` oneof values (awg/naive/…) — do not break loading existing rows.
- Migrations `protocol='awg'` / `lucxTunnel_*` — no-op on vanilla.

**See Pattern 1n** (debug): Scan `wg_keep_alive`.

### 1. LUCX-HOOK Isolation

ALL changes to upstream 3x-ui files go inside `// LUCX-HOOK` / `// END LUCX-HOOK` markers. Never modify 3x-ui core code outside these markers without explicit instruction.

```go
// LUCX-HOOK: Description of what this does
// ... your code ...
// END LUCX-HOOK
```

```ts
// LUCX-HOOK: Description
// ... your code ...
// END LUCX-HOOK
```

Run `grep -rn "LUCX-HOOK" internal/ frontend/ install.sh` to find all integration points.

### 2. Isolated Modules

New functionality lives ONLY in:
- **Go:** `internal/awg/` — AWG sidecar (manager, process, instance, traffic, orphans)
- **Go:** `internal/lucx/` — subdirectories: `parser/`, `nodetype/`, `outbound_link/` (Smart Cluster), `tunnel/` (tunnel sidecars: NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel)
- **Go:** `internal/database/migrate_awg.go` — legacy DB migration
- **Go:** `internal/web/service/tunnel.go`, `internal/web/controller/tunnel.go`, `internal/web/job/tunnel_job.go` — tunnel sidecar web layer
- **Frontend:** `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema
- **Frontend:** `frontend/src/pages/inbounds/form/protocols/awg.tsx` — React form
- **Frontend:** `frontend/src/schemas/tunnel.ts`, `frontend/src/api/tunnels.ts`, `frontend/src/pages/tunnels/TunnelsPage.tsx` — tunnel sidecar UI
- **Shell:** `bin/install-awg-module.sh` — DKMS install

Integration points (`model.go`, `db.go`, `web.go`, `runtime/local.go`, `service/xray.go`, `install.sh`, `inbound-defaults.ts`, `InboundFormModal.tsx`, `protocols/index.ts`, `primitives/protocol.ts`, `protocols/inbound/index.ts`, `api.go`, `routes.tsx`, `AppSidebar.tsx`, `queryKeys.ts`, `endpoints.ts`) get LUCX-HOOK blocks only.

### 4. Paranoid Logging

Every critical operation logs with a prefix:
```
[LUCX-AWG]            — AWG service operations (legacy logAWG helper)
awg: <label> | <line> — sidecar process output (procLogWriter, matches mtproto)
```

### 5. No Telemt

The old LucX-UI had a `internal/lucx/telemt/` package for MTProto. Upstream replaced it with native `internal/mtproto/`. Do not re-add Telemt code; use the upstream MTProto implementation.

### 6. No tun2socks

The old architecture used a `tun2socks` userspace daemon to bridge the AWG kernel TUN to a hidden SOCKS5 inbound. The sidecar architecture makes it redundant — Xray supports a native TUN inbound (`injectAwgEgress`). Do not re-add tun2socks.

### 7. Test Discipline

- **Go:** `go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v`
- **Frontend:** `cd frontend && npm run typecheck && npm run lint`
- DB-dependent service tests require `CGO_ENABLED=1` (sqlite). Unit tests for AWG logic (instance, manager state, inject, stripHiddenKeys) run without cgo.
- Add tests for every new AWG function: instance parsing, fingerprint stability, config rendering, inject behavior, migration logic.

### 8. Upstream Sync

Procedure (validated on v3.5.0→v3.6.0, 103 upstream commits / 432 files / 7 conflicts):

1. `git fetch origin --tags`, branch off our current head, then `git merge --no-commit --no-ff origin/main`.
2. **Record the LUCX-HOOK marker count per file BEFORE resolving** (`git grep -c "LUCX-HOOK"`). After the merge, any file whose count dropped silently lost our code — that is the only reliable detector.
3. **Resolve conflicts ONLY from the terminal.** Editing a file while it is in conflict state makes the IDE rewrite it from its own merge cache and silently drop content (v3.6.0: `install.sh` lost all 16 LUCX-HOOK blocks, `db.go` lost the new upstream functions).
4. **Resolve block by block, never one blanket strategy.** Upstream usually *adds* code NEXT TO a HOOK block rather than replacing it, so a wholesale `--ours` yields uncompilable code (v3.6.0: `undefined: database.BackupSQLite`). Rule of thumb: take **both** sides when upstream added a sibling call/test/field; take **ours** only where our block is a deliberate substitution (fork URLs, `prerelease: false`).
5. Verify: `go build ./...`, `go vet ./...`, `go test ./internal/awg/... ./internal/lucx/...`, `bin/check-lucx.sh`, and frontend `lint` + `typecheck` + `vitest run --project=unit` + `build`.
6. If upstream added a new cross-cutting invariant test, satisfy it for our code too — v3.6.0 introduced `frontend/src/test/i18n-dead-keys.test.ts`, which demands every locale carry the exact en-US key set (our AWG keys had lived only in en+ru).

Windows caveat: `internal/database` needs CGO (`sqlite3.Backup`), so `go build ./...` fails there without gcc — reproducible on pristine upstream, not our regression. Release binaries are Linux-only with `CGO_ENABLED=1` anyway.

### 9. Frontend Stack

Upstream rewrote the frontend from Vue to React + TypeScript + AntD v6 + Zod. AWG follows the same pattern:
- `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema (`AwgInboundSettingsSchema`), includes `mimicryProfile`, `browserProfile`, `outboundTag`, `routeThroughXray`
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` — AntD form (`AwgFields`), uses `useFormContext` (react-hook-form), `FormField` (not `Form.Item`), `message.useMessage()` (not `App.useApp()`)
- `frontend/src/lib/xray/inbound-defaults.ts` — `createDefaultAwgInboundSettings` (LUCX-HOOK)
- `frontend/src/lib/xray/inbound-link.ts` — `genAwgLink`/`genAwgConfig` (share-link + .conf generation, I1-I5 written as-is, no double CPS tag wrapping)
- `frontend/src/pages/clients/wireguardConfig.ts` — `buildAwgClientConfig` (full client .conf with obfuscation block)
- `frontend/src/pages/clients/ClientQrModal.tsx` — AWG panel with QR + download
- Registered in `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`

### 10. License

LucX-UI components (`internal/awg/`, `internal/awg/cps/`, `internal/awg/signature/`, `internal/lucx/`, `internal/database/migrate_awg*.go`, `internal/web/controller/awg.go`, `internal/web/controller/awg_outbound.go`, `internal/web/controller/lucx.go`, `internal/web/controller/tunnel.go`, `internal/web/job/awg_job.go`, `internal/web/job/tunnel_job.go`, `internal/web/service/client_awg.go`, `internal/web/service/awg_outbound.go`, `internal/web/service/tunnel.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/schemas/tunnel.ts`, `frontend/src/api/tunnels.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `frontend/src/pages/inbounds/form/protocols/mieru.tsx`, `frontend/src/pages/inbounds/form/protocols/trusttunnel.tsx`, `frontend/src/schemas/protocols/inbound/mieru.ts`, `frontend/src/schemas/protocols/inbound/trusttunnel.ts`, `frontend/src/pages/inbounds/form/awg-inbound-id-context.ts`, `frontend/src/lib/mieru/presets.ts`, `frontend/src/pages/tunnels/TunnelsPage.tsx`, `frontend/src/pages/clients/wireguardConfig.ts`, `bin/install-awg-module.sh`, `bin/check-lucx.sh`, `bin/pre-push`, `bin/build-release.sh`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use (including VPN resale) requires explicit written permission from the author.

Original 3x-ui code remains under GPL-3.0.

**Every new LucX-owned file MUST carry the SPDX header** (see any existing file in `internal/awg/` for the exact 5-line block). Files with `//go:build` tags put the header after the constraint line; shell scripts after the shebang. The full split (which files are PolyForm vs GPL, why, commercial contact) is documented in [LICENSING.md](LICENSING.md); the canonical license text is [LICENSE-PolyForm-Noncommercial.txt](LICENSE-PolyForm-Noncommercial.txt). Upstream files with LUCX-HOOK blocks stay GPL — never put SPDX headers in them.

### 11. Documentation language — English

**Law:** agent and project documentation is **English only**.

Write in English:
- `.agents/*`, `progress.md`, `LICENSING.md`, and any other agent/project ops docs
- Architecture Map, Known Issues, Debug Patterns, workflow notes
- New entries in `progress.md` and edits to existing doc prose

Exceptions (do not “translate away”):
- **Commit messages** — Russian (see Commit Convention), unless asked otherwise
- **Release notes** — RU + EN blocks for operators (see Release notes style)
- **UI i18n** — locale files under `internal/web/translation/` stay multi-language
- Quoted user/tester phrases, log lines, or fixed product strings that are inherently non-English

Do not mix Russian/Chinese prose into `.agents/` or progress.md. English keeps one style and saves tokens for every agent session.
