# Базовый промт: написать плагин для LucX-UI

Готовый prompt-шаблон для нейросети (Devin, ChatGPT, Claude). Скопируй блок
ниже, допиши свою задачу в `<ЗАДАЧА ПЛАГИНА>` и вставь в ИИ. Полный контракт:
[plugin-sidecars.md](plugin-sidecars.md).

```text
Ты пишешь плагин для панели LucX-UI (форк 3x-ui) в формате sidecar.
Правила контракта плагина (apiVersion 1, панель форсит их при установке и
запуске), нарушать нельзя:

1. Доставка: один ZIP ≤ 200 МБ. В корне архива manifest.json с полями:
   id — латиница в нижнем регистре, цифры, "-", "_", до 64 символов, не с
   "-"/"_"; name — человекочитаемое имя; version — семвер; apiVersion: 1;
   type: "sidecar"; entrypoint — ОТНОСИТЕЛЬНЫЙ путь до исполняемого файла
   внутри архива, без ".." и не абсолютный; architectures: [] для скриптов
   или список GOARCH, который реально собран ("amd64", "arm64" и т.п.);
   permissions: {"network": [], "panelApi": [], "bot": [], "sub": []} —
   панель проверяет разрешения при установке, незнакомые значения
   отклоняются.
2. Плагин — отдельный процесс с рабочей папкой bin/plugins/<id>/. Базовые
   переменные окружения: LUCX_PLUGIN_ID, LUCX_PLUGIN_VERSION,
   LUCX_PLUGIN_DATA — приватная папка на запись: весь mutable state (конфиги,
   базы, кэш) держи только там. Ничего не пиши вне своей папки.
3. Разрешения проси только те, что плагину реально нужны, и учитывай их
   ограничения:
   - network: ["none"] — своя сетевая namespace без сети (Linux). Ставь по
     умолчанию, если плагину сеть не нужна. НЕ сочетается с seam'ами, которым
     нужна сеть (panelApi, bot, sub), и с health-пробой — панель отклонит.
   - panelApi: ["ресурс:read|write|*"] — панель выдаёт в env
     LUCX_PANEL_TOKEN (bearer) и LUCX_PANEL_URL (loopback-URL панели, обычно
     self-signed TLS — отключи проверку сертификата для этого соединения).
     Ресурсы: clients, inbounds, hosts, server, awg-outbounds,
     sidecar-outbounds, tunnel. Запрос: GET/HEAD = read, остальное = write,
     вызов LUCX_PANEL_URL + panel/api/... с Authorization: Bearer. Служебные
     маршруты (настройки, xray, обновление панели и т.п.) плагинам закрыты
     всегда — не пытайся их вызывать.
   - bot: ["telegram" | "discord"] — панель отдаёт токен бота в env
     LUCX_TELEGRAM_BOT_TOKEN / LUCX_DISCORD_BOT_TOKEN (только если бот
     настроен в панели). Токен не логируй.
   - sub: ["after"] — панель присылает POST /_lucx/sub/after на unix-сокет
     плагина с {"subId","host","count","emails"}, плагин отвечает
     {"links": ["vless://..."]}: ссылки будут дописаны ПОСЛЕ клиентских
     записей в сыром теле /sub; менять/удалять уже выданные ссылки нельзя.
4. События: если в manifest дополнительно объявить "events" (до 16 имён из
   словаря панели: outbound.down, outbound.up, xray.crash, node.down,
   node.up, cpu.high, memory.high, login.attempt), панель даёт в env
   LUCX_PLUGIN_SOCKET путь unix-сокета (плагин сам unlink/bind/chmod 0600)
   и шлёт на него POST /_lucx/events c {"id","type","source","time","data"}
   для каждого подписанного события. Отвечай любым 2xx быстро: у панели 5
   секунд, повторов нет — это best effort доставка.
5. Лимиты и пробы (запрашивай по необходимости):
   resources.maxMemoryMB (16..131072) — лимит виртуальной памяти;
   resources.maxCPUPercent (1..100) — CPU-квота, доступна только на хосте
   с systemd (без него манифест отклоняется на старте);
   health {type: "http"|"tcp", target, intervalSeconds, timeoutSeconds} —
   проба только репортится в статус плагина, рестарты сама панель не делает.
   Плагин со включённым форсингом запускается как transient systemd-юнит
   (lucx-plugin-<id>: PrivateNetwork, MemoryMax/CPUQuota,
   ProtectSystem=strict, без capabilities); юнит переживает рестарт панели;
   на хосте без systemd манифест с форсингом отклоняется на старте (fail
   closed).
6. Логи — только stdout/stderr, панель пишет их в plugin.log (один файл на
   все запуски). Пиши осмысленные строки с таймстампами: старт, конфиг,
   ошибки, стоп. Секреты и длинные дампы в лог не сливай.
7. Жизненный цикл: при Stop панель шлёт SIGINT и через 3 секунды SIGKILL.
   Обработай SIGINT/SIGTERM и завершайся чисто (закрой сокеты и файлы).
   Авто-рестарта нет: упал — лежит до ручного запуска из панели. Значит,
   процесс должен сам жить постоянно (сервисный цикл), обработчики сокета и
   HTTP должны быть быстрыми и не падать на пустых/чужих входных данных.

ЗАДАЧА ПЛАГИНА:
<ЗАДАЧА ПЛАГИНА: что делает, что слушает, какие порты, какие разрешения
просит, что хранит в LUCX_PLUGIN_DATA>

Выдай результат ровно в таком составе:
(1) manifest.json — готовый файл;
(2) код плагина: shell-скрипт со строкой #! ИЛИ статичный Go-бинарь
    (сборка: CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build -trimpath
    -ldflags="-s -w") + при необходимости go.mod;
(3) команда сборки ZIP: zip -r <id>.zip manifest.json <остальные файлы>;
(4) локальная проверка: как запустить вручную с выставленными
    LUCX_PLUGIN_* / LUCX_PANEL_* , какая строка ожидается в логе на старте,
    как проверить чистое завершение по SIGINT (kill -INT <pid>), и если
    плагин объявляет events/sub — как подать POST на сокет и что должно
    вернуться.
Никаких пояснений сверх этого списка — только артефакты и шаги проверки.
```

Мини-плагин для Smoke-теста контракта (shell, без тулчейна) — в
[plugin-sidecars.md](plugin-sidecars.md#minimal-plugin-example-no-toolchain).

## Промт 2: ревью готового плагина

```text
Review this plugin against the LucX-UI sidecar contract (apiVersion 1) and
list violations or risks:
1. manifest.json validity: id regex, apiVersion=1, type=sidecar, entrypoint
   relative + no "..", architectures vs the binary build targets;
2. permissions vs needs: panelApi entries are "<resource>:<read|write|*>"
   with resources clients|inbounds|hosts|server|awg-outbounds|
   sidecar-outbounds|tunnel; bot only "telegram"|"discord"; sub only
   "after"; network only "none"; nothing requested that the plugin never
   uses; panelApi/bot/sub never combined with network: ["none"];
3. runtime contract: only LUCX_PLUGIN_DATA is writable; SIGINT/SIGTERM
   handled; endless-crash loops absent; stdout logging present; secrets
   never logged;
4. seams used properly: panel calls carry Bearer LUCX_PANEL_TOKEN against
   LUCX_PANEL_URL with TLS verification off on that loopback URL; the unix
   socket (events/sub) is unlinked/bound/chmod 0600 by the plugin, requests
   to /_lucx/events (5 s budget) and /_lucx/sub/after (300 ms budget) are
   answered with 2xx fast, and sub/after only appends links — never
   rewrites client entries;
5. safety: no hardcoded secrets left in code, no writes outside its own
   directory, no listening port collisions with the panel/Xray ports;
6. resources/health: maxMemoryMB/maxCPUPercent within 16..131072 / 1..100,
   health target reachable from the panel and not combined with netns.
Return a short verdict: shippable / needs fixes, with the exact fixes.
Plugin sources: <attach code or ZIP listing>
```
