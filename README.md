<!-- LUCX-HOOK: LucX-UI fork README — Streamlined RU README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Продвинутая панель Xray** — AmneziaWG (ядро + родной, до 3.1), импорт существующего AWG, туннельные сайдкары и sidecar outbounds (NaiveProxy · olcRTC · qWDTT · CSQTT · mieru · TrustTunnel · AnyTLS · Telegram WEB proxy), подписки Clash / Amnezia `vpn://` / Happ, RoscomVPN geo + Happ routing.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="docs/LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
  <a href="https://boosty.to/alexeylcp"><img src="https://img.shields.io/badge/boosty-subscribe-orange" alt="Boosty"></a>
</p>

<p align="center">
  <a href="docs/readme/README.en_US.md">English</a> |
  <b>Русский</b> |
  <a href="docs/readme/README.fa_IR.md">فارسی</a> |
  <a href="docs/readme/README.ar_EG.md">العربية</a> |
  <a href="docs/readme/README.zh_CN.md">中文</a> |
  <a href="docs/readme/README.es_ES.md">Español</a> |
  <a href="docs/readme/README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **Только для личного, некоммерческого, научного, исследовательского и образовательного использования.** Коммерческое использование — включая перепродажу VPN или платные панели — требует явного письменного разрешения по лицензии PolyForm Noncommercial 1.0.0.

---

## ⚡ Быстрый старт

Установка одной командой на **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch и др.)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Если GitHub недоступен — те же файлы можно взять с Яндекса (SourceCraft). Токены и git не нужны: панель, geo-файлы и скрипты скачиваются одним архивом:

```bash
mkdir -p /tmp/lucx-dist && curl -fsSL https://codeload.sourcecraft.tech/alexeylcp/lucx-ui/tarball/refs/heads/dist | tar -xz --strip-components=1 -C /tmp/lucx-dist && sudo bash /tmp/lucx-dist/install.sh --yandex
```

Дальше `x-ui update` качает обновления оттуда же (источник записывается в `/etc/x-ui/install-source`).

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

Или `docker compose up -d` — образ тот же; `docker compose build` собирает локально.

С PostgreSQL: раскомментируйте `XUI_DB_*` в `docker-compose.yml` и запустите:

```bash
docker compose --profile postgres up -d
```

### Основные переменные окружения (`/etc/default/x-ui`)
| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд БД (`sqlite` или `postgres`) | `sqlite` |
| `XUI_DB_DSN` | DSN для PostgreSQL | — |
| `XUI_ENABLE_FAIL2BAN` | Включить Fail2ban для принудительного лимита IP | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `notice`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ Почему LucX-UI?

[3x-ui](https://github.com/MHSanaei/3x-ui) — отличная мультипротокольная панель с современным фронтендом (React 19 + Ant Design 6). LucX-UI сохраняет всё, что есть в 3x-ui, и добавляет то, чего апстриму не хватает: **kernel AmneziaWG** (рядом с родным `amneziawg` из апстрима), **импорт существующего AWG**, **туннельные сайдкары** (NaiveProxy · olcRTC · qWDTT · CSQTT · mieru · TrustTunnel · AnyTLS · Telegram WEB proxy), **расширенные подписки** (Clash Meta AWG, Amnezia `vpn://`, Happ) и **пакеты RoscomVPN geo / профили Happ** (geodata browser при этом уже есть в апстриме — [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165), v3.7.0):

<details>
<summary><b>Сравнение с 3x-ui</b></summary>

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
| Туннельный сайдкар CSQTT (TURN/RTP, под надзором; не qWDTT) | ✗ | ✓ |
| Туннельный сайдкар mieru (`mita`, per-client трафик, под надзором) | ✗ | ✓ |
| Туннельный сайдкар AnyTLS (anytls-go, TLS-прокси) | ✗ | ✓ |
| Сайдкар TrustTunnel (протокол AdGuard VPN, похож на HTTPS, под надзором) | ✗ | ✓ |
| Sidecar outbounds (клиент Naive / mieru / TrustTunnel → SOCKS, routing и пулы) | ✗ | ✓ |
| Geodata browser — выбор категорий geosite/geoip из панели | ✓ | ✓ |
| Пакет RoscomVPN geo (`geoip/geosite_ROSCOM.dat`, списки РКН) | ✗ | ✓ |
| Профили маршрутизации Happ (RoscomVPN deeplink + custom) | ✗ | ✓ |
| Outbound-ссылки Smart Cluster (генератор outbound-конфигов из inbound) | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 фронтенд | ✓ | ✓ (inherited) |
| Все протоколы Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Telegram WEB proxy inbound (`tproxy`, t.me/webproxy) | ✗ | ✓ |
| Бесшовный upstream sync (изоляция LUCX-HOOK) | — | ✓ |

</details>

Kernel sidecar (так же, как MTProto `mtg` в 3x-ui) означает, что AWG работает как настоящий интерфейс ядра, а не userspace-обёртка: Xray маршрутизирует расшифрованный трафик через собственный TUN inbound, поэтому на AWG-трафике работают все возможности Xray — роутинг, sniffing, доменные правила. Если модуль собрать не удалось, тот же LucX-inbound `awg` поднимается на встроенном amneziawg-go. Родный протокол апстрима `amneziawg` остаётся в панели рядом.

---

## 🌟 О проекте LucX-UI

**LucX-UI** — расширенный форк [3x-ui](https://github.com/MHSanaei/3x-ui) (сейчас синхронизирован с upstream **v3.8.5**). К штатным протоколам Xray добавляются: **AmneziaWG** в двух режимах — kernel sidecar `awg` (как MTProto/`mtg`) и родной `amneziawg` апстрима — вплоть до **AWG 3.1**; **импорт** awg-multi / toolza3 / Docker; **туннельные сайдкары** под надзором панели (NaiveProxy, olcRTC, qWDTT, CSQTT, mieru, TrustTunnel, AnyTLS); расширенные **подписки** (Clash Meta AWG, Amnezia `/awg/` + `vpn://`, Happ routing); **Telegram WEB proxy** (`tproxy`) и **пакет RoscomVPN geo** (браузер категорий — общий с апстримом начиная с v3.7.0). 100% совместимость с upstream обеспечивает строгая изоляция `LUCX-HOOK`.

<details>
<summary><b>🛡️ Возможности AmneziaWG (AWG)</b></summary>

- **AWG Inbounds & Outbounds** — kernel sidecar (`awg-quick`), клиентский режим dial-out к upstream AWG-серверам (`awgo-{id}`), автоматический reconcile каждые 10 секунд и встроенный сборщик DKMS kernel-модуля.
- **Два движка** — в панели доступны и kernel-режим `AmneziaWG (ядро)` (`awg-quick`, если модуль установлен), и родной `amneziawg` из апстрима. Если модуля нет, LucX-inbound `awg` работает через встроенный amneziawg-go (SOCKS в Xray); с модулем — через ядро, без смены настроек.
- **Импорт существующего AWG** — на Inbounds появляется баннер импорта: awg-multi / toolza3 / Docker Amnezia. Ключи, IP, порт и обфускация переносятся как есть; kernel-интерфейс переименовывается на месте, handshake при этом не падает.
- **Живая скорость** — колонки скорости AWG-клиентов и инбаундов на страницах Clients / Inbounds (Xray-статистика AWG не видит).
- **Продвинутая обфускация** — пресеты Lite / Standard / Pro / Premium (Jc/Jmin/Jmax/S1–S4/H1–H4, у Premium ещё и размеры TLS 1.3 handshake + HPK), мимикрия CPS-пакетов (TLS, DNS, SIP, QUIC) и TLS-отпечатки браузеров (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — защита заголовков AmneziaWG 3 с автоматически генерируемыми 32-байтовыми ключами; максимальная версия протокола, заданная на сервере, определяет, какие фичи пишутся в клиентские конфиги.
- **AWG 3.1** — `RandomTrailers` (случайный хвост пакета, ломает DPI-классификацию по размерам) и `DisableCookies`; kernel-модуль и утилиты автоматически обновляются до v3.1 вместе с панелью.
- **Пресеты версий клиентов** — генерация клиентских конфигов AWG 1.5 / 2 / 3 / 3.1 из одного inbound: выберите формат, который понимает ваше клиентское приложение.
- **Live Signature Capture** — обфускационные параметры I1–I5 собираются из реальных QUIC-handshake'ов с front-доменов.
- **Маршрутизация и диагностика** — два режима маршрутизации (Kernel NAT и Route through Xray с policy routing и sniffing) плюс диагностика AWG одним кликом из панели.

</details>

<details>
<summary><b>🚇 Туннельные сайдкары (NaiveProxy, olcRTC, qWDTT, CSQTT, mieru, TrustTunnel, AnyTLS, Telegram WEB proxy)</b></summary>

- **NaiveProxy** — Caddy с плагином `forward_proxy` (форк [klzgrad](https://github.com/klzgrad/forwardproxy), HTTP/2- и HTTP/3-padding) работает как сайдкар под присмотром панели: панель сама рендерит Caddyfile, управляет start/stop/restart с автоматическим перезапуском при падении и трёхуровневой проверкой здоровья (процесс → TCP → TLS).
- **Per-client креды** — каждый включённый клиент панели автоматически получает личную пару `basic_auth` (она выводится из секрета панели и нигде не хранится); если клиента выключить, креды отзываются на следующем reconcile.
- **Подписки** — в подписке каждого клиента рядом с Xray/AWG появляется его личная ссылка `naive+https://` (стандарт NekoBox / husi / Exclave); в панели есть QR-код и генератор сильного пароля.
- **UX панели** — Auto TLS (Let's Encrypt) или свой cert/key, raw-режим Caddyfile с проверкой через `caddy adapt`, предпросмотр Caddyfile, логи процесса, загрузка/скачивание бинарника.
- **Маршрут через Xray (опционально)** — Caddy может ходить к назначениям через скрытый loopback SOCKS-мост (`upstream socks5://127.0.0.1:…` в нативном forward_proxy, без патчей бинарника) с тегом `lucx-tunnel-naive`: трафик NaiveProxy получает полный роутинг / sniffing / доменные правила Xray (как MTProto). По умолчанию egress прямой.
- **olcRTC** — TCP-over-WebRTC туннель через легальную видео-комнату ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc), WTFPL): Jitsi / Яндекс Телемост / WB Stream. Публичные порты на VPS не нужны — бинарник входит в комнату как обычный тихий участник. Панель рендерит server YAML, следит за процессом и выдаёт копируемый `olcrtc://` URI для клиентов owenclave / olcbox.
- **qWDTT** — WireGuard через TURN-релеи VK Calls ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android), GPL-3.0 server). Нужен root (TUN + NAT). Панель следит за процессом и выдаёт `qwdtt://` / `wdtt://` и JSON-подписку для Android-клиента. Оператор передаёт живые VK call hash сам.
- **CSQTT** — TURN/RTP туннель ([amurcanov/csqtt](https://github.com/amurcanov/csqtt), PolyForm NC). С qWDTT не совместим (не живут на одном хосте). Один пароль = одно устройство. Share-ссылка `csqtt://connect?v=2…`. Клиенты: Android CSQTT APK, iOS [anton48/vk-turn-proxy-ios](https://github.com/anton48/vk-turn-proxy-ios) в режиме CSQTT. Коммерческая лицензия LucX на CSQTT не распространяется.
- **mieru** — прокси, устойчивый к цензуре, поверх собственного протокола вместо TLS ([enfein/mieru](https://github.com/enfein/mieru) `mita`, GPL-3.0). Мульти-клиент: у каждого клиента панели свои HMAC-креды, отдельный счётчик трафика и онлайн-статус, share-ссылка `mierus://`. Клиенты: mieru CLI, mihomo, Clash Verge Rev, husi, Exclave.
- **TrustTunnel** — протокол AdGuard VPN ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel), Apache-2.0): трафик неотличим от обычного HTTPS (HTTP/1.1 + HTTP/2 + QUIC). Использует ACME-серт панели (нужен домен с выпущенным сертом), для Flutter / CLI клиентов выдаёт `tt://?` deep-link.
- **Сайдкар AnyTLS** — [anytls/anytls-go](https://github.com/anytls/anytls-go) `anytls-server` с LucX-оверлеем сертификатов: TLS-прокси, разбивающий внешний TLS-handshake, чтобы спрятать фингерпринт TLS-in-TLS. Серты — ACME панели (`webCertFile`/`webKeyFile`) или свои certFile/keyFile (без серта или если SNI не в SAN — inbound не сохранится). Один общий пароль на inbound; share `anytls://pass@host:port/?sni=…`. Клиенты: sing-box, mihomo, Shadowrocket, Stash, Loon.
- **Telegram WEB proxy (`tproxy`)** — сайдкар `tproxy-server` + официальный MTProxy + Caddy TLS reverse_proxy на `hostname:443`, клиенту выдаётся `t.me/webproxy`. Маршрут «через Xray» пока **припаркован** (MTProxy ходит напрямую; см. lucx.211).
- **Sidecar outbounds** — клиентский режим Naive / mieru / TrustTunnel: вставляете share-ссылку (`naive+https://` / `mierus://` / `tt://`) — тег появляется в routing и пулах балансировщиков (как у AWG outbound). При выключении трафик уходит в blackhole, а не в `direct` — ничего не утекает. Клиентские бинарники лежат в tar.gz.

</details>

<details>
<summary><b>📦 Подписки, geodata и маршрутизация клиентов</b></summary>

- **Подписка Amnezia** — отдельный endpoint `/awg/{subId}` отдаёт чистый AmneziaWG `.conf` (или `?format=vpn` → тело `vpn://…`) для AmneziaVPN / Happ; ссылки появляются рядом с Clash / JSON / base64 в панели и Telegram-боте.
- **AWG в Clash Meta** — подписка выдаёт пиры AmneziaWG через `amnezia-wg-option`, так что Clash Meta принимает AWG вместе с VLESS/Trojan.
- **Geodata browser** — откройте любой `geoip*.dat` / `geosite*.dat` прямо из UI роутинга: поиск категорий, multi-select в правило (в апстриме с [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0, автор [STRENCH0](https://github.com/STRENCH0)).
- **Пакет RoscomVPN geo** — свежие `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([hydraponique/roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)): списки РКН (`category-geoblock-ru`, `category-ru`, ads, YouTube / Telegram / Steam, …). Обновление: панель Version → Geofiles или меню `x-ui`.
- **Профили Happ** — Settings → Happ: выбор источника маршрутизации — встроенные deeplink RoscomVPN (Default / JsonSub / Whitelist) или свой free-text (на основе [hydraponique/roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

</details>

<details>
<summary><b>🚀 Базовые фичи 3x-ui</b></summary>

- **Протоколы:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, TUIC, MTProto, HTTP, SOCKS, TUN.
- **Транспорты и безопасность:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Управление:** Квоты трафика, IP-лимиты (Fail2ban), статус онлайн, подписки, Telegram-бот, REST API, Multi-node, SQLite / PostgreSQL.

</details>

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

LucX-UI использует ту же схему БД Xray-core / SQLite (или PostgreSQL), что и 3x-ui; AWG-таблицы создаются автоматически при первом запуске. Чтобы поставить поверх работающего 3x-ui, сначала сделайте резервную копию базы, затем запустите стандартную команду установки:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Установщик автоматически собирает AWG kernel-модуль (`bin/install-awg-module.sh`, DKMS). После установки запустите `x-ui` в консоли, чтобы увидеть версию модуля, и добавляйте AWG inbounds из панели.

**После установки:** подписки (`/sub/`, `/json/`, `/clash/`, `/awg/`) слушают **отдельный порт** (по умолчанию **2096**), а не порт панели — reverse proxy должен проксировать и его. Кастомные geo-файлы кладите под **отдельным именем** — stock-имена (`geoip.dat` / `geosite.dat` и `_IR` / `_RU` / `_ROSCOM`) перезаписываются при обновлении geofile.

<details>
<summary><b>Ключи AWG в панели</b></summary>

Отдельного экрана «ключи» в панели нет — ключом считается клиент AmneziaWG-inbound:

1. **Inbounds → Add inbound**, протокол **AmneziaWG (ядро)** (или родной `amneziawg`).
2. **Clients → Add client**, привязать к этому inbound.
3. QR / скачать `.conf` / подписка `/awg/{subId}`.

По умолчанию inbound получает подсеть `/24` — до ~253 клиентов.

</details>

<details>
<summary><b>С существующего AWG на хосте</b></summary>

Если на сервере уже крутится **awg-multi**, **toolza3** или **Docker Amnezia** — панель **не трогает** чужие `awg0`/`awg1`. На странице Inbounds появится баннер **«Импорт существующего AWG»**: превью пиров → один inbound на интерфейс. Ключи / IP / порт / обфускация копируются как есть. Kernel-интерфейс переименовывается на месте (`awg{id}`), handshake при этом не рвётся. Userspace/Docker: остановите старый менеджер — клиенты переподключатся один раз.

Без kernel-модуля LucX-инбаунды `awg` всё равно поднимаются на встроенном amneziawg-go. Родной протокол апстрима `amneziawg` доступен в панели рядом.

</details>

---

## 📜 Лицензия и условия

Проект публикуется под **двумя лицензиями** на собственный код, а third-party бинарники/данные распространяются по условиям их апстрима (полная матрица — в [LICENSING.md](docs/LICENSING.md)):

<details>
<summary><b>Матрица лицензий</b></summary>

| Компонент | Лицензия |
|---|---|
| Исходный код оригинального 3x-ui | **GPL-3.0** |
| Компоненты LucX-UI (`internal/awg/`, `internal/lucx/`, LucX-страницы frontend) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| Плагин `forward_proxy` ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy / `bin/naive-client-*` ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/csqtt-*` ([amurcanov/csqtt](https://github.com/amurcanov/csqtt)) | **PolyForm Noncommercial 1.0.0** (amurcanov; грант LucX не покрывает) |
| `bin/mieru-*` (`mita`, [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| `bin/anytls-*` ([anytls/anytls-go](https://github.com/anytls/anytls-go)) | Явной лицензии в апстриме нет (см. LICENSING.md) |
| AmneziaWG kernel module & tools ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (модуль; ставится на хост) |
| Готовые geo `.dat` (Loyalsoldier / IR / RU / ROSCOM) | Условия каждого датасета (см. LICENSING.md) |

Туннельные бинарники — **дочерние процессы**, панель не линкует их статически. GPL у qWDTT относится к этому бинарнику и его исходникам, а не к PolyForm-коду LucX. CSQTT — чужой PolyForm NC: редистрибуция с их LICENSE, коммерческое разрешение LucX его не покрывает.

</details>

---

## 🤝 Благодарности и источники

LucX-UI построен на множестве open-source проектов и трудах многих людей. Спасибо всем.

<details>
<summary><b>Тестировщики и контрибьюторы</b></summary>

- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13) — AWG `routeThroughXray`), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, команда **[3x-ui](https://github.com/MHSanaei/3x-ui)** ([MHSanaei](https://github.com/MHSanaei) и контрибьюторы).

</details>

<details>
<summary><b>Финансовая поддержка</b></summary>

- **Игорь**, **пётр смолин**, **Камслат Глорихо**, **Михаил Ляшенко**, **Aleksandr S.**, **Сила Растений**, **Виталий Зайцев**.

</details>

<details>
<summary><b>Upstream PR, которые мы портировали</b></summary>

- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) *feat(xray): browse geosite/geoip categories from routing rules* (geodata browser).

</details>

<details>
<summary><b>Проекты и вдохновение</b></summary>

| Проект | Что используем | Лицензия |
|---|---|---|
| [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) | Базовая панель | GPL-3.0 |
| [amnezia-vpn](https://github.com/amnezia-vpn) — kernel module & tools | Протокол AmneziaWG / AWG3 | GPL-2.0 (модуль) |
| [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) | Протокол / клиент-референс NaiveProxy | BSD-3-Clause |
| [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) + Caddy | Бинарник сайдкара NaiveProxy | MIT + Apache-2.0 |
| [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) | Ядро olcRTC | WTFPL |
| [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) | Сервер qWDTT | GPL-3.0 |
| [amurcanov/csqtt](https://github.com/amurcanov/csqtt) | Сервер CSQTT | PolyForm NC |
| [anton48/vk-turn-proxy-ios](https://github.com/anton48/vk-turn-proxy-ios) | iOS-клиент CSQTT | GPL-3.0 |
| [enfein/mieru](https://github.com/enfein/mieru) | Сервер mieru `mita` | GPL-3.0 |
| [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) | Эндпоинт TrustTunnel | Apache-2.0 |
| [anytls/anytls-go](https://github.com/anytls/anytls-go) | Сервер AnyTLS | Явной лицензии нет |
| [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) | Референс интеграции Caddyfile | — |
| [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) | Концепция туннельных сайдкаров в панели (qWDTT / olcRTC) | — |
| [hydraponique/3x-ui](https://github.com/hydraponique/3x-ui), [roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite), [roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing) | Пакет RoscomVPN geo + профили Happ | Upstream |
| [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat), [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules), [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) | Готовые geoip/geosite | Upstream |
| [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) | Вдохновение по AWG ops | — |
| [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls) | Референсы TLS-отпечатков для CPS | — |

</details>

---

## ☕ Поддержать проект

LucX-UI бесплатен для личного использования. **Понравилось — поставьте ⭐ репозиторию**: это помогает другим найти проект и поддерживает разработку. Донаты необязательны, но всегда приятны:

<details>
<summary><b>Донаты</b></summary>

| Способ | Реквизиты |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🟠 **Boosty** (подписка) | [boosty.to/alexeylcp](https://boosty.to/alexeylcp) |
| 🟠 **Boosty** (разово) | [boosty.to/alexeylcp/donate](https://boosty.to/alexeylcp/donate) |
| 🇷🇺 **YooMoney** (RUB, Россия) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

</details>

---

## 🛠️ Для разработчиков

<details>
<summary><b>Архитектура, сборка и upstream sync (нажмите, чтобы развернуть)</b></summary>

**Архитектура и правило изоляции.** Весь код LucX живёт в изолированных пакетах (`internal/awg/`, `internal/lucx/`); правки файлов upstream 3x-ui вносятся только внутри маркеров `// LUCX-HOOK` / `// END LUCX-HOOK`, поэтому каждый upstream-релиз сводится к почти тривиальному портированию. См. [AGENTS.md](AGENTS.md) — там полная карта архитектуры, правила проекта, известные проблемы и шаблоны отладки.

**Сборка из исходников** (требуется Go 1.27+, Node.js 24+, gcc с CGO для SQLite — полноценно собирается только на Linux; на Windows без gcc падает пакет CGO sqlite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# проверка перед push: bin/check-lucx.sh  (LUCX-HOOK + internal/awg|lucx)
```

**Процедура upstream sync** (актуальная база — апстрим **v3.8.5**; мержить теги/main апстрима, не старые v3.5→v3.6):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# разрешение конфликтов блок за блоком (см. AGENTS.md правило 8) — никогда целиком --ours/--theirs
git grep -c "LUCX-HOOK"  # сравнить количество маркеров до/после — так видно потерянные блоки
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

<!-- END LUCX-HOOK -->
