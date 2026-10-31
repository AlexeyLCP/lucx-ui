# Sidecar plugins (MVP)

LucX-UI can install and supervise user-supplied sidecar plugins. This first
release intentionally exposes only process management. Plugins do not receive
panel API access, database access, Telegram bot tokens, or subscription seams.

## Package

Upload a ZIP from **Settings → Cores → Sidecar plugins**. The archive must have
a `manifest.json` at its root:

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

## Runtime contract

The plugin runs as a separate process with its working directory set to its
installation directory (`bin/plugins/<id>/`). LucX-UI provides:

- `LUCX_PLUGIN_ID`;
- `LUCX_PLUGIN_VERSION`;
- `LUCX_PLUGIN_DATA`, a private writable data directory
  (`bin/plugins/<id>/data`) — keep all mutable state here.

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

May not (not granted in this MVP):

- call the panel API or database — no credentials are passed;
- read another plugin's directory or data;
- get Telegram bot tokens or inject into subscriptions / client exports;
- exceed any resource limits — none are enforced;
- survive panel restarts automatically — starting a plugin remains a manual
  panel action.

`manifest.permissions` (`network`, `panelApi`, `bot`) is declared only — the
MVP does not enforce it yet; it exists so today's plugins declare intent for
later versions.

This MVP does not sandbox arbitrary native code. Install only packages you
trust. The next plugin iteration will add explicit permission enforcement,
health checks, resource limits, and authenticated panel seams before plugins
can replace bots or read panel data.

## REST API (session-authenticated, `/panel/api/plugins`)

| Method | Path | Purpose |
|---|---|---|
| GET | `/list` | all installed plugins with status |
| GET | `/:id/status` | manifest, running, pid, paths, last error |
| GET | `/:id/logs?lines=200` | tail of `plugin.log` |
| POST | `/install` | multipart field `file` = ZIP |
| POST | `/:id/start` / `/:id/stop` / `/:id/restart` | lifecycle |
| POST | `/:id/delete` | stop + remove directory |

## Pre-upload checklist

- [ ] `manifest.json` at the ZIP root, all required fields present;
- [ ] `id` matches the regex and equals the future directory name;
- [ ] `apiVersion` is `1`, `type` is `sidecar`;
- [ ] `entrypoint` is relative, exists, no `..`;
- [ ] `architectures` lists your target arch (or is empty for scripts);
- [ ] the entrypoint starts and stays up when run manually with
      `LUCX_PLUGIN_ID`, `LUCX_PLUGIN_VERSION`, `LUCX_PLUGIN_DATA` set;
- [ ] the process writes enough to stdout for post-mortem via Logs;
- [ ] package ≤ 200 MB.

## AI prompts for plugin development

Copy-paste these when using an AI assistant (ChatGPT, Devin, ...) to generate
a plugin. Prompt 1 generates from scratch, prompt 2 reviews existing code.

**Prompt 1 — generate a plugin (RU):**

```text
Ты пишешь плагин для панели LucX-UI (это форк 3x-ui) в формате sidecar.
Жёсткие правила плагина:
- ZIP с manifest.json в корне: поля id (regex ^[a-z0-9][a-z0-9_-]{0,63}$),
  name, version, apiVersion: 1, type: "sidecar", entrypoint (относительный
  путь, без "..", не абсолютный), architectures: [] для скриптов или список
  GOARCH ("amd64","arm64") для бинарников, permissions: {"network":[],"panelApi":[],"bot":[]}.
- Плагин — отдельный процесс. Панель даёт лишь три переменные окружения:
  LUCX_PLUGIN_ID, LUCX_PLUGIN_VERSION и LUCX_PLUGIN_DATA (приватная папка
  для записи состояния — всё mutable state только там).
- Доступа к API панели, базе данных, токенам бота и подпискам НЕТ — не
  используй их, не пытайся читать файлы панели.
- Логи — обычный stdout/stderr, они падают в plugin.log: пиши осмысленные
  строки с таймстампами.
- По Stop панель присылает SIGINT и через 3 секунды SIGKILL: обработай
  SIGINT/SIGTERM и завершайся чисто.
- Авто-рестарта нет: процесс должен сам жить (цикл/сервис), при падении
  остаётся выключенным.
Моя задача: <ОПИШИ ЗАДАЧУ ПЛАГИНА: что слушает/делает, какие порты, что
хранит>.
Выдай: (1) manifest.json, (2) код плагина (shell-скрипт со shebang ИЛИ
статичный Go-бинарник + команды сборки CGO_ENABLED=0 GOOS=linux,
goos по arch), (3) скрипт сборки ZIP (zip -r plugin.zip manifest.json ...),
(4) как проверить локально: запустить с выставленными LUCX_PLUGIN_* и
убедиться, что в лог пишется startup-строка, а SIGINT завершает чисто.
```

**Prompt 1 — generate a plugin (EN):**

```text
You are writing a sidecar plugin for the LucX-UI panel (a 3x-ui fork).
Hard rules:
- ZIP with manifest.json at the root: id (regex ^[a-z0-9][a-z0-9_-]{0,63}$),
  name, version, apiVersion: 1, type: "sidecar", entrypoint (relative path,
  no "..", not absolute), architectures: [] for scripts or a GOARCH list
  ("amd64","arm64") for binaries, permissions: {"network":[],"panelApi":[],"bot":[]}.
- The plugin is a separate process. The panel provides only three env vars:
  LUCX_PLUGIN_ID, LUCX_PLUGIN_VERSION, and LUCX_PLUGIN_DATA (a private
  writable data directory — all mutable state goes there and only there).
- There is NO access to the panel API, database, bot tokens or subscriptions —
  do not use them, do not try to read panel files.
- Logging = plain stdout/stderr, it lands in plugin.log: write meaningful
  timestamped lines.
- On Stop the panel sends SIGINT and after 3 seconds SIGKILL: handle
  SIGINT/SIGTERM and exit cleanly.
- No auto-restart: the process must stay alive by itself (loop/service); if
  it crashes it stays down until started manually.
My goal: <DESCRIBE THE PLUGIN TASK: what it serves/does, which ports, what
it stores>.
Deliver: (1) manifest.json, (2) plugin code (a shell script with shebang OR a
static Go binary + build commands with CGO_ENABLED=0 GOOS=linux, per-arch
build), (3) a ZIP packaging script (zip -r plugin.zip manifest.json ...),
(4) local verification steps: run with LUCX_PLUGIN_* set, confirm a startup
line appears in the log and SIGINT exits cleanly.
```

**Prompt 2 — review an existing plugin (EN, works for RU code too):**

```text
Review this plugin against the LucX-UI sidecar contract and list violations
or risks:
1. manifest.json validity: id regex, apiVersion=1, type=sidecar, entrypoint
   relative + no "..", architectures vs the binary build targets;
2. runtime contract: only LUCX_PLUGIN_DATA is writable; no panel API/DB use;
   SIGINT/SIGTERM handled; endless-crash loops absent; stdout logging present;
3. safety: no hardcoded secrets left in code, no writes outside its own
   directory, no listening port collisions with the panel/Xray ports;
4. packaging: ZIP layout (manifest.json at root), package size ≤ 200 MB.
Return a short verdict: shippable / needs fixes, with the exact fixes.
Plugin sources: <attach code or ZIP listing>
```

## Feedback

Plugin manager MVP shipped in `3.11.0-lucx.333` and is in alpha: capabilities
are still being added. Report issues at
<https://github.com/AlexeyLCP/lucx-ui/issues>.