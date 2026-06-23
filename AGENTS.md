# LucX-UI — Agent Operating Manual

This file is the law for every agent working on this project. Read it completely before touching any code.

---

## Project Overview

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) v3.3.1 that adds native AmneziaWG (AWG) support as a kernel-interface sidecar, mirroring upstream's MTProto (mtg) sidecar architecture. LucX-specific code lives in `internal/awg/` and `internal/lucx/`; all integration points in upstream files are wrapped in `LUCX-HOOK` / `END LUCX-HOOK` markers.

**Upstream sync strategy:** migrate (not rebase). The branch `feat/awg-sidecar` is a fresh checkout of `origin/main` (v3.3.1) with LucX code ported on top. The old `feature/awg-integration` branch is kept as a reference but is not the active line of development.

---

## Workflow: How an Agent Executes a Task

```
1. READ    → Read AGENTS.md, git log --oneline -15, check latest state
2. AUDIT   → Read all relevant files, trace data flow end-to-end
3. PLAN    → Write a short plan: which files, what changes, what tests
4. BRANCH  → Work on `feat/awg-sidecar` (current active branch)
5. CODE    → Implement changes inside LUCX-HOOK blocks in upstream files;
             new code goes in internal/awg/ or internal/lucx/
6. TEST    → Run tests:
               go test ./internal/awg/... ./internal/lucx/... -count=1 -v
               cd frontend && npm run typecheck && npm run lint
7. BUILD   → Frontend: cd frontend && npm run build
             Backend:  go build -o /tmp/x-ui .
                       (requires frontend/dist to exist for //go:embed)
8. DEPLOY  → SCP to vps-finland-lucx, restart x-ui.service
9. VERIFY  → Check `sudo systemctl status x-ui`, check server logs
10. COMMIT → `git add` specific files, `git commit` with descriptive message
11. STATUS → Output `git status` and `git log --oneline -15` after commits
12. DOCS   → Update this file if architecture changes
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
- **Go:** `internal/awg/` — AWG sidecar (manager, process, instance, traffic, orphans, obfuscation: params/cps/config/types/templates/helpers)
- **Go:** `internal/lucx/` — subdirectories: `parser/`, `nodetype/`, `outbound_link/` (Smart Cluster)
- **Go:** `internal/database/migrate_awg.go` — legacy DB migration
- **Frontend:** `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema
- **Frontend:** `frontend/src/pages/inbounds/form/protocols/awg.tsx` — React form
- **Shell:** `bin/install-awg-module.sh` — DKMS install

Integration points (`model.go`, `web.go`, `runtime/local.go`, `service/xray.go`, `install.sh`, `inbound-defaults.ts`, `InboundFormModal.tsx`, `protocols/index.ts`, `primitives/protocol.ts`) get LUCX-HOOK blocks only.

### 3. AWG Sidecar Architecture (mirrors mtproto)

AWG runs as a kernel-interface sidecar managed by `internal/awg.Manager`, exactly symmetric with `internal/mtproto.Manager`:

```
mtproto:  mtg sidecar (userspace)  → TCP → SOCKS loopback inbound → Xray routing
AWG:      awg kernel module        → IP   → TUN inbound             → Xray routing
```

- **Manager** (`internal/awg/manager.go`): singleton with `Ensure`/`Reconcile`/`StopAll`/`CollectTraffic`/`SyncPeers`, fingerprint-based restart on config change, orphan sweep at first call.
- **Process** (`internal/awg/process.go`): wraps `awg-quick up/down` (kernel interface lifecycle, not a daemon). No tun2socks — routing is via Xray TUN inbound.
- **Instance** (`internal/awg/instance.go`): desired runtime state + `InstanceFromInbound` + `fingerprint`.
- **Traffic** (`internal/awg/traffic.go`): `awg show <iface> transfer` parsing for per-peer byte accounting (replaces mtg's Prometheus HTTP scrape).
- **Orphans** (`internal/awg/orphans_{linux,other}.go`): sweep orphaned awg interfaces from a previous x-ui run.
- **Job** (`internal/web/job/awg_job.go`): cron `@every 10s` — Reconcile desired inbounds + fold traffic deltas.
- **Egress** (`internal/web/service/xray.go:injectAwgEgress`): inject TUN inbound into generated Xray config when `routeThroughXray` is set, symmetric with `injectMtprotoEgress`.
- **Runtime** (`internal/web/runtime/local.go`): delegate AWG `AddInbound`/`DelInbound` to `awg.GetManager()`; `AddUser`/`RemoveUser` are no-ops (peer sync via Reconcile).

### 4. Paranoid Logging

Every critical operation logs with a prefix:
```
[LUCX-AWG]            — AWG service operations (legacy logAWG helper)
awg: <label> | <line> — sidecar process output (procLogWriter, matches mtproto)
```

### 5. No Telemt

The old LucX-UI had a `internal/lucx/telemt/` package for MTProto. Upstream v3.3.1 replaced it with native `internal/mtproto/`. Do not re-add Telemt code; use the upstream MTProto implementation.

### 6. No tun2socks

The old architecture used a `tun2socks` userspace daemon to bridge the AWG kernel TUN to a hidden SOCKS5 inbound. The sidecar architecture makes it redundant — Xray v3.3.1 supports a native TUN inbound (`injectAwgEgress`). Do not re-add tun2socks.

### 7. Test Discipline

- **Go:** `go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v`
- **Frontend:** `cd frontend && npm run typecheck && npm run lint`
- DB-dependent service tests require `CGO_ENABLED=1` (sqlite). Unit tests for AWG logic (instance, manager state, inject, stripHiddenKeys) run without cgo.
- Add tests for every new AWG function: instance parsing, fingerprint stability, config rendering, inject behavior, migration logic.

### 8. Upstream Sync

When pulling from upstream (`git fetch origin`):
- Re-run `go build ./internal/awg/... ./internal/lucx/...` — these packages have no upstream dependencies and should always compile.
- Check `grep -rn "LUCX-HOOK"` integration points for conflicts.
- Run `go test ./internal/awg/... ./internal/lucx/...` and frontend `typecheck`/`lint`.
- The `.patch`-file system from the old branch is gone; integration is inline via LUCX-HOOK markers in the new structure.

### 9. Frontend Stack

Upstream v3.3.1 rewrote the frontend from Vue to React + TypeScript + AntD v6 + Zod. AWG follows the same pattern:
- `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema (`AwgInboundSettingsSchema`)
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` — AntD form (`AwgFields`)
- `frontend/src/lib/xray/inbound-defaults.ts` — `createDefaultAwgInboundSettings`
- Registered in `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`

### 10. License

LucX-UI components (`internal/awg/`, `internal/lucx/`, `internal/database/migrate_awg.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `bin/install-awg-module.sh`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use (including VPN resale) requires explicit written permission from the author.

Original 3x-ui code remains under GPL-3.0.

---

## Architecture Map

```
internal/awg/                      AWG sidecar (mirrors internal/mtproto/)
├── manager.go                     Manager singleton: Ensure/Reconcile/StopAll/CollectTraffic/SyncPeers
├── process.go                     Process wrapping awg-quick up/down + procLogWriter
├── instance.go                    Instance + InstanceFromInbound + fingerprint
├── traffic.go                     scrapeTransfer via `awg show transfer`
├── orphans_linux.go               killStrayAwgInterfaces
├── orphans_other.go               no-op off Linux
├── params.go                      GenerateAWGParams (obfuscation + Curve25519 keys)
├── cps.go                         GenerateCPS (I1-I5 connection proxy signatures)
├── config.go                      BuildServerConfig / BuildClientConfig
├── types.go                       AWGConfig / AWGClient / PeerSpec / etc.
├── templates.go                   PostUp/PostDown template rendering
├── helpers.go                     settings parsing + awgQuick helper
├── instance_test.go               Instance/fingerprint/render tests
└── manager_test.go                Manager state-machine tests

internal/lucx/                     Smart Cluster (unchanged from old branch)
├── parser/                        SSH output → NodeCreds
├── nodetype/                      LucX vs vanilla detection (MTProtoVersion)
└── outbound_link/                 Inbound → outbound config generator

internal/database/
├── migrate_awg.go                 pruneLegacyAwgHiddenChildren + stripHiddenKeys
└── migrate_awg_test.go            stripHiddenKeys unit tests

internal/web/
├── runtime/local.go               AWG delegation in AddInbound/DelInbound (LUCX-HOOK)
├── job/awg_job.go                 AwgJob cron — Reconcile + CollectTraffic
├── service/xray.go                injectAwgEgress + AWG exclusion from Xray config (LUCX-HOOK)
└── web.go                         cadenceAwg + StopAll wiring (LUCX-HOOK)

internal/database/model/model.go   AWG Protocol const + validate oneof (LUCX-HOOK)

frontend/src/
├── schemas/protocols/inbound/awg.ts        AwgInboundSettingsSchema (Zod)
├── pages/inbounds/form/protocols/awg.tsx   AwgFields (React + AntD)
├── lib/xray/inbound-defaults.ts            createDefaultAwgInboundSettings (LUCX-HOOK)
├── schemas/protocols/inbound/index.ts      InboundSettingsSchema union (LUCX-HOOK)
├── schemas/primitives/protocol.ts          ProtocolSchema + Protocols map (LUCX-HOOK)
└── pages/inbounds/form/protocols/index.ts  AwgFields export (LUCX-HOOK)

bin/install-awg-module.sh          DKMS build of amneziawg kernel module + tools
install.sh                         Calls bin/install-awg-module.sh (LUCX-HOOK)
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
```