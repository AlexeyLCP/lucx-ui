<!-- LUCX-HOOK: LucX-UI fork README — Streamlined RU README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **Только для личного, некоммерческого, научного и образовательного использования.** Коммерческое использование (перепродажа VPN, платные панели) требует письменного разрешения автора под лицензией PolyForm Noncommercial 1.0.0.

---

## ⚡ Быстрый старт

Установка на **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch и др.)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ Дополнительные варианты установки (cloud-init, Docker, PostgreSQL, Env Vars)</b></summary>

### Автоматическая установка (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
Пароль и логин сохранятся в `/etc/x-ui/install-result.env`.

### Docker с PostgreSQL
```bash
docker compose --profile postgres up -d
```

### Основные переменные окружения (`/etc/default/x-ui`)
| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд БД (`sqlite` или `postgres`) | `sqlite` |
| `XUI_DB_DSN` | Подключение PostgreSQL | — |
| `XUI_ENABLE_FAIL2BAN` | Включение Fail2ban для лимита IP | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🌟 О проекте LucX-UI

**LucX-UI** — это современная панель управления веб-серверами [Xray-core](https://github.com/XTLS/Xray-core), созданная как улучшенный форк [3x-ui](https://github.com/MHSanaei/3x-ui) с нативной поддержкой **AmneziaWG (AWG)** в виде kernel-interface сайдкара.

### 🛡️ Возможности AmneziaWG (AWG)
- **AWG Inbounds & Outbounds** — сайдкар ядра (`awg-quick`), клиентский режим подключения к внешним AWG-серверам (`awgo-{id}`), автоматический reconcile каждые 10 секунд и DKMS-модуль ядра.
- **Продвинутая обфускация** — пресеты Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4), мимикрия CPS-пакетов (TLS, DNS, SIP, QUIC) и TLS-отпечатки браузеров (Chrome, Firefox, Safari).
- **Живая сигнатура (Live Capture)** — автоматический захват QUIC-сигнатуры с любого домена в параметры I1–I5.
- **Маршрутизация и диагностика** — два режима (Kernel NAT и Route through Xray с policy routing и sniffing'ом), встроенная однокликовая диагностика в панели.

### 🚀 Базовые фичи 3x-ui
- **Протоколы:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Безопасность и транспорты:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Управление:** Квоты трафика, IP-лимиты (Fail2ban), статус онлайн, подписки, Telegram-бот, REST API, мульти-узлы (Multi-node), SQLite / PostgreSQL.

<details>
<summary><b>📸 Скриншоты панели</b></summary>

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

## 📜 Лицензия

Проект распространяется под **двумя лицензиями** (см. [LICENSING.md](LICENSING.md)):

| Компонент | Лицензия |
|---|---|
| Исходный код 3x-ui | **GPL-3.0** |
| Модули LucX-UI (`internal/awg/`, `internal/lucx/`, frontend) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Благодарности и источники

- **Тестирование и код:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, команда **3x-ui**.
- **Проекты и источники:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Поддержать проект

LucX-UI бесплатен для личного использования. Поддержать разработку:

| Способ | Реквизиты |
|---|---|
| 🇷🇺 **ЮMoney** (рубли, РФ) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ Динамика звёзд

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
