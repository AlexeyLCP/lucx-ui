# 04 — Current state

Extracted from AGENTS.md. This file is project law.
Read when the task depends on a known issue or product constraint.
Changelog: `docs/progress.md`, last 1–2 entries only.

---

## Known Issues

### 1. ~~AWG sidecar bloated vs mtproto (the baseline)~~ — CLOSED

**Resolved (2026-07-13):** refactor by deleting dead code. Files `params.go`, `cps.go`, `config.go`, `templates.go`, `types.go`, `helpers.go` + 5 tests were fully dead — their functions (`GenerateAWGParams`, `GenerateCPS`, `BuildServerConfig`, `RenderPostUp`, etc.) were only called by tests, no live call site used them. Key/obfuscation generation is done in the frontend (`createDefaultAwgInboundSettings`). AWG cut from 19 to 8 files (6 .go + 2 tests) — nearly symmetric with mtproto (9 files). Upstream updates now need ~20 files ported instead of 29.

**Finished (2026-07-18):** slimming toward mtproto parity. `cps/` and `signature/` stay separate. **Do not re-slim:** import, diagnostics, outbound (`awgo-N`), portfwd, vpnuri, kernel/gVisor fallback are live product, not dead mtproto copies. A second Amnezia path (`protocol=amneziawg` / `amneziawgnet`) exists for no-module hosts — do not mix with kernel `protocol=awg` on the same iface name without checking.

### 2. ~~Sidecar not verified in real runtime on a VPS~~ — CLOSED

**Resolved (2026-07-16):** sidecar verified in real runtime on tester VPSes (VladufQa on ruvds-rdu8b, Kirill Rudenko on runode). Kernel routing (without routeThroughXray) works — handshake, ICMP, HTTPS, traffic. routeThroughXray works after PR #13 (needRestart + policy routing + sniffing). Releases v3.5.0-lucx.20–31 tested by testers.

### 3. Dependabot — security updates only

Version updates (weekly PRs for new versions) are disabled — `updates: []` in `.github/dependabot.yml`. This removes noise from minor npm/gomod/github-actions updates that piled up as open PRs (10 were closed before the v3.5.0 migration). Security updates (CVE) stay on via GitHub Settings → Dependabot security updates — Dependabot will auto-open a PR for any found vulnerability. To restore version updates — replace `updates: []` with the full list (template in a comment in the yml).

**⚠️ Gotcha (2026-07-18):** `updates: []` does NOT stop **grouped** version-update PRs if they are enabled in the GitHub UI (Settings → Advanced Security → Dependabot grouped version updates) — that’s a separate toggle that doesn’t read our yml. On 18.07 ten PRs appeared (grpc, antd, vite, storybook, actions/*) — closed, each with comment “version updates disabled”. Dependabot on a closed PR says “won't notify you again about this release, but will on a new version” — so it comes back periodically. **Full disable:** Settings → Advanced Security → turn off Dependabot version updates (+ grouped). Every new version-update PR — close with the same comment; **check the queue before every push (step 11.5)**.

### 4. routeThroughXray — harder than mtproto

AWG routeThroughXray is **fundamentally harder** than mtproto because of the kernel→userspace bridge:

| | mtproto | AWG |
|---|---|---|
| Sidecar type | userspace daemon (mtg) | kernel module (awg-quick) |
| Traffic type | TCP (FakeTLS → MTProto) | IP packets (kernel) |
| Bridge into Xray | SOCKS5 loopback (TCP) | TUN inbound (IP) |
| How traffic reaches Xray | mtg itself dials 127.0.0.1:port | policy routing: `ip rule iif awgN lookup 1000+N` → `default dev tunN` |
| NAT | not needed (mtg → SOCKS → Xray) | not needed (Xray → outbound NATs itself) |
| needRestart | `mtprotoRoutesThroughXray` in AddInbound/DelInbound/UpdateInbound | `awgRoutesThroughXray` — same points (added in PR #13) |
| Route maintenance | not needed (SOCKS port is permanent) | `ensureXrayRouting` in the reconcile loop (10s) — tunN is recreated on every Xray restart. In kernel mode — `ensureNatRules` (same loop): MASQUERADE/FORWARD die on iptables flush |
| Sniffing | SOCKS inbound does it itself | TUN inbound — needs explicit `sniffing: {routeOnly:true}` (without it domain rules don’t work) |

Not to re-add: tun2socks (replaced by TUN inbound), DNS in the server .conf (breaks system DNS), fixed table 100 + gateway 10.254.254.1/30 (break multi-inbound).

**Post-restart window (CLOSED 2026-07-19):** restarting Xray (panel button) killed tunN and the `default dev tunN table 1000+N` route until the next AWG reconcile-cron tick (up to 10s routed clients had no internet; “re-select outbound” just triggered reconcile earlier than cron). Fix: `ensureAwgRouting()` in `RestartXray` right after `p.Start()` — route is restored in sync with the new tunN. Verified on v3.6.0-lucx.48/test2: after `systemctl restart x-ui` at t+8s the route is there, ping 0% loss.

### 5. ~~AWG3 (AmneziaWG 3) — forward-compat field `headerProtectionKey`~~ — CLOSED (lucx.50)

**Resolved (2026-07-31):** AWG3 officially merged upstream and enabled in LucX-UI from lucx.50.
- `amnezia-vpn/amneziawg-linux-kernel-module`: PR #192 “feat: AmneziaWG 3.0” merged to master 30.07.2026T21:54Z, tags **`v3.0.20260730`**/**`v3.0.20260731`**(+ -02…-04). `header_protection.c` exists only from those tags; in `v1.0.20260611`/`v1.0.20260725` it does NOT. ⚠️ the kernel rejects HPK with `-EINVAL` if any of S1–S4 < 12.
- ⚠️ **Module version numbers do NOT reflect protocol version (lucx.58):** upstream stamps `PACKAGE_VERSION="1.0.0"` (src/dkms.conf) and `WIREGUARD_VERSION=1.0.0` (src/Makefile) in **every** release — a module built from a v3 tag reports the same “1.0.0” via modinfo/dkms as a v1 module. The only reliable AWG3 signal is a functional probe: symbol `awg_header_protection_set_key` in `/proc/kallsyms` (kernel) + `awg version` ≥ v3 (tools). See Pattern 1j.
- `amnezia-vpn/amneziawg-tools`: PR #60 merged 30.07.2026, tag **`v3.0.20260730`**. `HeaderProtectionKey` is parsed in `.conf` (`config.c`, `parse_key`); `awg version` prints `amneziawg-tools v3.0.20260730 - https://amnezia.org` (fallback from src/version.h when git-describe didn’t run).
- ⚠️ Building module v3.0 failed on kernels < 6.7 (`nla_put_uint`) — fix is already in master, so a current master build works on older kernels too; auto kernel upgrade in `bin/install-awg-module.sh` (lucx.58) solves it systemically.

**What lucx.50 did:**
1. `generateObfuscation` (`controller/awg.go`) again returns `headerProtectionKey` — but **only when `awgVersion == "3"`** in the request. For v1.5/v2 the field is absent from the response (not `""`), so `regenerateObfuscation` (`Object.entries(obf).forEach(setValue)`) won’t overwrite the operator’s manual value.
2. Renderers `renderServerConf` (`manager.go`), `renderClientConf` (`client_conf.go`), `inboundAwgHints` (`inbound.go`) write HPK **only when `awgVersion == "3"` AND the key is non-empty**. For v1/v2 the line is omitted — old kernels keep working.
3. Generator `GenerateAWGParams` (`cps/params.go`) now **guarantees S1–S4 ≥ 12** (`MinSForHPK = 12`, `enforceSMin`) for all profiles — config is valid for AWG3 whether or not HPK is set. `GenerateHeaderProtectionKey()` + `AWGParams.WithHeaderProtectionKey()` generate the key (crypto/rand, 32 bytes, base64).
4. New field `awgVersion` (`"1.5"`/`"2"`/`"3"`) across the pipeline — on the inbound (server ceiling) and in client export (≤ ceiling, runtime selector in `ClientQrModal`/`ClientInfoModal`).
5. Migration renamed: `pruneAwgHeaderProtectionKey` → `migrateAwgVersion` (`migrate_awg_hpk.go`). Now backfills `awgVersion:"2"` on pre-lucx.50 inbounds/outbounds AND clears non-empty HPK from anything that isn’t v3 (fix for lucx.47 regression victims + guard against a future version bump).

**Lesson (still holds):** “Regenerate obfuscation” silently writes into the form everything the backend returned. Any field unsupported by the current kernel → reconcile crash. Solution — version-gate emission, not full silence: the field is returned/written only when the version explicitly supports it.

### 6. geo files overwritten on panel update — upstream behavior, we do NOT fix (decision 2026-08-09)

**Essence:** if geo is in the tarball, `update.sh` unpacks over the top → stock names reset. Symptom (Aleksandr SacredX, lucx.88): custom geosite groups vanished after web update → Xray won’t start. **Decision:** leave unpack-overwrite alone (parity with upstream). Advice: keep custom groups in a **separate filename**. lucx.198: fetch runs **before** first start so a fresh install is not missing geo. Geo is not packed in the panel tarball.

**Stock since lucx.99 (8 files):** `geoip/geosite.dat` (Loyalsoldier), `_IR` (chocolate4u), `_RU` (runetfreedom), **`_ROSCOM`** (hydraponique/roscomvpn-{geoip,geosite} — RKN geoblock / category-ru / category-ads / youtube/telegram/steam). ROSCOM is a separate name, doesn’t overwrite others’ customs. Update: panel Version → Geofiles / `x-ui` menu → RoscomVPN / `update-all-geofiles`. In routing: `ext:geosite_ROSCOM.dat:category-geoblock-ru` etc. (presets in `constants.ts`). Geodata browser picks up any `*.dat` in `bin/`.

**Important for diagnosing “/awg/ doesn’t work”:** sub endpoints (`/sub/`, `/json/`, `/clash/`, `/awg/`) listen on the sub service on a **separate port** (default 2096), not the panel port — reverse proxy must forward them to the sub port.

---
