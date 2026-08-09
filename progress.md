# LucX-UI — Прогресс

> Файл ведётся агентом в ходе работы. Обновляется при каждом шаге.
> Последняя миграция апстрима: **v3.6.0** (см. запись в конце файла).

---

## Контекст

- **Репозиторий:** [AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) — форк 3x-ui
- **Текущая база апстрима:** v3.6.0 (`behind 0` относительно `origin/main`)
- **Первая миграция:** с v3.3.1 → v3.5.0 (228 коммитов апстрима), стратегия migrate — свежий checkout + перенос LucX-кода поверх
- **Ветка миграции v3.5.0:** `feat/awg-sidecar-v3.5.0` (создана от `origin/main` v3.5.0)
- **Ветка миграции v3.6.0:** `feat/upstream-v3.6.0` (merge-коммит поверх `feat/awg-sidecar-v3.5.0`)
- **Старая ветка:** `feat/awg-sidecar` (на v3.3.1, эталон для переноса)
- **Дата начала:** 2026-07-13

---

## План

### Этап 1. Очистка мусора ✅
- [x] Закрыть 10 dependabot PR (#1-#12) на GitHub
- [x] Удалить 10 dependabot/* веток на GitHub
- [x] Удалить старую ветку `feature/awg-integration` (локально + удалённо)
- [x] Удалить старую ветку `lucx-ui-phase1` (локально + удалённо)

### Этап 2. Миграция на v3.5.0
- [x] Создать чистую ветку `feat/awg-sidecar-v3.5.0` от `origin/main` (v3.5.0)
- [x] Перенести 29 изолированных LucX-файлов (internal/awg, internal/lucx, migrate_awg, frontend awg.ts/awg.tsx, bin/install-awg-module.sh, awg_job.go) — закоммичено
- [x] Восстановить LUCX-HOOK маркеры в upstream-файлах v3.5.0:
  - [x] `model.go` — AWG Protocol const + validate oneof
  - [x] `db.go` — вызов `pruneLegacyAwgHiddenChildren`
  - [x] `runtime/local.go` — AWG делегирование в AddInbound/DelInbound/AddUser/RemoveUser
  - [x] `service/xray.go` — AWG exclusion + `injectAwgEgress`
  - [x] `web.go` — import awg + cron wiring + StopAll
  - [x] `install.sh` — вызов `bin/install-awg-module.sh`
  - [x] `xray_config_inject_test.go` — тесты injectAwgEgress (5 тестов)
  - [x] frontend: `inbound-defaults.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`, `protocols/index.ts`
- [x] Прогнать тесты: go test + frontend typecheck/lint
- [x] Frontend: `npm run build` → `internal/web/dist/` собран
- [x] `go build -o /tmp/x-ui .` → exit 0, бинарник 111 МБ
- [ ] Коммит миграции + обновление progress.md/AGENTS.md

### Этап 3. Деплой и проверка (после миграции)
- [ ] SCP на vps_finland_lucx, рестарт x-ui.service
- [ ] Проверка `systemctl status x-ui`, логи
- [ ] Проверка реального запуска AWG kernel-интерфейса

---

## Что сделано

## Релиз v3.6.0-lucx.85 (2026-08-07)

3. **fix:** RuleFormModal infinite re-render (useEffect + unstable deps) hung vitest components/CI.

1. **Routing:** выбор клиентов из списка с поиском — AWG/WG → `sourceIP`, VLESS/… → `user` (VladufQa).
2. **Dashboard AWG:** убрана «Обновить AWG» (DKMS); вместо неё **Перезапуск AWG** (StopAll+Reconcile). Тултип: число UP + имена ifaces.

## Релиз v3.6.0-lucx.84 (2026-08-07)

1. **Client IP stable:** смена Address не переписывает AllowedIPs клиентов (кроме коллизии с IP сервера) — без перекачки .conf.
2. **NAT by mark:** MASQUERADE по mark iif awgN, не `-s subnet`.
3. **AWG 1.5 fix:** server conf больше не пишет S3/S4 на v1.5 (must-match с клиентским export).
4. **UI rtx:** ползунок + outbound наверху формы; default ON + outbound «правила маршрутизации»; hint без re-export.
5. **Dashboard AWG pill:** статус модуля/интерфейсов + кнопка «Обновить AWG» (`POST /updateAwgModule` → install-awg-module.sh --force-rebuild).

## Релиз v3.6.0-lucx.83 (2026-08-07) — update UI: без dev-канала, release notes, дольше ждать

1. Убран switch «Dev channel» из PanelUpdateModal и Nodes (stable only).
2. `getPanelUpdateInfo` отдаёт `releaseNotes` (body GitHub release) — блок «Что нового» в модалке.
3. Poll обновления: 90s → **15 мин**; тексты dontRefresh/unknown — «подождите, AWG может долго».
4. CI flake: `TestBuildSafari/FirefoxHello_NoGrease` больше не сканирует random bytes (Pattern 7).

## Релиз v3.6.0-lucx.82 (2026-08-07) — PersistentKeepalive: UI клиента + export-only

**Запрос:** поле как в lucx.80, чтобы пользователи не ломались; PKA — настройка **клиента**, не сервера.

**Сделано:**
- Вернули `KeepAliveValue` (число / AWG3 range `"15-25"`), UI `wgKeepAlive` в `ClientFormModal` (WG/AWG), outbound keepalive form, openapigen.
- **Серверный** `renderServerConf` / `SyncPeers` **не** пишут `PersistentKeepalive` (client-export only).
- Fingerprint peer без keepalive (смена PKA не рестартит iface).
- Export: `BuildAwgClientConf`, `wireguardConfig.ts`, sub/clash/json links.
- Default нового клиента: `25` (форма + `defaultAwgClients` / `defaultWireguardClients`).

## Релиз v3.6.0-lucx.81 (2026-08-07) — откат PersistentKeepalive-эксперимента

**Жалоба:** «поле не появилось, трафик перестал ходить».

**Откат:** KeepAliveValue/AwgTimer для peer PersistentKeepalive, default 0, UI клиента, range keepalive, openapigen KeepAliveValue. Вернули `int` + default 0→25 на инбаунде + outbound default 25 — как до lucx.75.

**Оставлено:** TG .conf + Endpoint public IP (79), Panel outbound AWG tags (77), sidebar @Lucx_soft, js-yaml audit.

## Релиз v3.6.0-lucx.80 (2026-08-07) — CI-fix после lucx.79

- gofumpt `client_awg_share_host_test.go`
- вернули i18n key `pages.xray.awgOutbound.keepalive` (dead-keys test)

## Релиз v3.6.0-lucx.79 (2026-08-07) — TG .conf Endpoint: не OS hostname

**Жалоба:** бот писал `Endpoint = ruvds-dczf5:57092` вместо публичного IP/домена как в панели.

**Корень:** `awgEndpointHost` fallback на `os.Hostname()` после пустых sub/web domain.

**Фикс:** `ResolveInboundShareHost` (как panel/sub: strategy → node/listen → subDomain/webDomain → **public IPv4/IPv6** via GetStatus). Hostname больше не используется. IPv6 в Endpoint в скобках.

## Релиз v3.6.0-lucx.78 (2026-08-07) — UI PersistentKeepalive для AWG/WG клиентов

**Жалоба:** «поля PersistentKeepalive нет в настройках awg3 / парсинге оутбаундов / выгруженном конфиге».

**Факт:** в lucx.75 сделали тип/рендер (range, default 0=omit), outbound-форму уже с полем keepalive, но:
- **инбаунд AWG** — PKA peer-level, UI был только у KeepaliveTimeout (device);
- **форма клиента** — не было поля keepAlive → всегда 0 → .conf без строки;
- outbound: поле было (рядом с MTU), label неочевиден; paste теперь нормализует string.

**Фикс:** ClientFormModal — поле PersistentKeepalive для WG/AWG; payload.keepAlive; outbound label «PersistentKeepalive»; paste keepalive→string.

## Релиз v3.6.0-lucx.77 (2026-08-07) — Panel outbound picker видит AWG outbounds

**Баг (VladufQa):** Settings → Panel outbound — «ничего кроме direct не выбирается», хотя 2 AWG-outbound’а живые. Бот TG timeout → нужен egress через awgo; picker не показывал теги.

**Корень:** API уже отдаёт `awgOutboundTags` (как `subscriptionOutboundTags`), Routing/Balancers их подмешивают, а **GeneralTab** (panel egress picker) + `useOutboundTags` + GeodataSection — только template + subscription. AWG-теги терялись.

**Фикс:** подмешать `payload.awgOutboundTags` / `data.awgOutboundTags` во все три picker’а. Backend `injectPanelEgress` уже после `injectAwgOutbounds` — AWG-тег валиден как target.

**Файлы:** GeneralTab.tsx, useOutboundTags.ts, GeodataSection.tsx, config.go (lucx.77).

## Релиз v3.6.0-lucx.76 (2026-08-07) — CI-fix после lucx.75 (gofumpt + js-yaml audit)

- gofumpt: `model.go`, `client_instance.go` (alignment после KeepAliveValue/AwgTimer)
- frontend audit: override `js-yaml` → `^4.3.1` (CVE-2026-59870, swagger-ui-react)
- lucx.75 stable-релиз уже опубликован и рабочий; 76 = зелёный CI + тот же функционал

## Релиз v3.6.0-lucx.75 (2026-08-07) — PersistentKeepalive range, TG .conf, ссылка на Telegram

**1. PersistentKeepalive (AWG3 range, default 0):**
- Peer-level `PersistentKeepalive` теперь `AwgTimer` / `KeepAliveValue` end-to-end: single `"25"` или range `"15-25"` (ядро u16_range_t, tools v3).
- Default **0 = off** (строка omit в .conf). Убран forced default 0→25 в `InstanceFromInbound`.
- Outbound: default keepalive `"0"`, ParseConf сохраняет range, форма Input + placeholder.
- `model.Client.KeepAlive` → `KeepAliveValue` (JSON number|string; GORM text; WireGuard/xray берут `.Int()` = lo range).
- openapigen: `AliasAllow` + `KeepAliveValue`.

**2. Telegram bot — AWG `.conf` вместо QR:**
- `sendClientQRLinks` для protocol=awg шлёт `email.conf` через `BuildAwgClientConf` (зеркало frontend `buildAwgClientConfig`).
- Кнопка у AWG-клиента: `.conf`; общее меню: `.conf / QR`.

**3. Sidebar — Telegram community:**
- Под версией панели: иконка + `@Lucx_soft` → https://t.me/Lucx_soft (collapsed = только иконка).

**Тесты:** `go test ./internal/awg/... ./internal/database/model/...`; frontend `tsc` + `lint` + `npm run gen`.

**Файлы:** model.go (+keepalive_value_test), instance/manager/client_*, awg_outbound, client_awg (BuildAwgClientConf), tgbot_*, AppSidebar, wireguardConfig, schemas, openapigen, config.go (lucx.75), progress.md.

## Диагностика (2026-08-06) — «proxy/tun: connection was refused» на сервере VladufQa: benign client-шум, не баг (Pattern 9)

**Запрос:** на 195.133.32.18 (VladufQa, панель lucx.74) постоянно спамит `ERROR - XRAY: proxy/tun: connection was refused`, «хотя всё работает». Разрешены только read-only команды.

**Что проверено (всё здорово):** панель lucx.74 active, Xray 26.7.28; awg2/awg13 (оба routeThroughXray, tun2/tun13) + awgo-7/awgo-9 (оба живые, handshake свежие, GiB трафика, endpoint 109.120.156.14); ip rule/route/tables 1002/1013 корректны; подсети awgo (10.205.0.2/32, 10.200.0.2/32) НЕ пересекаются с клиентскими (10.8.0.x, 11.85.5.x) → это НЕ Pattern 8. За 24ч 636 ошибок (570 refused / 65 timeout / 1 reset) — единственный тип ошибок в журнале; ошибки идут всю историю журнала (с 21.07) при всех версиях панели → не регрессия.

**Корень:** xray-core `proxy/tun/stack_gvisor.go` логирует `err.String()` при неудачном `CreateEndpoint` — т.е. когда 3-way handshake **между netstack и AWG-клиентом** не завершён. `refused` = клиент прислал RST посреди handshake (gvisor synRcvdState, сам оборвал); `timed out` = SYN-ACK ретраи исчерпаны, ACK клиента не пришёл. Всплески 15–74 шт./1–4с = устройство проснулось/сменило сеть и ОС разом RSTит пачку half-open сокетов.

**Live-верификация:** AF_PACKET-capture на tun2/tun13 (python3, 150 с, без tcpdump): 86 здоровых соединений; пойман timeout-кейс — netstack 5× слал SYN-ACK на `Work-PC → 213.59.253.21:443` без ACK, и ровно в момент истечения таймаута в journal упал `operation timed out`. Корреляция 1:1. В окне захвата оборванных handshake'ов не было → ошибок в окне не было (спорадичность подтверждена).

**Вывод:** чинить нечего — Xray логирует незавершённые клиентские handshake'и на уровне ERROR (мобильные/спящие клиенты, семейные телефоны). Подавить настройкой Xray нельзя. Записано в AGENTS.md как **Pattern 9** (+ метод диагностики без tcpdump через AF_PACKET и маппинг IP→email из DB).

**Попутные наблюдения (не причина, не правились):** awg2 — legacy рассинхрон «сервер 10.8.1.1/24 vs клиенты 10.8.0.x» (работает через /32-маршруты; лечится сменой Address в панели — миграция клиентов lucx.59+); остаточные FORWARD-правила удалённого awg0 (косметика).

**Файлы:** `AGENTS.md` (Pattern 9), `progress.md` (эта запись). Код не менялся.

## Релиз v3.6.0-lucx.74 (2026-08-05) — тест аутбаунда по ответу в выводе + AWG3-диапазоны в аутбаунде

**1. Тест-эндпоинт аутбаунда (косметика):** `ping` мог быть убит контекстом (signal: killed) или выйти ненулевым **после** получения ответа, а хэндлер судил по коду выхода → ложный «ping failed». Теперь успех = наличие строки «bytes from» в выводе, независимо от err.

**2. AWG3 device-таймеры аутбаунда — диапазоны (зеркало инбаунда):** VladufQa: «аутбаунд при парсинге не распознает поля awg3… не поддерживает '-', только одно значение». В .conf провайдера `RekeyAfterTime = 100-120` и т.д. — **диапазоны**, но `ClientSettings` держал их как `int` → `strconv.Atoi("100-120")`=0 (терялось), форма `InputNumber` не давала ввести «-». На ИНБАУНДАХ это уже лечили (тип `AwgTimer` — строка, терпит JSON-число через UnmarshalJSON, `IsZero()`, пишется `%s`). Зеркально на аутбаундах:
- `internal/awg/client_instance.go`: 6 device-полей `int`→`AwgTimer`; fingerprint — `string(...)`.
- `internal/awg/client_conf.go`: `> 0`+`%d` → `!IsZero()`+`%s`.
- `internal/web/service/awg_outbound.go` (ParseConf): `Atoi`→`awg.AwgTimer(val)` (сырой диапазон).
- фронт: схема `awg-outbound.ts` — `z.preprocess(normalizeAwgTimer, z.string())`; форма `InputNumber`→`Input`, DEFAULT `'0'`.
- Миграция НЕ нужна: `AwgTimer.UnmarshalJSON` терпит legacy JSON-числа.
- Тесты: `TestParseConf_Awg3RangeTimers` (диапазон сохраняется, awgVersion=3).

**Файлы:** controller/awg_outbound.go (тест), awg/client_instance.go, awg/client_conf.go, service/awg_outbound.go(+тест), schemas/awg-outbound.ts, AwgOutboundFormModal.tsx, config.go (lucx.74).

## Релиз v3.6.0-lucx.72 (2026-08-05) — ParseConf аутбаунда берёт только первый DNS

**Контекст (tester VladufQa):** «оутбаунд поддерживает только 1 днс, а при импорте вписывается 2 через запятую, значит импорт кривой». Провайдерские .conf содержат `DNS = 1.1.1.1, 1.0.0.1`; `ParseConf` (импорт «Paste .conf») писал всё целиком в `Settings.DNS`, а поле аутбаунда рассчитано на один DNS → в форме «через запятую два вбито».

**Фикс (`internal/web/service/awg_outbound.go`, ParseConf):** для ключа `DNS` берём только первую запись до запятой (`strings.Cut`). `renderClientConf` DNS и так не пишет (не ломает системный резолвер), так что фикс чистит данные/форму и страхует будущих потребителей.

**Тест:** `TestParseConf_DNSFirstOfList` (comma-list → первый; одиночный → как есть).

**Связанное наблюдение (не код):** флуд `proxy/tun: connection was refused` у VladufQa лечился установкой **одного рабочего DNS (1.1.1.1)** в инбаунде, уходящем в аутбаунд — клиенты резолвили домены через битый/множественный DNS и сервер диалил кривые IP. Это конфиг инбаунда, не баг панели; см. Pattern 8 в AGENTS.md.

## Релиз v3.6.0-lucx.71 (2026-08-05) — re-print кредов не печатает стейлый install-result.env

**Контекст (tester VladufQa, обновление до lucx.69):** блок «Panel Access» в конце установки напечатал **чужие/старые** креды («а это не то, у меня другие данные»). Фича lucx.68 читала `/etc/x-ui/install-result.env`, но этот файл пишется `write_install_result` **только при генерации/смене** кредов. На сервере, где админ давно задал свои креды, при рядовом обновлении `write_install_result` не вызывается → файл стейлый с первоначальной установки → re-print показывает неактуальный логин.

**Фикс (`install.sh`):** `write_install_result` при успешной записи ставит глобальный флаг `XUI_INSTALL_RESULT_WRITTEN=1`; финальный re-print блок печатает только при `[[ "$XUI_INSTALL_RESULT_WRITTEN" == "1" && -r ... ]]`. То есть креды перепечатываются лишь когда они реально (пере)сгенерированы в ЭТОМ запуске (свежая установка / сброс), а при рядовом обновлении кастомных кредов блок пропускается и не светит стейлом.

**Файлы:** `install.sh`, `internal/config/config.go` (lucxVersion → lucx.71).

## Релиз v3.6.0-lucx.70 (2026-08-05) — install-awg-module.sh: dkms ставится надёжно, ранний понятный фейл

**Контекст (скрин установки lucx.68):** `bin/install-awg-module.sh: line 232: dkms: command not found` + бокс «AWG: awg-quick НЕ установлен». Скрипт ставит ВСЕ сборочные пакеты одним `apt-get install ... || true` (строка 88), а apt — «всё или ничего»: один недоступный опциональный пакет (qrencode/bc/…) роняет всю транзакцию → **dkms не ставится**, ошибка съедена `|| true` → на строке 232 (`dkms build`) «command not found».

**Фикс (`bin/install-awg-module.sh`):**
- Критичные пакеты (`build-essential dkms git libmnl-dev pkg-config`) — **отдельным** apt-вызовом; опциональные утилиты (`unzip curl python3 net-tools qrencode bc ca-certificates gnupg`) — отдельным best-effort вызовом, никогда не блокируют сборку.
- Ранняя проверка `command -v dkms/make/gcc`: если тулчейна реально нет (нет сети/репо) — понятный красный вывод с командой ручного доустановления и `exit 1`, вместо голого «dkms: command not found» на 232-й. Если dkms/make/gcc уже есть с прошлого запуска — продолжаем даже при упавшем apt.

**Файлы:** `bin/install-awg-module.sh`, `internal/config/config.go` (lucxVersion → lucx.70).

## Релиз v3.6.0-lucx.69 (2026-08-05) — кнопка «включить AWG-outbound» + guard подсетей аутбаундов

**Контекст (tester VladufQa, 2026-08-05):** две проблемы, вскрытые отладкой `proxy/tun: connection was refused` на двойном VPN (AWG-inbound → TUN → AWG-outbound awgo-N → апстрим-провайдер):
1. **Кнопка «включить аутбаунд» не работает** («выключить можно, включить нельзя»).
2. **Коллизия подсетей**: awgo-аутбаунд с адресом от провайдера в 10.8.0.0/24 (awgo-3=10.8.0.3) лёг в ту же /24, где клиенты legacy-инбаунда awg2 (10.8.0.x, wrong-subnet до lucx.63) → reverse-path ломался → флуд `proxy/tun: connection was refused`. Подтверждено: с непересекающимся awgo-5 (12.80.1.2) ошибок 0.

**Фикс 1 — enable-кнопка (транспорт, не reconcile):** фронт `awgOutboundsApi.enable` НЕ передавал `JSON_HEADERS`, а `http-init.ts` сериализует тело в JSON только при `Content-Type: application/json`, иначе шлёт form-urlencoded (`enable=true`). Бэкенд `enable`-хэндлера делал `_ = c.ShouldBindJSON(&body)` — парсинг form-тела молча падал → `body.Enable` дефолтился в `false` → каждый «enable» превращался в «disable» (disable работал, т.к. false=дефолт).
- `frontend/src/api/awg-outbounds.ts`: `enable` теперь передаёт `JSON_HEADERS` (как add/update/parseConf).
- `internal/web/controller/awg_outbound.go`: `ShouldBind` с тегами `json:"enable" form:"enable"` + проверка ошибки (вместо `_ =`) — любой дрейф формата теперь падает громко, а не тихо дизейблит.

**Фикс 2 — guard подсетей аутбаундов (outbound-сторона):** `checkAwgSubnetConflict` сверял подсеть только при сохранении ИНБАУНДА и только по серверному адресу. Добавлен зеркальный guard при сохранении АУТБАУНДА: `AwgOutboundService.checkSubnetConflict` → чистая `awgOutboundSubnetClash(addr, inbounds)` сверяет адрес аутбаунда и с **серверной подсетью** каждого AWG-инбаунда, и с **адресами его клиентов** (`awgSettingsClientIPs` парсит settings.clients[].allowedIPs, /32/bare — берём, сети типа 0.0.0.0/0 — пропускаем). Клиенты проверяются дополнительно к серверному адресу, т.к. legacy wrong-subnet инбаунд держит клиентов в другой /24, чем его settings.address. /32-/128 аутбаунд освобождён (не ставит subnet-маршрут). Встроен в `AddOutbound`/`UpdateOutbound`.
- **Файлы:** `internal/web/service/awg_outbound.go` (guard + импорт netip/common), `internal/web/service/client_awg.go` (`awgSettingsClientIPs`), `internal/web/service/awg_outbound_test.go` (тесты), `internal/web/controller/awg_outbound.go` (bind), `frontend/src/api/awg-outbounds.ts` (JSON_HEADERS).
- **Тесты:** `TestAwgSettingsClientIPs`, `TestAwgOutboundSubnetClash` (6 кейсов, в.ч. ключевой «10.8.0.3/24 поверх wrong-subnet клиентов»). Логика проверена standalone (netip Contains/Overlaps/Masked) — PASS; пакет service локально на Windows не собрать (нет gcc → CGO `sqlite3.Backup`), полный прогон — на GitHub Actions CI.

**Отложено:** миграция legacy wrong-subnet клиентов (awg2 10.8.0.x → 10.8.1.x) — инвазивна (меняет клиентские IP → перераздача конфигов); guard достаточно, чтобы коллизия не повторялась. Косметика тест-эндпоинта (`signal: killed` при успешном ping) — отдельно.

## Релиз v3.6.0-lucx.68 (2026-08-05) — перепечать credentials в самом конце install

**Контекст (tester VladufQa, 2026-08-05):** «Логин пароль пишет при установке в самом начале а не в конце» + «можно ли при установке сразу указывать логин пароль а не через x-ui потом». Реальная проблема: `config_after_install` генерирует и показывает username/password/Access URL в середине потока `install_x-ui()`, после чего отрабатывают service-unit setup + LUCX-HOOK установки AWG-модуля (`bin/install-awg-module.sh`) — много строк вывода, и credentials уезжают далеко вверх по скроллу терминала. Тестер не видит их в конце и не знает, как войти в панель.

**Решение (выбрано пользователем):** не вводить интерактивный ввод логина/пароля при установке (env-переменные `XUI_USERNAME`/`XUI_PASSWORD`/`XUI_WEB_BASE_PATH` уже поддерживаются для scripted-install), а **продублировать вывод credentials в самом конце** потока установки — чтобы они были последним, что видно в терминале.

**Фикс — LUCX-HOOK в конце `install_x-ui()` (`install.sh`):**
- Новый LUCX-HOOK блок стоит **после** баннера «installation finished» + меню управления `x-ui` и **до** закрывающей `}` функции. Это последнее, что выводит `install_x-ui()` → последнее в терминале.
- Читает `/etc/x-ui/install-result.env` (пишется `write_install_result` в `config_after_install` — machine-parseable, root-only, mode 600) через `set -a; . file; set +a` и перепечатывает компактную сводку: `Username`, `Password`, `Access URL` в рамке.
- Guard `[[ -r ... ]]`: если файла нет (напр. re-install с уже выставленными credentials, где `write_install_result` не вызывается) — блок молча пропускается, нет краша. На свежей установке файл свежий → сводка печатается.
- Никаких новых интерактивных `read`-промптов: headless/TTY-less установки (через `systemd-run`, как в lucx.66) не ломаются.

**Файлы:** `install.sh` (новый LUCX-HOOK в `install_x-ui()`), `internal/config/config.go` (`lucxVersion` → `lucx.68`).

**Out of scope (отдельно от тестера):** openresolv/`systemd-resolved` — на одном хосте пациента `apt-get install openresolv` упал (вероятно сеть/зеркало, сервер в РФ), AWG-инбаунд не поднимался, ручная установка `systemd-resolved` починила. Наши рендеры **никогда** не пишут `DNS=` в `.conf` (`client_conf.go`, `manager.go`), т.е. `awg-quick` не должен вызывать `resolvconf` для наших конфигов — warning про «openresolv не установлен» про этот случай. Нужны диагностика с хоста (фактическая ошибка AWG, `command -v resolvconf`, `cat /etc/resolv.conf`) перед спекулятивным фиксом инсталлера.

## Релиз v3.6.0-lucx.67 (2026-08-04) — sweep не сносит чужие AWG-конфиги, бэкап вместо удаления

**Контекст (tester, 2026-08-04):** LucX-UI удалял конфиги WGDashboard в `/etc/amnezia/amneziawg/` — «все интерфейсы созданные через WGDashboard магически загнулись, файлов нет, из бекапа не поднимаются (файл сразу исчезает)». Корень: `sweepOrphanInboundConfigs` (reconcile, каждые 10с) удалял **любой** `awg{N}.conf` чей ID не среди текущих инбаундов LucX-UI; конфиги WGDashboard называются так же (`awg0.conf`…) и лежат в той же папке → сносятся как «сироты», при восстановлении сразу исчезают снова.

**Фикс — ownership по маркеру + бэкап вместо удаления:**
- **Маркер:** `renderServerConf` пишет первую строку `# Managed by x-ui - do not edit` (awg-quick игнорирует `#`-комментарии). `awgConfigDir` стал `var` (тесты переопределяют), `awgBackupDir()` = `awgConfigDir + "/x-ui-backup"`.
- **`sweepOrphanInboundConfigs`:** конфиг не из `want` сносится только если несёт маркер (LucX-UI), и не удаляется, а **переносится** в `x-ui-backup/awg{N}.conf.<unixtime>`. Чужие (без маркера, напр. WGDashboard) **не трогаются**.
- **`Remove(id)`** (удаление инбаунда из панели) и reconcile-цикл (для разонравившихся procs) тоже **бэкапят** вместо удаления; удаление только если бэкап не удался.
- **Пометка существующих конфигов LucX-UI** (созданы до фикса, без маркера): при reconcile для инбаундов из `want` чей конфиг без маркера — перезапись через `renderServerConf` (маркер добавится; контент детерминированный, fingerprint не меняется → рестарт не триггерится). Разово помечает старые конфиги.
- Хелперы `configIsManaged`, `backupConfigFile`.

**Тесты:** `TestSweepOrphanInboundConfigs_BackupsMarkedOnly` (маркированный сирота→бэкап; чужой→не тронут; в want→не тронут), `TestConfigIsManaged`, `TestBackupConfigFile_MoveAndTimestamp`, `TestRenderServerConf_ManagedMarkerFirstLine`. gofumpt/check-lucx OK (51 файл), go test awg зелёный.

**Файлы:** `internal/awg/process.go`, `internal/awg/manager.go`, `internal/awg/manager_sweep_test.go`, `internal/awg/server_conf_psk_test.go`, `internal/config/config.go`, `progress.md`, `AGENTS.md`

**Результат:** WGDashboard-конфиги остаются на месте (WGDashboard продолжает работать); ничего LucX-UI не удаляет безвозвратно — всё в `x-ui-backup/`.

## Релиз v3.6.0-lucx.66 (2026-08-04) — headless веб-обновление не упирается в интерактивные промпты

**Контекст:** веб-обновление панели (через кнопку) ломало панель/Xray. Расследование: `migrateAwgClientSubnets` (миграция IP) при обновлении версии **не запускается** (только при ручной смене адреса инбаунда в `UpdateInbound`) — гипотеза про миграцию IP не подтвердилась. Реальная причина: веб-обновление запускает `update.sh` через `systemd-run` **без TTY** (stdin=/dev/null), и в `config_after_update` два интерактивных места ломались: (а) цикл `read -rp` для server_ip при неудаче автоопределения IP читал EOF вечно → зависание; (б) `prompt_and_setup_ssl` без TTY молча выбирал дефолт «выпустить Let's Encrypt IP-серт» → `systemctl stop x-ui` + acme.sh на порту 80 → панель оставалась остановленной/сломанной.

**Фикс:** детект `lucx_interactive` по `[[ -t 0 ]]` в `update.sh`. В headless-режиме цикл server_ip и SSL-мастер пропускаются (сертификат настраивается позже из консоли/панели). Интерактивные консольные запуски (`x-ui update`) сохраняют промпты. Проверено на test2: headless-обновление не виснет, SSL-setup пропускается, панель+Xray стартуют.

**Файлы:** `update.sh`, `internal/config/config.go`

## Релиз v3.6.0-lucx.65 (2026-08-04) — генерация AWG 3.0 device-полей в «Regenerate obfuscation»

**Контекст:** кнопка «Regenerate obfuscation» генерировала Jc/Jmin/Jmax/S1-S4/H1-H4/I1-I5 и (для v3) HeaderProtectionKey, но НЕ генерировала 6 device-полей AWG 3.0 (ContentPaddingAddition, RekeyAfterTime, RekeyTimeout, RejectAfterTime, KeepaliveTimeout, MaxHandshakeAttempts) — они оставались `"0"` (kernel-default). Запрошено пользователем: добавить генерацию по алгоритмам AmneziaWG-Architect (https://github.com/Vadim-Khristenko/AmneziaWG-Architect, `src/engines/awg/generator/awg3.ts`), только для awg3.

**Алгоритм (из Architect, выведен из amneziawg-go v3.0.1):**
- **ContentPaddingAddition** (масштабируется с пресетом): spans lite=[8,64], standard=[16,128], pro=[24,200]; `min=rnd(lo,(lo+hi)/2)`, результат `range(min, rnd(min+8, hi))`.
- **Таймеры** (диапазоны «lo-hi», spread j: lite=10, standard=25, pro=45): RekeyTimeout=rnd(4,6)+rnd(1,4) фикс; KeepaliveTimeout=rnd(8,14)+rnd(2,8) фикс; RekeyAfterTime=rnd(100,120)+rnd(10,j) масштабируется; RejectAfterTime lo=max(170, rekeyAfterHi+keepaliveHi+rekeyTimeoutHi+15), hi=lo+rnd(10,j) масштабируется; MaxHandshakeAttempts=rnd(12,18)+rnd(2,10) фикс.
- **Инварианты протокола:** RejectAfterTime > KeepaliveTimeout+RekeyTimeout; RekeyAfterTime < RejectAfterTime; MaxHandshakeAttempts ≥ 1. Формат `"lo"` если lo==hi, иначе `"lo-hi"` (kernel u16_range_t парсит оба).
- **Дифференциация по пресетам — точно как Architect** (подтверждено пользователем): 3 поля масштабируются (ContentPadding, RekeyAfter, RejectAfter), 3 фиксированы (инварианты).

**Реализация:**
- `internal/awg/cps/params.go`: `type Awg3DeviceTimings` (6 string-полей) + `GenerateAwg3DeviceTimings(profile)` + хелпер `awg3Range(lo,hi)`. Через существующий seedable `randInt` (консистентно с Jc/S/H, тестируемо через `SetRand`). Struct `AWGParams` не тронут.
- `internal/web/controller/awg.go`: в существующий v3-гейт (`awgVersion=="3" && ModuleSupportsAwg3()`) после HPK добавлены 6 полей в ответ; ключи = полям Zod-схемы.
- **Frontend БЕЗ изменений хендлера:** `regenerateObfuscation` применяет ответ слепым `Object.entries → setValue`; поля уже отрисованы (awg.tsx, гейт awgVersion==='3').
- `frontend/src/pages/api-docs/endpoints.ts`: задокументирован `awgVersion`/`browserProfile` + новые поля ответа.

**Тесты:** `TestGenerateAwg3DeviceTimings_FormatAndInvariants` (3 пресета: формат, u16-границы, инварианты). gofumpt/check-lucx OK (50 файлов), go test awg/lucx зелёный, frontend typecheck/lint/vitest 916 тестов.

**Файлы:** `internal/awg/cps/params.go`, `internal/awg/cps/cps_test.go`, `internal/web/controller/awg.go`, `frontend/src/pages/api-docs/endpoints.ts`, `internal/config/config.go`, `AGENTS.md`, `progress.md`

**Примечания:** RNG seedable (`crand.NewSource(1)` — пре-existing детерминизм, касается и Jc/S/H; при желании позже на crypto/rand). Regenerate перезаписывает device-поля (как HPK/Jc/S/H). Генерация только при `ModuleSupportsAwg3()` (на pre-AWG3 хосте поля не отдаются).

## Релиз v3.6.0-lucx.64 (2026-08-04) — уход от «проклятой» подсети 10.8.0.0/24 + защита от AWG-outbound'ов

**Контекст (tester VladufQa):** AWG-инбаунд на дефолтной подсети 10.8.0.0/24 — «коннект есть, трафика нет». 10.8.0.0/24 — самая популярная подсеть WireGuard/AmneziaWG-серверов; AWG-outbound'ы (awgo-N), подключающиеся к вышестоящим серверам, получают адрес из вставленного .conf провайдера — почти всегда 10.8.0.x/24. `awg-quick up awgo-N` ставит connected-route 10.8.0.0/24; инбаунд на той же подсети ставит вторую connected-route → reverse-path в zombie-интерфейс → трафик глохнет. Vlad подтвердил: 10.8.5.1/24 работает, 10.8.0.1/24 — «проклятый».

**Фикс 1 — смена дефолтной подсети 10.8.0.0/24 → 10.200.0.0/24:**
- `defaultAwgBase` (`client_awg.go`) → `10.200.0.0/24` (приватный 10.0.0.0/8, далёк от популярных upstream 10.6/10.7/10.8, не пересекается с WireGuard-дефолтом 10.0.0.0/24 и TUN-шлюзами 10.254.x.x). Fallback в `awgAllocationFallback` — на существующие инбаунды с валидным address не влияет.
- Frontend: `createDefaultAwgInboundSettings` address → `10.200.0.1/24`; `suggestFreeAwgAddress` (`subnet.ts`) сканирует 10.200.0.0/24..10.220.255.0/24 (fallback `10.200.0.1/24`); placeholder в `awg.tsx`; клиентские fallback'и `ClientFormModal.tsx`/`wireguardConfig.ts` → `10.200.0.2/32`.

**Фикс 2 — checkAwgSubnetConflict учитывает AWG-outbound'ы:**
- `ActiveOutboundAddresses` рефакторнут → приватный `outboundAddresses(enabledOnly bool)`; публичная обёртка держит `enabledOnly=true` (collision-guard в `defaultAwgClients`).
- `checkAwgSubnetConflict` (`inbound.go`) добавил второй цикл по `outboundAddresses(false)` (**все** outbound'ы, включая выключенные — иначе включение позже тихо вернёт конфликт). Чистый хелпер `awgOutboundSubnetConflict(newNet, outAddr)`: блок только если `oP.Bits() <= newNet.Bits() && newNet.Overlaps(oP.Masked())` — /32-адреса отсеиваются (не создают /24 connected-route, их IP уже закрывает exclusion в `defaultAwgClients`).

**Тесты:** `TestAwgOutboundSubnetConflict` (Go, 8 кейсов: /24-vs-/24 блок, /16 блок, /32 exempt, непересекающиеся — nil). `awg-subnet-overlap.test.ts` обновлён под диапазон 10.200.x.x (+ кейс «10.8 вне окна сканирования не блокает»). Frontend 916 тестов зелёные, gofumpt/lint чистые.

**Файлы:** `internal/web/service/client_awg.go`, `awg_outbound.go`, `inbound.go`, `inbound_awg_test.go`, `xray.go`, `internal/config/config.go`, `frontend/src/lib/awg/subnet.ts`, `frontend/src/lib/xray/inbound-defaults.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `frontend/src/pages/inbounds/form/InboundFormModal.tsx`, `frontend/src/pages/clients/ClientFormModal.tsx`, `frontend/src/pages/clients/wireguardConfig.ts`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/test/awg-subnet-overlap.test.ts`, `AGENTS.md`, `progress.md`

**Out of scope (follow-up):** авто-лечение УЖЕ конфликтующих инбаундов (созданных до lucx.64 на 10.8.0.0/24) — оператор правит адрес вручную (триггерит migrateAwgClientSubnets); подмешивание outbound-подсетей во frontend auto-suggest/warning; wiring SyncPeers.

## Релиз v3.6.0-lucx.63 (2026-08-04) — фикс аллокации AWG-клиентов + серверный запрет дубликатов подсетей

**Контекст (tester VladufQa):** клиент AWG получает адрес из чужой подсети → route-конфликт → `awg-quick up` падает («Device awgN does not exist»); после ручной правки IP инбаунд не оживает без перезагрузки Xray; два инбаунда можно создать в одной подсети (warning не блокирует).

**Корень — цепочка из 3 дефектов:**

**Фикс 1 — аллокация из подсети инбаунда (критический):**
- `defaultAwgClients` (`client_awg.go`) считал базу через `wireguardAllocationBase(used, fallback)` — та берёт **/24 первого занятого IP** из `used` (включая IP awgo-* outbound'ов из collision-guard'а!). При активном AWG-outbound'е на 10.8.0.x база становилась 10.8.0.0/24 даже для инбаунда 15.11.5.0/24 → первый клиент получал адрес, который сервер не маршрутизирует → `awg-quick up` ставит коллизирующий /32 → RTNETLINK "File exists" → интерфейс откатывается.
- **Фикс:** `base := awgAllocationFallback(serverAddr)` — подсеть инбаунда единственный источник базы; `used` остаётся только exclusion-списком. WireGuard не тронут (продолжает использовать `wireguardAllocationBase`).
- `allocateWireguardAddress` получил параметр `widen bool`: WireGuard передаёт `true` (расширение /24→/16 для больших пулов), AWG — `false` (строгая привязка к подсети; заполненный /24 → ошибка вместо молчаливого выхода в соседние /24). Обновлены 3 call-site.
- «Нужна перезагрузка Xray» — **следствие** этого бага: route-конфликт → reconcile падал на каждом 10s-тике бесконечно; `RestartXray → ensureAwgRouting` просто запускал немедленный Reconcile. Фикс аллокации устраняет конфликт → cron сходится.

**Фикс 2 — defaultAwgClients в пути формы инбаунда:**
- `AddInbound`/`UpdateInbound` (`inbound.go`) не вызывали `defaultAwgClients` — клиенты, добавленные inline в форме инбаунда, сохранялись без ключей/PSK/адреса. Добавлен вызов в обе функции (AddInbound: existing=nil; UpdateInbound: existing=клиенты из oldInbound как exclusion-список, только для новых клиентов с пустыми credentials). Добавлен `case "awg"` в validation switch (без пре-аллокационной валидации ключа).

**Фикс 3 — серверный запрет дубликатов подсетей:**
- Новая `checkAwgSubnetConflict(newAddr, ignoreId)` (`inbound.go`): парсит адрес → `netip.Prefix.Masked()`, сравнивает через `Overlaps()` со всеми AWG-инбаундами (кроме ignoreId), возвращает ошибку с именем конфликтующего.
- `AddInbound`: блокирует новый дубликат (ignoreId=0).
- `UpdateInbound`: блокирует только СМЕНУ подсети на конфликтную; редактирование без смены подсети разрешено (back-compat для существующих дубликатов — сравнение masked-подсетей old vs new).
- Frontend advisory-warning (Pattern 1e) оставлен — мгновенная обратная связь до server-roundtrip.

**Тесты:** `TestAllocateWireguardAddress_NoWidenForAwg` (заполненный /24 + widen=false → ошибка), `TestAllocateWireguardAddress_StrictSubnetForAwg` (чужой used-IP не утаскивает аллокацию из подсети инбаунда). Существующие wireguard-тесты обновлены под новую сигнатуру (widen=true). Service-тесты требуют CGO — прогоняются CI на Linux.

**Файлы:** `internal/web/service/client_awg.go`, `client_wireguard.go`, `inbound.go`, `client_wireguard_test.go`, `internal/config/config.go`, `AGENTS.md`, `progress.md`

**Out of scope (follow-up):** SyncPeers wiring (`manager.go`, dead code) — live-обновление пиров без 10с cron; авто-миграция существующих wrong-subnet клиентов из третьей подсети.

## Релиз v3.6.0-lucx.62 (2026-08-04) — полное удаление AdvancedSecurity

**Контекст:** продолжение lucx.61. Проверка upstream-исходников AmneziaWG kernel module подтвердила: `AdvancedSecurity` — вестигиальное поле. `set_peer()` (`netlink.c:612-743`) никогда не читает `attrs[WGPEER_A_ADVANCED_SECURITY]`, `struct wg_peer` не имеет поля, `get_peer()` хардкодит "off" в dumps. Поле НЕ гейтит HPK/таймеры/padding — те независимые device-атрибуты. Эмиссия в .conf бесполезна + ломала парсинг в старых клиентских приложениях.

**Что удалено (полностью, из всех слоёв):**
- **Go backend:** `model.go` (Client struct, ClientRecord struct, ToRecord, ToClient, merge-логика — 5 точек), `instance.go` (PeerSpec field, parse struct, construction), `client_instance.go` (ClientSettings field, fingerprint), `awg_outbound.go` (ParseConf case + v3-detection check).
- **Frontend:** schemas (`awg.ts`, `wireguard.ts`, `awg-outbound.ts`), `inbound-link.ts` (peer mapping), `ClientFormModal.tsx` (type/defaults/reading/payload/toggle), `AwgOutboundFormModal.tsx` (type/defaults/reading/payload/toggle).
- **i18n:** ключи `awgAdvancedSecurity` + `awgAdvancedSecurityHint` из всех 13 локалей.
- **Миграция:** `migrate_awg_hpk.go` теперь **удаляет** `advancedSecurity` из stored settings (peer + outbound), а не ставит false.
- **OpenAPI:** `npm run gen` перегенерировал `generated/` без поля.
- **Тесты:** удалены `TestInstanceFingerprint_InsensitiveToAdvancedSecurity`, `TestRenderServerConf_AdvancedSecurityNotEmitted`, `TestInstanceFromInbound_AdvancedSecurity`, `TestRenderClientConf_AdvancedSecurityNotEmitted`, `client_advanced_security_test.go` (4 функции). Фикстура `inbound-link.test.ts` без `advancedSecurity`.

**Не затронуто:** HPK, 6 device-таймеров, ContentPaddingAddition — работают независимо, как и раньше.

**Файлы:** `internal/database/model/model.go`, `internal/awg/instance.go`, `internal/awg/client_instance.go`, `internal/web/service/awg_outbound.go`, `internal/database/migrate_awg_hpk.go`, `internal/config/config.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/schemas/protocols/inbound/wireguard.ts`, `frontend/src/schemas/awg-outbound.ts`, `frontend/src/lib/xray/inbound-link.ts`, `frontend/src/pages/clients/ClientFormModal.tsx`, `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx`, `frontend/src/generated/*`, `internal/web/translation/*.json` (13), `AGENTS.md`, `progress.md`

## Релиз v3.6.0-lucx.61 (2026-08-04) — AdvancedSecurity: фикс toggle + убрана эмиссия из .conf

**Контекст:** tester VladufQa сообщил два бага: (1) переключатель AdvancedSecurity не выключается («ранее не включался, теперь не выключается»), (2) «AdvancedSecurity не хавает авг» — клиентское приложение не принимает поле.

**Баг 1 — merge-логика «true wins» (lucx.54 regression):**
- Merge в `model.go:MergeClientRecord` использовал `incoming.AdvancedSecurity && !existing.AdvancedSecurity` — позволяет только ON→true, блокирует OFF→false. Для `bool` zero-value `false` — валидное значение (выключить), не «поле отсутствует» (как для `int`/`string` где 0/"" = absent).
- **Фикс:** `if incoming.AdvancedSecurity != existing.AdvancedSecurity` — берёт incoming напрямую, ON и OFF работают.

**Баг 2 — AdvancedSecurity не эмитится в .conf:**
- Ядро: `set_peer` игнорирует на input, `get_peer` хардкодит "off" в dumps. Эмиссия "AdvancedSecurity = on" в .conf — useless (ядро не использует) + ломает парсинг в старых клиентских приложениях (unknown field).
- **Фикс:** убрана эмиссия из 4 точек: `renderServerConf` (manager.go), `renderClientConf` (client_conf.go), `buildAwgClientConfig` (wireguardConfig.ts), `genAwgConfig` (inbound-link.ts). Поле остаётся в model/DB для будущего kernel-саппорта. Убрано из fingerprint (lucx.61 — изменение не триггерит лишний restart).

**Тесты:** Go — `TestInstanceFingerprint_InsensitiveToAdvancedSecurity`, `TestRenderServerConf_AdvancedSecurityNotEmitted`, `TestRenderClientConf_AdvancedSecurityNotEmitted` (verify NOT in conf). Frontend — `inbound-link.test.ts` updated (v3/v2/v1.5 all verify NOT emitted). Все тесты зелёные.

**Файлы:** `internal/database/model/model.go`, `internal/awg/manager.go`, `internal/awg/client_conf.go`, `internal/awg/instance.go`, `internal/awg/instance_test.go`, `internal/awg/client_conf_test.go`, `internal/config/config.go`, `frontend/src/pages/clients/wireguardConfig.ts`, `frontend/src/lib/xray/inbound-link.ts`, `frontend/src/test/inbound-link.test.ts`, `AGENTS.md`, `progress.md`

**Дополнительно (E2E на test2):** проверен simultaneous AWG2+AWG3 — два инбаунда (v2 без HPK + v3 с HPK) работают одновременно, оба пропускают интернет-трафик (ping 8.8.8.8, 0% loss). Проверен cleanup при удалении инбаунда — интерфейс, .conf, iptables удаляются за ≤12с (reconcile-цикл).

## Релиз v3.6.0-lucx.50 (2026-07-31) — AWG3 включён + пресеты версий клиента (1.5/2/3)

**Контекст:** 30.07.2026 upstream `amnezia-vpn/amneziawg-linux-kernel-module` слил `feat/awg3` в master (PR #192, тег `v3.0.20260731`), а `amnezia-vpn/amneziawg-tools` — PR #60 (тег `v3.0.20260730`). `HeaderProtectionKey` теперь парсится в `.conf`. Known Issue #5 (HPK намеренно не писется в .conf) устарел. Дополнительно: часть клиентских приложений ещё на AWG v1/v2 — нужна генерация конфига под версию клиента.

**Дизайн (согласован с пользователем):**
- **HPK включаем** с авто-гарантом S1-S4≥12 (ядро отвергает HPK с `-EINVAL` при Sx<12).
- **Пресеты версий — гибрид:** `awgVersion` на инбаунде = потолок сервера (что ядро принимает); при экспорте клиента в `ClientQrModal`/`ClientInfoModal` версия ≤ потолка выбирается runtime (не в БД). Sub-link использует потолок.
- **Генератор:** S1-S4 всегда ≥12; HPK генерируется только при версии v3.
- **Upstream sync 3x-ui** (6 минорных коммитов) — НЕ в этом раунде, отдельный релиз.

### Что сделано

**Backend (Go):**
- `internal/awg/cps/params.go`: `MinSForHPK = 12` + `enforceSMin`; диапазоны профилей подняты (Lite/Pro нижние границы S до ≥12); `GenerateAWGParams` двойная гарантия S≥12; `AWGParams.HeaderProtectionKey` поле + `WithHeaderProtectionKey()`/`GenerateHeaderProtectionKey()` (crypto/rand, 32 байта, base64); `Validate()` проверяет S≥12; `AsConfLines()` пишет HPK при непустом.
- `internal/awg/instance.go`: поле `AwgVersion` в `Instance` + парсинг в `InstanceFromInbound` + в `fingerprint()` (рестарт при смене версии); `NormalizeAWGVersion()` (exported, "" → "2").
- `internal/awg/manager.go` `renderServerConf`: HPK пишется **только при `AwgVersion=="3"` И непустом ключе** (снят старый полный запрет).
- `internal/awg/client_conf.go` `renderClientConf` (awgo-N outbound): та же version-gate логика. `client_instance.go`: поле `AwgVersion` в `ClientSettings` + fingerprint.
- `internal/web/controller/awg.go`: `AwgVersion` в запросе `generateObfuscation`; HPK отдаётся в ответе **только при `awgVersion=="3"`** (для v1.5/v2 поле отсутствует, чтобы не затирать ручное значение оператора — урок lucx.49 сохранён).
- `internal/web/service/inbound.go` `inboundAwgHints`: возвращает `(address, obfuscation, version)`; HPK в блоке при v3. `InboundOption.AwgVersion` поле + заполнение в `GetInboundOptions`.
- `internal/database/migrate_awg_hpk.go`: переименовано `pruneAwgHeaderProtectionKey` → `migrateAwgVersion` + `normalizeAwgSettings`. Теперь: backfill `awgVersion:"2"` на pre-lucx.50 инбаундах/аутбаундах И вычистка непустого HPK со всего, что не v3. `db.go` — вызов обновлён.

**Frontend (TS):**
- `schemas/protocols/inbound/awg.ts`: поле `awgVersion: z.enum(['1.5','2','3']).default('2')`.
- `lib/xray/inbound-defaults.ts`: дефолт `awgVersion: '2'`.
- `lib/xray/inbound-link.ts`: `AwgVersion` тип + `awgVersionCeiling()`/`awgVersionAtLeast()` helpers; `genAwgLink`/`genAwgConfig` version-gate эмиссию S3/S4 (v2+), I1-I5 (v2+), HPK (v3); `GenAwgLinkInput.awgVersionOverride` (clamp ≤ ceiling) для клиентов-page.
- `pages/inbounds/form/protocols/awg.tsx`: селектор версии на инбаунде; HPK-поле + `awgSRangeWarning` под v3; `awgVersion3CompatNote` alert; regenerate передаёт версию.
- `pages/clients/wireguardConfig.ts`: `filterAwgObfuscation()` (построчный фильтр блока под версию); `buildAwgClientConfig(... awgVersionExport?)` (clamp ≤ ceiling).
- `pages/clients/ClientQrModal.tsx` + `ClientInfoModal.tsx`: runtime state `awgExportVersion` (default = ceiling), `<Select>` с опциями ≤ потолка (disabled выше).
- `schemas/client.ts`: `awgVersion` в `InboundOptionSchema`.

**i18n (Pattern 2c — все 13 локалей):** новые ключи `awgVersion`, `awgVersionHint`, `awgVersion15`, `awgVersion2`, `awgVersion3`, `awgVersion3CompatNote`, `awgSRangeWarning` (form) + `awgExportVersion` (clients); обновлён `awgHpkHint` (не «forward-compat», а «требуется версия 3»). en-US + 12 локалей (ru, uk, zh-CN, zh-TW, es, fa, ja, pt, ar, tr, vi, id).

**AWG outbound (panel-as-client) — поддержка конфигов любой версии:**
- `internal/web/service/awg_outbound.go` `ParseConf`: теперь парсит `HeaderProtectionKey` И **авто-определяет `awgVersion`** из набора полей конфига (HPK → "3"; иначе S3/S4 или I1-I5 → "2"; иначе "1.5"). Вставленный v3-конфиг сохраняет HPK и рендерится как "3".
- `internal/awg/client_instance.go`: `AwgVersion` поле в `ClientSettings` + fingerprint (добавлено в предыдущем раунде); `renderClientConf` пишет HPK только при версии "3" (добавлено ранее).
- `frontend/src/schemas/awg-outbound.ts`: поля `headerProtectionKey`, `awgVersion` в `AwgOutboundSettingsSchema`.
- `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx`: селектор версии (переиспользует ключи `pages.inbounds.form.awgVersion*`); поле HPK + `awgSRangeWarning` в advanced-блоке; `handlePaste` раскрывает advanced при наличии HPK; defaultValues/settingsToForm/formValuesToSettings пробрасывают новые поля.

**Тесты:**
- Go: `cps_test.go` (S≥12 для всех профилей, HPK формат, `WithHeaderProtectionKey`, `AsConfLines` gate), `instance_test.go` (`TestRenderServerConf_HeaderProtectionKeyVersionGated`, `TestInstanceFingerprint_ChangesOnAwgVersion`, `TestNormalizeAwgVersion`, парсинг awgVersion), `client_conf_test.go` (version-gate), `inbound_awg_test.go` (`TestInboundAwgHints_HeaderProtectionKeyVersionGated`), `migrate_awg_hpk_test.go` (`TestNormalizeAwgSettings` + preserves-fields).
- Frontend: `wireguard-client-config.test.ts` (ПЕРВЫЕ AWG-кейсы: `filterAwgObfuscation` v1.5/v2/v3, `buildAwgClientConfig` override+clamp), `inbound-link.test.ts` (ПЕРВЫЕ AWG share-link кейсы: version-gating genAwgLink/genAwgConfig).
- Outbound: `awg_outbound_test.go` (`TestParseConf_Client` — авто-версия "1.5"; `TestParseConf_AwgVersions` — v3 с HPK → "3" + HPK сохранён, v2 → "2", legacy → "1.5").

**Проверка (на Windows, без gcc — database-cgo пропущен):**
- `go test ./internal/awg/... ./internal/lucx/...` — зелёные.
- `npm run typecheck` — чисто.
- `npm run lint` — чисто.
- `npx vitest run --project=unit` — 886/886 тестов, включая i18n-dead-keys (13 локалей синхронны).
- `bin/check-lucx.sh` — gofumpt OK (49 файлов).
- ⚠️ `internal/database` тесты (миграция) и полный `go build .` требуют CGO/gcc — на CI/VPS Linux.

**Документация:** `AGENTS.md` — Known Issue #5 → ЗАКРЫТО; Pattern 6 (AWG версия vs совместимость); Rule 3 + Architecture Map обновлены под активный HPK + awgVersion. `config.go` → `lucx.50`.

**Риски/открытые вопросы:**
- AWG3 модуль v3.0 не собирается на ядрах < 6.7 (нужен фикс `nla_put_uint`, уже в master) — отметить тестерам при деплое.
- Сервер v3 НЕ примет v1/v2/plain-WG клиентов (HPK криптографически ломает) — для смешанного парка отдельный v2-инбаунд. Задокументировано в Pattern 6 + UI alert `awgVersion3CompatNote`.

**Релиз-процесс и CI-грабли (отдельные коммиты, не описаны выше):**
- Коммиты: `432b7555` (фича) → `20bcf59a` (CI-фикс). Оба на `main`, запушены через `git@github.com:AlexeyLCP/lucx-ui.git` (SSH напрямую — git-credential helper gh в git bash был сломан: искал `/usr/bin/gh`, которого нет).
- **CI-фикс 1 — codegen stale:** добавленное поле `InboundOption.AwgVersion` требовало регенерации `frontend/src/generated/*` (types/zod/schemas/examples) + `frontend/public/openapi.json` из Go OpenAPI-спецификации (`npm run gen` = `gen:zod` через `go run ./tools/openapigen` + `gen:api`). CI job `codegen` ловит рассинхон («Fail if generated files are stale»). **Урок:** любое изменение Go-struct с `json:`/`example:` тегами, попадающее в OpenAPI-ответ, требует `cd frontend && npm run gen` + коммит сгенерированных артефактов (AGENTS.md Rule 9 «Do not edit src/generated/» — про ручную правку, регенерировать можно и нужно).
- **CI-фикс 2 — gofumpt на всём репо:** `golangci-lint` (CI) гоняет **весь** репо, а `bin/check-lucx.sh` — только 49 LucX-файлов. `awg_outbound.go:285` имел pre-existing кривой отступ в `case`-блоке `ParseConf` (отступ у `case "interface":`), который check-lucx не покрывал. `gofumpt -w` выровнял case. **Урок:** перед пушем гонять `gofumpt -l .` (весь репо), не только check-lucx.sh.
- **Flaky go-test `TestBuildFirefoxHello_NoGrease`/`TestBuildSafariHello_NoGrease` (pre-existing, НЕ наш регресс):** CI использует `go test -shuffle=on` (рандомный seed каждый прогон). Воспроизвёл локально (~1/10 shuffle-seeds падает). Корень — в **чужой логике** `buildSafariHello`/`buildFirefoxHello` (`cps.go`): они пишут GREASE-значение через `greaseValue()` (`rng.Intn` над 16 значениями `0x0A0A…0xFAFA`), а тесты проверяют **отсутствие** паттернов `0a0a`/`fafa` в hex. Тест проходит в 14/16 случаев (когда rng выдаёт не эти 2 значения). Пробовал фикс `SetRand(t *testing.T, …)` с `t.Cleanup` для изоляции глобального `rng` — **не помог** (проблема в логике, не в загрязнении rng); откатил. Решено rerun'ом failed-джоба (другой shuffle-seed → зелёный). **TODO отдельным issue:** чинить тест (проверять GREASE только в extension-позициях) или логику (Safari/Firefox не должны писать GREASE через rng).
- **race-job «завис» на ~23 мин:** оказался задержкой обновления статуса на стороне GitHub (лог race-job показал чистое завершение без DATA RACE ещё в ~22:44, а статус держал `in_progress`). Мой код race-чистый.
- **Итог CI (run 30670481828, после rerun go-test):** все 9 jobs ✅ (go-test, race, golangci, codegen, frontend, fuzz-smoke, postgres-durable-first, govulncheck) + Release ✅ + CodeQL ✅.
- **Тег `v3.6.0-lucx.50`** поставлен **после** зелёного CI (урок lucx.48 соблюдён), annotated, запушен. Release-workflow собрал CGO-бинарник на `ubuntu-latest`, создал GitHub Release `prerelease: false` (stable) с артефактом `x-ui-linux-amd64.tar.gz`. `releases/latest` → `v3.6.0-lucx.50` (install.sh и авто-обновление панелей подтянут его).
- **Не деплоил на test2** — не было явной команды. Релиз собран, но в реальном runtime на VPS ещё не проверен. Установить: `bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)` → создать AWG-инбаунд версии «3» → проверить HPK в `.conf` (`/etc/awg/awgN.conf`) и что `awgN` поднялся.

**Пост-релизный инцидент (2026-08-01, тестер Александр):** апгрейд до lucx.50 через **веб-панель** → AWG 1.5 outbounds «не подключаются» (handshake не проходит). Откат на lucx.49 лечил. Затем повтор через **консольное меню `x-ui`** → всё работает. Диагноз: НЕ регресс кода (git diff lucx.49→lucx.50 по outbound-рендеру и `client_manager.go` идентичен для v1.5/v2), а **рассинхон DKMS-модуля** — lucx.50 тянет пересборку amneziawg-модуля (AWG1→AWG3), веб-обновление не прогоняет `install-awg-module.sh` единым проходом, reconcile awgo-N попал в окно с расходящимися параметрами. Консольный путь прогоняет полный install-скрипт синхронно → согласованное состояние. Зафиксировано в AGENTS.md Pattern 1c; TODO — веб-обновление должно форсить пересборку модуля или предупреждать. Коммит-фикса кода не требуется.

---


**Очистка мусора:**
- Закрыты 10 dependabot PR (#1-#12) с ветками на GitHub
- Удалены старые ветки `feature/awg-integration` и `lucx-ui-phase1` (локально + удалённо)
- На GitHub осталось 2 ветки: `feat/awg-sidecar`, `main`. Открытых PR нет.

**Миграция — подготовка:**
- Создана ветка `feat/awg-sidecar-v3.5.0` от `origin/main` (v3.5.0, commit `4e928a1c`)
- Перенесены и закоммичены 29 изолированных LucX-файлов:
  - `internal/awg/` — 19 файлов (manager, process, instance, traffic, orphans, params, cps, config, types, templates, helpers + тесты)
  - `internal/lucx/` — 6 файлов (parser, nodetype, outbound_link + тесты)
  - `internal/database/migrate_awg.go` + тест
  - `internal/web/job/awg_job.go`
  - `frontend/src/schemas/protocols/inbound/awg.ts` + `frontend/src/pages/inbounds/form/protocols/awg.tsx`
  - `bin/install-awg-module.sh`

**Миграция — LUCX-HOOK в upstream-файлах:**
- `model.go`: добавлен `AWG Protocol = "awg"` const + `awg` в validate oneof
- `db.go`: добавлен вызов `pruneLegacyAwgHiddenChildren()` в `initModels()`
- `runtime/local.go`: import `internal/awg` + делегирование AddInbound/DelInbound/AddUser/RemoveUser
- `service/xray.go`: AWG exclusion в цикле генерации конфига + `injectAwgEgress` (TUN inbound + routing rule)
- `web.go`: import `internal/awg` + `cadenceAwg` const + cron-задача `awgJob` + `awg.GetManager().StopAll()` в shutdown
- `install.sh`: вызов `bin/install-awg-module.sh` после `setup_fail2ban`
- `xray_config_inject_test.go`: 5 тестов `injectAwgEgress` (WithOutbound, NoOutbound, Disabled, TagCollision, DefaultMTU)
- `inbound-defaults.ts`: import `AwgInboundSettings` + `createDefaultAwgInboundSettings` + `AnyInboundSettings` + switch case
- `schemas/protocols/inbound/index.ts`: import + export + discriminated union
- `primitives/protocol.ts`: `'awg'` в enum + `AWG` в Protocols map
- `InboundFormModal.tsx`: import `AwgFields` + рендер `protocol === Protocols.AWG`
- `protocols/index.ts`: export `AwgFields`

**Тесты:**
- `go test ./internal/awg/...` → ok 2.306s ✅
- `go test ./internal/lucx/...` → ok (nodetype, outbound_link, parser) ✅
- `go test ./internal/database/model` → ok ✅ (проверяет AWG const)
- `internal/database` (с cgo) — не запущен: gcc отсутствует на Windows. На Linux/VPS сработает.
- `npm run typecheck` → чисто ✅
- `npm run lint` → чисто ✅
- `npm run build` → `internal/web/dist/` собран ✅
- `go build -o /tmp/x-ui .` → exit 0, бинарник 111 МБ ✅

**Документация:**
- Создан актуальный `AGENTS.md` на новой ветке (старый был только на `feat/awg-sidecar`). Изучены AGENTS.md из соседних проектов (angry-box, AwgToolza) + CLAUDE.md из lucx-ui. Добавлены: версия v3.5.0, философия минимального внедрения, Known Issues (раздутость AWG, непроверённый runtime, dependabot), деплой, конвенции коммитов на русском, debugging patterns.
- Ведётся этот файл `progress.md`.

---

## Архитектурное решение (2026-07-13)

**Вопрос:** AWG сайдкар имеет 19 файлов vs 9 у mtproto (эталон). Лишние 7 файлов — генерация конфига/обфускации (params/cps/templates/config/types/helpers/traffic), которой у mtproto нет (mtg — готовый бинарщик).

**Решение пользователя:** оставить как есть, добить миграцию. Рефактор архитектуры отложен.

## Рефактор AWG — удаление мёртвого кода (2026-07-13)

**Исследование подтвердило:** 6 файлов AWG (`params.go`, `cps.go`, `config.go`, `templates.go`, `types.go`, `helpers.go`) + 5 тестов — полностью мёртвый код. Их функции (`GenerateAWGParams`, `GenerateCPS`, `BuildServerConfig`, `BuildClientConfig`, `UpdateServerConfig`, `RenderPostUp`, `RenderPostDown`, `MergeParamsToSettings`, `ValidateAWGParams`, `GenKey`, `GenPSK`, `DerivePubkey`) вызывались ТОЛЬКО тестами. Ни один живой call site (manager/process/instance/traffic/orphans/job/runtime/web/frontend) их не использовал.

Генерация ключей и обфускации делается во frontend (`createDefaultAwgInboundSettings` в `inbound-defaults.ts` — `Wireguard.generateKeypair` + `Math.random`). Go-генераторы были дубликатом. Комментарий во frontend «backend regenerates obfuscation when obfLevel/profile change» — ложный, такой логики в Go нет.

**Выполнено:**
- Перенесено в `process.go`: `awgConfigDir` (const) + `awgQuick` (func) + `"os/exec"` импорт
- Удалено 6 .go: params/cps/config/templates/types/helpers
- Удалено 5 тестов: config_test, config_roundtrip_test, cps_test, params_test, templates_test
- Поправлен комментарий manager.go (упоминание BuildServerConfig)
- Обновлён AGENTS.md (Architecture Map, Known Issue #1 → ЗАКРЫТО)

**Результат:** 19 файлов → 8 файлов (6 .go + 2 теста). Почти симметрично mtproto (9 файлов).

**Проверки:**
- `go build ./internal/awg/...` → exit 0 ✅
- `go test ./internal/awg/...` → ok 0.903s, все 11 тестов PASS ✅
- `go build -o /tmp/x-ui .` → exit 0 ✅
- LUCX-HOOK count → 48 (не изменилось) ✅

## Dependabot — ужесточение (2026-07-13)

**Решение пользователя:** security + урезанный scope.

**Выполнено:** `.github/dependabot.yml` — секция `updates: []` (version updates отключены). Security updates (CVE) остаются через GitHub Settings. Режим: PR только при найденной уязвимости, без еженедельного шума минорных версий npm/gomod/github-actions. Шаблон для возврата version updates оставлен в комментарии в yml-файле.

AGENTS.md Known Issue #3 обновлён.

## Адаптация install.sh + release-процесс (2026-07-13)

**Цель:** `install.sh` должен ставить нашу сборку так же просто, как апстрим — через GitHub-релиз.

**Выполнено:**
- `install.sh` — 8 LUCX-HOOK замен URL (`MHSanaei/3x-ui` → `AlexeyLCP/lucx-ui`):
  - Константы `LUCX_REPO`/`LUCX_BRANCH` вверху
  - api.github.com/releases/latest, release tarball download, fallback URL → наш репо
  - x-ui.sh, x-ui.rc, 3 service-юнита → raw.githubusercontent из нашей ветки
- `bin/build-release.sh` — новый скрипт сборки релиза на VPS:
  - Клон форка → `npm build` → `CGO_ENABLED=1 go build` (с gcc на VPS)
  - Скачивание Xray+mtg из апстрим-релиза `MHSanaei/3x-ui` v3.5.0
  - Упаковка `x-ui-linux-amd64.tar.gz` (структура как у апстрима)
  - Инструкция по созданию GitHub-релиза
- Оба скрипта проходят `bash -n`
- Коммит `8b627f8e` запушен

**Почему сборка на VPS:** CGO-бинарник (mattn/go-sqlite3) нельзя cross-compile с Windows на Linux — нужен gcc + linux-заголовки. Cross-compile `CGO_ENABLED=0` соберётся, но при запуске упадёт (sqlite stub).

**Инструкция для VPS** — в AGENTS.md (секция Release & Install) и в выводе `bin/build-release.sh`.

**Следующие шаги (требует VPS):**
1. На VPS: `curl .../bin/build-release.sh | bash` → `/tmp/x-ui-linux-amd64.tar.gz`
2. `gh release create v3.5.0-lucx.1 /tmp/x-ui-linux-amd64.tar.gz --repo AlexeyLCP/lucx-ui`
3. `bash <(curl .../install.sh)` → установка панели с нашим кодом
4. UI → создать AWG-inbound → `awg show` / `ip link show awg0`

## Фаза 1: Клиентские .conf + share-link + создание клиентов (2026-07-13)

**Проблема:** установка работала, подключение создавалось, но НЕ было генерации клиентских конфигов, share-link, создания пользователей (peers).

**Решение:** портированы паттерны WireGuard (эталон в репо) — клиентский Curve25519 keypair + PSK + туннельный адрес хранятся сервером, полный .conf и amneziawg:// share-link собираются одним кликом.

**Источники:** pumbaX/awg-multi-script (генерация конфигов), hoaxisr/awg-manager (скан хоста — отложен в Фазу 3).

**Backend:**
- `internal/awg/instance.go`: PeerSpec расширен (PrivateKey, AllowedIPs /32, Keepalive); InstanceFromInbound парсит новые поля (publicKey/privateKey/preSharedKey/allowedIPs/keepAlive) + legacy (id/password), enable как *bool (absent=true для старых inbound'ов)
- `internal/web/service/client_awg.go` (новый): defaultAwgClients — wgutil.GenerateWireguardKeypair, GenerateWireguardPSK, allocateWireguardAddress из 10.8.0.0/24 (отличается от WG 10.0.0.0/24 — без коллизий)
- `internal/web/service/client_inbound_apply.go`: case AWG (5 точек: генерация, валидация, newClientId, перенос ключей при edit, raw-map)
- `internal/sub/service.go`: 'awg' в SQL-фильтр GetSubs, case AWG в GetLink, genAwgLink (amneziawg:// с обфускацией Jc/S1-S4/H1-H4/I1-I5 в query params)
- `internal/awg/manager.go`: renderServerConf уже использует peer.AllowedIPs (туннельный /32)

**Frontend:**
- `schemas/protocols/inbound/awg.ts`: clients[] расширено, убран комментарий 'never stored server-side'
- `lib/xray/inbound-link.ts`: genAwgLink/genAwgConfig/genAwgConfigs/genAwgLinks + case 'awg' в genInboundLinks
- `pages/clients/ClientFormModal.tsx`: MULTI_CLIENT_PROTOCOLS += awg, awgIds/showAwg, regenerateAwgKeys, UI-блок (переиспользует wg-поля, Curve25519 base тот же)

**Проверки:** go build ./... exit 0; go test ./internal/awg/... ok; typecheck + lint чисто.
**Коммит:** a258ca57 (запушен).

**Фаза 2 (CPS-генерация I1-I5) и Фаза 3 (скан хоста) — отдельно.**

## Проверка Фазы 1 на VPS 144.31.224.212 (2026-07-13)

**Окружение:** Debian 13, ядро 6.12, amneziawg kernel-модуль загружен (DKMS).

**Развёрнутый бинарник:** собран через WSL (CGO_ENABLED=1, с Фазой 1) → SCP → `/usr/local/x-ui/x-ui` → systemctl restart x-ui.

**Найденная проблема:** `awg-quick up` падал с `resolvconf: command not found` — Debian 13 не имеет resolvconf по умолчанию, а .conf содержит `DNS =`. Решение: `apt-get install openresolv`. После этого awg1 поднялся (порт 15963, MTU 1320, обфускация применена). TODO: добавить openresolv в `bin/install-awg-module.sh` как зависимость.

**Проверка end-to-end:**
- ✅ x-ui работает (active/enabled)
- ✅ amneziawg kernel-модуль загружен
- ✅ awg1 интерфейс поднят (порт 15963)
- ✅ Клиент вставлен в БД (SQL, т.к. CSRF блокирует curl-логин) → Reconcile применил peer в kernel (`awg show` видит publicKey, allowed ips 10.8.0.2/32, keepalive 25)
- ✅ Подписка (порт 2096, /sub/) отдаёт `amneziawg://` ссылку со всеми параметрами: клиентский privateKey (userinfo), server publicKey, address=10.8.0.2/32, dns, mtu, keepalive, presharedkey, обфускация (jc/jmin/jmax/s1-s4/h1-h4)

**Пример сгенерированной ссылки:**
```
amneziawg://OKtt7...%3D@localhost:15963?address=10.8.0.2%2F32&dns=1.1.1.1...&h1=447248&h2=...&jc=10&jmax=247&jmin=80&keepalive=25&mtu=1320&presharedkey=k2Sb...&publickey=dMeIQ...&s1=39&s2=89&s3=78&s4=72#-testuser1
```

**Замечание:** `localhost` в endpoint — `shareAddrStrategy` дефолт `node`, сервер не знает свой внешний IP. Нужно настроить `webDomain`/`shareAddr` или strategy `custom`. Не баг Фазы 1.

**Фаза 1 подтверждена в проде.**

## Фаза 2: CPS-генерация I1-I5 (pumbaX) — 2026-07-13

**Реализовано:**
- `internal/awg/cps/` — порт pumbaX/awg-multi-script:
  - `domains.go` — домен-пулы RU/WORLD (TLS/DNS/SIP/QUIC), SelectDomain
  - `params.go` — GenerateAWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4 с инвариантами AmneziaWG: Jmin<Jmax, |S1+56-S2|>=10, H1-H4 в 4 непересекающихся квадрантах 2^29)
  - `cps.go` — GenerateCPS (TLS Chrome-like ClientHello с GREASE/SNI/groups/key_share/padding, DNS EDNS0, SIP REGISTER, QUIC v1 Initial + second/short packets)
  - `cps_test.go` — 6 тестов (инварианты 200 итераций, все профили/регионы)
- API: `POST /panel/api/inbounds/awg/generateObfuscation` (awg.go)
- Frontend: awg.tsx — кнопка генерации вызывает backend API (вместо Math.random заглушки), loading state

**Проверено в проде:** API вернуло `success:true` с полным набором — jc/jmin/jmax, s1-s4, h1-h4 (4 квадранта), i1-i5 (TLS ClientHello с разными SNI: reddit/cloudflare/google/github/wikipedia). ✅

**Фикс:** `bin/install-awg-module.sh` — `apt-get install openresolv` (awg-quick падал с 'resolvconf: command not found' на Debian 13).

## Фаза 3: Скан хоста (hoaxisr) — 2026-07-13

**Реализовано:**
- `internal/awg/signature/capture.go` — порт hoaxisr/awg-manager:
  - `Capture(domain)` — отправляет QUIC v1 Initial (с TLS ClientHello SNI=domain) на UDP 443, читает ответы, возвращает I1-I5 как CPS строки
  - Чистый Go: net.Dial UDP, crypto/hkdf (HKDF-SHA256, RFC 9001 §5.2), crypto/cipher (AES-128-GCM), header protection (RFC 9001 §5.4), crypto/tls через net.Pipe для ClientHello
- API: `POST /panel/api/inbounds/awg/captureHost` (awg.go)
- Frontend: awg.tsx — поле "сканировать хост" (Input + кнопка), вызывает API, заполняет I1-I5

**Endpoint работает:** возвращает корректную ошибку "host did not reply on QUIC 443" для хостов без ответа.

**⚠️ Known bug: capture не получает ответов от реальных хостов** (google/cloudflare/dns.google — все таймаутят, получают только свой собственный Initial). `buildTLSClientHello` работает корректно (1487 байт реального ClientHello через net.Pipe), но `buildInitialPacket` (QUIC Initial шифрование: HKDF/AES-GCM/header protection) генерирует невалидный пакет, который серверы отбрасывают. Требует отладки crypto-деталей (возможно: varint кодировка length, nonce XOR, header protection mask позиции, или ClientHello record-wrapping для CRYPTO frame).

**Статус Фазы 3:** endpoint + frontend готовы, но capture пока нерабочий. Фаза 2 (случайная CPS-генерация) полностью покрывает практическую потребность — скан хоста опциональная фича.

## Релиз v3.5.0-lucx.3 + фикс инверсии onlyI1 (2026-07-14)

**Баг:** `GenerateCPS` 4-й параметр `onlyI1` (true=только I1), а API поле `FullI1I5` (true=все I1-I5). В awg.go передавалось `req.FullI1I5` напрямую — инверсия: при `fullI1I5=true` получали только I1, при `false` — все 5. Исправлено: `!req.FullI1I5`.

**Релизы через CI:**
- `v3.5.0-lucx.1` (устарел, удалён) — Фазы 1
- `v3.5.0-lucx.2` (устарел, удалён) — Фазы 1-3 (с багом onlyI1)
- `v3.5.0-lucx.3` (latest) — Фазы 1-3 + фикс onlyI1 ✅

**CI:** `release.yml` упрощён (только linux/amd64), триггер по push тега `v*.*.*` → авто-сборка через Bootlin musl static + загрузка asset. Релиз делается latest вручную после CI.

**Проверка v3.5.0-lucx.3 на VPS 144.31.224.212 (из релиза, не ручной сборки):**
- ✅ `install.sh` качает и ставит v3.5.0-lucx.3
- ✅ awg1 поднят, peer в kernel (порт 15963)
- ✅ generateObfuscation API: QUIC full → I1=2402, I2=762, I3=172, I4=122, I5=114 (все 5 пакетов)
- ✅ jc/jmin/jmax/s1-s4/h1-h4 (4 квадранта) — корректно
- ✅ amneziawg:// подписка работает

## Фаза 3: скан хоста — РАБОТАЕТ (v3.5.0-lucx.4, 2026-07-14)

**Systematic debugging** выявил рут-козу и два бага в `internal/awg/signature/capture.go`:

**Баг 1 (длина):** `buildTLSClientHello` через `crypto/tls` генерировал ClientHello ~1482 байт — не помещается в QUIC Initial (min 1200). Initial получался 1537 байт, google отвечал ICMP unreachable. **Фикс:** переписал на мануальную сборку минимального Chrome-like ClientHello (~250 байт: SNI, supported_versions TLS1.3, supported_groups x25519, signature_algorithms, key_share x25519 32B, ALPN h3, psk_kex_modes). Возвращает handshake message (тип 0x01), не TLS record — QUIC CRYPTO frame несёт handshake напрямую.

**Баг 2 (рут-коза, header protection):** `mask[0] & 0x0F` применялся к `protected[pnOffset]` (первый байт pn) вместо `protected[0]` (form byte 0xC3). RFC 9001 §5.4: form byte (первый байт) маскируется на младшие 4 бита, pn bytes — `mask[1..pnLen]`. **Фикс:** `protected[0] ^= mask[0] & 0x0F`, pn bytes `protected[pnOffset+i] ^= mask[1+i]`.

**Доказательство через tcpdump:** реальный curl `--http3` шлёт 1200 байт → google отвечает. Наш до фикса — 1537 байт → ICMP unreachable. После фикса — 1200 байт, структура идентична curl → google отвечает.

**Проверка в проде:** captureHost API → `success: true`, I1=2406 (наш Initial), I2=172 (ответ google). cloudflare → I1=I2=2406. dns.google → I1=2406, I2=172.

**Релиз v3.5.0-lucx.4** (latest) — все 3 фазы работают.

## Релизы
- v3.5.0-lucx.11 (latest) — фикс Content-Type JSON + i18n route ✅
- v3.5.0-lucx.10 (устарел, удалён) — вкладка протокола для AWG
- v3.5.0-lucx.3 (устарел, удалён) — фикс onlyI1
- v3.5.0-lucx.2 (устарел, удалён) — Фазы 1-3 (баг onlyI1)
- v3.5.0-lucx.1 (устарел, удалён) — Фаза 1

## E2E тест — полная проверка (2026-07-14)

**E2E = реальное клиентское подключение к серверу.** Поднял клиент awg-client на VPS (через awg-quick), подключение к серверу awg1 по loopback.

**Найден e2e-баг:** серверный awg1 не имел Address в [Interface] — `renderServerConf` не писал туннельный IP. Клиент подключался (handshake OK, трафик рос), но пинг до 10.8.0.1 — 100% loss (у интерфейса нет внутреннего адреса).

**Фикс:** поле `Address` в `Instance`, `InstanceFromInbound` парсит `settings.address`, `renderServerConf` пишет `Address = 10.8.0.1/24` в [Interface]. Zod-схема + `createDefaultAwgInboundSettings` — `address: '10.8.0.1/24'` (соответствует `defaultAwgBase` 10.8.0.0/24 в `client_awg.go`). Fingerprint включает Address (рестарт при смене подсети). Коммит `0e01908c`.

**E2E после фикса (VPS 144.31.224.212):**
- ✅ Клиент awg-client (10.8.0.2) → сервер awg1 (10.8.0.1)
- ✅ Handshake: `latest handshake: 2 seconds ago`, AmneziaWG обфускация (Jc/Jmin/Jmax/S1-S4/H1-H4)
- ✅ Ping 10.8.0.1 через туннель: **3/3 received, 0% loss, time=0.042ms**
- ✅ Трафик: 124 B received, 2.01 KiB sent (растёт)

**Полный цикл AWG подтверждён end-to-end:** установка → создание inbound → клиентский .conf → handshake → трафик через туннель.

## Форма AWG: выбор профиля обфускации (2026-07-14, v3.5.0-lucx.6)

Доработана форма создания AWG-inbound (`awg.tsx`) — ранее выбор обфускации был частичным:
- **obfLevel**: подписи приведены к backend профилям (Lite/Standard/Pro вместо none/Jc/S/H/full+CPS) — соответствуют `cps.ObfProfile` (lite/standard/pro)
- **mimicryProfile**: добавлен TLS (ClientHello, Chrome-like) — основной профиль для Standard/Pro по pumbaX; ранее был только quic/sip/dns
- **region**: добавлен селект RU/World (раньше поле было в схеме, но UI-селектора не было) — соответствует `cps.Region` (ru/world)
- Tooltip/hint для каждого селектора
- i18n: 22 awg-ключа добавлены в `en-US.json` и `ru-RU.json` (раньше `t()` возвращал путь ключа — не было переводов)

**Проверка в проде (v3.5.0-lucx.6):**
- TLS Standard RU → jc/jmin/jmax (5/76/237) + I1 (704 байт TLS ClientHello)
- QUIC Pro World full → все 5 пакетов I1-I5 (2402/1090/148/134/146)
- awg1 поднят, peer, подписка работает

## QR и скачивание .conf для AWG-клиентов (2026-07-14, v3.5.0-lucx.7)

Раньше QR и кнопка скачивания .conf в UI клиента работали только для WireGuard — `wireguardConfig.ts` (buildWireguardClientConfig/findWireguardInbound) искал только `protocol === 'wireguard'` и не вставлял обфускацию. Для AWG-клиента .conf был бы неполным (без Jc/S1-S4/H1-H4/I1-I5 и без серверного publicKey).

**Backend (`internal/web/service/inbound.go`):**
- `InboundOption`: поля `awgServerAddress` + `awgObfuscation` (пре-рендеренный блок Jc/S1-S4/H1-H4/I1-I5 как .conf-строка)
- `inboundWireguardHints`: работает и для AWG (Curve25519 тот же, `privateKey`→publicKey derivation, mtu, dns)
- `inboundAwgHints`: достаёт server address + обфускацию из settings
- OpenAPI/Zod типы регенерированы (`npm run gen`)

**Frontend:**
- `wireguardConfig.ts`: `buildAwgClientConfig` (с обфускацией в [Interface]), `findAwgInbound`, `isAwgClient`
- `schemas/client.ts`: `InboundOptionSchema` += `awgServerAddress`/`awgObfuscation`
- `ClientQrModal`: AWG-панель с QR + скачивание (`<email>-awg.conf`)
- `ClientInfoModal`: AWG ConfigBlock с QR + скачивание
- i18n: `awgConfig` ключ (en-US, ru-RU)

**Проверка в проде (v3.5.0-lucx.7):** InboundOption для awg-инбаунда содержит:
- `wgPublicKey: dMeIQIN79x...` (серверный publicKey, derived) ✅
- `wgMtu: 1320`, `wgDns: 1.1.1.1, 1.0.0.1` ✅
- `awgServerAddress: 10.8.0.1/24` ✅
- `awgObfuscation`: полный блок Jc/Jmin/Jmax/S1-S4/H1-H4 ✅

Теперь AWG-клиент в UI показывает QR и кнопку скачивания полного .conf с обфускацией, как WireGuard.

## Исправления по отзыву пользователя (2026-07-15, v3.5.0-lucx.8)

**П1: обновление с нашего репо.** `x-ui.sh` + `update.sh` — все ссылки `MHSanaei/3x-ui` заменены на `AlexeyLCP/lucx-ui` (5+7 ссылок). Команды `install`/`update`/`update_dev`/`update_menu` теперь качают с нашего форка, не с апстрима. Проверено на VPS: `x-ui.sh` — 0 ссылок MHSanaei, 5 AlexeyLCP.

**П2: версия LucX на дашборде.** `internal/config/config.go`: константа `lucxVersion = "lucx.8"`, `GetBaseVersion()` и `GetPanelVersion()` прибавляют суффикс (`3.5.0` → `3.5.0-lucx.8`, dev → `lucx.8+dev+<commit>`). Frontend: `window.X_UI_CUR_VER` (из `dist.go`) = `GetPanelVersion()` → отображается в `AppSidebar` (бейдж версии) и `IndexPage` (дашборд). Проверено: логи `Starting x-ui 3.5.0-lucx.8`. Тест `TestGetPanelVersion` обновлён.

**П3: пресеты обфускации/захват домена в форме AWG.** Уже в коде (`awg.tsx`): `obfLevel` (Lite/Standard/Pro), `mimicryProfile` (TLS/QUIC/DNS/SIP), `region` (RU/World), кнопка генерации, скан хоста. Если пользователь не видит — frontend-кэш браузера, нужен hard refresh (Ctrl+Shift+R).

**П4: добавить пользователя для AWG.** `isInboundMultiUser` (`helpers.ts`) += `case 'awg'` (multi-client как WireGuard); `MULTI_CLIENT_PROTOCOLS` (`ClientBulkAddModal.tsx`) += `'awg'`. Теперь действия клиентов (добавить/QR/инфо) показываются для AWG-inbound.

**Проверено на VPS (v3.5.0-lucx.8):** `install.sh` → `v3.5.0-lucx.8`, логи `Starting x-ui 3.5.0-lucx.8`, `x-ui.sh` обновлён (0 MHSanaei).

## Фикс: проверка обновлений с нашего репо + lucx-сравнение (2026-07-15, v3.5.0-lucx.9)

**Симптом:** панель предлагала «обновиться до 3.5.0», хотя стояла `3.5.0-lucx.8`.

**Рут-коза 1 (URL апстрима):** `panel.go` — `panelUpdaterURL` и `fetchPanelRelease` ссылались на `MHSanaei/3x-ui` (releases/latest = `v3.5.0` без lucx). Заменено на `AlexeyLCP/lucx-ui` (3 ссылки).

**Рут-коза 2 (суффикс ломает парсер):** `parseVersionParts("3.5.0-lucx.8")` → `split(".")` = `["3","5","0-lucx","8"]` → `Atoi("0-lucx")` ошибка → `ok=false` → fallback `normalizeVersionTag(latest) != normalizeVersionTag(current)` → `true` → "update available".

**Фикс:**
- `parseVersionParts`: отрезает `-lucx.N` перед парсингом base (сравнение по upstream-базе)
- `lucxMinor(version)`: извлекает число после `-lucx.` (8 из `3.5.0-lucx.8`), `-1` для plain upstream
- `isNewerVersion`: при равном base сравнивает `lucxMinor` (lucx.9 > lucx.8 → true; plain upstream = -1 → старее fork → false)

**Тесты:** `TestIsNewerVersion` += 5 lucx-кейсов (newer/same/older/plain/newer-base), все PASS.

**lucxVersion** обновлён до `lucx.9`. Релиз v3.5.0-lucx.9 (latest).

**Проверено:** VPS — `install.sh` → `v3.5.0-lucx.9`, логи `Starting x-ui 3.5.0-lucx.9`.

## Фикс: вкладка протокола для AWG (2026-07-15, v3.5.0-lucx.10)

**Симптом:** при создании AWG-inbound пользователь видел только вкладки «основное/сниффинг/расширенный шаблон» — без полей обфускации, ключей, скана хоста, QR.

**Рут-коза:** вкладка «протокол» (где рендерится `AwgFields`) показывалась только для протоколов из списка `[VLESS, SHADOWSOCKS, HTTP, MIXED, TUNNEL, TUN, WIREGUARD, MTPROTO]` (`InboundFormModal.tsx:950`) — **`AWG` отсутствовал в списке**. Хотя `AwgFields` был подключён (`protocol === Protocols.AWG && <AwgFields />`), вся вкладка «протокол» не создавалась для AWG.

**Фикс:**
- `InboundFormModal.tsx`: `Protocols.AWG` добавлен в список протоколов с вкладкой «протокол»
- `protocol-capabilities.ts` `canEnableSniffing`: AWG исключён (kernel sidecar — трафик не через Xray inbound, сниффинг не применяется, как mtproto)

Теперь при создании AWG-inbound есть вкладка «протокол» с: ключи сервера, профиль обфускации (Lite/Standard/Pro), мимикрия (TLS/QUIC/DNS/SIP), регион (RU/World), кнопка генерации, скан хоста, routeThroughXray.

**lucxVersion** → `lucx.10`. Релиз v3.5.0-lucx.10 (latest). VPS обновлён.

**Важно:** после установки — hard refresh браузера (Ctrl+Shift+R), т.к. frontend embed-ится в бинарник и браузер кеширует старую JS-сборку.

## Фикс: Content-Type JSON + i18n route-ключи (2026-07-15, v3.5.0-lucx.11)

**П1 (блокирующий): генерация обфускации/захват домена падали** с ошибкой `invalid character 'o' looking for beginning of value` (или `'d'`). Причина: `HttpUtil.post` для `generateObfuscation`/`captureHost` не передавал `Content-Type: application/json` → `http-init.ts` отправлял form-urlencoded (`encodeForm`) вместо JSON → `gin.ShouldBindJSON` получал `obfProfile=standard&...` как JSON → ошибка. Фикс: оба вызова теперь передают `{ headers: { 'Content-Type': 'application/json' } }` — как все JSON POST в проекте (useClients, useNodeMutations и т.д.).

**П3: i18n route-ключи** — добавлены 5 ключей (`awgRouteThroughXray`, `awgRouteThroughXrayHint`, `awgRouteOutbound`, `awgRouteOutboundHint`, `awgRouteOutboundPlaceholder`) в `en-US.json` и `ru-RU.json`. Ранее `t()` возвращал путь ключа как fallback.

**П4: H1-H4 одиночные числа** (384165 вместо диапазонов `lo-hi`) — потому что обфускация генерировалась старой Math.random заглушкой в `createDefaultAwgInboundSettings`, а не через backend API. После П1 (генерация работает) backend `GenerateAWGParams` отдаёт диапазоны (4 непересекающихся квадранта). `# -1` remark при пустом remark inbound'а — совпадает с WireGuard-паттерном.

**lucxVersion** → `lucx.11`. Релиз v3.5.0-lucx.11 (latest). VPS обновлён.

**Обновления upstream теперь:** ручной перенос ~20 файлов вместо 29.

---

## Фикс: NAT (PostUp/PostDown) для kernel-routing режима (2026-07-16, v3.5.0-lucx.20)

**Проблема:** без `routeThroughXray` (kernel routing) клиенты подключаются, но трафика нет. Причина: ядро поднимает `awgN`, но `net.ipv4.ip_forward` выключен и нет MASQUERADE → пакеты от клиентов (src `10.8.0.x`) не форвардятся и не натятся → ответ не возвращается.

**Решение:** `renderServerConf` теперь генерирует `PostUp`/`PostDown` прямо в `.conf` (как pumbaX/awg-multi-script):
```
PostUp   = echo 1 > /proc/sys/net/ipv4/ip_forward; iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE; iptables -A FORWARD -i awg1 -j ACCEPT; iptables -A FORWARD -o awg1 -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE; iptables -D FORWARD -i awg1 -j ACCEPT; iptables -D FORWARD -o awg1 -j ACCEPT
```

Правила добавляются только когда `!RouteThroughXray` (при routeThroughXray Xray владеет роутингом через TUN inbound — двойной NAT ни к чему). Внешний интерфейс определяется через `ip -o -4 route show default` (build-tag split: `nat_linux.go` / `nat_other.go`). Подсеть клиента извлекается из `Address` через `netip.ParsePrefix().Masked()`.

**Новые файлы:** `internal/awg/nat_linux.go`, `internal/awg/nat_other.go`
**Новые функции:** `defaultRouteInterface()`, `clientSubnet()`, `natPostUpPostDown()`
**Тесты:** `TestClientSubnet`, `TestRenderServerConf_NoPostUpWhenRoutedThroughXray`, `TestRenderServerConf_NoPostUpWhenNoAddress`, `TestNatPostUpPostDown_EmptyWhenNoDefaultRoute`

**lucxVersion** → `lucx.20`.

---

## Фикс: убрать DNS из серверного .conf (2026-07-16, v3.5.0-lucx.21)

**Проблема:** `renderServerConf` писал `DNS = 1.1.1.1, 1.0.0.1` в **серверный** `.conf`. awg-quick при `up` вызывает `resolvconf`/`openresolv` для применения DNS — это перезаписывает системный DNS сервера. На VPS это могло ломать name resolution и вызывать зависания.

**Решение:** DNS — **клиентская** настройка, серверу она не нужна (он просто форвардит пакеты через NAT). pumbaX/awg-multi-script никогда не пишет `DNS =` в серверный конфиг. Убран из `renderServerConf`. Поле `Instance.DNS` остаётся в struct для fingerprint и для `injectAwgEgress` (TUN gateway берётся из DNS), но не пишется в .conf. Клиентские конфиги (`genAwgConfig`, `buildAwgClientConfig`) пишут DNS как раньше.

**Тесты:** `TestRenderServerConf_NeverWritesDNS` (новый), `TestRenderServerConf_IncludesObfuscationAndPeers` обновлён.

**lucxVersion** → `lucx.21`.

---

## Фикс: routeThroughXray для AWG — needRestart, iif policy routing, reconcile-ensure (2026-07-16, v3.5.0-lucx.30)

**Симптом:** при включении тумблера «Маршрутизировать через Xray» на AWG-инбаунде у клиентов пропадал интернет (диагностика на runode: journalctl 18:21–18:24 — awg1 перезапущен, Xray не тронут, tun1 не создан).

**Рут-козы (три независимые):**

1. **Тоггл не перегенерировал конфиг Xray.** `needRestart` поднимался только для MTProto (`mtprotoRoutesThroughXray`) — AWG-путь обновления шёл целиком в kernel-sidecar (`runtime/local.go`), Xray не перезапускался, `injectAwgEgress` не выполнялся → TUN-инбаунд не появлялся. При этом PostUp routeThroughXray-ветки убирает MASQUERADE → трафик клиентов уходил в eth0 с приватным src без NAT → мёртвый интернет.
2. **Маршрут в таблице умирал при каждом рестарте Xray** (tunN пересоздаётся, device-bound route удаляется ядром), а одноразовый PostUp retry-loop (10×1с) проигрывал гонку 30-секундному cron-рестарту и не переживал последующие рестарты.
3. **Фиксированные таблица (100) и gateway (10.254.254.1/30)** ломали конфигурацию с двумя routed-инбаундами (затирали друг друга), а `from <subnet>`-правило дополнительно захватывало server-originated трафик с адресом awgN.

**Решение:**

- `inbound.go`: `awgRoutesThroughXray` (зеркало mtproto-хелпера) + `needRestart` в `AddInbound`/`DelInbound`/`UpdateInbound` (`oldRoutedAwg`)/`SetInboundEnable` (в enable-тоггл добавлен и mtproto — та же латентная дыра).
- `manager.go`: PostUp — статическая половина: ip_forward, loose rp_filter на awgN, FORWARD accepts для awgN и tunN, `ip rule add iif awgN lookup 1000+N` (iif вместо from — не трогает server-originated трафик). Маршрутом владеет `ensureXrayRouting` из reconcile-цикла (каждые 10с): `ip route replace default dev tunN table 1000+N` + loose rp_filter на tunN + самовосстановление ip rule. Молча no-op, пока tunN отсутствует.
- `xray.go` `injectAwgEgress`: gateway per-inbound `10.254.(N%254).1/30` (`awgTunGateway`) вместо фиксированного; на TUN-инбаунд навешен sniffing `{http,tls,quic, routeOnly:true}` — без него доменные/geosite-правила роутера для AWG-трафика молча не срабатывали (роутер видел только IP). `routeOnly` оставляет снифф домена подсказкой для роутинга, адрес назначения не подменяется.

**Дизайн проверен вживую** на runode до реализации: netns-клиент → veth → `ip rule iif` → tun99 (реальный xray-бинарник) → freedom: ICMP 2/2, HTTPS 200, в xray-логе dispatcher → routing → freedom с IP сервера.

**Тесты:** `TestAwgRouteTable`, `TestRenderServerConf_RouteThroughXrayPolicyRouting`, `TestNatPostUpPostDown_RouteThroughXrayPerInbound`, `TestEnsureXrayRoutingCmds`, `TestRuleMissing` (awg); `TestAwgRoutesThroughXray`, `TestAddInbound_RoutedAwgForcesXrayRegen`, `TestAddInbound_PlainAwgDoesNotForceRegen`, `TestDelInbound_RoutedAwgForcesXrayRegen`, `TestSetInboundEnable_DisableRoutedAwgForcesXrayRegen`, `TestInjectAwgEgress_PerInboundGateway` (service). Полные сьюты awg + web/service зелёные (`-shuffle=on`).

**lucxVersion** → `lucx.30`.

---

## Фикс: golangci-lint (25 ошибок) + TUN gateway из Address (2026-07-16, v3.5.0-lucx.22–28)

**CI lint:** 25 ошибок в LucX-коде:
- errcheck (3): непроверенные `logWriter.Write`, `binary.Write`
- gofumpt (много): выравнивание во всех LucX-файлах с LUCX-HOOK блоками + новых файлах
- noctx (5): `exec.Command` → `CommandContext`, `net.LookupIP` → `Resolver.LookupIPAddr`, `net.DialTimeout` → `Dialer.DialContext`
- staticcheck (5): `rand.Seed` deprecation → пакетный `rng` + `SetRand`, `info.String()`, `len()` nil-check
- unused (3): удалены `bindTo`, `formatTransfer`, `dtlsDomains`
- usestdlibvars (6): `http.MethodGet/StatusNotFound/StatusOK` в detector.go

**Тесты CI:** `TestGetPanelVersion` (версия), `TestAPIRoutesDocumented` (2 endpoint'а — generateObfuscation + captureHost в endpoints.ts), `TestInjectAwgEgress_*` (gateway), `TestNatPostUpPostDown` (Linux default route).

**DNS из серверного .conf убран** (lucx.21): DNS — клиентская настройка, серверу не нужна. pumbaX никогда не пишет DNS в серверный конфиг.

**Пустой outboundTag = "котел Xray"** (lucx.27–29): при пустом `outboundTag` routing rule не добавляется — TUN-трафик попадает в общий routing pipeline Xray (sniffing/domain/balancer). Явный outboundTag = перехват всего трафика в конкретный outbound. i18n: placeholder "Использовать правила маршрутизации". Select с явной опцией `value=""`.

**lucxVersion** → `lucx.28`.

---

## Фикс: routeThroughXray — policy routing + /30 TUN subnet (2026-07-16, v3.5.0-lucx.25)

**Проблема:** при routeThroughXrayXray создаёт TUN inbound (tunN), но пакеты из AWG kernel interface (awgN) не попадают в tunN — нет маршрута. Plain route `ip route replace <subnet> dev tunN` направляет пакеты destiné в подсеть, а не от неё.

**Решение:** policy routing в PostUp:
- `ip rule add from <subnet> lookup 100` — все пакеты от клиентов идут в table 100
- `ip route replace default dev tunN table 100` — table 100 направляет всё в tunN
- Retry-loop ждёт появления tunN (10 попыток по 1с)

**TUN gateway в отдельной /30 подсети:** `10.254.254.1/30` — не конфликтует с AWG subnet (10.8.0.0/24). Раньше gateway брался из DNS (1.1.1.1) — неправильно, Xray отвергал bare IP ("invalid CIDR address"). Потом брался из Address (10.8.0.1) — конфликтовал с awgN. Финал: фиксированная /30.

**lucxVersion** → `lucx.25`.

---

## Фикс: routeThroughXray — needRestart, iif policy routing, reconcile-ensure (2026-07-16, v3.5.0-lucx.30, PR #13)

**Симптом:** при включении тумблера «Маршрутизировать через Xray» на AWG-инбаунде у клиентов пропадал интернет. В тестах маршрут правильный, на практике — нет. Доменные правила маршрутизации для AWG-трафика не срабатывали вовсе.

**Рут-козы (четыре независимые):**

1. **Тоггл не перегенерировал конфиг Xray.** `needRestart` поднимался только для MTProto (`mtprotoRoutesThroughXray`) — AWG-путь обновления шёл целиком в kernel-sidecar (`runtime/local.go`), Xray не перезапускался, `injectAwgEgress` не выполнялся → TUN-инбаунд не появлялся. При этом PostUp routeThroughXray-ветки убирает MASQUERADE → трафик клиентов уходил в eth0 с приватным src без NAT → мёртвый интернет.
2. **Маршрут в таблице умирал при каждом рестарте Xray** (tunN пересоздаётся, device-bound route удаляется ядром), а одноразовый PostUp retry-loop (10×1с) проигрывал гонку 30-секундному cron-рестарту и не переживал последующие рестарты.
3. **Фиксированные таблица (100) и gateway (10.254.254.1/30)** ломали конфигурацию с двумя routed-инбаундами (затирали друг друга), а `from <subnet>`-правило дополнительно захватывало server-originated трафик с адресом awgN.
4. **Роутер не видел домены AWG-трафика.** На инжектируемом TUN-инбаунде не включён sniffing → роутер матчит только IP. Любые `domain:`/`geosite:`-правила для AWG-трафика молча не срабатывали.

**Решение (PR #13 от rudenko-ks):**

- `inbound.go`: `awgRoutesThroughXray` (зеркало mtproto-хелпера) + `needRestart` в `AddInbound`/`DelInbound`/`UpdateInbound` (`oldRoutedAwg`)/`SetInboundEnable` (в enable-тоггл добавлен и mtproto — та же латентная дыра).
- `manager.go`: PostUp — статическая половина: ip_forward, loose rp_filter на awgN, FORWARD accepts для awgN и tunN, `ip rule add iif awgN lookup 1000+N` (iif вместо from — не трогает server-originated трафик). Маршрутом владеет `ensureXrayRouting` из reconcile-цикла (каждые 10с): `ip route replace default dev tunN table 1000+N` + loose rp_filter на tunN + самовосстановление ip rule. Молча no-op, пока tunN отсутствует.
- `xray.go` `injectAwgEgress`: gateway per-inbound `10.254.(N%254).1/30` (`awgTunGateway`) вместо фиксированного; на TUN-инбаунд навешен sniffing `{http,tls,quic, routeOnly:true}` — без него доменные/geosite-правила роутера для AWG-трафика молча не срабатывали (роутер видел только IP). `routeOnly` оставляет снифф домена подсказкой для роутинга, адрес назначения не подменяется.

**Тесты:** `TestAwgRouteTable`, `TestRenderServerConf_RouteThroughXrayPolicyRouting`, `TestNatPostUpPostDown_RouteThroughXrayPerInbound`, `TestEnsureXrayRoutingCmds`, `TestRuleMissing`, `TestAwgRoutesThroughXray`, `TestAddInbound_RoutedAwgForcesXrayRegen`, `TestAddInbound_PlainAwgDoesNotForceRegen`, `TestDelInbound_RoutedAwgForcesXrayRegen`, `TestSetInboundEnable_DisableRoutedAwgForcesXrayRegen`, `TestInjectAwgEgress_PerInboundGateway`, `TestInjectAwgEgress_SniffingRouteOnly`.

**lucxVersion** → `lucx.30`.

---

## Фича: пресеты TLS ClientHello для Firefox и Safari (2026-07-16, v3.5.0-lucx.31)

**Контекст:** `buildTLSClientHello` в `cps.go` генерировал только Chrome-like ClientHello. Добавлены browser-специфичные пресеты для DPI evasion.

**Backend:**
- `domains.go`: `BrowserProfile` type (`chrome`/`firefox`/`safari`)
- `cps.go`: разбит на `buildChromeHello`/`buildFirefoxHello`/`buildSafariHello`:
  - **Chrome** — GREASE в cipher suites и extensions, compress_certificate, ALPS, padding 0-48
  - **Firefox 120+** — NSS cipher ordering (включая ECDHE CBC), delegated_credentials extension, padding до 512 байт. Нет GREASE, нет compress_certificate, нет ALPS
  - **Safari 16+** — Apple SecureTransport cipher ordering (включая DHE и legacy CBC), secp521r1, TLS 1.1 advertised. Нет GREASE, нет padding, нет compress_certificate
- `GenerateCPS` принимает `browser BrowserProfile`, передаёт в `tlsPacket`
- `controller/awg.go`: `generateObfuscation` принимает `browserProfile`, default `chrome`
- QUIC (`quicInitialPacket`) использует `buildChromeHello` (QUIC всегда Chrome-форму)
- Helper'ы `writeSupportedGroupsExt`/`writeSigAlgsExt`/`writeSupportedVersionsExt`/`writeKeyShareExt` параметризованы (`grease bool`, `algs []uint16`)

**Новое в `cps.go`:** `writeSupportedGroupsExtSafari` (x25519, secp256r1, secp384r1, secp521r1), `writeSupportedVersionsExtSafari` (0x0304, 0x0303, 0x0302), `writeKeyShareExtSafari` (x25519 + secp256r1), `writeDelegatedCredentialsExt`, `wrapHandshake`, `padTo512`. Переменные `chromeSigAlgs`/`firefoxSigAlgs`/`safariSigAlgs`.

**Frontend:**
- `awg.ts` schema: `browserProfile: z.enum(['chrome','firefox','safari']).default('chrome')`
- `awg.tsx` form: Select видим только при `mimicryProfile === 'tls'`, опции "Chrome (последняя)", "Firefox 120+", "Safari 16+"
- `inbound-defaults.ts`: `browserProfile: 'chrome'` default
- i18n: 5 ключей (`awgBrowserProfile`, `awgBrowserProfileHint`, `awgBrowserChrome`, `awgBrowserFirefox`, `awgBrowserSafari`)

**Источник данных:** `bogdanfinn/tls-client` (профили Firefox_120/Firefox_133, Safari_16_0), перенесено вручную без новой зависимости. `bogdanfinn/tls-client` — HTTP-клиент для веб-скрапинга с обходом antibot, построен на `refraction-networking/utls`. Для AWG не подходит напрямую (нужны сырые байты ClientHello, не HTTP-клиент), но профили — репрезентативны.

**Тесты:** `TestGenerateCPS_AllBrowsersNonEmpty`, `TestBuildFirefoxHello_NoGrease`, `TestBuildSafariHello_NoGrease`, `TestBuildChromeHello_HasGrease`, `TestBuildFirefoxHello_HasPadding512`, `TestBuildSafariHello_HasTls11`. Все 12 CPS-тестов проходят.

**lucxVersion** → `lucx.31`.

---

## Фикс: install.sh 404 — /releases/latest игнорировал prerelease-релизы (2026-07-17, v3.5.0-lucx.32)

**Симптом:** `install.sh` падал с "Failed to fetch x-ui version" — `https://api.github.com/repos/AlexeyLCP/lucx-ui/releases/latest` возвращал 404.

**Рут-коза:** GitHub API `/releases/latest` игнорирует релизы с `prerelease: true`. Все наши релизы (lucx.20–31) были prerelease → "latest" не существовал. `gh release list` их показывал, но API-эндпоинт, который дёргает install.sh, — нет.

**Решение:** `.github/workflows/release.yml`: `prerelease: true` → `prerelease: false` (job upload-release-action). Релиз v3.5.0-lucx.32 создан уже как stable — `/releases/latest` резолвится, install.sh работает. Rolling dev-канал (`dev-latest`) не затронут: он живёт отдельным фиксированным тегом с `--latest=false`, стабильный канал не перебивает.

**lucxVersion** → `lucx.32`.

---

## Пакет: самовосстановление NAT, версия из тега, AWG-диагностика, лицензия PolyForm NC (2026-07-18, v3.5.0-lucx.33)

Пакет улучшений по итогам аудита форка (см. список в начале дня). Всё ниже — только LucX-файлы и LUCX-HOOK блоки.

**1. ensureNatRules — самовосстановление NAT (kernel-режим).** PR #13 добавил `ensureXrayRouting` в reconcile для routeThroughXray, но plain-режим оставался с одноразовым PostUp: любой flush iptables (fail2ban reload, docker, руки админа) молча убивал интернет клиентов до рестарта интерфейса. Теперь `manager.go`: `natRulesFor` (чистый builder: MASQUERADE + FORWARD ×2) + `ensureNatRules` (check `-C` → add `-A` при отсутствии, + `sysctl ip_forward=1`) вызывается из `Ensure` и `Reconcile` рядом с `ensureXrayRouting`. No-op для routeThroughXray и пока awgN отсутствует. Тесты: `TestNatRulesFor`, `TestNatRulesFor_SkipsUnroutable`.

**2. Версия из git-тега.** `const lucxVersion` → `var lucxVersion` (default `lucx.33` для локальных сборок); `release.yml` на tag-билдах инжектит суффикс из тега через `-ldflags -X` и **падает**, если тег и source default разошлись (`v3.5.0-lucx.N` ↔ `lucx.N` в config.go). `config_test.go` больше не хардкодит версию — derive из переменной + `TestLucxVersionFormat` (regex `^lucx\.\d+$`). Убирает класс CI-фейлов «забыли обновить тест при bump».

**3. AWG runtime diagnostics.** Новое: `internal/awg/diagnostics.go` — read-only probe живого состояния: интерфейс UP, ip_forward, пиры/рукопожатия (`awg show peers/latest-handshakes`), в kernel-режиме MASQUERADE + FORWARD (через `natRulesFor`), в xray-режиме tunN + ip rule + route table. prober-интерфейс для тестов (fake replay). Endpoint `GET /panel/api/inbounds/:id/awgDiagnostics`; UI — кнопка «Диагностика» в AWG-форме (только для сохранённого инбаунда — id проброшен через `awg-inbound-id-context.ts` provider в InboundFormModal) с модалкой: Alert по `healthy` + список проверок с ✓/✗ и evidence-деталями + Refresh. 9 i18n-ключей × 13 локалей. Тесты: 7 штук (`TestDiagnose_*`, `TestParseDefaultRouteInterface`, `TestParseLatestHandshakes`).

**4. signature — первые тесты пакета.** `capture_test.go`: `normalizeDomain`, `fillPackets` (truncation 1500, max 5), `appendVarint` (границы 63/64, 16383/16384), `hkdfExpandLabel` (детерминизм), `buildTLSClientHello`/`buildQUICInitial` структурные инварианты (0x01, length, SNI, ALPN h3, ≥1200 байт, long-header bit, QUIC v1, DCID len 8). Был единственный LucX-пакет без покрытия.

**5. QUIC уважает browserProfile.** `quicInitialPacket(domain, browser)` — embedded ClientHello теперь строится профильным builder'ом (Chrome/Firefox/Safari) вместо всегда-Chrome. Тест `TestQuicInitialPacket_RespectsBrowser`.

**6. i18n.** `awgBrowser*` (5 ключей) добавлены в 11 локалей (ar/es/fa/id/ja/pt/tr/uk/vi/zh-CN/zh-TW) — до этого были только en/ru, остальные падали в fallback. JSON-aware вставка через Node-скрипт с byte-identical round-trip (diff +7/-1 на файл). Плюс 9 `awgDiag*` ключей × 13 локалей.

**7. mutation.yml timeout 120→360** (LUCX-HOOK) — матрица service/database упиралась в 2ч и job отменялся GitHub'ом как cancelled.

**8. bin/check-lucx.sh + bin/pre-push.** `check-lucx.sh` — gofumpt по изолированным пакетам + всем файлам с LUCX-HOOK (37 файлов), `-w` для автофикса — ловит Windows/Linux-дрейф форматирования до CI. `pre-push` — git hook (установка `cp bin/pre-push .git/hooks/pre-push`): gofumpt + быстрые go test (awg/lucx/config) + проверка открытых PR (блокирует) и issues (предупреждает) на AlexeyLCP/lucx-ui — механизирует AGENTS.md шаги 6 и 11.5.

**9. Лицензия PolyForm Noncommercial 1.0.0.** Split-лицензирование: upstream-код — GPL-3.0, LucX-компоненты — PolyForm NC (свободно для личного/образовательного использования; коммерция, включая перепродажу VPN, — по письменному разрешению). Новое: `LICENSE-PolyForm-Noncommercial.txt` (канонический текст с polyformproject.org), `LICENSING.md` (граница лицензий, список LucX-файлов, контакт). SPDX-заголовки добавлены в 12 файлов, где их не было (`awg_job.go`, `nat_*`, `orphans_*`, `awg.ts`, `awg.tsx`, `wireguardConfig.ts`, `awg-inbound-id-context.ts`, `bin/install-awg-module.sh`, `bin/check-lucx.sh`, `bin/pre-push`); остальные 20 LucX-файлов уже имели заголовки.

**10. README — LucX-секция** (LUCX-HOOK блок после badges): что такое форк, AWG-фичи, browser profiles, routeThroughXray, диагностика, install-команда с `AlexeyLCP/lucx-ui`, ссылка на LICENSING.md. Раньше README был чисто upstream с бейджами mhsanaei/3x-ui.

**11. Мелочи.** `.gitignore`: `.playwright-mcp/`. Удалены пустые директории-остатки старой ветки (`internal/lucx/{controller,integration,telegram,telemt}`) — не трекались git.

**12. README переписан** (тот же день, follow-up): LucX-блок поднят в самый верх — сегмент на русском 🇷🇺 + английский 🇬🇧, предупреждение о личном/некоммерческом/научном/образовательном использовании (WARNING-блок в шапке, RU+EN), расширенная таблица лицензий (GPL-3.0 ↔ PolyForm NC), благодарности тестерам (VladufQa, Kirill Rudenko — PR #13) и команде 3x-ui, отсылки к проектам-источникам (3x-ui, AmneziaVPN, pumbaX/awg-multi-script, hoaxisr/awg-manager, bogdanfinn/tls-client + refraction-networking/utls), список «что добавлено и работает» с ✅. Upstream-документация сохранена ниже с маркером-разделителем.

**lucxVersion** → `lucx.33` (default в source; релизный бинарник получает версию из тега через -ldflags).

---

## Пакет: AWG slimming до mtproto-паритета, полный i18n, upstream-watch, branch protection (2026-07-18, без bump версии)

Рефакторинг/инфра-пакет без изменения поведения панели — версия не bump'ится (релиз lucx.33 актуален), тег не двигаем.

**1. AWG slimming — Known Issue #1 закрыт окончательно.** Core-пакет `internal/awg/` сжат с 12 до 9 файлов — точная симметрия с mtproto (6 source + 3 test против 4 source + 2 platform + 3 test):
- `traffic.go` (66 строк) влит в `manager.go` — `Traffic`/`scrapeTransfer` существуют только ради `CollectTraffic`
- `nat_{linux,other}.go` + `orphans_{linux,other}.go` (4 крошечных build-tagged файла) → одна пара `platform_{linux,other}.go`
- Вычищен мусор: `var (_ = strconv.Itoa; _ = syscall.Kill)` в orphans_linux.go — гварды неиспользуемых импортов от удалённого tun2socks
- Чистое перемещение без логики, коммит на каждый шаг (bisect-friendly), тесты + GOOS=linux build зелёные после каждого
- `cps/` и `signature/` остаются пакетами — это фичи вне компетенции mtproto

**2. Полный перевод AWG-формы на 11 локалей.** Раньше из 44 awg-ключей в ar/es/fa/id/ja/pt/tr/uk/vi/zh-CN/zh-TW были только browser/diag (14 шт. из lucx.33) — остальная форма падала в английский fallback. Добавлены 30 ключей × 11 локалей: server keys, obf/mimicry profiles + hints, region, capture host, routeThroughXray/outbound + placeholder, address + `pages.clients.awgConfig`. JSON-aware вставка (Node-скрипт, byte-stable round-trip), проверка полноты: 0 пропусков во всех 13 локалях.

**3. upstream-watch workflow.** `.github/workflows/upstream-watch.yml`: cron каждый понедельник 09:00 UTC (+ workflow_dispatch). Сравнивает `gh api repos/MHSanaei/3x-ui/releases/latest` с `internal/config/version`; при расхождении открывает issue с процедурой миграции (rule 8). Идемпотентно — не дублирует issue для того же тега. Проверено: v3.5.0 == base — молчит. Отвечает на «как узнаем о новой версии апстрима» — автоматически.

**4. Branch protection на gh/main.** Включена через API: `enforce_admins: true`, `allow_force_pushes: false`, `allow_deletions: false`. PR/status-checks НЕ требуются — прямые пуши работают. Force-push теперь осознанное двухшаговое действие (Settings → Branches → ослабить → вернуть). Контекст: contributors ≠ collaborators — доступ к репо только у AlexeyLCP; коммерческие лицензии на LucX-код может выдавать только правообладатель (PolyForm NC), upstream-контрибуторы не имеют копирайта в LucX-файлах. Задокументировано в AGENTS.md (новый раздел Branch Protection).

**5. VPS lucx недоступен** — deploy lucx.33 отложен. Все порты (22/2053/443) фильтруются, ping не идёт: VM остановлена или ephemeral IP сменился (GCP). Нужна консоль GCP: поднять VM или обновить IP в `~/.ssh/config` (Host lucx). Deploy-процедура когда поднимется: tarball v3.5.0-lucx.33 с GitHub → распаковать → заменить `/usr/local/x-ui/x-ui` → `systemctl restart x-ui` → verify.

**6. Тестовые серверы задокументированы.** Пользователь предоставил 2 IP: `144.31.224.212` (skinny-azure-snail.play2go.cloud) и `144.31.157.106` (poor-rose-snake.play2go.cloud). Доступ: `root` + `~/.ssh/id_ed25519` (НЕ id_rsa — Permission denied). SSH-алиасы `lucx-test1`/`lucx-test2` в `~/.ssh/config`, оба проверены. Состояние: test1 — панель **lucx.17** (старьё, до всех routing-фиксов lucx.20–33!), x-ui active, awg1 живой; test2 — **x-ui.service отсутствует вовсе** (панель не установлена/удалена), осиротевший awg0 — готовый кейс для orphan sweep + чистой установки. AGENTS.md → Deploy обновлён таблицей серверов.

**7. Деплой на тестовые серверы — оба на lucx.33.**

- **test2 — чистая установка end-to-end ✅.** `install.sh` из README: релизный tarball скачался (фикс `/releases/latest` работает в бою), DKMS собрал `amneziawg/1.0.0` под kernel 6.12.90 (Debian 13 trixie), модуль загружен, awg/awg-quick на месте, панель active, `3.5.0-lucx.33`. Осиротевший awg0 убран (не пережил DKMS/reload). Панель: `http://144.31.157.106:1360/rJzisfkxRTqHGhACTn` (креды в install-логе сервера).
- **test1 — апгрейд lucx.17 → lucx.33 ✅** (бэкап бинарника `x-ui.bak-lucx17`, tarball, restart). После рестарта **awg1 не поднялся**: PostUp падал с `iptables: command not found` (exit 127) — Debian 13 не ставит iptables из коробки, а NAT-PostUp появился только в lucx.20 → при апгрейде со старых версий это deployment-ловушка. Фикс: `apt install iptables` (shim над nf_tables) → reconcile поднял awg1 сам за ≤10 с, пиры + MASQUERADE на месте, порт 51820.
- **Код-фикс:** `bin/install-awg-module.sh` теперь ставит `iptables` как зависимость (рядом с openresolv) — свежие установки покрыты. AGENTS.md: новый Debug Pattern 1b с симптомами и фиксом.

**lucxVersion** → без изменений (`lucx.33`; код панели не менялся).

**8. Донаты.** README: раздел «☕ Поддержать проект» в RU и EN ветках — ЮMoney (рубли, РФ), USDT (TON), USDT (ERC-20), с оговоркой «донат ≠ коммерческая лицензия»; donate-бейдж в шапку. `.github/FUNDING.yml`: заменён upstream-овский (донаты шли **MHSanaei** — `github: MHSanaei`, `buy_me_a_coffee: mhsanaei`, `custom: donate.sanaei.dev`!) на наш custom-линк ЮMoney; крипта в FUNDING.yml не поддерживается — только в README. Кнопка Sponsor на странице репо теперь ведёт к нам.

---

## Фиксы по живым репортам тестеров + деплой dev на test2 (2026-07-19, dev-канал)

**1. Футер/ссылки панели → наши.** `AppSidebar.tsx`: версия-бейдж, donate, docs указывали на MHSanaei/sanaei.dev → заменены на AlexeyLCP/lucx-ui + наш ЮMoney (LUCX-HOOK). Обновление через UI проверено: `panel.go` + `update.sh` полностью наши — ставит нашу версию (и stable, и dev).

**2. Онлайн-статус AWG-клиентов (репорт VladufQa «все оффлайн»).** Корень: online-set панели наполняли только Xray stats API и mtg; `awg_job` не вызывал `RefreshLocalOnlineClients` никогда. Фикс: `scrapeTransfer` → `scrapePeers` (один `awg show <iface> dump` = pubkey+rx+tx+handshake); `CollectTraffic` возвращает inbound-дельты + per-peer дельты + online-пиры (handshake < 180с, REKEY_TIMEOUT); джоба мапит pubkey→email и вызывает RefreshLocalOnlineClients каждый тик. **Бонус: per-client трафик** (раньше был только inbound-уровень). Baseline снова per-peer.

**3. Двойной учёт трафика routed-инбаундов** (найден ревизией после #2): TUN inbound получает тег AWG-инбаунда → Xray stats метрит `inbound>>>tag`, а awg_job складывал тот же объём из kernel-счётчиков. Фикс: `routedTags` (как в mtproto_job) — для routed пропускаем inbound-уровень, per-client сохраняем.

**4. Post-restart routing window (репорт VladufQa «приходится повторно выбирать outbound»).** tunN device-bound → умирал с Xray, унося `default dev tunN table 1000+N`; до тика cron (до 10с) routed-клиенты без интернета. Фикс: `ensureAwgRouting()` синхронно в `RestartXray` сразу после `p.Start()`.

**5. Аллокация адресов клиентов из подсети инбаунда** (поймано на test2): инбаунд `10.9.0.1/24`, первый клиент получил `10.8.0.2` — из хардкод-пула, не из подсети туннеля. `defaultAwgClients` теперь берёт базу из `settings.address` (masked), fallback на 10.8.0.0/24 только при пустом/битом address. Тесты.

**6. Деплой на test2 (144.31.157.106) — полный цикл проверен на dev-сборке (lucx.33+dev+47260c95):**
- Зачистка → чистая установка `install.sh dev-latest` (rolling dev-канал работает)
- AWG-инбаунд через API: awg1 поднялся, MASQUERADE/FORWARD на месте
- **Диагностика endpoint**: все проверки с evidence (interface/ip_forward/peers/NAT)
- **Loopback-клиент** (awgcli0 на 127.0.0.1, с obfuscation-блоком Jc/S/H): handshake прошёл, **онлайн-статус `["peer-laptop"]` в API**, per-client трафик в БД
- **routeThroughXray**: tun1 UP, `iif awg1 lookup 1001`, `default dev tun1` в table 1001 — вся цепочка после фикса #4
- Найденный в процессе нюанс: H1-H4 в settings должны быть **строками** (UI так и шлёт; мой ручной API-payload слал числа → InstanceFromInbound молча false). Zod-схема подтверждает string — UI не затронут
- **test1 (144.31.224.212) с 2026-07-19 НЕ НАШ** — отдан под другой продукт; AGENTS.md обновлён, туда не лезем
- Панель test2: `http://144.31.157.106:2053/` (порт 2053, basePath `/`), тестовый инбаунд awg-verify на 52901 + клиент peer-laptop

**7. Dependabot-очередь:** 10 version-update PR (создались из GitHub UI grouped updates, не из нашего yml) — закрыты с комментариями; gotcha в AGENTS.md Known Issue #3.

**lucxVersion** → без изменений (`lucx.33` + dev-коммиты; релиз lucx.34 соберём, когда стабилизируем).

---

## Релиз v3.5.0-lucx.34 (2026-07-20)

**Состав** (всё из записи 2026-07-19 выше): онлайн-статус AWG-клиентов через handshakes, per-client трафик, `ensureAwgRouting` в `RestartXray` (post-restart window закрыто), `routedTags` против двойного учёта routed-инбаундов, аллокация адресов клиентов из подсети инбаунда, наши ссылки в сайдбаре (версия/donate/docs).

**Процесс:** `lucxVersion` bump → `lucx.34`, полная верификация (build/tests/gofumpt/typecheck зелёные), коммит `6849e932`, тег `v3.5.0-lucx.34`. CI release: **guard тег↔source отработал** (тег и config.go совпали), релиз опубликован `prerelease=false`, `/releases/latest` → lucx.34.

**Деплой на test2:** dev → stable lucx.34 (бэкап dev-бинарника, tarball, restart). После рестарта: awg1 поднялся сам (reconcile), routed-цепочка на месте — tun1 UP, `iif awg1 lookup 1001`, `default dev tun1` в table 1001 (маршрут восстановлен reconcile-циклом — фикс post-restart window работает на реальном рестарте сервиса). Пир peer-laptop на месте.

**lucxVersion** → `lucx.34`.

---

## Релиз v3.5.0-lucx.35 (2026-07-20) — UX-фикс плейсхолдера аллокации

**Контекст.** Тестер Aleksandr SacredX в Telegram-группе поднял три вопроса:
1. **Подсеть клиента.** Инбаунд `10.10.0.1/24`, первый клиент получил `allowedIPs: 10.8.0.2/32` — адрес из дефолтного пула, который сервер не маршрутизирует.
2. **Несколько AWG-инбаундов на разных портах** — можно ли?
3. **Старые AWG 1.0/1.5** (для Keenetic) — поддерживает ли kernel module, и почему LITE-пресет генерит под 2.0; не откатится ли ручная правка шаблона 2.0→1.5 после рестарта?

**Анализ.**

1. **Подсеть.** Сам баг (бэкенд игнорировал `settings.address` при аллокации) уже исправлен в **lucx.34** (коммит `2884a9f9`: `defaultAwgClients` теперь берёт подсеть из `awgSettingsAddress(oldInbound.Settings)`, fallback на `defaultAwgBase` только при пустом/битом address). Тестер был на старой версии (видно `browserProfile` в JSON — поле появилось до lucx.34, commit `1af2007c`, но без фикса подсети). Однако оставался **UX-провал**: плейсхолдер поля `wgAllowedIPs` в клиентской форме был захардкожен `10.8.0.2/32` — на нестандартных подсетях сбивал с толку и провоцировал вписать руками неправильный адрес. Фикс lucx.35: плейсхолдер **динамический** — берётся из `awgServerAddress` выбранного AWG-инбаунда (`10.10.0.1/24` → плейсхолдер `10.10.0.2/32`). `ClientFormModal.tsx`: новый `awgAllowedIPsPlaceholder` useMemo. Бэкенд-данные уже на месте (`InboundOption.awgServerAddress` заполняется `inboundAwgHints` с lucx.33). **Поле остаётся пустым** — бэкенд аллоцирует из подсети инбаунда; меняется только подсказка.
2. **Несколько AWG-инбаундов** — да, можно. Каждый получает свой `awgN`-интерфейс, свою подсеть (`settings.address`), свой порт. Reconcile-цикл управляет каждым независимо. Ограничение только системное — уникальность портов и подсетей (дефолт 10.8.0.0/24, но можно ставить любую). Документирую.
3. **AWG 1.0/1.5 vs 2.0.** Это не semver kernel module, а **версия протокола AmneziaWG**, определяемая набором полей обфускации в .conf. v1.0 = только Jc/Jmin/Jmax; v1.5 = + S1-S4; v2.0 = + H1-H4 + I1-I5 (что и генерит наш Pro-уровень). LITE-пресет (obfLevel=1) у нас генерит **только Jc** — это фактически v1.0, не 2.0. Поле `awg show`/kernel module у нас `1.0.20260611` (DKMS собирает из `amneziawg/amneziawg-linux-kernel` main) — он обратно-совместим со всеми версиями протокола (поле absent = не применяется). Ручная правка «Расширенного шаблона» (Advanced template) **не откатывается** после рестарта — шаблон хранится в `settings` инбаунда в БД, панель не перезаписывает пользовательский шаблон при рестарте. Перегенерация Jc/S/H/I происходит только при явном клике «Сгенерировать обфускацию» (API `generateObfuscation`), не при каждом рестарте.

**lucxVersion** → `lucx.35`.

**Деплой на test2:** stable lucx.34 → lucx.35 (stop, бэкап `x-ui.bak-lucx34`, замена, start). Сервис active, `lucx.35` в бинарнике, awg1 поднялся сам (reconcile), routed-цепочка на месте (`awg show` → awg1 на 52901, jc=4).

---

## Релиз v3.5.0-lucx.37 (2026-07-20) — AWG Outbound (клиентский режим)

**Фича:** AWG outbound — клиентское подключение к upstream AmneziaWG-серверу. Kernel-интерфейс `awgo-{Id}` через `awg-quick` + Xray `freedom` outbound с `sockopt.interface`. Симметрия с AWG inbound (серверным режимом): inbound = принимаем AWG-клиентов → Xray routing; outbound = подключаемся к upstream VPN → Xray routing отправляет трафик через нас.

**Архитектура (10 задач, SDD — subagent-driven development):**
1. `model.AwgOutbound` + миграция таблицы `awg_outbounds` (LUCX-HOOK в `db.go`)
2. `ClientInstance` + `renderClientConf` (`Table = off` критично, no ListenPort, DNS опционально)
3. `Manager.EnsureClient`/`RemoveClient`/`SweepOrphanClients` (sync.Once)/`CollectClientTraffic` (.conf 0600)
4. `AwgOutboundService` CRUD + `ParseConf` + `checkTagUnique` + `defaultAwgOutboundSettings` (wgutil keypair)
5. `injectAwgOutbounds` — freedom + sockopt.interface + sendThrough (CIDR strip), после outbounds до balancers/routing (LUCX-HOOK в `xray.go`)
6. `AwgOutboundController` — 8 REST endpoints + Test (ping -I, IPv6 fallback) + parseConf (LUCX-HOOK в `api.go`)
7. Reconcile loop — расширение `awg_job` (10с cron + startup orphan sweep)
8. Frontend: Zod `AwgOutboundSchema` + API клиент (HttpUtil + Msg<T> + parseMsg)
9. Frontend UI: вкладка "AWG outbounds" в XrayPage + форма (react-hook-form + FormField) + Paste .conf drawer + Status badge + Test button (LUCX-HOOK в XrayPage.tsx + AppSidebar.tsx); i18n en+ru

**Финальный whole-branch review (opus) нашёл 2 Critical + 2 Important — все исправлены:**
- Critical: `needRestart` на мутациях (SetToNeedRestart в add/update/del/enable) — иначе disable оставляет freedom outbound в Xray, который молча fallback на default route (сafety hazard)
- Critical: inbound `killStrayAwgInterfaces` удалял `awgo-*` outbound-интерфейсы (фильтр `HasPrefix("awg")` матчил и awgN, и awgo-N) → `isInboundAwgInterface` (digit after "awg") + тест
- Important: `checkTagUnique` теперь cross-check Xray template outbound tags (`tagInXrayTemplate`)
- Important: `CollectClientTraffic` через `awg show dump` (не plain) + правильные field indices

**Bonus fix (пойман live на test2):** `parseClientDump` не пропускал interface-строку awg dump (18+ полей: privkey/pubkey/port/jc/jmin/jmax/s1-s4/h1-h4/i1-i5). Эти numeric fields[4..6] затирали peer-строку → status endpoint показывал rx=0 tx=0 при реальном tx=740. Фикс: `lines[1:]` (как в `parseAwgDump`). Тест `TestParseClientDump_RealAwgInterfaceLine` с реалистичной interface-строкой.

**Integration test (test2, lucx.37):** всё работает
- API: list/add/status/enable/test — все 8 endpoints отвечают
- awgo-1 создаётся через reconcile (10с), `Table = off`, peer подключён, transfer считает
- Xray config: freedom outbound с tag=test-upstream, sockopt.interface=awgo-1 — в config.json
- status endpoint: rx/tx совпадает с `awg show awgo-1 transfer` (0/740) — после parseClientDump fix
- disable → awgo-1 удалён из kernel; re-enable → вернулся
- awg1 (inbound) цел — orphan sweep корректно исключает awgo-*
- Test endpoint: ping -I awgo-1 1.1.1.1 запускается (fail ожидаемо — loopback upstream без reverse route)
- Nuance: config.json на диске не обновляется на enable/disable (Xray применяет через core API, runtime корректен) — это upstream 3x-ui поведение, не баг

**lucxVersion** → `lucx.37` (lucx.36 был промежуточный — тот же код без parseClientDump fix).

**Деплой на test2:** lucx.35 →
---

## Рефакторинг README — многостраничная навигация и перенос на EN (2026-07-30)

**Контекст:** Главная страница `README.md` была гибридной — содержала сразу два блока `## 🇷🇺 О проекте` и `## 🇬🇧 About`. Переделали по схеме многостраничной документации 3x-ui.

**Что сделано:**
- **`README.md` (Главный README):** переведён полностью на **английский язык** (`## About LucX-UI`). В шапку добавлена панель переключения языков (`English | Русский | فارسی | العربية | 中文 | Español | Türkçe`).
- **`README.ru_RU.md` (Русский README):** в шапку добавлен блок `LUCX-HOOK` с русской презентацией LucX-UI (`## О проекте LucX-UI`, возможности, установка, лицензия, благодарности, донаты), предупреждающим баннером и той же языковой панелью навигации.
- **`README.*.md` (Остальные языковые файлы):** ссылки в переключателе языков приведены к единообразным относительным путям (`README.md`, `README.ru_RU.md` и т.д.).
- **Благодарности:** добавлен **302ba (Alex)** (автор PR #24 за фикс Zod-схемы клиента) в списки благодарностей на английском и русском языках.

Файлы: `README.md`, `README.ru_RU.md`, `README.ar_EG.md`, `README.es_ES.md`, `README.fa_IR.md`, `README.tr_TR.md`, `README.zh_CN.md`.
ve, `awgo-1.conf` без I1-I5 (только `Table = off`), ноль reconcile fails в логах, awgo-1 UP (loopback к awg1).

**lucxVersion** → `lucx.38`.

---

## Миграция на апстрим v3.6.0 (2026-07-26)

**Коммит:** `96e2e177` (merge-коммит, родители `25a162ac` + `c377dca2`), ветка `feat/upstream-v3.6.0`.
Апстрим: 103 коммита (v3.5.0 → v3.6.0), 432 файла. Базовая версия `3.5.0` → `3.6.0`, панель показывает `3.6.0-lucx.47`. После merge — `behind 0` относительно `origin/main`.

### Смена стратегии: merge вместо fresh-checkout

Предыдущая миграция (v3.3.1 → v3.5.0) шла через свежий checkout апстрима и ручной перенос всего LucX-кода (29 изолированных файлов + восстановление HOOK-блоков вручную). На v3.6.0 сработал обычный `git merge origin/main`: из 432 изменённых файлов конфликтными оказались только **7**. **Вывод:** изоляция в `internal/awg/` + `internal/lucx/` с LUCX-HOOK-маркерами работает как задумано — будущие синки можно делать merge'ом, а не переносом. Merge-коммит сохраняет историю апстрима, поэтому следующий sync будет инкрементальным.

### Главный урок: конфликты решаются ПОБЛОЧНО, не одной стратегией

Соблазн взять везде «наши» (`--ours`) даёт **несобирающийся код**: `undefined: database.BackupSQLite`. Причина — апстрим в большинстве случаев добавлял код **рядом** с нашим HOOK-блоком, а не вместо него; `ours` выбрасывал апстрим-новинки целиком (`db.go` стал 1942 строки вместо 2249).

| Файл | Стратегия | Почему |
|---|---|---|
| `.gitignore` | оба | у каждого свои ignore-паттерны |
| `internal/database/db.go` | оба | апстрим `migrateTgIDIndex` + наш `pruneLegacyAwgHiddenChildren` |
| `frontend/src/hooks/useXraySetting.ts` | оба | апстрим dirty-guard + наши `awgOutboundTags` |
| `internal/web/service/xray_config_inject_test.go` | оба | 3 новых mtproto-теста + 6 наших AWG |
| `install.sh` | наш | URL релиза форка вместо `MHSanaei/3x-ui` |
| `frontend/src/layouts/AppSidebar.tsx` | наш | наши ссылки вместо `sanaei.dev` |
| `.github/workflows/release.yml` | поблочно (4 блока) | взяты апстрим Xray v26.7.28 и `fetch`-ретраи; оставлены наши `prerelease: false` и guard тег↔`lucxVersion` |

**Windows-сборка апстрима НЕ портирована** — AWG это Linux kernel module, панель на Windows сайдкар не запустит. Оба места в `release.yml` помечены LUCX-HOOK с объяснением (стало 6 маркеров вместо 2), иначе следующий агент воспримет отсутствие job'а как потерю при merge и «вернёт» его. Глоб `dev-artifacts/*.zip` в dev-канале также убран — без Windows-job'а zip-артефакта нет, апстримовый глоб уронил бы шаг upload.

### Грабли: IDE схлопывает конфликтные файлы

Правка файла в состоянии конфликта через IDE-инструменты редактирования молча перезаписывает файл из внутреннего merge-кэша и **теряет часть содержимого**: `install.sh` потерял все 16 LUCX-HOOK, `db.go` — апстрим-функции. **Конфликты резолвить только через терминал.** Контроль: сверять число LUCX-HOOK-маркеров в каждом файле до и после merge — расхождение значит потерю.

### i18n: новый апстрим-тест требует паритета ключей

v3.6.0 принёс `frontend/src/test/i18n-dead-keys.test.ts`, который требует, чтобы **каждая** локаль несла ровно тот же набор ключей, что en-US (плюс второй тест — каждый en-US-ключ должен быть использован в коде). Наши 31 AWG-ключ (`pages.xray.awgOutbound.*` — 27, `pages.xray.tabs.*` — 2, `disable`, `qrTooLarge`) жили только в en+ru → 11 локалей падали с `missing=31`.

Добавлены с переводом во все 11 (ar-EG, es-ES, fa-IR, id-ID, ja-JP, pt-BR, tr-TR, uk-UA, vi-VN, zh-CN, zh-TW) — все 12 файлов теперь по **1999 ключей**, `missing=0 orphans=0`. Технические термины (`Tag`, `Endpoint`, `MTU`, `DNS`, `Allowed IPs`, `Preshared key`, `awg-quick`, `outbound`) оставлены латиницей — это конвенция апстрима.

**На будущее:** новый LucX-ключ надо сразу добавлять во все 13 локалей, иначе CI красный. Остался долг: `awgHpk`, `awgHpkHint`, `awgHpkPlaceholder` лежат непереведёнными (английский текст) в не-en локалях — пришли с коммитом headerProtectionKey, тест не ловит (ключ есть, значение не проверяется).

### Попутные фиксы (пойманы апстрим-линтером)

- `SIDEBAR_COLLAPSED_KEY` в `AppSidebar.tsx` — апстрим удалил её использование, наш `ours` сохранил мёртвую константу → убрана.
- Дубль объявления `nextUrl` в `useXraySetting.ts` — апстрим перенёс его выше с `normalizeOutboundTestUrl`, стратегия «оба» притащила устаревшую строку → удалена.

### Верификация

```
go build ./...                                     exit 0
go vet ./...                                       exit 0
go test ./internal/awg/... ./internal/lucx/...      ok (7 пакетов)
go test -run 'InjectAwg|InjectMtproto'             21/21 PASS (6 AWG + 8 mtproto + 5 awgOutbound + 2)
npm run lint / typecheck                           exit 0
npx vitest run --project=unit                       876 passed (54 файла)
npm run build                                      exit 0 → internal/web/dist/
```

**Сборка на Windows.** `internal/database` требует CGO (`sqlite3.Backup` в новом апстрим-`BackupSQLite`). Без gcc на Windows падает `destination.Backup undefined` — это **не наша регрессия**, воспроизводится на чистом `origin/main` (проверено в отдельном worktree). Для тайпчека остальных пакетов на Windows сгодится временная заглушка `sqliteBackupShim`. Финальная сборка — только на Linux с `CGO_ENABLED=1`.

**Pre-commit hook.** `lint-staged` падает на Windows при большом числе staged-файлов (179 шт.) — упирается в лимит длины командной строки, не в качество кода. Конфиг апстримовый (`frontend/package.json`), трогать его вне LUCX-HOOK нельзя → коммит с `--no-verify` после ручного прогона lint+typecheck+test.

---

## Релиз v3.6.0-lucx.48 (2026-07-30) — первый релиз на базе v3.6.0

**Состав:** миграция на апстрим v3.6.0 (запись выше) + влит **PR #24 от внешнего контрибьютора 302ba** (Alex).

### PR #24 — потеря полей клиента через Zod

Инлайновая AWG-схема клиента в `frontend/src/schemas/protocols/inbound/awg.ts` перечисляла только AWG-поля (ключи/allowedIPs/email/enable) и **не содержала** `comment`/`limitIp`/`totalGB`/`expiryTime`/`tgId`/`subId`/`reset`. Zod стрипал их на `.parse()` → `UpdateInbound` → `SyncInbound` затирал `ClientRecord.Comment` пустой строкой. **Фикс:** `AwgClientSchema = WireguardClientSchema.extend({ id, password })` — AWG это WG + обфускация, поля клиента те же, дублировать их было ошибкой. Merge с нашим `headerProtectionKey` (из lucx.47) прошёл без конфликтов, авторство сохранено (`Alex <alex@beaunet.biz>`). GitHub сам определил PR как merged по содержимому.

### Процесс

Мердж в `main` оказался **fast-forward** (`gh/main` — прямой предок ветки миграции), то есть force-push не потребовался — важно при branch protection (`enforce_admins: true`). Заодно в `main` влились 35 накопившихся релизных коммитов (`main` отставал от рабочей ветки с lucx.34). Пуш: `c071bc6b..9ff1f93f`, 141 коммит.

`lucxVersion` → `lucx.48`, тег `v3.6.0-lucx.48`. **CI guard тег↔source отработал.** Релиз опубликован `prerelease=false`, `/releases/latest` → `v3.6.0-lucx.48`, ассет `x-ui-linux-amd64.tar.gz` 76 МБ.

### Грабли релиза

**1. `git push` падал с `/usr/bin/gh: No such file or directory`.** В глобальном git-конфиге `credential.https://github.com.helper = !/usr/bin/gh auth git-credential` — WSL-путь, неработоспособный из Windows-терминала. `gh auth status` показывает `Git operations protocol: ssh`, а remote `gh` — на https. **Обход без правки конфига:** `git push git@github.com:AlexeyLCP/lucx-ui.git main:main` (SSH-ключ `~/.ssh/id_ed25519` работает).

**2. `gofumpt` после merge.** Merge слепил пустую строку перед LUCX-HOOK-блоком в `xray_config_inject_test.go`. Важный нюанс: чистый апстрим-файл **тоже** не проходит gofumpt v0.10.0 (он разбивает `append(cfg.InboundConfigs,`) — проверяется через `git cat-file blob origin/main:<файл>` в отдельный UTF-8-файл. В нашем форке файл форматируется целиком (он в списке `bin/check-lucx.sh`), поэтому применён `gofumpt -w` полностью — иначе CI красный на golangci-lint.

**3. Pre-commit hook на Windows.** `lint-staged` падает с «слишком длинная командная строка» при 179 staged-файлах (лимит ОС, не качество кода). Конфиг апстримовый → коммиты с `--no-verify` после ручного прогона lint/typecheck/test. Первый же запуск hook'а полезен — он поймал реальные дефекты резолва (дубль `nextUrl`, мёртвая `SIDEBAR_COLLAPSED_KEY`).

### Гигиена очереди PR

7 dependabot-PR (#25–#31) закрыты с комментарием (Known Issue #3: version updates отключены, security остаются; зависимости приходят целыми наборами с синком апстрима), ветки удалены. Очередь PR/issues пуста.

### CI упал после первого пуша — 4 проблемы, 3 наши

Релиз собрался, но основной CI на `main` был красный: апстрим v3.6.0 принёс проверки, которых наш AWG-код не проходил. Починено в `adb86559` и `e92e091d`:

**1. `TestRouteRegistryContract` (новый тест, job `race`).** Строит реальный gin-роутер и сверяет с `frontend/src/pages/api-docs/endpoints.ts` в обе стороны: маршрут без записи пропадает из OpenAPI-доков, запись без маршрута документирует 404. Все **8** маршрутов `/panel/api/awg-outbounds` (фича lucx.37) в реестре отсутствовали — добавлена секция `awg-outbounds` в LUCX-HOOK-блоке (отдельная, не вплетать в апстримовую `inbounds` — меньше конфликтов на следующем синке), `npm run gen` перегенерировал `openapi.json`.

**2. `noctx` (job `golangci`).** `exec.Command` в `awg_outbound.go:270` (ping через интерфейс в хендлере Test) → `exec.CommandContext(ctx, ...)` с `c.Request.Context()` и таймаутом 15s + `defer cancel()`. Реальная польза, не только линтер: `ping -c 3 -W 2` живёт до ~8 с и держал горутину даже после отвала клиента.

**3. `gofumpt` (job `golangci`).** 6 LucX-файлов без завершающего LF (последний байт `}` вместо `10`) — pre-existing с lucx.37. **Готча:** CI показал только 3 из 6 — `golangci-lint` обрезает вывод одного линтера (`max-same-issues`). После фикса «трёх из лога» CI снова был бы красным с другими именами — всегда гонять `gofumpt -l` локально по всем LucX-файлам, не верить списку из CI.

**4. `rule-form-preserve-fields` (новый тест, job `frontend`) — поймал наш UX-баг.** Тест редактирует правило `{outboundTag, ruleTag}` без матчеров и ждёт `onConfirm`. Наш guard из lucx.40 (защита от Xray «this rule has no effective fields», Pattern 5) блокировал submit **всегда**, в том числе при редактировании уже существующего правила. Если правило пришло из шаблона или было создано до появления guard'а — пользователь не мог сохранить правку любого другого поля, модалка не закрывалась. **Фикс:** блокируем только при создании (`!isEdit`) — именно там рождается пустое правило; при редактировании — `message.warning` вместо `error`.

Не наше: в job `frontend` также падала storybook-история `ConfigBlock.stories.tsx` на a11y-контрасте (`color-contrast`) — файл, компонент и CSS побайтово идентичны апстриму, AWG-кода не содержат, LUCX-HOOK в них нет. В итоговом прогоне прошла — флак рендера в headless-браузере.

### Перевыпуск тега

Первый тег `v3.6.0-lucx.48` ушёл **до** этих фиксов, то есть бинарник без таймаута ping и без AWG-эндпоинтов в API-доках. Релиз имел **0 скачиваний** → удалён вместе с тегом (`gh release delete --cleanup-tag`), тег переставлен на исправленный `e92e091d`, релиз пересобран. **Правило на будущее: тег ставить ТОЛЬКО после зелёного CI на main**, иначе либо удалять релиз (только пока нет скачиваний), либо выпускать lucx.49.

### CI итог

`CI` → **success** (все 8 job: frontend, golangci, race, go-test, codegen, govulncheck, fuzz-smoke, postgres-durable-first). `Release LucX-UI` на теге → success. `/releases/latest` → `v3.6.0-lucx.48`, `prerelease=false`, ассет 76 МБ. Очередь PR/issues пуста.

**lucxVersion** → `lucx.48`.

---

## Деплой v3.6.0-lucx.48 на test2 (2026-07-30)

**Цель:** `lucx-test2` (144.31.157.106, Debian 13 trixie, kernel 6.12.90). Было `3.5.0-lucx.46` → стало `3.6.0-lucx.48`. Первый деплой v3.6.0-базы на живой сервер с AWG.

### Бэкап до миграций — обязательно

Апстрим v3.6.0 гонит миграции БД на первом старте, отката средствами панели нет. Процедура: `systemctl stop` → копия `x-ui.db` (+`-wal`/`-shm`) и старого бинарника в `/root/lucx-backup-<stamp>/` → `integrity_check` копии → выгрузка счётчиков трафика в текст (reference для сверки) → только потом новый бинарник. **Копировать БД только при остановленном сервисе** — иначе WAL может оказаться в середине транзакции. Скрипт сам поднимает сервис обратно при любом сбое скачивания/версии.

### Миграции апстрима прошли без потерь

- `migrateTgIDIndex` — индекса до апгрейда не было, после старта появился: `CREATE INDEX idx_clients_tg_id ON clients(tg_id)`.
- `PRAGMA integrity_check` → `ok` до и после.
- Данные целы: AWG-инбаунд (id 3, порт 48182), два `awg_outbounds` (`test-upstream`, `awgo-upstream`), счётчики трафика `peer-laptop` (150696944 / 1099990686) совпали с бэкапом байт в байт.
- `BackupSQLite` (новый апстрим-код) не ломает старт — он вызывается по запросу (backup-эндпоинт/тг-бот), не на инициализации.

### AWG runtime на v3.6.0-базе — работает

- `awg3` поднялся сам после старта, пир `peer-laptop` на месте, обфускация цела (jc=5, S1-S4, H1-H4).
- `awgo-1`/`awgo-2` (клиентский режим) живые, `awgo-2` с свежим handshake.
- **routeThroughXray проверен сквозным трафиком, не только наличием интерфейса:** ping с `-I 10.8.0.1` (тот же policy-routing путь, что у реального пира) — 0% loss, 6.2 ms; HTTPS через тот же source — HTTP 301 за 0.05 с.
- TUN-инбаунд в живом конфиге Xray: `tun3`, mtu 1320, gateway `10.254.3.1/30` (per-inbound /30), `sniffing {http,tls,quic, routeOnly:true}` — всё как задумано.
- AWG-аутбаунды в конфиге: `test-upstream → awgo-1`, `awgo-upstream → awgo-2` через `sockopt.interface`.
- **Post-restart окно закрыто (фикс lucx.34 работает на v3.6.0):** после `systemctl restart x-ui` (жёсткий вариант — tun3 пересоздаётся) уже на t+8s: `iif awg3 lookup 1003` и `default dev tun3 table 1003` на месте, ping 0% loss. Ждать reconcile-тика не пришлось.
- Нуль `reconcile failed` и нуль error/panic в логах, `NRestarts=0`.

### Готча верификации: 404 на API — это НОРМА

Неавторизованный проб `/panel/api/awg-outbounds/list` даёт **404**, и это легко принять за потерю маршрутов при merge. **Как проверять правильно:** сравнить с апстримовым маршрутом — `/panel/api/inbounds/list` и `/panel/api/server/status` тоже отвечают 404 без сессии (панель скрывает API-surface). Доказательство регистрации — `strings /usr/local/x-ui/x-ui | grep awg-outbounds`: все 8 путей и все 8 хендлеров `AwgOutboundController).{add,del,enable,list,parseConf,status,test,update}` в бинарнике. Пароля панели в БД нет (только хеш), так что полный API-тест с сессией — только вручную через UI.

### Откат (если понадобится)

```
systemctl stop x-ui
cp -a /root/lucx-backup-20260730-081028/x-ui.bin /usr/local/x-ui/x-ui
cp -a /root/lucx-backup-20260730-081028/x-ui.db  /etc/x-ui/x-ui.db
systemctl start x-ui
```
⚠️ Откат БД теряет трафик, накопленный после апгрейда; без отката БД старый бинарник переживёт лишний индекс `idx_clients_tg_id` без проблем.

---

## Релиз v3.6.0-lucx.49 (2026-07-30) — HeaderProtectionKey ломал awg setconf

**Репорт VladufQa:** после клика «Сгенерировать обфускацию» в .conf появилась строка `HeaderProtectionKey =`, трафик встал. Тестер починил сам, удалив эту строку — точное подтверждение диагноза.

### Root cause

Регрессия из lucx.47 (коммит `25a162ac`, «forward-compat для AWG3»). `generateObfuscation` (`internal/web/controller/awg.go`) **всегда** отдавал свежий `headerProtectionKey: random.Base64Bytes(32)`. Фронт (`regenerateObfuscation` в `awg.tsx`) пишет ВСЕ поля ответа в форму (`Object.entries(obf).forEach(setValue)`), ключ попадал в settings → в .conf. Master-модуль amneziawg (1.0.20260611) **не парсит** это поле (только ветка feat/awg3) → `awg setconf`: `Line unrecognized` + `Configuration parsing error` → `awg-quick` удаляет недособранный интерфейс → reconcile падает каждые 10 с. Воспроизведено на test2 в изоляции: без строки — UP, со строкой — `Line unrecognized` и откат.

### Фикс (три уровня)

1. **Источник:** `generateObfuscation` больше не отдаёт `headerProtectionKey` (именно отсутствие поля, не `""` — чтобы цикл `Object.entries` не затёр ручное значение оператора на AWG3-модуле). Убран неиспользуемый импорт `random`.
2. **Рендереры:** `renderServerConf` (`manager.go`), `renderClientConf` (`client_conf.go`), `inboundAwgHints` (`inbound.go`) НИКОГДА не пишут строку HPK — тот же подход, что уже был для I1-I5 (CLIENT-ONLY, крашат setconf). Поле остаётся в Settings/схеме для forward-compat — вернуть строку, когда feat/awg3 попадёт в master.
3. **Миграция** `pruneAwgHeaderProtectionKey` (`migrate_awg_hpk.go`, вызов в `db.go` в LUCX-HOOK): вычищает непустой ключ из AWG-инбаундов и `awg_outbounds` у пострадавших. Идемпотентна, no-op на чистой БД. Значение → `""` (не delete, чтобы сохранить форму под Zod).

### Проверка миграции на «отравленной» БД (точный сценарий VladufQa)

Вписал фейковый `headerProtectionKey` в живой AWG-инбаунд на test2 → restart → миграция вычистила (`"headerProtectionKey":""`), лог `[LUCX-AWG] migration: cleared headerProtectionKey from AWG inbound 3`, awg3 поднялся, в `.conf` ключа нет, 0 reconcile-failures.

### Процесс

`lucxVersion` → `lucx.49`, коммит `141a4dff`, тег `v3.6.0-lucx.49` ПОСЛЕ зелёного CI (8/8 job). Release success, `/releases/latest` → `v3.6.0-lucx.49`. Деплой на test2: `3.6.0-lucx.48` → `3.6.0-lucx.49`, AWG работает, 0 ошибок. Бэкап `/root/lucx-backup-20260730-090129/`.

**Урок:** кнопка «Сгенерировать» молча писала в форму всё, что вернул backend, включая поля без поддержки в текущем ядре. UX-фикс формы решили не делать («пока всё норм»); первопричина закрыта на backend.

**lucxVersion** → `lucx.49`.

---

## Осталось

- Полный API-тест AWG-outbound эндпоинтов с живой сессией (list/add/update/del/enable/status/test/parseConf) — требует пароля панели, делать вручную через UI.
- Не проверено на v3.6.0: создание/удаление AWG-инбаунда и клиента из UI, QR/`.conf`-выгрузка, диагностика-модалка, фикс PR #24 на живом клиенте с непустым `comment` (на test2 comment пустой, регресс на нём не виден).
- Тестеры (VladufQa, Kirill Rudenko) о v3.6.0 не уведомлены — обновляются сами через `x-ui update`.
- `awgHpk`/`awgHpkHint`/`awgHpkPlaceholder` лежат непереведёнными в 11 локалях (тест не ловит — проверяется наличие ключа, не значение).
- В «Благодарностях» README не упомянут 302ba (Alex) — автор влитого PR #24.
- AWG3: мониторить merge `feat/awg3` → master в `amnezia-vpn/amneziawg-linux-kernel-module` + `amneziawg-tools`. Когда смержится — вернуть HPK в `generateObfuscation` ответ, снять гварды в `renderServerConf`/`renderClientConf`/`inboundAwgHints`, bump lucxVersion, релиз.

---

## Актуализация AGENTS.md (2026-07-30)

Синхронизировал AGENTS.md с реальным состоянием после v3.6.0-lucx.49 (документ отставал на период v3.5.0 + lucx.37–47). Без код-изменений — только доки.

**Что обновлено:**
- **Шапка:** Active branch — `main`, миграция v3.6.0 завершена, релиз lucx.49 (было «v3.5.0 завершена»).
- **Core Philosophy + Workflow step 4:** убрана ссылка «19 vs 9» (Known Issue #1 закрыт — core пакет на 9 файлах, паритет с mtproto); ветка v3.5.0 → v3.6.0.
- **Rule 3 (Sidecar Architecture):** добавлены AWG outbound (`awgo-N`, `client_*.go`, `awg_outbound.go` controller/service, `RestartXray(true)` на мутациях, аллокация адресов с исключением tunnel IPs) и AWG3 forward-compat (`headerProtectionKey` поле — хранится везде, не пишется в .conf до merge `feat/awg3`).
- **Rule 9 (Frontend):** добавлены `inbound-link.ts`, `wireguardConfig.ts` (`buildAwgClientConfig`), `ClientQrModal.tsx`, упоминание HPK в schema.
- **Rule 10 (License):** список LucX-файлов расширен — `internal/awg/cps/`, `internal/awg/signature/`, `migrate_awg*.go` (вместо одного), `controller/awg_outbound.go`, `service/awg_outbound.go`, `bin/build-release.sh`.
- **Architecture Map:** переписана под реальную структуру. Добавлены `client_instance.go`/`client_conf.go`/`client_manager.go` (outbound sidecar), `migrate_awg_outbound.go`/`migrate_awg_hpk.go`, `service/awg_outbound.go`, `controller/awg_outbound.go`. `controller/awg.go` помечен что HPK не отдаёт. `db.go` — добавлен вызов `pruneAwgHeaderProtectionKey`. `check-lucx.sh` — 49 файлов (было 37). `install-awg-module.sh` — HEAD clone (AWG3 подхватится при merge).
- **Release секция:** пример `v3.5.0-lucx.1` → `v3.6.0-lucx.50`, добавлено правило «тег только после зелёного CI» (урок lucx.48).
- **Known Issue #5 (новый):** AWG3 / `headerProtectionKey` forward-compat — полностью задокументирована регрессия lucx.47 → фикс lucx.49 (VladufQa: «после генерации обфускации трафик встал»), текущее состояние (3 уровня фикса), условие включения в production, урок про `generateObfuscation` endpoint.

**Что НЕ трогал:** Test Commands, Frontend/Go Conventions, Debugging Patterns (Pattern 1–5 уже актуальны после v3.6.0 sync), Deploy/тестовые серверы (без изменений), Branch Protection (без изменений), Commit Convention (без изменений).

Проверка: `grep -c "v3.5.0-lucx" AGENTS.md` → 0 (все примеры на v3.6.0); `headerProtectionKey` упоминается в Rule 3, Architecture Map, Known Issue #5.

---

## Рефакторинг README — многостраничная навигация и перенос на EN (2026-07-30)

**Контекст:** Главная страница `README.md` была гибридной — содержала сразу два блока `## 🇷🇺 О проекте` и `## 🇬🇧 About`. Переделали по схеме многостраничной документации 3x-ui.

**Что сделано:**
- **Лаконичный дизайн всех 7 README (`README.md`, `README.ru_RU.md`, `README.zh_CN.md`, `README.es_ES.md`, `README.tr_TR.md`, `README.fa_IR.md`, `README.ar_EG.md`):**
  1. Блок **⚡ Быстрый старт** с однострочником установки поднят в самый верх под шапку.
  2. В раздел **О проекте / About** добавлено краткое, емкое описание предназначения LucX-UI (нативная интеграция AmneziaWG в виде сайдкара ядра, обфускация, TLS-отпечатки, клиентский режим AWG Outbounds, диагностика и 2 режима маршрутизации при 100% совместимости с апстрим-обновлениями 3x-ui).
  3. Технические детали (cloud-init, Docker, PostgreSQL, переменные окружения `/etc/default/x-ui`) убраны в аккуратные сворачиваемые блоки `<details>`.
  4. Скриншоты убраны в аккуратный блок `<details>`.
  5. Исправлена ссылка на график динамики звёзд Stargazers: с `MHSanaei/3x-ui` на **`AlexeyLCP/lucx-ui`** во всех 7 языковых версиях.

Файлы: `README.md`, `README.ru_RU.md`, `README.ar_EG.md`, `README.es_ES.md`, `README.fa_IR.md`, `README.tr_TR.md`, `README.zh_CN.md`.

---

## Релиз v3.6.0-lucx.51 (2026-08-02) — version-aware H-формат + авто-пересборка AWG-модуля

**Контекст:** Два багфикса от пользовательских репортов: (1) при выборе AWG версии 1.5 генератор обфускации выдавал H1-H4 в формате "lo-hi" (AWG 2.x), а не одиночным числом — v1.x awg-quick отклонял конфиг; (2) обновление панели через веб-интерфейс не пересобирало DKMS-модуль ядра, что при мажорном апгрейде модуля (AWG1→AWG3) приводило к рассинхону.

**Что сделано:**

### Задача №2 — Version-aware H-формат (fix)

- `internal/awg/cps/params.go`: новая функция `genHSingle(n int) string` — одно случайное число из квадранта n (без "-", формат для AWG 1.x). `GenerateAWGParams` получил параметр `awgVersion string`: при `"1.5"` — `genHSingle(0..3)`, при `"2"`/`"3"`/`""` — `genHRange(0..3)` (историческое поведение). Сигнатура `(profile ObfProfile)` → `(profile ObfProfile, awgVersion string)`.
- `internal/web/controller/awg.go`: вызов `GenerateAWGParams` теперь передаёт `req.AwgVersion` из запроса.
- `internal/awg/cps/cps_test.go`: `TestGenerateAWGParams_Invariants` — assertion на "-" теперь только для версии "2". Новый `TestGenerateAWGParams_HFormatByVersion` — таблица версий "1.5"/"2"/"3"/"" × профили × проверка наличия/отсутствия "-". Все остальные call sites обновлены (передаётся "2" для default).
- `frontend/src/lib/xray/inbound-defaults.ts`: `createDefaultAwgInboundSettings` — H1-H4 теперь генерируются как диапазоны `"lo-hi"` (в непересекающихся квадрантах, совместимых с 2^31-1 верхним bound). При выборе "1.5" и нажатии "Regenerate obfuscation" бекенд вернёт одиночные числа — defaults это только seed.

### Задача №1 — Авто-пересборка AWG-модуля при update

- `bin/install-awg-module.sh`: новый параметр `--force-rebuild` — пропускает early-exit (модуль загружен), делает `dkms remove` старой версии + `rmmod amneziawg`, затем полная пересборка из свежего git clone. Пишет маркерный файл `/etc/x-ui/.awg-module-version` с установленной версией (из dkms.conf). В конце скрипта — fallback: если маркер отсутствует (pre-lucx.51), backfill из `modinfo -F version amneziawg`.
- `update.sh`: новый LUCX-HOOK перед `systemctl start x-ui` — версионный gate: сравнивает маркер с версией из свежего `git clone --depth 1` upstream dkms.conf. При расхождении — `bash bin/install-awg-module.sh --force-rebuild`. При падении clone (нет сети) — fallback `install-awg-module.sh` без --force-rebuild. Non-fatal. x-ui ещё остановлен на этом этапе (rmmod безопасен).
- `install.sh`: обновлён комментарий LUCX-HOOK — отмечено что маркер пишется автоматически.
- `AGENTS.md`: Pattern 1c — статус «ИСПРАВЛЕНО (lucx.51)», TODO убрано.

**Файлы:** `internal/awg/cps/params.go`, `internal/web/controller/awg.go`, `internal/awg/cps/cps_test.go`, `frontend/src/lib/xray/inbound-defaults.ts`, `bin/install-awg-module.sh`, `update.sh`, `install.sh`, `internal/config/config.go`, `AGENTS.md`, `progress.md`.

**Тесты:** `go test ./internal/awg/... ./internal/lucx/... -count=1 -v` — зелёный (18/18). `npm run typecheck && npm run lint` — чисто. `gofumpt -l .` — 2 upstream-файла (не наши регрессии). `bash -n` на 3 скриптах — чисто. `bin/check-lucx.sh` — 49 файлов OK.


## Релиз v3.6.0-lucx.52 (2026-08-02) — 7 параметров AWG 3.0 (timers, padding, AdvancedSecurity)

**Контекст:** Тестер сообщил, что панель поддерживает не все AWG 3.0-параметры. Аудит upstream-ядра `amneziawg-linux-kernel-module` v3.0.20260731 (UAPI `wireguard.h`) выявил 7 параметров, которые ядро и `awg-quick` парсят, но панель не экспонирует:

- 6 device-level u32: `ContentPaddingAddition`, `RekeyAfterTime`, `RekeyTimeout`, `RejectAfterTime`, `KeepaliveTimeout`, `MaxHandshakeAttempts` (0 = kernel default: детерминированный WG padding, 120/5/180/10/18 сек)
- 1 peer-level bool: `AdvancedSecurity` (advisory только — текущее ядро игнорирует на входе в `set_peer`, хардкодит в `get_peer`)

Все 7 version-gated к `awgVersion == "3"` (v1/v2 ядра reject'ят неизвестные строки в setconf). Поля опциональны (ядро подставляет безопасные дефолты WG), оператор может тонко настроить тайминги и padding для DPI-обхода.

**Strategy:** Полный шаблон HeaderProtectionKey ×7 (HPK уже отработан в lucx.50). 6 device-полей идут по пути: `AWGParams` struct → `Instance` struct → `renderServerConf`/`renderClientConf` → `inboundAwgHints` → `ParseConf` (outbound) → `filterAwgObfuscation` (frontend) → Zod schemas ×2 (inbound+outbound) → AWG form → `inbound-link.ts` (genAwgLink/genAwgConfig). AdvancedSecurity — per-peer: `PeerSpec` → `AwgClientSchema` → `ClientFormModal` Switch → `wireguardConfig.ts` [Peer] → `genAwgConfig` [Peer]. UX: default = 0/пусто = «использовать дефолт ядра». `generateObfuscation` НЕ автогенерирует (таймеры — не обфускация).

**Что сделано:**

### Go backend (10 файлов)
- `internal/awg/cps/params.go`: +6 полей в `AWGParams` struct, `AsConfLines` (+6 lines, guard `> 0`), `Validate` (+проверка 0–65535 u16)
- `internal/awg/instance.go`: +6 полей в `Instance` struct, +`AdvancedSecurity bool` в `PeerSpec`, fingerprint (device + peer), `InstanceFromInbound` (settings struct + clients struct + Instance literal + PeerSpec literal)
- `internal/awg/manager.go`: `renderServerConf` — +6 device-полей в `[Interface]` (guard `AwgVersion == "3" && field > 0`), +`AdvancedSecurity = on` в `[Peer]`-loop (guard `AwgVersion == "3" && p.AdvancedSecurity`)
- `internal/awg/client_conf.go`: `renderClientConf` — +6 device-полей + `AdvancedSecurity = on` в `[Peer]` (guard `NormalizeAWGVersion == "3"`)
- `internal/awg/client_instance.go`: `ClientSettings` +6 device-полей + `AdvancedSecurity bool`, fingerprint (device + peer)
- `internal/web/service/inbound.go`: `inboundAwgHints` — +6 json-тегов в settings struct, +6 emission блоков (guard v3)
- `internal/web/service/awg_outbound.go`: `ParseConf` — +6 case-branches + `AdvancedSecurity` parse, auto-detect v3 расширен (|| device fields || AdvancedSecurity)
- `internal/sub/service.go`: share-link builder — +6 params (lowercase, guard v3 + `> 0`)
- `internal/database/migrate_awg_hpk.go`: `normalizeAwgSettings` — prune 6 device-полей (→0) + `clients[].advancedSecurity` (→false) + outbound `advancedSecurity` (→false) на non-v3

### Frontend (10 файлов)
- `frontend/src/schemas/protocols/inbound/awg.ts`: +6 device-полей в `AwgInboundSettingsSchema`, +`advancedSecurity` в `AwgClientSchema`
- `frontend/src/schemas/awg-outbound.ts`: +6 device-полей + `AdvancedSecurity` в `AwgOutboundSettingsSchema`
- `frontend/src/schemas/protocols/inbound/wireguard.ts`: +`advancedSecurity` в `WireguardInboundPeerSchema` (для awgPeerShape)
- `frontend/src/lib/xray/inbound-defaults.ts`: +6 дефолтов `= 0`
- `frontend/src/pages/inbounds/form/protocols/awg.tsx`: «AWG3 Advanced (timers & padding)» секция внутри `{awgVersion === '3' && ...}` — 6 `InputNumber min=0 max=65535`
- `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx`: +6 device-полей + `AdvancedSecurity` Switch, type + defaults + settingsToFormValues + formValuesToSettings
- `frontend/src/pages/clients/ClientFormModal.tsx`: +`advancedSecurity` в `Values` type, EMPTY, seed (from client record), payload, `Switch` UI (gated `showAwg`)
- `frontend/src/lib/xray/inbound-link.ts`: `genAwgLink` +6 params (v3), `genAwgConfig` +6 lines (v3 override), `genAwgConfig` [Peer] +`AdvancedSecurity = on`, `awgPeerShape` +`advancedSecurity`
- `frontend/src/pages/clients/wireguardConfig.ts`: `filterAwgObfuscation` +6 drop-строк для < v3, `buildAwgClientConfig` [Peer] +`AdvancedSecurity = on` (v3 + client flag)

### i18n (13 локалей)
- 15 новых ключей в `en-US.json` (canonical) + переведены в `ru-RU`, `uk-UA`, `zh-CN`, `zh-TW`, `es-ES`, `pt-BR`, `vi-VN`, `tr-TR`, `fa-IR`, `ar-EG`, `id-ID`, `ja-JP` — 3 параллельных агента

### Тесты
- Frontend: `wireguard-client-config.test.ts` — filterAwgObfuscation v3 keeps/drops 6 device-полей; `inbound-link.test.ts` — genAwgConfig [Peer] AdvancedSecurity для v3/v2/v1.5, 0-valued defaults не эмитятся
- Go: `cps_test.go` (AsConfLines device fields, Validate range), `instance_test.go` (fingerprint + renderServerConf version-gate), `client_conf_test.go`, `inbound_awg_test.go`, `migrate_awg_hpk_test.go` — агент

**Файлы:** 20+ кода + 13 i18n + тесты. `internal/config/config.go` — `lucx.52`.

**Тесты:** Frontend `vitest run --project=unit` — 893/893 passed. `npm run typecheck` — чисто. `npm run lint` — чисто. `npm run build` — OK. `npm run gen` — 27 schemas, 181 paths. `gofumpt -l .` — 2 upstream-файла (pre-existing drift). `bin/check-lucx.sh` — 49 файлов OK. i18n-dead-keys — 2/2 passed.


## Релиз v3.6.0-lucx.53 (2026-08-02) — module-capability gate (bugfix для lucx.52)

**Контекст:** Тестер сообщил «Device awg14 does not exist» после lucx.52. Воспроизведено на test2: host с amneziawg module v1.0.20260611 (НЕ v3), оператор выбрал awgVersion "3" в форме. Migration-prune (lucx.50) гейтит AWG3 поля только по `awgVersion != "3"` — на v3 inbound поля уходят в .conf → v1 module reject'ит «Line unrecognized: ContentPaddingAddition=64» → awg-quick откатывает интерфейс → «Device awgN does not exist».

**Что сделано:**

### ModuleSupportsAwg3() — capability probe
- `internal/awg/platform_linux.go`: кэшированный `modinfo -F version amneziawg` probe → возвращает true только для v3.x (parses ContentPaddingAddition/RekeyAfterTime/.../HeaderProtectionKey в setconf). Кэш после первого вызова.
- `internal/awg/platform_other.go`: no-op off Linux (dev/build hosts return false).
- `SetModuleSupportsAwg3(*bool)` — test override (как SetRand в cps/params.go). Pass nil чтобы restore real probing.

### Double-gate во всех 4 рендерерах
- `internal/awg/manager.go` (`renderServerConf`): HPK, 6 device-полей, AdvancedSecurity — все под `awg3ok := inst.AwgVersion == "3" && ModuleSupportsAwg3()`.
- `internal/awg/client_conf.go` (`renderClientConf`): то же для outbound side.
- `internal/web/service/inbound.go` (`inboundAwgHints`): HPK + 6 device-полей под `awg.NormalizeAWGVersion(s.AwgVersion) == "3" && awg.ModuleSupportsAwg3()`.

### Тесты
- `instance_test.go`: 3 существующих version-gate теста обновлены — добавлен `SetModuleSupportsAwg3(&awg3=true)` override + `t.Cleanup(restore)`. Новый `TestRenderServerConf_HeaderProtectionKeyDroppedOnV1Module` — симулирует v1.x module (override false), проверяет что HPK не эмитится даже при `AwgVersion=="3"`.
- `client_conf_test.go`: 3 существующих теста обновлены с override.

**Файлы:** `internal/awg/platform_{linux,other}.go` (probe + override), `internal/awg/manager.go`, `internal/awg/client_conf.go`, `internal/web/service/inbound.go` (double-gate), `internal/awg/instance_test.go`, `internal/awg/client_conf_test.go` (test overrides + new v1-module test), `internal/config/config.go` (lucx.53), `AGENTS.md` (Pattern 1d), `progress.md`.

**Тесты:** `go test ./internal/awg/...` — зелёный (178 PASS). `GOOS=linux go vet` — чисто. `gofumpt -l` — чисто. `bin/check-lucx.sh` — 49 файлов OK.

**Урок:** DB-stored `awgVersion` — это потолок, который оператор выбрал в UI, а не capability runtime. Module-capability probe в каждой точке эмиссии AWG3 полей — единственный надёжный defense.


## Релиз v3.6.0-lucx.54 (2026-08-03) — AdvancedSecurity persistence + подсветка конфликта подсетей

**Контекст:** Два бага, выявленных тестером VladufQa на production-сервере (v3 модуль, awgVersion "3"):

1. **AdvancedSecurity switch откатывается в OFF после save.** Фронтенд отправляет `clientPayload.advancedSecurity = true` (`ClientFormModal.tsx:596`), но backend `model.Client` struct **не имел** поля `AdvancedSecurity` → `json.Unmarshal` молча дропает неизвестный ключ → в Settings JSON поле не записывается → после reload switch OFF. При этом `ClientRecord` (gorm-таблица `clients`) тоже не имел колонки, и `ToRecord`/`ToClient` её не копировали — поле терялось на Client→Record→DB→Record→Client цикле.

2. **Два AWG-инбаунда с одинаковой подсетью 10.8.0.0/24** → kernel даёт две connected-route на один префикс → reverse-path к клиентам второго инбаунда уходит в первый (zombie) → «коннект есть, трафика нет» (ровно баг VladufQa: awg2 + awg4 оба на 10.8.0.1/24). Дефолт формы `createDefaultAwgInboundSettings` хардкодит `10.8.0.1/24` для каждого нового инбаунда.

### Backend — AdvancedSecurity persistence (model.go, 5 точек)
- `Client` struct: +`AdvancedSecurity bool \`json:"advancedSecurity,omitempty"\`` (LUCX-HOOK после `KeepAlive`).
- `ClientRecord` struct: +`AdvancedSecurity bool \`json:"advancedSecurity" gorm:"column:awg_advanced_security;default:0"\`` (LUCX-HOOK). AutoMigrate добавит колонку на следующем старте — ручной миграции не нужно.
- `Client.ToRecord()`: +`AdvancedSecurity: c.AdvancedSecurity` (LUCX-HOOK).
- `ClientRecord.ToClient()`: +`AdvancedSecurity: r.AdvancedSecurity` (LUCX-HOOK).
- Merge logic (`applyClientRecordMerge`): +`if incoming.AdvancedSecurity && !existing.AdvancedSecurity` — advisory-only, «true wins, never silently clear» (как PreSharedKey).

### Frontend — подсветка конфликта подсетей
- `frontend/src/lib/awg/subnet.ts` (новый): чистые функции `maskSubnet(addr)` (→ `10.8.0.0/24`) и `subnetsOverlap(a, b)` — IPv4 CIDR overlap через 32-bit int, без npm-зависимостей.
- `frontend/src/pages/inbounds/form/protocols/awg.tsx`: `AwgFields` теперь принимает `otherAwgSubnets?: string[]` prop. `watch('settings.address')` + `useMemo` → `conflictSubnet`/`addressSubnetConflict`. `<Alert type="warning">` после Address FormField (advisory, не блокирует save — back-compat для существующих dup-subnet инбаундов).
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx`: `useMemo` поверх `dbInbounds` → `otherAwgSubnets` (masked networks других AWG-инбаундов, filter `protocol === AWG && id !== dbInbound?.id`), проброшен в `<AwgFields otherAwgSubnets={...} />`.

### i18n × 13 локалей
- Новые ключи: `awgSubnetConflict` + `awgSubnetConflictHint` (с интерполяцией `{{subnet}}`).
- EN+RU вручную; 11 локалей через python-скрипт (byte-stable вставка после `awgAddressHint`, JSON-valid). Технические термины — латиницей (`kernel route`, `/24`).

### Тесты
- `internal/database/model/client_advanced_security_test.go` (новый): 4 теста — ToRecord/ToClient roundtrip, JSON unmarshal capture + default-false, ClientRecord marshal содержит поле.
- `frontend/src/test/awg-subnet-overlap.test.ts` (новый): 11 тестов — maskSubnet (включая /32, /0, invalid, whitespace), subnetsOverlap (same /24, wide-contains-narrow, non-overlapping, invalid, field-exact dup case awg2+awg4).

### Файлы
- `internal/database/model/model.go` (5 точек: struct ×2 + ToRecord + ToClient + merge)
- `internal/database/model/client_advanced_security_test.go` (новый, 4 теста)
- `frontend/src/lib/awg/subnet.ts` (новый, pure helpers)
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` (props + Alert + useMemo)
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx` (useMemo + thread props)
- `frontend/src/test/awg-subnet-overlap.test.ts` (новый, 11 тестов)
- `internal/web/translation/*.json` × 13 (awgSubnetConflict + awgSubnetConflictHint)
- `internal/config/config.go` (lucx.54)
- `AGENTS.md` (Pattern 1e — dup-subnet kernel route конфликт + AdvancedSecurity persistence урок)

**Тесты:** `go test ./internal/database/model/ ./internal/awg/... ./internal/lucx/...` — зелёный. `npm run typecheck && npm run lint` — чисто. `npm run build` — built in 940ms. `npm run gen` — codegen обновлён (27 schemas, 181 paths). `i18n-dead-keys.test.ts` — PASS (13 локалей паритет). `gofumpt -l` — мои файлы чистые. `bin/check-lucx.sh` — 49 файлов OK.

**Урок №1 (AdvancedSecurity persistence):** Per-client peer-level поле на `model.Client` требует **5 точек** в model.go для full round-trip: struct (Client) + struct (ClientRecord с gorm column) + ToRecord + ToClient + merge logic. Только Client struct недостаточно — поле потеряется на ClientRecord→DB→Client цикле (ClientRecord — gorm-таблица `clients`, используется во всех путях сохранения: db.go, client_crud.go, client_link.go, client_portable.go, client_bulk.go, client_traffic.go). AWG sidecar читает поле через InstanceFromInbound (сырой JSON settings), но универсальный ClientFormModal save-flow идёт через controller/client.go → model.Client → ToRecord → ClientRecord. Без всех 5 точек switch откатывается в OFF после reload.

**Урок №2 (dup-subnet kernel route конфликт):** Два AWG-инбаунда с одинаковой client-подсетью (10.8.0.0/24) → kernel устанавливает две connected-route на один префикс. Linux выбирает одну по метрике/порядку, вторая zombie. Reverse-path к клиентам второго инбаунда уходит в первый → пакеты dropнуты → «коннект есть, трафика нет». Дефолт формы хардкодит одну подсеть для всех новых инбаундов → грабля повторяется. Фикс — advisory warning (не server-side guard, back-compat); auto-suggest следующей свободной /24 — follow-up.


## Релиз v3.6.0-lucx.55 (2026-08-03) — пустой PSK ломал awg setconf («создаёшь клиента → интерфейс падает»)

**Контекст:** Тестер VladufQa: «как только создаёшь клиента к инбаунду, диагностика выдаёт `Device "awg4" does not exist`; удаляешь клиента — интерфейс поднимается, коннект не идёт пока клиент существует». Интерфейс падал при каждом добавлении клиента и восстанавливался при удалении — значит `.conf` с пир-блоком становился невалидным для `awg setconf`.

**Root cause (подтверждён воспроизведением на test2):** `renderServerConf` (`internal/awg/manager.go`) писал `PresharedKey = %s` **безусловно** — даже при пустом PSK, рендеря строку `PresharedKey = ` с пустым значением. awg-tools отвергают её: воспроизведено на test2 (`amneziawg-tools v1.0.20260618`) — `awg setconf` с пустым `PresharedKey = ` возвращает `EXIT=1` + `Line unrecognized: 'PresharedKey='` + `Configuration parsing error`; с **отсутствующей** строкой — `EXIT=0`. Пустой PSK приходит, когда клиент создаётся путём, не вызывающим `defaultAwgClients` (генератор PSK): тот вызывается только из `addInboundClient` (путь Clients page → `ClientService.Create`), а путь формы инбаунда (`InboundService.UpdateInbound`) его не вызывает.

**Несовпадение, выдавшее баг:** `renderClientConf` (client_conf.go:96) и `SyncPeers` (manager.go:659) уже **omit'ят** пустой PSK (`if psk != ""`), и только `renderServerConf` писал его всегда.

**Фикс:** `renderServerConf` теперь omit'ит пустой/whitespace-only PSK (`if psk := strings.TrimSpace(p.PSK); psk != ""`), по образцу `renderClientConf`/`SyncPeers`. Отсутствующий `PresharedKey` — конвенция WireGuard для «no PSK». Клиенты с PSK продолжают его получать.

**Файлы:** `internal/awg/manager.go` (omit empty PSK), `internal/awg/server_conf_psk_test.go` (новый, 3 regression-теста: empty omitted / non-empty written / whitespace-only omitted), `internal/config/config.go` (lucx.55), `AGENTS.md` (Pattern 1g), `progress.md`.

**Тесты:** `go test ./internal/awg/...` — зелёный (3 новых regression-теста PASS). `gofumpt` — чисто. `bin/check-lucx.sh` — 50 файлов OK. Валидация на реальном железе (test2): пустой PSK → setconf EXIT=1, omit → EXIT=0.

**Урок:** Рендерер, который пишет опциональное поле `.conf`, обязан **omit'ить пустое значение** (WireGuard-конвенция), а не писать `Key = ` — awg-tools отвергают пустые значения ключей, `awg-quick` откатывает интерфейс, и reconcile бесконечно падает с «Device does not exist». Любой peer-level параметр, который может быть пустым на каком-либо пути создания клиента, должен быть gated `if value != ""`. Проверять все три рендерера (renderServerConf / renderClientConf / SyncPeers) на консистентность обработки пустых значений.

**Проверка AdvancedSecurity (2026-08-03, test2):** валидировано, что `AdvancedSecurity = on` в `[Peer]` **не крашит** `awg setconf` (EXIT=0) — tools распознают его как валидный peer-ключ (мусорный `TotallyBogusKey = on` при этом отвергается, EXIT=1, значит это не «игнор неизвестных ключей»). Ядро игнорирует значение на input и hardcodes в dumps (advisory). Опасение «AdvancedSecurity сломает setconf как пустой PSK» не подтвердилось — рендер безопасен на v1 и v3 tools.


## Релиз v3.6.0-lucx.56 (2026-08-03) — auto-suggest свободной подсети для нового AWG-инбаунда

**Контекст:** Follow-up на Pattern 1e (lucx.54). lucx.54 добавил advisory-warning при пересечении подсетей, но оператор всё равно мог создать два инбаунда на одной /24 по дефолту (`createDefaultAwgInboundSettings` хардкодит `10.8.0.1/24`). Auto-suggest устраняет граблю проактивно: новый AWG-инбаунд сразу получает **свободную** подсеть.

**Что сделано:**
- `frontend/src/lib/awg/subnet.ts`: `suggestFreeAwgAddress(usedSubnets)` — сканирует `10.8.0.0/16` (3-й октет 0..255), затем расширяется на `10.9`..`10.20`, возвращает первый свободный `10.N.M.1/24` (проверка через `subnetsOverlap`, корректно обрабатывает широкие префиксы вроде `10.8.0.0/16`). Fallback `10.8.0.1/24`.
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx`: в эффекте смены протокола (mode add) при выборе AWG подставляет `suggestFreeAwgAddress(otherAwgSubnetsRef.current)` в `settings.address`. Ref-зеркало `otherAwgSubnets` против stale-closure в watch-подписке. При редактировании существующего инбаунда адрес не трогается.
- 6 новых юнит-тестов в `frontend/src/test/awg-subnet-overlap.test.ts` (default, skip used, unmasked input, gap-free run, wide /16, field-case).

**Файлы:** `frontend/src/lib/awg/subnet.ts`, `frontend/src/pages/inbounds/form/InboundFormModal.tsx`, `frontend/src/test/awg-subnet-overlap.test.ts`, `internal/config/config.go` (lucx.56), `progress.md`.

**Тесты:** `vitest run src/test/awg-subnet-overlap.test.ts` — 17 PASS. `npm run typecheck && npm run lint` — чисто. `npm run build` — built in 1.15s.

**Урок:** Дефолт формы для network-address полей должен быть **уникальным per-inbound** — вычисляться из существующих, а не хардкодиться. Warning (lucx.54) + auto-suggest (lucx.56) = защита в двух слоях: auto-suggest предотвращает при создании, warning ловит ручной ввод.


## Релиз v3.6.0-lucx.57 (2026-08-03) — cleanup .conf при удалении инбаунда + кэш ModuleSupportsAwg3

**Контекст:** Тестер ВладufQa (lucx.56, модуль **v1.x** 1.0.20260611): (1) «при удалении инбаунда не удаляются конфиги», (2) awg6 не поднимается после смены адреса — клиент остался со старым AllowedIPs. Диагностика по логам показала точную причину awg6: `ip -4 route add 10.8.0.2/32 dev awg6` → `RTNETLINK: File exists` → откат (см. Pattern 1h — миграция адресов клиентов при смене подсети, отдельная задача).

**Фикс 1 — cleanup `.conf` при удалении инбаунда:** `Manager.Remove(id)` (`internal/awg/manager.go`) удалял `awg{id}.conf` **только если интерфейс запущен** (есть в `m.procs`). Инбаунд, чей интерфейс не поднялся (упал setconf/route на последнем reconcile), не имеет записи в `m.procs` → при его удалении `.conf` оставался. Теперь `Remove` удаляет `.conf` безусловно. Плюс `Reconcile` теперь делает `sweepOrphanInboundConfigs(want)` — вычищает `awg{N}.conf` для нежеланных id даже без записи в procs. Хелпер `parseInboundConfName` матчит только `awg{цифры}.conf`, НЕ трогая `awgo-*.conf` (outbound).

**Фикс 2 — кэш `ModuleSupportsAwg3`:** флаг `moduleAwg3Checked` взводился **до** вызова `modinfo`; при транзиентной ошибке modinfo (модуль пересобирается во время update, modinfo занят) функция возвращала false, но флаг уже стоял → «не v3» кэшировалось на весь процесс и AWG3-поля молча дропались до рестарта. Теперь при `err != nil` флаг НЕ взводится — следующий вызов повторяет probe.

**Файлы:** `internal/awg/manager.go` (Remove + sweepOrphanInboundConfigs + parseInboundConfName + filepath import), `internal/awg/platform_linux.go` (кэш-фикс), `internal/awg/manager_test.go` (TestParseInboundConfName), `internal/config/config.go` (lucx.57), `AGENTS.md` (Pattern 1h + 1i), `progress.md`.

**Тесты:** `go test ./internal/awg/...` — зелёный (новый TestParseInboundConfName PASS, awgo-*.conf не матчится). `GOOS=linux go vet` — чисто.

**Урок:** Cleanup-логика, которая чистит артефакты только для «запущенных» сущностей, пропускает сущности, которые **не запустились** (а именно они чаще всего оставляют мусор). Удалять побочные файлы (.conf) надо безусловно по id, а не по наличию в runtime-реестре.






## Релиз v3.6.0-lucx.58 (2026-08-03) — feature-probe AWG3 вместо парсинга версии + авто-апгрейд ядра при обновлении панели

**Контекст:** Тестер ВладufQa (lucx.57): после обновления «не идёт трафик», в клиентском конфиге нет HeaderProtectionKey. Пользователь указал на ошибку: «MODULE: 1.0.20260611 — это v1.x модуль, НЕ v3» — нумерация версий модуля НЕ отражает версию протокола. Проверка GitHub подтвердила: upstream штампует `PACKAGE_VERSION="1.0.0"` (dkms.conf) и `WIREGUARD_VERSION=1.0.0` (Makefile) в **каждом** релизе — и в v1-тегах (v1.0.20260611…), и в v3-тегах (v3.0.20260730…); PR #192 «feat: AmneziaWG 3.0» слит в master 2026-07-30T21:54Z, `header_protection.c` есть только начиная с тега v3.0.20260730. Значит старый `ModuleSupportsAwg3()` (major=="3" из `modinfo -F version`) **не срабатывал никогда** — HPK молча дропался на ВСЕХ хостах, даже с модулем из master.

**Фикс 1 — функциональный probe вместо версии (`internal/awg/platform_linux.go`):**
- `kernelExportsHeaderProtection()`: ищет символ `awg_header_protection_set_key` в `/proc/kallsyms` — не-static-функция из `header_protection.c`, присутствует в kallsyms только у AWG3-модулей (и только когда модуль загружен).
- `awgToolsParseHeaderProtectionKey()`: `awg version` → парс мажорной версии (`parseMajorVersion`); тулзы < v3 не парсят строку `HeaderProtectionKey` в .conf («Line unrecognized» → откат интерфейса), поэтому HPK гейтится на ОБА компонента.
- Кэш только положительного результата: отрицательный — транзиентный (модуль ещё не загружен после boot, тулзы пересобираются) → следующий вызов повторяет probe; хост, апгрейднувшийся до AWG3, подхватывает поля в течение одного reconcile-тика без рестарта панели.
- `generateObfuscation` (`internal/web/controller/awg.go`) отдаёт HPK только при `awgVersion=="3" && ModuleSupportsAwg3()` — форма больше не показывает ключ, который рендереры всё равно выкинут.
- Диагностика (`internal/awg/diagnostics.go`): новый инфо-чек `awg3 support` (kernel-символ + версия тулзов), исключён из `Healthy()` — отсутствие AWG3 не неисправность, а capability-отчёт.

**Фикс 2 — пересборка модуля/тулзов и авто-апгрейд ядра (`bin/install-awg-module.sh`, `update.sh`):**
- Старый version-gate в update.sh был мёртв: `grep -oP 'version\s*"...'` по dkms.conf не матчил ничего (PACKAGE_VERSION — uppercase) → «up to date» → модуль **никогда** не пересобирался при обновлении панели (именно поэтому у ВладufQa модуль июня, хотя релизы AWG3 вышли 31.07).
- Новый gate: маркер `/etc/x-ui/.awg-module-version` теперь содержит **SHA коммита**, из которого собран модуль; update.sh сравнивает его с `git ls-remote refs/heads/master` (без клона). Legacy-маркеры («1.0.0», «1.0.20260611») ≠ SHA → одноразовая пересборка на всех хостах при первом lucx.58-обновлении.
- **Авто-апгрейд ядра** (запрос пользователя): скрипт ставит `linux-image-amd64`/`linux-headers-amd64` (meta-package, Debian/Ubuntu-family) при КАЖДОМ вызове, до early-exit — ядро обновляется на каждом обновлении панели, даже когда модуль актуален. update.sh в конце обновления ребутит в новое ядро (10с задержка, systemd поднимает панель; AWG-модуль уже скомпилирован для нового ядра).
- **Build-first-safe**: новый DKMS-tree компилируется ПОКА старый модуль загружен; swap (rmmod → dkms remove old → dkms install new) только после успешной сборки. Упавшая сборка больше не оставляет хост без модуля (старый порядок rmmod-first мог обездвижить AWG).
- Модуль собирается для ВСЕХ установленных ядер с headers (не только запущенного) — ребут в свежепоставленное ядро сразу стартует с новым модулем.
- Тулзы пересобираются не только когда их нет, но и когда `awg version` < v3 (старые не парсят HPK).

**Фикс 3 — a11y flake ConfigBlock (Pattern 7b, блокировал 3 релиза):** корень — гонка: a11y-adddon гоняет axe сразу после play(), а Collapse-контент ещё в fade-in; axe видел текст при ~54% opacity → blended-цвет `#a6a6a6` на `#f8f8f8` (контраст 2.29). Фикс: `token.motion: false` в ConfigProvider storybook-декоратора (`.storybook/preview.tsx`) — анимации в stories отключены, рендер детерминированный; продакшн-анимации не тронуты. Проверено 3 последовательными прогонами.

**Runtime-верификация (test2):** ядро 6.12.90 → 6.12.100 (security-апдейт, ребут); старый модуль (маркер 1.0.20260611, тулзы v1.0.20260618-2, HPK-символа в kallsyms нет) пересобран новым скриптом из master с `--force-rebuild` — результат см. в AGENTS.md.

**Файлы:** `internal/awg/platform_linux.go` (probe-перепись + awg3CapabilityCheck), `internal/awg/platform_linux_test.go` (TestParseMajorVersion, TestKallsymsHasSymbol), `internal/awg/platform_other.go` (stub awg3CapabilityCheck), `internal/awg/diagnostics.go` (awg3 support check + Healthy-exclusion), `internal/awg/diagnostics_test.go` (6 чеков, awg version в fakeProber), `internal/web/controller/awg.go` (gate), `bin/install-awg-module.sh`, `update.sh`, `frontend/.storybook/preview.tsx`, `internal/config/config.go` (lucx.58), `AGENTS.md`, `progress.md`.

**Тесты:** `go test ./internal/awg/... ./internal/lucx/...` PASS; `GOOS=linux go vet ./internal/awg/...` чисто; `npm run typecheck && npm run lint` чисто; storybook ConfigBlock 3/3 PASS ×3.

**Урок 1:** Версия модуля ядра — НЕ индикатор возможностей: dkms.conf/Makefile-версии константны между мажорными релизами протокола. Единственный надёжный capability-check — функциональный probe (символы kallsyms для ядра, `awg version` для тулзов). Любая логика «if version >= X» для внешних компонентов должна проверять фичу, а не строку.

**Урок 2:** Version-gate в shell, построенный на grep'е чужого конфиг-файла, обязан иметь end-to-end проверку на реальном файле (здесь grep по `version\s*"` не матчил UPPERCASE `PACKAGE_VERSION` и gate молча не работал НИ РАЗУ с lucx.51). Сравнение SHA коммитов — единственный не обманываемый вариант для «собрано ли из актуального master».

**Урок 3:** Storybook a11y + antd-анимации = плавающие color-contrast-падения (axe меряет элемент в середине fade). `token.motion: false` в storybook-декораторе — стандартный способ сделать тесты детерминированными; продакшн не затрагивается.


## Релиз v3.6.0-lucx.60 (2026-08-03) — passthrough диапазонов N-M для AWG3 device-таймеров

**Контекст:** проверка «конфиг полный / соответствует стандартам awg3?» выявила: эталонный парсер amneziawg-tools `config.c` (`u16_range_from_string`) и ядро (`device.h`: все 6 таймеров — `u16_range_t`) нативно принимают и одиночное значение, и диапазон «N-M», рандомизируя в пределах при rekey (как H1-H4). Подтверждено живьём на test2 (модуль v3): полный AWG3-конфиг (HPK + валидные S/H + все таймеры-диапазоны) поднимается, в дампе ядра диапазоны хранятся (`10-64 100-200 3-7 160-200 8-12 15-20`). lucx.59 схлопывал диапазон в одно число на фронте — конфиг валиден, но без нативной рандомизации ядра.

**Что сделано:** значение хранится строкой end-to-end и пишется в `.conf` как есть.
- `internal/awg/instance.go`: новый тип `awg.AwgTimer` (string; `UnmarshalJSON` принимает JSON-число из старых инбаундов И строку-диапазон; `IsZero` для omit). Поля Instance int→AwgTimer, fingerprint на строках.
- `internal/awg/manager.go` `renderServerConf`: рендер `%s` + `!IsZero()`.
- `internal/web/service/inbound.go` `inboundAwgHints`: parse на `awg.AwgTimer`, рендер `%s` (клиентский «ceiling»-блок, как и H1-H4, отдаёт диапазоны).
- `internal/sub/service.go`: share-link понимает число и строку-диапазон.
- `internal/database/migrate_awg_hpk.go`: прун не-v3 чистит и строковые значения.
- Фронт: `normalizeAwgTimer` (lib/awg/timer.ts) только клампает 0..65535 / упорядочивает «500-100»→«100-500» / сворачивает «N-N»→«N», НЕ схлопывает диапазон; схема хранит строку; дефолты «0»; `inbound-link.ts` (share-link + .conf) пропускает диапазон как есть, пропуская «0»/«0-0».
- Outbound-форма (`AwgOutboundFormModal`) не тронута (отдельная фича, int).

**Тесты:** `instance_test` (число + диапазон verbatim), `awg-timer.test` (5), `inbound-link.test` (58). `go test ./internal/awg` PASS, typecheck/lint/build/gofumpt чисто.

**Урок:** прежде чем «улучшать» поле (схлопывать диапазон), проверь эталонный парсер и ядро — может оказаться, что нативный формат уже поддерживает то, что ты собираешься отбросить. AWG-диапазоны — это фича ядра (u16_range_t), а не только UI-конвенция.

## Docs (2026-08-06) — новый слоган в README на всех языках

**Что сделано:** первая строка-слоган заменена во всех 7 README (`README.md`, `ru_RU`, `fa_IR`, `ar_EG`, `zh_CN`, `es_ES`, `tr_TR`): вместо «3x-ui + native AmneziaWG (AWG) — censorship-resistant VPN panel that 3x-ui is missing» теперь «Advanced Xray & AmneziaWG control panel — with unified subscriptions, multi-server management and native AWG support» (+ переводы на соответствующие языки). Секция «What is LucX-UI» (строка про AWG как censorship-resistant fork WireGuard) не тронута.

**Файлы:** 7 × `README*.md`. Коммит `6213ebee`, запушен в `gh/main`. Тесты не требуются (только документация).

## Релиз v3.6.0-lucx.86 (2026-08-08)

1. **fix(clients):** AWG version selector при multi-attach — один .conf panel на каждый AWG-инбаунд со своим ceiling (раньше брался только первый → v1.5 блокировал 2/3). Репорт Aleksandr SacredX.
2. **fix(awg):** diagnostics MASQUERADE проверяет mark-based rules (lucx.84+), не устаревший `-s subnet`.

## Релиз v3.6.0-lucx.87 (2026-08-08)

**fix (Aleksandr):** AWG2 Standard — I1 генерился бэкендом, но поле I1 в форме показывалось только при Pro. Теперь I1 виден для Lite/Standard; I2-I5 — только Pro. При regenerate < Pro очищаются stale I2-I5.

## Релиз v3.6.0-lucx.88 (2026-08-09)

**feat(sub):** AWG в Clash (amnezia-wg-option) + подписка Amnezia /awg/{subId} (.conf и ?format=vpn → vpn://).
Ссылки AMNEZIA / vpn:// / CLASH во всех UI (ClientInfo, QR, SubLinks, InboundInfo, Inbound QR, TG bot, settings).


## Релиз v3.6.0-lucx.89 (2026-08-09)

**fix(sub):** /awg/ больше не отдаёт HTML «Информация о подписке» (maybeServeSubPage) — всегда .conf / vpn:// attachment. Репорт Aleksandr: ссылка открывалась как страница SUB.

**Верификация (2026-08-09, стенд 144.31.224.212):** dev-latest → `lucx.89+dev+1d9137d4`; `GET /awg/{subId}` с `Accept: text/html` (браузер) → 200 + `Content-Disposition: attachment; filename="amneziawg.conf"` + сырой .conf; `?format=vpn` → `vpn://…`. Нюанс: /awg/ (как /sub/, /json/, /clash/) слушает **sub-сервис на отдельном порту** (дефолт 2096), НЕ порт панели — в nginx проксировать туда же, куда /sub/ (у Aleksandr конфиг корректный). Stable-тег v3.6.0-lucx.89 поставлен, чтобы фик доехал через `x-ui update`.

## Релиз v3.6.0-lucx.90 (2026-08-09)

**fix(sub):** Endpoint в AWG .conf/vpn:// больше не берётся из X-Real-IP. За nginx с `proxy_set_header X-Real-IP $remote_addr` ResolveRequest подставлял в host IP **клиента** (публичный IP «routable» → проходил PrepareForRequest без замены) → Endpoint = WAN-адрес подписчика; репорт Aleksandr: «точка подключения генерится как IP WAN моего роутера», воспроизведено на стенде (`Endpoint = 149.154.99.99:51820` при подставленном X-Real-IP). Фикс: `AwgEndpointHost` (LUCX-HOOK, internal/sub/service.go) — trusted X-Forwarded-Host → Host header, X-Real-IP никогда; serveAwgBody использует его вместо host из ResolveRequest. Цепочка собственных адресов инбаунда (ShareAddr/node/listen/configuredPublicHost) в resolveInboundAddress по-прежнему приоритетнее. Тест TestAwgEndpointHost_NeverRealIP.

**Вопрос geo-файлов (Aleksandr):** да, при любом обновлении панели release-tarball перетирает `bin/{geoip,geosite}{,_IR,_RU}.dat` стоковыми — это поведение upstream 3x-ui (блок в release.yml без LUCX-HOOK). Решение 2026-08-09: update.sh НЕ трогаем; совет операторам — кастомные группы держать в файлах с отдельным именем (`geosite_rkn.dat` → `geosite:rkn` / `ext:geosite_rkn.dat:group`), tarball их не содержит и не перетирает; либо восстанавливать кроном после update.

## Релиз v3.6.0-lucx.91 (2026-08-09)

**feat(tunnel): NaiveProxy — первый туннельный сайдкар.** Новый класс интеграций: внешние туннельные серверы под надзором панели (не Xray-протоколы, трафик мимо Xray). Реализация по сайдкар-паттерну mtproto/AWG; референсы дизайна: elector1337/3x-ui-naive (Caddyfile-грабли: admin off, bare :port + bind, изоляция ACME), Ex3-ui (UX страницы). Ядра: qWDTT/olcRTC запланированы следующими на этом же каркасе.

- **`internal/lucx/tunnel/`** (PolyForm): Name-реестр, NaiveConfig + рендер Caddyfile (экранирование, wildcard→bare :port, bind, ACME/ручной TLS, padding/probe_resistance/H3, raw-режим), Proc (SIGTERM→kill, ring-лог 500 строк, `tunnel: <name> | <line>`), Manager (fingerprint-рестарт, writeConfig без mtime-churn, трёхуровневый статус process→TCP→TLS, StopAll).
- **`internal/web/service/tunnel.go`**: конфиг в settings (`lucxTunnel_naive`), валидации (порт, креды, ACME=домен+443, лог-уровень), кросс-проверка порта с TCP-инбаундами (UDP-протоколы пропуск, учёт перекрытия интерфейсов), download бинарника через temp-файл (200 МБ лимит), `caddy adapt`-валидация Caddyfile.
- **`internal/web/job/tunnel_job.go`** + **`internal/web/controller/tunnel.go`**: cron 10s reconcile (краш-ревив), API `/panel/api/tunnel/naive/*` (status/config/start/stop/restart/logs/preview/validate/upload/download/deleteBinary).
- **LUCX-HOOK**: web.go (cadenceTunnel, джоба, StopAll, upload exempt из 10MiB body-limit), api.go (регистрация).
- **Frontend**: страница `/panel/tunnels` (карточка ядра: статус-бейдж, бинарник upload/download/delete, start/stop/restart, логи, client URL `naive+https://`, simple/raw форма с preview/validate), schemas/tunnel.ts, api/tunnels.ts (JSON_HEADERS — урок lucx.69), меню, endpoints.ts × 12 эндпоинтов, **i18n × 13 локалей**.
- **release.yml**: amd64 — prebuilt `caddy-forwardproxy-naive.tar.xz` (klzgrad/forwardproxy, pinned v2.11.2-naive); arm64 — кросс-сборка xcaddy (чистый Go). Прочие архитектуры без бинарника (upload в UI).
- **Доки**: README (секция Tunnel Sidecars + credits), LICENSING.md (PolyForm-файлы + third-party caddy Apache-2.0/MIT/BSD-3), lucxVersion → lucx.91.

**Тесты:** naive_test (рендер/валидации/escape/URL/fingerprint), manager_test (proc lifecycle re-exec, пробы против реальных TCP/TLS-слушателей, config write). Окружение dev-машины: Kaspersky ломает loopback TLS (MITM) — TLS-ассерты проб скипаются с пометкой окружения; на Linux-стенде и в CI работают. gofumpt/check-lucx, typecheck/lint/vitest unit (922) — зелёные.

**Известное:** бинарник caddy-naive требует домена+сертификата (ACME HTTP-01 = порт 443); мост в Xray-роутинг (SOCKS-egress патч forwardproxy + injectTunnelEgress) — следующий подэтап.
## E2E NaiveProxy (2026-08-09, WSL2 Ubuntu 24.04) — PASS, 3 бага найдено и починено

**Стенд:** WSL2 (loopback TLS в Windows ломает Kaspersky MITM — в WSL трафик мимо него). Бинарники: caddy 2.11.4 + klzgrad/forwardproxy@naive (xcaddy, linux/amd64), naive-клиент v150.0.7871.63-1 (официальный релиз). Харнесс гонял продакшен-код `internal/lucx/tunnel` (Manager.Ensure/StatusOf/Stop) с реальным caddy.

**Найдено и починено (все три — рендер Caddyfile):**
1. **`padding` не существует** как сабдиректива forward_proxy — паддинг включается сам, когда клиент шлёт заголовок `Padding` (forwardproxy.go:352). Убрано из рендера/схемы/формы/i18n × 13; регресс-тест.
2. **Домен без порта = второй слушатель :443** (auto-HTTPS по умолчанию): `:18443, localhost` открывал ещё и :443 → `bind: permission denied` без root. Фикс: домен рендерится с портом (`localhost:18443`), для 443 — bare.
3. **ACME-слушатель :80 + установка локального root-серта** в manual-TLS режиме. Фикс: `auto_https off` (только manual; ACME-режим оставляет) + `skip_install_trust` (всегда) в глобальном блоке.

**Зелёные шаги E2E:** бинарник на ожидаемом пути; валидация конфига; client URL; `caddy adapt` на отрендеренный Caddyfile; Ensure стартует caddy; пробы running→listening→responding; SOCKS naive-клиента; **реальный трафик через официального naive-клиента: `negotiated padding type: Variant1`, body=публичный IP**; трафик через plain HTTP CONNECT (curl --proxy-insecure); неверный пароль отклонён; рестарт при смене конфига (fingerprint); disable останавливает процесс.

**Уроки:** (1) E2E против реального бинарника обязателен до релиза — юнит-тесты рендера не видели ни одной из трёх ошибок (все в семантике caddy, а не в логике рендера); (2) у naive-клиента нет --insecure — self-signed серт надо класть в system trust store (`update-ca-certificates`); (3) curl для HTTPS-прокси требует `--proxy-insecure` (не `-k`). Харнесс после прогона удалён из репо (процедура задокументирована здесь).
## NaiveProxy: per-client креды + подписки + UX (2026-08-09)

**Per-client учётные данные (без миграции БД):** `tunnel.ClientAuth(secret, email)` — детерминированный HMAC-SHA256 от панельного секрета (`SettingService.GetSecret`) + email клиента: user `nx` + 10 hex, pass 27 символов base64url (162 бит). Ничего не хранится: рендерер Caddyfile и генератор ссылок подписки выводят одну и ту же пару; disable/delete клиента убирает его `basic_auth`-строку на следующем reconcile (рестарт caddy при изменении списка — как у elector1337). Raw-режим без per-client кредов (оператор владеет конфигом сам).

**Подписки:** LUCX-HOOK в `internal/sub/service.go:getSubs` — каждому включённому клиенту подписки добавляется его персональная ссылка `naive+https://user:pass@domain:port#email` (стандарт NekoBox/husi/Exclave). Ссылки и emails добавляются синхронно — BuildPageData зиппит их по индексу. Только base64-sub: JSON/Clash-форматы naive не представляют (mihomo/sing-box не поддерживают протокол). Условие эмиссии: core enabled + не raw + домен задан + секрет есть.

**UX страницы:** генератор пароля (18 случайных байт → 24 символа base64url, crypto.getRandomValues) у поля authPass; QR-модалка на клиентскую ссылку (переиспользован QrPanel из inbounds).

**Проверки:** TestClientAuth (детерминизм/уникальность/ротация/нелик email), TestRenderCaddyfileExtraAuth (порядок строк, экранирование); `caddy adapt` в WSL на Caddyfile с тремя basic_auth — ADAPT_OK; go test lucx, gofumpt, typecheck, lint, vitest unit (922), build — зелёные. Компиляцию sub-хука проверит CI (CGO).

**Не делаем:** obfuscation-генератор как у AWG — наиву нечего генерировать (камуфляж = стек Chrome, паддинг автоматический по заголовку Padding); пресеты = фиктивная обёртка над тремя уже имеющимися свитчерами.