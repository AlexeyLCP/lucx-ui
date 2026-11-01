# Plugin Manager — project doc

Status: **stage 2 (safety) implemented, unreleased — see its section below;
stage 1 (process-management MVP) shipped in `v3.11.0-lucx.333` (2026-10-31),
in alpha**; follow-up fixes in lucx.334 (i18n keys, sidebar, `CsqttMember`
migration) and lucx.335 (translations, settings navigation).

- Today's plugin contract (what a plugin may/may not do, manifest format, API): [plugin-sidecars.md](plugin-sidecars.md)
- Prompt to write/review a plugin: [plugin-prompt.md](plugin-prompt.md), see also the AI prompts in [plugin-sidecars.md](plugin-sidecars.md)
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

## Stage 2 — safety (implemented; unreleased)

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

## Stage 3 — panel seams (planned; unlocks bots)

What plugins may **not** do today (no panel API, no DB, no bot tokens, no subscription seams) becomes possible through **authenticated seams**:

- authenticated **panel API** seam (plugin calls panel endpoints with a scoped credential, not operator login);
- **bot token** seam (panel delivers a stored token to a trusted, permission-granted plugin; token stays in panel settings);
- **subscription seams** — read/export and `sub/after` hooks (injected links after client entries);
- DB access: **no direct DB** is the working decision — plugins go through the panel API seam; revisit only if the API seam proves insufficient.

This is the stage that "lets plugins replace bots or read panel data" (plugin-sidecars.md), i.e. scenarios 2–3 of the goal.

## Stage 5 — UI seams (planned; goal scenario 4)

Feasibility assessment (2026-11-01). The frontend is React 19 + AntD 6, built at
release time and embedded into the Go binary (`frontend/CLAUDE.md`), so plugins
cannot ship raw code inside the panel bundle. Three tiers:

| Tier | What | Mechanism | Verdict |
|---|---|---|---|
| A. Themes / color schemes | Own palettes, light/dark variants, token overrides | Panel settings (AntD `ConfigProvider` theme tokens) + optional theme manifests from plugins — runtime CSS/token switch, no rebuild | **Realistic**, could even land as a panel feature without plugins |
| B. Declarative menu + plugin pages | Extra sidebar entries, custom pages served by the plugin | Manifest declares UI entries; the panel renders them as iframes proxied through the panel (plugin hosts its own HTML/JS in its data dir), keeping plugin JS out of the panel bundle | **Realistic**; sandboxed by iframe, needs the Stage 3 API seam for data |
| C. Deep UI injection | Arbitrary React components inside panel pages | Third-party JS in the panel bundle | **Not planned**: admin-session XSS risk + version coupling; revisit only for signed/first-party plugins |

A and B are the working scope: themes first (cheap, standalone), then the menu/page seam once Stage 3 delivers an authenticated API seam — a plugin page otherwise has nothing safe to talk to. UI UX requirements from the wishlist apply to every plugin-rendered page.

## Stage 6 — plugin runtime v2 (researched against better-csqtt)

First real consumer of the seams is `better-csqtt` (checked 2026-10-31, wishlist). Its requirements define the v2 runtime:

- `lucx-plugin.json` loading (next-generation manifest beyond the stage-1 `manifest.json`);
- plugin credentials;
- Unix-socket process supervisor;
- `sub/after` seam API.

Then: install `better-csqtt`, verify per-client links, member creation/deletion, disabled clients, and fallback behavior. Manifest evolution/compatibility policy (`apiVersion` 1 → `lucx-plugin.json`) to be decided when stage 4 starts.

## Scenarios → requirements

| Goal scenario | Needs | Status |
|---|---|---|
| Any new sidecar | Stage 1 | Works in alpha |
| Fix/rework Telegram bot | Stage 2 + panel API & bot-token seams (Stage 3) | Not started |
| Bot for another messenger | Stage 3 seams + multi-bot coexistence rules | Not started |
| Own color schemes | Stage 5 tier A | Not started |
| Menu changes / plugin pages | Stage 5 tier B + Stage 3 API seam | Not started |
| Hardened sidecars (deny-network, memory cap, health probe) | Stage 2 | Implemented (unreleased) |

## Ideas backlog (what else plugins could enable)

Grounded wish, not committed work. "Today" = stage 1 alpha (no panel access).

| When | Idea | Notes |
|---|---|---|
| Today | New cores as sidecars | Working pattern: Telemt, MTProto Zig ship this way |
| Today | Independent daemons: exporters, own-token alerters, cron-style tasks | No panel access needed |
| Stage 3 | Traffic dashboards, mass AWG ZIP export, messenger bots issuing clients, CRM/shop provisioning, `sub/after` subscription post-processors | Needs panel API + token seams; mass ZIP export is the excluded wishlist item moving into a plugin |
| Stage 3 + events | Event-driven bridges: on inbound down / traffic limit → Slack/gotify/email, auto-actions | `internal/eventbus` exists; needs an event seam on the API seam |
| Data plugins (new manifest `type`) | **Preset packs**: obfuscation/stream presets, routing presets, geo rule sets | Declarative JSON validated by Zod, merged into form choices — no code in the panel bundle, no iframe; presets currently live in the frontend build (`finalmask-presets.ts`, `mieru/presets.ts`, `wgo/preset.ts`); feasible before Stage 3. Scope note: Xray disguises are structural (transport / REALITY fingerprint / mKCP header / FinalMask) — a preset pack is static field values plus a form selector, **no generation algorithm**. Randomized params with invariants are AWG-only and already exist as product code (`cps` + `generateObfuscation`); adding a new AWG mimicry profile is a product change (cps generator + Zod enum), not a preset pack. |
| Stage 5 | Themes, menu, custom pages, plugin-hosted preset editors | See Stage 5 tiers |

A plugin `type` per group is the working rule: `sidecar` (process), `preset` (data), later `ui`. One manifest type per package keeps validation honest.

## Open questions

1. Token delivery: env at process start vs. a credentials endpoint; rotation while plugin runs?
2. Coexistence with built-in bots (Telegram, Discord): replace the built-in one, or both run with conflict rules (settings ownership, chat routing)?
3. Subscription permission granularity: read-only export vs. link injection (`sub/after`) granted separately.
4. Health-check shape: HTTP/TCP probes ship in Stage 2 (report-only, no auto-restart); an in-plugin exec probe and auto-restart policies stay open.
5. UI page auth: how a plugin iframe authenticates to the panel API seam (scoped token injected into the iframe; expiry and revocation), and CSP/sandbox policy for it.
6. Theme scope: stock panel themes as a settings feature first, plugin-provided themes after — or themes skip the plugin system entirely?
7. Data plugin type (`preset`): validation depth (full Zod mirror of form choices?), merge priority vs. stock presets, and whether packs are versioned with `apiVersion`.
8. Network isolation ops: an isolated plugin's loopback is its own namespace — how such plugins expose ports to the host (forwarding policy) if they ever need to listen.

## Constraints (project law)

- Rule 1/2 (LUCX-HOOK, isolated modules): plugin code lives in `internal/lucx/plugin/…`; any touch of upstream files gets `LUCX-HOOK` markers.
- Rules 0 / 0b: any seam that touches subscriptions or data must not mutate issued client configs and must survive overlay installs on a vanilla 3x-ui DB.
- Rule 7: every new plugin function ships with tests (`go test ./internal/lucx/...`).
- Rule 10: new LucX-owned files carry the SPDX header; license split per [LICENSING.md](LICENSING.md).
- Rule 11: this doc stays in English; commits in Russian.

## Project log

- 2026-10-31 — lucx.333: MVP shipped (ZIP validation, lifecycle, API, panel page); lucx.334: i18n/migration/CI fixes; lucx.335: translation/navigation fixes.
- 2026-11-01 — this project doc created.
- 2026-11-01 — added goal scenario 4 (UI customization) and Stage 5 UI seams (tiers A/B/C).
- 2026-11-01 — added Ideas backlog (incl. `preset`-type data plugins for obfuscation/routing presets).
- 2026-11-01 — Stage 2 implemented: fail-closed permission validation, netns memory-limit enforcement (Linux), http/tcp health probes, status/UI exposure, docs + 14 locales updated.