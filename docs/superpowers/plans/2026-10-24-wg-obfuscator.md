# WG Obfuscators (wgobfs) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the spec `docs/superpowers/specs/2026-10-24-wg-obfuscator-design.md`: obfuscation presets on the kernel WireGuard inbound + a standalone `wgosocks` SOCKS5-exit inbound, both driven by one GPL sidecar binary `wgobfs` (Phobos fork of ClusterM wg-obfuscator).

**Architecture:** New tunnel cores `wgo` / `wgosocks` in `internal/lucx/tunnel/` (one binary, two Name values — caddy-naive precedent). WG presets render an INI config with `mode=wireguard` sections targeting `127.0.0.1:<wgPort>`; lifecycle rides the kernel-WG job + service apply hooks. `wgosocks` is a standard inbound-only sidecar (mieru shape) with `socks5-users` from panel clients. Client exports (`[instance]` + `phobos://`) are frontend builders in `frontend/src/lib/wgo/link.ts`.

**Tech Stack:** Go 1.x (existing module), React + TS + AntD v6 + Zod + react-hook-form (upstream stack), vitest, iptables (owner-commented rules), C sidecar built by `bin/pack-sidecars.sh` on CI/Linux only.

## Global Constraints

- Spec of record: `docs/superpowers/specs/2026-10-24-wg-obfuscator-design.md`. Engine config keys/masking values verbatim from spec §2.1: `mode`, `role`, `source-lport`, `target`, `key`, `masking` ∈ STUN|MEDIA|TLS|AUTO|NONE, `media-ssrc`, `media-pt`, `media-clock`, `obfuscate-bytes`, `max-dummy` (0..1024), `idle-timeout` (server INI = ms), `max-clients`, `socks5-users`.
- Rule 0: never mutate issued client data; `blockDirectPort` default false; WG port stays open unless the operator ticks it.
- Rule 0b: `obfuscation` settings key optional; `wgosocks` is a new Protocol oneof value; vanilla DB must load with the feature hidden.
- Rule 1: upstream-file changes only inside `LUCX-HOOK` / `END LUCX-HOOK`.
- Rule 10: every new LucX file carries the PolyForm SPDX header (after `//go:build` if any). No AGPL Phobos panel code is copied — formats only, from the spec.
- Rule 11: docs in English; commit messages in Russian (`feat(sidecars): …` style); UI strings via i18n keys ×13 locales.
- Tests: `go test ./internal/lucx/... ./internal/database/... -count=1` (no CGO needed for these packages), `cd frontend && npm run typecheck && npm run lint && npx vitest run --project=unit`. `bin/check-lucx.sh` before push.
- Binary is built by CI/Linux (`pack-sidecars.sh`); on the Windows dev box we do not build `wgobfs` — unit tests cover config rendering only.

---

### Task 1: Engine core — tunnel `wgo.go` (types, validation, WG-conf render)

**Files:**
- Create: `internal/lucx/tunnel/wgo.go`
- Test: `internal/lucx/tunnel/wgo_test.go`

**Interfaces (produced, used by Tasks 3–7):**
```go
type WgoPreset struct {
    ID string; Name string; ExtPort int; Key string; Masking string
    MediaSsrc string; MediaPt int; MediaClock int
    MaxDummy int; IdleTimeout int; ClientWgLocalPort int; IsDefault bool
}
type WgoObfuscation struct { Enabled bool; BlockDirectPort bool; Presets []WgoPreset }
func ParseObfuscation(ib *model.Inbound) (WgoObfuscation, bool)
func ValidateObfuscation(o WgoObfuscation, wgPort int, reserved func(port int, proto string) bool) error
func RenderWgoConf(o WgoObfuscation, wgPort int) string   // INI, sections named by preset
func InstanceSection(p WgoPreset, host string) string      // client [instance] block
func PhobosLink(conf, name string) string                  // phobos://<b64url>#<name>
```

- [ ] **Step 1: write failing tests** — `wgo_test.go` covering: `ParseObfuscation` (absent key → false; malformed JSON → false; full preset round-trip), `ValidateObfuscation` (dup extPort, extPort == wgPort, bad masking, key empty/>255, MEDIA allows ssrc, TLS rejected in WG mode, idle/dummy ranges, reserved-port callback reject), `RenderWgoConf` golden:

```go
const goldenWgoConf = `[phobos-ru]
mode = wireguard
source-lport = 51822
target = 127.0.0.1:51820
key = k1
masking = STUN
max-dummy = 4
idle-timeout = 300000
verbose = 1

`
```
(sections sorted by name; MEDIA adds `media-ssrc`/`media-pt`/`media-clock` only when non-zero/non-empty; `verbose = 1`.)

- `PhobosLink("x", "Клиент 1")` → `phobos://eA==<no padding, url-safe>#%D0%9A...` (assert prefix + no `=` + fragment percent-encoded).
- `InstanceSection` golden per spec §2.2 (source-if/source-lport/target/key/masking/verbose=INFO/idle-timeout=<sec>/max-dummy, media-ssrc only when pinned).
- [ ] **Step 2: run** `go test ./internal/lucx/tunnel/ -run TestWgo -count=1` → FAIL (undefined).
- [ ] **Step 3: implement** `wgo.go` (PolyForm header; masking constants `WgoMaskSTUN = "STUN"` etc.; section name = sanitized preset name: lowercase [a-z0-9-], fallback `presetN`; idle-timeout stored in seconds in settings, rendered ×1000 ms in server conf, seconds in client `[instance]` per spec §10.2 — single source helpers `idleMs()/idleSec()`).
- [ ] **Step 4: tests green.**
- [ ] **Step 5: commit** `feat(sidecars): ядро wgo — типы, валидация, рендер конфигов wgobfs`.

### Task 2: Register cores `wgo`/`wgosocks` in the tunnel Manager

**Files:**
- Modify: `internal/lucx/tunnel/tunnel.go` (Name consts, `All()`, `Valid()`, `DisplayName()`, `BinaryName()` → both `wgobfs-%s-%s`)
- Modify: `internal/lucx/tunnel/manager.go` (start() case `args = []string{"-c", cfgPath}`; `ReconcileWgo`/`ReconcileWgosocks` mirroring `ReconcileMieru` with sweep prefixes `wgo-` / `wgosocks-`)
- Test: extend `internal/lucx/tunnel/manager_test.go`

**Interfaces (produced):** `const Wgo Name = "wgo"`, `const Wgosocks Name = "wgosocks"`; `func WgoKey(id int) string` (`wgo-<id>`), `func WgosocksKey(id int) string`; binary name `wgobfs-linux-amd64|arm64`; both cores share it.
**Consumes:** Task 1 types.

- [ ] **Step 1: failing tests** — `Wgo.Valid()`, `Wgosocks.Valid()`, `DisplayName()` ("WG obfuscator", "WG obfuscator SOCKS5"), `BinaryName()` equality for both cores, `startArgs` render `-c <conf>` (reuse the manager argv test helper pattern; if argv is not directly testable, assert via a tiny exported `StartArgs(core, path)` extracted in this task).
- [ ] **Step 2: FAIL** → **Step 3: implement** (comment in `All()` doc: engine = Ground-Zerro/Phobos `src/phobos-obfuscator`, GPL-3.0, ClusterM derivative; qWDTT precedent).
- [ ] **Step 4: green** (`go test ./internal/lucx/tunnel/ -count=1`).
- [ ] **Step 5: commit** `feat(sidecars): регистрация ядра wgobfs в tunnel-Manager (wgo/wgosocks)`.

### Task 3: `wgosocks` inbound — settings, render, instance

**Files:**
- Create: `internal/lucx/tunnel/wgo_inbound.go`
- Test: `internal/lucx/tunnel/wgo_inbound_test.go`

**Interfaces (produced, used by Tasks 6–8):**
```go
type WgosocksClient struct { Email, Login, Password string; Enable *bool }
type WgosocksSettings struct {
    Port int; Key string; Masking string
    MediaSsrc string; MediaPt int; MediaClock int
    MaxDummy int; IdleTimeout int; MaxClients int
    Clients []WgosocksClient
}
func WgosocksConfigFromInbound(ib *model.Inbound) (WgosocksSettings, bool)
func ValidateWgosocks(s WgosocksSettings, reserved func(port int, proto string) bool) error
func RenderWgosocksConf(s WgosocksSettings) string          // mode=socks5 role=server + socks5-users
func WgosocksInstanceFromInbound(ib *model.Inbound) (Instance, bool) // ProbePort = Port (TCP probe)
func WgosocksClientConf(s WgosocksSettings, host string, c WgosocksClient) string // [instance]+[socks5] payload
```

- [ ] **Step 1: failing tests** — golden conf:

```ini
[wgosocks]
mode = socks5
role = server
source-lport = 51862
key = sk
masking = MEDIA
max-dummy = 4
idle-timeout = 300000
verbose = 1
socks5-users = alice:pw1,bob:pw2
```
  (users only enabled clients, order stable by login; masking TLS/AUTO/NONE/MEDIA valid; validation: port 1..65535, key rules, client login/password non-empty ≤255, duplicates rejected, count ≤ 512 (panel-side cap until the engine MAX_SOCKS5_USERS is verified on Linux — spec §10.1); `WgosocksClientConf` golden per spec §2.2 incl. `[socks5]` section and `source-lport = 1080`).
- [ ] **Step 2: FAIL** → **Step 3: implement** → **Step 4: green.**
- [ ] **Step 5: commit** `feat(sidecars): инбаунд wgosocks — настройки, рендер role=server, клиентский конфиг`.

### Task 4: Protocol `wgosocks` in the model

**Files:**
- Modify: `internal/database/model/model.go` (LUCX-HOOK block: `Wgosocks Protocol = "wgosocks"` + validate oneof append `wgosocks`)

- [ ] **Step 1:** add const with comment (SOCKS5-exit sidecar, multi-client login/password, engine wgobfs GPL-3.0). Append to the oneof list at `model.go:182`.
- [ ] **Step 2:** `go build ./internal/database/...` (CGO note from Rule 8 on Windows — `go vet` suffices if gcc missing) + existing protocol tests if any reference oneof.
- [ ] **Step 3: commit** `feat(model): протокол wgosocks в oneof инбаундов (LUCX-HOOK)`.

### Task 5: kernel WG obfuscation integration (job + service + firewall)

**Files:**
- Create: `internal/lucx/wireguard/obfuscator.go` (+ `_test.go`)
- Create: `internal/lucx/wireguard/obfuscator_firewall_linux.go`, `obfuscator_firewall_other.go`
- Modify: `internal/web/job/kernel_wireguard_job.go` (extend interfaces + Run: build wgo instances, `tunnel.GetManager().Ensure`, sweep `wgo-` orphans via `StopPrefixed`+`sweepOrphanConfigFiles` path used by ReconcileWgo)
- Modify: `internal/web/service/kernel_wireguard.go` (validate: call `tunnel.ValidateObfuscation` with the existing `checkPortConflict` reserved-callback shape; apply/remove: Ensure/StopKey so saves apply immediately, not on the 10s tick)

**Interfaces:**
```go
// internal/lucx/wireguard/obfuscator.go
func ObfuscationInstance(ib *model.Inbound) (tunnel.Instance, bool) // wraps Parse + RenderWgoConf + WgoKey
func EnsureObfuscation(ib *model.Inbound) error                     // Manager Ensure + firewall
func StopObfuscation(inboundID int) error                           // StopKey + firewall remove
func EnsureWgoFirewall(inst Instance, blockDirect bool) error       // linux; other = no-op
```
Firewall (linux): owner-commented iptables rule mirroring `inputRuleArgs` shape: `iptables -w 5 {-I INPUT 1|-D INPUT} -p udp --dport <wgPort> ! -i lo -m comment --comment "lucx-wgo-<id>" -j DROP`; ensure = `-C` then `-I`; remove best-effort `-D`; `exec.ErrNotFound` → warn-once like `ensureInput`.

- [ ] **Step 1: failing tests** — `ObfuscationInstance` from a fixture inbound (enabled/disabled/absent), conf golden reuses Task 1; firewall args builder unit test (pure function `dropRuleArgs(port int, id int, op string) []string`) covering `-I` vs `-D` forms; job-level test: fake manager + fake tunnel manager interfaces (extend the existing `kernelWireguardManager` test fakes pattern) asserting wgo Ensure called for obfuscated enabled inbound and StopKey for removed.
- [ ] **Step 2: FAIL** → **Step 3: implement.** Job Run addition (after `ReconcilePreserving`): loop rows again for `kernelwg.IsKernel && ib.Enable && NodeID==nil` → `EnsureObfuscation`; plus `ReconcileWgo` sweep with wanted keys. Service: in `validateKernelWireguard` tail add obfuscation validation; in `applyLocalKernelWireguard` add `EnsureObfuscation`; find the kernel-WG delete path (`service/inbound.go` DelInbound local apply) and add `StopObfuscation(id)` beside the existing interface removal (LUCX-HOOK).
- [ ] **Step 4:** `go test ./internal/lucx/... ./internal/web/job/... -count=1` (job tests without CGO).
- [ ] **Step 5: commit** `feat(wg): обфускация kernel-WG — пресеты wgo, job-reconcile, blockDirectPort`.

### Task 6: `wgosocks` lifecycle + traffic in tunnel job

**Files:**
- Modify: `internal/web/job/tunnel_job.go` (wgosocks branch: mieru shape — filter `model.Wgosocks && Enable && NodeID==nil`, `WgosocksInstanceFromInbound` → Manager.Ensure/ReconcileWgosocks; traffic `CollectTproxyTraffic(WgosocksKey(id), ib.Tag)` proc-io deltas; sole-enabled-client attribution (email only when exactly one enabled client, else inbound rollup); `LiveTags` for activeTags; slim `settings.clients` injection like mieru so share-only clients appear)
- Test: `internal/web/job/tunnel_job_wgosocks_test.go`

- [ ] **Step 1: failing test** — fake store with one wgosocks inbound (two clients) → Ensure called with rendered conf; disabled inbound → instance disabled; traffic delta from fake proc-io (tproxy test pattern) attributes to the sole client email when single-client.
- [ ] **Step 2: FAIL** → **Step 3: implement** → **Step 4: green** (`go test ./internal/web/job/... -count=1`).
- [ ] **Step 5: commit** `feat(sidecars): жизненный цикл и трафик wgosocks в tunnel-джобе`.

### Task 7: wgo / wgosocks controller endpoints (status/logs)

**Files:**
- Modify: `internal/web/controller/tunnel.go` — pattern of mieru/anytls: `GET /panel/api/tunnel/wgosocks/status/:id`, `/logs/:id` returning `Manager.StatusOf` + `LogsKey` (JSON_HEADERS). Plus the WG-attached engine: `GET /panel/api/inbounds/:id/wgoStatus` and `/wgoLogs` (spec §8) resolving key `wgo-<id>` — status + last-log lines for the preset status line in the WG form (Task 9 consumes). Verify the existing route registration shape in `controller/tunnel.go` + `web.go` during implementation and follow it verbatim (mieru precedent).
- Test: extend the tunnel controller test file for the new endpoints (in-memory manager fake).

- [ ] **Step 1: failing route test** → **Step 2: FAIL** → **Step 3: implement** → **Step 4: green.**
- [ ] **Step 5: commit** `feat(sidecars): API статуса/логов wgo и wgosocks`.

### Task 8: Frontend — Zod schema + protocol registration

**Files:**
- Create: `frontend/src/schemas/protocols/inbound/wgosocks.ts` (`WgosocksSettingsSchema`: port 1..65535, key 1..255, masking enum MEDIA|TLS|AUTO|NONE, media fields optional, maxDummy 0..1024 default 4, idleTimeout default 300, maxClients default 1024, clients array {email, login 1..255, password 1..255, enable default true}; `WgosocksInboundSettingsSchema` with defaults)
- Create: `frontend/src/pages/inbounds/form/protocols/wgosocks.tsx` (`WgosocksFields`: port/key/masking selects/media fields + clients table with add/regenerate-password — mirror mieru.tsx/anytls.tsx structure, `useFormContext`, `FormField`, `message.useMessage()`)
- Modify (LUCX-HOOK): `frontend/src/schemas/primitives/protocol.ts` (list + map entry `Wgosocks: 'wgosocks'`), `frontend/src/pages/inbounds/form/protocols/index.ts` (export), `frontend/src/lib/xray/inbound-defaults.ts` (`createDefaultWgosocksSettings`), `frontend/src/pages/inbounds/form/InboundFormModal.tsx` (case → WgosocksFields), inbound schema union index.

- [ ] **Step 1: schema + form + registrations** (form code mirrors anytls.tsx: masking Select from `WGO_MASKINGS_SOCKS`, clients editable via Form.List, password Regenerate button using `crypto.getRandomValues` base62 16 chars).
- [ ] **Step 2:** `npm run typecheck && npx vitest run --project=unit` (schema parse tests: defaults, invalid masking rejected, client dedupe) → green.
- [ ] **Step 3: commit** `feat(frontend): форма и схема инбаунда wgosocks`.

### Task 9: Frontend — WG obfuscation block in the WireGuard form

**Files:**
- Modify: `frontend/src/pages/inbounds/form/protocols/wireguard.tsx` (inside the existing kernel LUCX-HOOK area): switch "Обфускация" bound to `settings.obfuscation.enabled`, presets Form.List (name/extPort/key+generate/masking select STUN|MEDIA|AUTO|NONE/media fields when MEDIA/maxDummy/idleTimeout/clientWgLocalPort/isDefault radio), `blockDirectPort` checkbox (default false) with hint text; hidden when `settings.kernel` is false.
- Modify: `frontend/src/schemas/protocols/inbound/wireguard.ts` (extend existing kernel schema with optional `obfuscation` object — optional so old inbounds parse unchanged).
- Test: extend `frontend/src/test/kernel-wireguard.test.ts` (defaults off, preset add/render, MEDIA shows media fields, validation errors).

- [ ] **Step 1: failing vitest** → **Step 2: FAIL** → **Step 3: implement** → **Step 4: typecheck+lint+vitest green.**
- [ ] **Step 5: commit** `feat(frontend): блок обфускации в форме kernel-WireGuard (пресеты wgobfs)`.

### Task 10: Frontend — client exports (`[instance]`, `phobos://`, ClusterM INI)

**Files:**
- Create: `frontend/src/lib/wgo/link.ts`:
  ```ts
  export interface WgoPresetLike { name: string; extPort: number; key: string; masking: string;
    mediaSsrc?: string; mediaPt?: number; mediaClock?: number; maxDummy: number; idleTimeout: number;
    clientWgLocalPort: number }
  export function instanceSection(p: WgoPresetLike, host: string): string   // [instance] …
  export function phobosLink(conf: string, name: string): string            // b64url no padding
  export function clustermIni(p: WgoPresetLike, host: string): string       // [main] standalone INI
  export function obfuscatedEndpoint(p: WgoPresetLike): string              // 127.0.0.1:<clientWgLocalPort>
  export function wgosocksClientConf(host: string, login: string, password: string, p: {port:number;key:string;masking:string}): string
  ```
- Modify: `frontend/src/pages/clients/wireguardConfig.ts` (when peer has `preset` and inbound has obfuscation: build variant confs), `frontend/src/lib/xray/inbound-link.ts` (wgosocks share: `genWgosocksLink` → `phobos://…` from client login/password), `ClientQrModal.tsx` / `ClientInfoModal.tsx` (preset select per WG client writing `preset` field; export tabs: `.conf` download, `phobos://` copy+QR, ClusterM INI when masking ∈ NONE|STUN; warning alert key `wgoHelperWarning`), client form preset Select for wireguard-kernel inbounds.
- Test: `frontend/src/test/wgo-link.test.ts` — goldens mirroring the Go goldens (same fixtures), b64url edge cases, `none`-padding rule applies ONLY inside phobos payload (conf download never padded).

- [ ] **Step 1: failing tests with exact golden strings** → **Step 2: FAIL** → **Step 3: implement** → **Step 4: green + typecheck + lint.**
- [ ] **Step 5: commit** `feat(frontend): экспорт клиентов — [instance], phobos://, ClusterM INI`.

### Task 11: i18n ×13 locales

**Files:** `internal/web/translation/*.json` (all 13).

- [ ] **Step 1:** collect the key set used by Tasks 8–10 (form labels, hints, export buttons, warning; prefixed `wgo*` / `wgosocks*`). Write en + ru fully; insert the same keys into the other 11 locales with en values via a byte-stable Node script (AWG precedent: insert after a known anchor key, keep JSON valid, en values for untranslated technical strings).
- [ ] **Step 2:** `npx vitest run --project=unit -- i18n-dead-keys` (upstream invariant test: every locale carries the exact en-US key set) → green.
- [ ] **Step 3: commit** `feat(i18n): ключи wgo/wgosocks ×13 локалей`.

### Task 12: Sidecar build + pins + licensing/attribution docs

**Files:**
- Modify: `bin/pack-sidecars.sh` — build `wgobfs-linux-amd64|arm64`: `git clone --depth 1` Phobos repo at pinned SHA into a temp dir, `make -C src/phobos-obfuscator` (CC/musl flags follow the script's existing sidecar build conventions; arm64 via the script's cross path), copy to `third_party/sidecars/linux-{arch}/wgobfs-linux-{arch}.gz`.
- Modify: `bin/lucx-pins.txt` — `wgobfs-linux-amd64|arm64  <sha256>  src: Ground-Zerro/Phobos@<SHA> (src/phobos-obfuscator, GPL-3.0; derived from ClusterM/wg-obfuscator)`.
- Modify: `docs/LICENSING.md` — engine entry (GPL-3.0 external binary, not linked; LucX integration code PolyForm).
- Modify: `README.md` — third-party credits block (ClusterM — wg-obfuscator; Ground-Zerro — Phobos fork core, preset model, phobos:// format; client ecosystem links).
- Create: `docs/content/docs/en/config/wg-obfuscator.mdx` — operator guide: how it works, presets/masking table, which client for which masking (fork clients for MEDIA/TLS/SOCKS5; ClusterM upstream for NONE/STUN), blockDirectPort, wgosocks client setup (`socks5://login:pass@127.0.0.1:1080`), traffic-accounting limitation, troubleshooting (logs endpoints).
- Modify: `.agents/02-architecture.md` (engine entry in §3b), `.agents/08-hooks.md` (new/changed hook files), `docs/progress.md` (entry, English).

- [ ] **Step 1:** implement all file changes; pins `TBD-until-CI` is NOT allowed — compute sha256 only if the binary is built on this machine is impossible (Windows): instead add the build step + leave pin computation to the release pipeline step that already hashes sidecars (`pack-sidecars.sh` writes `lucx-pins.txt` — verify how existing pins are produced and follow exactly; if pins are committed by the pack script, the CI run refreshes them in the release PR).
- [ ] **Step 2:** `bash -n bin/pack-sidecars.sh` syntax check; `bin/check-lucx.sh`.
- [ ] **Step 3: commit** `feat(build): сборка wgobfs из Phobos-форка + пины; docs: лицензии/атрибуция/операторская`.

### Task 13: Overlay regression + full gate

- [ ] **Step 1:** unit test `internal/database` or lucx: a vanilla inbound settings JSON (no `obfuscation`, protocol wireguard) parses with the new schema/code paths untouched; `wgosocks` oneof does not break existing rows (read-only scan test).
- [ ] **Step 2:** full gates: `go test ./internal/lucx/... ./internal/database/... -count=1` (CGO-free set), `cd frontend && npm run typecheck && npm run lint && npx vitest run --project=unit`, `bin/check-lucx.sh`.
- [ ] **Step 3: commit** `test: overlay-регрессия wgo/wgosocks (vanilla DB)`.

### Task 14: Release

- [ ] **Step 1:** follow `.agents/06-workflow.md` (read it before acting): bump `lucxVersion` in `internal/config/config.go` (next lucx.N), final `docs/progress.md` entry (English, includes E2E remaining-work note: live wgobfs run + Phobos-client interop verified on the test stand, not on the dev box).
- [ ] **Step 2:** `bin/pre-push` gate; commit `release: lucx.N — обфускаторы WireGuard (wgobfs: пресеты на kernel-WG + wgosocks)`.
- [ ] **Step 3:** push/tag only if 06-workflow.md's release procedure requires it and the user's "релизь" instruction covers it (it does — follow the documented release steps exactly).
