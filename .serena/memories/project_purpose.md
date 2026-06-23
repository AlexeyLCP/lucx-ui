# LucX-UI Project Purpose

LucX-UI — форк панели 3x-ui (MHSanaei/3x-ui) с расширенной поддержкой DPI-обхода.

**Ключевые дополнения:**
- **AWG (AmneziaWG)** — WireGuard с обфускацией (CPS I1-I5, обфусцирующие параметры Jc/Jmin/Jmax/S1-S4/H1-H4)
- **Telemt** — MTProto-прокси (Rust) с интеграцией через SOCKS5
- **DPI Presets** — предустановки для обхода ТСПУ (без Cloudflare/Akamai/Fastly — используются gosuslugi.ru, online.sberbank.ru, системные домены обновлений)
- **Smart SSH Import** — парсинг SSH-вывода для импорта нод
- **Node Type Detection** — определение LucX vs vanilla через `/lucx/hello`
- **Outbound Link** — генератор inbound→outbound конфигураций

**Репозиторий:** https://github.com/AlexeyLCP/lucx-ui (public)
**Ветка:** lucx-ui-phase1 → main
**Сервер:** 34.88.118.168:2053 (vps-finland-lucx, GCP)
**Лицензия LucX-компонентов:** PolyForm Noncommercial 1.0.0