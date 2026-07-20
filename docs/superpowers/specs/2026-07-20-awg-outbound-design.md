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

## Xray integration

При `enable=true` AWG outbound инжектится в generated Xray config:

```json
{
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "awgo-1",
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

- `tag` = `awgo-N` — используется в routing rules и `outboundTag` инбаундов
- `sockopt.interface` = имя kernel-интерфейса → Xray отправляет пакеты через него
- `sendThrough` = tunnel IP (опционально, для дополнительной страховки source-binding)

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

## License

Все новые файлы — PolyForm Noncommercial 1.0.0 (SPDX header), по AGENTS.md Rule 10.