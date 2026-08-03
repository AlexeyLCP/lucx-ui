# LucX-UI — Agent Operating Manual

This file is the law for every agent working on this project. Read it completely before touching any code.

---

## Project Overview

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (currently **v3.6.0**) that adds native AmneziaWG (AWG) support as a kernel-interface sidecar, mirroring upstream's MTProto (mtg) sidecar architecture. LucX-specific code lives in `internal/awg/` and `internal/lucx/`; all integration points in upstream files are wrapped in `LUCX-HOOK` / `END LUCX-HOOK` markers.

**Upstream sync strategy:** **merge** `origin/main` (not rebase, not fresh-checkout). The v3.5.0→3.6.0 sync proved the isolation works: of 432 upstream-changed files only **7** conflicted, so a plain `git merge origin/main` is now the procedure — see Rule 8. The merge commit keeps upstream history, so each next sync is incremental. The old `.patch`-file system is gone; integration is inline.

**Remotes:**
- `origin` → `MHSanaei/3x-ui` (upstream)
- `gh` → `AlexeyLCP/lucx-ui` (our fork)

**Active branch:** `main` (миграция v3.6.0 завершена, релиз v3.6.0-lucx.49; текущий `lucxVersion` смотри в `internal/config/config.go`).

---

## Core Philosophy

**Minimal invasion for easy upstream sync.** The goal is: every upstream release should be a near-trivial port. This means:
- LucX code lives in isolated packages (`internal/awg/`, `internal/lucx/`), not scattered across upstream files.
- Upstream files get ONLY `LUCX-HOOK` blocks — never free-form edits.
- The AWG sidecar should be as thin as the MTProto sidecar. If mtproto does it in N files, AWG should aim for N too. (Known Issue #1 is closed — core package is at 9 files, exact parity with mtproto.)

**AWG sidecar = mtproto pattern.** AWG runs as a kernel-interface sidecar exactly symmetric with `internal/mtproto/`:

```
mtproto:  mtg sidecar (userspace)  → TCP → SOCKS loopback inbound → Xray routing
AWG:      awg kernel module        → IP   → TUN inbound             → Xray routing
```

---

## Workflow: How an Agent Executes a Task

```
1. READ    → Read AGENTS.md, progress.md, git log --oneline -15, check latest state
2. AUDIT   → Read all relevant files, trace data flow end-to-end
3. PLAN    → Write a short plan: which files, what changes, what tests
4. BRANCH  → Work on `main` (активная ветка, миграция v3.6.0 слита)
5. CODE    → Implement changes inside LUCX-HOOK blocks in upstream files;
             new code goes in internal/awg/ or internal/lucx/
6. TEST    → Run tests:
               go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v
               cd frontend && npm run typecheck && npm run lint
7. BUILD   → Frontend: cd frontend && npm run build
             Backend:  go build -o /tmp/x-ui .
                       (requires frontend/dist to exist for //go:embed)
8. DEPLOY  → SCP to vps_finland_lucx, restart x-ui.service
9. VERIFY  → Check `sudo systemctl status x-ui`, check server logs
10. COMMIT → `git add` specific files, `git commit` with descriptive message (Russian)
11. STATUS → Output `git status` and `git log --oneline -15` after commits
11.5. CHECK PR/ISSUES → ПЕРЕД пушем ВСЕГДА проверяй открытые PR и issues:
             `gh pr list --repo AlexeyLCP/lucx-ui --state open`
             `gh issue list --repo AlexeyLCP/lucx-ui --state open`
             Если есть необработанные PR (не от тебя) или issues — НЕ пушь
             сразу. Сообщи пользователю: какие PR/issues открыты, кем, и
             предложи: (а) сначала проверить/смержить PR, (б) сначала
             исправить issue, (в) пушить после. Не пушь молча поверх
             чужого PR — можно затереть или сломать чужую работу.
12. DOCS   → ВСЕГДА актуализируй progress.md и AGENTS.md. Каждый коммит — новая
             запись в progress.md (что сделано, какой lucxVersion, какие файлы,
             какие тесты). При изменении архитектуры — обнови AGENTS.md
             (Architecture Map, Known Issues, Debug Patterns). НЕ оставляй
             пробелов: если сделал фикс — запиши его. Файлы — закон проекта.
```

---

## The 10 Rules

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
- **Go:** `internal/lucx/` — subdirectories: `parser/`, `nodetype/`, `outbound_link/` (Smart Cluster)
- **Go:** `internal/database/migrate_awg.go` — legacy DB migration
- **Frontend:** `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema
- **Frontend:** `frontend/src/pages/inbounds/form/protocols/awg.tsx` — React form
- **Shell:** `bin/install-awg-module.sh` — DKMS install

Integration points (`model.go`, `db.go`, `web.go`, `runtime/local.go`, `service/xray.go`, `install.sh`, `inbound-defaults.ts`, `InboundFormModal.tsx`, `protocols/index.ts`, `primitives/protocol.ts`, `protocols/inbound/index.ts`) get LUCX-HOOK blocks only.

### 3. AWG Sidecar Architecture (mirrors mtproto)

AWG runs as a kernel-interface sidecar managed by `internal/awg.Manager`, exactly symmetric with `internal/mtproto.Manager`:

- **Manager** (`internal/awg/manager.go`): singleton with `Ensure`/`Reconcile`/`StopAll`/`CollectTraffic`/`SyncPeers`, fingerprint-based restart on config change, orphan sweep at first call. Reconcile-loop convergence: `ensureXrayRouting` (routeThroughXray: table/rule into tunN, dies with tunN on Xray restart) + `ensureNatRules` (kernel NAT: MASQUERADE/FORWARD, dies on iptables flush — fail2ban/docker).
- **Process** (`internal/awg/process.go`): wraps `awg-quick up/down` (kernel interface lifecycle, not a daemon). No tun2socks — routing is via Xray TUN inbound.
- **Instance** (`internal/awg/instance.go`): desired runtime state + `InstanceFromInbound` + `fingerprint`.
- **Traffic** (`internal/awg/manager.go`, влито из traffic.go): `awg show <iface> transfer` parsing for per-peer byte accounting (replaces mtg's Prometheus HTTP scrape).
- **Diagnostics** (`internal/awg/diagnostics.go`): read-only probe chain (interface UP, ip_forward, peers/handshakes, then mode-specific: MASQUERADE+FORWARD or tunN+rule+table). `Diagnose(inst)` → ordered `DiagCheck`s with evidence details; served by `GET /panel/api/inbounds/:id/awgDiagnostics` and rendered by the AWG form's diagnostics modal. Fixes belong to reconcile — diagnostics only makes failures visible.
- **Platform** (`internal/awg/platform_{linux,other}.go`): `defaultRouteInterface()` for MASQUERADE target + sweep of orphaned awg interfaces from a previous x-ui run.
- **Job** (`internal/web/job/awg_job.go`): cron `@every 10s` — Reconcile desired inbounds + fold inbound/per-client traffic deltas + RefreshLocalOnlineClients (AWG online status comes from fresh handshakes, not Xray stats).
- **Egress** (`internal/web/service/xray.go:injectAwgEgress`): inject TUN inbound into generated Xray config when `routeThroughXray` is set, symmetric with `injectMtprotoEgress`. Per-inbound gateway `10.254.(N%254).1/30` (separate /30 subnet, never conflicts with AWG tunnel subnet). Sniffing `{http,tls,quic, routeOnly:true}` on TUN inbound so domain/geosite rules work for AWG traffic.
- **Runtime** (`internal/web/runtime/local.go`): delegate AWG `AddInbound`/`DelInbound` to `awg.GetManager()`; `AddUser`/`RemoveUser` are no-ops (peer sync via Reconcile).
- **CPS** (`internal/awg/cps/`): CPS packet generators (TLS/DNS/SIP/QUIC) + AWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4). TLS and QUIC have browser-specific fingerprints (Chrome/Firefox/Safari).
- **Signature** (`internal/awg/signature/`): QUIC host capture — sends QUIC Initial to UDP 443, reads replies → I1-I5.
- **Controller** (`internal/web/controller/awg.go`): `generateObfuscation` + `captureHost` + `awgDiagnostics` API endpoints.
- **NAT** (`internal/awg/platform_{linux,other}.go`): `defaultRouteInterface()` for MASQUERADE target.
- **Inbound needRestart** (`internal/web/service/inbound.go`): `awgRoutesThroughXray` — needRestart on AddInbound/DelInbound/UpdateInbound/SetInboundEnable so Xray regenerates config when routeThroughXray toggles.
- **AWG outbound** (`internal/awg/client_*.go` + `internal/web/{service,controller}/awg_outbound.go`): symmetric sidecar for chaining VPN-of-VPN. Each `awg_outbounds` row = one `awgo-N` kernel interface (client to an upstream AWG server) exposed as a freedom outbound with `sockopt.interface = awgo-N`. Manager: `EnsureClient`/`RemoveClient`/`SweepOrphanClients` (fingerprint-based restart, mirrors inbound Manager). Client .conf via `renderClientConf` (Table=off, no DNS, no I1-I5; HPK only when `AwgVersion == "3"` and non-empty). `ParseConf` eats a .conf of any version (incl. HPK) and auto-detects `AwgVersion` from the field set. Controller uses `RestartXray(true)` on mutations (hot-apply can't add a freedom outbound with sockopt.interface). Address allocation (`client_awg.go`) excludes AWG outbound tunnel IPs to avoid collision.
- **AWG3 / version presets** (`headerProtectionKey` + `awgVersion` fields): upstream `feat/awg3` merged to master on 2026-07-30 (kernel `v3.0.20260731`, tools `v3.0.20260730`), so HPK is now **enabled**. The `awgVersion` field (`"1.5"`/`"2"`/`"3"`) lives on the inbound (server ceiling) and gates HPK emission everywhere — `generateObfuscation` returns it only for `"3"`; `renderServerConf`/`renderClientConf`/`inboundAwgHints` write the `.conf` line only when `AwgVersion == "3"` AND the key is non-empty. Generator guarantees S1–S4 ≥ 12 (`MinSForHPK`). Client export selector (`ClientQrModal`/`ClientInfoModal`) clamps to ≤ ceiling. See Known Issue #5 (CLOSED) and Pattern 6 (version compatibility).
- **AWG3 advanced parameters** (`contentPaddingAddition`, `rekeyAfterTime`, `rekeyTimeout`, `rejectAfterTime`, `keepaliveTimeout`, `maxHandshakeAttempts` + per-peer `advancedSecurity`): 7 device/peer fields from the upstream kernel UAPI that the panel now exposes (lucx.52). All version-gated to `"3"`, all default 0/`false` = kernel uses built-in WireGuard constant (120/5/180/10/18 sec / deterministic WG padding). Device fields written to `[Interface]`, `AdvancedSecurity` to `[Peer]`. `generateObfuscation` does NOT auto-generate them (timers are not obfuscation). Migration prunes non-v3 values. `ParseConf` auto-detects v3 from any of these fields. AdvancedSecurity is advisory only (kernel `set_peer` ignores on input, `get_peer` hardcodes in dumps).

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

LucX-UI components (`internal/awg/`, `internal/awg/cps/`, `internal/awg/signature/`, `internal/lucx/`, `internal/database/migrate_awg*.go`, `internal/web/controller/awg.go`, `internal/web/controller/awg_outbound.go`, `internal/web/job/awg_job.go`, `internal/web/service/client_awg.go`, `internal/web/service/awg_outbound.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `frontend/src/pages/inbounds/form/awg-inbound-id-context.ts`, `frontend/src/pages/clients/wireguardConfig.ts`, `bin/install-awg-module.sh`, `bin/check-lucx.sh`, `bin/pre-push`, `bin/build-release.sh`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use (including VPN resale) requires explicit written permission from the author.

Original 3x-ui code remains under GPL-3.0.

**Every new LucX-owned file MUST carry the SPDX header** (see any existing file in `internal/awg/` for the exact 5-line block). Files with `//go:build` tags put the header after the constraint line; shell scripts after the shebang. The full split (which files are PolyForm vs GPL, why, commercial contact) is documented in [LICENSING.md](LICENSING.md); the canonical license text is [LICENSE-PolyForm-Noncommercial.txt](LICENSE-PolyForm-Noncommercial.txt). Upstream files with LUCX-HOOK blocks stay GPL — never put SPDX headers in them.

---

## Architecture Map

```
internal/awg/                      AWG sidecar — INBOUND (mirrors internal/mtproto/) + OUTBOUND (awgo-N clients)
├── manager.go                     Manager singleton: Ensure/Reconcile/StopAll/CollectTraffic/SyncPeers + renderServerConf/writeServerConfigFile + natPostUpPostDown + ensureXrayRouting + ensureNatRules/natRulesFor + Traffic/PeerTraffic/scrapePeers (one `awg show dump` per iface: counters + handshakes)
├── process.go                     Process wrapping awg-quick up/down + procLogWriter + awgConfigDir + awgQuick
├── instance.go                    Instance + InstanceFromInbound + fingerprint + PeerSpec (server-side desired state for awgN)
├── diagnostics.go                 Diagnose(inst) — read-only probe chain (interface/ip_forward/peers/NAT or TUN rules), prober interface, DiagCheck/Diagnostics
├── platform_linux.go              defaultRouteInterface() + killStrayAwgInterfaces (was nat_linux + orphans_linux)
├── platform_other.go              no-ops off Linux
├── client_instance.go             ClientInstance + ClientSettings + ClientInstanceFromOutbound + fingerprint (desired state for awgo-N outbounds)
├── client_conf.go                 renderClientConf — awg-quick .conf for an awgo-N outbound (Table=off, no DNS, no I1-I5; HPK only when AwgVersion=="3" and non-empty)
├── client_manager.go              outbound client manager: EnsureClient/RemoveClient/SweepOrphanClients (fingerprint-based restart)
├── *_test.go                      instance/manager/diagnostics/client_conf/client_instance/client_manager/platform tests

internal/awg/cps/                  CPS packet generators (TLS/DNS/SIP/QUIC) + AWGParams
├── cps.go                         GenerateCPS + tlsPacket (Chrome/Firefox/Safari) + buildChromeHello/buildFirefoxHello/buildSafariHello + DNS/SIP/QUIC packet builders (quicInitialPacket respects browserProfile)
├── domains.go                     MimicryProfile + BrowserProfile + ObfProfile types + domain pools (RU/World)
├── params.go                      GenerateAWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4) + SetRand for tests + rng
└── cps_test.go                    CPS unit tests (all browsers, invariants, signatures, QUIC browser)

internal/awg/signature/            QUIC host capture (hoaxisr port)
├── capture.go                     Capture(domain) — sends QUIC Initial, reads replies → I1-I5
└── capture_test.go                normalizeDomain/fillPackets/varint/HKDF/ClientHello+Initial structure tests

internal/lucx/                     Smart Cluster
├── parser/                        SSH output → NodeCreds
├── nodetype/                      LucX vs vanilla detection (MTProtoVersion)
└── outbound_link/                 Inbound → outbound config generator

internal/database/
├── migrate_awg.go                 pruneLegacyAwgHiddenChildren + stripHiddenKeys
├── migrate_awg_outbound.go        outbound-side migration (stripHiddenKeys for awg_outbounds)
├── migrate_awg_hpk.go             pruneAwgHeaderProtectionKey — clears non-empty HPK (regression from lucx.47; see Known Issue #5)
└── migrate_awg*_test.go           unit tests

internal/web/
├── runtime/local.go               AWG delegation in AddInbound/DelInbound (LUCX-HOOK)
├── job/awg_job.go                 AwgJob cron — Reconcile + CollectTraffic (inbound + per-client + online) + pubkey→email mapping + outbound client reconcile
├── service/xray.go                injectAwgEgress (TUN inbound + per-inbound gateway + sniffing) + injectAwgOutbounds (freedom per awgo-N with sockopt.interface) + AWG exclusion + ensureAwgRouting (post-restart route restore) (LUCX-HOOK)
├── service/inbound.go             awgRoutesThroughXray + needRestart (LUCX-HOOK) + inboundAwgHints (pre-renders Jc/S/H/I block + HeaderProtectionKey for client .conf)
├── service/client_awg.go          defaultAwgClients — keypair + PSK + address allocation (excludes AWG outbound tunnel IPs)
├── service/awg_outbound.go        AwgOutboundService — CRUD + parseConf + ActiveOutboundTags/ActiveOutboundAddresses (collision guard)
├── controller/awg.go              generateObfuscation + captureHost + awgDiagnostics API endpoints (LUCX-HOOK). HPK is intentionally NOT emitted (Known Issue #5).
├── controller/awg_outbound.go     AWG outbound CRUD + parseConf + test endpoints; RestartXray(true) on add/del/update/enable (hot-apply can't add freedom with sockopt.interface)
├── web.go                         cadenceAwg + StopAll wiring (LUCX-HOOK)

internal/database/model/model.go   AWG Protocol const + validate oneof (LUCX-HOOK)
internal/database/db.go            pruneLegacyAwgHiddenChildren + pruneAwgHeaderProtectionKey calls (LUCX-HOOK)

frontend/src/
├── schemas/protocols/inbound/awg.ts        AwgInboundSettingsSchema (Zod) — includes headerProtectionKey + awgVersion (1.5/2/3 ceiling)
├── pages/inbounds/form/protocols/awg.tsx   AwgFields (React + AntD) + diagnostics modal + HPK field (obfLevel 3)
├── pages/inbounds/form/awg-inbound-id-context.ts  editing inbound id provider for diagnostics (LUCX)
├── pages/inbounds/form/InboundFormModal.tsx       AwgInboundIdProvider wrap (LUCX-HOOK)
├── lib/xray/inbound-defaults.ts            createDefaultAwgInboundSettings (LUCX-HOOK) — HPK defaults to ''
├── lib/xray/inbound-link.ts                genAwgLink/genAwgConfig (share-link + .conf, I1-I5 as-is)
├── pages/clients/wireguardConfig.ts        buildAwgClientConfig (full client .conf, reads pre-rendered awgObfuscation block)
├── pages/clients/ClientQrModal.tsx         AWG panel with QR + download
├── schemas/protocols/inbound/index.ts      InboundSettingsSchema union (LUCX-HOOK)
├── schemas/primitives/protocol.ts          ProtocolSchema + Protocols map (LUCX-HOOK)
└── pages/inbounds/form/protocols/index.ts  AwgFields export (LUCX-HOOK)

bin/install-awg-module.sh          DKMS build of amneziawg kernel module + tools (HEAD of upstream master → pulls AWG3 v3.0.20260731 since lucx.50; needs kernel ≥ 6.7 for v3 build)
bin/check-lucx.sh                  gofumpt check for LucX files (49) — run before push; -w autofixes
bin/pre-push                       git hook: check-lucx + fast go tests + PR/issues guard (AGENTS.md 11.5)
install.sh                         Calls bin/install-awg-module.sh (LUCX-HOOK)
LICENSING.md                       GPL-3.0 / PolyForm-NC split documentation
LICENSE-PolyForm-Noncommercial.txt Canonical PolyForm NC 1.0.0 text
```

---

## Test Commands

```bash
# Go unit tests (no cgo required)
go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v

# Frontend
cd frontend && npm run typecheck && npm run lint

# Full project build (requires frontend/dist)
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .

# Pre-push hygiene (gofumpt on all LucX files — catches Windows/Linux drift before CI)
bin/check-lucx.sh          # check;  bin/check-lucx.sh -w  # autofix

# CRITICAL before push (CI catches these, check-lucx.sh does NOT):
#   1. gofumpt on the WHOLE repo (CI's golangci-lint runs on ./..., not just
#      the 49 LucX files). check-lucx.sh only covers LucX-owned files, so a
#      pre-existing formatting drift in an upstream file you touched (e.g. a
#      case-block indentation) will pass locally and fail CI.
gofumpt -l .               # list;  gofumpt -w <file>  # fix one
#   2. Regenerate OpenAPI artifacts after editing any Go struct with json:/example:
#      tags that flows into an API response. CI's `codegen` job fails on stale
#      generated files. AGENTS.md Rule 9 "Do not edit src/generated/" means no
#      HAND edits — regenerating via the tool is required and expected.
cd frontend && npm run gen  # gen:zod (go run ./tools/openapigen) + gen:api

# Optional: install the git hook that runs check-lucx + fast tests + PR/issues guard (step 11.5)
cp bin/pre-push .git/hooks/pre-push && chmod +x .git/hooks/pre-push
```

---

## Deploy

- **Target:** `lucx` (SSH alias in `~/.ssh/config`, GCP Finland) — ⚠️ с 2026-07-18 недоступен (VM остановлена или ephemeral IP сменился, порты фильтруются; нужна консоль GCP)
- **Service:** `x-ui.service` (systemd)
- **Procedure:** SCP binary (или tarball релиза на самом сервере) → `sudo systemctl restart x-ui` → verify `systemctl status x-ui` + logs
- **AWG runtime check:** `awg show` should list active interfaces; `ip link show awgN` for TUN

### Тестовые серверы (SSH alias'ы в `~/.ssh/config`, user `root`, ключ `~/.ssh/id_ed25519`)

| Alias | IP | Хост | Назначение |
|---|---|---|---|
| `lucx-test2` | 144.31.157.106 | poor-rose-snake.play2go.cloud | **Наш единственный тестовый сервер** — install-тесты, AWG runtime, проверка релизов |

- **test1 (144.31.224.212)** — с 2026-07-19 **НЕ НАШ**: отдан под тестирование другого продукта. Не трогать, не деплоить, не логиниться без запроса.
- **Testers:** VladufQa, Kirill Rudenko — обновляются сами через `x-ui update` или reinstall; на их панели без запроса не лезем.

---

## Release & Install (форк)

`install.sh` адаптирован под наш форк (`AlexeyLCP/lucx-ui`): скачивает релиз-tarball и raw-скрипты (x-ui.sh, x-ui.rc, service-юниты) из `main`. Xray-core + mtg переиспользуются из апстрим-релиза `MHSanaei/3x-ui`.

### Сборка релиза (на VPS, Linux/amd64, с gcc + go + node)

CGO-бинарник (mattn/go-sqlite3) нельзя cross-compile с Windows — сборка только на Linux.

```bash
# 1. Собрать tarball
curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/bin/build-release.sh | bash
# → /tmp/x-ui-linux-amd64.tar.gz

# 2. Создать GitHub-релиз (нужен gh CLI с auth). ВЕРСИЯ = база апстрима + lucx.N
gh release create v3.6.0-lucx.50 /tmp/x-ui-linux-amd64.tar.gz \
  --repo AlexeyLCP/lucx-ui \
  --title "v3.6.0-lucx.50" \
  --notes "LucX-UI v3.6.0 с AWG-сайдкаром (см. progress.md)"

# 3. Установить панель (на этом или другом VPS)
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
# → скачает наш релиз, поставит x-ui + systemd + Xray + mtg + fail2ban + AWG-модуль
```

> Тег ставится **только после зелёного CI на main** (урок lucx.48: первый тег уехал до CI-фиксов, пришлось удалять релиз и переставлять тег). `lucxVersion` в `internal/config/config.go` должен совпадать с суффиксом тега — CI guard ловит расхождение.

### Зависимости VPS для сборки
- Go 1.23+ (рекомендуется 1.26)
- Node.js 20+ и npm
- gcc (для CGO)
- git, curl, tar

### Структура релиза (как у апстрима)
```
x-ui-linux-amd64.tar.gz → x-ui/
  ├── x-ui                    ← наш бинарник (CGO, собран из форка)
  ├── x-ui.sh, x-ui.rc        ← из репо
  ├── x-ui.service.{debian,arch,rhel}  ← из репо
  └── bin/
      ├── xray-linux-amd64    ← из апстрим-релиза (не наш код)
      ├── mtg-linux-amd64     ← из апстрим-релиза (не наш код)
      └── install-awg-module.sh  ← наш DKMS-скрипт
```

---

## Branch Protection (gh/main)

`main` на `AlexeyLCP/lucx-ui` защищён (Settings → Branches): **force-push и удаление ветки запрещены для всех, включая админа** (`enforce_admins: true`, `allow_force_pushes: false`, `allow_deletions: false`). PR и status checks НЕ требуются — прямые пуши работают как раньше. Если когда-либо понадобится force-push (например, squash истории) — сначала осознанно ослабить правило в Settings → Branches, выполнить, вернуть обратно. Двухшаговость — by design.

---

## Commit Convention

- Префиксы: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`
- Область: `feat(awg): ...`, `fix(frontend): ...`, `chore(codegen): ...`
- Сообщения коммитов — на русском (если не запрошено иное)
- Пример: `feat(awg): порт изолированных пакетов на v3.5.0`

---

## Known Issues

### 1. ~~AWG sidecar раздут относительно mtproto (эталона)~~ — ЗАКРЫТО

**Решено (2026-07-13):** рефактор удалением мёртвого кода. Файлы `params.go`, `cps.go`, `config.go`, `templates.go`, `types.go`, `helpers.go` + 5 тестов были полностью мёртвым кодом — их функции (`GenerateAWGParams`, `GenerateCPS`, `BuildServerConfig`, `RenderPostUp` и др.) вызывались только тестами, ни один живой call site их не использовал. Генерация ключей/обфускации делается во frontend (`createDefaultAwgInboundSettings`). AWG сокращён с 19 до 8 файлов (6 .go + 2 теста) — почти симметрично mtproto (9 файлов). Обновления upstream теперь требуют переноса ~20 файлов вместо 29.

**Дожато (2026-07-18):** финальный slimming до точной паритетности. `traffic.go` влит в `manager.go` (Traffic + scrapeTransfer, позже → scrapePeers, живут только ради CollectTraffic); `nat_{linux,other}.go` + `orphans_{linux,other}.go` слиты в одну платформенную пару `platform_{linux,other}.go`; заодно вычищены var-гварды неиспользуемых импортов (`strconv`/`syscall`) — мусор от удалённого tun2socks. Итог core-пакета: **6 source + 3 test = 9 файлов**, ровно как mtproto (4 source + 2 platform + 3 test). `cps/` и `signature/` остаются отдельными пакетами — это фичи, которых у mtproto нет.

### 2. ~~Сайдкар не проверен в реальном runtime на VPS~~ — ЗАКРЫТО

**Решено (2026-07-16):** сайдкар проверен в реальном runtime на VPS тестеров (VladufQa на ruvds-rdu8b, Kirill Rudenko на runode). Kernel routing (без routeThroughXray) работает — handshake, ICMP, HTTPS, traffic. routeThroughXray работает после PR #13 (needRestart + policy routing + sniffing). Релизы v3.5.0-lucx.20–31 протестированы тестерами.

### 3. Dependabot — только security updates

Version updates (еженедельные PR на новые версии) отключены — `updates: []` в `.github/dependabot.yml`. Это убирает шум минорных обновлений npm/gomod/github-actions, которые накапливались как незакрытые PR (10 шт. были закрыты перед миграцией на v3.5.0). Security updates (CVE) остаются включёнными через GitHub Settings → Dependabot security updates — Dependabot автоматически создаст PR при найденной уязвимости в любой зависимости. Чтобы вернуть version updates — замените `updates: []` на полный список (шаблон в комментарии в yml-файле).

**⚠️ Готcha (2026-07-18):** `updates: []` НЕ останавливает **групповые** version-update PR, если они включены в GitHub UI (Settings → Advanced Security → Dependabot grouped version updates) — это отдельный тумблер, не читающий наш yml. 18.07 появилось 10 PR (grpc, antd, vite, storybook, actions/*) — закрыты, к каждому коммент «version updates отключены». Dependabot на закрытый PR отвечает «won't notify you again about this release, но напишет при новой версии» — то есть периодически возвращается. **Полное отключение:** Settings → Advanced Security → выключить Dependabot version updates (+ grouped). Каждый новый version-update PR — закрывать с тем же комментом; **проверяй очередь перед каждым пушем (шаг 11.5)**.

### 4. routeThroughXray — сложнее чем mtproto

AWG routeThroughXray **принципиально сложнее** mtproto из-за kernel→userspace моста:

| | mtproto | AWG |
|---|---|---|
| Тип sidecar | userspace daemon (mtg) | kernel module (awg-quick) |
| Тип трафика | TCP (FakeTLS → MTProto) | IP-пакеты (kernel) |
| Мост в Xray | SOCKS5 loopback (TCP) | TUN inbound (IP) |
| Как трафик попадает в Xray | mtg сам dial 127.0.0.1:port | policy routing: `ip rule iif awgN lookup 1000+N` → `default dev tunN` |
| NAT | не нужен (mtg → SOCKS → Xray) | не нужен (Xray → outbound сам натит) |
| needRestart | `mtprotoRoutesThroughXray` в AddInbound/DelInbound/UpdateInbound | `awgRoutesThroughXray` — те же точки (добавлено в PR #13) |
| Route maintenance | не нужен (SOCKS порт постоянный) | `ensureXrayRouting` в reconcile-цикле (10с) — tunN пересоздаётся при каждом рестарте Xray. В kernel-режиме — `ensureNatRules` (тот же цикл): MASQUERADE/FORWARD умирают при iptables flush |
| Sniffing | SOCKS inbound сам делает | TUN inbound — нужен явный `sniffing: {routeOnly:true}` (без него domain rules не работают) |

Not to re-add: tun2socks (заменено TUN inbound), DNS в серверный .conf (ломает системный DNS), фиксированные table 100 + gateway 10.254.254.1/30 (ломают мульти-инбаунд).

**Post-restart window (ЗАКРЫТО 2026-07-19):** рестарт Xray (кнопка в панели) убивал tunN и маршрут `default dev tunN table 1000+N` до следующего тика AWG reconcile-cron (до 10 с routed-клиенты без интернета; «повторный выбор outbound» просто триггерил reconcile раньше cron'а). Фикс: `ensureAwgRouting()` в `RestartXray` сразу после `p.Start()` — маршрут восстанавливается синхронно с появлением нового tunN. Проверено на v3.6.0-lucx.48/test2: после `systemctl restart x-ui` на t+8s маршрут на месте, ping 0% loss.

### 5. ~~AWG3 (AmneziaWG 3) — forward-compat поле `headerProtectionKey`~~ — ЗАКРЫТО (lucx.50)

**Решено (2026-07-31):** AWG3 официально слит в upstream и включён в LucX-UI с lucx.50.
- `amnezia-vpn/amneziawg-linux-kernel-module`: PR #192 слит в master 30.07.2026, тег **`v3.0.20260731`**. `WGDEVICE_A_HEADER_PROTECTION_KEY` в master. ⚠️ ядро отвергает HPK с `-EINVAL`, если любое из S1–S4 < 12.
- `amnezia-vpn/amneziawg-tools`: PR #60 слит 30.07.2026, тег **`v3.0.20260730`**. `HeaderProtectionKey` парсится в `.conf` (`config.c`, `parse_key`).
- ⚠️ Сборка модуля v3.0 падает на ядрах < 6.7 (`nla_put_uint`) — фикс уже в master, но на старых VPS может потребоваться обновление ядра.

**Что сделано в lucx.50:**
1. `generateObfuscation` (`controller/awg.go`) снова отдаёт `headerProtectionKey` — но **только при `awgVersion == "3"`** в запросе. Для v1.5/v2 поле отсутствует в ответе (не `""`), чтобы `regenerateObfuscation` (`Object.entries(obf).forEach(setValue)`) не затёр ручное значение оператора.
2. Рендереры `renderServerConf` (`manager.go`), `renderClientConf` (`client_conf.go`), `inboundAwgHints` (`inbound.go`) пишут HPK **только при `awgVersion == "3"` И непустом ключе**. Для v1/v2 строка опускается — старые ядра продолжают работать.
3. Генератор `GenerateAWGParams` (`cps/params.go`) теперь **гарантирует S1–S4 ≥ 12** (`MinSForHPK = 12`, `enforceSMin`) для всех профилей — конфиг валиден для AWG3 независимо от того, установлен ли HPK. `GenerateHeaderProtectionKey()` + `AWGParams.WithHeaderProtectionKey()` генерируют ключ (crypto/rand, 32 байта, base64).
4. Новое поле `awgVersion` (`"1.5"`/`"2"`/`"3"`) во всём пайплайне — на инбаунде (потолок сервера) и в клиентском экспорте (≤ потолка, runtime-селектор в `ClientQrModal`/`ClientInfoModal`).
5. Миграция переименована: `pruneAwgHeaderProtectionKey` → `migrateAwgVersion` (`migrate_awg_hpk.go`). Теперь backfill'ит `awgVersion:"2"` на pre-lucx.50 инбаундах/аутбаундах И вычищает непустой HPK с всего, что не v3 (фикс регрессии lucx.47 для пострадавших + защита от будущего bump'а версии).

**Урок (сохраняется):** «Regenerate obfuscation» молча пишет в форму всё, что вернул backend. Любое поле без поддержки в текущем ядре → краш reconcile. Решение — version-gate эмиссию, а не полное умолчание: поле отдаётся/пишется только когда версия явно его поддерживает.

---

## Frontend Conventions

- Ant Design 6 only — no Tailwind/shadcn.
- TS strict; `@typescript-eslint/no-explicit-any` is an error. Zod schemas in `src/schemas/` are the source of truth; infer types with `z.infer`, never hand-write. Do not edit `src/generated/`.
- Editing `frontend/src` does NOT change what users see until the Vite build is regenerated into `internal/web/dist/`.
- After touching share-link logic (`src/lib/xray/`), run `npm run test` (golden fixtures).

---

## Go Conventions

- Stdlib `testing` only (no testify). Table-driven, `t.Run` subtests.
- NO `//` line comments in committed Go/TS (except directives like `//go:build`). Names carry meaning. (Inherited from upstream CLAUDE.md — applies to upstream code; LucX HOOK blocks may carry the `// LUCX-HOOK:` marker comment by design.)
- `golangci-lint run` / `make lint` for formatting (gofumpt + goimports).
- Conventional-commit prefixes, Russian commit messages.

---

## Debugging Patterns

### Pattern 1: AWG inbound не стартует
- **Cause:** `awg-quick` не установлен или kernel module не загружен.
- **Fix:** `bin/install-awg-module.sh` на сервере. Проверить `awg show`, `ip link show awgN`.

### Pattern 1b: AWG inbound не стартует после апгрейда — "iptables: command not found"
- **Cause:** Debian 13+ не ставит iptables из коробки (только nftables). PostUp с MASQUERADE/FORWARD (появился в lucx.20) падает с exit 127 → awg-quick откатывается → awgN не поднимается вообще. Бьёт по инсталляциям, обновлённым с версий < lucx.20 (iptables не требовался раньше). В логах: `awg-quick: line 295: iptables: command not found` + `reconcile failed ... exit status 127`.
- **Fix:** `apt-get install -y iptables` (ставит shim над nf_tables — наши правила работают прозрачно). Reconcile поднимет интерфейс сам за ≤10 с. Свежие установки покрыты: `bin/install-awg-module.sh` ставит iptables как зависимость (с 2026-07-18).
- **Наблюдали:** lucx-test1 (144.31.224.212) при апгрейде lucx.17 → lucx.33, 2026-07-18.

### Pattern 1c: AWG outbounds «не подключаются» после апгрейда панели (рассинхон модуля) — ИСПРАВЛЕНО (lucx.51)
- **Cause:** Мажорный апгрейд upstream amneziawg-модуля (как AWG1 → AWG3 в lucx.50, `v3.0.20260731`) требует **пересборки DKMS-модуля**. `bin/install-awg-module.sh` делает это, но `update.sh` (общий путь для веб-панели и консоли `x-ui update`) НЕ вызывал его — модуль оставался старым, новый бинарник стартовал с новыми настройками → handshake не проходил. Симптом: «подключение есть, но не подключается», откат на прошлый релиз лечит.
- **Fix (lucx.51):** `update.sh` теперь содержит LUCX-HOOK с версионным gate: сравнивает маркерный файл `/etc/x-ui/.awg-module-version` (пишется `install-awg-module.sh` при каждой успешной сборке) с версией из свежего `git clone --depth 1` upstream dkms.conf; при расхождении вызывает `install-awg-module.sh --force-rebuild` (rmmod старого + dkms remove + полная пересборка). Gate стоит **до** `systemctl start x-ui`, так что модуль пересобран на остановленной панели (rmmod безопасен). При падении git clone (нет сети) — fallback: вызов без --force-rebuild (покрывает «модуля вообще нет»). Non-fatal: ошибка пересборки не блокирует старт панели.
- **Наблюдали:** тестер Александр при апгрейде до lucx.50 через веб-панель — AWG 1.5 outbounds не подключались; повтор через консольное меню `x-ui` всё починил (2026-08-01). С lucx.51 веб-обновление должно пересобрать модуль автоматически.

### Pattern 1d: AWG inbound «Device awgN does not exist» когда оператор выбрал v3 на v1.x module — ИСПРАВЛЕНО (lucx.53)
- **Cause:** Migration-prune (migrateAwgVersion) гейтит AWG3 поля (HPK + 6 device timers/padding + AdvancedSecurity) только по `awgVersion != "3"`. Если оператор выбрал v3 в форме, но host ещё на amneziawg kernel module v1.x, поля уходят в .conf → v1 module reject'ит «Line unrecognized: ContentPaddingAddition=64» в setconf → awg-quick откатывает интерфейс → «Device awgN does not exist». Симптом:AWG inbound не поднимается, в логах `awg setconf ... Configuration parsing error`. Воспроизведено на lucx-test2 (module v1.0.20260611).
- **Fix (lucx.53):** `ModuleSupportsAwg3()` (platform_{linux,other}.go) — кэшированный probe `modinfo -F version amneziawg`, возвращает true только для v3.x. Все 4 рендерера (renderServerConf, renderClientConf, inboundAwgHints + transitively sub/service.go через Prune AWG3 fields) теперь double-gated: `AwgVersion == "3" && ModuleSupportsAwg3()`. Тесты override через `SetModuleSupportsAwg3(&bool)`.
- **Урок:** DB-stored `awgVersion` — это потолок, который оператор выбрал, а не capabilities runtime. Нужна explicit module capability check в каждой точке эмиссии AWG3 полей.

### Pattern 2: LUCX-HOOK конфликт при upstream sync
- **Cause:** Upstream изменил файл с HOOK-маркером между релизами.
- **Fix:** Решать каждый блок отдельно (см. Rule 8). Не `git checkout` весь файл и не сплошной `--ours` — потеряешь upstream-изменения.

### Pattern 2b: после merge файл потерял LUCX-HOOK-блоки целиком
- **Cause:** файл в состоянии конфликта правили через IDE — она перезаписывает его из своего merge-кэша и молча выкидывает часть содержимого. На v3.6.0: `install.sh` потерял все 16 блоков, `db.go` — апстрим-функции.
- **Detect:** `git grep -c "LUCX-HOOK"` до и после — падение числа есть потеря. Проверять также число строк: если результат меньше И нашего, И апстримового варианта — выпал код.
- **Fix:** `git checkout --merge -- <файлы>` восстанавливает конфликтное состояние с маркерами; дальше резолвить только из терминала, затем `git add` для снятия unmerged-записей (иначе IDE продолжает предлагать свой мастер и может затереть работу).

### Pattern 2c: CI красный на i18n-dead-keys после добавления LucX-фичи
- **Cause:** `frontend/src/test/i18n-dead-keys.test.ts` (из v3.6.0) требует, чтобы **каждая** из 13 локалей несла ровно тот же набор ключей, что `en-US.json` (и наоборот — ни одного лишнего). Добавил ключ только в en+ru — 11 локалей падают с `missing=N`.
- **Fix:** добавлять новые ключи сразу во все 13 файлов `internal/web/translation/*.json`, с переводом на язык локали (конвенция проекта); технические термины (`Tag`, `MTU`, `DNS`, `Allowed IPs`, `awg-quick`, `outbound`) оставлять латиницей. Второй тест набора требует обратного: неиспользуемый в коде ключ — тоже падение, удаляй его из всех 13.

### Pattern 3: Frontend не видит AWG-протокол
- **Cause:** Забыта регистрация в одном из: `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`.
- **Fix:** `grep -rn "awg\|Awg\|AWG" frontend/src/` — проверить все 5 точек регистрации.

### Pattern 4: routeThroughXray — нет интернета
- **Cause 1:** needRestart не сработал → Xray не перегенерировал конфиг → TUN не создан.
  **Fix:** Проверить `awgRoutesThroughXray` в `inbound.go` (AddInbound/DelInbound/UpdateInbound/SetInboundEnable).
- **Cause 2:** `ip rule iif awgN lookup 1000+N` отсутствует или маршрут в table 1000+N потерян (tunN пересоздан).
  **Fix:** `ip rule show | grep awg`, `ip route show table 1000+N`. Reconcile-цикл (10с) должен восстановить.
- **Cause 3:** TUN gateway конфликтует с AWG subnet.
  **Fix:** gateway должен быть `10.254.(N%254).1/30` (per-inbound /30, не AWG subnet).
- **Cause 4:** Domain rules не работают (SNI не виден).
  **Fix:** TUN inbound должен иметь `sniffing: {routeOnly:true}`. Проверить `awgEgressTunSniffing` в `xray.go`.

### Pattern 5: Xray падает "this rule has no effective fields"
- **Cause:** Routing rule без `outboundTag`/`balancerTag`/`domain`/`ip` — только `type` и `inboundTag`.
  **Fix:** Проверить routing template config в панели. `injectAwgEgress` не создаёт rule при пустом `outboundTag` (котел Xray). Если rule приходит из template — убрать пустой rule.

### Pattern 6: AWG клиент не подключается — версия сервера vs версия клиента (lucx.50+)
- **Cause:** Реальная граница совместимости — это **серверный** конфиг, а не длина клиентского. Поля AmneziaWG делятся на:
  - **must-match** (сервер↔клиент обязаны совпадать): `S1`–`S4`, `H1`–`H4`, `HeaderProtectionKey`. Если сервер v3 (с HPK), а клиент v2 — handshake падает на must-match полях.
  - **may-differ**: `Jc`/`Jmin`/`Jmax`, `I1`–`I5`.
  - **version-gated**: `S3`/`S4` + `I1`–`I5` появились в AWG v2 (Android 2.0.1); `HeaderProtectionKey` — в AWG v3 (desktop 5.0.0.5 / Android 3.0.1).
- **Fix:**
  - Селектор `awgVersion` на инбаунде задаёт **потолок сервера** — какие поля сервер примет. Сервер v3 НЕ примет v1/v2/plain-WG клиентов (HPK криптографически ломает совместимость).
  - Для смешанного парка клиентов (часть старая, часть новые) — **создавай отдельный инбаунд v2** (без HPK). v2-сервер принимает и v2, и v1.5 клиентов.
  - В модалке клиента (`ClientQrModal`/`ClientInfoModal`) селектор «Client config version» позволяет экспортировать конфиг ≤ потолка инбаунда. Это только **избегает ошибок парсинга** в старом клиентском приложении (лишние поля отрезаются), но НЕ даёт совместимости, если клиент старше сервера.
  - ⚠️ HPK требует S1–S4 ≥ 12 (генератор гарантирует; при ручном вводе проверяй — форма показывает `awgSRangeWarning`).
- **Симптом:** клиент висит на handshake, в логах сервера `awg0` peer без рукопожатия. Сравни must-match поля в серверном `.conf` (`/etc/awg/awgN.conf`) и клиентском — любое расхождение = причина.

### Pattern 7: CI go-test красный на `TestBuildFirefoxHello_NoGrease`/`TestBuildSafariHello_NoGrease` (pre-existing flaky)
- **Cause:** CI гоняет `go test -shuffle=on` (рандомный seed каждый прогон). Тесты `NoGrease` утверждают, что Safari/Firefox ClientHello **не содержит** GREASE-паттернов `0a0a`/`fafa` в hex. Но `buildSafariHello`/`buildFirefoxHello` (`cps.go`) **пишут** GREASE через `greaseValue()` — `rng.Intn` над 16 значениями `[0x0A0A … 0xFAFA]`. Тест проходит в 14/16 случаев (когда rng не выдаёт `0x0A0A`/`0xFAFA`), падает в ~1/10 shuffle-прогонов. Воспроизводится локально: `go test ./internal/awg/cps/... -count=20 -shuffle=on`. **НЕ связано с AWG-изменениями** — чужой баг в логике обфускации.
- **Fix (временный):** `gh run rerun <id> --failed` — другой shuffle-seed с высокой вероятностью проходит.
- **Fix (корневой, TODO отдельным issue):** либо тест должен проверять GREASE только в extension-позициях (а не по всему hex), либо `buildSafariHello`/`buildFirefoxHello` не должны писать GREASE через rng (Safari/Firefox по реальным fingerprint'ам GREASE не используют — только Chrome). Пробовал `SetRand(t *testing.T, …)` с `t.Cleanup` для изоляции глобального `rng` — не помог (проблема в логике, не в загрязнении); откатил. НЕ чинить наспех — это domain-логика CPS-обфускации.
- **Урок:** при падении CI-джоба сначала проверь, твой ли это регресс. Воспроизведи локально с точным shuffle-seed из лога CI (`-test.shuffle <N>`). Если проходит локально и не связано с твоими файлами — это flaky, rerun оправдан.