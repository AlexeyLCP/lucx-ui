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

### 0. Client config is sacred — STRICT (lucx.106+)

**Закон:** обновление панели (`x-ui update` / веб-update / первый старт после апгрейда) **НЕ ДОЛЖНО** ломать уже работающие inbound’ы и выданные клиентские конфиги.

**Никогда** не менять данные, которые попадают в пользовательский клиентский конфиг, без **явного** действия/запроса оператора **на этом сервере**.

Сюда входят (неполный список):
- tunnel **Address / AllowedIPs** (AWG, WireGuard) — смена IP = клиент обязан перекачать .conf
- **ключи** (private/public/PSK), secrets, UUID, password
- endpoint/host/port, obfuscation must-match поля (S/H/HPK), если уже выданы клиенту
- enable/disable peers, peer routes, всё что ломает handshake/трафик без клика оператора

**Строго запрещено (даже «только сломанным» / «идемпотентно» / «на благо»):**
- startup- / InitDB- / migrate-on-boot, которые **переписывают** peer/client адреса, ключи или settings на живой БД
- auto-fix / re-allocate / «repair» IP при старте панели или reconcile (уроки lucx.91→92, lucx.105→106)
- «тихий» re-allocate при Update/save inbound/client «заодно»
- любая миграция в релизе, которая на чужих прод-серверах меняет выданные клиентам параметры без opt-in

**Разрешено:**
- выделить **новый** IP/ключи **только** если поле **пустое** (новый клиент / новый attach с очищенным AllowedIPs)
- менять IP при **явной** смене Address inbound **кнопкой оператора** в UI (и это задокументировано)
- **читать** per-inbound IP для export/QR (не мутировать)
- schema/backfill **без** смены значений (добавить ключ с default, не трогая существующие)

**Сломанный peer на сервере:** чинить **только** по запросу оператора или opt-in кнопка/SQL — **не** автоматом на всех хостах при update.

**Перед merge/релизом агент обязан спросить себя:** «Если тестер сделает `x-ui update` на живой панели с сотней клиентов — кто-то из них потеряет интернет или должен перекачать .conf без своего желания?» Если да — **нельзя** мержить.

### 0b. Vanilla 3x-ui overlay is sacred — STRICT (lucx.119+)

**Закон:** установка LucX-UI **поверх** живой БД ванильной 3x-ui (`/etc/x-ui/x-ui.db` без переустановки, `x-ui update` / install поверх) **ОБЯЗАНА** поднимать панель и показывать существующих клиентов/инбаунды без краша.

Типичный путь тестера: стоит MHSanaei/3x-ui → ставит LucX → открывает Clients / Inbounds. Любой регресс здесь = «форк не ставится».

**Перед merge/релизом агент обязан спросить себя:** «Если взять свежую SQLite от upstream 3.6.x с WireGuard-клиентами и `wg_keep_alive INTEGER` и запустить наш бинарник — Clients page и Xray start работают?» Если нет — **нельзя** мержить.

**Строго запрещено:**
- Менять тип Go-поля, которое мапится на **существующую** upstream-колонку (`clients.*`, `inbounds.*`, …), **без** `sql.Scanner` / `driver.Valuer` (или эквивалента), принимающего **легаси-форму** драйвера. Урок lucx.119: `KeepAlive int` → `KeepAliveValue string` без `Scan` → `unsupported Scan, storing driver.Value type int64 into type *model.KeepAliveValue` на каждом `Find(&[]ClientRecord)` — панель «Что-то пошло не так».
- Считать, что GORM AutoMigrate «сам расширит» INTEGER→TEXT на SQLite. На SQLite affinity per-value: колонка **остаётся** INTEGER, драйвер отдаёт `int64` для старых строк. Нужен Scanner, не «type:text в теге».
- Zod/frontend-схемы, которые принимают **только** новый тип (например `keepAlive: z.number()`), если backend после LucX-записи может отдать строку (`"25"` / `"15-25"`) — форма Edit inbound не сохранится. Принимать number|string (preprocess), как `normalizeAwgTimer` / `optionalKeepAlive`.
- Startup-миграции, которые на vanilla-БД **мутируют** клиентские данные (см. Rule 0). Schema widen / backfill defaults / new empty tables — ок.

**Обязательно при смене типа колонки / custom type:**
1. `Scan` (+ `Value` при записи) для **всех** форм, которые отдаёт driver на legacy DB: `nil`, `int64`, `[]byte`, `string`, при необходимости `float64`.
2. Postgres: явный `ALTER … TYPE text USING …` **до** AutoMigrate, если strict type не даст писать новый формат (`migrate_awg_keepalive.go`).
3. Регрессия: unit Scan + (если CGO) `Find` по таблице с INTEGER-колонкой; JSON unmarshal vanilla `{"keepAlive":25}`; frontend parse number **и** string.
4. Не полагаться на «свежая LucX-БД зелёная» — overlay path проверять отдельно.

**Разрешено / fine на overlay:**
- Новые таблицы (`awg_outbounds`) пустыми, новые optional settings-ключи с default.
- Новые значения `Protocol` oneof (awg/naive/…) — не ломают load существующих строк.
- Миграции `protocol='awg'` / `lucxTunnel_*` — no-op на vanilla.

**См. Pattern 1n** (debug): Scan `wg_keep_alive`.

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
- **Go:** `internal/lucx/` — subdirectories: `parser/`, `nodetype/`, `outbound_link/` (Smart Cluster), `tunnel/` (tunnel sidecars: NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel)
- **Go:** `internal/database/migrate_awg.go` — legacy DB migration
- **Go:** `internal/web/service/tunnel.go`, `internal/web/controller/tunnel.go`, `internal/web/job/tunnel_job.go` — tunnel sidecar web layer
- **Frontend:** `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema
- **Frontend:** `frontend/src/pages/inbounds/form/protocols/awg.tsx` — React form
- **Frontend:** `frontend/src/schemas/tunnel.ts`, `frontend/src/api/tunnels.ts`, `frontend/src/pages/tunnels/TunnelsPage.tsx` — tunnel sidecar UI
- **Shell:** `bin/install-awg-module.sh` — DKMS install

Integration points (`model.go`, `db.go`, `web.go`, `runtime/local.go`, `service/xray.go`, `install.sh`, `inbound-defaults.ts`, `InboundFormModal.tsx`, `protocols/index.ts`, `primitives/protocol.ts`, `protocols/inbound/index.ts`, `api.go`, `routes.tsx`, `AppSidebar.tsx`, `queryKeys.ts`, `endpoints.ts`) get LUCX-HOOK blocks only.

### 3. AWG Sidecar Architecture (mirrors mtproto)

AWG runs as a kernel-interface sidecar managed by `internal/awg.Manager`, exactly symmetric with `internal/mtproto.Manager`:

- **Manager** (`internal/awg/manager.go`): singleton with `Ensure`/`Reconcile`/`StopAll`/`CollectTraffic`/`SyncPeers`, fingerprint-based restart on config change, orphan sweep at first call. Reconcile-loop convergence: `ensureXrayRouting` (routeThroughXray: table/rule into tunN, dies with tunN on Xray restart) + `ensureNatRules` (kernel NAT: MASQUERADE/FORWARD, dies on iptables flush — fail2ban/docker).
- **Process** (`internal/awg/process.go`): wraps `awg-quick up/down` (kernel interface lifecycle, not a daemon). No tun2socks — routing is via Xray TUN inbound.
- **Instance** (`internal/awg/instance.go`): desired runtime state + `InstanceFromInbound` + `fingerprint`.
- **Traffic** (`internal/awg/manager.go`, влито из traffic.go): `awg show <iface> transfer` parsing for per-peer byte accounting (replaces mtg's Prometheus HTTP scrape).
- **Diagnostics** (`internal/awg/diagnostics.go`): read-only probe chain (interface UP, ip_forward, peers/handshakes, then mode-specific: MASQUERADE+FORWARD or tunN+rule+table). `Diagnose(inst)` → ordered `DiagCheck`s with evidence details; served by `GET /panel/api/inbounds/:id/awgDiagnostics` and rendered by the AWG form's diagnostics modal. Fixes belong to reconcile — diagnostics only makes failures visible.
- **Platform** (`internal/awg/platform_{linux,other}.go`): `defaultRouteInterface()` for MASQUERADE target + sweep of orphaned awg interfaces from a previous x-ui run.
- **Job** (`internal/web/job/awg_job.go`): cron `@every 10s` — Reconcile desired inbounds + fold inbound/per-client traffic deltas + RefreshLocalOnlineClients (AWG online status comes from fresh handshakes, not Xray stats). **Live speed (lucx.135):** each tick stores its deltas, normalized to the 5s window, in `awg_speed_buffer.go` (sticky snapshot, TTL 20s); `XrayTrafficJob` folds them into its 5s broadcast frame (LUCX-HOOK) so the Clients/Inbounds speed columns cover AWG, which Xray's stats API never sees.
- **Egress** (`internal/web/service/xray.go:injectAwgEgress`): inject TUN inbound into generated Xray config when `routeThroughXray` is set, symmetric with `injectMtprotoEgress`. Per-inbound gateway `10.254.(N%254).1/30` (separate /30 subnet, never conflicts with AWG tunnel subnet). Sniffing `{http,tls,quic, routeOnly:true}` on TUN inbound so domain/geosite rules work for AWG traffic.
- **Runtime** (`internal/web/runtime/local.go`): delegate AWG `AddInbound`/`DelInbound` to `awg.GetManager()`; `AddUser`/`RemoveUser` are no-ops (peer sync via Reconcile).
- **CPS** (`internal/awg/cps/`): CPS packet generators (TLS/DNS/SIP/QUIC) + AWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4). TLS and QUIC have browser-specific fingerprints (Chrome/Firefox/Safari).
- **Signature** (`internal/awg/signature/`): QUIC host capture — sends QUIC Initial to UDP 443, reads replies → I1-I5.
- **Controller** (`internal/web/controller/awg.go`): `generateObfuscation` + `captureHost` + `awgDiagnostics` API endpoints. `generateObfuscation` does **not** auto-set `randomTrailers` (must-match; Amnezia 5.0.1.1 / NekoBox+ drop handshake). Operator opt-in only.
- **NAT** (`internal/awg/platform_{linux,other}.go`): `defaultRouteInterface()` for MASQUERADE target.
- **Inbound needRestart** (`internal/web/service/inbound.go`): `awgRoutesThroughXray` — needRestart on AddInbound/DelInbound/UpdateInbound/SetInboundEnable so Xray regenerates config when routeThroughXray toggles.
- **AWG outbound** (`internal/awg/client_*.go` + `internal/web/{service,controller}/awg_outbound.go`): symmetric sidecar for chaining VPN-of-VPN. Each `awg_outbounds` row = one `awgo-N` kernel interface (client to an upstream AWG server) exposed as a freedom outbound with `sockopt.interface = awgo-N`. Manager: `EnsureClient`/`RemoveClient`/`SweepOrphanClients` (fingerprint-based restart, mirrors inbound Manager). Client .conf via `renderClientConf` (Table=off, no DNS, no I1-I5; HPK only when `AwgVersion == "3"` and non-empty). `ParseConf` eats a .conf of any version (incl. HPK) and auto-detects `AwgVersion` from the field set. Controller uses `RestartXray(true)` on mutations (hot-apply can't add a freedom outbound with sockopt.interface). Address allocation (`client_awg.go`) excludes AWG outbound tunnel IPs to avoid collision.
- **AWG3 / version presets** (`headerProtectionKey` + `awgVersion` fields): upstream `feat/awg3` merged to master on 2026-07-30 (kernel `v3.0.20260731`, tools `v3.0.20260730`), so HPK is now **enabled**. The `awgVersion` field (`"1.5"`/`"2"`/`"3"`) lives on the inbound (server ceiling) and gates HPK emission everywhere — `generateObfuscation` returns it only for `"3"`; `renderServerConf`/`renderClientConf`/`inboundAwgHints` write the `.conf` line only when `AwgVersion == "3"` AND the key is non-empty. Generator guarantees S1–S4 ≥ 12 (`MinSForHPK`). Client export selector (`ClientQrModal`/`ClientInfoModal`) clamps to ≤ ceiling. See Known Issue #5 (CLOSED) and Pattern 6 (version compatibility).
- **AWG3 advanced parameters** (`contentPaddingAddition`, `rekeyAfterTime`, `rekeyTimeout`, `rejectAfterTime`, `keepaliveTimeout`, `maxHandshakeAttempts`): 6 device fields from the upstream kernel UAPI that the panel exposes (lucx.52). All version-gated to `"3"`, all default 0 = kernel uses built-in WireGuard constant (120/5/180/10/18 sec / deterministic WG padding). Device fields written to `[Interface]`. **generateObfuscation DOES auto-generate them for v3 (lucx.65):** `GenerateAwg3DeviceTimings(profile)` in `cps/params.go` (algorithm ported from AmneziaWG-Architect awg3.ts, derived from amneziawg-go v3.0.1) — ContentPadding/RekeyAfter/RejectAfter scale with obfProfile (lite/standard/pro → low/medium/high), RekeyTimeout/KeepaliveTimeout/MaxHandshakeAttempts fixed safe spans (protocol invariants: RejectAfter>Keepalive+RekeyTimeout, RekeyAfter<RejectAfter, attempts≥1); emitted as `"lo-hi"` range strings in the same v3 gate as HPK (`awgVersion=="3" && ModuleSupportsAwg3()`), response keys match the Zod schema so the form's blind `Object.entries.forEach(setValue)` applies them. Migration prunes non-v3 values. `ParseConf` auto-detects v3 from any of these fields. **AdvancedSecurity removed (lucx.62):** the per-peer `advancedSecurity` field was fully deleted from model/DB/forms/.conf — upstream kernel `set_peer` never reads it (`attrs[WGPEER_A_ADVANCED_SECURITY]` unreferenced in `netlink.c:set_peer`), `get_peer` hardcodes "off" in dumps, and it does NOT gate HPK/timers/padding (those are independent device attrs). Migration `migrate_awg_hpk.go` deletes stale `advancedSecurity` keys from stored settings. **Ranges (lucx.60):** each of the 6 timers accepts a single value `"150"` OR an inclusive range `"100-500"` — the kernel's `u16_range_t` (device.h) + tools' `u16_range_from_string` parse both and randomize within the range at rekey (same semantics as H1-H4), verified live on a v3 module. The value is stored as `awg.AwgTimer` (string) end-to-end and written to the .conf VERBATIM (never collapsed). `AwgTimer.UnmarshalJSON` accepts a legacy JSON number too; `IsZero` ("", "0", "0-0") → renderer omits the line. Frontend `normalizeAwgTimer` clamps/orders the range but does NOT collapse it.

### 3b. Tunnel Sidecars Architecture (lucx.91+)

Туннельные сайдкары — внешние туннельные серверы, которые панель супервизит **рядом** с Xray (не Xray-протоколы). Ядра: **NaiveProxy** (Caddy + `forward_proxy` klzgrad, HTTP/2-паддинг; опционально routeThroughXray), **olcRTC** (TCP-over-WebRTC через meet-комнаты Jitsi/Telemost/WB Stream), **qWDTT** (WG over VK TURN). Каркас общий:

- **`internal/lucx/tunnel/`** (PolyForm): `Name`-реестр ядер; `NaiveConfig` + рендер Caddyfile; `Proc` (exec, SIGTERM→kill, ring-лог); `Manager` (singleton, fingerprint-рестарт при смене конфига, `Ensure`/`Stop`/`StopAll`, трёхуровневый статус process→TCP-probe→TLS-probe).
- **Caddyfile-грабли** (выучены elector1337/3x-ui-naive + E2E lucx.91, зашиты в рендер): `admin off` (иначе инстансы дерутся за :2019); wildcard-listen → bare `:port` (явный `0.0.0.0:port` Caddy понимает как host-matcher), конкретный IP → `bind`; per-инстанс `XDG_DATA_HOME` (ACME-хранилища не дерутся); кавычки+экранирование всех пользовательских значений. **Три грабли из E2E:** (1) сабдирективы `padding` у forward_proxy НЕТ — паддинг включается сам по заголовку `Padding` от клиента; (2) домен в адресе сайта ОБЯЗАН нести нестандартный порт (`domain:8443`), иначе bare-домен открывает второй слушатель :443; (3) manual-TLS требует `auto_https off` + `skip_install_trust` в глобальном блоке, иначе Caddy поднимает ACME-слушатель :80 и ставит локальный root-серт в системный trust.
- **Хранение:** settings-таблица, ключ `lucxTunnel_naive` (JSON-блоб). Веб-слой: `service/tunnel.go` (валидации, кросс-проверка порта с TCP-инбаундами — UDP-протоколы не конфликтуют; download бинарника через temp-файл), `controller/tunnel.go` (`/panel/api/tunnel/naive/*`: status/config/start/stop/restart/logs/preview/validate/upload/download/deleteBinary), `job/tunnel_job.go` (cron 10s: reconcile + Naive access_log traffic/online). **lucx.115:** settings-блобы — ТОЛЬКО legacy-контур: миграции ставят маркер `migratedToInbound` (и до-ставят его, если inbound уже есть), reconcile-fallback при маркере принудительно `Enabled=false` + свипит orphan `{core}-{id}` даже при пустом want, а legacy Start/Restart/Save отказывают («manage on the Inbounds page»), если блоб мигрирован или есть inbound протокола; Stop разрешён всегда (убийство зомби). См. Pattern 1m.
- **Naive online/traffic (best-effort):** JSON `access_log` per-instance (`dataDir/access.json`); `Manager.CollectNaiveTraffic` tail + user→email (`ClientAuthForInbound`); grace 120s; `AddTraffic` per-client always, inbound rollup only when !routeThroughXray. Long CONNECT may update only at session end.
- **Per-client креды + подписки:** `tunnel.ClientAuth(panelSecret, email)` — детерминированный HMAC-SHA256, без хранения в БД; каждый включённый клиент панели получает свою `basic_auth`-строку в Caddyfile и свою ссылку `naive+https://user:pass@domain:port#email` в base64-подписке (LUCX-HOOK в `sub/service.go:getSubs`; стандарт NekoBox/husi/Exclave). Disable клиента убирает креды на следующем reconcile. JSON/Clash-подписки naive не получают (форматы не поддерживают протокол). Обфускационного генератора НЕТ по дизайну: камуфляж наива = стек Chrome, паддинг включается сам по заголовку `Padding`.
- **Поставка бинарника:** release.yml — amd64 prebuilt из klzgrad/forwardproxy (pinned тег), arm64 — xcaddy кросс-сборка; прочие архитектуры без бинарника (upload/download в UI). Имя: `bin/caddy-naive-<os>-<arch>`.
- **Ограничение ACME:** Let's Encrypt HTTP-01 требует порт 443 (валидация не даёт ACME на другом порту).
- **Dev-готча:** Kaspersky на dev-машине ломает loopback TLS (MITM) — TLS-пробы в тестах скипаются с пометкой окружения; на Linux работает.
- **Мост в Xray (lucx.93):** опциональный `routeThroughXray` — Caddy dial через нативный `upstream socks5://127.0.0.1:port` (klzgrad/forwardproxy, **без патча бинарника**) + скрытый SOCKS loopback inbound (`injectTunnelEgress`, тег `lucx-tunnel-naive`, симметрично mtproto). Порт аллоцируется backend'ом и стабилен; `outboundTag` опционально force-route. Raw-Caddyfile mode несовместим. Default = прямой egress.
- **olcRTC (lucx.94):** `OlcrtcConfig` → YAML (`mode: srv`, provider/room/crypto/transport/dns/vp8) → бинарник `olcrtc-linux-{arch}` (единственный CLI-арг = путь к YAML). Settings key `lucxTunnel_olcrtc`. Connect URI `olcrtc://provider?transport@room#key`. Probe = process-only (нет listen-порта). Клиенты: owenclave / olcbox. Upstream: openlibrecommunity/olcrtc (WTFPL). **ПИН в release.yml (lucx.132):** собираем из SHA `3339cd36…` (последний master до upstream-мержа OLC2 wire-break), НЕ из `master` — см. Pattern 1o. Снимать пин только когда клиенты (owenclave/olcbox) выпустят OLC2-сборки и прогнан e2e на живой комнате.
- **qWDTT (lucx.95):** `QwdttConfig` → CLI argv (`-listen/-wg-port/-password/-dns/-listen-raw/-config-dir`) → бинарник `qwdtt-linux-{arch}` (GPL-3.0, external process). Settings key `lucxTunnel_qwdtt`. State dir: passwords.json + wg-keys.dat. Share: `qwdtt://config?…`, legacy `wdtt://…`, subscription JSON. **Нужен root/CAP_NET_ADMIN** (TUN + MASQUERADE). Клиент: SpaceNeuroX Android APK. Upstream: SpaceNeuroX/proxy-turn-vk-android server.go.
- **mieru (lucx.117):** inbound-only (`mieru-{id}`), без legacy-блоба. `mita run` + `MITA_CONFIG_JSON_FILE` / `MITA_UDS_PATH` / `MITA_INSECURE_UDS=1`. HMAC-креды `lucx-mieru-*`. Share `mierus://` (порт всегда в query). Трафик: `mita get users` (1-day, compact `1.5MiB` и spaced). routeThroughXray default OFF. `/var/lib/mita/metrics.pb` общий на все инстансы (upstream). **Traffic shaping (lucx.128):** официальный `trafficPattern` (seed/unlockAll/tcpFragment/nonce/padding/lowEntropy) рендерится в mita JSON И в `mierus://?traffic-pattern=` (base64 protobuf, кодер на `protowire` в `mieru_pattern.go`, без зависимости на enfein/mieru; золотой тест = пример из официальных docs). `multiplexing`/`handshakeMode` — клиентские, только в ссылку, в mita JSON не попадают никогда. Padding-поля — указатели (0 = «выключить слот», ≠ «не задано»). Пресеты off/lite/standard/stealth — фронтенд-сахар (`lib/mieru/presets.ts`), в БД хранятся значения, не имя пресета (Rule 0); каждый Apply — новый seed. Все поля опциональны: пусто = omit везде, существующие инбаунды/ссылки не меняются.
- **TrustTunnel (lucx.117/120/122/139):** inbound-only (`trusttunnel-{id}`). `trusttunnel_endpoint vpn.toml hosts.toml`. Серт = панельный ACME (`webCertFile`/`webKeyFile`); нет домена/серта → отказ при save (NotBefore/NotAfter/SAN). Share (подписка/QR) — Throne URI `tt://user:pass@host:port?security=tls&sni=…&alpn=h2#tag` (`ClientURI`); официальный TLV `tt://?` остаётся в `ClientDeepLink` (официальное приложение). Адрес = share-host:port (включая 443). Prometheus — inbound-трафик; per-client трафик/онлайн только если у inbound ровно один включённый клиент (метрик без username нет). `listenPreset` stock|fast (default fast — окна из репорта тестера; stock = дефолты CONFIGURATION.md). `clientRandomPrefix` (hex/mask) → rules.toml allow + TLV `0x0B`; auto-gen on save, Regenerate in Advanced. Remove/sweep deletes `trusttunnel-N*` configs. HMAC `lucx-trusttunnel-*`.
- **AWG 3.1 (lucx.117):** потолок `"3.1"` + `RandomTrailers`/`DisableCookies` (omit при false; эмиссия только 3.1 && тулзы≥3.1). HPK/таймеры — `IsAwg3Plus` (`3` и `3.1`). Живые v3 не бампаются. Гейт тулзов в `install-awg-module.sh`: `< v3.1`. **lucx.136:** (а) early-exit в `install-awg-module.sh` срабатывает только при актуальных тулзах (иначе fallthrough пересобирает тулзы), `rmmod` busy → `.awg-reboot-needed`; (б) `RebuildAwgModule` сначала `StopAll()`+`StopAllClients()` (новый метод), `AwgJob` пропускает reconcile при `AwgRebuildRunning()`; (в) видимость — `ModuleAwg31` в `HostStatus`/hello/`Status.Awg`(`moduleAwg31`,`rebootNeeded`) + тег AWG3.1/«AWG3.1 OK» + инфо-строка `awg31CapabilityCheck` в диагностике.
- **Пресеты обфускации (lucx.136, канон Amnezia; H — lucx.139):** `GenerateAWGParams` перекалиброван под `AwgInstaller::generateAwgParameters` (amnezia-client): Lite/Standard/Pro вокруг Jc=4..6, Jmin=10, Jmax=50, S1/S2=12..149, S3=12..63, **S4=12 фикс**. Инвариант полный (`isPacketSizeEqual`): 148+S1, 92+S2, 64+S3, 32+S4 попарно различны. H1–H4 по версии: «1.5» → одиночные в узких полосах; «2»/«3»/«3.1» → узкие диапазоны шириной ≤100000 (`hBand`). lucx.136 ставил «3»/«3.1» в «1/2/3/4» (HPK шифрует заголовок) — тестеры читали это как сломанный генератор; lucx.139 вернул ranges. Дефолт `mimicryProfile` = `tls`. **lucx.137:** `quicInitialPacket` добивает до 1200 случайными байтами, не `0x00` (DPI-маркер «простыня нулей» в I1).

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

LucX-UI components (`internal/awg/`, `internal/awg/cps/`, `internal/awg/signature/`, `internal/lucx/`, `internal/database/migrate_awg*.go`, `internal/web/controller/awg.go`, `internal/web/controller/awg_outbound.go`, `internal/web/controller/lucx.go`, `internal/web/controller/tunnel.go`, `internal/web/job/awg_job.go`, `internal/web/job/tunnel_job.go`, `internal/web/service/client_awg.go`, `internal/web/service/awg_outbound.go`, `internal/web/service/tunnel.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/schemas/tunnel.ts`, `frontend/src/api/tunnels.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `frontend/src/pages/inbounds/form/protocols/mieru.tsx`, `frontend/src/pages/inbounds/form/protocols/trusttunnel.tsx`, `frontend/src/schemas/protocols/inbound/mieru.ts`, `frontend/src/schemas/protocols/inbound/trusttunnel.ts`, `frontend/src/pages/inbounds/form/awg-inbound-id-context.ts`, `frontend/src/lib/mieru/presets.ts`, `frontend/src/pages/tunnels/TunnelsPage.tsx`, `frontend/src/pages/clients/wireguardConfig.ts`, `bin/install-awg-module.sh`, `bin/check-lucx.sh`, `bin/pre-push`, `bin/build-release.sh`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use (including VPN resale) requires explicit written permission from the author.

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
├── platform_linux.go              defaultRouteInterface() + killStrayAwgInterfaces + ModuleSupportsAwg3 (kallsyms-символ + awg version ≥ 3, кэш только true) + ModuleSupportsAwg31 (тулзы ≥ 3.1) + awg3CapabilityCheck/awg31CapabilityCheck для диагностики
├── platform_other.go              no-ops off Linux
├── client_instance.go             ClientInstance + ClientSettings + ClientInstanceFromOutbound + fingerprint (desired state for awgo-N outbounds)
├── client_conf.go                 renderClientConf — awg-quick .conf for an awgo-N outbound (Table=off, no DNS, no I1-I5; HPK only when AwgVersion=="3" and non-empty)
├── client_manager.go              outbound client manager: EnsureClient/RemoveClient/SweepOrphanClients (fingerprint-based restart)
├── *_test.go                      instance/manager/diagnostics/client_conf/client_instance/client_manager/platform tests

internal/awg/cps/                  CPS packet generators (TLS/DNS/SIP/QUIC) + AWGParams
├── cps.go                         GenerateCPS + tlsPacket (Chrome/Firefox/Safari) + buildChromeHello/buildFirefoxHello/buildSafariHello + DNS/SIP/QUIC packet builders (quicInitialPacket respects browserProfile)
├── domains.go                     MimicryProfile + BrowserProfile + ObfProfile types + domain pools (RU/World)
├── params.go                      GenerateAWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4, канон Amnezia lucx.136) + Awg3DeviceTimings + SetRand/tests + rng
└── cps_test.go                    CPS unit tests (all browsers, invariants, signatures, QUIC browser)

internal/awg/signature/            QUIC host capture (hoaxisr port)
├── capture.go                     Capture(domain) — sends QUIC Initial, reads replies → I1-I5
└── capture_test.go                normalizeDomain/fillPackets/varint/HKDF/ClientHello+Initial structure tests

internal/lucx/                     Smart Cluster + tunnel sidecars
├── parser/                        SSH output → NodeCreds
├── nodetype/                      LucX vs vanilla detection (GET /panel/api/lucx/hello + features gate)
├── outbound_link/                 Inbound → outbound config generator
└── tunnel/                        Tunnel sidecars (Naive, olcRTC, qWDTT, mieru, TrustTunnel)
    ├── tunnel.go                  Name registry + binary/config/data paths
    ├── mieru.go / mieru_inbound.go / mieru_traffic.go / mieru_pattern.go (trafficPattern + proto-кодер ссылки)
    ├── trusttunnel.go / trusttunnel_inbound.go / trusttunnel_traffic.go
    ├── auth.go                    scoped HMAC (naive/mieru/trusttunnel)
    ├── naive.go                   NaiveConfig + Caddyfile render (admin off, bind, access_log, escaping) + client URL
    ├── traffic.go                 CollectNaiveTraffic — access_log tail, per-client deltas, online last-seen
    ├── process.go                 Proc: exec + SIGTERM→kill + ring log (500 lines)
    ├── manager.go                 Manager singleton: Ensure/Stop/StopAll, fingerprint restart, 3-level probe (process→TCP→TLS)
    └── *_test.go                  render/validation/fingerprint/probe/process/traffic tests

internal/database/
├── migrate_awg.go                 pruneLegacyAwgHiddenChildren + stripHiddenKeys
├── migrate_awg_outbound.go        outbound-side migration (stripHiddenKeys for awg_outbounds)
├── migrate_awg_hpk.go             pruneAwgHeaderProtectionKey — clears non-empty HPK (regression from lucx.47; see Known Issue #5)
├── migrate_awg_keepalive.go       Postgres: clients.wg_keep_alive INTEGER/bigint → text before AutoMigrate (SQLite no-op; Scan handles int64)
└── migrate_awg*_test.go           unit tests

internal/web/
├── runtime/local.go               AWG delegation in AddInbound/DelInbound (LUCX-HOOK)
├── job/awg_job.go                 AwgJob cron — Reconcile + CollectTraffic (inbound + per-client + online) + pubkey→email mapping + outbound client reconcile
├── job/awg_speed_buffer.go        sticky snapshot of AWG deltas normalized to the 5s window; XrayTrafficJob merges it into the speed broadcast (lucx.135)
├── service/xray.go                injectAwgEgress (TUN inbound + per-inbound gateway + sniffing) + injectAwgOutbounds (freedom per awgo-N with sockopt.interface) + AWG exclusion + ensureAwgRouting (post-restart route restore) (LUCX-HOOK)
├── service/inbound.go             awgRoutesThroughXray + needRestart (LUCX-HOOK) + inboundAwgHints (pre-renders Jc/S/H/I block + HeaderProtectionKey for client .conf)
├── service/client_awg.go          defaultAwgClients — keypair + PSK + address allocation (excludes AWG outbound tunnel IPs)
├── service/awg_outbound.go        AwgOutboundService — CRUD + parseConf + ActiveOutboundTags/ActiveOutboundAddresses (collision guard)
├── controller/awg.go              generateObfuscation + captureHost + awgDiagnostics API endpoints (LUCX-HOOK). HPK is intentionally NOT emitted (Known Issue #5).
├── controller/client.go           awgBody + subBody (LUCX-HOOK): same-origin subscription bodies (vpn:///.conf/sub/json/clash) — the public sub port has no CORS headers; subBody loops back to the LOCAL sub server, path+query only (no SSRF)
├── controller/awg_outbound.go     AWG outbound CRUD + parseConf + test endpoints; RestartXray(true) on add/del/update/enable (hot-apply can't add freedom with sockopt.interface)
├── controller/lucx.go             GET /panel/api/lucx/hello — LucX identity + features for multi-node deploy gating
├── service/tunnel.go              TunnelService — naive config persist (settings key lucxTunnel_naive), validations, TCP port cross-check vs inbounds, caddy adapt, binary download via temp file
├── controller/tunnel.go           /panel/api/tunnel/naive/* — status/config/start/stop/restart/logs/preview/validate/upload/download/deleteBinary
├── job/tunnel_job.go              TunnelJob cron @every 10s — reconcile + Naive access_log traffic/online
├── web.go                         cadenceAwg + cadenceTunnel + StopAll wiring + upload body-limit exempt (LUCX-HOOK)

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
├── pages/sub/SubPage.tsx                   AMNEZIA row on the public subscription page: .conf / vpn:// downloads + body copy (same-origin; PageData.SubAwgUrl, lucx.135)
├── pages/clients/ClientInfoModal.tsx       AmneziaWG block: per-inbound .conf editor + version select + vpn:// copy (lucx.137)
├── pages/clients/ClientQrModal.tsx         AWG panel with QR + download
├── schemas/protocols/inbound/index.ts      InboundSettingsSchema union (LUCX-HOOK)
├── schemas/primitives/protocol.ts          ProtocolSchema + Protocols map (LUCX-HOOK)
├── pages/inbounds/form/protocols/index.ts  AwgFields export (LUCX-HOOK)
├── schemas/tunnel.ts                       NaiveConfig/NaiveStatus Zod schemas (LUCX)
├── api/tunnels.ts                          tunnel API client, JSON_HEADERS (LUCX)
├── pages/tunnels/TunnelsPage.tsx           Tunnels page: status badge, lifecycle, logs, binary mgmt, simple/raw Caddyfile form (LUCX)
├── routes.tsx + layouts/AppSidebar.tsx     /panel/tunnels route + menu item (LUCX-HOOK)
└── pages/api-docs/endpoints.ts             tunnel endpoints registry (contract test)

bin/install-awg-module.sh          DKMS install (`--force-rebuild`, `--no-kernel-upgrade`, `--uninstall`); kernel auto-upgrade только без `--no-kernel-upgrade`; маркер = SHA коммита; .conf при uninstall не трогает
bin/check-lucx.sh                  gofumpt check for LucX files (49) — run before push; -w autofixes
bin/pre-push                       git hook: check-lucx + fast go tests + PR/issues guard (AGENTS.md 11.5)
install.sh                         AWG module installs by default again (lucx.131 reverted lucx.130 opt-in) — fresh AND over-the-top installs run `bin/install-awg-module.sh`. Over-the-top install keeps login/password/port/webBasePath (detects existing sqlite DB / postgres env). Re-prints panel credentials from /etc/x-ui/install-result.env (LUCX-HOOK, lucx.68)
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

# Full project build (requires frontend/dist) — LINUX/CGO ONLY.
# На Windows без gcc этот `go build` падает в internal/database (sqlite3.Backup,
# CGO) — это ПРЕ-существующее, не регресс, и НЕ гейт. Полный бинарник и все
# тесты авторитетно собираются GitHub Actions (CI + release.yml, ubuntu, CGO).
# Локально гейтиться на gofumpt + точечных `go test` без cgo + frontend-чеках,
# а за полным билдом/тестами — пуш в main → смотреть `gh run list` / CI.
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

- ⚠️ **GCP-прод (`lucx`, 34.88.71.12, GCP Finland) ЗАКРЫТ** (2026-08-03, решение владельца): Google-проекты свёрнуты, сервер больше не цель деплоя. **НЕ стучаться туда, не обновлять, не считать его «недоступным продом, который надо поднять».** Единственный живой сервер проекта — `lucx-test2` (ниже). SSH-алиас `lucx` в `~/.ssh/config` — исторический, не использовать.
- **Target:** `lucx-test2` (144.31.157.106, poor-rose-snake.play2go.cloud) — единственный тестовый/проверочный сервер.
- **Service:** `x-ui.service` (systemd)
- **Procedure:** `x-ui update` на сервере (тянет latest release + новый `update.sh`: SHA-gate пересборки AWG-модуля + kernel-gate) → verify `systemctl status x-ui` + logs. Чистая установка — `install.sh` (см. Release & Install).
- **AWG runtime check:** `awg show` should list active interfaces; `ip link show awgN` for TUN

### Тестовые серверы (SSH alias'ы в `~/.ssh/config`, user `root`, ключ `~/.ssh/id_ed25519`)

| Alias | IP | Хост | Назначение |
|---|---|---|---|
| (нет alias — `root@144.31.224.212`) | 144.31.224.212 | skinny-azure-snail.play2go.cloud | **Единственный тестовый стенд** (с 2026-08-05): install-тесты, веб-обновление, AWG runtime, проверка релизов. Debian 13, key-auth |

- **144.31.157.106 (lucx-test2)** — переустановлен владельцем 2026-08-05, SSH нестабилен (сбросы на kex); **больше не используется** — стенд перенесён на 144.31.224.212 по решению Alexey («другие тестовые забудь»).
- **Testers:** VladufQa, Kirill Rudenko — обновляются сами через `x-ui update` или reinstall; на их панели без запроса не лезем.

---

## Release & Install (форк)

`install.sh` адаптирован под наш форк (`AlexeyLCP/lucx-ui`): скачивает релиз-tarball и raw-скрипты (x-ui.sh, x-ui.rc, service-юниты) из `main`. Xray-core + mtg переиспользуются из апстрим-релиза `MHSanaei/3x-ui`.

### Сборка релиза — только GitHub Actions (НЕ собирать на VPS вручную)

Все сборки делает `.github/workflows/release.yml` (ubuntu-latest, CGO через
Bootlin musl-toolchain → статический бинарник, Node из `.nvmrc` (=24, Vite 8
не собирается на Node 20). Ручные сборки на VPS (`bin/build-release.sh`) —
legacy, не использовать: расходятся с CI по xray/mtg-версиям.

```bash
# 0. ОБЯЗАТЕЛЬНО до тега — незарелизенные коммиты (урок lucx.133):
#    тег цепляет ВЕСЬ main, но notes легко описывают только последний фикс
#    и теряют фичи/фиксы, которые уехали в main без своего тега.
git fetch gh --tags
git log --oneline v3.6.0-lucx.$((N-1))..HEAD
# В notes и progress.md — КАЖДЫЙ не-docs коммит из этого списка, не только lucx.N.
# Пустой список кроме docs-хвоста прошлого релиза — ок.

# 1. Дождаться зелёного CI на main, затем поставить тег — Release-workflow
#    сам соберёт tarball и опубликует stable-релиз:
git tag v3.6.0-lucx.N && git push gh v3.6.0-lucx.N
gh run watch --repo AlexeyLCP/lucx-ui          # Release LucX-UI
gh release view v3.6.0-lucx.N --repo AlexeyLCP/lucx-ui   # asset x-ui-linux-amd64.tar.gz

# 2. ОБЯЗАТЕЛЬНО: body релиза (release notes) — то, что видит оператор
#    в панели при обновлении (getPanelUpdateInfo → releaseNotes).
#    upload-release-action часто оставляет body пустым → сразу дописать:
gh release edit v3.6.0-lucx.N --repo AlexeyLCP/lucx-ui --notes-file - <<'EOF'
## v3.6.0-lucx.N

- пункт 1 (что изменилось для пользователя)
- пункт 2
- …
EOF
# Источник: запись lucx.N в progress.md (сжать до 5–15 буллетов, по-русски
# или EN+RU). Без notes оператор видит пустой «Что нового» или сырой compare.

# 3. Установить/обновить панель на VPS:
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
# или на уже установленной: x-ui update (консоль) / кнопка в веб-панели
```

### Release notes — закон (обязательно)

**Каждый stable-тег `v3.6.0-lucx.N` ДОЛЖЕН иметь непустой GitHub Release body.**

- Панель показывает `release.Body` в модалке обновления (`PanelUpdateInfo.releaseNotes`).
- Пустой body → fallback `fetchCompareNotes` (сырые commit subjects) — плохо для тестеров.
- Писать **после** успешного Release workflow (`gh release edit … --notes` / `--notes-file`).
- Параллельно: запись в `progress.md` (подробно, для агентов) + notes (кратко для UI/тестеров).
- Если забыли notes на уже опубликованном теге — дописать сразу (`gh release edit`), не ждать следующего релиза.

#### Стиль notes (как пишем для людей)

**Язык: RU + EN.** Сначала блок на русском, затем `---` и тот же смысл на английском (короче можно, смысл тот же). Панель и GitHub читают и RU, и EN операторы.

Тон: живой разговорный, как от админа к своим. Без корпоративщины, канцелярита и воды. Коротко, по делу, можно с лёгким юмором. Техническое — просто, без лекций. EN — тот же тон (casual), не «We are pleased to announce».

Оформление:
- В начале эмодзи-заголовок (`🔈` / `🆕` / `⚠️` / `🔧` / `✅` и т.п.)
- Короткие абзацы, много воздуха
- Списки с `1️⃣2️⃣3️⃣` или буллетами `🔘` `✅` `❎`
- Разделители `---` между блоками (и между RU/EN)
- Важное — **жирным**
- Команды / пути / протоколы — в `` `code` ``
- В конце RU: `⚡️ Приятного использования!` · EN: `⚡️ Enjoy!`

Структура (каждый язык):
1. Что случилось + зачем (1–2 предложения)
2. Что изменилось (списком)
3. Что делать пользователю (если нужно)
4. Короткий финал

Нельзя: длинные вступления; «Мы рады сообщить» / «We are pleased to announce»; стена текста без эмодзи; овер-формальность; дамп diff/commit hash; только RU или только EN.

**Перед тегом агент обязан:** `git log --oneline <последний-stable-тег>..HEAD` и включить в notes все наши незарелизенные изменения (не только тему текущего коммита). Иначе коммит вроде lucx.132+ (плейсхолдер outbound) «потеряется» — в панели его не будет в «Что нового». Docs-only хвост прошлого релиза можно не дублировать.

Тег ставится **только после зелёного CI на main** (урок lucx.48). `lucxVersion`
в `internal/config/config.go` обязан совпадать с суффиксом тега — guard в
release.yml роняет сборку при расхождении. Push в main без тега обновляет
rolling pre-release `dev-latest` (Dev-канал панели); `releases/latest` при
этом остаётся на последнем stable-теге.

### Что делает `x-ui update` (lucx.58+)
1. Ставит новый бинарник/фронтенд, останавливает панель.
2. **Авто-апгрейд ядра** до последнего packaged (Debian/Ubuntu meta-package) — только если AWG уже установлен (внутри `install-awg-module.sh`).
3. AWG-gate (только если модуль уже ставили: маркер / `amneziawg` loaded / `awg-quick` в PATH). Иначе skip — `x-ui install-awg`. Маркер `/etc/x-ui/.awg-module-version` vs `git ls-remote refs/heads/master`; расхождение → `--force-rebuild`.
4. Старт панели, migrate, fail2ban; если установлено новое ядро — **ребут через 10с** (AWG-модуль уже собран для нового ядра; панель поднимает systemd).

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
- `amnezia-vpn/amneziawg-linux-kernel-module`: PR #192 «feat: AmneziaWG 3.0» слит в master 30.07.2026T21:54Z, теги **`v3.0.20260730`**/**`v3.0.20260731`**(+ -02…-04). `header_protection.c` есть только начиная с этих тегов; в `v1.0.20260611`/`v1.0.20260725` его НЕТ. ⚠️ ядро отвергает HPK с `-EINVAL`, если любое из S1–S4 < 12.
- ⚠️ **Нумерация версий модуля НЕ отражает версию протокола (lucx.58):** upstream штампует `PACKAGE_VERSION="1.0.0"` (src/dkms.conf) и `WIREGUARD_VERSION=1.0.0` (src/Makefile) в **каждом** релизе — модуль, собранный из v3-тега, сообщает modinfo/dkms ту же «1.0.0», что и v1-модуль. Единственный надёжный признак AWG3 — функциональный probe: символ `awg_header_protection_set_key` в `/proc/kallsyms` (ядро) + `awg version` ≥ v3 (тулзы). См. Pattern 1j.
- `amnezia-vpn/amneziawg-tools`: PR #60 слит 30.07.2026, тег **`v3.0.20260730`**. `HeaderProtectionKey` парсится в `.conf` (`config.c`, `parse_key`); `awg version` печатает `amneziawg-tools v3.0.20260730 - https://amnezia.org` (fallback из src/version.h, когда git-describe не сработал).
- ⚠️ Сборка модуля v3.0 падала на ядрах < 6.7 (`nla_put_uint`) — фикс уже в master, так что текущая сборка из master встаёт и на старые ядра; авто-апгрейд ядра в `bin/install-awg-module.sh` (lucx.58) снимает проблему системно.

**Что сделано в lucx.50:**
1. `generateObfuscation` (`controller/awg.go`) снова отдаёт `headerProtectionKey` — но **только при `awgVersion == "3"`** в запросе. Для v1.5/v2 поле отсутствует в ответе (не `""`), чтобы `regenerateObfuscation` (`Object.entries(obf).forEach(setValue)`) не затёр ручное значение оператора.
2. Рендереры `renderServerConf` (`manager.go`), `renderClientConf` (`client_conf.go`), `inboundAwgHints` (`inbound.go`) пишут HPK **только при `awgVersion == "3"` И непустом ключе**. Для v1/v2 строка опускается — старые ядра продолжают работать.
3. Генератор `GenerateAWGParams` (`cps/params.go`) теперь **гарантирует S1–S4 ≥ 12** (`MinSForHPK = 12`, `enforceSMin`) для всех профилей — конфиг валиден для AWG3 независимо от того, установлен ли HPK. `GenerateHeaderProtectionKey()` + `AWGParams.WithHeaderProtectionKey()` генерируют ключ (crypto/rand, 32 байта, base64).
4. Новое поле `awgVersion` (`"1.5"`/`"2"`/`"3"`) во всём пайплайне — на инбаунде (потолок сервера) и в клиентском экспорте (≤ потолка, runtime-селектор в `ClientQrModal`/`ClientInfoModal`).
5. Миграция переименована: `pruneAwgHeaderProtectionKey` → `migrateAwgVersion` (`migrate_awg_hpk.go`). Теперь backfill'ит `awgVersion:"2"` на pre-lucx.50 инбаундах/аутбаундах И вычищает непустой HPK с всего, что не v3 (фикс регрессии lucx.47 для пострадавших + защита от будущего bump'а версии).

**Урок (сохраняется):** «Regenerate obfuscation» молча пишет в форму всё, что вернул backend. Любое поле без поддержки в текущем ядре → краш reconcile. Решение — version-gate эмиссию, а не полное умолчание: поле отдаётся/пишется только когда версия явно его поддерживает.

### 6. geo-файлы перетираются при обновлении панели — upstream-поведение, НЕ чиним (решение 2026-08-09)

**Суть:** `release.yml` пакует в tarball свежие сток-geo; `update.sh` распаковывает поверх → **любое** обновление панели сбрасывает эти имена в сток. Симптом (Aleksandr SacredX, lucx.88): кастомные группы geosite исчезли после веб-обновления → Xray не стартует (routing не находит группы в geo.dat). **Решение:** `update.sh` не трогаем (паритет с upstream). Совет операторам: кастомные группы держать в файлах с **отдельным именем** — tarball их не тронет; либо восстанавливать кроном после update.

**Сток с lucx.99 (8 файлов):** `geoip/geosite.dat` (Loyalsoldier), `_IR` (chocolate4u), `_RU` (runetfreedom), **`_ROSCOM`** (hydraponique/roscomvpn-{geoip,geosite} — RKN geoblock / category-ru / category-ads / youtube/telegram/steam). ROSCOM — отдельное имя, не перетирает чужие кастомы. Обновление: панель Version → Geofiles / `x-ui` меню → RoscomVPN / `update-all-geofiles`. В routing: `ext:geosite_ROSCOM.dat:category-geoblock-ru` и т.п. (пресеты в `constants.ts`). Geodata browser подхватывает любой `*.dat` в `bin/`.

**Важно для диагностики «/awg/ не работает»:** sub-эндпоинты (`/sub/`, `/json/`, `/clash/`, `/awg/`) слушают sub-сервис на **отдельном порту** (дефолт 2096), не на порту панели — в reverse proxy их надо проксировать на sub-порт.

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
- **Cause:** `awg-quick` не установлен или kernel module не загружен. С lucx.131 модуль снова ставится при `install.sh` по умолчанию (lucx.130 был opt-in, отменён решением владельца); хосты, поставленные в окне lucx.130–131 без модуля, остаются без него до ручной установки.
- **Fix:** `x-ui install-awg` / Settings → Cores → Install / `bash /usr/local/x-ui/bin/install-awg-module.sh`. Откат: `x-ui uninstall-awg` (`.conf` остаются). Проверить `awg show`, `ip link show awgN`.
- **Кнопка Cores «не ставит»:** панель гоняет скрипт с `--no-kernel-upgrade` + `DEBIAN_FRONTEND=noninteractive` (иначе apt/needrestart висит без TTY). Смотри логи `awg: rebuild | …`; статус `rebuildRunning` крутится, пока идёт сборка. После успеха reboot обычно не нужен.

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
- **Fix (lucx.53, probe переписан в lucx.58):** `ModuleSupportsAwg3()` (platform_{linux,other}.go). Все 4 рендерера (renderServerConf, renderClientConf, inboundAwgHints + transitively sub/service.go через Prune AWG3 fields) double-gated: `AwgVersion == "3" && ModuleSupportsAwg3()`. Тесты override через `SetModuleSupportsAwg3(&bool)`. **lucx.53-реализация (major=="3" из `modinfo -F version`) была сломана в принципе:** upstream штампует `PACKAGE_VERSION="1.0.0"` в dkms.conf/Makefile КАЖДОГО релиза, так что и v1-, и v3-модуль сообщают «1.0.0» — gate не срабатывал никогда, HPK молча дропался на всех хостах. **lucx.58:** функциональный probe — символ `awg_header_protection_set_key` в `/proc/kallsyms` (ядро) + мажорная версия из `awg version` ≥ 3 (тулзы < v3 не парсят строку HPK в .conf). Кэш только положительного результата. См. Pattern 1j.
- **Урок:** DB-stored `awgVersion` — это потолок, который оператор выбрал, а не capabilities runtime. Нужна explicit capability check в каждой точке эмиссии AWG3 полей — и проверять надо **фичу** (символ/поведение), а не строку версии.

### Pattern 1e: «коннект есть, трафика нет» — два AWG-инбаунда на одной подсети (kernel route конфликт) — ИСПРАВЛЕНО (lucx.54)
- **Cause:** Дефолт формы `createDefaultAwgInboundSettings` хардкодит `address: '10.8.0.1/24'` для каждого нового AWG-инбаунда. Два подряд созданных инбаунда получают **идентичную client-подсеть 10.8.0.0/24** → kernel устанавливает две connected-route на один префикс (`10.8.0.0/24 dev awg2` + `10.8.0.0/24 dev awg4`). Linux выбирает одну по метрике/порядку, вторая zombie. Reverse-path от сервера к клиентам второго инбаунда уходит в preferred-интерфейс (awg2), где peer с этим pubkey не зарегистрирован → пакеты dropнуты → клиенты на awg4 видят handshake (UDP port input не зависит от route), но не получают data-трафика.
- **Симптом (tester VladufQa, 2026-08-03):** «коннект идет не трафик не идет». awg2 + awg4 оба UP, `ip rule iif awg4 lookup 1004` + `default dev tun4 table 1004` на месте (TUN-routing здоров), MASQUERADE/FORWARD на месте. Но reverse-path в kernel route 10.8.0.0/24 → awg2 (zombie для awg4).
- **Fix (lucx.54):** Advisory warning в AWG-форме (`AwgFields`) — `watch('settings.address')` + `useMemo` поверх masked подсетей других AWG-инбаундов (`otherAwgSubnets` prop, проброшен из `InboundFormModal` через `dbInbounds`). Чистые функции `maskSubnet`/`subnetsOverlap` в `frontend/src/lib/awg/subnet.ts` (IPv4, 32-bit int, без npm-зависимостей). `<Alert type="warning">` (advisory, НЕ блокирует save — back-compat для существующих dup-subnet инбаундов).
- **Не делаем:** server-side guard / отказ в сохранении dup-subnet (намеренно — ломает back-compat; оператор может сознательно хотеть пересечение).
- **Дополнено (lucx.56):** auto-suggest следующей свободной /24 — `suggestFreeAwgAddress(usedSubnets)` в `frontend/src/lib/awg/subnet.ts`, подставляется в `settings.address` при выборе протокола AWG в mode add (`InboundFormModal` эффект смены протокола). Новый AWG-инбаунд сразу получает свободную подсеть (10.8.N.0/24, при занятии всего /16 — 10.9+). Защита в двух слоях: auto-suggest предотвращает при создании, warning ловит ручной ввод.
- **Дополнено (lucx.63) — server-side блок:** `checkAwgSubnetConflict(newAddr, ignoreId)` в `inbound.go` парсит адрес → `netip.Prefix.Masked()` и через `Overlaps()` сверяет со всеми AWG-инбаундами. `AddInbound` блокирует новый дубликат; `UpdateInbound` блокирует только **смену** подсети на конфликтную (сравнение masked old vs new) — редактирование существующего dup-инбаунда без смены подсети разрешено (back-compat). Frontend advisory-warning оставлен как мгновенная обратная связь. Третий слой защиты: auto-suggest (предотвращает) → warning (ловит ручной ввод) → server-блок (последний рубеж).
- **Дополнено (lucx.64) — уход от 10.8.0.0/24 + конфликт с outbound'ами:** (а) Дефолт формы и `defaultAwgBase` сменены с **10.8.0.0/24 → 10.200.0.0/24**; auto-suggest сканирует 10.200.0.0/24..10.220.255.0/24. Причина: 10.8.0.0/24 — самая популярная подсеть upstream WireGuard/AmneziaWG-серверов, и AWG-**outbound'ы** (awgo-N, .conf от провайдера) почти всегда получают адрес там же → две connected-route на один префикс → тот же «handshake ok, no traffic» (Vlad: 10.8.5.1/24 работает, 10.8.0.1/24 — «проклятый»). (б) `checkAwgSubnetConflict` теперь сверяется и с **AWG-outbound'ами** (`outboundAddresses(false)` — все, включая выключенные) через чистый хелпер `awgOutboundSubnetConflict`: блок только при `oP.Bits() <= newNet.Bits() && Overlaps` (/32 exempt — не создаёт /24 connected-route, его IP уже закрывает exclusion в `defaultAwgClients`). Существующие инбаунды на 10.8.0.0/24, созданные до lucx.64, НЕ лечатся автоматически — оператор меняет адрес вручную (триггерит `migrateAwgClientSubnets`).
- **Урок:** Дефолт формы для network-address полей должен быть **уникальным per-inbound**, либо форма должна предупреждать о конфликте. Connected-route конфликт — не падает на awg-quick up (интерфейс поднимается), а молча ломает reverse-path → диагностика видит «всё UP», но трафика нет. Дефолтную подсеть надо выбирать подальше от диапазонов, которые используют **внешние** серверы (upstream WireGuard 10.6/10.7/10.8), — иначе AWG-outbound'ы, подключающиеся к ним, занимают ту же подсеть.

### Pattern 1f: Per-client peer-level поле на model.Client не сохраняется — ИСПРАВЛЕНО (lucx.54)
- **Cause:** Фронтенд отправляет `clientPayload.advancedSecurity = true`, но backend `model.Client` struct **не имел** поля `AdvancedSecurity` → `json.Unmarshal` молча дропает неизвестный ключ (стандартное поведение Go) → в Settings JSON поле не записывается → после reload switch OFF. `ClientRecord` (gorm-таблица `clients`) тоже не имел колонки, и `ToRecord`/`ToClient` её не копировали.
- **Симптом (tester VladufQa, 2026-08-03):** «я включаю ползунок, жму сохранить, а он выключается». AWG3 AdvancedSecurity switch toggles ON, после save возвращается в OFF.
- **Fix (lucx.54):** Поле добавлено в **5 точках** в `model.go` для full round-trip: `Client` struct (json tag) + `ClientRecord` struct (gorm column `awg_advanced_security`) + `ToRecord()` (copy) + `ToClient()` (copy) + merge logic. AutoMigrate добавит колонку на следующем старте.
- **Fix (lucx.61):** Merge-логика «true wins, never silently clear» (`incoming.AdvancedSecurity && !existing.AdvancedSecurity`) не давала **выключить** переключатель — условие ложно при `incoming=false`, поэтому ON→OFF невозможен. Для `bool` zero-value `false` — валидное значение (выключить), а не «поле отсутствует» (как для `int`/`string` где 0/"" = absent). Фикс: `if incoming.AdvancedSecurity != existing.AdvancedSecurity` — берёт incoming напрямую. Плюс: AdvancedSecurity **больше не эмитится в .conf** (серверный, outbound-клиентский и пользовательский) — ядро игнорирует поле на input (`set_peer`) и хардкодит "off" в dumps (`get_peer`), эмиссия только ломала парсинг в старых клиентских приложениях. Поле остается в model/DB для будущего kernel-саппорта. Убрано из fingerprint (изменение не триггерит лишний restart).
- **Урок:** Per-client peer-level поле на `model.Client` требует **5 точек** для full round-trip, НЕ одной struct-field. `ClientRecord` — gorm-таблица `clients`, используется во **всех** путях сохранения (`db.go`, `client_crud.go`, `client_link.go`, `client_portable.go`, `client_bulk.go`, `client_traffic.go`). Только `Client` struct недостаточно — поле потеряется на Client→Record→DB→Record→Client цикле. AWG sidecar читает поле через `InstanceFromInbound` (сырой JSON settings), но универсальный `ClientFormModal` save-flow идёт через `controller/client.go` → `model.Client` → `ToRecord` → `ClientRecord`. В contrast, 6 device-полей (ContentPaddingAddition и др.) **не** страдают этим — они живут на `inbound.settings` (инбаунд-level), а не в per-client `model.Client`.

### Pattern 1g: «создаёшь клиента → интерфейс падает, удаляешь → поднимается» — пустой PSK в renderServerConf — ИСПРАВЛЕНО (lucx.55)
- **Cause:** `renderServerConf` (`internal/awg/manager.go`) писал `PresharedKey = %s` **безусловно**, даже при пустом PSK → строка `PresharedKey = ` с пустым значением. awg-tools отвергают её: `awg setconf` → `Line unrecognized: 'PresharedKey='` + `Configuration parsing error` → awg-quick откатывает интерфейс → «Device awgN does not exist». Пустой PSK приходит когда клиент создаётся путём, не вызывающим `defaultAwgClients` (генератор PSK): тот вызывается только из `addInboundClient` (Clients page → `ClientService.Create`), а путь формы инбаунда (`InboundService.UpdateInbound`) — нет. Воспроизведено на test2: пустой PSK → setconf EXIT=1, omit → EXIT=0.
- **Несовпадение, выдавшее баг:** `renderClientConf` (client_conf.go:96) и `SyncPeers` (manager.go:659) уже omit'ят пустой PSK (`if psk != ""`); только `renderServerConf` писал всегда.
- **Fix (lucx.55):** `renderServerConf` omit'ит пустой/whitespace-only PSK (`if psk := strings.TrimSpace(p.PSK); psk != ""`), по образцу renderClientConf/SyncPeers. Отсутствующий `PresharedKey` — WireGuard-конвенция «no PSK». 3 regression-теста в `server_conf_psk_test.go`.
- **Урок:** Рендерер опционального поля `.conf` обязан **omit'ить пустое значение**, а не писать `Key = ` — awg-tools отвергают пустые значения, awg-quick откатывает интерфейс, reconcile бесконечно падает. Любой peer-level параметр, пустой на каком-либо пути создания клиента, должен быть gated `if value != ""`. Проверять все три рендерера (renderServerConf / renderClientConf / SyncPeers) на консистентность.

### Pattern 1h: смена адреса AWG-инбаунда не мигрирует AllowedIPs клиентов → интерфейс падает — ИСПРАВЛЕНО (lucx.59 миграция + lucx.63 база аллокации)
- **Cause:** клиент инбаунда получает AllowedIPs из подсети инбаунда при создании (`defaultAwgClients`). Когда оператор **меняет адрес инбаунда** (напр. 10.8.0.1/24 → 10.8.2.1/24, чтобы уйти от dup-subnet Pattern 1e), AllowedIPs существующих клиентов **остаются на старой подсети** (10.8.0.2/32). При `awg-quick up` awg-quick добавляет route для каждого peer AllowedIPs: `ip -4 route add 10.8.0.2/32 dev awg6`. Если старая подсеть занята другим инбаундом (awg7 на 10.8.0.0/24) → `RTNETLINK answers: File exists` → awg-quick откатывает → «Device awgN does not exist». Воспроизведено по логам ВладufQa (lucx.56).
- **Симптом:** после смены Address в логах `reconcile failed ... ip -4 route add <старый IP> dev awgN` + `RTNETLINK: File exists` + `ip link delete`.
- **Fix (lucx.59):** `migrateAwgClientSubnets(oldAddr, newAddr, settings)` в `client_awg.go`, вызывается из `UpdateInbound` при смене адреса — ре-аллоцирует single-host клиентов из старой подсети в новую (ключи/PSK/email сохраняются), кастомные записи (0.0.0.0/0, чужая подсеть, IPv6) не трогает.
- **Дополнено (lucx.63) — корень «клиент с чужим IP»:** `defaultAwgClients` считал базу аллокации через `wireguardAllocationBase(used, fallback)` — та берёт **/24 первого занятого IP** из `used` (включая IP awgo-* outbound'ов из collision-guard'а). При активном AWG-outbound'е на 10.8.0.x первый клиент инбаунда 15.11.5.0/24 получал 10.8.0.x → маршрут не там → тот же RTNETLINK-конфликт, но **уже при создании**, а не при смене адреса. Плюс `allocateWireguardAddress` расширял пул до /16 при заполнении /24 (выдача адресов вне подсети инбаунда). **Фикс:** база = `awgAllocationFallback(serverAddr)` (только подсеть инбаунда); `allocateWireguardAddress(used, base, widen)` — AWG передаёт `widen=false` (заполненный /24 → ошибка, не выход в соседние). «Нужна перезагрузка Xray чтобы инбаунд ожил» (ВладufQa) — следствие: route-конфликт → reconcile падал бесконечно, RestartXray просто триггерил немедленный Reconcile.
- **Урок:** client-адреса, выделенные из подсети инбаунда, привязаны к этой подсети на момент создания. Смена подсети инбаунда инвалидирует их — нужен migration. И база аллокации должна браться ИЗ ПОДСЕТИ ИНБАУНДА, а не из первого занятого IP в exclusion-списке (exclusion-list ≠ source-of-truth для базы).

### Pattern 1i: удаление инбаунда оставляло .conf + кэш ModuleSupportsAwg3 на ошибке — ИСПРАВЛЕНО (lucx.57)
- **Cause 1:** `Manager.Remove(id)` удалял `awg{id}.conf` только при запущенном интерфейсе (запись в `m.procs`). Инбаунд, чей интерфейс не поднялся (упал setconf/route), не имел записи → `.conf` переживал удаление инбаунда (вопрос ВладufQa «почему не удаляет конфиги»). Конфиги в `/etc/amnezia/amneziawg/` (awgConfigDir), НЕ в `/etc/awg/`.
- **Cause 2:** `ModuleSupportsAwg3` взводил `moduleAwg3Checked=true` ДО `modinfo`; транзиентная ошибка modinfo (модуль пересобирается при update) кэшировала «не v3» на весь процесс → AWG3-поля молча дропались до рестарта.
- **Fix (lucx.57):** `Remove` удаляет `.conf` безусловно; `Reconcile` добавляет `sweepOrphanInboundConfigs(want)` (чистит `awg{N}.conf` нежеланных id без записи в procs; `parseInboundConfName` не матчит `awgo-*.conf`); `ModuleSupportsAwg3` не кэширует при `err != nil` (повторяет probe).
- **Урок:** cleanup, привязанный к «запущенным» сущностям, пропускает именно те, что не запустились (они и оставляют мусор). Побочные файлы удалять по id безусловно.

### Pattern 1j: «модуль v1.x / не v3» по версии — ложь; version-gate пересборки никогда не работал — ИСПРАВЛЕНО (lucx.58)
- **Cause 1 (детект AWG3):** `ModuleSupportsAwg3` парсил major из `modinfo -F version amneziawg` и ждал «3». Upstream штампует `PACKAGE_VERSION="1.0.0"` (dkms.conf) и `WIREGUARD_VERSION=1.0.0` (Makefile) в каждом релизе — и v1-теги (v1.0.20260611), и v3-теги (v3.0.20260731) дают модуль «1.0.0». Gate не срабатывал никогда → HPK не эмиссился НИ на одном хосте (симптом ВладufQa «в конфиге так и не появилась HeaderProtectionKey», даже если модуль пересобран из master).
- **Cause 2 (gate пересборки в update.sh):** сравнение `grep -oP 'version\s*"\K[^"]+' src/dkms.conf` с маркером. В dkms.conf переменная UPPERCASE (`PACKAGE_VERSION=`) → grep матчил пусто → «up to date» → модуль **ни разу не пересобирался** за всё время существования gate (lucx.51+). У ВладufQa модуль остался июньским (pre-HPK) после всех обновлений.
- **Fix (lucx.58):**
  1. `ModuleSupportsAwg3` = функциональный probe: символ `awg_header_protection_set_key` в `/proc/kallsyms` (есть только у AWG3-модуля, и только когда он загружен) + `awg version` major ≥ 3 (тулзы < v3 не парсят HPK-строку в .conf). Кэш только true; false = транзиентно → retry каждый вызов (апгрейд подхватывается без рестарта панели).
  2. Маркер `/etc/x-ui/.awg-module-version` = **SHA коммита** сборки; gate в update.sh = `git ls-remote refs/heads/master` (без клона). Legacy-маркеры ≠ SHA → одноразовая пересборка на всех хостах при первом lucx.58-обновлении.
  3. **Авто-апгрейд ядра** (linux-image/headers meta-package) при каждом вызове install-awg-module.sh; update.sh ребутит в новое ядро в конце обновления (AWG-модуль уже собран и для него).
  4. Build-first-safe: новый DKMS-tree компилируется при загруженном старом; swap только после успешной сборки. Модуль собирается для ВСЕХ установленных ядер с headers. Тулзы пересобираются при `awg version` < v3.
- **Верификация (test2, 2026-08-03):** ядро 6.12.90→6.12.100; старый модуль (маркер 1.0.20260611, тулзы v1.0.20260618-2) → `--force-rebuild` → модуль v3.0.20260731-04 (kallsyms-символ на месте), тулзы v3.0.20260730, маркер = SHA master; reconcile пересоздал awg3/awgo-* за один тик; старый lucx.49-бинарник + v2-конфиг работает на v3-модуле (back-compat), формат `awg show dump` не изменился.
- **Нюанс горячего swap'а:** при запущенной панели rmmod может не сработать (интерфейсы держат модуль) — тогда новый модуль подхватится после ребута/рестарта панели. В штатном update.sh панель остановлена до AWG-хука, так что там swap чистый.
- **Урок 1:** версии внешних компонентов (dkms/modinfo) часто константны между мажорными релизами протокола. Хочешь знать capability — probe'и фичу (символ ядра, поведение бинарника), не строку.
- **Урок 2:** shell-gate по содержимому чужих файлов проверяй end-to-end на реальном файле: grep, не сматчивший UPPERCASE-переменную, молча отключил gate на 3 релиза. Для «актуальности сборки» сравнивай SHA коммитов — строки версий обманывают.

### Pattern 1k: orphan-sweep сносил ЧУЖИЕ AWG-конфиги (WGDashboard) — ИСПРАВЛЕНО (lucx.67)
- **Cause:** `sweepOrphanInboundConfigs` (reconcile, каждые 10с) удалял **любой** `awg{N}.conf` в `/etc/amnezia/amneziawg/` чей ID не среди текущих инбаундов LucX-UI. Конфиги WGDashboard называются так же (`awg0.conf`…) и лежат в той же папке → сносятся как «сироты»; восстановленный из бэкапа файл исчезает снова через ≤10с (симптом тестера: «интерфейсы WGDashboard магически загнулись, из бекапа не поднимаются»).
- **Fix (lucx.67) — ownership по маркеру + бэкап вместо удаления:**
  1. `renderServerConf` пишет первую строку-маркер `# Managed by x-ui - do not edit` (awg-quick игнорирует `#`-комментарии). `awgConfigDir` стал `var` (тесты переопределяют), `awgBackupDir()` = `awgConfigDir + "/x-ui-backup"`.
  2. `sweepOrphanInboundConfigs`: конфиг не из `want` сносится только если несёт маркер (LucX-UI), и **переносится** в `x-ui-backup/awg{N}.conf.<unixtime>`, не удаляется. Чужие (без маркера) **не трогаются**. Хелперы `configIsManaged`, `backupConfigFile`.
  3. `Remove(id)` и reconcile-цикл (разонравившиеся procs) тоже **бэкапят** вместо удаления; удаление только если бэкап не удался.
  4. Пометка существующих конфигов LucX-UI (созданы до фикса): при reconcile для инбаундов из `want` чей конфиг без маркера — перезапись через `renderServerConf` (контент детерминированный, fingerprint не меняется → рестарт не триггерится).
- **Урок:** подкаталог `/etc/amnezia/amneziawg/` общий с другими инструментами (WGDashboard). «Сиротский» sweep по шаблону имени `awg{N}.conf` обязан различать СВОИ конфигы (маркер ownership), а не считать всё без владельца своим; удаление лучше заменять переносом в бэкап.

### Pattern 1l: «коннект есть, трафика нет» + клиент со старым IP при переприсоединении к AWG — ИСПРАВЛЕНО (lucx.91)
- **Cause:** `defaultAwgClients` заполняет только ПУСТЫЕ креды («existing values are never overwritten»). Клиент, отсоединённый от AWG-инбаунда и присоединённый заново — после смены подсети инбаунда или с другого AWG-инбаунда — тащил старый single-host адрес из строки clients-таблицы. Цепочка: awg-quick ставит peer-маршрут /32 на чужую подсеть → либо RTNETLINK-конфликт и интерфейс откатывается («в статусе запускается… завис»), либо peer поднимается, но сервер не владеет подсетью адреса → рукопожатие проходит (ключи совпадают), трафик умирает. Репорт VladufQa + Aleksandr SacredX на lucx.85–90.
- **Fix (lucx.91), 3 слоя:**
  1. `awgAllowedIPsStale` (client_awg.go): все записи single-host (/32 или /128) И хотя бы одна вне текущей подсети инбаунда → stale. Кастомные (0.0.0.0/0, не-host) не трогаются никогда. Ре-аллокация прямо в `defaultAwgClients` при attach — ключи/PSK сохраняются, ротируется только адрес. **Остался активен в lucx.92** — срабатывает только на явном (ре)аттаче клиента, это операторное действие.
  2. Стартовая миграция `migrateAwgStaleClients` (internal/database, в LUCX-HOOK db.go): чинила УЖЕ сохранённых протухших клиентов без ручного переприсоединения. **ОТКЛЮЧЕНА в lucx.92** (`awgStaleMigrationEnabled = false`), **удалена в lucx.123**: в lucx.91 она отработала автоматически на живых серверах при старте панели — смена адресов без спроса оператора признана ошибкой, хотя и трогала только уже сломанных клиентов. Год кода под `const false` — это код, который никто не читает и никто не тестирует; если понадобится, он лежит в истории (`git show v3.6.0-lucx.122 -- internal/database/migrate_awg_stale_clients.go`). Откат для серверов, успевших выполнить lucx.91: `journalctl -u x-ui | grep 'migration.*address'` печатает `client "email" address OLD -> NEW` по каждому ротированному клиенту; вернуть старый адрес можно вручную через allowedIPs в карточке клиента. Обычно откат не нужен — эти клиенты были нерабочими до ротации.
  3. После ротации адреса клиент обязан перескачать конфиг (панель/подписка) — старый конфиг в приложении остаётся с протухшим адресом.
- **Откат для уже мигрировавших на lucx.91:** бэкап не сохранялся, единственная запись old→new — журнал панели: `journalctl -u x-ui | grep 'migration.*address'` (строки `client "email" address OLD -> NEW`). Откат нужен редко: ротированные клиенты до миграции не работали, новый адрес валиден — достаточно перескачать конфиг. Вернуть старый адрес — вручную в allowedIPs клиента.
- **Урок:** «не перезаписывать существующее» безопасно для ключей, но НЕ для адресов, привязанных к подсети инбаунда: адрес, выданный из подсети, протухает вместе с ней. Любой re-attach обязан сверять single-host адрес с ТЕКУЩЕЙ подсетью. **И главный урок lucx.92:** миграции данных на старте панели НЕЛЬЗА делать автоматическими — на живых серверах любое изменение только opt-in (явное действие оператора); автоматом можно только идемпотентные no-op на свежей БД.

### Pattern 1n: overlay 3x-ui → Clients «Что-то пошло не так» / Scan wg_keep_alive int64 — ИСПРАВЛЕНО (lucx.119)
- **Cause:** LucX сменил `Client.KeepAlive` / `ClientRecord.KeepAlive` с `int` на `KeepAliveValue` (string) ради AWG3-диапазонов (`"15-25"`). Upstream-колонка `clients.wg_keep_alive` остаётся INTEGER; sqlite-driver отдаёт `int64`. Без `sql.Scanner` database/sql: `unsupported Scan, storing driver.Value type int64 into type *model.KeepAliveValue` на `Find(&[]ClientRecord)` (Clients page, ListForInbound → Xray config).
- **Симптом:** после install/update поверх ванильной 3x-ui красный экран «Получить (sql: Scan error on column … wg_keep_alive …)». Свежая LucX-БД (TEXT) — ок; ломается только overlay.
- **Fix (lucx.119):** `KeepAliveValue.Scan`/`Value` (`model.go`) — int64/string/[]byte/float64/nil; Postgres `migrateClientKeepAliveColumnType` (`migrate_awg_keepalive.go`) widen → text до AutoMigrate; frontend WG inbound `optionalKeepAlive` принимает number|string (после LucX write-back `"25"`).
- **Урок:** смена типа поля на колонке, которую создал upstream = Rule 0b. AutoMigrate + `type:text` на SQLite **не** переписывает affinity старых строк. Тестируй legacy INTEGER path, не только fresh DB. См. Rule 0b.

### Pattern 1m: tunnel-sidecar живёт после удаления inbound («сыпит логами, хотя удалил») — ИСПРАВЛЕНО (lucx.115)
- **Cause:** двойной источник правды у tunnel-ядер. Кроме inbound'ов (`olcrtc-{id}` / `naive-{id}`) жив legacy-контур: settings-блоб `lucxTunnel_{naive,olcrtc,qwdtt}` + карточка Tunnels-страницы (Start/Stop/Save) + ключ менеджера без префикса (`olcrtc`). `reconcile{Naive,Olcrtc}Inbounds` при ОТСУТСТВИИ inbound'ов падал в fallback на блоб и `Ensure`-ил legacy-ядро: блоб с `enabled:true` воскрешал процесс каждый тик (10 с). Удаление inbound'а сносило только `{core}-{id}`. Как блоб становится `enabled:true` после миграции lucx.102: миграция пишет маркер `migratedToInbound` и УБИРАЕТ `enabled`, но legacy-кнопка Start/Save на Tunnels-странице пересохраняет блоб struct'ом без маркера и с `enabled:true`. Два соседних gap'а: при пустом `want` не вызывался `Reconcile{Naive,Olcrtc}` → orphan `{core}-{id}` не свипились; migrated-блоб считался легитимным desired-state. Те же грабли у всех трёх ядер (naive/olcrtc/qwdtt).
- **Симптом (VladufQa, 13.08.2026):** удалил olcRTC inbound — панель продолжает сыпать `[ice] TRACE`-логами olcrtc, процесс держит клиента (STUN с клиентского IP).
- **Fix (lucx.115, `internal/web/service/tunnel.go`):** (1) `tunnelBlobMigrated(key)` читает маркер; fallback всех трёх reconcile'ов при migrated-блобе принудительно `Enabled=false`; (2) fallback вместо голого `Ensure` вызывает `Reconcile{Naive,Olcrtc}` с legacy-инстансом в want → orphan `{core}-{id}` свипятся и при пустом списке inbound'ов; (3) `legacyLifecycleBlocked` — Start/Restart/Save legacy-эндпоинтов отказывают («manage on the Inbounds page»), если блоб мигрирован ИЛИ есть inbound протокола; Stop НЕ блокируется (кнопка убийства зомби).
- **Диагностика:** `ps aux | grep -E 'olcrtc|caddy-naive|qwdtt'` — путь к конфигу в argv выдаёт ключ: `tunnel/olcrtc.yaml` = legacy-ядро, `tunnel/olcrtc-N.yaml` = orphan inbound'а. Состояние блоба: `sqlite3 /etc/x-ui/x-ui.db "select value from settings where key like 'lucxTunnel_%'"` (`enabled:true` без маркера = зомби-состояние).
- **Лечение уже пострадавшего хоста без обновления:** Tunnels → карточка ядра → Stop (персистит enabled=false), либо `pkill -f olcrtc-linux` / `pkill -f caddy-naive` — текущий reconcile без фикса поднимет снова, с фиксом не поднимет.
- **Урок:** если фича мигрирует хранилище (settings-блоб → inbound), reconcile-fallback на старое хранилище обязан уважать маркер миграции, а lifecycle-эндпоинты старого контура — отказывать после миграции. Иначе удаление НОВОЙ сущности воскрешает СТАРУЮ. Sweep orphan-ключей обязан работать и при пустом `want`.

### Pattern 1g: Веб-обновление «ломало» панель/Xray — headless-промпты (ИСПРАВЛЕНО lucx.66, верифицировано репро)
- **Cause:** веб-обновление (`updatePanel`) запускает `update.sh` через `systemd-run` **без TTY** (stdin=/dev/null). Старый `update.sh` имел безусловные интерактивные промпты (цикл `read -rp` server_ip и SSL-мастер), которые в headless читали EOF вечно / уводили в дефолт «выпустить Let's Encrypt» с остановкой панели → панель/Xray оставались лежать.
- **Fix (lucx.66):** детект `lucx_interactive=[[ -t 0 ]]`; в headless цикл server_ip и SSL-мастер **пропускаются** (SSL настраивается позже из консоли/панели). `update.sh` для веб-обновления всегда скачивается **свежим** с `main` (`panelUpdaterURL`), поэтому фикс действует и при апгрейде со старых билдов.
- **Верификация (2026-08-05, стенд 144.31.224.212):** поставлен старый билд `v3.6.0-lucx.63`, затем веб-обновление (`updatePanel` через API с CSRF) → update-юнит «Deactivated successfully», в логе «Non-interactive update: skipping SSL setup», x-ui active, xray слушает, версия стала lucx.74. **Не падает.** Исторические «падения» = pre-lucx.66 headless-баг; текущий update.sh здоров.
- **Урок:** любой `read -rp` в скрипте, запускаемом через systemd-run из панели, обязан быть обёрнут в интерактив-гард; репро «старый билд → веб-обновление» — единственный способ убедиться, что апгрейд-путь не регресснул.

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
  **Fix:** TUN inbound (AWG/qWDTT) и SOCKS-мост сайдкаров (mieru/TT/naive/olcrtc, `injectSocksEgress`) должны иметь `sniffing: {http,tls,quic, routeOnly:true}` (`awgEgressTunSniffing` в `xray.go`). Без этого сайдкар отдаёт в SOCKS уже резолвленный IP → `geosite:youtube` молчит, трафик уходит в default outbound. Hysteria — нативный inbound Xray: sniffing включается в форме инбаунда (default off).

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
- **Что реально умеют клиенты (срез на 2026-08-14, разбор issue #44).** Панель отдаёт корректный AWG3-конфиг; «не импортируется» почти всегда упирается в клиент, а не в сервер:

  | Клиент | Версия на срезе | AWG 3 / HPK | AWG 3.1 |
  |---|---|---|---|
  | AmneziaVPN | 5.0.0.5 (2026-07-26) | Заявлено в changelog | Нет (3.1 приехал в amneziawg-go ~2026-08-12) |
  | AmneziaWG for Windows (standalone) | 2.0.2 (2026-07-21) | **Нет** — релиз старше AWG3, парсера HPK нет → «incorrect HeaderProtectionKey» ожидаем | Нет |
  | FlClash | текущая | **Нет** (стек Clash/mihomo не знает HPK) | Нет |
  | Throne + ThroneAWGcore | core v0.1.0 | **Нет** — документирует только Jc/S/H/I | Нет |

  - «AmneziaVPN показывает version 2 и не подключается» = импорт уронил HPK, а сервер v3 его требует → handshake не завершится (см. must-match выше).
  - Практический дефолт для парка Windows-клиентов — **отдельный инбаунд `awgVersion = 2`**; `3.1` не ставить, пока нет ни одного стабильного GUI-клиента.
  - Прежде чем винить клиент, проверь, что модуль реально AWG3: `grep awg_header_protection_set_key /proc/kallsyms`, `awg version` ≥ v3. Если HPK не появляется в экспортируемом `.conf` — модуль/инструменты старые, лечится `x-ui update`.

### Pattern 7: CI go-test красный на `TestBuildFirefoxHello_NoGrease`/`TestBuildSafariHello_NoGrease` (pre-existing flaky)
- **Cause:** CI гоняет `go test -shuffle=on` (рандомный seed каждый прогон). Тесты `NoGrease` утверждают, что Safari/Firefox ClientHello **не содержит** GREASE-паттернов `0a0a`/`fafa` в hex. Но `buildSafariHello`/`buildFirefoxHello` (`cps.go`) **пишут** GREASE через `greaseValue()` — `rng.Intn` над 16 значениями `[0x0A0A … 0xFAFA]`. Тест проходит в 14/16 случаев (когда rng не выдаёт `0x0A0A`/`0xFAFA`), падает в ~1/10 shuffle-прогонов. Воспроизводится локально: `go test ./internal/awg/cps/... -count=20 -shuffle=on`. **НЕ связано с AWG-изменениями** — чужой баг в логике обфускации.
- **Fix (временный):** `gh run rerun <id> --failed` — другой shuffle-seed с высокой вероятностью проходит.
- **Fix (корневой, TODO отдельным issue):** либо тест должен проверять GREASE только в extension-позициях (а не по всему hex), либо `buildSafariHello`/`buildFirefoxHello` не должны писать GREASE через rng (Safari/Firefox по реальным fingerprint'ам GREASE не используют — только Chrome). Пробовал `SetRand(t *testing.T, …)` с `t.Cleanup` для изоляции глобального `rng` — не помог (проблема в логике, не в загрязнении); откатил. НЕ чинить наспех — это domain-логика CPS-обфускации.
- **Урок:** при падении CI-джоба сначала проверь, твой ли это регресс. Воспроизведи локально с точным shuffle-seed из лога CI (`-test.shuffle <N>`). Если проходит локально и не связано с твоими файлами — это flaky, rerun оправдан.

### Pattern 7b: ~~CI frontend красный на storybook a11y `ConfigBlock.stories.tsx → Collapsed`~~ — ИСПРАВЛЕНО (lucx.58)
- **Cause (уточнён по логу CI, attempt 1 run 30816344911):** axe сообщал `insufficient color contrast of 2.29 (foreground #a6a6a6, background #f8f8f8)` на `<code class="config-block-text">`. `#a6a6a6` — это финальный цвет текста (#595959 из `body.light .config-block-text`), **смешанный с фоном при ~54% opacity**: a11y-addon гоняет axe сразу после play(), а antd Collapse ещё в fade-in анимации разворачивания. Гонка по таймингу: axe попадает либо на середину fade (fail), либо после (pass).
- **Fix (lucx.58):** `token.motion: false` в ConfigProvider storybook-декоратора (`.storybook/preview.tsx`) — анимации antd в stories отключены, разворачивание мгновенное, axe всегда видит финальное состояние. Продакшн-анимации не затронуты (декоратор — только storybook). Проверено 3 последовательными прогонами. CSS не трогали — `--ant-*` переменные резолвятся корректно (cssVar-режим antd даёт переменным scope, а не переименовывает префикс).
- **Урок:** storybook a11y + анимации появления = плавающие color-contrast-падения (axe меряет элемент в середине fade и видит blended-цвет). Отключение motion в test-декораторе — стандартный фикс; не чинить CSS, если финальный контраст в порядке.
- **Урок:** если CI frontend падает ЕДИНСТВЕННЫМ тестом `storybook ... ConfigBlock → Collapsed` с `color-contrast` — это этот flake; проверь что твои тесты зелёные и rerun, не трать время на поиск регрессии в своём коде.

### Pattern 8: «выключить аутбаунд можно, включить нельзя» + коллизия подсетей awgo vs клиенты инбаунда — ИСПРАВЛЕНО (lucx.69)
- **Cause 1 (enable-кнопка):** фронт `awgOutboundsApi.enable` не передавал `JSON_HEADERS`; `http-init.ts` сериализует тело в JSON **только** при `Content-Type: application/json`, иначе form-urlencoded (`enable=true`). Бэк `_ = c.ShouldBindJSON(&body)` молча не парсил form-тело → `body.Enable=false` → каждый enable становился disable. **Урок:** любой POST с JSON-телом во фронте обязан передавать `JSON_HEADERS`; в контроллере не глотать ошибку бинда (`_ =`) — проверять и падать громко.
- **Cause 2 (коллизия подсетей):** awgo-аутбаунд берёт адрес из .conf провайдера (часто 10.8.0.0/24). Если клиенты AWG-инбаунда сидят в той же /24 (legacy wrong-subnet), reverse-path ломается → флуд `ERROR XRAY: proxy/tun: connection was refused`. Старый `checkAwgSubnetConflict` сверял только серверный адрес инбаунда и только при сохранении ИНБАУНДА.
- **Fix (lucx.69):** (1) фронт enable → `JSON_HEADERS`; (2) бэк `ShouldBind` с `json:"enable" form:"enable"` + проверка ошибки; (3) новый guard `AwgOutboundService.checkSubnetConflict`→`awgOutboundSubnetClash` в Add/Update аутбаунда: сверяет адрес awgo и с серверной подсетью, и с **клиентскими IP** каждого AWG-инбаунда (`awgSettingsClientIPs`), /32-/128 освобождены.
- **Диагностика «proxy/tun: connection was refused»:** ошибки СЫПЯТ только при живом awgo-аутбаунде с трафиком и исчезают, когда аутбаунд выключен/убран → первым делом проверить подсеть awgo против клиентских подсетей AWG-инбаундов (`ip route`, `awg show`). Это НЕ баг TUN/Xray как таковой, а коллизия маршрутизации. ВТОРАЯ частая причина (lucx.72, VladufQa): **битый/множественный DNS в инбаунде**, уходящем в аутбаунд — клиенты резолвят кривые IP и сервер диалит их → RST/timeout; лечится одним рабочим DNS (1.1.1.1) в инбаунде. ParseConf аутбаунда с lucx.72 берёт только первый DNS из «a, b».
- **Урок:** локально на Windows пакет `internal/web/service` не собрать (нет gcc → CGO `sqlite3.Backup`), чистую логику проверять standalone-программой, полный тест — GitHub Actions CI (см. Test Commands).

### Pattern 9: «proxy/tun: connection was refused / operation timed out» спамит при ЗДОРОВОМ сервере — benign client-шум (диагностика 2026-08-06, НЕ баг)
- **Симптом:** в логах постоянно `ERROR - XRAY: proxy/tun: connection was refused` (и реже `operation timed out`, `connection reset by peer`), всплесками по 15–74 шт. за 1–4 с, при этом «всё работает»: handshake свежие, трафик идёт, других ошибок в журнале НЕТ.
- **Когда это НЕ Pattern 8:** подсеть awgo-аутбаунда НЕ пересекается с клиентскими подсетями инбаундов (проверить `ip -4 -br addr` + .conf'ы), DNS в инбаундах рабочий. Тогда причина ниже.
- **Первоисточник:** xray-core `proxy/tun/stack_gvisor.go` — TCP-forwarder gvisor-netstack: `ep, err := r.CreateEndpoint(&wq); if err != nil { errors.LogError(t.ctx, err.String()) }`. `CreateEndpoint` блокируется до завершения 3-way handshake **между netstack и AWG-клиентом** (SYN клиента → SYN-ACK netstack → ACK клиента) и падает, если клиент handshake не завершил. Строки — это `String()` ошибок gvisor `tcpip` (pkg/tcpip/errors.go): `ErrConnectionRefused` = «connection was refused», `ErrTimeout` = «operation timed out», `ErrConnectionReset` = «connection reset by peer».
- **Семантика (gvisor `tcp/handshake`, synRcvdState/synSentState):** `refused` = клиент прислал **RST во время handshake** (сам оборвал соединение, которое сам же инициировал — приложение отменило коннект, устройство уснуло/переключило сеть, ОС клиента разом закрывает пачку half-open сокетов → всплеск). `timed out` = netstack дослал SYN-ACK до исчерпания ретраев, а финальный ACK клиента так и не пришёл (потеря пакета на последнем хопе клиента / клиент исчез).
- **Верификация (стенд VladufQa 195.133.32.18, lucx.74):** за 24ч 636 ошибок (570 refused / 65 timeout / 1 reset) — ЕДИНСТВЕННЫЙ тип ошибок за весь период; ошибки идут ВСЮ историю журнала (с 21.07) при всех версиях панели → не регрессия. AF_PACKET-capture на tun2/tun13 (150 с, python без tcpdump): 86 здоровых соединений, 0 оборванных handshake'ов в окне; пойман один timeout-кейс — netstack 5× шлёт SYN-ACK на `Work-PC → 213.59.253.21:443` без ACK клиента, и ровно в момент истечения таймаута в journal падает `operation timed out`. Прямая корреляция 1:1.
- **Вывод:** это Xray логирует на уровне ERROR незавершённые клиентские handshake'ы — шум мобильных/спящих клиентов (семейные телефоны, браузерные connection-racing, doze). НЕ баг панели/AWG/маршрутизации; чинить нечего, лог не подавляется настройкой Xray (ERROR пишется всегда). Отличать от Pattern 8: там ошибки СЫПЯТ только при живом awgo с трафиком и исчезают с его выключением + есть пересечение подсетей.
- **Метод диагностики без tcpdump** (tcpdump на сервере может быть не установлен, а ставить пакеты запрещено): python3-скрипт с `socket.AF_PACKET, SOCK_RAW` на tun-интерфейсах (read-only sniff), парсинг IPv4/TCP-флагов, учёт SYN/SYN-ACK/ACK/RST per 4-tuple; сверять окно захвата с `journalctl -u x-ui` по минутам. Клиент IP→email мапится из DB: `sqlite3 /etc/x-ui/x-ui.db "select settings from inbounds where protocol='awg'"` → `clients[].email` + `clients[].allowedIPs`.

### Pattern 10: адрес в ссылках/подписке = IP самого подписчика (X-Real-IP) — ИСПРАВЛЕНО (lucx.125)
- **Симптом:** у инбаунда без собственного адреса (wildcard listen, без узла, без managed-хоста — типичная форма AWG) в подписке стоит чужой адрес: `Endpoint` в `.conf`/`vpn://` или `server:` в Clash-профиле равен WAN/локальному IP того, кто скачал подписку. У VLESS/Reality/WS того же оператора адрес правильный — просто потому что у тех инбаундов адрес есть свой (host-запись, listen или shareAddr). Репорты Aleksandr SacredX: lucx.89 (`.conf`), lucx.124 (Clash).
- **Cause:** `ResolveRequest` (internal/sub/service.go) брал `host` в порядке `X-Forwarded-Host` → **`X-Real-IP`** → `Host`. Nginx по нашей же документации ставит `proxy_set_header X-Real-IP $remote_addr` — то есть адрес КЛИЕНТА, и он «routable», поэтому `PrepareForRequest` его не отбраковывал. Дальше `resolveInboundAddress` доходит до `s.address` последним звеном цепочки (shareAddr/node/listen → subDomain/webDomain → адрес запроса) — и подставляет IP подписчика. Тот же порядок был в `resolveHost` (internal/web/controller/inbound.go) — оттуда IP админа попадал в ссылки и QR панели.
- **Fix (lucx.90, частичный):** `AwgEndpointHost` — только для `/awg/`. Остальные форматы (raw `/sub/`, `/json/`, `/clash/`) и панельные ссылки продолжали течь.
- **Fix (lucx.125, полный):** общий `requestServerHost(c, trusted)` — trusted `X-Forwarded-Host`, иначе хост из `Host`; `X-Real-IP` не читается никогда. На него переведены `ResolveRequest` и `AwgEndpointHost`; из `resolveHost` контроллера ветка `X-Real-IP` удалена. Тесты: `TestResolveRequest_HostNeverRealIP`, `TestGetProxies_AwgWithoutOwnAddressUsesSubscriptionHost`, `TestResolveHostNeverUsesRealIp`.
- **Диагностика (одной командой, репро без клиента):** `curl -s -H 'X-Real-IP: 203.0.113.77' https://<sub-домен>/clash/<subId> | grep -B2 'type: wireguard'` — если в `server:` появился `203.0.113.77`, панель ещё течёт.
- **Урок:** `X-Real-IP`/`X-Forwarded-For` отвечают на вопрос «кто пришёл», а не «куда подключаться». В любом коде, который собирает АДРЕС СЕРВЕРА для клиента, эти заголовки запрещены; допустимы только `X-Forwarded-Host` (от доверенного прокси) и `Host`.

### Pattern 1o: olcRTC «работал, потом сломался» после обновления панели — upstream wire-break + беспинный master — ИСПРАВЛЕНО (lucx.132)
- **Симптом:** olcRTC-туннель (любой провайдер — Telemost/Jitsi/WB) работал, после `x-ui update` / веб-обновления клиенты перестают подключаться, хотя конфиг/комната/провайдер не менялись. Репорт NoName (16.08.2026): «в пятницу летало через яндекс, вчера перестал».
- **Cause:** `release.yml` собирал olcrtc из **неприкреплённого `master`**. Апстрим 14.08.2026 слил PR #140 «Refactor/global overhaul» (252 файла), полностью переписавший крипто-слой: старый формат (сырой ключ → XChaCha20-Poly1305, фрейм `[24B nonce][ct][tag]`) заменён на «OLC2» (HKDF-SHA256 directional-ключи, фрейм `magic "OLC2"|counter|16B prefix|ct|tag`, replay-window, AAD). **Fallback на старый формат НЕТ** — их readme: «no compatibility fallback… Upgrade both endpoints together». Сервер после обновления говорит OLC2, клиентское приложение (owenclave/olcbox) остаётся на старом крипто → ни один пакет не проходит auth → туннель мёртв. YAML/URI-схемы совместимы — ломается только data-plane.
- **Диагностика:** версия бинарника не печатается (`usage: olcrtc <config.yaml>` на любой флаг). Смотреть дату файла `/usr/local/x-ui/bin/olcrtc-linux-amd64` (совпадает с датой последнего обновления панели) и логи сайдкара `journalctl -u x-ui | grep -i olcrtc` (префикс `olcrtc: <label> |`). Ключ-признак: сервер обновлялся, клиентское приложение — нет.
- **Fix (lucx.132):** пин `OLCRTC_REF` в release.yml на `3339cd36716885e583429f97e73462cde4984e2e` (последний master до PR #140 = бинарник lucx.112–118, проверенный на Telemost). Клонирование через `git init + fetch --depth 1 <SHA> + checkout FETCH_HEAD`: `git clone --branch <SHA>` на GitHub НЕ работает («Remote branch … not found»), fetch по SHA — работает.
- **Лечение уже пострадавшего хоста без lucx.132:** вытащить `olcrtc-linux-amd64` из tarball `v3.6.0-lucx.118`, заменить `/usr/local/x-ui/bin/`, перезапустить inbound; либо на клиенте поставить OLC2-сборку (olcbox nightly от 16.08.2026+; owenclave OLC2 ещё не имеет).
- **Урок:** внешние sidecar-бинарники, собираемые из чужого `master` без пина, — бомба: любой wire-breaking мерж апстрима молча уезжает в следующий релиз. Пинуются ВСЕ ядра (mieru/TrustTunnel/caddy-naive уже; olcrtc был исключением). Снимать пин olcrtc только когда клиенты выпустят OLC2-сборки И прогнан e2e на реальном провайдере.

### Pattern 1p: TrustTunnel «слушает, трафика нет» + outbound AWG — ИСПРАВЛЕНО (lucx.133)
- **Симптом:** процесс `trusttunnel-N` UP, TCP/UDP :443 слушает, клиент коннектится, интернета нет. В логе `trusttunnel egress : target tag [ SW ] not found, skipping injection`. `ss` не показывает loopback SOCKS (`routeXrayPort`).
- **Cause:** `injectAwgOutbounds` шёл после SOCKS-инжекта. `injectSocksEgress` при неизвестном теге **выходил целиком** — SOCKS не поднимался, а TOML уже писал `socks5 = 127.0.0.1:<port>`. AWG-TUN так не делает (мост всегда).
- **Fix (lucx.133):** awgo-теги инжектятся до egress; неизвестный тег → warning + SOCKS без force-route.
- **Обход без обновления:** выключить «через Xray» или очистить outbound (пусто = котёл, SOCKS поднимется).
- **Урок:** инжектор моста не должен быть all-or-nothing из-за опционального force-route. Цель правила должна существовать **до** lookup.

### Pattern 1q: AllowedIPs клиента AWG «сохраняет и откатывает» — ИСПРАВЛЕНО (lucx.134)
- **Симптом (VladufQa):** меняешь IP в карточке клиента — сохраняется, после reload старый. Чтобы получить нужный адрес на втором хостере, плодил фейковых клиентов и удалял лишних.
- **Cause:** `ClientService.Update`/`Create`/`bulk` всегда делали `per.AllowedIPs = nil` на AWG/WG, чтобы multi-attach не копировал один IP на разные подсети. Пустое поле → `fillAwgClients` выделяет следующий свободный (часто тот же старый). Операторный ввод выбрасывался даже когда инбаунд один.
- **Fix (lucx.134):** `clearBroadcastTunnelIP` чистит IP только если в этом save больше одного AWG/WG inbound. Один туннель — пишем как в форме (коллизия по-прежнему ошибка).
- **Не делаем:** два инбаунда на одной панели с одной подсетью (Pattern 1e). Совпадение IP на **разных** серверах — как раз ручной AllowedIPs.
- **Урок:** защита от broadcast не должна глушить единственный легитимный путь задать адрес.

### Pattern 1r: «Failed to fetch» при скачивании подписки + AWG без скорости — ИСПРАВЛЕНО (lucx.135)
- **Симптом (Kirill/Chingiz):** в карточке клиента Download у SUB/AMNEZIA даёт «Failed to fetch»; Copy у AMNEZIA кладёт URL вместо тела; на публичной sub-странице у AWG только amneziawg; колонка «Скорость» у AWG-клиентов — вечное «—», хотя «Трафик» растёт.
- **Cause 1 (CORS):** sub-сервер слушает отдельный порт без CORS-заголовков; браузерный `fetch` с origin панели падает, когда origin'ы различаются (обычная конфигурация). Работал только vpn:// через same-origin `awgBody` (lucx.98).
- **Cause 2 (скорость):** колонка «Скорость» = дельты 5-секундного broadcast'а `XrayTrafficJob`; AWG в stats API Xray не попадает (kernel TUN). Суммы в БД пишет `AwgJob` — поэтому «Трафик» виден, «Скорость» нет.
- **Fix (lucx.135):** (1) `GET /panel/api/clients/subBody?url=…` — loopback в локальный sub-сервер (path+query только, host игнор; Host=subDomain для DomainValidator; нейтральный UA); фронт — все строки модалки через `fetchSubscriptionBody`, Copy у AMNEZIA = тело конфига; sub-страница получает `PageData.SubAwgUrl` и строку AMNEZIA (.conf/vpn:// — `<a href>` с attachment-заголовками, copy = same-origin fetch). (2) `awg_speed_buffer.go` + LUCX-HOOK в `xray_traffic_job.go` — нормированные к 5 с дельты AWG подмешиваются в тот же broadcast-фрейм.
- **Урок:** публичный listener без CORS ≠ источник данных для браузера панели; любой «скачать тело» в UI идёт через same-origin прокси. Живая скорость = только broadcast-дельты; всё, что метится вне Xray (AWG, mtproto, tunnel), обязано подмешивать свои дельты в общий фрейм.
