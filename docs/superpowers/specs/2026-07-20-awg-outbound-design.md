# AWG Outbound (клиентский режим) — Design Spec

**Date:** 2026-07-20
**Status:** Approved → to writing-plans
**Author:** brainstorming session (user + agent)

## Motivation

LucX-UI умеет принимать AWG-клиентов как **inbound** (серверный режим: `ListenPort` + peers, маршрутизация через Xray). Этого недостаточно — оператору часто нужно **подключиться к upstream AWG-серверу** и роутить через него трафик от локальных инбаундов. Например:

- upstream VPN-провайдер на AmneziaWG, хочется пустить трафик части клиентов через него
- цепочка прокси `inbound → AWG outbound → upstream` для гео-смены или DPI-evasion
- балансировка между несколькими upstream-серверами

Сейчас такой сценарий требует ручной настройки `awg-quick` + Xray freedom outbound — панели нет. Спек добавляет нативный AWG outbound как kernel-interface sidecar, симметричный по духу с существующим AWG inbound.

## Concept

AWG outbound = **клиентское** подключение к upstream AmneziaWG-серверу. Панель:

1. Создаёт kernel-интерфейс `awgo-N` через `awg-quick up /etc/amnezia/amneziawg/awgo-N.conf`
2. Добавляет в Xray-конфиг `freedom` outbound с tag'ом, `sockopt.interface=awgo-N`
3. Routing rules / AWG-inbound `outboundTag` ссылаются на этот tag как на обычный Xray outbound

**Симметрия с inbound:**

| | Inbound (существует) | Outbound (добавляем) |
|---|---|---|
| Роль | сервер: принимает AWG-клиентов | клиент: подключается к upstream |
| Kernel-интерфейс | `awgN` (ListenPort + peers) | `awgo-N` (Endpoint + peer) |
| Xray integration | TUN inbound + routing rules | freedom outbound + sockopt.interface |
| Маршрутизация | kernel→TUN→Xray routing | Xray routing→freedom→kernel→upstream |
| Config fields | ListenPort, peers[] | Endpoint, single peer (upstream) |

## Interface naming

- **Server (inbound):** `awg1`, `awg2`, ... (как сейчас)
- **Client (outbound):** `awgo-1`, `awgo-2`, ... — короткий префикс, программно отличим от inbound, не путается с `awgN` при парсинге `awg show`

**Tag vs Interface name — разделение (user feedback 2026-07-20):**

- **`Tag` (в Xray routing rules) — редактируемый**, по умолчанию `awgo-N`. Оператор может переименовать (например, `vpn-frankfurt`) для читаемости routing rules.
- **Interface name — всегда `awgo-{id}`** (id = `AwgOutbound.Id`, БД-стабильный). Никогда не меняется при переименовании Tag — иначе пришлось бы переименовывать kernel-интерфейс и .conf, лишняя сложность.
- Соответствие Tag ↔ interface: хранится в БД (`AwgOutbound.Tag` + `AwgOutbound.Id → awgo-{Id}`), не выводится из строки.
- `sockopt.interface` всегда = `awgo-{Id}`, не Tag. `tag` в Xray config = редактируемый `AwgOutbound.Tag`.
- При удалении и пересоздании с тем же Tag — новый Id → новый интерфейс (Tag не уникален в Xray; уникальность Tag проверяется на add/update, как у обычных Xray outbounds).

## Client .conf (renderClientConf)

Клиентский AWG-конфиг имеет критические отличия от серверного:

```ini
[Interface]
PrivateKey = <our client Curve25519 private key>
Address = 10.9.0.5/32        # ОБЯЗАТЕЛЬНО: tunnel IP, выданный upstream
MTU = 1320
Table = off                   # КРИТИЧНО: не трогаем системную маршрутизацию
                              # Xray freedom + sockopt.interface управляет egress
Jc = 3                        # obfuscation (Jc/S/H всегда, I1-I5 если заданы)
Jmin = 50
Jmax = 150
S1 = 20
S2 = 30
S3 = 40
S4 = 50
H1 = 100000-500000
H2 = ...
H3 = ...
H4 = ...

[Peer]
PublicKey = <upstream server public key>
PresharedKey = <optional PSK>
Endpoint = upstream.example.com:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
```

**Почему `Table = off`:** awg-quick по умолчанию (`Table = auto`) ставит default route через туннель — это перекроет системный маршрут и сломает всю остальную маршрутизацию панели. Xray через `sockopt.interface` сам заворачивает нужные потоки в интерфейс; ядру не нужно знать маршрут.

**Почему `Address` обязателен:** без него kernel module не инициализируется как клиент. upstream выдаёт tunnel IP (обычно из подсети сервера, например `10.9.0.0/24`).

**DNS в клиентском .conf (user feedback 2026-07-20):**

При `Table = off` строка `DNS =` обычно не нужна — Xray сам резолвит через `domainStrategy: UseIP` и системный DNS. Поэтому:

- `DNS` **не пишется** в `renderClientConf` по умолчанию
- `dns` поле в Settings — **опциональное**, живёт в секции Advanced формы
- Если оператор явно задал `dns` (non-empty) — пишем в .conf, но предупреждаем в tooltip что при `Table = off` это обычно избыточно
- Симметрично с серверным .conf, где DNS тоже не пишется (комментарий в `renderServerConf` уже фиксирует это)

## Xray integration

При `enable=true` AWG outbound инжектится в generated Xray config:

```json
{
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "vpn-frankfurt",
      "settings": {
        "domainStrategy": "UseIP",
        "sendThrough": "10.9.0.5"
      },
      "streamSettings": {
        "sockopt": {
          "interface": "awgo-1"
        }
      }
    }
  ]
}
```

- `tag` = редактируемый `AwgOutbound.Tag` (по умолчанию `awgo-N`, оператор может задать `vpn-frankfurt` для читаемости routing rules) — используется в routing rules и `outboundTag` инбаундов
- `sockopt.interface` = `awgo-{Id}` (БД-стабильный, не Tag) → Xray отправляет пакеты через него
- `sendThrough` = tunnel IP **без CIDR-маски** (обрезаем `/NN` перед инжектом — см. ниже), опционально, для дополнительной страховки source-binding

### sendThrough: strip CIDR (user feedback 2026-07-20)

`settings.address` хранится как `10.9.0.5/32` (или `fd00::5/128` для IPv6), но `sendThrough` в Xray принимает **только IP**, без маски. Обрезаем явно:

```go
sendThrough := strings.SplitN(ci.Address, "/", 2)[0]
```

Иначе Xray может не принять значение с маской (молчаливый fallback или ошибка валидации). Применяется в `injectAwgOutbounds` при формировании outbound JSON.

### Tag uniqueness — против всех Xray outbounds (user feedback 2026-07-20)

Tag проверяется на уникальность не только в таблице `awg_outbounds`, а **против всех outbound-тегов, которые попадут в финальный Xray-конфиг**:

- обычные Xray outbounds (из `XrayConfig.Outbounds`)
- уже существующие AWG outbounds (из `awg_outbounds`)
- системные (`direct`, `block`, `api`, и т.д. — из `basics` / настроек Xray)

Иначе при коллизии Xray упадёт с `duplicate outbound tag`. Проверка делается в `AwgOutboundService.AddOutbound` / `UpdateOutbound`:

```go
// collect all tags that will be in the final Xray config
existingTags := collectAllOutboundTags(xrayConfig, existingAwgOutbounds, systemTags)
if slices.Contains(existingTags, newTag) {
    return ErrDuplicateTag
}
```

Список системных тегов берётся из существующего хелпера для Xray outbounds (если есть) или захардкожен (`direct`, `block`, `api`).

### IPv6 / dual-stack (user feedback 2026-07-20)

`address` может быть IPv6 (`fd00::5/128`), `sendThrough` тоже поддерживает IPv6 (strip CIDR работает одинаково), `allowedIPs` по умолчанию `0.0.0.0/0, ::/0` — dual-stack поддерживается. `awg-quick` и kernel module работают с IPv6 нативно. Никакой специальной логики — просто не хардкодить IPv4-предположения.

### Injection order + disable

**Injection order:** `injectAwgOutbounds` вызывается **после** обычных Xray outbounds, но **до** balancers и routing rules — чтобы tag был доступен для ссылок.

**При `enable=false` (или удалении):** outbound **полностью убирается** из Xray-конфига (не blackhole). См. "Interface-down behavior" в Risks — обоснование.

**`needRestart`:** `SetOutboundEnable`, `AddOutbound`, `DelOutbound`, `UpdateOutbound` вызывают `needRestart` (как `awgRoutesThroughXray` в `inbound.go`), чтобы Xray перегенерировал конфиг.

## Data model

Новая таблица `awg_outbounds` (GORM, зеркало по стилю `inbounds`):

```go
type AwgOutbound struct {
    Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
    Tag        string `json:"tag" form:"tag" gorm:"uniqueIndex;not null"`
    Remark     string `json:"remark" form:"remark"`
    Enable     bool   `json:"enable" form:"enable" gorm:"default:true"`
    Settings   string `json:"settings" form:"settings"`
    CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt  int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

`Settings` JSON schema:

```ts
{
  privateKey: string,        // наш client private key (генерим, если пусто)
  address: string,           // ОБЯЗАТЕЛЬНО: tunnel IP, e.g. "10.9.0.5/32"
  mtu: number,               // default 1320
  publicKey: string,         // upstream server public key (обязательно)
  psk: string,               // опционально
  endpoint: string,          // "host:port" (обязательно)
  keepalive: number,         // default 25
  allowedIPs: string,        // default "0.0.0.0/0, ::/0"
  // AWG obfuscation (опционально, если upstream требует):
  jc: number,
  jmin: number, jmax: number,
  s1: number, s2: number, s3: number, s4: number,
  h1: string, h2: string, h3: string, h4: string,
  i1: string, i2: string, i3: string, i4: string, i5: string,
}
```

Migration: `internal/database/migrate_awg_outbound.go` — `AutoMigrate(&AwgOutbound{})` при инициализации БД.

## ClientInstance (новый тип в internal/awg/)

```go
type ClientInstance struct {
    Id          int
    Ifname      string  // "awgo-1"
    PrivateKey  string
    Address     string
    MTU         int
    PublicKey   string  // upstream server pubkey
    PSK         string
    Endpoint    string
    Keepalive   int
    AllowedIPs string
    // Obfuscation
    Jc, Jmin, Jmax, S1, S2, S3, S4 int
    H1, H2, H3, H4 string
    I1, I2, I3, I4, I5 string
}

// ClientInstanceFromOutbound(o *AwgOutbound) (ClientInstance, bool)
// (ci ClientInstance) fingerprint() string
// renderClientConf(ci ClientInstance) string
```

### .conf file permissions (user feedback 2026-07-20)

`awg-quick` ругается (и в некоторых версиях отказывается читать), если файл world-readable — `.conf` содержит приватный ключ. При записи:

```go
err := os.WriteFile(confPath, []byte(renderClientConf(ci)), 0600)
```

`0600` (owner read/write only) — стандарт для WireGuard/AWG конфигов. Симметрично с тем, как должен вести себя серверный `writeServerConfigFile` (проверить и при необходимости поправить существующий код — отдельный mini-task). Конфиги лежат в `/etc/amnezia/amneziawg/` (владелец root, режим 0700 на директорию уже от `install-awg-module.sh`).

## Manager extensions (internal/awg/manager.go)

Расширяем singleton Manager:

```go
// EnsureClient reconciles a single client AWG interface to desired state.
// Creates the awg-quick conf, runs `awg-quick up` if down, restarts if
// fingerprint changed. Idempotent; safe to call every 10s from awg_job.
func (m *Manager) EnsureClient(ci ClientInstance) error

// RemoveClient tears down a client interface (awg-quick down + rm conf).
func (m *Manager) RemoveClient(ifname string) error

// CollectClientTraffic returns handshake age + rx/tx for a client interface.
func (m *Manager) CollectClientTraffic(ifname string) (handshakeAge time.Duration, rx, tx int64, ok bool)
```

Конфиги хранятся в `/etc/amnezia/amneziawg/awgo-N.conf` (параллельно серверным `awgN.conf`).

## Reconcile loop (internal/web/job/awg_job.go)

Расширяем существующий `@every 10s` cron:

- Для каждого enabled `AwgOutbound` → `manager.EnsureClient(ci)`
- Для disabled или удалённых → `manager.RemoveClient(ifname)`
- Fingerprint-based restart (как у inbound)

### Startup orphan cleanup (user feedback 2026-07-20)

Reconcile каждые 10с — хорошо, но при **старте панели** (или после крэша) нужно **один раз явно** пройти по `awgo-*`:

1. Найти все `awgo-*` интерфейсы (`ip link show` или `awg show`) и `awgo-*.conf` в `/etc/amnezia/amneziawg/`
2. Для каждого: если `awgo-{N}` **не соответствует** записи в `awg_outbounds` с `Id=N` **или** `Enable=false` → `manager.RemoveClient("awgo-N")` (down + rm conf)
3. После этого — обычный reconcile поднимает нужные

Иначе после падения панели могут остаться «мёртвые» интерфейсы от прошлой сессии. Реализация: метод `Manager.SweepOrphanClients()` вызывается **один раз** при первом `EnsureClient` (через `sync.Once`, как у inbound `killStrayAwgInterfaces`). Не повторяется каждый тик — только на старте.

## Controller (internal/web/controller/awg.go)

Новые endpoints:

```
GET    /panel/api/awg-outbounds/list           — список с status (up/down, handshake age, rx/tx)
POST   /panel/api/awg-outbounds/add
POST   /panel/api/awg-outbounds/del/:id
POST   /panel/api/awg-outbounds/update/:id
POST   /panel/api/awg-outbounds/enable/:id
GET    /panel/api/awg-outbounds/status/:id
POST   /panel/api/awg-outbounds/test/:id        — ICMP test через outbound → latency
POST   /panel/api/awg-outbounds/parseConf       — parse pasted .conf → settings JSON (для формы)
```

Все endpoints — в LUCX-HOOK блоках существующего файла или в новом `internal/web/controller/awg_outbound.go`. Auth + CSRF как у остальных.

### Test-кнопка: ping -I (user feedback 2026-07-20)

Самый простой и достаточный способ — **system ping через интерфейс**:

```bash
ping -c 3 -W 2 -I awgo-{Id} 1.1.1.1
```

(или `8.8.8.8` / `cloudflare` как target). Через Xray делать сложнее и не нужно для «жив ли туннель».

Реализация `POST /test/:id`:
- Бэкенд: `exec.Command("ping", "-c", "3", "-W", "2", "-I", fmt.Sprintf("awgo-%d", id), "1.1.1.1")`
- Парсит вывод → `{ok: bool, latency_ms: int, error: string}`
- Target `1.1.1.1` (Cloudflare DNS, стабилен) — захардкожен или берётся из настроек
- Timeout 10с (3 пакета × 2с + overhead)
- Если интерфейс down → `ok=false, error="interface awgo-N is down"` (не запускаем ping)
- IPv6-фолбэк: если `address` IPv6, target тоже IPv6 (`2606:4700:4700::1111`), флаг `-6` (или `ping6`)

**Не использовать** curl через Xray freedom outbound — это сложнее (нужен Xray с outbound в конфиге, корректный routing rule) и не отвечает на вопрос «жив ли туннель до upstream».

## Service (internal/web/service/awg_outbound.go)

```go
type AwgOutboundService struct{}

func (s *AwgOutboundService) AddOutbound(o *model.AwgOutbound) (*model.AwgOutbound, error)
func (s *AwgOutboundService) DelOutbound(id int) error
func (s *AwgOutboundService) UpdateOutbound(o *model.AwgOutbound) error
func (s *AwgOutboundService) SetOutboundEnable(id int, enable bool) error
func (s *AwgOutboundService) GetOutbounds() ([]*model.AwgOutbound, error)
func (s *AwgOutboundService) GetOutbound(id int) (*model.AwgOutbound, error)
```

CRUD + defaultAwgOutboundSettings (генерация client keypair, если не задан), parseConf (parse .conf text → settings).

## Xray config injection (internal/web/service/xray.go)

Новая функция `injectAwgOutbounds(cfg *model.XrayConfig, outbounds []*model.AwgOutbound)`, вызывается из генератора Xray-конфига (рядом с `injectAwgEgress`):

- Для каждого enabled outbound добавляет freedom outbound с tag, sockopt.interface, sendThrough
- Если outbound имеет `outboundTag`, указанный в routing rules / AWG-inbound, Xray подхватывает его

## UI (XrayPage)

В `XrayPage.tsx`:
- `SECTION_SLUGS` → добавить `'awg-outbound'` после `'outbound'`
- Переименовать label вкладки `outbound` в **"Xray outbounds"** (i18n key `pages.xray.tabs.outbounds` → `pages.xray.tabs.xrayOutbounds`)
- Новая вкладка **"AWG outbounds"** (`pages.xray.tabs.awgOutbounds`)

**Новый компонент `frontend/src/pages/xray/awg-outbounds/AwgOutboundsTab.tsx`:**

```
[Таблица]
| Tag       | Remark   | Endpoint               | Tunnel IP    | Enable | Status                                  | Actions |
| awgo-1    | upstream | example.com:51820      | 10.9.0.5/32  | ✓      | ● Up (handshake 45s ago, ↓1.2M ↑0.8M) | edit/test/del |
| awgo-2    | backup   | backup.example:51820   | 10.9.0.6/32  | ✗      | ○ Down                                  | edit/enable/del |
```

- Status badge: `● Up` + handshake age + rx/tx (из `awg show awgo-N dump`) или `○ Down`
- Кнопка **"Add AWG outbound"** → `AwgOutboundFormModal`

**AwgOutboundFormModal:**
- **Форма** (react-hook-form):
  - Tag (auto `awgo-N`, edit)
  - Remark
  - Endpoint (обязательно)
  - Tunnel IP (обязательно, placeholder `10.9.0.5/32`)
  - PrivateKey (auto-generate кнопкой, или paste)
  - PublicKey (upstream, обязательно)
  - PSK (опционально)
  - Keepalive (default 25)
  - MTU (default 1320)
  - AllowedIPs (default `0.0.0.0/0, ::/0`)
  - **"Paste .conf"** кнопка → drawer с textarea → submit → `parseConf` API → автозаполнение формы
  - **"Advanced"** collapsible: obfuscation (Jc/S/H/I), DNS (клиентский, в отличие от серверного)
- Edit/delete/enable/disable actions
- **Test button** в строке таблицы → ICMP ping через outbound → popover с latency/ошибкой

## Tests

**Backend (`internal/awg/`):**
- `ClientInstanceFromOutbound` — parse settings JSON, validation (endpoint, address, publicKey)
- `(ClientInstance).fingerprint()` — stability, change detection
- `renderClientConf` — Table=off, Address, Endpoint, PersistentKeepalive, obfuscation lines
- `parseConf` (service) — parse pasted .conf → settings JSON, обратный к renderClientConf

**Backend (`internal/web/service/`):**
- CRUD на `AwgOutbound` (add/update/del/enable)
- `defaultAwgOutboundSettings` — генерация client keypair если privateKey пустой
- `parseConf` — парсинг `[Interface]` + `[Peer]` секций

**Frontend:**
- typecheck + lint

**Integration на test2:**
- Создать AWG outbound (endpoint = тестовый upstream или публичный AWG-сервер)
- `awg show awgo-1` → интерфейс up, peer подключён
- Routing rule `outboundTag: awgo-1` для тестового инбаунда → трафик уходит через upstream
- Кнопка Test → latency
- Disable → интерфейс down → `awg show` не содержит awgo-1

## Sequence (build order)

1. `internal/database/model/awg_outbound.go` — модель
2. `internal/database/migrate_awg_outbound.go` — миграция
3. `internal/awg/client_instance.go` — `ClientInstance` + `ClientInstanceFromOutbound` + `fingerprint`
4. `internal/awg/client_conf.go` — `renderClientConf` (Table=off!)
5. `internal/awg/manager.go` — `EnsureClient` / `RemoveClient` / `CollectClientTraffic`
6. `internal/web/service/awg_outbound.go` — CRUD + `parseConf` + `defaultAwgOutboundSettings`
7. `internal/web/service/xray.go` — `injectAwgOutbounds` (freedom + sockopt.interface)
8. `internal/web/controller/awg_outbound.go` — endpoints
9. `internal/web/job/awg_job.go` — reconcile расширение
10. `internal/web/web.go` — регистрация роутов (LUCX-HOOK)
11. Frontend: `frontend/src/schemas/awg-outbound.ts` — Zod schema
12. Frontend: `frontend/src/pages/xray/awg-outbounds/AwgOutboundsTab.tsx` + form modal
13. Frontend: `frontend/src/pages/xray/XrayPage.tsx` — новая вкладка + rename outbound tab label
14. Frontend: i18n (13 locales) — `awgOutbound*` keys
15. Frontend: codegen (`npm run gen`)
16. Tests: backend + frontend
17. Integration test на test2

## Risks / open questions

- **`sockopt.interface` поддержка в Xray-core:** нужно проверить, что upstream Xray поддерживает `sockopt.interface` (должен — freedom outbound + sockopt — стандартная фича). Если нет — fallback на `sendThrough` (source IP).
- **`Table = off` + `awg-quick`:** `awg-quick` понимает `Table = off` (стандартный WireGuard/AWG conf option). Не должно быть сюрпризов.
- **Несколько outbounds одновременно:** каждый `awgo-N` — отдельный интерфейс, отдельный upstream. Нет конфликтов. Routing rules решают, какой использовать.
- **Upstream без obfuscation:** если upstream не AWG (просто WireGuard), Jc/S/H не задаются → `renderClientConf` пропускает эти строки. Совместимо.
- **`I1-I5` в клиентском conf:** как у сервера, I1-I5 клиент-только (DPI-evasion перед handshake). Upstream их игнорирует. Записываем всегда, если заданы.

### PrivateKey generation (user feedback 2026-07-20)

**Не генерировать random bytes** — нужен валидный Curve25519 keypair. Два варианта:

1. **Host-side:** вызывать `awg genkey` / `awg pubkey` на сервере (как `awg-quick` — должен быть установлен). Минимум Go-кода, но зависимость от бинарника.
2. **Pure Go:** `golang.org/x/crypto/curve25519` — уже используется в `internal/util/wireguard` (`wgutil.GenerateWireguardKeypair`). **Рекомендуется** — симметрично с inbound, не требует fork/exec.

`defaultAwgOutboundSettings` вызывает `wgutil.GenerateWireguardKeypair()` (как `createDefaultAwgInboundSettings` во frontend и `defaultAwgClients` в backend). Публичный ключ upstream-сервера оператор вводит сам — его не генерируем.

### Directory scan filter (user feedback 2026-07-20)

Существующий inbound-код менеджера, скорее всего, смотрит на `/etc/amnezia/amneziawg/*.conf` (для orphan sweep). Нужно явно фильтровать:

- **Inbound-менеджер:** обрабатывает только `awgN.conf` (без префикса `awgo-`), игнорирует `awgo-*.conf`
- **Outbound-менеджер (новый):** обрабатывает только `awgo-*.conf`, игнорирует `awgN.conf`
- Orphan sweep для outbound: проверяет только `awgo-*` интерфейсы (через `awg show` или `ip link show`), не трогает `awgN` серверные

Реализация: явный `strings.HasPrefix(ifname, "awgo-")` / `!HasPrefix(ifname, "awgo-")` в фильтрах. Документируется в комментариях в коде.

### Interface-down behavior + sockopt fallback (user feedback 2026-07-20)

`sockopt.interface` в Xray при **отсутствии интерфейса** сейчас часто делает **fallback на default route** (известное поведение Xray-core: если интерфейс не существует, sockopt игнорируется и трафик уходит через системный default). Это **опасно** для нашего use case:

- Если `awgo-N` упал, а outbound остался в Xray-конфиге → трафик routing rules молча уходит через default route (мимо VPN), оператор не знает
- В статусе UI и в Test-кнопке нужно **явно показывать** это состояние: не просто `Down`, а `Down (traffic falls back to default route — WARNING)`
- Документация (README + tooltip в UI) должна предупреждать: при disable AwgOutbound outbound **полностью убирается** из Xray-конфига (см. ниже), но при **аварийном падении** интерфейса (kernel panic, awg-quick crash) — outbound остаётся в конфиге и Xray fallback на default route. Рекомендуется мониторинг статуса + alert.

**Mitigation в reconcile loop:** если `awgo-N` упал и не поднимается (awg-quick fail) после N попыток → логировать warning + статус в UI = `Down (fallback active)`. Test-кнопка при Down = показывает "interface down — traffic bypasses VPN" вместо ICMP-латентности.

### Xray config injection order (user feedback 2026-07-20)

`injectAwgOutbounds` должен вызываться **после** обычных Xray outbounds, но **до** balancers и routing rules — чтобы tag был доступен в:

- routing rules (`outboundTag: awgo-1`)
- balancers (`balancerTag` со ссылкой на `awgo-1` в selector)
- AWG-inbound `outboundTag` (который тоже попадает в routing rules)

**При disable AwgOutbound:** outbound **полностью убирается** из Xray-конфига (не blackhole). Причины:

1. Blackhole- outbound даст молчаливый дроп трафика — оператор не увидит, что routing rule ссылается на несуществующий outbound
2. Полное удаление → Xray при запуске упадёт с "outbound not found" если routing rule всё ещё ссылается на tag → **явная ошибка**, оператор видит и чинит
3. Это симметрично с тем, как обычные Xray outbounds обрабатываются (disable = удалить из конфига)

**При enable:** outbound добавляется обратно, Xray перегенерирует конфиг (через `needRestart` в `SetOutboundEnable`, как у `awgRoutesThroughXray`).

## License

Все новые файлы — PolyForm Noncommercial 1.0.0 (SPDX header), по AGENTS.md Rule 10.