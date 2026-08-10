<!-- LUCX-HOOK: LucX-UI fork README — Streamlined RU README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Продвинутая панель управления Xray и AmneziaWG** — с едиными подписками, управлением несколькими серверами и нативной поддержкой AWG.

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
> **Только для личного, некоммерческого, научного, исследовательского и образовательного использования.** Коммерческое использование — включая перепродажу VPN или платные панели — требует явного письменного разрешения по лицензии PolyForm Noncommercial 1.0.0.

---

## ⚡ Быстрый старт

Установка в одну строку на **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch и др.)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ Дополнительные варианты установки (Cloud-Init, Docker, PostgreSQL, Env Vars)</b></summary>

### Автоматическая установка (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
Учётные данные сохраняются в `/etc/x-ui/install-result.env`.

### Docker с PostgreSQL
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

[3x-ui](https://github.com/MHSanaei/3x-ui) — отличная мультипротокольная панель с современным React 19 + Ant Design 6 фронтендом. LucX-UI сохраняет всё, что есть у 3x-ui, и добавляет **нативный AmneziaWG (AWG)** — устойчивый к блокировкам форк WireGuard, — которого у 3x-ui нет:

| Возможность | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (kernel sidecar через `awg-quick`) | ✗ | ✓ |
| AWG CPS обфускация (TLS / DNS / SIP / QUIC + отпечатки браузеров) | ✗ | ✓ |
| AWG outbound — VPN chaining к upstream AWG-серверам (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| Пресеты версий клиентских конфигов (1.5 / 2 / 3) | ✗ | ✓ |
| Диагностика AWG из панели (routing / NAT / peers / handshakes) | ✗ | ✓ |
| Туннельный сайдкар NaiveProxy (Caddy + forward_proxy, под надзором панели) | ✗ | ✓ |
| Per-client креды NaiveProxy + `naive+https://` в подписках | ✗ | ✓ |
| Smart Cluster outbound-связи | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 фронтенд | ✓ | ✓ (inherited) |
| Все протоколы Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Бесшовный upstream sync (изоляция LUCX-HOOK, 49 файлов) | — | ✓ |

Kernel sidecar (как у MTProto `mtg` в 3x-ui) означает, что AWG работает как настоящий интерфейс ядра — а не как userspace-обёртка — поэтому Xray маршрутизирует расшифрованный трафик через собственный TUN inbound, давая вам полную мощь маршрутизации, sniffing'а и доменных правил Xray на AWG-трафике.

---

## 🌟 О проекте LucX-UI

**LucX-UI** — это расширенный форк [3x-ui](https://github.com/MHSanaei/3x-ui) (на данный момент синхронизирован с upstream **v3.6.0**), добавляющий нативную поддержку **AmneziaWG (AWG)** в виде kernel-interface sidecar, зеркалируя архитектуру MTProto у upstream. Панель сохраняет 100% совместимость с upstream за счёт строгой изоляции кода через `LUCX-HOOK`.

### 🛡️ Возможности AmneziaWG (AWG)
- **AWG Inbounds & Outbounds** — kernel sidecar (`awg-quick`), клиентский режим dial-out к upstream AWG-серверам (`awgo-{id}`), цикл автоматического reconcile каждые 10 секунд и сборщик DKMS kernel-модуля.
- **Продвинутая обфускация** — пресеты Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4), мимикрия CPS-пакетов (TLS, DNS, SIP, QUIC) и TLS-отпечатки браузеров (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — защита заголовков AmneziaWG 3 c автоматически генерируемыми 32-байтовыми ключами; серверный потолок версии управляет эмиссией фич на клиента.
- **Пресеты версий клиентов** — генерация клиентских конфигов для AWG 1.5 / 2 / 3 из одного inbound — выберите формат, который понимает ваше клиентское приложение.
- **Live Signature Capture** — преобразование реальных QUIC-handshake'ов с front-доменов в параметры обфускации I1–I5.
- **Маршрутизация и диагностика** — двойной режим маршрутизации (Kernel NAT и Route through Xray с policy routing и sniffing'ом) + однокликовая диагностика из панели.

### 🚇 Туннельные сайдкары (NaiveProxy)
- **NaiveProxy** — Caddy с плагином `forward_proxy` (форк [klzgrad](https://github.com/klzgrad/forwardproxy), HTTP/2 padding) работает как сайдкар под надзором панели: рендер Caddyfile, start/stop/restart с crash-revive reconcile и трёхуровневым health-probe (process → TCP → TLS).
- **Per-client креды** — каждый включённый клиент панели автоматически получает личную пару `basic_auth` (выводится из секрета панели, ничего не хранится); disable клиента отзывает креды на следующем reconcile.
- **Подписки** — в подписке каждого клиента его личная ссылка `naive+https://` рядом с Xray/AWG (стандарт NekoBox / husi / Exclave), плюс QR-код и генератор сильного пароля в панели.
- **UX панели** — Auto TLS (Let's Encrypt) или свой cert/key, raw-Caddyfile режим с валидацией `caddy adapt`, preview Caddyfile, логи процесса, upload/download бинарника.
- Трафик по дизайну идёт мимо Xray (собственный TLS + креды) — камуфляжное дополнение к AWG и REALITY, а не цель маршрутизации.

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

## 🔄 Переход с 3x-ui

LucX-UI использует ту же базу схемы Xray-core / SQLite (или PostgreSQL), что и 3x-ui, а AWG-таблицы создаются автоматически при первом запуске. Для установки поверх существующего 3x-ui сначала сделайте резервную копию базы, затем запустите стандартную команду установки:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

AWG kernel-модуль собирается автоматически установщиком (`bin/install-awg-module.sh`, DKMS). После установки запустите `x-ui` в консоли, чтобы подтвердить версию AWG kernel-модуля, и начните добавлять AWG inbounds из панели.

---

## 📜 Лицензия и условия

Проект публикуется под **двумя лицензиями** (подробности в [LICENSING.md](LICENSING.md)):

| Компонент | Лицензия |
|---|---|
| Исходный код оригинального 3x-ui | **GPL-3.0** |
| Компоненты LucX-UI (`internal/awg/`, `internal/lucx/`, frontend) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Благодарности и источники

- **Тестировщики и контрибьюторы:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, команда **3x-ui**.
- **Проекты и вдохновение:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) & [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) (туннельный сайдкар NaiveProxy), [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) (референс интеграции Caddyfile), [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) (референс концепции туннельных сайдкаров: qWDTT / olcRTC), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Поддержать проект

LucX-UI бесплатен для личного использования. Вы можете поддержать дальнейшую разработку:

| Способ | Реквизиты |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Россия) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ Для разработчиков

<details>
<summary><b>Архитектура, сборка и upstream sync (нажмите, чтобы развернуть)</b></summary>

**Архитектура и правило изоляции.** Весь код LucX живёт в изолированных пакетах (`internal/awg/`, `internal/lucx/); изменения файлов upstream 3x-ui вносятся только внутри маркеров `// LUCX-HOOK` / `// END LUCX-HOOK`, поэтому каждый upstream-релиз сводится к почти тривиальному портированию. См. [AGENTS.md](AGENTS.md) — полная карта архитектуры, 10 правил, известные проблемы и шаблоны отладки.

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

---

## ⭐ Динамика звёзд

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
