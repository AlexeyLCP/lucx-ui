<!-- LUCX-HOOK: LucX-UI fork README — Unified FA README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
  <a href="README.ru_RU.md">Русский</a> |
  <b>فارسی</b> |
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **صرفاً برای استفاده شخصی، غیرتجاری، علمی، پژوهشی و آموزشی.** هرگونه استفاده تجاری — شامل فروش مجدد سرویس VPN، پنل‌های پولی یا خدمات اشتراکی بر پایه این کد — نیازمند کسب اجازه کتبی از صادرکننده می‌باشد. از استفاده در مقاصد غیرقانونی خودداری کنید.

---

## درباره LucX-UI

پروژه **LucX-UI** یک پنل مدیریت وب پیشرفته برای سرورهای [Xray-core](https://github.com/XTLS/Xray-core) است که به عنوان یک فورک توسعه‌یافته از [3x-ui](https://github.com/MHSanaei/3x-ui) (نسخه v3.6.0) با پشتیبانی بومی از **AmneziaWG (AWG)** ساخته شده است. AWG به عنوان یک Sidecar در سطح کرنل عمل می‌کند — دقیقاً منطبق بر معماری که در پروژه اصلی برای MTProto (mtg) استفاده شده است: پنل مدیریت چرخه زندگی و محاسبه ترافیک را بر عهده دارد و Xray در صورت تمایل ترافیک را مسیریابی می‌کند.

### ویژگی‌های کلیدی

#### 🛡️ قابلیت‌های پیشرفته AmneziaWG (AWG)
- **ورودی‌های AWG Inbounds** — سایدی‌کار کرنل بر پایه `awg-quick`: ساخت، همگام‌سازی (reconcile) هر ۱۰ ثانیه، پاکسازی اینترفیس‌های یتیم و نصب‌کننده ماژول کرنل DKMS.
- **خروجی‌های AWG Outbounds (حالت کلاینت)** — پنل می‌تواند مستقیماً به یک سرور AmneziaWG بالادستی متصل شود: تب اختصاصی در بخش Xray، جای‌گذاری فایل `.conf` موجود و مدیریت اینترفیس کرنل `awgo-{id}` توسط چرخه reconcile. یک outbound از نوع `freedom` با `sockopt.interface` در کانفیگ Xray تزریق می‌شود تا قوانین مسیریابی ترافیک را از طریق VPN بالادستی ارسال کنند.
- **مخفی‌سازی (Obfuscation)** — پروفایل‌های Lite/Standard/Pro (شامل Jc/Jmin/Jmax/S1–S4/H1–H4) و شبیه‌سازی پکت‌های CPS برای: TLS، DNS، SIP و QUIC.
- **اثرانگشت TLS مرورگرها** — پشتیبانی از Chrome (GREASE)، Firefox 120+ (ترتیب NSS و padding)، و Safari 16+ (ترتیب Apple و TLS 1.1) برای TLS و QUIC.
- **استخراج امضا از دامنه زنده** — تبدیل دست‌دادن (Handshake) واقعی QUIC از یک فرانت‌دامنه به پارامترهای I1–I5.
- **مدیریت کلاینت‌ها** — کدهای QR، دانلود فایل `.conf` و محاسبه ترافیک مجزا برای هر پیر (`awg show transfer`).
- **دو حالت مسیریابی**:
  - **Kernel NAT** — هدایت مستقیم توسط کرنل؛ قواعد NAT پس از فلش شدن iptables به صورت خودکار توسط چرخه reconcile بازیابی می‌شوند.
  - **Route through Xray** — ترافیک از طریق TUN inbound، مسیریابی بر اساس سیاست (policy routing) و sniffing شناسایی شده و از کامل‌ترین پایپ‌لاین مسیریابی Xray عبور می‌کند.
- **عیب‌یابی در پنل** — دکمه عیب‌یابی تک‌کلیکه در فرم inbound: بررسی وضعیت اینترفیس، ip_forward، پیرها/دست‌دادن‌ها و قواعد NAT/TUN.

#### 🚀 ویژگی‌های اصلی 3x-ui
- **ورودی‌های چندپروتکله** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، Hysteria2، HTTP، SOCKS (مخلوط) و TUN.
- **پروتکل‌های انتقال و امنیت مدرن** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، ایمن‌شده با TLS، XTLS و REALITY.
- **مکانیسم Fallback** — ارائه چند پروتکل روی یک پورت واحد (مثلاً VLESS و Trojan روی پورت ۴۴۳).
- **مدیریت کلاینت‌ها** — حجم ترافیک، تاریخ انقضا، محدودیت IP، وضعیت آنلاین و لینک‌های اشتراک/QR.
- **آمار ترافیک** — به تفکیک ورودی، کلاینت و خروجی.
- **پشتیبانی از چند نود** — مدیریت و مقیاس‌پذیری سرورهای متعدد از یک پنل.
- **خروجی‌ها و مسیریابی** — WARP، NordVPN، قوانین سفارشی و لودبالانسرها.
- **سرور اشتراک داخلی** با قالب‌های سفارشی.
- **ربات تلگرام** برای مانیتورینگ از راه دور.
- **RESTful API** همراه با مستندات Swagger.
- **ذخيره‌سازی انعطاف‌پذیر** — SQLite (پیش‌فرض) یا PostgreSQL.
- **ادغام با Fail2ban** برای اعمال محدودیت IP.

### تصاویر محیط پنل

<details>
<summary>برای مشاهده کلیک کنید</summary>

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

## راه اندازی سریع

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

نصب پنل از [آخرین نسخه انتشار یافته](https://github.com/AlexeyLCP/lucx-ui/releases/latest)، سرویس systemd، هسته Xray-core و mtg و کامپایل ماژول کرنل AmneziaWG از طریق DKMS (`bin/install-awg-module.sh`).

پس از نصب، دستور `x-ui` را برای ورود به منوی مدیریت اجرا کنید.

### نصب خودکار (Non-interactive)

اسکریپت نصب از حالت غیرتعاملی برای cloud-init پشتیبانی می‌کند. با تنظیم `XUI_NONINTERACTIVE=1` نصب بدون پرسش انجام شده و اطلاعات در `/etc/x-ui/install-result.env` ذخیره می‌شود.

## سیستم‌عامل‌ها و معماری‌های پشتیبانی‌شده

**سیستم‌عامل‌ها:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine و Windows.

**معماری‌ها:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## تنظیمات دیتابیس

3X-UI از دو دیتابیس پشتیبانی می‌کند:

- **SQLite** (پیش‌فرض) — فایل `/etc/x-ui/x-ui.db`.
- **PostgreSQL** — پیشنهادی برای تعداد کلاینت‌های بالا یا سرورهای متعدد.

متغیرهای محیطی در `/etc/default/x-ui`:
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### داکر (Docker)

برای استفاده از PostgreSQL در داکر، خطوط `XUI_DB_*` را در `docker-compose.yml` از حالت کامنت خارج کرده و اجرا کنید:
```bash
docker compose --profile postgres up -d
```

## متغیرهای محیطی

| متغیر | توضیح | مقدار پیش‌فرض |
| --- | --- | --- |
| `XUI_DB_TYPE` | نوع دیتابیس: `sqlite` یا `postgres` | `sqlite` |
| `XUI_DB_DSN` | آدرس اتصال به PostgreSQL (در صورت `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | مسیر پوشه فایل دیتابیس SQLite | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | فعال‌سازی Fail2ban برای محدودیت IP | `true` |
| `XUI_LOG_LEVEL` | سطح لاگ‌ها (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | مانیتور سلامت تونل | `false` |

## مجوزها و شرایط (License)

این پروژه تحت **دو مجوز** منتشر می‌شود (جزئیات در [LICENSING.md](LICENSING.md)):

| بخش | مجوز |
|---|---|
| کد اصلی 3x-ui | **GPL-3.0** (طبق الزامات پروژه اصلی) |
| قطعات اختصاصی LucX (`internal/awg/`, `internal/lucx/`, فرانت‌اند AWG و اسکریپت‌ها) | **PolyForm Noncommercial 1.0.0** |

برای استفاده‌های شخصی، غیرتجاری، علمی، پژوهشی و آموزشی **کاملاً رایگان** است. **استفاده تجاری** (فروش VPN، پنل‌های پولی) نیازمند مجوز کتبی از توسعه‌دهنده است: یک [issue](https://github.com/AlexeyLCP/lucx-ui/issues) باز کنید یا با مالک مخزن ارتباط بگیرید.

## مشارکت در پروژه

مشارکت شما موجب خوشحالی است. لطفاً پیش از ارسال issue یا PR، [راهنمای مشارکت](/CONTRIBUTING.md) را مطالعه کنید.

## قدردانی و منابع

### تست‌کنندگان و مشارکت‌کنندگان
- **VladufQa** — تست‌های سرور واقعی (ruvds): اولین دست‌دادن‌ها، ترافیک، زنجیره‌سازی و گزارش باگ‌ها.
- **Kirill Rudenko** — تست‌ها (runode) و **PR #13**: قابلیت needRestart در AWG، مسیریابی iif، جداول/گیت‌وی‌های مجزا، بازیابی مسیرها و sniffing.
- **302ba (Alex)** — **PR #24**: رفع مشکل از دست رفتن فیلدهای کلاینت هنگام پارس اسکیما Zod.
- **alireza0** — مشارکت‌کننده پروژه اصلی.
- تیم **3x-ui** — برای پایه عالی و معماری سایدی‌کار.

### منابع و الهام‌بخش‌ها
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — پایه فورک (GPL-3.0)، معماری سایدی‌کار MTProto.
- [AmneziaVPN](https://github.com/amnezia-vpn) — پروتکل اصلی AmneziaWG و ماژول کرنل.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — الگوی PostUp NAT (MASQUERADE + FORWARD)، مولدهای QUIC Initial و DKMS.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — پورت استخراج امضای QUIC (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) و [refraction-networking/utls](https://github.com/refraction-networking/utls) — اثرانگشت‌های TLS مرورگرها.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) و [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) — قوانین مسیریابی.

### ابزارهای جامعه
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (مجوز: **MIT**): مدیریت کانفیگ‌ها و پنل به‌صورت کد.

## ☕ حمایت از پروژه

پروژه LucX-UI برای استفاده شخصی و غیرتجاری رایگان است. اگر این پنل به شما کمک کرده، می‌توانید از توسعه آن حمایت کنید:

| روش | جزئیات |
|---|---|
| 🇷🇺 **YooMoney** (روبل روسیه) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

حمایت‌های مالی صرفاً جهت تشکر است و به منزله خرید یا اعطای مجوز تجاری نمی‌باشد و شرایط [LICENSING.md](LICENSING.md) را تغییر نمی‌دهد.

## ستاره‌ها در طول زمان

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)

<!-- END LUCX-HOOK -->
