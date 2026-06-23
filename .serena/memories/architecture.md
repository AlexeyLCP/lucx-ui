# Architecture Overview

## LUCX-HOOK Isolation Pattern
Весь новый код изолирован в:
- **Backend:** `internal/lucx/` (31 Go файл)
- **Frontend:** `frontend/src/lucx/` (9 Vue/JS файлов)

Оригинальные файлы 3x-ui получают минимальные изменения, обёрнутые в маркеры `LUCX-HOOK` / `END LUCX-HOOK`. Всего 152 маркера. Это обеспечивает безопасный upstream merge — конфликты возможны только в маркированных блоках.

## Backend Packages (`internal/lucx/`)
| Пакет | Назначение |
|-------|-----------|
| `parser` | Парсинг SSH-вывода для импорта нод |
| `nodetype` | Детекция LucX vs vanilla через /lucx/hello |
| `outbound_link` | Генератор inbound→outbound конфигураций |
| `awg` | AWG параметры, CPS I1-I5, шаблоны PostUp/PostDown, сервис |
| `telemt` | Telemt TOML конфиг, менеджер процессов, сервис |
| `telegram` | Telegram бот: персистентность языка, отправка .conf, tg://proxy |
| `controller` | HTTP handlers: /lucx/hello, /lucx/parse-ssh, /lucx/inbound-to-outbound, /lucx/awg/*, /lucx/telemt/* |
| `integration` | E2E lifecycle тесты на реальном SQLite |
| `stress_test.go` | Chaos engineering: 5000 concurrent ops, fuzzing, leak detection |

## Frontend Components (`frontend/src/lucx/`)
| Файл | Назначение |
|------|-----------|
| `presets.js` | 18 пресетов для 6 протоколов (без Cloudflare/Akamai/Fastly) |
| `PresetButtons.vue` | Кнопки пресетов в один клик |
| `AWGForm.vue` | Форма создания AWG (obfLevel, mimicry, region, DNS, MTU) |
| `TelemtForm.vue` | Форма Telemt + ручной ввод ee-secret с hex-валидацией |
| `SshParser.vue` | Текстареа SSH-вывода → авто-заполнение формы ноды |
| `NodeBadge.vue` | Бейдж LucX-UI / Vanilla 3x-ui |
| `OutboundLinkButton.vue` | Кнопка inbound → outbound |
| `awg-config-gen.js` | Генератор AWG .conf с параметрами обфускации |
| `client-generators.js` | Генераторы ключей AWG/Telemt (crypto.getRandomValues) |

## Key Design Decisions
- **Traffic Accounting:** AWG через Xray TUN (tag `awg-tun-{id}`), Telemt через SOCKS5 (tag `telemt-in-{id}`). Тотал через Xray gRPC API, per-user через нативные инструменты (awg show, Telemt REST)
- **QR отключён для AWG:** Слишком плотный конфиг → .conf download через QrCodeModal с `noQR: true`
- **Telemt secret:** Ручной ввод (ee + 32+ hex) с валидацией + кнопка генерации
- **Пустой SNI удалён:** Пустой SNI = мгновенный бан ТСПУ (легитимный TLS всегда имеет SNI)
- **Порты 47000+ для high-security пресетов:** 443 мониторится ТСПУ