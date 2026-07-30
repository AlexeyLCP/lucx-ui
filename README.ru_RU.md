<!-- LUCX-HOOK: LucX-UI fork README — Unified RU README. Keep in sync with LICENSING.md and AGENTS.md. -->
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

**LucX-UI** — продвинутая веб-панель управления серверами [Xray-core](https://github.com/XTLS/Xray-core), созданная как улучшенный форк [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) с нативной поддержкой **AmneziaWG (AWG)**. AWG работает как kernel-interface сайдкар — в точности по той же архитектуре, по которой в апстриме устроен MTProto (mtg): панель управляет жизненным циклом, учитывает трафик, а Xray при желании маршрутизирует.

### Ключевые возможности

#### 🛡️ Улучшения AmneziaWG (AWG)
- **AWG-инбаунды** — kernel-сайдкар на `awg-quick`: создание, reconcile каждые 10 секунд, подчистка осиротевших интерфейсов, DKMS-установщик модуля ядра.
- **AWG-аутбаунды (клиентский режим)** — панель сама подключается к upstream AmneziaWG-серверу: своя вкладка в разделе Xray, вставка готового `.conf`, kernel-интерфейс `awgo-{id}` под управлением reconcile-цикла. В конфиг Xray инжектится `freedom` outbound с `sockopt.interface`, поэтому routing-правила и балансировщики могут гнать трафик через upstream VPN.
- **Обфускация** — профили Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4) и CPS-мимикрия пакетов: TLS, DNS, SIP, QUIC.
- **TLS-отпечатки браузеров** — Chrome (GREASE), Firefox 120+ (NSS-порядок, padding), Safari 16+ (Apple-порядок, TLS 1.1). Для TLS и QUIC.
- **Захват сигнатуры с живого хоста** — реальное QUIC-рукопожатие с front-домена превращается в I1–I5.
- **Клиенты** — QR-коды, скачивание `.conf`, учёт трафика per-peer (`awg show transfer`).
- **Два режима маршрутизации**:
  - **Kernel NAT** — прямая маршрутизация ядра; NAT-правила самовосстанавливаются reconcile-циклом после flush iptables.
  - **«Маршрутизировать через Xray»** — трафик идёт через весь routing-pipeline Xray (доменные/geosite-правила, балансировщики, каскады-аутбаунды) через TUN-инбаунд с policy routing и sniffing'ом.
- **Диагностика из панели** — одна кнопка в форме инбаунда: интерфейс, ip_forward, пиры/рукопожатия, NAT/TUN-правила — сразу видно, где обрыв.

#### 🚀 Базовые возможности 3x-ui
- **Многопротокольные входящие** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door и TUN.
- **Современные транспорты и безопасность** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade и XHTTP, защищённые с помощью TLS, XTLS и REALITY.
- **Fallback** — обслуживание нескольких протоколов на одном порту (например, VLESS и Trojan на 443).
- **Управление клиентами** — квоты трафика, даты истечения, лимиты IP, статус «онлайн», ссылки, QR-коды и подписки.
- **Статистика трафика** — по входящим, клиентам и исходящим подключениям.
- **Поддержка нескольких узлов** — управление и масштабирование на несколько серверов из одной панели.
- **Исходящие подключения и маршрутизация** — WARP, NordVPN, пользовательские правила маршрутизации, балансировщики и цепочки прокси.
- **Встроенный сервер подписок** с поддержкой кастомных шаблонов.
- **Telegram-бот** для удаленного мониторинга.
- **RESTful API** с документацией Swagger.
- **Гибкое хранилище** — SQLite (по умолчанию) или PostgreSQL.
- **Fail2ban интеграция** для применения лимитов IP по каждому клиенту.

### Скриншоты

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
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Ставит панель из [последнего релиза](https://github.com/AlexeyLCP/lucx-ui/releases/latest), systemd-юнит, Xray-core и mtg (из апстрим-релиза 3x-ui) и собирает модуль ядра AmneziaWG через DKMS (`bin/install-awg-module.sh`).

Во время установки генерируются случайные имя пользователя, пароль и путь доступа. После установки выполните `x-ui`, чтобы открыть меню управления.

### Автоматическая установка

Установщик работает в **неинтерактивном** режиме для cloud-init. Задайте `XUI_NONINTERACTIVE=1`, и установка пройдет без единого запроса, сохранив данные в `/etc/x-ui/install-result.env`. Смотрите [`deploy/`](deploy/) для руководств по cloud-init.

## Поддерживаемые платформы

**Операционные системы:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine и Windows.

**Архитектуры:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Базы данных

3X-UI поддерживает два бэкенда:

- **SQLite** (по умолчанию) — файл `/etc/x-ui/x-ui.db`.
- **PostgreSQL** — рекомендуется для большого числа клиентов или распределенных узлов.

Переменные окружения в `/etc/default/x-ui`:
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Docker

Для работы с PostgreSQL в Docker раскомментируйте строки `XUI_DB_*` в `docker-compose.yml` и запустите:
```bash
docker compose --profile postgres up -d
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд БД: `sqlite` или `postgres` | `sqlite` |
| `XUI_DB_DSN` | Строка подключения PostgreSQL (при `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Каталог для файла БД SQLite | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | Включение Fail2ban для лимита IP | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Включение мониторинга здоровья туنнеля | `false` |

## Лицензии и условия

Проект распространяется под **двумя лицензиями** (подробности в [LICENSING.md](LICENSING.md)):

| Часть | Лицензия |
|---|---|
| Оригинальный код 3x-ui | **GPL-3.0** (как требует апстрим) |
| Компоненты LucX (`internal/awg/`, `internal/lucx/`, AWG-frontend, скрипты) | **PolyForm Noncommercial 1.0.0** |

**Свободно** для личного, некоммерческого, научного, исследовательского и образовательного использования. **Коммерческое использование** (перепродажа VPN, платные панели, встраивание в коммерческий продукт) — только с письменного разрешения автора: [issues](https://github.com/AlexeyLCP/lucx-ui/issues) или владелец репозитория. Заголовки `SPDX-License-Identifier` в каждом файле делают границу однозначной: нет заголовка — это GPL-3.0.

## Участие в разработке

Вклад приветствуется. Пожалуйста, прочитайте [руководство по участию](/CONTRIBUTING.md) перед созданием issue или pull request.

## Благодарности и источники

### Тестирование и контрибьюторы
- **VladufQa** — тестирование на боевом VPS (ruvds): первые handshake'и, трафик, каскады, багрепорты по маршрутизации.
- **Kirill Rudenko** — тестирование (runode) и **PR #13**: needRestart для AWG, iif policy routing, per-inbound таблицы/gateway, reconcile-ensure маршрута, sniffing.
- **302ba (Alex)** — **PR #24**: исправление потери полей клиента при парсинге Zod-схемы.
- **alireza0** — участник разработки апстрима.
- Команда **3x-ui** — за отличную базу и архитектуру сайдкаров, которую мы зеркалим.

### Источники идей и кода
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — база форка (GPL-3.0), архитектура MTProto-сайдкара.
- [AmneziaVPN](https://github.com/amnezia-vpn) — сам протокол AmneziaWG и kernel-модуль.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — паттерн PostUp NAT (MASQUERADE + FORWARD), генераторы QUIC Initial без криптобиблиотек, подход к DKMS-установке.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — порт захвата QUIC-сигнатуры (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) и [refraction-networking/utls](https://github.com/refraction-networking/utls) — репрезентативные TLS-профили Firefox/Safari для наших ClientHello-пресетов.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) и [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) — наборы правил маршрутизации.

### Инструменты сообщества
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Лицензия: **MIT**): Управление входящими, клиентами и настройками панели через код.

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

<!-- END LUCX-HOOK -->
