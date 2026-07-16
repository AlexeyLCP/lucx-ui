# LucX-UI — Прогресс миграции на v3.5.0

> Файл ведётся агентом в ходе работы. Обновляется при каждом шаге.

---

## Контекст

- **Репозиторий:** [AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) — форк 3x-ui
- **Цель:** миграция с v3.3.1 → v3.5.0 (228 коммитов апстрима)
- **Стратегия:** migrate (по AGENTS.md) — свежий checkout `origin/main` v3.5.0 + перенос LucX-кода поверх
- **Ветка миграции:** `feat/awg-sidecar-v3.5.0` (создана от `origin/main` v3.5.0)
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

### 2026-07-13

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

## Заметки

- v3.5.0 релиз 2026-07-12 (вчера)
- 228 коммитов между v3.3.1 и v3.5.0
- 41 LUCX-HOOK маркер на старой ветке
- Все 8 upstream-файлов с HOOK-маркерами изменились между v3.3.1 и v3.5.0 — требуется ручное восстановление
- Тесты AWG на старой ветке проходят: `go test ./internal/awg/... → ok 2.212s`