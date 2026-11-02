# Sidecar plugins (MVP)

Project plan / roadmap: [plugin-manager.md](plugin-manager.md).

LucX-UI can install and supervise user-supplied sidecar plugins. This first
release intentionally exposes only process management. Plugins do not receive
panel API access, database access, Telegram bot tokens, or subscription seams.

## Package

Upload a ZIP from **Settings → Cores → Sidecar plugins**. The archive must have
a `manifest.json` at its root:

## Install from a GitHub link

Next to the ZIP upload, **Plugins → Install from link** takes a public GitHub
repository URL: `https://github.com/<owner>/<repo>`, optionally
`/tree/<branch|tag|commit>` to pin a ref. Two steps:

1. the panel validates the repository `manifest.json` (raw.githubusercontent.com
   fetch, the same rules as below) and shows the operator id/name/version/
   entrypoint/permissions plus a trust warning;
2. install happens only after the confirm click: the repository archive
   (≤ 200 MB) goes through the same apiVersion 1 contract as the ZIP install —
   same checks, same 0700 layout, `data/` preserved, plugin not activated.

A bare link re-reads the repository HEAD every time; pin a ref for
reproducible installs, and the install aborts when the manifest changed between
preview and download. Design follows the community plugin system by AllIsA1
(<https://github.com/AllIsA1/lucx-plugins>).

For the ZIP upload the archive must have a `manifest.json` at its root:

```json
{
  "id": "example-sidecar",
  "name": "Example sidecar",
  "version": "1.0.0",
  "apiVersion": 1,
  "type": "sidecar",
  "entrypoint": "bin/example-sidecar",
  "architectures": ["amd64", "arm64"],
  "permissions": {
    "network": [],
    "panelApi": [],
    "bot": []
  }
}
```

`entrypoint` must stay inside the plugin directory. The current host
architecture must be listed. The package is limited to 200 MB and must contain
an executable at the declared path after extraction.

Manifest rules exactly as the panel validates them:

- `id` — `^[a-z0-9][a-z0-9_-]{0,63}$` (lower-case, digits, `-`, `_`; max 64 chars);
- `apiVersion` — must be `1`; `type` — must be `sidecar`;
- `entrypoint` — relative path inside the archive, no `..`, no absolute path;
- `architectures` — must contain the host `GOARCH` (`amd64`, `arm64`, ...);
  empty list = any architecture allowed;
- the manifest `id` must equal the name of the plugin directory — always the
  value in `manifest.json`, nothing else.

Executable bits from the ZIP are not preserved: every extracted file gets
`0700` (owner rwx), so a script entrypoint needs a proper `#!/bin/sh` shebang
and works out of the box regardless of how the ZIP was built.

### Stage 2 enforcement (apiVersion 1)

- `permissions` may contain only `network`, `panelApi`, `bot`, up to 16 short
  entries each. `panelApi`/`bot` must stay **empty** — apiVersion 1 grants no
  panel API or bot seams; a non-empty list is refused at install and at start.
- `network` accepts only the value `"none"`: the plugin starts inside its own
  network namespace with **no network access** (Linux, the panel's privileges
  — typically root). An empty `network` list or an absent `permissions` keeps
  today's behavior: the plugin may use the network. Any other value is refused.
- `resources.maxMemoryMB` (16..131072, or absent) — RLIMIT_AS for the plugin
  process (Linux only): the panel execs the entrypoint through a
  `/bin/sh -c 'ulimit -v N && exec "$0"'` wrapper, so the cap is virtual
  memory in KB = MB×1024. Go binaries reserve large virtual arenas — declare
  the cap only if the process survives it.
- `health` — optional process probe, reported in the status:
  `{ "type": "http" | "tcp", "target": "http://127.0.0.1:8080/healthz" or
  "host:port", "intervalSeconds": 5..3600 (default 30), "timeoutSeconds":
  1..60 (default 5) }`. Probes never restart the plugin. A probe is rejected
  together with network isolation (an isolated plugin is unreachable).
- `resources.maxCPUPercent` (1..100, or absent) — CPU cgroup cap
  (systemd `CPUQuota`) for the plugin. Available **only** on the unit runner;
  a manifest that declares it is refused at start when the unit runner is
  unavailable, so the cap is never silently skipped.
- **Runner** (from lucx.338 / stage 3a): plugins that ask for enforcement
  start as transient systemd units (`lucx-plugin-<id>`, started with
  `systemd-run`: `PrivateNetwork=yes` for `network: ["none"]`, `MemoryMax`
  and `LimitAS` for the memory cap, `CPUQuota` for the CPU cap, empty
  `CapabilityBoundingSet`, `ProtectSystem=strict`, `ProtectHome=yes`,
  `PrivateTmp=yes`, `RestrictNamespaces=yes`, `NoNewPrivileges`;
  `KillSignal=SIGINT` and `TimeoutStopSec=3` preserve the Stop contract).
  The status reports the runner (`"unit"` or `"process"`) and which caps are
  actually enforced. When the host prevents the panel from starting units
  (no system systemd manager, or the panel unit blocks it), a plugin that
  asks **only** for netns/memory cap falls back to the child process runner
  (which enforces or refuses fail-closed as before), while a plugin that
  asks for a CPU cap is refused: units are the only way to cap CPU. Plugin
  units **outlive a panel restart** (documented behavior change since
  lucx.338: a started plugin keeps running when the panel restarts; delete
  and uninstall stop the unit first).
- Fail closed: on a host without enforcement (non-Linux, or no privileges on
  Linux) a manifest that asks for isolation or limits is refused at start —
  the panel refuses to run it unsandboxed silently.

## Panel API seam (stage 3c, apiVersion 1)

A manifest may declare `panelApi` as a list of grants, each
`<resource>:<kind>` with kind `read`, `write` or `*`:

```json
"permissions": {"panelApi": ["inbounds:read", "clients:write", "server:*"]}
```

Resources mirror the panel API route groups and are closed vocabulary:
`clients`, `inbounds`, `hosts`, `server`, `awg-outbounds`, `sidecar-outbounds`
and `tunnel`. GET/HEAD calls are `read`; everything else is `write`. Unknown
resources, kinds or duplicates are refused at install.

When the plugin is started the panel grants it a scoped bearer credential
(persisted per plugin, survives panel restarts and reinstall-free re-starts)
and hands over two env vars:

- `LUCX_PANEL_TOKEN` — the plugin's bearer credential;
- `LUCX_PANEL_URL` — the loopback base URL of the panel API
  (`https://127.0.0.1:2053/e2etestpath/`; when the panel speaks TLS the URL
  is https and the plugin must dial that one loopback connection with
  certificate verification off — the operator's cert is typically
  self-signed).

The plugin calls `LUCX_PANEL_URL` + `panel/api/...` with
`Authorization: Bearer $LUCX_PANEL_TOKEN`. The panel enforces the matrix on
every call: a call passes only if the resolved route's resource and verb
kind are granted. **Never reachable, whatever the permission**: everything
under `/plugins/` (the plugin system itself), `/setting/`, `/xray/`,
`/nodes/`, `/backuptotgbot`, and these server routes: `importDB`, `getDb`,
`updatePanel`, `getMigration`, `restartPanel`, `getNewXray`,
`getNewXrayGeodata`, `getNewXrayCert`, `getNewEchCert`, `getNewPanelCert`,
`getNewMrzCert`, `downloadXray`, `restartXrayService`, `restartXray`,
`stopXrayService`, `installPanel`, `updateConfig`, `logs/:type`,
`checkPanelLoginSecurity`. A plugin with `panelApi` must not also declare
`network: ["none"]` — an isolated namespace has no route to the panel; such
manifests are refused.

## Bot token seam (stage 3c, apiVersion 1)

`permissions.bot` names the bots whose panel-stored credentials the plugin
may receive: `telegram` or `discord` (closed vocabulary). The tokens are
delivered as env vars at start — `LUCX_TELEGRAM_BOT_TOKEN` /
`LUCX_DISCORD_BOT_TOKEN`; a bot that is not configured in the panel
settings is simply not delivered. Rotation: change the token in panel
settings and restart the plugin. A bot plugin needs real network access
(never combine with `network: ["none"]`).

## Sub/after seam (stage 3d, apiVersion 1)

`permissions.sub: ["after"]` registers the plugin as a subscription
post-processor. For every raw `/sub/:subId` body the panel POSTs the
subscription context to `/_lucx/sub/after` on the plugin socket with

```json
{ "subId": "...", "host": "...", "count": 3, "emails": ["user@example.com"] }
```

and the plugin answers `{"links": ["vless://...#extra", ...]}`. Contract:

- links are appended **after** every client entry — existing entries are
  never rewritten (rules 0/0b);
- expired/depleted info-node responses stay pure, and the injections never
  reach the sub info page, the json/clash/awg bodies or the panel link
  displays — the raw links body only;
- the panel allows 300 ms per call, drops empty or > 2 KiB lines, and caps
  at 32 links per plugin and 64 links per body; any plugin error means its
  links are simply absent (fail-open, one line in `plugin.log` on the first
  failure);
- `sub` cannot be combined with `network: ["none"]` (the panel would lose
  the route to the plugin socket) — refused at install.

Reference plugin: `docs/examples/plugins/sub-extender/`.

## Events (stage 3b, apiVersion 1)

A manifest may declare `events` — up to 16 names from the panel's event
vocabulary, e.g. `"events": ["cpu.high", "xray.crash"]`. Only names with a
real publisher are valid today: `outbound.down`, `outbound.up`, `xray.crash`,
`node.down`, `node.up`, `cpu.high`, `memory.high`, `login.attempt`
(`client.*` and `inbound.*` arrive with later stages, when panel publishers
exist). Unknown or duplicated names are refused at install.

Plugins that declare events receive:

- the unix socket path in `LUCX_PLUGIN_SOCKET` (bind rules above);
- a `POST` to `/_lucx/events` with a JSON body
  `{ "id", "type", "source", "time", "data" }` for every matching event,
  serialized one at a time (publication order per plugin). Answer any 2xx
  quickly; the panel waits 5 s and does **not retry** — delivery is best
  effort, and a stopped or hung plugin simply loses events and counts
  `failed`.
- `node.*` events only fire when node monitoring is in use; `events`
  requiring the socket work under both runners.

Payload `data` is the raw event payload with Go field names: a string for
`xray.crash`, `{"Percent","Threshold"}` for `cpu.high`/`memory.high`,
`{"Username","IP","Time","Status","Reason"}` for `login.attempt`,
`{"Delay","Error"}` for outbound events and `{"NodeId","LatencyMs",
"CpuPct","MemPct","XrayState","XrayError"}` for `node.*`. Payloads carry
identifiers, never keys or passwords.

The status reports `events: {delivered, failed, socket}` while the plugin
runs (counters reset on panel restart; the subscription re-arms on the next
status poll after a restart). A working example lives in
[docs/examples/plugins/event-logger](examples/plugins/event-logger).

## Runtime contract

The plugin runs as a separate process with its working directory set to its
installation directory (`bin/plugins/<id>/`). LucX-UI provides:

- `LUCX_PLUGIN_ID`;
- `LUCX_PLUGIN_VERSION`;
- `LUCX_PLUGIN_DATA`, a private writable data directory
  (`bin/plugins/<id>/data`) — keep all mutable state here;
- `LUCX_PLUGIN_SOCKET` — set only when the manifest declares `events` (see
  below): the unix socket the plugin must `unlink`, `bind` and `chmod 0600`
  itself. Under the unit runner it lives in the systemd runtime directory
  `/run/x-ui-plug-<id>/` (created and cleaned by systemd for the unit);
  under the child process runner it is `<id>/plugin.sock`.

Everything the process writes to stdout/stderr goes to
`bin/plugins/<id>/plugin.log` (append mode, one log for all runs).

Lifecycle: the panel sends `SIGINT` on Stop, waits 3 seconds, then
`SIGKILL`. Handle `SIGINT`/`SIGTERM` and shut down graceful if you can.
There is no auto-restart: if the process exits with a non-zero code it stays
down and the error shows in the plugin's status ("LastError").

## Minimal plugin example (no toolchain)

A working plugin can be a single shell script. Layout of the ZIP:

```
manifest.json
run.sh
```

`manifest.json`:

```json
{
  "id": "hello-sidecar",
  "name": "Hello sidecar",
  "version": "0.1.0",
  "apiVersion": 1,
  "type": "sidecar",
  "entrypoint": "run.sh",
  "architectures": []
}
```

`run.sh`:

```sh
#!/bin/sh
# A sidecar plugin: read the LucX env, log to stdout, keep state in ${LUCX_PLUGIN_DATA}.
echo "hello-sidecar ${LUCX_PLUGIN_VERSION} starting, data=${LUCX_PLUGIN_DATA}"
mkdir -p "${LUCX_PLUGIN_DATA}"
echo "$(date -Is) run" >> "${LUCX_PLUGIN_DATA}/visits"
trap 'echo "stopping"; exit 0' INT TERM
while :; do sleep 5; done
```

Package it: `zip hello-sidecar.zip manifest.json run.sh` (on Windows any ZIP
tool works — execute bits are normalized on install). Then Cores → Sidecar
plugins → install → start, and open Logs to verify the hello line.

For a real sidecar, compile a static Go binary the same way the project builds
its sidecars (`CGO_ENABLED=0`, static musl binary for Linux), put it under
`bin/` inside the archive and point `entrypoint` at it. List both
architectures you build for, or leave `architectures` empty for
architecture-independent script plugins.

## What a plugin may and may not do

May (works today):

- run as its own supervised process (start/stop/restart from the panel);
- read `LUCX_PLUGIN_*` env vars and write inside `LUCX_PLUGIN_DATA`;
- write logs (stdout/stderr → `plugin.log`);
- open network connections and listen on ports it chooses — you are
  responsible for firewall rules and port conflicts;
- store files under its own installation directory.

May not (enforced in Stage 2):

- call the panel API or database — no credentials are passed, and a manifest
  declaring `panelApi`/`bot` permissions is refused at install/start;
- use the network while `permissions.network` is `["none"]` (own network
  namespace, Linux);
- exceed `resources.maxMemoryMB` (RLIMIT_AS, Linux);
- read another plugin's directory or data;
- get Telegram bot tokens or inject into subscriptions / client exports;
- survive panel restarts automatically — starting a plugin remains a manual
  panel action.

`manifest.permissions` is declared at package time and enforced on start:
apiVersion 1 understands only the empty set and `network: ["none"]`; anything
else is refused, so future versions can extend the vocabulary without a
back-compat break.

This plugin system does not sandbox arbitrary native code beyond the Stage 2
process-level enforcement (network namespace + memory limit on Linux). Install
only packages you trust; the process-level contract is described above and the
status API reports what is enforced and what probes say. The next plugin
iteration adds the authenticated panel seams (API, tokens, subscriptions)
before plugins can replace bots or read panel data.

## REST API (session-authenticated, `/panel/api/plugins`)

| Method | Path                                        | Purpose                                                        |
| ------ | ------------------------------------------- | -------------------------------------------------------------- |
| GET    | `/list`                                     | all installed plugins with status                              |
| GET    | `/:id/status`                               | manifest, running, pid, paths, last error, enforcement, health |
| GET    | `/:id/logs?lines=200`                       | tail of `plugin.log`                                           |
| POST   | `/install`                                  | multipart field `file` = ZIP                                   |
| POST   | `/install/url/preview`                      | JSON `{url}` — validate the linked repo, install nothing       |
| POST   | `/install/url`                              | JSON `{url}` — download that repo and install (same contract)  |
| POST   | `/:id/start` / `/:id/stop` / `/:id/restart` | lifecycle                                                      |
| POST   | `/:id/delete`                               | stop + remove directory                                        |

## Pre-upload checklist

- [ ] `manifest.json` at the ZIP root, all required fields present;
- [ ] `id` matches the regex and equals the future directory name;
- [ ] `apiVersion` is `1`, `type` is `sidecar`;
- [ ] `entrypoint` is relative, exists, no `..`;
- [ ] `architectures` lists your target arch (or is empty for scripts);
- [ ] the entrypoint starts and stays up when run manually with
      `LUCX_PLUGIN_ID`, `LUCX_PLUGIN_VERSION`, `LUCX_PLUGIN_DATA` set;
- [ ] the process writes enough to stdout for post-mortem via Logs;
- [ ] `permissions`: `panelApi`/`bot` empty; `network` is `["none"]` only if
      the plugin truly needs no network (Linux-only isolation);
- [ ] `resources.maxMemoryMB` set to a realistic cap, if any (Linux-only);
      `resources.maxCPUPercent` (1..100) when the plugin must share the host
      politely (needs the systemd unit runner);
- [ ] `health` probe target reachable from the panel and NOT combined with
      `network: ["none"]`;
- [ ] package ≤ 200 MB.

## AI prompts for plugin development

Copy-paste prompts to generate and review a plugin with an AI assistant live
in [plugin-prompt.md](plugin-prompt.md) — they track the current
apiVersion 1 contract (Stage 2 enforcement + Stage 3 seams).

## Feedback

Plugin manager MVP shipped in `3.11.0-lucx.333` and is in alpha: capabilities
are still being added. Report issues at
<https://github.com/AlexeyLCP/lucx-ui/issues>.
