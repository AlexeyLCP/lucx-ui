# LucX-UI

Долгосрочно поддерживаемый форк [3x-ui](https://github.com/MHSanaei/3x-ui) с нативной интеграцией AmneziaWG, Telemt (MTProto), умным управлением кластером и пресетами обхода ТСПУ для России (май 2026).

**English:** [README.md](README.md)

## Установка

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install-lucx.sh)
```

Доступ: `http://<server_ip>:<port>/<basepath>` — реквизиты выводятся установщиком в конце.

## Добавленные протоколы

| Протокол | Транспорт | Обфускация | Путь трафика |
|----------|-----------|-----------|-------------|
| **AWG** (AmneziaWG) | UDP, модуль ядра | Jc/Jmin/Jmax, S1-S4, H1-H4, CPS I1-I5 | Клиент → awgN → Xray TUN → Маршрутизация → Outbound |
| **Telemt** (MTProto) | TCP 443/8443 | FakeTLS (секрет `ee`), SOCKS5 upstream через Xray | Клиент → Telemt → 127.0.0.1:SOCKS5 → Xray Маршрутизация |

Оба протокола создают скрытые дочерние Xray-инбаунды (TUN для AWG, SOCKS5 для Telemt). Учёт трафика — через нативный gRPC API Xray: дочерние инбаунды опрашиваются по тегу, суммируется общий трафик. Разбивка по пользователям делегирована нативным инструментам протоколов (`awg show`, Telemt REST API). Никаких самописных парсеров и bash-костылей.

## Кластер (Multi-Node)

- **Smart Import:** вставьте вывод установочного скрипта в форму добавления ноды — поля заполнятся автоматически
- **Детекция типа ноды:** каждая нода опрашивается через `GET /panel/api/lucx/hello` — в UI отображаются бейджи LucX или vanilla 3x-ui
- **Vanilla Guard:** протоколы AWG/Telemt заблокированы на уровне API для vanilla-нод 3x-ui
- **Inbound → Outbound:** копирование удалённого инбаунда как outbound-конфига в один клик

## Пресеты обхода ТСПУ (Россия, май 2026)

Все пресеты избегают доменов Cloudflare, Fastly и Akamai. Используется критическая инфраструктура РФ (`gosuslugi.ru`, `online.sberbank.ru`) и домены системных обновлений (`update.microsoft.com`, `releases.ubuntu.com`), блокировка которых невозможна без нарушения работы основных сервисов.

- **VLESS Reality:** Ghost Mode (gosuslugi.ru + randomized fingerprint), Best Speed (XHTTP + update.microsoft.com), RF Critical (Сбербанк), Stealth QUIC, Anti-DPI
- **Trojan Reality:** Ghost Mode, Best Speed (TLS 1.3), Stealth (WS)
- **Hysteria2:** Salamander + Masquerade + порт-хоппинг (1000 портов)
- **AWG:** Jumbo Random (Jc 3-10, Jmin 50-100, Jmax 150-250)
- **Telemt:** FakeTLS Neutral (ee + hex-кодирование домена)
- **Shadowsocks:** 2022 blake3-aes-128-gcm

Порт 443 мониторится ТСПУ — пресеты используют порты 47000+. Split Tunneling (`geosite:category-ru` → direct) настраивается автоматически при запуске панели.

## Telegram-бот

`/lang` — выбор языка (EN/RU/FA/ZH). Клиенты AWG получают `.conf` файлы через Telegram Document. Клиенты Telemt получают `tg://proxy` ссылки с inline-кнопкой «Подключиться». Языковые настройки сохраняются между перезапусками (`/etc/lucx-ui/lucx_tg_langs.json`).

## Структура проекта

```
internal/lucx/
├── parser/              Парсер SSH-вывода (smart import)
├── nodetype/            Детекция LucX vs vanilla
├── outbound_link/       Генератор inbound → outbound
├── awg/                 Параметры AWG, CPS, шаблоны, сервис
├── telemt/              Конфиг Telemt, менеджер процессов, сервис
├── telegram/            Помощники бота (языки, ссылки AWG/Telemt)
├── controller/          HTTP-обработчики API
├── integration/         Сквозные интеграционные тесты
└── stress_test.go       Хаос-инжиниринг и стресс-тесты

frontend/src/lucx/
├── presets.js           Пресеты обфускации для всех протоколов
├── PresetButtons.vue    Кнопки применения пресетов
├── AWGForm.vue          Форма создания AWG
├── TelemtForm.vue       Форма создания Telemt
├── SshParser.vue        Вставка и парсинг SSH-вывода
├── NodeBadge.vue        Бейдж LucX/Vanilla
├── OutboundLinkButton.vue  Кнопка inbound → outbound
├── awg-config-gen.js    Генератор .conf файлов AWG
└── client-generators.js Генератор ключей AWG/Telemt
```

## Тесты

```bash
# Модульные + интеграционные (требуется Go 1.24+)
cd frontend && npm install && npm run build && cd ..
go test ./internal/lucx/... ./internal/lucx/integration/... -v -count=1

# Хаос-инжиниринг (пропустить с -short для CI)
go test ./internal/lucx/ -v -run "Vector" -count=1

# Все тесты
go test ./internal/lucx/... ./internal/lucx/integration/... ./database/model/... -count=1
```

Категории тестов:
- **parser:** 7 тестов (SSH-вывод, ANSI-escape, граничные случаи)
- **awg:** 13 тестов (параметры, CPS, шаблоны, валидация конфига)
- **telemt:** 11 тестов (конфиг, секреты, TOML, proxy-ссылки, table-driven)
- **nodetype:** 3 теста (LucX, vanilla, таймаут)
- **outbound_link:** 4 теста (VLESS, отклонение AWG/Telemt)
- **telegram:** 9 тестов (конфиг AWG, ссылки Telemt, валидация)
- **integration:** 3 теста (жизненный цикл CRUD, учёт трафика, параллельные клиенты)
- **stress:** 6 тестов (конкурентность 5000 операций, фаззинг, утечки ресурсов, crash recovery)

## Архитектурные правила

Весь новый код — в `internal/lucx/` (Go) и `frontend/src/lucx/` (Vue). Изменения в оригинальных файлах 3x-ui обёрнуты в:

```
// LUCX-HOOK:
// ... вызов нового кода ...
// END LUCX-HOOK
```

Список всех точек интеграции: `grep -rn "LUCX-HOOK"`. Это позволяет безопасно забирать изменения из апстрима — конфликты ограничены маркированными блоками.

## Благодарности

- **3x-ui** — [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) (AGPL-3.0)
- **Логика обфускации AWG** — [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) (MIT)
- **AmneziaWG** — модуль ядра и userspace-утилиты (MIT)
- **Telemt** — MTProto-прокси на Rust, [telemt/telemt](https://github.com/telemt/telemt)
- **Xray-core** — [XTLS/Xray-core](https://github.com/XTLS/Xray-core) (MPL-2.0)
- **GeoIP/GeoSite** — [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)

## Лицензия

Компоненты LucX-UI (`internal/lucx/`, `frontend/src/lucx/`) распространяются под лицензией **PolyForm Noncommercial 1.0.0**. Свободно для личного и образовательного использования. Коммерческое использование — включая перепродажу VPN, платный прокси/VPN-хостинг, управляемые сервисы — требует явного письменного разрешения автора.

Оригинальный код 3x-ui остаётся под AGPL-3.0.

См. `LICENSE-LucX.md` для полных условий.
