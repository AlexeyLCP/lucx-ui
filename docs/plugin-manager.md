# Plugin Manager — project doc

Status: **the plugin manager's Stage 1–3 are implemented and released —
Stage 1 (process-management MVP) shipped in `v3.11.0-lucx.333` (2026-10-31, alpha),
Stage 2 (fail-closed safety and enforcement) and the whole Stage 3
(execution + events + panel/subscription seams) shipped in
`v3.11.0-lucx.338` (2026-11-02)**, each increment live-verified on the test
stand. Follow-up fixes in lucx.334 (i18n keys, sidebar, `CsqttMember`
migration) and lucx.335 (translations, settings navigation).

- Today's plugin contract (what a plugin may/may not do, manifest format, API): [plugin-sidecars.md](plugin-sidecars.md)
- Prompt to write/review a plugin: [plugin-prompt.md](plugin-prompt.md)
- Plugin UX requirements + `better-csqtt` notes: [wishlist.md](wishlist.md)

## Goal

One supported mechanism that adds any supervised sidecar to the panel — **without forking or rebuilding the panel**:

1. **Any new sidecar.** An operator installs a third-party daemon (new tunnel core, helper service, exporter) as a ZIP. Panel supervises it: install, start/stop/restart, logs, delete.
2. **Fix / rework the Telegram bot.** The bot logic today is compiled into the panel (`internal/web/service/tgbot`). The plugin system must let bot logic live in a plugin, so it can be fixed and evolved independently of panel releases.
3. **Bots for other messengers.** A new bot (another messenger) ships as a plugin next to the built-in bots (Telegram, Discord), reusing the same seams instead of patching the panel.
4. **Panel UI customization.** Operators change the look and shape of the panel: own color schemes, menu changes, custom pages — without rebuilding the frontend. This splits into three tiers of difficulty (see Stage 5).

Non-goal (declared in plugin-sidecars.md): the plugin system does not sandbox arbitrary native code. Install only trusted packages.

## Stage 1 — process management (done, lucx.333)

Code: `internal/lucx/plugin/plugin.go` (+ `plugin_test.go`), controller wired in `internal/web/controller/api.go`; page under **Settings → Cores → Sidecar plugins**.

- ZIP with `manifest.json`: id regex `^[a-z0-9][a-z0-9_-]{0,63}$`, `apiVersion=1`, `type=sidecar`, relative `entrypoint`, architecture match for host GOARCH, ≤ 200 MB.
- Isolation: `bin/plugins/<id>/`, env `LUCX_PLUGIN_ID` / `LUCX_PLUGIN_VERSION` / `LUCX_PLUGIN_DATA`; stdout/stderr → `plugin.log`.
- Lifecycle: start/stop/restart; Stop = SIGINT → 3 s → SIGKILL. **No auto-restart, no auto-start on panel boot** — start is manual.
- REST API (session-auth): `/panel/api/plugins` — list, status, logs, install, start/stop/restart, delete.
- `manifest.permissions` (`network`, `panelApi`, `bot`) is **declared only — not enforced, not validated**.

## Stage 2 — safety (implemented, released in lucx.338)

Live verification on the test stand (2026-11-01, `144.31.224.212`, KVM,
Debian 12, dev build of b6f9fc9a): install/start/status/delete via the real
API with a Bearer token; refusal of `panelApi` manifests at install; health
probe pass (tcp → sshd) and fail (tcp → closed port with the dial error as
detail); plain plugin keeps the host netns and PING works; `maxMemoryMB: 64`
lands as `ulimit -v 65536` (RLIMIT_AS verified inside the process); the
netns-isolated plugin is refused with a fail-closed message because the
panel's hardened unit denies the `CLONE_NEWNET` clone. Two product bugs found
by that run and fixed: relative-entrypoint exec doubled its path under
`cmd.Dir` (fix: absolute path), and the ulimit wrapper must replace
`cmd.Path` rather than prepend to `cmd.Args` (Go executes `Path`; `Args` is
argv only).

- **Permission enforcement** (fail-closed): `panelApi`/`bot` must stay empty in
  apiVersion 1 — a non-empty list is refused at install and start; `network`
  understands only `["none"]` (own network namespace, Linux); unknown rules are
  refused. Absent/empty `permissions` keeps the MVP behavior.
- **Resource limits**: `resources.maxMemoryMB` (16..131072) → RLIMIT_AS on the
  child (Linux only). CPU/process caps were not implemented — RLIMIT semantics
  do not give a percent and cgroups are unusable under the hardened systemd
  unit; revisit with the Stage 3 seams.
- **Health checks**: `health {type http|tcp, target, interval, timeout}`,
  reported as pass/fail plus detail and timestamp in `/status`. Probes do not
  restart the plugin (contract unchanged) and are rejected together with
  network isolation.
- **Fail-closed platform rule**: a manifest asking for enforcement on a host
  without it (non-Linux, or unprivileged Linux) is refused at start and the
  refusal lands in the plugin status.
- **UX**: the plugin card shows the enforcement tags (isolated / memory cap /
  not enforced here) and the probe status; six new i18n keys landed in all 14
  locales; a components test pins the card rendering.

Back-compat: plugins installed before Stage 2 (no enforcement requested)
start exactly as before; manifests declaring only empty permission arrays stay valid.

## Stage 3 — execution + seams (implemented and live-verified; shipped in lucx.338)

What plugins could **not** do before Stage 3 (no panel API, no DB, no bot tokens,
no subscription seams) is now unlocked through **authenticated seams**, shipped
incrementally (3a unit runner → 3b events → 3c panel API + bot tokens → 3d
sub/after), every increment live-verified on the test stand:

- authenticated **panel API** seam (plugin calls panel endpoints with a scoped credential, not operator login) — **3c, done**;
- **bot token** seam (panel delivers a stored token to a trusted, permission-granted plugin; token stays in panel settings) — **3c, done**;
- **subscription seams** — read/export (via `panelApi: ["clients:read"]`) and the `sub/after` hook (injected links after client entries) — **3d, done**;
- DB access: **no direct DB** is the working decision — plugins go through the panel API seam; revisit only if the API seam proves insufficient.

Seams need the plugin to serve requests on a unix socket — that contract and the event
delivery over it are Stage 3b/3c. Stage 3 ships in increments:

| Inc | Content                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Why this order                                                                                                                                                                    |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 3a  | **Unit runner**: plugins start as `systemd-run` transient units (PrivateNetwork, ProtectSystem/caps, MemoryMax, CPUQuota) with the in-process child runner as fallback                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Resolves open question 8 live-verified under the hardened panel unit (netns EPERM); unit namespace/protect properties are kernel-enforced regardless of uid                       |
| 3b  | **Socket contract + events**: manifest `events` list (bus vocabulary only, unknown names refused at install), panel reuses `internal/eventbus` per-plugin subscriptions (own 64-event queue + serial worker = ordered delivery), POSTs envelopes `{id,type,source,time,data}` to `/_lucx/events` on the unix socket (best effort, 5 s, no retry), `delivered`/`failed` counters in the status; re-subscribes automatically after a panel restart                                                                                                                                                                          | Socket gets a real consumer (events) before seams lean on it; delivery rides the bus's subscriber machinery; `client.*`/`inbound.*` wait for their publish points in later stages |
| 3c  | **API seam + bot token seam (implemented, live-verified)**: scoped panel credential at start (env `LUCX_PANEL_TOKEN`/`LUCX_PANEL_URL`, token persisted per plugin across panel restarts), permission matrix `panelApi: ["resource:read\|write\|*"]` enforced inside the production auth path (`ApiScopePlugin` + `enforceTokenScope`, never-reachable route list always dark; plugins may not combine the seam with `network:["none"]`); `bot: ["telegram"\|"discord"]` delivers panel-stored tokens as env                                                                                                               | Smallest seam that unblocks "bots that read panel data"; enforcement rides the existing scope machinery instead of a gateway service                                              |
| 3d  | **Subscription seams (implemented, live-verified)**: read/export rides the API seam (`panelApi: ["clients:read"]` already covers `/clients/export`, `/clients/subLinks/:subId`, `/clients/links/:email`); link injection = `sub: ["after"]` granted separately — the panel POSTs the subscription context to `/_lucx/sub/after` on the plugin socket and appends the returned links after the client entries of the raw /sub body (additive only, 300 ms, fail-open, caps 32/call and 64/body; expired/depleted info-node responses, the sub info page, json/clash/awg bodies and panel link displays are never injected) | Rule 0/0b: never mutates issued client configs; the raw `/sub` text body is the one place injections make sense without format churn                                              |

Runner semantics (3a decision, refines the borrowed AllIsA1 design to run in-panel instead of a separate host service):

- A plugin that requests enforcement (`network: ["none"]`, `maxMemoryMB`, `maxCPUPercent`) is started as a **transient systemd unit** (`lucx-plugin-<id>`, `systemd-run`): `PrivateNetwork=yes` for netns, `MemoryMax`/`LimitAS` for the memory cap, `CPUQuota` for the CPU cap, empty `CapabilityBoundingSet`, `ProtectSystem=strict`, `ProtectHome=yes`, `PrivateTmp=yes`, `PrivateDevices=yes`, `RestrictNamespaces=yes`, `NoNewPrivileges`, `KillSignal=SIGINT` + `TimeoutStopSec=3` (Stop law preserved), `StandardOutput/StandardError=append:` to `plugin.log`, `WorkingDirectory` = plugin dir.
- The unit runs **as the panel's user** (no DynamicUser in 3a: plugin code and data stay under the panel's ownership; data-path contract unchanged). Confinement is reported honestly as _unit (partial)_ — namespaces, mounts, capabilities and limits are kernel-enforced; the filesystem stays readable, unlike AllIsA1's strong "own user" level. DynamicUser/state-directory ownership is reconsidered with the socket stage.
- A plugin that requests nothing may still start as a unit when available, or fall back to the child process (today's behavior). A plugin that requests enforcement and a host where the panel cannot create units → **refused at start, fail closed** (no silent downgrade).
- The panel supervises state via `systemctl show` (ActiveState/MainPID); stop/restart goes through the unit; plugin units outlive a panel restart by design (documented behavior change), delete/uninstall stops the unit first.

Live verification (2026-11-02, test stand, dev build): `network: ["none"]` started as a unit under the hardened panel unit — distinct `ns/net` inode from the panel, only `lo` inside, `PING-BLOCKED-AS-EXPECTED`; `maxCPUPercent: 30` landed as `CPUQuotaPerSecUSec=300ms`, `maxMemoryMB: 64` as `MemoryMax=67108864`; all three test plugins reported `runner: unit`; units survived a panel restart (status stays `running` via `systemctl show` with the same MainPID); stop/delete leave no units behind. Two runner lessons baked into the code: systemd ≤ 252 has no `systemd-run --replace` (a pre-clearing `systemctl reset-failed` is used instead), and **systemd 252 rejects relative `-p` paths silently (exit 1, no stderr)** — all unit properties must be absolute, cwd-resolved paths.

## Stage 5 — UI seams (planned; goal scenario 4)

Feasibility assessment (2026-11-01). The frontend is React 19 + AntD 6, built at
release time and embedded into the Go binary (`frontend/CLAUDE.md`), so plugins
cannot ship raw code inside the panel bundle. Three tiers:

| Tier                               | What                                                     | Mechanism                                                                                                                                                                           | Verdict                                                                                                 |
| ---------------------------------- | -------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| A. Themes / color schemes          | Own palettes, light/dark variants, token overrides       | Panel settings (AntD `ConfigProvider` theme tokens) + optional theme manifests from plugins — runtime CSS/token switch, no rebuild                                                  | **Realistic**, could even land as a panel feature without plugins                                       |
| B. Declarative menu + plugin pages | Extra sidebar entries, custom pages served by the plugin | Manifest declares UI entries; the panel renders them as iframes proxied through the panel (plugin hosts its own HTML/JS in its data dir), keeping plugin JS out of the panel bundle | **Realistic**; sandboxed by iframe, needs the Stage 3 API seam for data                                 |
| C. Deep UI injection               | Arbitrary React components inside panel pages            | Third-party JS in the panel bundle                                                                                                                                                  | **Not planned**: admin-session XSS risk + version coupling; revisit only for signed/first-party plugins |

A and B are the working scope: themes first (cheap, standalone), then the menu/page seam once Stage 3 delivers an authenticated API seam — a plugin page otherwise has nothing safe to talk to. UI UX requirements from the wishlist apply to every plugin-rendered page.

## Stage 6 — plugin runtime v2 (researched against better-csqtt)

First real consumer of the seams is `better-csqtt` (checked 2026-10-31, wishlist). Its requirements define the v2 runtime:

- `lucx-plugin.json` loading (next-generation manifest beyond the stage-1 `manifest.json`);
- plugin credentials;
- Unix-socket process supervisor;
- `sub/after` seam API.

Then: install `better-csqtt`, verify per-client links, member creation/deletion, disabled clients, and fallback behavior. Manifest evolution/compatibility policy (`apiVersion` 1 → `lucx-plugin.json`) to be decided when stage 4 starts.

## Scenarios → requirements

| Goal scenario                                              | Needs                                           | Status            |
| ---------------------------------------------------------- | ----------------------------------------------- | ----------------- |
| Any new sidecar                                            | Stage 1                                         | Works in alpha    |
| Fix/rework Telegram bot                                    | Stage 2 + panel API & bot-token seams (Stage 3) | Seams ready (338) |
| Bot for another messenger                                  | Stage 3 seams + multi-bot coexistence rules     | Seams ready (338) |
| Own color schemes                                          | Stage 5 tier A                                  | Not started       |
| Menu changes / plugin pages                                | Stage 5 tier B + Stage 3 API seam               | Not started       |
| Hardened sidecars (deny-network, memory cap, health probe) | Stage 2                                         | Implemented (338) |

## Ideas backlog (what else plugins could enable)

Grounded wish, not committed work. "Today" = stage 1 alpha (no panel access).

| When                               | Idea                                                                                                                                     | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Now                                | **Install by GitHub repo link** (next to the ZIP)                                                                                        | A URL fetcher feeding the exact same validation path as the ZIP install: public GitHub repos only, optional `/tree/<branch\|tag\|commit>` ref pin, archive cap like the ZIP's 200 MB. Two-step UX: preview request returns the validated manifest, the panel shows id/name/version/entrypoint/permissions/enforcement plus a trust warning, install happens only after the confirm click; re-install from the same link updates the code and keeps `data/`.                                                                                                                                                                                                      |
| Today                              | New cores as sidecars                                                                                                                    | Working pattern: Telemt, MTProto Zig ship this way                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| Today                              | Independent daemons: exporters, own-token alerters, cron-style tasks                                                                     | No panel access needed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| Stage 3                            | Traffic dashboards, mass AWG ZIP export, messenger bots issuing clients, CRM/shop provisioning, `sub/after` subscription post-processors | Needs panel API + token seams; mass ZIP export is the excluded wishlist item moving into a plugin                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Stage 3 + events                   | Event-driven bridges: on inbound down / traffic limit → Slack/gotify/email, auto-actions                                                 | `internal/eventbus` exists; needs an event seam on the API seam                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Data plugins (new manifest `type`) | **Preset packs**: obfuscation/stream presets, routing presets, geo rule sets                                                             | Declarative JSON validated by Zod, merged into form choices — no code in the panel bundle, no iframe; presets currently live in the frontend build (`finalmask-presets.ts`, `mieru/presets.ts`, `wgo/preset.ts`); feasible before Stage 3. Scope note: Xray disguises are structural (transport / REALITY fingerprint / mKCP header / FinalMask) — a preset pack is static field values plus a form selector, **no generation algorithm**. Randomized params with invariants are AWG-only and already exist as product code (`cps` + `generateObfuscation`); adding a new AWG mimicry profile is a product change (cps generator + Zod enum), not a preset pack. |
| Stage 5                            | Themes, menu, custom pages, plugin-hosted preset editors                                                                                 | See Stage 5 tiers                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |

A plugin `type` per group is the working rule: `sidecar` (process), `preset` (data), later `ui`. One manifest type per package keeps validation honest.

## Open questions

1. Token delivery: **decided (lucx.338, stage 3c/3d)** — env at process start (`LUCX_PANEL_TOKEN`, `LUCX_PANEL_URL`, `LUCX_TELEGRAM_BOT_TOKEN`, `LUCX_DISCORD_BOT_TOKEN`); credentials survive panel restarts; rotation = restart the plugin (a re-start regrants). A credentials endpoint stays a v2 idea.
2. Coexistence with built-in bots (Telegram, Discord): replace the built-in one, or both run with conflict rules (settings ownership, chat routing)?
3. Subscription permission granularity: **decided (lucx.338, stage 3d)** — read/export needs no dedicated grant (`panelApi: ["clients:read"]` covers the export and per-client/per-subId link routes); link injection is the `sub: ["after"]` permission, granted separately, additive-only over the raw /sub body.
4. Health-check shape: HTTP/TCP probes ship in Stage 2 (report-only, no auto-restart); an in-plugin exec probe and auto-restart policies stay open.
5. UI page auth: how a plugin iframe authenticates to the panel API seam (scoped token injected into the iframe; expiry and revocation), and CSP/sandbox policy for it.
6. Theme scope: stock panel themes as a settings feature first, plugin-provided themes after — or themes skip the plugin system entirely?
7. Data plugin type (`preset`): validation depth (full Zod mirror of form choices?), merge priority vs. stock presets, and whether packs are versioned with `apiVersion`.
8. Network isolation ops: **decided (lucx.338, stage 3a)** — the spawner is the `systemd-run` transient unit with `PrivateNetwork=yes`; the panel's own hardened unit only needs permission to talk to systemd once per start, so it no longer needs the netns clone. Until the runner lands, `network: ["none"]` stays fail-closed.
9. Install-by-link trust model: same-session confirm is enough, or should a second install from a NEW link always re-confirm; and does an update-by-link show the permission diff before applying (see AllIsA1's update UX).

## External prior art — AllIsA1/lucx-plugins

A community member built a parallel plugin system as an **overlay over upstream releases**
(<https://github.com/AllIsA1/lucx-plugins>, MIT): a signed build pipeline that rebuilds the panel
binary with the plugin UI baked in, plus a separate `lucx-plugin-host` service that installs, runs
(confined), updates and catalogs plugins. Their pain point matches ours to note: every upstream
release means re-applying the overlay and rebuilding the UI — auto-builds break when the panel UI
churns. Our roadmap keeps plugin logic outside the panel bundle (seams, processes, sandboxed
iframes) precisely to avoid that class of breakage.

What is worth adopting, and where it slots into the roadmap:

| Their feature                                                                                                                                                                         | Our road                                   | Notes                                                                                                                                                                     |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Install by GitHub link (public only, ref pin, permission preview before confirm)                                                                                                      | Backlog "Now"                              | Design above; their two-step check→review→install flow is the UX to match                                                                                                 |
| Plugins as own systemd transient units (dynamic user, no capabilities, no network unless granted, memory/CPU caps)                                                                    | Stage 3 spawner decision (open question 8) | Directly solves our verified netns-EPERM under the panel unit; stronger than in-process Cloneflags                                                                        |
| Events: POST to the plugin socket (`client.*`, `inbound.*`, `xray.crash`, `node.*`, `cpu.high`…)                                                                                      | Stage 3 (eventbus seam)                    | Their event vocabulary + best-effort ordered delivery + counters are a working reference                                                                                  |
| `pipeline:xray.config` — JSON Patch (add/remove/replace/test) of the generated Xray config, protected sections (api/stats/api-inbound/existing inbounds), 30 s cache, skip-on-failure | Stage 3 seam design                        | The concrete contract for blocklist/routing preset plugins (their WARP-over-AWG and ad-block plugins build on it)                                                         |
| Seams: before/replace/after on API + sub routes, never-reachable route list, failure counting/backoff                                                                                 | Stage 3                                    | Adopted in stages 3b–3d (events, panel API with never-reachable list, sub/after with their call budgets and caps); before/replace on API routes and backoff stay v2 ideas |
| UI pages in sandboxed iframes without `allow-same-origin`, CSP `connect-src 'none'`, time-limited token frame, postMessage bridge + `lucx.applyTheme()` SDK                           | Stage 5 tier B                             | Safer than the earlier iframe sketch; adopt frame-token + bridge-v1 ideas into the Stage 5 spec                                                                           |
| Plugins on nodes (owner opt-in; master pushes pinned commit + manifest hash; update-on-node)                                                                                          | Stage 4+ / Smart Cluster                   | Scope: `master`/`node`/`both`                                                                                                                                             |
| Signed catalog with sequence-only-grows + manifest-hash pinning                                                                                                                       | After Stage 3                              | v1 stays link-based; catalog is a trust layer on top                                                                                                                      |

License note: their examples are MIT and ideas are free to take; the service code is an overlay
build, not a drop-in for our tree — port concepts (contracts, limits, threat model), not code,
or relicense ported snippets per [LICENSING.md](LICENSING.md).

## Constraints (project law)

- Rule 1/2 (LUCX-HOOK, isolated modules): plugin code lives in `internal/lucx/plugin/…`; any touch of upstream files gets `LUCX-HOOK` markers.
- Rules 0 / 0b: any seam that touches subscriptions or data must not mutate issued client configs and must survive overlay installs on a vanilla 3x-ui DB.
- Rule 7: every new plugin function ships with tests (`go test ./internal/lucx/...`).
- Rule 10: new LucX-owned files carry the SPDX header; license split per [LICENSING.md](LICENSING.md).
- Rule 11: this doc stays in English; commits in Russian.

## Project log

- 2026-11-02 — Stage 3 closed in increments: 3a unit runner, 3b events over
  unix socket, 3c panel API + bot token seams, 3d sub/after seam — all
  implemented and live-verified on the test stand; subscription read/export
  resolved to `panelApi: ["clients:read"]`; `sub: ["after"]` injection with
  AllIsA1-reconciled limits.
- 2026-10-31 — lucx.333: MVP shipped (ZIP validation, lifecycle, API, panel page); lucx.334: i18n/migration/CI fixes; lucx.335: translation/navigation fixes.
- 2026-11-01 — this project doc created.
- 2026-11-01 — added goal scenario 4 (UI customization) and Stage 5 UI seams (tiers A/B/C).
- 2026-11-01 — added Ideas backlog (incl. `preset`-type data plugins for obfuscation/routing presets).
- 2026-11-01 — Stage 2 implemented: fail-closed permission validation, netns memory-limit enforcement (Linux), http/tcp health probes, status/UI exposure, docs + 14 locales updated.
- 2026-11-01 — release v3.11.0-lucx.336 (Stage 2 + fix for stale vendored olcRTC core, #183); live stand verification found/fixes the relative-entrypoint exec path and the ulimit wrapper Path/Args bug.
- 2026-11-01 — external prior art (AllIsA1/lucx-plugins) mapped into the roadmap; "install by GitHub link" added to the backlog as the next smallest increment.
