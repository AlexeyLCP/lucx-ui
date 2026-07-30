<!-- LUCX-HOOK: LucX-UI fork README — FA lead section, license, credits, sources. Keep in sync with LICENSING.md and AGENTS.md. -->
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

پروژه **LucX-UI** یک فورک از [3x-ui](https://github.com/MHSanaei/3x-ui) (نسخه v3.6.0) با پشتیبانی بومی از **AmneziaWG (AWG)** است. AWG به عنوان یک Sidecar در سطح کراتل (Kernel Interface) عمل می‌کند — دقیقاً منطبق بر معماری که در پروژه اصلی برای MTProto (mtg) استفاده شده است: پنل مدیریت چرخه زندگی و محاسبه ترافیک را بر عهده دارد و Xray در صورت تمایل ترافیک را مسیریابی می‌کند.

### ویژگی‌های اضافه شده و فعال

- ✅ **ورودی‌های AWG Inbounds** — سایدی‌کار کرنل بر پایه `awg-quick`: ساخت، همگام‌سازی (reconcile) هر ۱۰ ثانیه، پاکسازی اینترفیس‌های یتیم و نصب‌کننده ماژول کرنل DKMS.
- ✅ **خروجی‌های AWG Outbounds (حالت کلاینت)** — پنل می‌تواند مستقیماً به یک سرور AmneziaWG بالادستی متصل شود: تب اختصاصی در بخش Xray، جای‌گذاری فایل `.conf` موجود و مدیریت اینترفیس کرنل `awgo-{id}` توسط چرخه reconcile. یک outbound از نوع `freedom` با `sockopt.interface` در کانفیگ Xray تزریق می‌شود تا قوانین مسیریابی و لودبالانسرها بتوانند ترافیک را از طریق VPN بالادستی ارسال کنند.
- ✅ **مخفی‌سازی (Obfuscation)** — پروفایل‌های Lite/Standard/Pro (شامل Jc/Jmin/Jmax/S1–S4/H1–H4) و شبیه‌سازی پکت‌های CPS برای: TLS، DNS، SIP و QUIC.
- ✅ **اثرانگشت TLS مرورگرها** — پشتیبانی از Chrome (GREASE)، Firefox 120+ (ترتیب NSS و padding)، و Safari 16+ (ترتیب Apple و TLS 1.1) برای TLS و QUIC.
- ✅ **استخراج امضا از دامنه زنده** — تبدیل دست‌دادن (Handshake) واقعی QUIC از یک فرانت‌دامنه به پارامترهای I1–I5.
- ✅ **مدیریت کلاینت‌ها** — کدهای QR، دانلود فایل `.conf` و محاسبه ترافیک مجزا برای هر پیر (`awg show transfer`).
- ✅ **دو حالت مسیریابی:**
  - **Kernel NAT** — هدایت مستقیم توسط کرنل؛ قواعد NAT پس از فلش شدن iptables به صورت خودکار توسط چرخه reconcile بازیابی می‌شوند.
  - **مسیریابی از طریق Xray (Route through Xray)** — ترافیک از طریق TUN inbound، مسیریابی بر اساس سیاست (policy routing) و sniffing شناسایی شده و از کامل‌ترین پایپ‌لاین مسیریابی Xray (قواعد دامنه/geosite، لودبالانسرها، زنجیره خروجی‌ها) عبور می‌کند.
- ✅ **عیب‌یابی در پنل** — دکمه عیب‌یابی تک‌کلیکه در فرم inbound: بررسی وضعیت اینترفیس، ip_forward، پیرها/دست‌دادن‌ها و قواعد NAT/TUN در یک نگاه.
- ✅ **تست شده در شرایط واقعی** — بر روی سرورهای مجازی تست شده است: Handshake، ICMP، HTTPS، محاسبه ترافیک، زنجیره‌سازی و هر دو حالت مسیریابی کاملاً عملیاتی هستند.

### نصب

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

نصب پنل از [آخرین نسخه انتشار یافته](https://github.com/AlexeyLCP/lucx-ui/releases/latest)، سرویس systemd، هسته Xray-core و mtg (از سورس اصلی 3x-ui) و کامپایل ماژول کرنل AmneziaWG از طریق DKMS (`bin/install-awg-module.sh`).

### مجوزها (License)

این پروژه تحت **دو مجوز** فعالیت می‌کند (جزئیات در [LICENSING.md](LICENSING.md)):

| بخش | مجوز |
|---|---|
| کد اصلی 3x-ui | **GPL-3.0** (طبق الزامات پروژه اصلی) |
| قطعات اختصاصی LucX (`internal/awg/`, `internal/lucx/`, فرانت‌اند AWG و اسکریپت‌ها) | **PolyForm Noncommercial 1.0.0** |

در عمل: برای استفاده‌های شخصی، غیرتجاری، علمی، پژوهشی و آموزشی **کاملاً رایگان** است. **استفاده تجاری** (فروش VPN، پنل‌های پولی یا ادغام در محصول تجاری) نیازمند مجوز کتبی صریح از توسعه‌دهنده است — می‌توانید یک [issue](https://github.com/AlexeyLCP/lucx-ui/issues) باز کنید یا با مالک مخزن در ارتباط باشید. هدرهای `SPDX-License-Identifier` در هر فایل مرز مجوزها را مشخص می‌کنند: عدم وجود هدر به معنی GPL-3.0 است.

### تقدیر و تشکر

- **VladufQa** — تست‌های سرور واقعی (ruvds): اولین دست‌دادن‌ها، ترافیک، زنجیره‌سازی و گزارش باگ‌های مسیریابی.
- **Kirill Rudenko** — تست‌ها (runode) و **PR #13**: قابلیت needRestart در AWG، مسیریابی سیاست‌محور iif، جدول/گیت‌وی مجزا برای هر inbound، بازیابی مسیرها و sniffing.
- **302ba (Alex)** — **PR #24**: رفع مشکل از دست رفتن فیلدهای کلاینت هنگام پارس کردن اسکیما Zod.
- تیم **3x-ui** — برای پایه عالی و معماری Sidecar که از آن الگوبرداری کردیم.

### منابع و الهام‌بخش‌ها

- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — پایه فورک (GPL-3.0) و معماری سایدی‌کار MTProto.
- [AmneziaVPN](https://github.com/amnezia-vpn) — پروتکل اصلی AmneziaWG و ماژول کرنل.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — الگوی PostUp NAT (MASQUERADE + FORWARD)، مولدهای QUIC Initial و شیوه نصب DKMS.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — پورت استخراج امضای QUIC (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) و [refraction-networking/utls](https://github.com/refraction-networking/utls) — اثرانگشت‌های TLS مرورگرهای Firefox/Safari برای پریست‌های ClientHello.

### ☕ حمایت از پروژه

پروژه LucX-UI برای استفاده شخصی و غیرتجاری رایگان است. اگر این پنل به شما کمک کرده، می‌توانید از توسعه آن حمایت کنید:

| روش | جزئیات |
|---|---|
| 🇷🇺 **YooMoney** (روبل روسیه) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

حمایت‌های مالی صرفاً جهت تشکر است و به منزله خرید یا اعطای مجوز تجاری نمی‌باشد و شرایط [LICENSING.md](LICENSING.md) را تغییر نمی‌دهد.

---

*مستندات اصلی **3x-ui** به زبان فارسی در ادامه حفظ شده است.*

<!-- END LUCX-HOOK -->

[English](README.md) | [فارسی](README.fa_IR.md) | [العربية](README.ar_EG.md) | [中文](README.zh_CN.md) | [Español](README.es_ES.md) | [Русский](README.ru_RU.md) | [Türkçe](README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
</p>

**3X-UI** یک پنل کنترل وب پیشرفته و متن‌باز برای مدیریت سرورهای [Xray-core](https://github.com/XTLS/Xray-core) است. این پنل یک رابط کاربری تمیز و چندزبانه برای استقرار، پیکربندی و نظارت بر طیف گسترده‌ای از پروتکل‌های پراکسی و VPN ارائه می‌دهد — از یک VPS تکی تا استقرارهای چندنودی.

‏3X-UI که به‌عنوان یک فورک بهبودیافته از پروژه‌ی اصلی X-UI ساخته شده است، پشتیبانی گسترده‌تر از پروتکل‌ها، پایداری بهتر، حسابداری ترافیک به‌ازای هر کلاینت و بسیاری از ویژگی‌های رفاهی را اضافه می‌کند.

> [!IMPORTANT]
> این پروژه فقط برای استفاده‌ی شخصی در نظر گرفته شده است. لطفاً از آن برای اهداف غیرقانونی یا در محیط تولید (production) استفاده نکنید.

## ویژگی‌ها

- **اینباندهای چندپروتکلی** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، Hysteria2، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **ترنسپورت‌ها و امنیت مدرن** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، ایمن‌شده با TLS، XTLS و REALITY.
- **فال‌بک (Fallback)** — ارائه‌ی چند پروتکل روی یک پورت واحد (مثلاً VLESS و Trojan روی پورت 443) با استفاده از قابلیت fallback در Xray.
- **مدیریت به‌ازای هر کلاینت** — سهمیه‌ی ترافیک، تاریخ انقضا، محدودیت IP، وضعیت آنلاینِ زنده و لینک‌های اشتراک‌گذاری، کدهای QR و سابسکریپشن‌ها با یک کلیک.
- **آمار ترافیک** — به‌ازای هر اینباند، هر کلاینت و هر اوتباند، همراه با کنترل بازنشانی (reset).
- **پشتیبانی از چند نود** — مدیریت و مقیاس‌دهی روی چندین سرور از یک پنل واحد.
- **اوتباند و مسیریابی** — WARP، NordVPN، قوانین مسیریابی سفارشی، متعادل‌کننده‌های بار (load balancer) و زنجیره‌کردن پراکسی اوتباند.
- **سرور سابسکریپشن داخلی** با چندین فرمت خروجی و [قالب‌های صفحه‌ی سفارشی](docs/custom-subscription-templates.md).
- **ربات تلگرام** برای نظارت و مدیریت از راه دور.
- **‏RESTful API** همراه با مستندات Swagger درون‌پنل.
- **ذخیره‌سازی منعطف** — SQLite (پیش‌فرض) یا PostgreSQL.
- **‏۱۳ زبان رابط کاربری** با تم‌های تیره و روشن.
- **یکپارچگی با Fail2ban** برای اعمال محدودیت IP به‌ازای هر کلاینت.

## اسکرین‌شات‌ها

<details>
<summary>برای باز شدن کلیک کنید</summary>

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

## شروع سریع

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

برای نصب یک نسخه‌ی مشخص، تگ آن را در انتها اضافه کنید (مثلاً `v3.4.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

برای نصب نسخه‌ی غلتانِ **dev** (آخرین پیش‌انتشار به‌ازای هر کامیت از شاخه‌ی `main`، نه یک انتشار پایدار)، مقدار `dev-latest` را پاس دهید:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

در حین نصب، یک نام کاربری، رمز عبور و مسیر دسترسی تصادفی تولید می‌شود. پس از نصب، دستور `x-ui` را اجرا کنید تا منوی مدیریت باز شود؛ در آنجا می‌توانید سرویس را شروع/متوقف کنید، اطلاعات ورود خود را ببینید یا بازنشانی کنید، گواهی‌های SSL را مدیریت کنید و کارهای دیگری انجام دهید.

برای مستندات کامل، لطفاً به [ویکی پروژه](https://github.com/MHSanaei/3x-ui/wiki) مراجعه کنید.

### نصب بدون نظارت

نصب‌کننده به‌صورت **غیرتعاملی** نیز برای cloud-init اجرا می‌شود.
‏`XUI_NONINTERACTIVE=1` را تنظیم کنید (یا بدون TTY از طریق pipe اجرا کنید) تا نصب به‌صورت سرتاسری و بدون
هیچ پرسشی انجام شود، اطلاعات ورود تصادفی تولید کرده و آن‌ها را در
`/etc/x-ui/install-result.env` می‌نویسد. برای موارد زیر به [`deploy/`](deploy/) مراجعه کنید:

- [user-data مربوط به Cloud-init](deploy/cloud-init/) — نصب بدون نظارت روی هر ابری (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [یادداشت‌های Hetzner Cloud](deploy/marketplace/hetzner/) — استقرار مبتنی بر cloud-init روی Hetzner

## پلتفرم‌های پشتیبانی‌شده

**سیستم‌عامل‌ها:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap)، Alpine و Windows.

**معماری‌ها:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## گزینه‌های پایگاه‌داده

‏3X-UI از دو بک‌اند پشتیبانی می‌کند که در حین نصب انتخاب می‌شوند:

- **SQLite** (پیش‌فرض) — یک فایل واحد در مسیر `/etc/x-ui/x-ui.db`. بدون نیاز به تنظیمات، ایده‌آل برای استقرارهای کوچک و متوسط.
- **PostgreSQL** — برای تعداد کلاینت بالا یا راه‌اندازی‌های چندنودی توصیه می‌شود. نصب‌کننده می‌تواند PostgreSQL را به‌صورت محلی برایتان نصب کند، یا یک DSN به یک سرور موجود را بپذیرد.

در زمان اجرا، بک‌اند از طریق متغیرهای محیطی انتخاب می‌شود (نصب‌کننده این موارد را برای شما در `/etc/default/x-ui` می‌نویسد):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### انتقال یک نصب موجود SQLite به PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# سپس XUI_DB_TYPE و XUI_DB_DSN را در /etc/default/x-ui تنظیم کرده و ری‌استارت کنید:
systemctl restart x-ui
```

فایل اصلی SQLite دست‌نخورده باقی می‌ماند؛ پس از اطمینان از صحت بک‌اند جدید، آن را به‌صورت دستی حذف کنید.

### Docker

دستور پیش‌فرض `docker compose up -d` همچنان از SQLite استفاده می‌کند. برای اجرا با سرویس PostgreSQL همراه، دو خط متغیر محیطی `XUI_DB_*` را در `docker-compose.yml` از حالت کامنت خارج کنید و با پروفایل زیر اجرا کنید:

```bash
docker compose --profile postgres up -d
```

این ایمیج، Fail2ban را (که به‌صورت پیش‌فرض فعال است) برای اعمال **محدودیت‌های IP** به‌ازای هر کلاینت همراه دارد. ‏Fail2ban متخلفان را با `iptables` مسدود می‌کند که به مجوز `NET_ADMIN` نیاز دارد. فایل `docker-compose.yml` این مجوز را از قبل از طریق `cap_add` می‌دهد؛ اگر به‌جای آن کانتینر را با `docker run` اجرا می‌کنید، خودتان مجوزها را اضافه کنید، در غیر این صورت مسدودسازی‌ها فقط ثبت می‌شوند اما هرگز اعمال نمی‌شوند:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## متغیرهای محیطی

| متغیر | توضیحات | پیش‌فرض |
| --- | --- | --- |
| `XUI_DB_TYPE` | بک‌اند پایگاه‌داده: `sqlite` یا `postgres` | `sqlite` |
| `XUI_DB_DSN` | رشته‌ی اتصال PostgreSQL (وقتی `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | پوشه‌ی فایل پایگاه‌داده‌ی SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | حداکثر اتصالات باز (استخر PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | حداکثر اتصالات بی‌کار (استخر PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | مسیر URI اولیه برای پنل وب | `/` |
| `XUI_ENABLE_FAIL2BAN` | فعال‌سازی اعمال محدودیت IP مبتنی بر Fail2ban | `true` |
| `XUI_LOG_LEVEL` | سطح گزارش‌گیری (`debug`، `info`، `warning`، `error`) | `info` |
| `XUI_DEBUG` | فعال‌سازی حالت دیباگ | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | فعال‌سازی پایشگر سلامت تونل (یک URL را پروب می‌کند و پس از خطاهای مکرر، xray را ری‌استارت می‌کند؛ یک ری‌استارت همه‌ی کلاینت‌ها را قطع می‌کند) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | پراکسی‌ای که پروب از طریق آن ارسال می‌شود؛ آن را به یک اینباند محلی xray اشاره دهید تا پروب خودِ تونل را آزمایش کند (مثلاً `socks5://127.0.0.1:1080`). خالی بودن یعنی پروب فقط اتصال به هاست را بررسی می‌کند | — |
| `XUI_TUNNEL_HEALTH_URL` | URL ای که برای سلامت تونل پروب می‌شود | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | فاصله‌ی زمانی بین پروب‌ها | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | مهلت زمانی هر پروب | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | تعداد خطاهای متوالی پیش از آن‌که یک ری‌استارت فعال شود | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | حداقل تأخیر بین ری‌استارت‌های متوالی | `5m` |

## زبان‌های پشتیبانی‌شده

رابط کاربری پنل به ۱۳ زبان در دسترس است:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## مشارکت

از مشارکت‌ها استقبال می‌شود. لطفاً پیش از باز کردن issue یا pull request، [راهنمای مشارکت](/CONTRIBUTING.md) را مطالعه کنید.

## تشکر ویژه از

- [alireza0](https://github.com/alireza0/)

## قدردانی

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (مجوز: **GPL-3.0**): _قوانین مسیریابی بهبود یافته v2ray/xray و v2ray/xray-clients با دامنه‌های ایرانی داخلی و تمرکز بر امنیت و مسدود کردن تبلیغات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (مجوز: **GPL-3.0**): _این مخزن شامل قوانین مسیریابی V2Ray به‌روزرسانی شده خودکار بر اساس داده‌های دامنه‌ها و آدرس‌های مسدود شده در روسیه است._

## ابزارهای جامعه

ابزارها و یکپارچه‌سازی‌هایی که توسط جامعه پیرامون 3x-ui ساخته شده‌اند.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (مجوز: **MIT**): _مدیریت اینباندها، کلاینت‌ها، تنظیمات پنل و پیکربندی Xray به‌صورت کد با Terraform / OpenTofu._

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
