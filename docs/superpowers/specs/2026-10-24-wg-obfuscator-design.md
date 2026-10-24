# WG Obfuscators (wg-obfuscator / Phobos) + SOCKS5-exit — Design

Date: 2026-10-24
Status: approved by maintainer (design dialogue 2026-10-24)
Scope: two features in one spec — (A) obfuscation presets on the kernel
WireGuard inbound, (B) standalone SOCKS5-exit inbound — both driven by one
engine binary.

## 1. Summary

Users asked for "wg-obfuscator (ClusterM)" and "wg-obfuscator (Phobos)".
Research result: these are **one engine**, not two.

- **ClusterM/wg-obfuscator** (GPL-3.0, C) — UDP→UDP obfuscation proxy in
  front of a WireGuard endpoint. Upstream masking: `STUN` / `AUTO` / `NONE`
  (since v1.4). INI config with per-instance sections. Client builds for
  Windows/macOS/Linux, OpenWrt/LuCI, MikroTik containers, Android port.
- **Ground-Zerro/Phobos** — a wg-easy-fork *panel* (AGPL-3.0) that vendors
  the ClusterM core in `src/phobos-obfuscator/` (GPL-3.0 with their
  modifications) and adds: `mode=socks5`, `role=relay|client|server`,
  masking `MEDIA` (RTP framing) and `TLS` (socks5-only), `socks5-users`
  (multi-user RFC 1929 validation server-side), per-preset isolation, and
  the `phobos://` client link format.

LucX integrates the **Phobos fork core** as one external GPL binary
(`wgobfs`), the same process-sidecar pattern as qWDTT/MTProxy. No AGPL
code is copied — only the binary and the public config/link formats.

Feature A: an **Obfuscation section on the kernel WG inbound** (off by
default): a list of presets (port/key/masking), one `wgo-{id}` process per
inbound, Phobos-compatible client exports.
Feature B: a **new inbound protocol `wgosocks`** (SOCKS5-exit, pattern of
naive/mieru inbound-only stacks): panel clients with login/password,
`wgosocks-{id}` process, `phobos://` share for SOCKS5 clients.

## 2. Engine facts (verified against sources 2026-10-24)

### 2.1 Phobos fork config (per-instance INI section)

Directive names (long forms; config keys match long option names):

| Key | Values / meaning |
|---|---|
| `mode` (`-M`) | `wireguard` (alias `wg`) / `socks5` |
| `role` (`-R`) | socks5 only: `relay` / `client` / `server` |
| `source-lport` (`-p`) | UDP/TCP port the process listens on |
| `target` (`-t`) | host:port to forward to (WG mode) / dial target (socks5 server exit) |
| `key` (`-k`) | XOR obfuscation key (string), same on both sides |
| `masking` (`-a`) | `STUN` / `MEDIA` / `TLS` (socks5-only) / `AUTO` / `NONE` |
| `media-ssrc` (`-S`) | RTP SSRC, hidden recognition token (hex/dec); fixed value recommended for reliable MEDIA detection |
| `media-pt` (`-P`) | RTP payload type 96..127 |
| `media-clock` (`-C`) | RTP timestamp clock, default 30 fps |
| `obfuscate-bytes` (`-O`) | WG mode; 0 = whole payload; defaults to 16 with MEDIA |
| `max-dummy` (`-d`) | 0..1024 random padding bytes, default 4 |
| `idle-timeout` (`-l`) | ms, default 300000 |
| `max-clients` (`-m`) | default 1024 |
| `socks5-users` (`-U`) | `login:pass,login:pass` — **role=server only**, validated in `socks5_auth_index()` over `socks5_users[]` (`MAX_SOCKS5_USERS` cap, `S5_CRED_MAX` length limit); zero users → NOAUTH |
| `static-bindings` (`-b`) | WG mode two-way: `ip:port:fwd_port`; socks5 relay: extra TCP routes `port:host:port` |
| `verbose` (`-v`) | 0..4 (ERRORS/WARNINGS/INFO/DEBUG/TRACE) |
| `threads`, `reuseport`, `source-if` (`-i`), `fwmark` (`-f`) | advanced |

`AUTO` semantics (upstream): server does not mask outgoing traffic by
default; if the client's first packet is masked, the server autodetects the
masking type and follows. `AUTO` in socks5 mode behaves like `NONE`.
`TLS` masking is rejected in wireguard mode. MEDIA = RTP framing (WG mode:
RTP over UDP; socks5 mode: RTP-over-TCP, RFC 4571).

### 2.2 `phobos://` client link format (docs/phobos-link-format.md)

```
phobos://<base64url(conf_text)>#<urlencoded(client_name)>
```

- base64url without padding; payload is the client `.conf` itself.
- Protocol detection by `mode` inside `[instance]`: absent/`wireguard` →
  WG payload (`[Interface]`/`[Peer]`/`[instance]`); `socks5` →
  `[instance]` + `[socks5]`.
- WG `[instance]` required keys (doc example values):
  `source-if = 127.0.0.1`, `source-lport = <clientWgLocalPort>`,
  `target = <server_host>:<extPort>`, `key = …`, `masking = STUN`,
  `verbose = INFO`, `idle-timeout = 300`, `max-dummy = 45`; with MEDIA,
  `media-ssrc` only if the preset pins a static SSRC (other RTP params
  stay random), `obfuscate-bytes` defaults to 16.
- SOCKS5 payload: `[instance]` with `mode = socks5`, `role = client`,
  `source-if`, `source-lport` (local socks5 port, e.g. 1080), `target`,
  `key`, `masking`, `verbose`; plus a separate section
  `[socks5] login = … / password = …` — kept outside `[instance]` because
  the binary errors on unknown keys; the client app reads `[socks5]` to
  build `socks5://login:password@127.0.0.1:<source-lport>`.
- Literal-`none` padding for missing fields applies **only** to the
  `phobos://` payload, never to the downloaded `.conf` (must stay
  wg-quick compatible). No schema version yet (future: `phobos://v2.`).

### 2.3 Client ecosystems

- **Fork builds** (Phobos installers): Keenetic/Netcraze (Entware+RCI),
  OpenWrt/ImmortalWrt (opkg+UCI), Debian/Ubuntu (apt+systemd), PhobosWG
  Android APK (WG client with built-in obfuscator engine, MEDIA capable),
  Windows client. Required for `MEDIA` / `TLS` / socks5 presets.
- **Upstream ClusterM builds**: Windows/macOS CLI, Linux, OpenWrt/LuCI,
  MikroTik containers, Android port. Support `NONE` / `STUN` (and AUTO)
  only — used via the plain-INI export.

## 3. Feature A — Obfuscation on the kernel WG inbound

### 3.1 UI

In the WireGuard inbound form (`frontend/src/pages/inbounds/form/protocols/wireguard.tsx`,
next to the existing `settings.kernel` block):

- Switch **"Obfuscation"** — default off. Hidden/ignored when
  `settings.kernel` is false.
- Preset list. Preset fields: `name`, `extPort` (public UDP; auto-suggest
  a free port), `key` (string 1..255, generate button), `masking`
  (`STUN`/`MEDIA`/`AUTO`/`NONE`), `mediaSsrc?`/`mediaPt?`/`mediaClock?`
  (shown for MEDIA), `maxDummy` (0..1024, default 4), `idleTimeout`
  (seconds, default 300), `clientWgLocalPort` (default 13255), `isDefault`
  (exactly one).
- Checkbox **"Block direct WG port from outside"** — default **off**
  (Rule 0). When on: LUCX-managed firewall rule drops external UDP to the
  WG listen port (loopback exempt) while obfuscation is enabled.
- Per-preset status line in the form: process running / listener up,
  log-tail link.

### 3.2 Data model

All in the inbound `settings` JSON, optional key `obfuscation`:

```json
{
  "obfuscation": {
    "enabled": true,
    "blockDirectPort": false,
    "presets": [
      { "id": "p1", "name": "phobos-ru", "extPort": 51822, "key": "…",
        "masking": "STUN", "mediaSsrc": "", "mediaPt": 0, "mediaClock": 30,
        "maxDummy": 4, "idleTimeout": 300, "clientWgLocalPort": 13255,
        "isDefault": true }
    ]
  }
}
```

Client-side (peer settings in the inbound client record): optional
`presetId` (empty = default preset). Peers without a preset are plain WG
clients — their config/export is unchanged.

### 3.3 Runtime

- One process **`wgo-{inboundId}`** per obfuscated inbound; config
  `bin/tunnel/wgo-{id}.conf` (0600) with one INI section per preset:
  `mode = wireguard`, `source-lport = <extPort>`,
  `target = 127.0.0.1:<wgListenPort>`, `key`, `masking`, media params,
  `max-dummy`, `idle-timeout` (ms), `verbose = 1`.
- Lifecycle bound to the kernel WG stack: `internal/lucx/wireguard`
  manager `ensure`/`remove` also Ensure/Stop the wgo process; the
  `kernel_wireguard_job` reconcile loop keeps both in sync
  (fingerprint = rendered config; restart on change — standard pattern).
- Disable obfuscation / delete inbound → stop process, remove conf, remove
  the direct-port firewall rule if it was ours.
- `blockDirectPort`: iptables/nftables rule consistent with the existing
  kernel WG firewall infra (`ensureInput` / `RenderFirewall` owner
  comments), LUCX-managed and swept on disable.

### 3.4 Client export (peer bound to a preset)

In the client export UI (ClientQrModal / ClientInfoModal,
`frontend/src/pages/clients/wireguardConfig.ts`):

1. **`.conf`**: standard WG sections with
   `Endpoint = 127.0.0.1:<preset.clientWgLocalPort>` plus `[instance]`
   section (`source-if = 127.0.0.1`, `source-lport = clientWgLocalPort`,
   `target = <shareHost>:<extPort>`, `key`, `masking`, `verbose = INFO`,
   `idle-timeout`, `max-dummy`, `media-ssrc` when pinned) —
   Phobos-installable, byte-format per §2.2.
2. **`phobos://<base64url(conf)>#<clientName>`** link + QR.
3. For `NONE`/`STUN` presets only: **"ClusterM INI"** download — the same
   `[instance]` content as a standalone INI for upstream ClusterM clients
   (their config format is the same directive set).
4. UI warning: a plain WG app importing this `.conf` will not connect
   without the local obfuscator helper (loopback endpoint) — inherent to
   the scheme.

Plain peers (no preset) keep today's export paths untouched.

### 3.5 Traffic / online

Unchanged: counters stay on the `lxwgN` interface (obfuscated packets
still land in the WG socket via the proxy). No new traffic machinery.
Process status: process probe + log tail (tunnel.Proc ring log).

## 4. Feature B — `wgosocks` inbound (SOCKS5-exit)

New inbound-only protocol stack, pattern of naive/mieru/TrustTunnel.

### 4.1 Settings

```json
{
  "port": 51862,            // public TCP port
  "key": "…",               // obfuscation key (generate button)
  "masking": "MEDIA",       // MEDIA | TLS | AUTO | NONE (TLS only here)
  "mediaSsrc": "", "mediaPt": 0, "mediaClock": 30,
  "maxDummy": 4,
  "idleTimeout": 300,
  "maxClients": 1024,
  "clients": [
    { "email": "user1", "login": "…", "password": "…", "enable": true }
  ]
}
```

- Clients are panel clients; login/password minted on add (random),
  stored plaintext (the engine needs plain pairs in the conf; file 0600).
  Regenerate button per client; removal → rewrite config + restart.
- TCP port cross-check vs other TCP inbounds/panels (naive validation
  pattern).
- Validation caps client count at the engine's `MAX_SOCKS5_USERS` (value
  verified at implementation) with an explicit error message.

### 4.2 Runtime & share

- Process **`wgosocks-{id}`**, conf `bin/tunnel/wgosocks-{id}.conf`:
  `mode = socks5`, `role = server`, `source-lport = <port>`, `key`,
  `masking`, media params, `socks5-users = login:pass,…`. Managed by the
  tunnel Manager pattern (`internal/lucx/tunnel/`), reconcile via
  tunnel job (mieru/TrustTunnel style).
- Share: `phobos://` payload = `[instance]` (`mode = socks5`,
  `role = client`, `source-if`, `source-lport` = 1080 default,
  `target = <shareHost>:<port>`, `key`, `masking`, `verbose`) +
  `[socks5] login/password`. Client runs the local obfuscator exposing
  `socks5://login:password@127.0.0.1:1080`; apps point there.
- Not in JSON/Clash subscriptions. No `routeThroughXray` (MVP).
  Master/local only (needs a host process, like kernel WG).

### 4.3 Traffic / online

- Traffic: `/proc/PID/io` rchar/wchar deltas (tproxy/olcRTC pattern),
  inbound-level. Per-client traffic is **not available** from the engine —
  sole-enabled-client attribution like tproxy (lucx.190); limitation
  documented in operator docs.
- Online: ESTABLISHED connections on the listen port; `activeTags` only
  with live sessions/bytes.

## 5. Engine integration & binary delivery

- Binary name: `wgobfs-linux-{amd64,arm64}` (avoids collision with any
  upstream `wg-obfuscator` package). Built from **pinned SHA** of
  Ground-Zerro/Phobos `src/phobos-obfuscator/` (Makefile) in
  `bin/pack-sidecars.sh`; sha256 pin in `bin/lucx-pins.txt`; GitHub
  tarball ships it; SourceCraft gets it via the Cores upload tab (csqtt /
  openflux pattern). Install/update scripts follow the existing
  pin-check-skip logic.
- **Bump policy: reactive** (lucx.309 precedent) — move the pin only when
  a masking mode breaks upstream or a stable feature appears.
- No code from the Phobos TypeScript panel (AGPL-3.0) is copied,
  referenced, or linked. Config/link formats are re-implemented from the
  public docs (§2). The engine stays a separate GPL-3.0 process — the
  qWDTT / MTProxy precedent.

## 6. Licensing & attribution (maintainer requirement)

- New LucX files: PolyForm Noncommercial 1.0.0 SPDX headers (Rule 10).
- `docs/LICENSING.md`: new entry — `wgobfs` engine = GPL-3.0 external
  binary (source Ground-Zerro/Phobos `src/phobos-obfuscator`, derived from
  ClusterM/wg-obfuscator), shipped as a separate process; LucX integration
  code = PolyForm.
- Fork README: third-party credits block — **ClusterM** (wg-obfuscator,
  GPL-3.0) and **Ground-Zerro** (Phobos fork core, preset model,
  `phobos://` format, client installers). Include upstream repo links.
- Operator doc `docs/content/docs/en/config/wg-obfuscator.mdx` (+ RU if
  the set has it): how it works, client ecosystems (Phobos installers /
  PhobosWG APK / ClusterM upstream clients), which masking needs which
  client, traffic-accounting limitation for `wgosocks`.
- `bin/lucx-pins.txt`: pin + source URL + license line per sidecar
  (extend the existing format).

## 7. Rule 0 / Rule 0b compliance

- **Rule 0**: enabling obfuscation changes nothing for existing peers —
  WG listen port and issued configs stay as-is (direct port open unless
  the operator explicitly ticks "block"). Preset binding is an explicit
  per-client action and only affects that client's export. Disabling
  obfuscation stops proxy processes (preset-bound clients lose
  connectivity — explicit operator action; UI warns). No startup
  migration rewrites anything.
- **Rule 0b (vanilla overlay)**: `obfuscation` is a new optional settings
  key (absent → feature hidden, no processes); `wgosocks` is a new
  Protocol oneof value; neither mutates existing rows/columns. Vanilla DB
  path explicitly tested.

## 8. File map (new / touched)

Backend (PolyForm unless noted):

- `internal/lucx/tunnel/wgo.go` — engine core: binary path/name, INI
  section render + validation, `phobos://` link builder, `[instance]`
  renderer (shared by both features). Golden-tested.
- `internal/lucx/tunnel/wgo_inbound.go` — `wgosocks` inbound: settings
  model, role=server render with `socks5-users`, process via tunnel
  Manager, `/proc/PID/io` traffic, ESTABLISHED online.
- `internal/lucx/wireguard/obfuscator.go` — presets from inbound settings
  → wgo conf; Ensure/Stop wired into `wireguard.Manager` ensure/remove;
  `blockDirectPort` firewall rule (owner-marked, swept).
- `internal/web/job/kernel_wireguard_job.go` — wgo reconcile in the
  kernel WG loop (extend job interfaces).
- `internal/web/service/kernel_wireguard.go` — obfuscation validation
  (ports/masking/keys), UDP port cross-check.
- `internal/web/service/tunnel.go` / `controller/tunnel.go` — `wgosocks`
  inbound service + status/logs endpoints (tunnel pattern); WG-attached
  wgo status/logs on the inbound (`/panel/api/inbounds/:id/wgoStatus`,
  logs tail).
- `internal/database/model/model.go` — Protocol oneof += `wgosocks`
  (LUCX-HOOK).
- `bin/pack-sidecars.sh`, `bin/lucx-pins.txt` — engine build + pins.

Frontend:

- `frontend/src/pages/inbounds/form/protocols/wireguard.tsx` —
  Obfuscation block (LUCX-HOOK extension of existing hook block).
- `frontend/src/pages/inbounds/form/protocols/wgosocks.tsx` +
  `frontend/src/schemas/protocols/inbound/wgosocks.ts` — new protocol
  form/schema; registered in `protocols/index.ts`,
  `schemas/inbound/index.ts`, `primitives/protocol.ts`,
  `InboundFormModal.tsx`, `inbound-defaults.ts` (LUCX-HOOKs).
- `frontend/src/pages/clients/wireguardConfig.ts` — obfuscated export:
  `.conf` + `[instance]`, `phobos://` link, ClusterM INI variant.
- `frontend/src/pages/clients/ClientQrModal.tsx` / `ClientInfoModal.tsx`
  — preset picker per client (client form `presetId`), new export panels.
- i18n keys ×13 locales (`internal/web/translation/`).

Docs:

- `docs/content/docs/en/config/wg-obfuscator.mdx` — operator guide.
- `docs/LICENSING.md`, README credits block, `.agents/02-architecture.md`
  (engine entry), `.agents/08-hooks.md` (new hook files).

## 9. Testing

- Go: golden renders (WG presets conf, wgosocks conf, `[instance]`,
  `phobos://` base64url + `none`-padding rule), validation units (port
  ranges, masking sets, key length, client caps), fingerprint stability,
  process lifecycle fakes, overlay test (vanilla DB / no `obfuscation`
  key → no processes, panel loads), `go test ./internal/lucx/...`
  `-count=1` per Rule 7.
- Frontend: `npm run typecheck && npm run lint`, vitest unit (form
  behavior, export builders), i18n dead-keys key-set test ×13.
- `bin/check-lucx.sh` gofumpt.
- E2E (manual, test stand): Phobos Android APK + Phobos installer client
  against STUN/MEDIA presets and a socks5 preset; upstream ClusterM
  client against a NONE/STUN preset via INI export.

## 10. Verify at implementation (open items)

1. `MAX_SOCKS5_USERS` and `S5_CRED_MAX` values (fork headers) →
   validation caps and error copy.
2. `verbose` accepted forms in `[instance]` (doc example says `INFO`;
   CLI takes 0..4) — pin one form, golden-test it. Same unit check for
   `idle-timeout`: the `[instance]` doc example shows `300` while the
   server INI default is `300000` (ms) — confirm which unit the client
   parser expects before freezing the golden files.
3. Phobos-fork arm64 build path in `pack-sidecars.sh` (Makefile cross
   toolchain or zig cc).
4. Interop: fork-client ↔ fork-server exact MEDIA framing match across
   pin bumps (client APKs bundle their own build) — keep a manual
   regression note in the operator doc.
5. `AUTO` server behavior with preset-bound exports (client picks
   masking) — decide whether the form allows AUTO for WG presets
   (upstream default) or sticks to explicit STUN/MEDIA/NONE.
6. Whether the kernel WG port block uses iptables INPUT (consistent with
   `ensureInput`) or the nftables table — pick the one that survives the
   existing sweep logic.
7. `phobos://` link distribution surface: export modal only (MVP) —
   subscription integration deferred.
