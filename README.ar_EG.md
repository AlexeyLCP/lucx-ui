<!-- LUCX-HOOK: LucX-UI fork README — Streamlined AR README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
  <a href="README.fa_IR.md">فارسی</a> |
  <b>العربية</b> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **مخصص للاستخدام الشخصي، غير التجاري، العلمي والتعليمي فقط.** الاستخدام التجاري (إعادة بيع VPN أو اللوحات المدفوعة) يتطلب إذناً كتابياً صريحاً بموجب ترخيص PolyForm Noncommercial 1.0.0.

---

## ⚡ التثبيت السريع

تثبيت بأمر واحد على **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch إلخ)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ التثبيت التلقائي والتكوين المتقدم (Cloud-Init, Docker, PostgreSQL, المتغيرات)</b></summary>

### التثبيت التلقائي (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
تُحفظ بيانات الدخول في `/etc/x-ui/install-result.env`.

### Docker مع PostgreSQL
```bash
docker compose --profile postgres up -d
```

### المتغيرات البيئية الرئيسية (`/etc/default/x-ui`)
| المتغير | الوصف | الافتراضي |
| --- | --- | --- |
| `XUI_DB_TYPE` | محرك قاعدة البيانات (`sqlite` أو `postgres`) | `sqlite` |
| `XUI_DB_DSN` | نص اتصال PostgreSQL | — |
| `XUI_ENABLE_FAIL2BAN` | تفعيل Fail2ban لحظر IP | `true` |
| `XUI_LOG_LEVEL` | مستوى السجلات (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🌟 حول LucX-UI

مشروع **LucX-UI** هو لوحة تحكم ويب متقدمة لإدارة سيرفرات [Xray-core](https://github.com/XTLS/Xray-core)، تم إنشاؤها كنسخة مفرعة ومتطورة من [3x-ui](https://github.com/MHSanaei/3x-ui) مع دعم أصلي لـ **AmneziaWG (AWG)** على مستوى النواة.

### 🛡️ ميزات AmneziaWG (AWG)
- **AWG Inbounds & Outbounds** — Sidecar للنواة (`awg-quick`)، الاتصال بسيرفرات AWG رئيسية (`awgo-{id}`)، دورة توفيق تلقائية كل 10 ثوانٍ، ومثبت موديل النواة عبر DKMS.
- **تعتيم متقدم** — بروفايلات Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4)، محاكاة حزم CPS (TLS, DNS, SIP, QUIC)، وبصمات TLS للمتصفحات (Chrome, Firefox, Safari).
- **التقاط التوقيع المباشر** — تحويل مصافحة QUIC الحقيقية إلى قيم تعتيم I1–I5.
- **التوجيه والتشخيص** — نمطان للتوجيه (Kernel NAT و Route through Xray) تشخيص بنقرة واحدة في اللوحة.

### 🚀 الميزات الأساسية لـ 3x-ui
- **البروتوكولات:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **الأمان والنقل:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **الإدارة:** حصص الترافيك، حدود IP (Fail2ban)، حالة الاتصال، الاشتراكات، بوت تلجرام، REST API، السيرفرات المتعددة، SQLite / PostgreSQL.

<details>
<summary><b>📸 لقطات الشاشة</b></summary>

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

## 📜 التراخيص والشروط

يعمل هذا المشروع تحت **ترخيصين** (التفاصيل في [LICENSING.md](LICENSING.md)):

| الجزء | الترخيص |
|---|---|
| كود 3x-ui الأصلي | **GPL-3.0** |
| مكونات LucX-UI (`internal/awg/`, `internal/lucx/`, الواجهة) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 الشكر والتقدير

- **المختبرون والمساهمون:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, فريق **3x-ui**.
- **المشاريع والإلهام:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ دعم المشروع

مشروع LucX-UI مجاني للاستخدام الشخصي. يمكنك دعم التطوير المستمر:

| الطريقة | التفاصيل |
|---|---|
| 🇷🇺 **YooMoney** (روبل، روسيا) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ النجوم عبر الزمن

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
