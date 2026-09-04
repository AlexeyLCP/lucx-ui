<!-- LUCX-HOOK: LucX-UI fork README — Streamlined FA README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **پنل پیشرفته Xray** — AmneziaWG (کرنل + بومی، تا 3.1)، وارد کردن AWG موجود، تونل‌های تحت نظارت و sidecar outbounds (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy)، اشتراک Clash / Amnezia `vpn://` / Happ، RoscomVPN geo و مسیریابی Happ.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="../../LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
</p>

<p align="center">
  <a href="README.en_US.md">English</a> |
  <a href="../../README.md">Русский</a> |
  <b>فارسی</b> |
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **صرفاً برای استفاده شخصی، غیرتجاری، علمی، پژوهشی و آموزشی.** هرگونه استفاده تجاری — از جمله فروش مجدد VPN یا پنل‌های پولی — نیازمند کسب اجازه کتبی صریح تحت مجوز PolyForm Noncommercial 1.0.0 می‌باشد.

---

## ⚡ راه‌اندازی سریع

نصب تک‌خطی روی **لینوکس (Ubuntu / Debian / CentOS / AlmaLinux / Arch و غیره)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

نصب اختیاری از Yandex (SourceCraft) وقتی GitHub در دسترس نیست. بدون توکن و git — پنل، geo و اسکریپت‌ها در یک بسته:

```bash
mkdir -p /tmp/lucx-dist && curl -fsSL https://codeload.sourcecraft.tech/alexeylcp/lucx-ui/tarball/refs/heads/dist | tar -xz --strip-components=1 -C /tmp/lucx-dist && sudo bash /tmp/lucx-dist/install.sh --yandex
```

بعداً `x-ui update` از همان منبع استفاده می‌کند (`/etc/x-ui/install-source`).

<details>
<summary><b>🛠️ نصب پیشرفته و پیکربندی (Cloud-Init، Docker، PostgreSQL، متغیرهای محیطی)</b></summary>

### نصب خودکار (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
اطلاعات ورود در `/etc/x-ui/install-result.env` ذخیره می‌شود.

### داکر با PostgreSQL
```bash
docker compose --profile postgres up -d
```

### متغیرهای محیطی اصلی (`/etc/default/x-ui`)
| متغیر | توضیح | مقدار پیش‌فرض |
| --- | --- | --- |
| `XUI_DB_TYPE` | موتور دیتابیس (`sqlite` یا `postgres`) | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL DSN | — |
| `XUI_ENABLE_FAIL2BAN` | فعال‌سازی Fail2ban برای محدودیت IP | `true` |
| `XUI_LOG_LEVEL` | سطح لاگ‌ها (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ چرا LucX-UI؟

[3x-ui](https://github.com/MHSanaei/3x-ui) یک پنل چندپروتکله عالی با فرانت‌اند React 19 + Ant Design 6 است. LucX-UI همهٔ قابلیت‌های 3x-ui را نگه می‌دارد و آنچه upstream ندارد را اضافه می‌کند: **AmneziaWG کرنل** (در کنار `amneziawg` بومیِ upstream)، **وارد کردن AWG موجود**، **سایدی‌کارهای تونل** (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy)، **اشتراک‌های غنی‌تر** (Clash Meta AWG، Amnezia `vpn://`، Happ) و **بسته‌های RoscomVPN geo + پروفایل‌های Happ** (geodata browser از [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0 در upstream است):

<details>
<summary><b>مقایسه با 3x-ui</b></summary>

| ویژگی | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG ورودی (سایدی‌کار کرنل از طریق `awg-quick`) | ✗ | ✓ |
| ورودی AmneziaWG بومی (`amneziawg`، فضای کاربری) | ✓ | ✓ |
| وارد کردن AWG موجود روی میزبان (awg-multi / toolza3 / Docker) | ✗ | ✓ |
| Kernel AWG بدون ماژول → amneziawg-go توکار | ✗ | ✓ |
| سرعت زنده کلاینت/ورودی AWG در پنل | ✗ | ✓ |
| AWG CPS پنهان‌سازی (TLS / DNS / SIP / QUIC + اثرانگشت مرورگر) | ✗ | ✓ |
| AWG خروجی — زنجیره VPN به سرورهای AWG بالادستی (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1 (`RandomTrailers` / `DisableCookies`، ضد DPI) | ✗ | ✓ |
| پیش‌تنظیم‌های نسخه کانفیگ کلاینت (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| عیب‌یابی AWG درون پنل (مسیریابی / NAT / همتاها / دست‌دادن‌ها) | ✗ | ✓ |
| سایدی‌کار تونل NaiveProxy (Caddy + forward_proxy، تحت نظارت پنل) | ✗ | ✓ |
| اعتبارنامه‌های per-client NaiveProxy + `naive+https://` در اشتراک‌ها | ✗ | ✓ |
| NaiveProxy → مسیریابی Xray (پل SOCKS loopback، اختیاری) | ✗ | ✓ |
| سایدی‌کار olcRTC (WebRTC از طریق اتاق meet، تحت نظارت) | ✗ | ✓ |
| سایدی‌کار qWDTT (WireGuard روی VK TURN، تحت نظارت) | ✗ | ✓ |
| سایدی‌کار mieru (`mita`، ترافیک per-client، تحت نظارت) | ✗ | ✓ |
| سایدی‌کار TrustTunnel (پروتکل AdGuard VPN، شبیه HTTPS، تحت نظارت) | ✗ | ✓ |
| Sidecar outbounds (کلاینت Naive / mieru / TrustTunnel → SOCKS، مسیریابی و استخرها) | ✗ | ✓ |
| AWG در Clash Meta + اشتراک Amnezia `/awg/` (`.conf` / `vpn://`) | ✗ | ✓ |
| Geodata browser — انتخاب دسته‌های geosite/geoip از پنل | ✓ | ✓ |
| بسته geo RoscomVPN (`geoip/geosite_ROSCOM.dat`) | ✗ | ✓ |
| پروفایل‌های مسیریابی Happ (RoscomVPN deeplink + سفارشی) | ✗ | ✓ |
| لینک‌های خروجی Smart Cluster | ✗ | ✓ |
| فرانت‌اند React 19 + AntD 6 + Vite 8 + Zod 4 | ✓ | ✓ (به ارث رسیده) |
| تمام پروتکل‌های Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| ورودی پروکسی وب تلگرام (`tproxy`، t.me/webproxy) | ✗ | ✓ |
| همگام‌سازی بدون اصطکاک بالا‌دست (ایزوله‌سازی LUCX-HOOK) | — | ✓ |

</details>

یک سایدی‌کار کرنل (مانند `mtg` در MTProto ِ 3x-ui) به این معناست که AWG به‌عنوان یک رابط کرنل واقعی اجرا می‌شود — نه یک shim فضای کاربری — در نتیجه Xray ترافیک رمزگشاشده را از طریق TUN ورودیِ خود عبور می‌دهد و قدرت کامل مسیریابی، شنود و قوانین دامنهٔ Xray را روی ترافیک AWG در اختیار شما قرار می‌دهد. بدون ماژول، همان inboundِ LucXیِ `awg` روی amneziawg-go توکار بالا می‌آید. پروتکل بومیِ upstream یعنی `amneziawg` هم در پنل کنار آن می‌ماند.

---

## 🌟 درباره LucX-UI

**LucX-UI** فورک ارتقایافتهٔ [3x-ui](https://github.com/MHSanaei/3x-ui) است (همگام با upstream **v3.7.0**). فراتر از پروتکل‌های Xray استوک: **AmneziaWG** در دو حالت — سایدی‌کار کرنل `awg` (همان ایدهٔ MTProto/`mtg`) و `amneziawg` بومیِ upstream، تا **AWG 3.1**؛ **وارد کردن** awg-multi / toolza3 / Docker؛ **تونل‌های تحت نظارت پنل** (NaiveProxy، olcRTC، qWDTT، mieru، TrustTunnel)، **اشتراک‌های گسترش‌یافته** (Clash Meta AWG، Amnezia `/awg/` + `vpn://`، Happ)، **پروکسی وب تلگرام** (`tproxy`) و **geo استوک RoscomVPN** (مرورگر دسته‌ها مشترک با upstream v3.7.0). سازگاری ۱۰۰٪ با upstream از طریق ایزولهٔ `LUCX-HOOK`.

<details>
<summary><b>🛡️ ویژگی‌های AmneziaWG (AWG)</b></summary>

- **ورودی‌ها و خروجی‌های AWG** — سایدی‌کار کرنل (`awg-quick`)، شماره‌گیری حالت کلاینت به سرورهای AWG بالا‌دستی (`awgo-{id}`)، حلقه همگام‌سازی اتوماتیک ۱۰ ثانیه‌ای، و سازنده ماژول کرنل DKMS.
- **دو موتور** — هم `AmneziaWG (kernel)` (با ماژول از طریق `awg-quick`) و هم `amneziawg` بومیِ upstream. بدون ماژول، inboundهای LucXیِ `awg` روی amneziawg-go توکار (SOCKS به Xray) اجرا می‌شوند؛ مسیر کرنل وقتی ماژول هست عوض نمی‌شود.
- **وارد کردن AWG موجود** — بنر در Inbounds: awg-multi / toolza3 / Docker Amnezia. کلیدها، IP، پورت و پنهان‌سازی همان‌طور کپی می‌شوند؛ اینترفیس کرنل درجا عوض‌نام می‌شود (دست‌دادن قطع نمی‌شود).
- **سرعت زنده** — ستون سرعت در Clients / Inbounds برای AWG (statsِ Xray آن را نمی‌بیند).
- **مخفی‌سازی پیشرفته** — پریست‌های Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4)، شبیه‌سازی پکت‌های CPS (TLS, DNS, SIP, QUIC) و اثرانگشت TLS مرورگرها (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — حفاظت هدر AmneziaWG 3 با کلیدهای ۳۲ بایتیِ اتوماتیک؛ سقف نسخه سمت سرور، انتشار ویژگی به‌ازای کلاینت را کنترل می‌کند.
- **AWG 3.1** — `RandomTrailers` (دم بستهٔ تصادفی، ضد DPI بر اساس اندازه) و `DisableCookies`؛ ماژول کرنل و ابزارها هنگام به‌روزرسانی پنل به‌طور خودکار به v3.1 ارتقا می‌یابند.
- **پیش‌تنظیم‌های نسخه کلاینت** — تولید کانفیگ کلاینت برای AWG 1.5 / 2 / 3 / 3.1 از یک ورودی واحد — فرمتی که اپ کلاینت شما می‌فهمد را انتخاب کنید.
- **استخراج امضای زنده** — تبدیل دست‌دادن‌های واقعی QUIC از دامنه‌های فرانت به پارامترهای مخفی‌سازی I1–I5.
- **مسیریابی و عیب‌یابی** — دو حالت مسیریابی (Kernel NAT و Route through Xray با مسیریابی سیاست و شنود) + عیب‌یابی تک‌کلیکه درون پنل.

</details>

<details>
<summary><b>🚇 سایدی‌کارهای تونل (NaiveProxy، olcRTC، qWDTT، mieru، TrustTunnel، Telegram WEB proxy)</b></summary>

- **NaiveProxy** — Caddy با افزونهٔ `forward_proxy` (فورک [klzgrad](https://github.com/klzgrad/forwardproxy)، padding مربوط به HTTP/2) به‌عنوان سایدی‌کار تحت نظارت پنل اجرا می‌شود: Caddyfile رندر‌شده، start/stop/restart با reconcile احیای پس از کرش، و پروب سلامت سه‌سطحی (process → TCP → TLS).
- **اعتبارنامه‌های per-client** — هر کلاینت فعال پنل به‌طور اتوماتیک یک جفت شخصی `basic_auth` می‌گیرد (مشتق از راز پنل، بدون ذخیره‌سازی)؛ غیرفعال‌کردن کلاینت در reconcile بعدی آن را باطل می‌کند.
- **اشتراک‌ها** — اشتراک هر کلاینت لینک شخصی `naive+https://` او را در کنار لینک‌های Xray/AWG حمل می‌کند (فرمت استاندارد NekoBox / husi / Exclave)، به‌علاوه کد QR و تولیدکنندهٔ رمز عبور قوی در پنل.
- **UX پنل** — Auto TLS (Let's Encrypt) یا cert/key خودتان، حالت raw-Caddyfile با اعتبارسنجی `caddy adapt`، پیش‌نمایش Caddyfile، لاگ‌های فرایند، آپلود/دانلود باینری.
- **مسیریابی از طریق Xray (اختیاری)** — سوییچ باعث می‌شود Caddy مقاصد را از طریق پل SOCKS loopback پنهان شماره‌گیری کند (`upstream socks5://127.0.0.1:…`، forward_proxy بومی — بدون پچ) با برچسب `lucx-tunnel-naive`، تا ترافیک NaiveProxy مسیریابی / شنود / قوانین دامنهٔ کامل Xray را بگیرد (همان الگوی MTProto). پیش‌فرض همچنان egress مستقیم است.
- **olcRTC** — تونل TCP-over-WebRTC از طریق اتاق ویدئوکال قانونی ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)).
- **qWDTT** — WireGuard از طریق TURN ِ VK Calls ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)).
- **mieru** — پروکسی مقاوم در برابر سانسور روی پروتکلی اختصاصی به‌جای TLS ([enfein/mieru](https://github.com/enfein/mieru) `mita`، GPL-3.0). چندکاربره با اعتبارنامه‌های HMAC به‌ازای هر کلاینت پنل، ترافیک و وضعیت آنلاین per-client، و لینک اشتراک `mierus://`. کلاینت‌ها: mieru CLI، mihomo، Clash Verge Rev، husi، Exclave.
- **TrustTunnel** — پروتکل AdGuard VPN ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)، Apache-2.0): ترافیک از HTTPS قابل تشخیص نیست (HTTP/1.1 + HTTP/2 + QUIC). از گواهی ACME پنل استفاده مجدد می‌کند (نیاز به دامنه با گواهی صادرشده) و deep-link `tt://?` برای کلاینت‌های Flutter / CLI تولید می‌کند.
- **پروکسی وب تلگرام (`tproxy`)** — `tproxy-server` + MTProxy رسمی + Caddy TLS reverse_proxy روی `hostname:443`، لینک اشتراک `t.me/webproxy`. مسیریابی از طریق Xray فعلاً **متوقف** است (خروج مستقیم MTProxy؛ نک. lucx.211).
- **Sidecar outbounds** — حالت کلاینت Naive / mieru / TrustTunnel: لینک اشتراک را بچسبانید (`naive+https://` / `mierus://` / `tt://`)، تگ در قوانین مسیریابی و استخرهای balancer ظاهر می‌شود (مثل AWG outbound). غیرفعال = blackhole (fail-closed، به `direct` نشت نمی‌کند). باینری‌های کلاینت در tar.gz.

</details>

<details>
<summary><b>📦 اشتراک‌ها، geodata و مسیریابی کلاینت</b></summary>

- **اشتراک Amnezia** — `/awg/{subId}` فایل `.conf` خالص یا `vpn://…` برمی‌گرداند.
- **AWG در Clash Meta** — peerها با `amnezia-wg-option`.
- **Geodata browser** — مرور `geoip*.dat` / `geosite*.dat` از UI مسیریابی (در upstream از [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0، [STRENCH0](https://github.com/STRENCH0)).
- **بسته RoscomVPN geo** — `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip) / [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)).
- **پروفایل‌های Happ** — Settings → Happ: deeplink RoscomVPN ([roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

</details>

<details>
<summary><b>🚀 ویژگی‌های اصلی 3x-ui</b></summary>

- **پروتکل‌ها:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **انتقال و امنیت:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **مدیریت:** حجم ترافیک، محدودیت IP (Fail2ban)، وضعیت آنلاین، اشتراک‌ها، ربات تلگرام، REST API، چند نود، SQLite / PostgreSQL.

</details>

<details>
<summary><b>📸 تصاویر محیط پنل</b></summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../../media/01-overview-dark.png">
  <img alt="Overview" src="../../media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../../media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="../../media/02-add-inbound-light.png">
</picture>

</details>

---

## 🔄 مهاجرت از 3x-ui و AWG موجود

LucX-UI همان پایگاهِ اسکیمای دیتابیس Xray-core / SQLite (یا PostgreSQL) را با 3x-ui به اشتراک می‌گذارد و جداول AWG در اولین اجرا به‌صورت اتوماتیک ساخته می‌شوند. برای نصب روی یک راه‌اندازی موجود 3x-ui، ابتدا دیتابیس خود را پشتیبان بگیرید و سپس دستور نصب استاندارد را اجرا کنید:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

ماژول کرنل AWG توسط نصب‌کننده اتوماتیک ساخته می‌شود (`bin/install-awg-module.sh`, DKMS). پس از نصب، برای تأیید نسخهٔ ماژول کرنل AWG در کنسول `x-ui` را اجرا کنید و افزودن ورودی‌های AWG را از پنل آغاز کنید.

**پس از نصب:** نقاط پایانی اشتراک (`/sub/`، `/json/`، `/clash/`، `/awg/`) روی **پورتی جدا** (پیش‌فرض **2096**) گوش می‌دهند، نه پورت پنل — reverse proxy باید آن را هم فوروارد کند. گروه‌های geo سفارشی را در **نام فایل جدا** نگه دارید — نام‌های استوک (`geoip.dat` / `geosite.dat` و `_IR` / `_RU` / `_ROSCOM`) هنگام به‌روزرسانی geofile بازنویسی می‌شوند.

<details>
<summary><b>از AWG موجود روی میزبان</b></summary>

اگر روی سرور از قبل **awg-multi**، **toolza3** یا **Docker Amnezia** در حال اجراست، پنل اینترفیس‌های خارجی `awg0`/`awg1` را **حذف نمی‌کند**. در Inbounds بنر **«وارد کردن AWG موجود»** ظاهر می‌شود: پیش‌نمایش همتاها → یک inbound برای هر اینترفیس. کلیدها / IP / پورت / پنهان‌سازی همان‌طور کپی می‌شوند. اینترفیس کرنل درجا عوض‌نام می‌شود (`awg{id}`) و دست‌دادن قطع نمی‌شود. Userspace/Docker: مدیر قدیمی را متوقف کنید؛ آن کلاینت‌ها یک‌بار دوباره وصل می‌شوند.

بدون ماژول کرنل، inboundهای LucXیِ `awg` همچنان روی amneziawg-go توکار بالا می‌آیند. پروتکل بومیِ upstream یعنی `amneziawg` در پنل کنار آن در دسترس است.

</details>

---

## 📜 مجوز و شرایط

این پروژه برای کد خود تحت **دو مجوز** و برای باینری/دادهٔ شخص ثالث تحت شرایط upstream منتشر می‌شود (جدول کامل در [LICENSING.md](../../LICENSING.md)):

<details>
<summary><b>جدول مجوزها</b></summary>

| بخش | مجوز |
|---|---|
| سورس اصلی 3x-ui | **GPL-3.0** |
| مولفه‌های LucX-UI (`internal/awg/`، `internal/lucx/`، صفحات LucX فرانت‌اند) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| پلاگین `forward_proxy` ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy / `bin/naive-client-*` ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/mieru-*` (`mita`، [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| ماژول و ابزار AmneziaWG ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (ماژول؛ روی میزبان نصب می‌شود) |
| geo `.dat` پیش‌فرض (Loyalsoldier / IR / RU / ROSCOM) | شرایط هر دیتاست (ن.ک. LICENSING.md) |

باینری‌های تونل **فرآیند فرزند** هستند — پنل آن‌ها را لینک نمی‌کند. GPL مربوط به qWDTT فقط برای همان باینری و سورس‌هایش است، نه کد PolyFormِ LucX.

</details>

---

## 🤝 قدردانی و اعتبارها

از همهٔ پروژه‌ها و افراد open-source سپاسگزاریم.

<details>
<summary><b>تست‌کنندگان و مشارکت‌کنندگان</b></summary>

- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13)), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, تیم **[3x-ui](https://github.com/MHSanaei/3x-ui)**.

</details>

<details>
<summary><b>PRهای upstream پورت‌شده</b></summary>

- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) geodata browser.

</details>

<details>
<summary><b>پروژه‌ها و الهام</b></summary>

[MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) · [amnezia-vpn](https://github.com/amnezia-vpn) · [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) / [forwardproxy](https://github.com/klzgrad/forwardproxy) · [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) · [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) · [enfein/mieru](https://github.com/enfein/mieru) · [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) · [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) · [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) · [hydraponique](https://github.com/hydraponique) RoscomVPN ([geoip](https://github.com/hydraponique/roscomvpn-geoip) / [geosite](https://github.com/hydraponique/roscomvpn-geosite) / [routing](https://github.com/hydraponique/roscomvpn-routing)) · [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) · [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules) · [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) · [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) · [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) · [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) · [refraction-networking/utls](https://github.com/refraction-networking/utls)

</details>

---

## ☕ حمایت از پروژه

LucX-UI برای استفاده شخصی رایگان است. **خوش‌تان آمد؟ ⭐ بزنید** — به دیده شدن پروژه کمک می‌کند. کمک مالی اختیاری است:

<details>
<summary><b>کمک مالی</b></summary>

| Method | Details |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

</details>

---

## 🛠️ برای توسعه‌دهندگان

<details>
<summary><b>معماری، بیلد و همگام‌سازی بالا‌دست (برای باز شدن کلیک کنید)</b></summary>

**قانون معماری و ایزوله‌سازی.** تمام کد LucX در پکیج‌های ایزوله زندگی می‌کند (`internal/awg/`, `internal/lucx/`)؛ تغییرات در فایل‌های 3x-ui بالا‌دست تنها در داخل نشانگرهای `// LUCX-HOOK` / `// END LUCX-HOOK` انجام می‌شوند تا هر نسخهٔ بالا‌دست یک پورت تقریباً ساده باشد. برای نقشهٔ کامل معماری، ۱۰ قانون، مشکلات شناخته‌شده و الگوهای دیباگ به [AGENTS.md](../../AGENTS.md) مراجعه کنید.

**بیلد از سورس** (نیازمند Go 1.27+, Node.js 24+, gcc — فقط لینوکس، CGO برای SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# pre-push hygiene: bin/check-lucx.sh  (LUCX-HOOK + internal/awg|lucx)
```

**روال همگام‌سازی بالا‌دست** (پایهٔ فعلی — آپ‌استریم **v3.7.0**؛ تگ/main آپ‌استریم را merge کنید، نه v3.5→v3.6 قدیمی):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolve block by block (see AGENTS.md Rule 8) — never blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # compare marker counts before/after to detect lost blocks
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

<!-- END LUCX-HOOK -->
