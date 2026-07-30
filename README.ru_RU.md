<!-- LUCX-HOOK: LucX-UI fork README — RU lead section, license, credits, sources. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <b>Русский</b> |
  <a href="README.fa_IR.md">فارسی</a> |
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **Только для личного, некоммерческого, научного, исследовательского и образовательного использования.** Коммерческое использование — включая перепродажу VPN-доступа, платные панели и подписки, построенные на этом коде, — только с явного письменного разрешения автора. Не используйте в противоправных целях.

---

## О проекте LucX-UI

**LucX-UI** — форк [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) с нативной поддержкой **AmneziaWG (AWG)**. AWG работает как kernel-interface сайдкар — в точности по той же архитектуре, по которой в апстриме устроен MTProto (mtg): панель управляет жизненным циклом, учитывает трафик, а Xray при желании маршрутизирует.

### Что мы добавили — и что работает

- ✅ **AWG-инбаунды** — kernel-сайдкар на `awg-quick`: создание, reconcile каждые 10 секунд, подчистка осиротевших интерфейсов, DKMS-установщик модуля ядра.
- ✅ **AWG-аутбаунды (клиентский режим)** — панель сама подключается к upstream AmneziaWG-серверу: своя вкладка в разделе Xray, вставка готового `.conf`, kernel-интерфейс `awgo-{id}` под управлением reconcile-цикла. В конфиг Xray инжектится `freedom` outbound с `sockopt.interface`, поэтому routing-правила и балансировщики могут гнать трафик через upstream VPN.
- ✅ **Обфускация** — профили Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4) и CPS-мимикрия пакетов: TLS, DNS, SIP, QUIC.
- ✅ **TLS-отпечатки браузеров** — Chrome (GREASE), Firefox 120+ (NSS-порядок, padding), Safari 16+ (Apple-порядок, TLS 1.1). Для TLS и QUIC.
- ✅ **Захват сигнатуры с живого хоста** — реальное QUIC-рукопожатие с front-домена превращается в I1–I5.
- ✅ **Клиенты** — QR-коды, скачивание `.conf`, учёт трафика per-peer (`awg show transfer`).
- ✅ **Два режима маршрутизации:**
  - **Kernel NAT** — прямая маршрутизация ядра; NAT-правила самовосстанавливаются reconcile-циклом после flush iptables.
  - **«Маршрутизировать через Xray»** — трафик идёт через весь routing-pipeline Xray (доменные/geosite-правила, балансировщики, каскады-аутбаунды) через TUN-инбаунд с policy routing и sniffing'ом.
- ✅ **Диагностика из панели** — одна кнопка в форме инбаунда: интерфейс, ip_forward, пиры/рукопожатия, NAT/TUN-правила — сразу видно, где обрыв.
- ✅ **Проверено в бою** на VPS тестеров: handshake, ICMP, HTTPS, учёт трафика, каскады, оба режима маршрутизации.

### Установка

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Ставит панель из [последнего релиза](https://github.com/AlexeyLCP/lucx-ui/releases/latest), systemd-юнит, Xray-core и mtg (из апстрим-релиза 3x-ui) и собирает модуль ядра AmneziaWG через DKMS (`bin/install-awg-module.sh`).

### Лицензия

Проект использует **две лицензии** (подробности — [LICENSING.md](LICENSING.md)):

| Часть | Лицензия |
|---|---|
| Оригинальный код 3x-ui | **GPL-3.0** (как требует апстрим) |
| Компоненты LucX (`internal/awg/`, `internal/lucx/`, AWG-frontend, скрипты) | **PolyForm Noncommercial 1.0.0** |

Это значит: **свободно** для личного, некоммерческого, научного, исследовательского и образовательного использования — хоть десять панелей для себя и друзей. **Коммерческое использование** (перепродажа VPN, платные сервисы на этом коде, встраивание в коммерческий продукт) — только с письменного разрешения автора: [issues](https://github.com/AlexeyLCP/lucx-ui/issues) или владелец репозитория. Заголовки `SPDX-License-Identifier` в каждом файле делают границу однозначной: нет заголовка — это GPL-3.0.

### Благодарности

- **VladufQa** — тестирование на боевом VPS (ruvds): первые handshake'и, трафик, каскады, багрепорты по маршрутизации.
- **Kirill Rudenko** — тестирование (runode) и **PR #13**: needRestart для AWG, iif policy routing, per-inbound таблицы/gateway, reconcile-ensure маршрута, sniffing — то, что заставило «Маршрутизировать через Xray» реально работать.
- **302ba (Alex)** — **PR #24**: исправление потери полей клиента при парсинге Zod-схемы.
- Команде **3x-ui** — за отличную базу и архитектуру сайдкаров, которую мы зеркалим.

### Источники идей и кода

- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — база форка (GPL-3.0), архитектура MTProto-сайдкара как эталон.
- [AmneziaVPN](https://github.com/amnezia-vpn) — сам протокол AmneziaWG и kernel-модуль.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — паттерн PostUp NAT (MASQUERADE + FORWARD), генераторы QUIC Initial без криптобиблиотек, подход к DKMS-установке.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — порт захвата QUIC-сигнатуры (`internal/awg/signature/`), предупреждение о TLS-несовместимости.
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) и [refraction-networking/utls](https://github.com/refraction-networking/utls) — репрезентативные TLS-профили Firefox/Safari для наших ClientHello-пресетов.

### ☕ Поддержать проект

LucX-UI бесплатен для личного и некоммерческого использования. Если панель экономит вам время — можно поддержать разработку:

| Способ | Реквизиты |
|---|---|
| 🇷🇺 **ЮMoney** (рубли, РФ) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Донаты — это благодарность, а не оплата: они не дают коммерческой лицензии и не отменяют условия [LICENSING.md](LICENSING.md).

---

*Ниже представлена документация оригинального **3x-ui** на русском языке.*

<!-- END LUCX-HOOK -->

[English](README.md) | [فارسی](README.fa_IR.md) | [العربية](README.ar_EG.md) | [中文](README.zh_CN.md) | [Español](README.es_ES.md) | [Русский](README.ru_RU.md) | [Türkçe](README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
</p>

**3X-UI** — продвинутая веб-панель управления с открытым исходным кодом для управления серверами [Xray-core](https://github.com/XTLS/Xray-core). Она предоставляет аккуратный многоязычный интерфейс для развёртывания, настройки и мониторинга широкого спектра протоколов прокси и VPN — от одного VPS до развёртываний с несколькими узлами.

Созданный как улучшенный форк оригинального проекта X-UI, 3X-UI добавляет более широкую поддержку протоколов, повышенную стабильность, учёт трафика по каждому клиенту и множество функций для удобства использования.

> [!IMPORTANT]
> Этот проект предназначен только для личного использования. Пожалуйста, не используйте его в незаконных целях или в производственной среде.

## Возможности

- **Многопротокольные входящие подключения** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel и TUN.
- **Современные транспорты и безопасность** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade и XHTTP, защищённые с помощью TLS, XTLS и REALITY.
- **Fallback** — обслуживание нескольких протоколов на одном порту (например, VLESS и Trojan на 443) с помощью функции fallback в Xray.
- **Управление по каждому клиенту** — квоты трафика, даты истечения, лимиты IP, статус «онлайн» в реальном времени, а также ссылки для общего доступа, QR-коды и подписки в один клик.
- **Статистика трафика** — по каждому входящему, по каждому клиенту и по каждому исходящему, с возможностью сброса.
- **Поддержка нескольких узлов** — управление и масштабирование на несколько серверов из одной панели.
- **Исходящие подключения и маршрутизация** — WARP, NordVPN, пользовательские правила маршрутизации, балансировщики нагрузки и цепочки исходящих прокси.
- **Встроенный сервер подписок** с несколькими форматами вывода и [пользовательскими шаблонами страниц](docs/custom-subscription-templates.md).
- **Telegram-бот** для удалённого мониторинга и управления.
- **RESTful API** с документацией Swagger внутри панели.
- **Гибкое хранилище** — SQLite (по умолчанию) или PostgreSQL.
- **13 языков интерфейса** с тёмной и светлой темами.
- **Интеграция с Fail2ban** для применения лимитов IP по каждому клиенту.

## Скриншоты

<details>
<summary>Нажмите, чтобы развернуть</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Быстрый старт

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

Чтобы установить конкретную версию, добавьте её тег (например, `v3.4.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

Чтобы установить скользящую **dev**-сборку (новейший предварительный релиз по каждому коммиту из ветки `main`, а не стабильный релиз), передайте `dev-latest`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

Во время установки генерируются случайные имя пользователя, пароль и путь доступа. После установки выполните `x-ui`, чтобы открыть меню управления, где можно запускать/останавливать сервис, просматривать или сбрасывать учётные данные для входа, управлять SSL-сертификатами и многое другое.

Полную документацию смотрите в [вики проекта](https://github.com/MHSanaei/3x-ui/wiki).

### Автоматическая установка

Установщик также работает в **неинтерактивном** режиме для cloud-init.
Задайте `XUI_NONINTERACTIVE=1` (или передайте по конвейеру без TTY), и установка пройдёт от начала до конца
без единого запроса: будут сгенерированы случайные учётные данные и записаны в
`/etc/x-ui/install-result.env`. Смотрите [`deploy/`](deploy/) для:

- [Cloud-init user-data](deploy/cloud-init/) — автоматическая установка в любом облаке (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Заметки по Hetzner Cloud](deploy/marketplace/hetzner/) — развёртывание на Hetzner на базе cloud-init

## Поддерживаемые платформы

**Операционные системы:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine и Windows.

**Архитектуры:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Варианты базы данных

3X-UI поддерживает два бэкенда, выбираемых при установке:

- **SQLite** (по умолчанию) — единый файл по пути `/etc/x-ui/x-ui.db`. Без настройки, идеально для небольших и средних развёртываний.
- **PostgreSQL** — рекомендуется при большом числе клиентов или конфигурациях с несколькими узлами. Установщик может установить PostgreSQL локально за вас или принять DSN к существующему серверу.

Во время выполнения бэкенд выбирается через переменные окружения (установщик записывает их за вас в `/etc/default/x-ui`):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Перенос существующей установки SQLite в PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# затем задайте XUI_DB_TYPE и XUI_DB_DSN в /etc/default/x-ui и перезапустите:
systemctl restart x-ui
```

Исходный файл SQLite остаётся нетронутым; удалите его вручную после проверки нового бэкенда.

### Docker

Команда по умолчанию `docker compose up -d` продолжает использовать SQLite. Чтобы запустить со встроенным сервисом PostgreSQL, раскомментируйте две строки переменных окружения `XUI_DB_*` в `docker-compose.yml` и запустите с профилем:

```bash
docker compose --profile postgres up -d
```

Образ включает Fail2ban (включён по умолчанию) для применения **лимитов IP** по каждому клиенту. Fail2ban блокирует нарушителей с помощью `iptables`, что требует возможности `NET_ADMIN`. `docker-compose.yml` уже предоставляет её через `cap_add`; если вы вместо этого запускаете контейнер через `docker run`, добавьте возможности самостоятельно, иначе блокировки будут регистрироваться, но никогда не применяться:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд базы данных: `sqlite` или `postgres` | `sqlite` |
| `XUI_DB_DSN` | Строка подключения PostgreSQL (когда `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Каталог для файла базы данных SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Максимум открытых соединений (пул PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Максимум простаивающих соединений (пул PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | Начальный URI-путь для веб-панели | `/` |
| `XUI_ENABLE_FAIL2BAN` | Включить применение лимитов IP на основе Fail2ban | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Включить режим отладки | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Включить монитор состояния туннеля (опрашивает URL и перезапускает xray после многократных сбоев; перезапуск отключает всех клиентов) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Прокси, через который отправляется проба; укажите локальный входящий xray, чтобы проба проверяла туннель (например, `socks5://127.0.0.1:1080`). Пустое значение означает, что проба проверяет только связь с хостом | — |
| `XUI_TUNNEL_HEALTH_URL` | URL, опрашиваемый для проверки состояния туннеля | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Интервал между пробами | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Таймаут на одну пробу | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Число последовательных сбоев до запуска перезапуска | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Минимальная задержка между последовательными перезапусками | `5m` |

## Поддерживаемые языки

Интерфейс панели доступен на 13 языках:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Участие в разработке

Вклад приветствуется. Пожалуйста, прочитайте [руководство по участию](/CONTRIBUTING.md), прежде чем открывать issue или pull request.

## Особая благодарность

- [alireza0](https://github.com/alireza0/)

## Благодарности

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Лицензия: **GPL-3.0**): _Улучшенные правила маршрутизации для v2ray/xray и v2ray/xray-clients со встроенными иранскими доменами и фокусом на безопасность и блокировку рекламы._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Лицензия: **GPL-3.0**): _Этот репозиторий содержит автоматически обновляемые правила маршрутизации V2Ray на основе данных о заблокированных доменах и адресах в России._

## Инструменты сообщества

Инструменты и интеграции, созданные сообществом вокруг 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Лицензия: **MIT**): _Управление входящими, клиентами, настройками панели и конфигурацией Xray через код с помощью Terraform / OpenTofu._

## ☕ Поддержать проект

LucX-UI бесплатен для личного и некоммерческого использования. Если панель экономит вам время — можно поддержать разработку:

| Способ | Реквизиты |
|---|---|
| 🇷🇺 **ЮMoney** (рубли, РФ) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Донаты — это благодарность, а не оплата: они не дают коммерческой лицензии и не отменяют условия [LICENSING.md](LICENSING.md).

## Звезды с течением времени

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
