# LucX code review — 2026-09-21

Read-only review of `internal/awg/`, `internal/lucx/`, `LUCX-HOOK` blocks in
upstream files, `bin/` scripts, and the frontend LucX code. Findings below were
fixed in the follow-up commit; notes on what was NOT changed are at the end.

## Fixed

### 1. Missing SPDX license headers — FIXED (42 files)

Every other LucX file carries the PolyForm Noncommercial block; these did not:

- `internal/web/service/inbound_lucx.go`
- `internal/web/service/inbound_sublink.go`
- `internal/web/service/port_conflict.go`
- `internal/web/service/node_contract.go`
- `bin/build-release.sh`
- `frontend/src/pages/inbounds/form/protocols/csqtt.tsx`
- `internal/awg/server_conf_psk_test.go`
- plus 35 more LucX frontend files found by the same sweep (amneziawg/qwdtt/
  olcrtc/tunnel/csqtt schemas + forms, awg tests, `amneziawgConfig.ts`,
  `AmneziaWGLogModal.tsx`, `amneziawg-obfuscation.ts`, `amneziawg.tsx` outbound,
  `tunnel.ts` schema + `tunnel.tsx` form, masking/tunnel/csqtt/qwdtt tests).

`bin/check-lucx.sh` now enforces the header on the gofumpt file set, `bin/*.sh`,
and LucX-named frontend files, so this cannot regress.

### 2. `bin/build-release.sh` — NOT A BUG (withdrawn)

The "mojibake" in the first pass was a PowerShell console codepage artifact —
the file on disk is valid UTF-8. Only the missing SPDX header was added.

### 3. Signature-algorithm lists in `internal/awg/cps/cps.go` — FIXED

All three browser lists deviated from the real clients (fingerprint tells):

- `chromeSigAlgs` had `0x0804` and `0x0201` duplicated → now the real Chrome
  list `{0403, 0804, 0401, 0503, 0805, 0203, 0806, 0402, 0201}`.
- `firefoxSigAlgs` had `0x0604` and `0x0201` duplicated → now the NSS list
  `{0403, 0503, 0603, 0804, 0805, 0806, 0401, 0501, 0203, 0201}`.
- `safariSigAlgs` had a bogus `0x0601` → now `{0403, 0804, 0401, 0503, 0805,
0501, 0806, 0201}`.
- `writeDelegatedCredentialsExt` used the same wrong `0x0604` → `0x0603`.

### 4. `internal/lucx/tunnel/manager.go` — `orphanKeyFromFilename` — NOT CHANGED

The `-`/`.` separator order edge case cannot produce a wrong key for any
filename the code actually writes (`{key}.toml`, `{key}-hosts.toml`, etc.).
Left as is — documented here only.

### 5. `internal/awg/import_discover.go` — `applyClientCPS` version bump — NOT CHANGED

A 1.5 server conf + client keys carrying I-fields silently upgrades to v2.
Rare and self-correcting (the kernel accepts the result or the import preview
shows it). Left as is.

### 6. `internal/web/service/sidecar_outbound.go` — `AddOutbound` ordering — FIXED

The default-tag uniqueness check now runs before the tag `Update`, and a
failure deletes the just-created row instead of leaving it persisted.

### 7. `internal/lucx/tunnel/manager.go` — `Remove`/`Ensure` file race — FIXED

`removeManagedFiles` now runs inside the `opMu` critical section, so a
concurrent `Ensure` on the same key cannot write a config that `Remove` then
deletes mid-start.

### 8. `internal/awg/import_parse.go` — name-only peers dropped — NOT CHANGED

Cosmetic undercount in the import preview only.

### 9. `internal/sub/awg_service.go` — remark sanitisation — NOT CHANGED

Comments in a .conf are inert; control chars in a remark are cosmetic.

### 10. `internal/awg/client_conf.go` — `MTU = 0` — FIXED

`renderClientConf` now omits the MTU line when `s.MTU <= 0` instead of
rendering `MTU = 0`, which awg-quick rejects. (Server side defaults MTU in
`Instance` construction and needs no guard.)

### 11. `internal/web/service/xray.go` — `awgTunGateway` collision — FIXED

ids ≥ 254 now map into `10.253.x.y/30` (id mod 65536) instead of
`10.252.(id%253)+1/30`, which collided for ids 253 apart. Small ids keep their
existing `10.254.{id}.1/30` mapping.

### 12. `internal/lucx/tunnel/anytls_traffic.go` — session count — NOT CHANGED

Ephemeral-port collision with a fixed listen port is near-impossible.

### 13. `bin/install-awg-module.sh` — `git_clone_sha` — FIXED

`rm -rf "$dest"`/`mkdir` now run only after a successful fetch + `tar -tzf`
verify, so a network failure no longer wipes the destination.

### 14. `bin/install-awg-module.sh` — `NEWEST_HEADERS` — FIXED

`ls -d ... | head -1` picked the lexically-lowest (oldest) kernel headers;
now `sort -V | tail -1` picks the actual newest.

### 15. Duplicated AWG obfuscation rendering — NOT CHANGED

`inboundAwgHints` (service) and the sub-package path both render the same
block consistently today; extracting a shared renderer is a refactor, not a
bug fix.

## Verified clean (first pass)

- `internal/web/service/tunnel.go` `downloadBinaryTo`: SSRF guards present.
- `internal/database/db.go` migration order correct.
- `internal/lucx/tunnel/manager.go` locking model sound.
- `internal/awg/{manager,instance,process,platform_linux,client_manager,
diagnostics,portfwd*,signature/capture}.go` — no correctness issues.
- `internal/lucx/tunnel/{naive,process,auth,tunnel,qwdtt_routing,
tproxy_firewall_linux,sidecar_traffic_linux,mieru_pattern,traffic}.go` —
  no correctness issues.
- `internal/web/job/awg_job.go`, `internal/web/controller/{awg,tunnel}.go` —
  binary upload has ELF magic check + size cap + tmp+rename.
- `internal/awg/cps/cps.go` ClientHello builders — GREASE dedup, padTo512 and
  varint boundaries correct.
- `internal/awg/import_{parse,discover,docker,build,backup}.go` — parser,
  dedup, backup-before-adopt, docker stop all sound.
- `internal/lucx/tunnel/trusttunnel.go` — TOML escaping, TLV encoder, cert
  validation correct.
- `internal/lucx/tunnel/gateway*.go` — SNI dedup, port allocation, PROXY
  gating, l4http probe cache sound.
- `internal/sub/awg_service.go` credential scoping correct.
- `frontend/src/schemas/protocols/inbound/awg.ts` — H-range rejection on
  v1.5 consistent with `genHSingle`.
- `internal/web/service/xray.go` injection order — awg/sidecar outbounds
  precede egress injectors (lucx.133 stays fixed).

## Not reviewed

Subagent results were lost; these got a light pass at most:
`internal/awg/cps/{descriptor,domains,session_*}.go` internals,
`internal/lucx/tunnel/{olcrtc,csqtt,cover,tproxy}*.go` renderers,
`internal/web/service/{awg_outbound,awg_host,inbound_sublink}.go` bodies,
`internal/sub/{service,json_service,clash_*}.go` LucX hooks beyond AWG,
`internal/web/runtime/local.go` hook blocks, `install.sh`/`update.sh` hook
ordering, frontend `wireguardConfig.ts`/`inbound-link.ts` conf generation,
TanStack Query invalidation after outbound mutations.
