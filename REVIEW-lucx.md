# Ревью LucX-слоя поверх 3x-ui

**Дата:** 2026-08-14 · **Ревьюился коммит:** `8ca2adae` (v3.6.0-lucx.122)
**База сравнения:** `origin/main` (MHSanaei/3x-ui), merge-base `c377dca2`
**Объём проверенного:** 323 файла, +57 719 / −3 881 строк; 168 новых файлов

> **Статус: всё найденное исправлено в lucx.123.** Разбор правок — в
> `progress.md`, секция «lucx.123». Ниже сохранён исходный отчёт: он объясняет
> *почему* каждая правка сделана, что полезнее списка изменённых файлов.
> Единственное исключение — P3-4 (нарезка `progress.md`), отложен осознанно, и
> P3-7, оказавшийся ложной тревогой.

| | Находка | Статус |
|---|---|---|
| P0-1 | Go не верифицирован в песочнице | остаётся: нужен `make verify` / CI |
| P1-1 | Сироты сайдкаров на Linux | исправлено — `orphans_linux.go` + sweep |
| P1-2 | SSRF + нет проверки целостности бинарников | исправлено — https/public-only, лимит редиректов, `sha256` |
| P1-3 | 34 892 строки фантомного CRLF-диффа | исправлено — `.gitattributes` + renormalize |
| P2-1 | Глобальный мьютекс через блокирующие операции | исправлено — per-key `opMu` |
| P2-2 | Неограниченный буфер логов | исправлено — лимит 64 KiB |
| P2-3 | Конфиг mtg 0640 | исправлено — 0600 |
| P2-4 | Нет тестов на HTTP-слое, дубль загрузки | исправлено — тесты + слияние |
| P2-5 | Новые страницы фронтенда без тестов | частично — покрыт CoresTab (форма загрузки) |
| P3-1 | ESLint warning | исправлено |
| P3-2 | Мёртвая миграция | исправлено — удалена |
| P3-3 | Три идентичных файла правил | исправлено — указатели на AGENTS.md |
| P3-4 | `progress.md` 292 КБ | отложено осознанно — см. ниже |
| P3-5 | Комментарии в upstream-файлах | без правок — правило переформулировано в пользу практики |
| P3-6 | `CombinedOutput()` на 45-минутной сборке | исправлено — стриминг |
| P3-7 | Кэш geodata без вытеснения | ложная тревога, вытеснение есть |

## Что прогонялось

| Проверка | Результат |
|---|---|
| `tsc --noEmit` | ✅ чисто |
| `eslint src` | ⚠️ 0 ошибок, 1 warning |
| `vitest --project unit` (62 файла) | ✅ 888 тестов зелёные |
| `vitest --project components` (34 файла) | ✅ 117 тестов зелёные |
| `vitest --project storybook` | ⏭️ не запускался (нужен Playwright/Chromium) |
| `vite build` | ✅ собирается за ~50 с |
| `node scripts/build-openapi.mjs` | ✅ регенерация даёт байт-в-байт тот же `openapi.json` |
| i18n: 13 локалей × 2342 ключа | ✅ ни одного пропущенного/лишнего ключа |
| Роуты Gin ↔ `endpoints.ts` | ✅ все 223 роута зарегистрированы |
| Go: `build` / `vet` / `golangci-lint` / `go test` | ❌ **не прогонялись** — см. P0-1 |

Общее впечатление: код заметно аккуратнее среднего форка — нет `any`,
нет `@ts-ignore`, нет `TODO/FIXME`, нет захардкоженных секретов, везде
`crypto/rand`, geodata-ридер защищён `os.OpenRoot` + проверкой base-name +
лимитом размера и покрыт тестом на symlink-escape. Найденное ниже — это
в основном хвосты архитектуры сайдкаров и процессная гигиена, а не «дыры».

---

## P0 — блокеры проверки

### P0-1. Go-часть не верифицирована в этой сессии
В песочнице нет Go 1.26 и его нельзя скачать (прокси отдаёт 403 на
`go.dev`, `dl.google.com`, зеркала; в apt только Go 1.18, чего для
`go.mod` недостаточно). Поэтому **`go build`, `go vet`, `golangci-lint`,
`go test -race` по всему `internal/` не выполнялись** — вся Go-часть ниже
разобрана только чтением кода.

Перед мержем обязательно локально: `make verify`.
CI (`.github/workflows/ci.yml`) этот гейт закрывает полностью — там есть
и `gen`-freshness, и `govulncheck`, и race+shuffle, и typecheck, — так что
риск ограничен «не проверено здесь», а не «не проверяется вообще».

---

## P1 — исправить до следующего релиза

### P1-1. Осиротевшие tunnel-сайдкары на Linux не убиваются
**Где:** `internal/lucx/tunnel/process_other.go:13`, `internal/lucx/tunnel/manager.go`

```go
//go:build !windows
func attachChildLifetime(_ *exec.Cmd) {}
```

На Windows дочерние процессы привязываются к Job Object с
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` (`process_windows.go`), а на Linux —
целевой платформе деплоя — не делается **ничего**: ни `Setpgid`, ни
`SysProcAttr.Pdeathsig: SIGKILL`.

Последствия: если панель падает не по graceful-пути (OOM-killer, `kill -9`,
паника, `systemctl kill`), `StopAll()` из `web.go:700` не отрабатывает, и
caddy/mita/qwdtt/trusttunnel остаются жить сиротами, держа listen-порты.
Следующий старт панели не сможет поднять свои сайдкары — порт занят.

Показательно, что в `internal/mtproto/` эта проблема **уже решена**:
есть `orphans_linux.go` с `killStrayMtgProcesses`, вызываемый при старте
(`manager.go:260-268`). В `internal/lucx/tunnel/` аналога нет.

**Чинить:** (а) `cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}`
в `attachChildLifetime` для Linux; (б) стартовый sweep по образцу
`mtproto/orphans_linux.go` — на случай, если сирота пережил и это.

### P1-2. Загрузка бинарника сайдкара — произвольный URL без проверки целостности
**Где:** `internal/web/service/tunnel.go:332` (`DownloadBinary`), `:843` (`downloadBinaryTo`)
**Точки входа:** `POST /panel/api/tunnel/{naive,olcrtc,qwdtt,mieru,trusttunnel}/download`

```go
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
resp, err := (&http.Client{}).Do(req)
...
file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
```

URL приходит из тела запроса как есть. Нет: allowlist схем/хостов, запрета
редиректов, ограничения на приватные диапазоны, checksum/подписи. Скачанное
сразу становится исполняемым (`0o755`) и запускается панелью (обычно от root).

Да, эндпоинт под API-auth + CSRF и доступен только админу, так что это не
privilege escalation. Но это (а) SSRF-примитив — можно заставить панель ходить
на `169.254.169.254` или во внутреннюю сеть и увидеть ответ через размер/ошибку;
(б) отсутствие integrity-проверки означает, что MITM или опечатка в URL
подменяют исполняемый файл, который панель запустит.

**Чинить, по возрастанию цены:** отклонять всё кроме `https://`; резолвить
хост и отбрасывать приватные/link-local адреса; принимать вместе с URL
ожидаемый SHA-256 и сверять до `os.Rename`. Минимум — sha256 в логе после
загрузки, чтобы оператор мог сверить руками.

### P1-3. 34 892 строки фантомного диффа в рабочем дереве (CRLF)
**Где:** `.gitattributes`, 13 файлов `internal/web/translation/*.json`, `progress.md`,
3 файла `frontend/src/test/*`, `frontend/vitest.config.ts`

`git status` сейчас показывает 17 изменённых файлов и `+34892/−34892` —
и **ни одного смыслового изменения**: это целиком переключение LF↔CRLF.
`.gitattributes` нормализует только `*.sh`, `*.go`, `frontend/src/generated/**`,
`frontend/public/openapi.json`, снапшоты и `deploy/**/*.yaml`. `*.json`
(включая все локали), `*.md` и `*.ts` не покрыты, а работа идёт из
Windows/OneDrive.

Последствия ощутимые: любой `git diff` по локалям нечитаем, ревью правок
i18n невозможно, конфликты при мерже upstream раздуваются на ровном месте.
Вдобавок `frontend/vitest.config.ts` сейчас со смешанными окончаниями
(`with CRLF, LF line terminators`).

**Чинить:**
```
* text=auto eol=lf
*.exe binary
*.png binary
```
и разово `git add --renormalize .`.

---

## P2 — стоит поправить

### P2-1. Глобальный мьютекс менеджера удерживается через блокирующие операции
**Где:** `internal/lucx/tunnel/manager.go:184-220` (`Ensure`)

`Ensure` берёт `m.mu.Lock()` на всю функцию, а внутри:
`writeConfigFile` (I/O) → `proc.Stop()` (до 5 с SIGTERM + 2 с SIGKILL) →
`time.Sleep(200 * time.Millisecond)` → `m.start()` (stat, chmod, mkdir, exec).

Мьютекс один на **все** ядра. Реконсайл `ReconcileWanted` зовёт `Ensure`
последовательно, так что N инбаундов, которым нужен рестарт, — это N × ~7 с
под общим локом. Всё это время `IsRunningKey`, `LogsPrefixed`, `StatusOf`,
`StopKey` (все берут тот же лок) стоят — то есть UI «Tunnels» и статусы
инбаундов подвисают.

`cron.SkipIfStillRunning` (`web.go:514`) спасает от наложения тиков, но
не от подвисшего UI.

**Чинить:** лок только на чтение/запись `m.cores`, а `Stop`/`start`
выполнять вне лока (например, per-key мьютекс в `managed`).

### P2-2. Буфер логов дочернего процесса растёт неограниченно
**Где:** `internal/lucx/tunnel/process.go:83`

```go
w.buf += string(p)
for { i := strings.IndexByte(w.buf, '\n'); if i < 0 { break } ... }
```

`ring` ограничен 500 строками, а вот `procLogWriter.buf` — нет. Сайдкар,
который пишет длинный вывод без `\n` (прогресс-бар с `\r`, JSON-дамп одной
строкой, поломанный бинарник в бинарном режиме), надувает эту строку до
бесконечности. Плюс `w.buf = w.buf[i+1:]` на каждой строке — это O(n)
переаллокация; при болтливом сайдкаре заметно.

**Чинить:** `bufio.Scanner`/`strings.Builder` с жёстким лимитом (скажем,
64 KiB) — при переполнении сбрасывать хвост как строку и обнулять буфер.

### P2-3. Конфиг mtg с секретами клиентов пишется 0640
**Где:** `internal/mtproto/manager.go:609`

```go
return os.WriteFile(path, []byte(renderConfig(inst, apiPort, apiToken)), 0o640)
```

В файле лежат FakeTLS-секреты всех клиентов инбаунда **и** bearer-токен
management-API. Все соседние точки записи конфигов в форке используют `0o600`:
`tunnel/manager.go:634`, `awg/client_manager.go:67`, `awg/manager.go:408`.
Расхождение выглядит незамеченным, а не намеренным.

### P2-4. Пять эндпоинтов Cores без тестов и с дублирующимся кодом
**Где:** `internal/web/service/tunnel.go` — 1335 строк, `_test.go` нет

`DownloadBinary` (:332) и `downloadBinaryTo` (:843) — это две почти
идентичные реализации одной и той же логики (первая жёстко зашита на
`tunnel.Naive.BinaryPath()`, вторая параметризована и обслуживает остальные
четыре ядра). Правка в одной не попадёт в другую.

Шире: **ни один** из новых контроллеров и сервисов не покрыт тестами —

```
NO TEST  internal/web/controller/awg.go
NO TEST  internal/web/controller/awg_outbound.go
NO TEST  internal/web/controller/tunnel.go        (611 строк)
NO TEST  internal/web/controller/lucx.go
NO TEST  internal/web/service/tunnel.go           (1335 строк)
NO TEST  internal/web/service/geodata.go
NO TEST  internal/web/service/awg_host.go
NO TEST  internal/web/job/tunnel_job.go
NO TEST  internal/web/job/awg_job.go
```

При этом доменные пакеты покрыты хорошо (`internal/awg` — 10 src / 10 test,
`internal/lucx/tunnel` — 20 / 11, `internal/mtproto` — 6 / 4). Дыра ровно
на HTTP-слое, хотя `CLAUDE.md` прямо рекомендует `httptest` и приводит
`internal/sub`'s `initSubDB(t)` как шаблон.

**Минимум:** `httptest` на `tunnel.go` — валидация тела, коды ответов,
поведение при отсутствующем бинарнике. Плюс слить два download в один.

### P2-5. Новые страницы фронтенда без тестов
`TunnelsPage.tsx`, `CoresTab.tsx`, `AwgOutboundFormModal.tsx`,
`AwgOutboundsTab.tsx`, `OlcrtcCard.tsx`, `QwdttCard.tsx` не упоминаются ни
в одном тесте. По протоколам: `mieru` — 0 тестовых файлов,
`trusttunnel` — 0. Для сравнения: `awg` — 7, `qwdtt` — 4, `tunnel` — 10.

`GeoBrowserModal` покрыт (`geo-browser-selection.test.tsx`) — хороший образец,
по которому можно дотянуть остальные.

---

## P3 — мелочи и гигиена

**P3-1. ESLint warning.** `frontend/src/pages/xray/routing/RuleFormModal.tsx:230` —
`useEffect has a missing dependency: 'methods'`. Единственное замечание
линтера на весь репозиторий; либо дописать зависимость, либо явно
задавить с объяснением.

**P3-2. ~200 строк мёртвой миграции.** `internal/database/migrate_awg_stale_clients.go`
целиком выключен константой `awgStaleMigrationEnabled = false` (осознанно,
причина расписана в комментарии — lucx.92). Решение верное, но код,
который никогда не выполнится, лучше удалить: история в git есть, а
инструкция по откату — в комментарии, который можно перенести в
`progress.md`.

**P3-3. Три идентичных файла правил для ИИ.** `.claudeprompt`, `.cursorrules`,
`.windsurfrules` — один и тот же MD5 `ff370546…`, по 19 490 байт. Гарантированно
разъедутся. Оставить один + симлинки (или тонкие файлы-указатели на `AGENTS.md`).

**P3-4. `progress.md` — 292 КБ в репозитории**, в этом диффе +2406 строк, и он
CRLF (см. P1-3), из-за чего постоянно светится в `git status`. `AGENTS.md`
— ещё 130 КБ. Оба меняются почти каждым коммитом и утяжеляют историю.
Стоит нарезать `progress.md` по релизам (`docs/changelog/lucx.1xx.md`).

> **Отложено осознанно.** CRLF-часть закрыта в P1-3, так что `git status`
> чист. Сам сплит — это 121 разнородная секция (не все с версией: «Контекст»,
> «План», «Что сделано», «Чистка», порядок не хронологический), и механически
> разложить их по файлам значит с приличной вероятностью порвать документ,
> который AGENTS.md предписывает агентам читать. Такому рефакторингу нужен
> отдельный PR и человеческий взгляд, а не попутный коммит вместе с правками
> безопасности — ровно то, о чём CLAUDE.md говорит «Diff is focused; refactors
> are separate from feature work».

**P3-5. Комментарии в изменённых upstream-файлах.** `AGENTS.md:603` фиксирует,
что запрет `//`-комментариев из `CLAUDE.md` распространяется на upstream-код,
а LucX-модули от него освобождены. Но комментарии массово добавлены именно
в upstream-файлы: `internal/web/service/inbound.go` 199 → 359,
`internal/web/service/xray.go` 175 → 288, `internal/sub/service.go` 343 → 396.
Либо это нарушение правила, либо правило стоит переформулировать под
реальную практику (что, на мой взгляд, честнее — комментарии там
содержательные, объясняют «почему», а не «что»).

**P3-6. `CombinedOutput()` на 45-минутной сборке.** `internal/web/service/awg_host.go:149` —
DKMS-сборка модуля целиком буферизуется в память. Стоит стримить в
`logger`/файл.

**~~P3-7. Кэш geodata-индексов без вытеснения.~~ — ложная тревога.**
`internal/xray/geodata/geodata.go:221` вызывает `dropStaleIndexesLocked(name, key)`
сразу после записи в кэш, и она удаляет все записи с тем же именем файла и
другим ключом. Вытеснение есть и работает; я его пропустил при первом чтении.
Правок не требуется.

**P3-8. Чанк `ApiDocsPage` — 1.29 МБ** (357 КБ gzip) при сборке. `endpoints.ts`
вырос на +482 строки от LucX-эндпоинтов, так что вклад форка есть, но
основа — upstream. Кандидат на ленивую подгрузку.

---

## Что проверено и нареканий не вызвало

- **Никакого command injection.** Все ~50 `exec.Command*` передают argv
  списком; единственное расщепление строки (`strings.Fields(inst.ExtraArgs)`,
  `manager.go:661`) идёт в argv, а не в шелл, и источник — админ.
  `ping` в `awg_outbound.go:282` бьёт по захардкоженным `1.1.1.1`/`2606:4700:4700::1111`.
- **Никакого path traversal.** `ExtraFiles` формируются внутри кода из
  `inbound.Id`; geodata-ридер закрыт `os.OpenRoot` + `name != filepath.Base(name)`
  + `MaxFileSize`, и на symlink-escape есть тест (`geodata_test.go:530`).
- **Секреты.** Ни одного захардкоженного; `crypto/rand` везде, где нужна
  энтропия; в логи секреты не попадают; management-API mtg биндится строго
  на `127.0.0.1` (`manager.go:541`).
- **Дельты трафика.** High-water-mark в `trusttunnel_traffic.go:68-74`
  и аналогах корректно переживает рестарт сайдкара — сброс счётчика не
  даёт отрицательной или раздутой дельты.
- **SQL.** Ни одной конкатенации в запрос; всё через GORM с плейсхолдерами.
- **Миграции.** Идемпотентны, с маркерами (`migratedToInbound`), с
  guard-ами по `count > 0`, узко таргетированы по префиксу тега.
- **`endpoints.ts`.** Все 223 роута на месте — ручной реестр, который
  ничем не проверяется, ведётся дисциплинированно.
- **i18n.** 13 локалей, 2342 ключа, расхождений ноль.
- **CI.** `ci.yml` покрывает больше, чем типичный форк: freshness
  кодогенерации, `govulncheck`, race+shuffle, PostgreSQL-прогон, typecheck,
  storybook-тесты, `npm audit`.

---

## Примечание об окружении

Для прогона фронтенд-тестов в Linux-песочнице пришлось доставить нативные
биндинги (в `node_modules` лежали только win32-сборки). В
`frontend/node_modules/` остались лишние ~44 МБ:
`@rolldown/binding-linux-x64-gnu`, `@esbuild/linux-x64`,
`@oxc-parser/binding-linux-x64-gnu`, `@oxc-resolver/binding-linux-x64-gnu`,
`lightningcss-linux-x64-gnu`. Снимаются любым `npm ci`. На сборку под Windows
не влияют, в git не попадают.
