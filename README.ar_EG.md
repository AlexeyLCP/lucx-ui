<!-- LUCX-HOOK: LucX-UI fork README — Unified AR README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **مخصص للاستخدام الشخصي، غير التجاري، العلمي، البحثي والتعليمي فقط.** الاستخدام التجاري — بما في ذلك إعادة بيع خدمة VPN، أو اللوحات المدفوعة، أو خدمات الاشتراك المبنية على هذا الكود — يتطلب إذناً كتابياً صريحاً من المؤلف. لا تستخدمه لأغراض غير قانونية.

---

## حول LucX-UI

مشروع **LucX-UI** هو لوحة تحكم ويب متقدمة لإدارة سيرفرات [Xray-core](https://github.com/XTLS/Xray-core)، تم إنشاؤها كنسخة مفرعة ومتطورة من [3x-ui](https://github.com/MHSanaei/3x-ui) (الإصدار v3.6.0) مع دعم أصلي لـ **AmneziaWG (AWG)**. يعمل AWG كـ Sidecar على مستوى واجهة النواة (Kernel Interface) — بشكل متطابق تمامًا مع البنية التي يستخدمها المشروع الأصلي لـ MTProto (mtg): تتولى اللوحة إدارة دورة الحياة وحساب حركة المرور، بينما يقوم Xray بتوجيه الترافيك اختياريًا.

### الميزات الرئيسية

#### 🛡️ تحسينات AmneziaWG (AWG)
- **AWG Inbounds (المنافذ الواردة)** — Sidecar للنواة عبر `awg-quick`: الإنشاء، إعادة التوفيق (Reconcile) كل 10 ثوانٍ، تنظيف الواجهات اليتيمة، ومثبت موديل النواة عبر DKMS.
- **AWG Outbounds (المنافذ الصادرة / وضع العميل)** — يمكن للوحة الاتصال بسيرفر AmneziaWG رئيسي: تبويب خاص في قسم Xray، لصق ملفات `.conf` جاهزة، وإدارة واجهة النواة `awgo-{id}` عبر دورة التوفيق. يتم إنعاش `freedom` outbound مع `sockopt.interface` في إعدادات Xray لتوجيه الترافيك عبر الـ VPN الرئيسي.
- **التعتيم (Obfuscation)** — بروفايلات Lite/Standard/Pro (تشمل Jc/Jmin/Jmax/S1–S4/H1–H4) ومحاكاة حزم CPS لـ: TLS، DNS، SIP، و QUIC.
- **بصمات TLS للمتصفحات** — دعم Chrome (GREASE)، و Firefox 120+ (ترتيب NSS و padding)، و Safari 16+ (ترتيب Apple و TLS 1.1) لـ TLS و QUIC.
- **التقاط التوقيع المباشر** — تحويل المصافحة الحقيقية لـ QUIC من نطاق فرعي إلى قيم I1–I5.
- **إدارة العملاء** — أكواد QR، تحميل ملفات `.conf`، وحساب الترافيك لكل عميل (`awg show transfer`).
- **نمطان للتوجيه**:
  - **Kernel NAT** — توجيه مباشر من النواة؛ تتعافى قواعد NAT تلقائيًا بعد إعادة تشغيل iptables.
  - **Route through Xray** — يمر الترافيك عبر واجهة TUN والتوجيه حسب السياسة والـ sniffing، ليمر عبر مسار التوجيه الكامل لـ Xray.
- **تشخيص دائم في اللوحة** — فحص بنقرة واحدة لـ حالة الواجهة، ip_forward، العملاء المتصلين، وقواعد NAT/TUN.

#### 🚀 الميزات الأساسية لـ 3x-ui
- **المنافذ الواردة متعددة البروتوكولات** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (المختلط), و TUN.
- **بروتوكولات النقل والأمان الحديثة** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade, و XHTTP المحمية بـ TLS, XTLS, و REALITY.
- **خاصية Fallbacks** — تقديم بروتوكولات متعددة على منفذ واحد (مثل VLESS و Trojan على 443).
- **إدارة العملاء** — حصص الترافيك، تواريخ الانتهاء، حدود IP، حالة الاتصال المباشر، وروابط المشاركة/QR.
- **إحصائيات حركة المرور** — لكل منفذ، عميل، وخيار صادر.
- **دعم سيرفرات متعددة (Multi-node)** — إدارة وتوسيع نطاق السيرفرات من لوحة واحدة.
- **التوجيه والـ Outbounds** — WARP, NordVPN, قواعد التوجيه المخصصة وموازنات الأحمال.
- **سيرفر الاشتراكات المدمج** مع دعم القوالب المخصصة.
- **بوت تلجرام** للمراقبة والإدارة عن بُعد.
- **RESTful API** مع توثيق Swagger مدمج.
- **قواعد بيانات مرنة** — SQLite (افتراضي) أو PostgreSQL.
- **تكامل Fail2ban** لتطبيق حدود IP لكل عميل.

### لقطات الشاشة

<details>
<summary>اضغط للتوسيع</summary>

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

## التثبيت السريع

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

يقوم بتثبيت اللوحة من [أحدث إصدار](https://github.com/AlexeyLCP/lucx-ui/releases/latest)، وخدمة systemd، و Xray-core و mtg، وبناء موديل النواة لـ AmneziaWG عبر DKMS (`bin/install-awg-module.sh`).

بعد التثبيت، قم بتبعية `x-ui` لفتح قائمة الإدارة.

### التثبيت التلقائي (Non-interactive)

يدعم مثبت البرنامج **الوضع التلقائي** لـ cloud-init. عند ضبط `XUI_NONINTERACTIVE=1` يتم التثبيت بدون أسئلة وتُحفظ البيانات في `/etc/x-ui/install-result.env`. انظر [`deploy/`](deploy/).

## المنصات المدعومة

**أنظمة التشغيل:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine, و Windows.

**المعمارية:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## خيارات قواعد البيانات

يدعم 3X-UI نوعين من قواعد البيانات:

- **SQLite** (افتراضي) — ملف واحد `/etc/x-ui/x-ui.db`.
- **PostgreSQL** — يوصى به للأعداد الكبيرة من العملاء والسيرفرات المتعددة.

المتغيرات البيئية في `/etc/default/x-ui`:
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Docker

لاستخدام PostgreSQL في Docker، احذف علامات التعليق عن `XUI_DB_*` في `docker-compose.yml` ثم نفذ:
```bash
docker compose --profile postgres up -d
```

## المتغيرات البيئية

| المتغير | الوصف | الافتراضي |
| --- | --- | --- |
| `XUI_DB_TYPE` | نوع قاعدة البيانات: `sqlite` أو `postgres` | `sqlite` |
| `XUI_DB_DSN` | نص الاتصال بـ PostgreSQL (عند `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | مجلد ملف SQLite | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | تفعيل Fail2ban لحظر IP | `true` |
| `XUI_LOG_LEVEL` | مستوى السجلات (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | تفعيل مراقب صحة النفق | `false` |

## التراخيص والشروط

يعمل هذا المشروع تحت **ترخيصين** (التفاصيل في [LICENSING.md](LICENSING.md)):

| الجزء | الترخيص |
|---|---|
| كود 3x-ui الأصلي | **GPL-3.0** (وفقًا لمتطلبات المشروع الرئيسي) |
| مكونات LucX الخاصة (`internal/awg/`, `internal/lucx/`, واجهة AWG، والسكربتات) | **PolyForm Noncommercial 1.0.0** |

**مجاني** للاستخدام الشخصي، غير التجاري، العلمي، البحثي والتعليمي. **الاستخدام التجاري** (إعادة بيع VPN، اللوحات المدفوعة) يتطلب إذنًا كتابيًا صريحًا من المؤلف: يمكنك فتح [issue](https://github.com/AlexeyLCP/lucx-ui/issues) أو التواصل مع مالك المستودع.

## المساهمة في المشروع

نرحب بمساهماتكم. يُرجى قراءة [دليل المساهمة](/CONTRIBUTING.md) قبل فتح issue أو طلب سحب.

## الشكر والتقدير

### المختبرون والمساهمون
- **VladufQa** — الاختبارات الميدانية على سيرفرات VPS (ruvds): المصافحات الأولى، الترافيك، وتقارير التوجيه.
- **Kirill Rudenko** — الاختبارات (runode) و **PR #13**: خاصية needRestart لـ AWG، التوجيه بالسياسات iif، الجداول والبوابات المنفصلة، وإعادة استعادة المسارات والـ sniffing.
- **302ba (Alex)** — **PR #24**: إصلاح فقدان حقول العميل أثناء تحليل مخطط Zod.
- **alireza0** — مساهم في المشروع الأصلي.
- فريق **3x-ui** — على القاعدة الممتازة وبنية الـ Sidecar.

### المصادر والإلهام
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — أساس المشروع (GPL-3.0) وبنية MTProto sidecar.
- [AmneziaVPN](https://github.com/amnezia-vpn) — بروتوكول AmneziaWG وموديل النواة.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — نمط PostUp NAT (MASQUERADE + FORWARD)، مولدات QUIC Initial، وطريقة تثبيت DKMS.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — التقاط توقيع QUIC (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) و [refraction-networking/utls](https://github.com/refraction-networking/utls) — بصمات TLS لمتصفحي Firefox/Safari.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) و [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) — قواعد التوجيه.

### أدوات المجتمع
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (الترخيص: **MIT**): إدارة إعدادات اللوحة كرمز.

## ☕ دعم المشروع

مشروع LucX-UI مجاني للاستخدام الشخصي وغير التجاري. إذا كانت اللوحة توفر لك الوقت، يمكنك دعم التطوير:

| الطريقة | التفاصيل |
|---|---|
| 🇷🇺 **YooMoney** (روبل، روسيا) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

التبرعات هي شكر وتقدير وليست عملية شراء: فهي لا تمنح ترخيصاً تجارياً ولا تغير شروط [LICENSING.md](LICENSING.md).

## النجوم عبر الزمن

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)

<!-- END LUCX-HOOK -->
