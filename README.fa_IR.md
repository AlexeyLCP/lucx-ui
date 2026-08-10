<!-- LUCX-HOOK: LucX-UI fork README — Streamlined FA README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **پنل مدیریت پیشرفته Xray و AmneziaWG** — با سابسکریپشن‌های یکپارچه، مدیریت چند سرور و پشتیبانی بومی از AWG.

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
> **صرفاً برای استفاده شخصی، غیرتجاری، علمی، پژوهشی و آموزشی.** هرگونه استفاده تجاری — از جمله فروش مجدد VPN یا پنل‌های پولی — نیازمند کسب اجازه کتبی صریح تحت مجوز PolyForm Noncommercial 1.0.0 می‌باشد.

---

## ⚡ راه‌اندازی سریع

نصب تک‌خطی روی **لینوکس (Ubuntu / Debian / CentOS / AlmaLinux / Arch و غیره)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

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

[3x-ui](https://github.com/MHSanaei/3x-ui) یک پنل چندپروتکله عالی با فرانت‌اند مدرن React 19 + Ant Design 6 است. LucX-UI تمام آنچه 3x-ui ارائه می‌دهد را حفظ کرده و **پشتیبانی بومی AmneziaWG (AWG)** — یک فورک مقاوم در برابر سانسور از WireGuard — را اضافه می‌کند که 3x-ui آن را ندارد:

| ویژگی | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG ورودی (سایدی‌کار کرنل از طریق `awg-quick`) | ✗ | ✓ |
| AWG CPS پنهان‌سازی (TLS / DNS / SIP / QUIC + اثرانگشت مرورگر) | ✗ | ✓ |
| AWG خروجی — زنجیره VPN به سرورهای AWG بالادستی (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| پیش‌تنظیم‌های نسخه کانفیگ کلاینت (1.5 / 2 / 3) | ✗ | ✓ |
| عیب‌یابی AWG درون پنل (مسیریابی / NAT / همتاها / دست‌دادن‌ها) | ✗ | ✓ |
| سایدی‌کار تونل NaiveProxy (Caddy + forward_proxy، تحت نظارت پنل) | ✗ | ✓ |
| اعتبارنامه‌های per-client NaiveProxy + `naive+https://` در اشتراک‌ها | ✗ | ✓ |
| NaiveProxy → مسیریابی Xray (پل SOCKS loopback، اختیاری) | ✗ | ✓ |
| لینک‌های خروجی Smart Cluster | ✗ | ✓ |
| فرانت‌اند React 19 + AntD 6 + Vite 8 + Zod 4 | ✓ | ✓ (به ارث رسیده) |
| تمام پروتکل‌های Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| همگام‌سازی بدون اصطکاک بالا‌دست (ایزوله‌سازی LUCX-HOOK، ۴۹ فایل) | — | ✓ |

یک سایدی‌کار کرنل (مانند `mtg` در MTProto ِ 3x-ui) به این معناست که AWG به‌عنوان یک رابط کرنل واقعی اجرا می‌شود — نه یک shim فضای کاربری — در نتیجه Xray ترافیک رمزگشاشده را از طریق TUN ورودیِ خود عبور می‌دهد و قدرت کامل مسیریابی، شنود و قوانین دامنهٔ Xray را روی ترافیک AWG در اختیار شما قرار می‌دهد.

---

## 🌟 درباره LucX-UI

**LucX-UI** یک فورک ارتقایافته از [3x-ui](https://github.com/MHSanaei/3x-ui) (در حال حاضر با **v3.6.0** بالا‌دست همگام است) است که پشتیبانی بومی **AmneziaWG (AWG)** را به‌عنوان یک سایدی‌کار رابط کرنل اضافه می‌کند و معماری MTProto بالا‌دست را بازتاب می‌دهد. این پروژه با ایزوله‌سازی سخت‌گیرانه کد `LUCX-HOOK`، سازگاری ۱۰۰٪ با بالا‌دست را حفظ می‌کند.

### 🛡️ ویژگی‌های AmneziaWG (AWG)
- **ورودی‌ها و خروجی‌های AWG** — سایدی‌کار کرنل (`awg-quick`)، شماره‌گیری حالت کلاینت به سرورهای AWG بالا‌دستی (`awgo-{id}`)، حلقه همگام‌سازی اتوماتیک ۱۰ ثانیه‌ای، و سازنده ماژول کرنل DKMS.
- **مخفی‌سازی پیشرفته** — پریست‌های Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4)، شبیه‌سازی پکت‌های CPS (TLS, DNS, SIP, QUIC) و اثرانگشت TLS مرورگرها (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — حفاظت هدر AmneziaWG 3 با کلیدهای ۳۲ بایتیِ اتوماتیک؛ سقف نسخه سمت سرور، انتشار ویژگی به‌ازای کلاینت را کنترل می‌کند.
- **پیش‌تنظیم‌های نسخه کلاینت** — تولید کانفیگ کلاینت برای AWG 1.5 / 2 / 3 از یک ورودی واحد — فرمتی که اپ کلاینت شما می‌فهمد را انتخاب کنید.
- **استخراج امضای زنده** — تبدیل دست‌دادن‌های واقعی QUIC از دامنه‌های فرانت به پارامترهای مخفی‌سازی I1–I5.
- **مسیریابی و عیب‌یابی** — دو حالت مسیریابی (Kernel NAT و Route through Xray با مسیریابی سیاست و شنود) + عیب‌یابی تک‌کلیکه درون پنل.

### 🚇 سایدی‌کارهای تونل (NaiveProxy)
- **NaiveProxy** — Caddy با افزونهٔ `forward_proxy` (فورک [klzgrad](https://github.com/klzgrad/forwardproxy)، padding مربوط به HTTP/2) به‌عنوان سایدی‌کار تحت نظارت پنل اجرا می‌شود: Caddyfile رندر‌شده، start/stop/restart با reconcile احیای پس از کرش، و پروب سلامت سه‌سطحی (process → TCP → TLS).
- **اعتبارنامه‌های per-client** — هر کلاینت فعال پنل به‌طور اتوماتیک یک جفت شخصی `basic_auth` می‌گیرد (مشتق از راز پنل، بدون ذخیره‌سازی)؛ غیرفعال‌کردن کلاینت در reconcile بعدی آن را باطل می‌کند.
- **اشتراک‌ها** — اشتراک هر کلاینت لینک شخصی `naive+https://` او را در کنار لینک‌های Xray/AWG حمل می‌کند (فرمت استاندارد NekoBox / husi / Exclave)، به‌علاوه کد QR و تولیدکنندهٔ رمز عبور قوی در پنل.
- **UX پنل** — Auto TLS (Let's Encrypt) یا cert/key خودتان، حالت raw-Caddyfile با اعتبارسنجی `caddy adapt`، پیش‌نمایش Caddyfile، لاگ‌های فرایند، آپلود/دانلود باینری.
- **مسیریابی از طریق Xray (اختیاری)** — سوییچ باعث می‌شود Caddy مقاصد را از طریق پل SOCKS loopback پنهان شماره‌گیری کند (`upstream socks5://127.0.0.1:…`، forward_proxy بومی — بدون پچ) با برچسب `lucx-tunnel-naive`، تا ترافیک NaiveProxy مسیریابی / شنود / قوانین دامنهٔ کامل Xray را بگیرد (همان الگوی MTProto). پیش‌فرض همچنان egress مستقیم است.

### 🚀 ویژگی‌های اصلی 3x-ui
- **پروتکل‌ها:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **انتقال و امنیت:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **مدیریت:** حجم ترافیک، محدودیت IP (Fail2ban)، وضعیت آنلاین، اشتراک‌ها، ربات تلگرام، REST API، چند نود، SQLite / PostgreSQL.

<details>
<summary><b>📸 تصاویر محیط پنل</b></summary>

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

## 🔄 مهاجرت از 3x-ui

LucX-UI همان پایگاهِ اسکیمای دیتابیس Xray-core / SQLite (یا PostgreSQL) را با 3x-ui به اشتراک می‌گذارد و جداول AWG در اولین اجرا به‌صورت اتوماتیک ساخته می‌شوند. برای نصب روی یک راه‌اندازی موجود 3x-ui، ابتدا دیتابیس خود را پشتیبان بگیرید و سپس دستور نصب استاندارد را اجرا کنید:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

ماژول کرنل AWG توسط نصب‌کننده اتوماتیک ساخته می‌شود (`bin/install-awg-module.sh`, DKMS). پس از نصب، برای تأیید نسخهٔ ماژول کرنل AWG در کنسول `x-ui` را اجرا کنید و افزودن ورودی‌های AWG را از پنل آغاز کنید.

---

## 📜 مجوز و شرایط

این پروژه تحت **دو مجوز** منتشر می‌شود (جزئیات در [LICENSING.md](LICENSING.md)):

| بخش | مجوز |
|---|---|
| سورس اصلی 3x-ui | **GPL-3.0** |
| مولفه‌های LucX-UI (`internal/awg/`, `internal/lucx/`, فرانت‌اند) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 قدردانی و اعتبارها

- **تست‌کنندگان و مشارکت‌کنندگان:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, تیم **3x-ui**.
- **پروژه‌ها و الهام‌بخش‌ها:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) & [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) (سایدی‌کار تونل NaiveProxy), [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) (مرجع طراحی یکپارچه‌سازی Caddyfile), [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) (مرجع مفهوم سایدی‌کار تونل: qWDTT / olcRTC), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ حمایت از پروژه

LucX-UI برای استفاده شخصی رایگان است. می‌توانید از توسعهٔ مداوم آن حمایت کنید:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ برای توسعه‌دهندگان

<details>
<summary><b>معماری، بیلد و همگام‌سازی بالا‌دست (برای باز شدن کلیک کنید)</b></summary>

**قانون معماری و ایزوله‌سازی.** تمام کد LucX در پکیج‌های ایزوله زندگی می‌کند (`internal/awg/`, `internal/lucx/`)؛ تغییرات در فایل‌های 3x-ui بالا‌دست تنها در داخل نشانگرهای `// LUCX-HOOK` / `// END LUCX-HOOK` انجام می‌شوند تا هر نسخهٔ بالا‌دست یک پورت تقریباً ساده باشد. برای نقشهٔ کامل معماری، ۱۰ قانون، مشکلات شناخته‌شده و الگوهای دیباگ به [AGENTS.md](AGENTS.md) مراجعه کنید.

**بیلد از سورس** (نیازمند Go 1.23+, Node.js 20+, gcc — فقط لینوکس، CGO برای SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# pre-push hygiene: bin/check-lucx.sh  (gofumpt on the 49 LucX-owned files)
```

**روال همگام‌سازی بالا‌دست** (اعتبارسنجی‌شده v3.5.0→v3.6.0، 103 commit / 432 فایل / 7 تداخل):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolve block by block (see AGENTS.md Rule 8) — never blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # compare marker counts before/after to detect lost blocks
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

---

## ⭐ ستاره‌ها در طول زمان

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
