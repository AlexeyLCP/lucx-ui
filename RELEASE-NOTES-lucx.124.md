## ⚡ v3.6.0-lucx.124 — AWG: скорость по умолчанию, BBR-тумблер, проверка MTU

Небольшой релиз по мотивам разбора лучших практик AmneziaWG: несколько
настроек по умолчанию были консервативнее, чем нужно на обычном VPS, плюс два
новых инструмента для диагностики скорости.

**Что изменилось**

- **MTU по умолчанию: 1320 → 1420** для новых AWG-инбаундов и аутбаундов.
  1420 = 1500 (типичный Ethernet) минус собственные накладные расходы
  WireGuard/AWG — оптимальное по скорости значение на обычном VPS. Старое
  значение 1320 никуда не делось: это теперь явно описанный «безопасный»
  вариант для мобильных сетей, CGNAT, PPPoE и серверов за ещё одним туннелем —
  подсказка под полем MTU объясняет, когда его выбирать.
- **Профили обфускации (Lite/Standard/Pro) теперь с пояснением скорости.**
  Подсказка в форме прямо говорит: `Jc` в основном влияет на скорость
  хендшейка и переподключения (мусорные пакеты шлются перед хендшейком), а не
  на скорость самой передачи данных — она примерно одинакова во всех трёх
  профилях.
- **Кнопка «Проверить MTU»** рядом с полем MTU в форме AWG-инбаунда.
  Бинарным поиском по DF-пингам находит реальный потолок MTU исходящего канала
  сервера и сравнивает с настроенным значением (с учётом ~80 байт накладных
  расходов WireGuard/AWG) — видно потенциальную фрагментацию до того, как это
  превратится в тикет в поддержку.
- **Тумблер TCP BBR** в Settings → Cores. Управление перегрузкой BBR (в паре с
  `fq`) ускоряет TCP-потоки, которые проксирует панель — исходящие Xray,
  трафик из AWG-туннеля — на путях с задержкой или потерями. Использует тот же
  `/etc/sysctl.d/99-bbr-x-ui.conf`, что и старая команда `x-ui bbr` в
  CLI-меню, так что оба способа управления не конфликтуют.

**Обновление:** `x-ui update` или кнопка в панели. Миграций данных нет,
существующие инбаунды/аутбаунды сохраняют свой текущий MTU — новые дефолты
касаются только новых.

⚡️ Приятного использования!

---

## ⚡ v3.6.0-lucx.124 — AWG: faster defaults, a BBR toggle, MTU testing

A small release following a pass over AmneziaWG best practices: a few defaults
were more conservative than needed on a normal VPS, plus two new tools for
diagnosing speed.

**What changed**

- **Default MTU: 1320 → 1420** for new AWG inbounds and outbounds. 1420 =
  1500 (typical Ethernet) minus WireGuard/AWG's own overhead — the
  throughput-optimal value on a normal VPS. The old 1320 default is still
  there as an explicitly documented "safe" fallback for mobile networks,
  CGNAT, PPPoE, and servers reached over another tunnel — a hint under the MTU
  field explains when to pick it.
- **Obfuscation profiles (Lite/Standard/Pro) now explain the speed trade-off.**
  The form hint spells out that `Jc` mainly affects handshake/reconnect speed
  (junk packets sent before the handshake), not steady-state throughput — that
  stays about the same across all three profiles.
- **A "Test MTU" button** next to the MTU field in the AWG inbound form.
  Binary-searches DF pings to find the server's actual outbound path-MTU
  ceiling and compares it against the configured value (accounting for
  ~80 bytes of WireGuard/AWG overhead) — surfaces potential fragmentation
  before it turns into a support ticket.
- **A TCP BBR toggle** in Settings → Cores. BBR congestion control (paired
  with `fq`) speeds up TCP flows the panel proxies — Xray outbounds, traffic
  relayed out of an AWG tunnel — on higher-latency or lossy paths. Shares the
  same `/etc/sysctl.d/99-bbr-x-ui.conf` as the legacy `x-ui` CLI's BBR menu
  entry, so the two controls never fight each other.

**Upgrade:** `x-ui update` or the button in the panel. No data migrations;
existing inbounds/outbounds keep their current MTU — the new defaults only
apply to new ones.

⚡️ Enjoy!
