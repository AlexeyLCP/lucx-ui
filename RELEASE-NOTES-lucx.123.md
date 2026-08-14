## v3.6.0-lucx.123 — аудит: сироты сайдкаров, безопасность загрузки ядер, отзывчивость UI

Релиз без новых функций: сплошное ревью всего, что LucX добавил поверх апстрима
(323 файла, +57.7k строк), и починка найденного.

**Что чинит на живой панели**

- **Туннельные сайдкары больше не остаются висеть после жёсткого падения панели.**
  Раньше, если панель убивал OOM-killer, `kill -9` или паника, процессы
  caddy/mita/qwdtt/trusttunnel выживали и продолжали держать порты своих
  инбаундов — следующий старт не мог забиндиться, и инбаунд поднимался мёртвым.
  Теперь при старте панель подчищает таких сирот (как уже давно делает для mtg).
- **Страница Tunnels и статусы инбаундов больше не подвисают.** Реконсайл держал
  один общий замок на всё время остановки и перезапуска ядра — при нескольких
  туннельных инбаундах это давало по 7 секунд заморозки на каждый. Теперь замок
  на каждое ядро свой.
- **Загрузка ядра по ссылке стала безопаснее.** Только `https`, только на
  публичные адреса (панель нельзя увести на localhost, локальную сеть или
  метадата-сервис облака), редиректы ограничены и перепроверяются. Появилось
  необязательное поле **SHA-256**: если указать, загрузка с несовпавшим хешем
  отбрасывается и рабочий бинарник остаётся на месте. Хеш скачанного файла в
  любом случае пишется в лог — можно сверить с чексуммами релиза.
- **Логи сайдкара больше не могут съесть память.** Ядро, которое пишет без
  переводов строки (прогресс-бар, однострочный JSON, мусор от битой загрузки),
  раньше раздувало буфер без ограничений.
- **Пересборка AWG-модуля показывает прогресс.** Вывод DKMS (до 45 минут) идёт в
  лог построчно, а не одним куском в самом конце.
- Конфиг mtg с секретами клиентов теперь пишется с правами `0600` вместо `0640`.

**Обновление:** `x-ui update` или кнопка в панели. Миграций данных нет, конфиги и
клиенты не трогаются.

---

## v3.6.0-lucx.123 — audit: sidecar orphans, safer core downloads, snappier UI

No new features: a full review of everything LucX adds on top of upstream
(323 files, +57.7k lines) and fixes for what it turned up.

**What this fixes on a running panel**

- **Tunnel sidecars no longer survive an ungraceful panel exit.** If the panel
  was killed by the OOM killer, `kill -9` or a panic, caddy/mita/qwdtt/trusttunnel
  kept running and kept holding their inbound's port, so the next start could not
  bind and the inbound came up dead. The panel now sweeps such orphans at
  startup, the same way it already did for mtg.
- **The Tunnels page and inbound statuses no longer freeze.** Reconcile held one
  global lock across a core's stop and restart — roughly 7 seconds of frozen UI
  per tunnel inbound. Locking is now per core.
- **Downloading a core binary by URL is constrained.** https only, public hosts
  only (the panel cannot be walked onto localhost, your LAN or a cloud metadata
  endpoint), redirects bounded and re-checked. A new optional **SHA-256** field
  discards a mismatched download instead of replacing a working binary; the
  digest is logged either way so you can compare against the release checksums.
- **Sidecar logs can no longer eat memory** when a core writes without newlines.
- **The AWG module rebuild streams its progress** to the panel log instead of
  showing 45 minutes of DKMS output only at the end.
- The mtg config holding client secrets is now written `0600`, not `0640`.

**Upgrade:** `x-ui update` or the button in the panel. No data migrations; your
configs and clients are untouched.
