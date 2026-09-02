<!-- LUCX-HOOK: LucX-UI fork README — Streamlined RU README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Продвинутая панель Xray** — AmneziaWG (ядро + родной, до 3.1), импорт существующего AWG, туннельные сайдкары и sidecar outbounds (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel), подписки Clash / Amnezia `vpn://` / Happ, geodata browser и RoscomVPN routing.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
  <a href="https://boosty.to/alexeylcp"><img src="https://img.shields.io/badge/boosty-subscribe-orange" alt="Boosty"></a>
</p>

<p align="center">
  <a href="README.en_US.md">English</a> |
  <b>Русский</b> |
  <a href="README.fa_IR.md">فارسی</a> |
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **Только для личного, некоммерческого, научного, исследовательского и образовательного использования.** Коммерческое использование — включая перепродажу VPN или платные панели — требует явного письменного разрешения по лицензии PolyForm Noncommercial 1.0.0.

---

## ⚡ Быстрый старт

Установка в одну строку на **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch и др.)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Опционально с Яндекса (SourceCraft), если GitHub недоступен. Без токенов и git — панель, geo и скрипты скачиваются одним пакетом:

```bash
mkdir -p /tmp/lucx-dist && curl -fsSL https://codeload.sourcecraft.tech/alexeylcp/lucx-ui/tarball/refs/heads/dist | tar -xz --strip-components=1 -C /tmp/lucx-dist && sudo bash /tmp/lucx-dist/install.sh --yandex
```

Дальше `x-ui update` ходит туда же (`/etc/x-ui/install-source`).

<details>
<summary><b>🛠️ Дополнительные варианты установки (Cloud-Init, Docker, PostgreSQL, Env Vars)</b></summary>

### Автоматическая установка (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
Учётные данные сохраняются в `/etc/x-ui/install-result.env`.

### Docker
Образы собираются на каждый релизный тег (`ghcr.io/alexeylcp/lucx-ui`):

```bash
docker run -d \
  --name lucx-ui \
  --restart unless-stopped \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  -p 2053:2053 \
  -v $PWD/db/:/etc/x-ui/ \
  ghcr.io/alexeylcp/lucx-ui:latest
```

Или `docker compose up -d` (тянет тот же образ; `docker compose build` собирает локально).

С PostgreSQL — раскомментировать `XUI_DB_*` в `docker-compose.yml` и:

```bash
docker compose --profile postgres up -d
```

### Основные переменные окружения (`/etc/default/x-ui`)
| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд БД (`sqlite` или `postgres`) | `sqlite` |
| `XUI_DB_DSN` | DSN для PostgreSQL | — |
| `XUI_ENABLE_FAIL2BAN` | Включение Fail2ban для лимита IP | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ Почему LucX-UI?

[3x-ui](https://github.com/MHSanaei/3x-ui) — отличная мультипротокольная панель с современным React 19 + Ant Design 6 фронтендом. LucX-UI сохраняет всё, что есть у 3x-ui, и добавляет то, чего у апстрима нет: **kernel AmneziaWG** (рядом с родным `amneziawg` апстрима), **импорт существующего AWG**, **туннельные сайдкары** (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel), **расширенные подписки** (Clash Meta AWG, Amnezia `vpn://`, Happ) и **geodata-инструменты** (браузер в панели + пакеты RoscomVPN):

| Возможность | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (kernel sidecar через `awg-quick`) | ✗ | ✓ |
| Родной AmneziaWG inbound (`amneziawg`, userspace) | ✓ | ✓ |
| Импорт существующего AWG (awg-multi / toolza3 / Docker) | ✗ | ✓ |
| Kernel AWG без модуля → встроенный amneziawg-go | ✗ | ✓ |
| Живая скорость AWG-клиентов и инбаундов в панели | ✗ | ✓ |
| AWG CPS обфускация (TLS / DNS / SIP / QUIC + отпечатки браузеров) | ✗ | ✓ |
| AWG outbound — VPN chaining к upstream AWG-серверам (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1 (`RandomTrailers` / `DisableCookies`, анти-DPI) | ✗ | ✓ |
| Пресеты версий клиентских конфигов (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| Диагностика AWG из панели (routing / NAT / peers / handshakes) | ✗ | ✓ |
| AWG в Clash Meta + подписка Amnezia `/awg/` (`.conf` / `vpn://`) | ✗ | ✓ |
| Туннельный сайдкар NaiveProxy (Caddy + forward_proxy, под надзором панели) | ✗ | ✓ |
| Per-client креды NaiveProxy + `naive+https://` в подписках | ✗ | ✓ |
| NaiveProxy → Xray routing (SOCKS loopback-мост, опционально) | ✗ | ✓ |
| Туннельный сайдкар olcRTC (WebRTC через meet-комнаты, под надзором) | ✗ | ✓ |
| Туннельный сайдкар qWDTT (WireGuard через VK TURN, под надзором) | ✗ | ✓ |
| Туннельный сайдкар mieru (`mita`, per-client трафик, под надзором) | ✗ | ✓ |
| Сайдкар TrustTunnel (протокол AdGuard VPN, похож на HTTPS, под надзором) | ✗ | ✓ |
| Sidecar outbounds (клиент Naive / mieru / TrustTunnel → SOCKS, routing и пулы) | ✗ | ✓ |
| Geodata browser — выбор категорий geosite/geoip из панели | ✗* | ✓ |
| Пакет RoscomVPN geo (`geoip/geosite_ROSCOM.dat`, списки РКН) | ✗ | ✓ |
| Профили маршрутизации Happ (RoscomVPN deeplink + custom) | ✗ | ✓ |
| Smart Cluster outbound-связи | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 фронтенд | ✓ | ✓ (inherited) |
| Все протоколы Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Бесшовный upstream sync (изоляция LUCX-HOOK, 49 файлов) | — | ✓ |

\* Upstream [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) (ещё не влит) — портирован в LucX-UI.

Kernel sidecar (как у MTProto `mtg` в 3x-ui) означает, что AWG работает как настоящий интерфейс ядра — а не как userspace-обёртка — поэтому Xray маршрутизирует расшифрованный трафик через собственный TUN inbound, давая вам полную мощь маршрутизации, sniffing'а и доменных правил Xray на AWG-трафике. Модуля нет — тот же LucX-inbound `awg` поднимается на встроенном amneziawg-go. Рядом в панели остаётся родной протокол апстрима `amneziawg`.

---

## 🌟 О проекте LucX-UI

**LucX-UI** — расширенный форк [3x-ui](https://github.com/MHSanaei/3x-ui) (сейчас синхронизирован с upstream **v3.7.0**). Поверх стоковых протоколов Xray: **AmneziaWG** в двух режимах — kernel sidecar `awg` (как MTProto/`mtg`) и родной `amneziawg` апстрима, до **AWG 3.1**; **импорт** awg-multi / toolza3 / Docker; **туннельные сайдкары** под надзором панели (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel), расширенные **подписки** (Clash Meta AWG, Amnezia `/awg/` + `vpn://`, Happ routing) и **geodata browser** со стоком RoscomVPN. 100% совместимость с upstream через строгую изоляцию `LUCX-HOOK`.

### 🛡️ Возможности AmneziaWG (AWG)
- **AWG Inbounds & Outbounds** — kernel sidecar (`awg-quick`), клиентский режим dial-out к upstream AWG-серверам (`awgo-{id}`), цикл автоматического reconcile каждые 10 секунд и сборщик DKMS kernel-модуля.
- **Два движка** — в панели и `AmneziaWG (ядро)` (`awg-quick`, если модуль есть), и родной `amneziawg` апстрима. Модуля нет — LucX-inbound `awg` идёт через встроенный amneziawg-go (SOCKS в Xray); kernel-путь не меняется, когда модуль на месте.
- **Импорт существующего AWG** — баннер на Inbounds: awg-multi / toolza3 / Docker Amnezia. Ключи, IP, порт и обфускация как есть; kernel-интерфейс переименовывается на месте (handshake не падает).
- **Живая скорость** — колонки скорости на Clients / Inbounds для AWG (stats Xray его не видит).
- **Продвинутая обфускация** — пресеты Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4), мимикрия CPS-пакетов (TLS, DNS, SIP, QUIC) и TLS-отпечатки браузеров (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — защита заголовков AmneziaWG 3 c автоматически генерируемыми 32-байтовыми ключами; серверный потолок версии управляет эмиссией фич на клиента.
- **AWG 3.1** — `RandomTrailers` (случайный хвост пакета, анти-DPI по размерам) и `DisableCookies`; kernel-модуль и тулзы автоматически обновляются до v3.1 при обновлении панели.
- **Пресеты версий клиентов** — генерация клиентских конфигов для AWG 1.5 / 2 / 3 / 3.1 из одного inbound — выберите формат, который понимает ваше клиентское приложение.
- **Live Signature Capture** — преобразование реальных QUIC-handshake'ов с front-доменов в параметры обфускации I1–I5.
- **Маршрутизация и диагностика** — двойной режим маршрутизации (Kernel NAT и Route through Xray с policy routing и sniffing'ом) + однокликовая диагностика из панели.

### 🚇 Туннельные сайдкары (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel)
- **NaiveProxy** — Caddy с плагином `forward_proxy` (форк [klzgrad](https://github.com/klzgrad/forwardproxy), HTTP/2 padding) работает как сайдкар под надзором панели: рендер Caddyfile, start/stop/restart с crash-revive reconcile и трёхуровневым health-probe (process → TCP → TLS).
- **Per-client креды** — каждый включённый клиент панели автоматически получает личную пару `basic_auth` (выводится из секрета панели, ничего не хранится); disable клиента отзывает креды на следующем reconcile.
- **Подписки** — в подписке каждого клиента его личная ссылка `naive+https://` рядом с Xray/AWG (стандарт NekoBox / husi / Exclave), плюс QR-код и генератор сильного пароля в панели.
- **UX панели** — Auto TLS (Let's Encrypt) или свой cert/key, raw-Caddyfile режим с валидацией `caddy adapt`, preview Caddyfile, логи процесса, upload/download бинарника.
- **Маршрут через Xray (опционально)** — Caddy ходит к назначениям через скрытый loopback SOCKS-мост (`upstream socks5://127.0.0.1:…`, нативный forward_proxy — без патча бинарника) с тегом `lucx-tunnel-naive`, так что трафик NaiveProxy получает полный роутинг / sniffing / доменные правила Xray (как MTProto). По умолчанию — прямой egress.
- **olcRTC** — TCP-over-WebRTC туннель через легальную видео-комнату ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc), WTFPL): Jitsi / Яндекс Телемост / WB Stream. На VPS нет публичных портов — бинарник входит в комнату как тихий участник. Панель рендерит server YAML, супервизит процесс и отдаёт копируемый `olcrtc://` URI для клиентов owenclave / olcbox.
- **qWDTT** — WireGuard через TURN-релеи VK Calls ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android), GPL-3.0 server). Нужен root (TUN + NAT). Панель супервизит процесс, отдаёт `qwdtt://` / `wdtt://` и JSON-подписку для Android-клиента. Оператор передаёт живые VK call hash.
- **mieru** — censorship-resistant прокси поверх собственного протокола вместо TLS ([enfein/mieru](https://github.com/enfein/mieru) `mita`, GPL-3.0). Мульти-клиент с HMAC-кредами на каждого клиента панели, per-client трафик и онлайн, шер-ссылка `mierus://`. Клиенты: mieru CLI, mihomo, Clash Verge Rev, husi, Exclave.
- **TrustTunnel** — протокол AdGuard VPN ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel), Apache-2.0): трафик неотличим от HTTPS (HTTP/1.1 + HTTP/2 + QUIC). Использует ACME-серт панели (нужен домен с выпущенным сертом), отдаёт `tt://?` deep-link для Flutter / CLI клиентов.
- **Sidecar outbounds** — клиентский режим Naive / mieru / TrustTunnel: вставил share-ссылку (`naive+https://` / `mierus://` / `tt://`), тег появляется в routing и пулах балансировщиков (как AWG outbound). Выключение = blackhole (не утекает в `direct`). Клиентские бинарники в tar.gz.

### 📦 Подписки, geodata и маршрутизация клиентов
- **Подписка Amnezia** — отдельный endpoint `/awg/{subId}` отдаёт чистый AmneziaWG `.conf` (или `?format=vpn` → тело `vpn://…`) для AmneziaVPN / Happ; ссылки рядом с Clash / JSON / base64 в панели и Telegram-боте.
- **AWG в Clash Meta** — подписка эмитит пиры AmneziaWG через `amnezia-wg-option`, чтобы Clash Meta принимал AWG вместе с VLESS/Trojan.
- **Geodata browser** — открыть любой `geoip*.dat` / `geosite*.dat` из UI роутинга, поиск категорий, multi-select в правило (порт upstream [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) от [STRENCH0](https://github.com/STRENCH0)).
- **Пакет RoscomVPN geo** — сток `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([hydraponique/roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)): списки РКН (`category-geoblock-ru`, `category-ru`, ads, YouTube / Telegram / Steam, …). Обновление: панель Version → Geofiles или меню `x-ui`.
- **Профили Happ** — Settings → Happ: встроенный deeplink RoscomVPN и free-text custom (из [hydraponique/roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

### 🚀 Базовые фичи 3x-ui
- **Протоколы:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Транспорты и безопасность:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Управление:** Квоты трафика, IP-лимиты (Fail2ban), статус онлайн, подписки, Telegram-бот, REST API, Multi-node, SQLite / PostgreSQL.

<details>
<summary><b>📸 Скриншоты</b></summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

</details>

---

## 🔄 Переход с 3x-ui и существующего AWG

LucX-UI использует ту же базу схемы Xray-core / SQLite (или PostgreSQL), что и 3x-ui, а AWG-таблицы создаются автоматически при первом запуске. Для установки поверх существующего 3x-ui сначала сделайте резервную копию базы, затем запустите стандартную команду установки:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

AWG kernel-модуль собирается автоматически установщиком (`bin/install-awg-module.sh`, DKMS). После установки запустите `x-ui` в консоли, чтобы подтвердить версию AWG kernel-модуля, и начните добавлять AWG inbounds из панели.

### Ключи AWG в панели

Отдельного экрана «ключи» нет. Ключ = клиент на inbound AmneziaWG:

1. **Inbounds → Add inbound**, протокол **AmneziaWG (ядро)** (или родной `amneziawg`).
2. **Clients → Add client**, привязать к этому inbound.
3. QR / скачать `.conf` / подписка `/awg/{subId}`.

По умолчанию подсеть inbound `/24` — до ~253 клиентов.

### С существующего AWG на хосте

Если на сервере уже крутится **awg-multi**, **toolza3** или **Docker Amnezia** — панель **не сносит** чужие `awg0`/`awg1`. На Inbounds появится баннер **«Импорт существующего AWG»**: превью пиров → один inbound на интерфейс. Ключи / IP / порт / обфускация копируются как есть. Kernel-интерфейс переименовывается на месте (`awg{id}`), handshake не падает. Userspace/Docker: остановите старый менеджер — клиенты переподключатся один раз.

Без kernel-модуля LucX-инбаунды `awg` всё равно поднимаются на встроенном amneziawg-go. Родной протокол апстрима `amneziawg` доступен в панели рядом.

---

## 📜 Лицензия и условия

Проект публикуется под **двумя лицензиями** на свой код плюс third-party бинарники/данные по условиям апстрима (полная матрица в [LICENSING.md](LICENSING.md)):

| Компонент | Лицензия |
|---|---|
| Исходный код оригинального 3x-ui | **GPL-3.0** |
| Компоненты LucX-UI (`internal/awg/`, `internal/lucx/`, LucX-страницы frontend) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| Плагин `forward_proxy` ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy / `bin/naive-client-*` ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/mieru-*` (`mita`, [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| AmneziaWG kernel module & tools ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (модуль; ставится на хост) |
| Сток geo `.dat` (Loyalsoldier / IR / RU / ROSCOM) | Условия каждого датасета (см. LICENSING.md) |

Туннельные бинарники — **дочерние процессы**, панель их не линкует. GPL у qWDTT относится к этому бинарнику и его исходникам, не к PolyForm-коду LucX.

---

## 🤝 Благодарности и источники

LucX-UI стоит на плечах многих open-source проектов и людей. Спасибо вам.

### Тестировщики и контрибьюторы
- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13) — AWG `routeThroughXray`), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, команда **[3x-ui](https://github.com/MHSanaei/3x-ui)** ([MHSanaei](https://github.com/MHSanaei) и контрибьюторы).

### Upstream PR, которые мы портировали / на которые опирались
- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) *feat(xray): browse geosite/geoip categories from routing rules* (geodata browser).

### Проекты и вдохновение
| Проект | Что используем | Лицензия |
|---|---|---|
| [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) | Базовая панель | GPL-3.0 |
| [amnezia-vpn](https://github.com/amnezia-vpn) — kernel module & tools | Протокол AmneziaWG / AWG3 | GPL-2.0 (модуль) |
| [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) | Протокол / клиент-референс NaiveProxy | BSD-3-Clause |
| [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) + Caddy | Бинарник сайдкара NaiveProxy | MIT + Apache-2.0 |
| [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) | Ядро olcRTC | WTFPL |
| [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) | Сервер qWDTT | GPL-3.0 |
| [enfein/mieru](https://github.com/enfein/mieru) | Сервер mieru `mita` | GPL-3.0 |
| [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) | Эндпоинт TrustTunnel | Apache-2.0 |
| [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) | Референс интеграции Caddyfile | — |
| [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) | Концепция туннельных сайдкаров в панели (qWDTT / olcRTC) | — |
| [hydraponique/3x-ui](https://github.com/hydraponique/3x-ui), [roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite), [roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing) | Пакет RoscomVPN geo + профили Happ | Upstream |
| [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat), [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules), [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) | Сток geoip/geosite | Upstream |
| [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) | Вдохновение по AWG ops | — |
| [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls) | Референсы TLS-отпечатков для CPS | — |

---

## ☕ Поддержать проект

LucX-UI бесплатен для личного использования. **Понравилось — ставь ⭐** репозиторию: это помогает другим найти проект и поддерживает разработку. Донаты необязательны, но всегда приятны:

| Способ | Реквизиты |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🟠 **Boosty** (подписка) | [boosty.to/alexeylcp](https://boosty.to/alexeylcp) |
| 🟠 **Boosty** (разово) | [boosty.to/alexeylcp/donate](https://boosty.to/alexeylcp/donate) |
| 🇷🇺 **YooMoney** (RUB, Россия) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ Для разработчиков

<details>
<summary><b>Архитектура, сборка и upstream sync (нажмите, чтобы развернуть)</b></summary>

**Архитектура и правило изоляции.** Весь код LucX живёт в изолированных пакетах (`internal/awg/`, `internal/lucx/`); изменения файлов upstream 3x-ui вносятся только внутри маркеров `// LUCX-HOOK` / `// END LUCX-HOOK`, поэтому каждый upstream-релиз сводится к почти тривиальному портированию. См. [AGENTS.md](AGENTS.md) — полная карта архитектуры, 10 правил, известные проблемы и шаблоны отладки.

**Сборка из исходников** (требуется Go 1.23+, Node.js 20+, gcc — только Linux, CGO для SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# проверка перед push: bin/check-lucx.sh  (gofumpt на 49 файлах LucX)
```

**Процедура upstream sync** (проверено на v3.5.0→v3.6.0, 103 коммита / 432 файла / 7 конфликтов):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# разрешать блок за блоком (см. AGENTS.md правило 8) — никогда не использовать blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # сравнить количество маркеров до/после, чтобы выявить потерянные блоки
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

<!-- END LUCX-HOOK -->
