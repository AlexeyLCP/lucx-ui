<!-- LUCX-HOOK: LucX-UI fork README — Streamlined FA README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **صرفاً برای استفاده شخصی، غیرتجاری، علمی و آموزشی.** هرگونه استفاده تجاری (فروش مجدد VPN یا پنل‌های پولی) نیازمند کسب اجازه کتبی تحت مجوز PolyForm Noncommercial 1.0.0 می‌باشد.

---

## ⚡ راه‌اندازی سریع

نصب تک‌خطی روی **لینوکس (Ubuntu / Debian / CentOS / AlmaLinux / Arch و غیره)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ نصب پیشرفته و پیکربندی (Cloud-Init، داکر، PostgreSQL، متغیرهای محیطی)</b></summary>

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
| `XUI_DB_DSN` | آدرس اتصال به PostgreSQL | — |
| `XUI_ENABLE_FAIL2BAN` | فعال‌سازی Fail2ban برای محدودیت IP | `true` |
| `XUI_LOG_LEVEL` | سطح لاگ‌ها (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🌟 درباره LucX-UI

پروژه **LucX-UI** یک پنل مدیریت وب پیشرفته برای سرورهای [Xray-core](https://github.com/XTLS/Xray-core) است که به عنوان یک فورک ارتقایافته از [3x-ui](https://github.com/MHSanaei/3x-ui) با پشتیبانی بومی از **AmneziaWG (AWG)** در سطح کرنل ساخته شده است.

### 🛡️ ویژگی‌های AmneziaWG (AWG)
- **ورودی‌ها و خروجی‌های AWG** — سایدی‌کار کرنل (`awg-quick`)، اتصال به سرورهای AWG بالادستی (`awgo-{id}`)، همگام‌سازی اتوماتیک ۱۰ ثانیه‌ای و ماژول‌ساز کرنل DKMS.
- **مخفی‌سازی پیشرفته** — پریست‌های Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4)، شبیه‌سازی پکت‌های CPS (TLS, DNS, SIP, QUIC) و اثرانگشت TLS مرورگرها (Chrome, Firefox, Safari).
- **استخراج امضا (Live Capture)** — تبدیل دست‌دادن‌های واقعی QUIC به پارامترهای I1–I5.
- **مسیریابی و عیب‌یابی** — دو حالت مسیریابی (Kernel NAT و Route through Xray) به همراه عیب‌یابی تک‌کلیکه در پنل.

### 🚀 ویژگی‌های اصلی 3x-ui
- **پروتکل‌ها:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **امنیت و انتقال:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
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

## 📜 مجوزها و شرایط

این پروژه تحت **دو مجوز** منتشر می‌شود (جزئیات در [LICENSING.md](LICENSING.md)):

| بخش | مجوز |
|---|---|
| سورس اصلی 3x-ui | **GPL-3.0** |
| ماژول‌های اختصاصی LucX-UI (`internal/awg/`, `internal/lucx/`, فرانت‌اند) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 قدردانی و منابع

- **تست‌کنندگان و مشارکت‌کنندگان:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, تیم **3x-ui**.
- **پروژه‌ها و الهام‌بخش‌ها:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ حمایت از پروژه

پروژه LucX-UI برای استفاده شخصی رایگان است. می‌توانید از توسعه آن حمایت کنید:

| روش | جزئیات |
|---|---|
| 🇷🇺 **YooMoney** (روبل روسیه) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ ستاره‌ها در طول زمان

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
