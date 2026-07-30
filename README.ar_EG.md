<!-- LUCX-HOOK: LucX-UI fork README — AR lead section, license, credits, sources. Keep in sync with LICENSING.md and AGENTS.md. -->
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

مشروع **LucX-UI** هو مشروع مفرع (Fork) من [3x-ui](https://github.com/MHSanaei/3x-ui) (الإصدار v3.6.0) يقدم دعمًا أصليًا لـ **AmneziaWG (AWG)**. يعمل AWG كـ Sidecar على مستوى واجهة النواة (Kernel Interface) — بشكل متطابق تمامًا مع البنية التي يستخدمها المشروع الأصلي لـ MTProto (mtg): تتولى اللوحة إدارة دورة الحياة وحساب حركة المرور، بينما يقوم Xray بتوجيه الترافيك اختياريًا.

### الميزات المضافة والجاهزة للاستخدام

- ✅ **AWG Inbounds (المنافذ الواردة)** — Sidecar للنواة عبر `awg-quick`: الإنشاء، إعادة التوفيق (Reconcile) كل 10 ثوانٍ، تنظيف الواجهات اليتيمة، ومثبت موديل النواة عبر DKMS.
- ✅ **AWG Outbounds (المنافذ الصادرة / وضع العميل)** — يمكن للوحة الاتصال بسيرفر AmneziaWG رئيسي: تبويب خاص في قسم Xray، لصق ملف `.conf` جاهز، وإدارة واجهة النواة `awgo-{id}` عبر دورة التوفيق. يتم إنعاش `freedom` outbound مع `sockopt.interface` في إعدادات Xray حتى تتوجه قواعد التوجيه وموازنات الأحمال عبر الـ VPN الرئيسي.
- ✅ **التعتيم (Obfuscation)** — بروفايلات Lite/Standard/Pro (تشمل Jc/Jmin/Jmax/S1–S4/H1–H4) ومحاكاة حزم CPS لـ: TLS، DNS، SIP، و QUIC.
- ✅ **بصمات TLS للمتصفحات** — دعم Chrome (GREASE)، و Firefox 120+ (ترتيب NSS و padding)، و Safari 16+ (ترتيب Apple و TLS 1.1) لـ TLS و QUIC.
- ✅ **التقاط التوقيع المباشر** — تحويل المصافحة الحقيقية لـ QUIC من نطاق فرعي إلى قيم I1–I5.
- ✅ **إدارة العملاء** — أكواد QR، تحميل ملفات `.conf`، وحساب الترافيك لكل عميل (`awg show transfer`).
- ✅ **نمطان للتوجيه:**
  - **Kernel NAT** — توجيه مباشر من النواة؛ تتعافى قواعد NAT تلقائيًا عند إعادة تشغيل iptables عبر دورة التوفيق.
  - **التوجيه عبر Xray (Route through Xray)** — يمر الترافيك عبر واجهة TUN والتوجيه حسب السياسة (policy routing) والـ sniffing، ليمر عبر مسار التوجيه الكامل لـ Xray (قواعد النطاقات/geosite، موازنات الأحمال، والـ outbounds المتسلسلة).
- ✅ **تشخيص دائم في اللوحة** — زر تشخيص بنقرة واحدة في نموذج الـ Inbound: فحص حالة الواجهة، ip_forward، العملاء المتصلين، وقواعد NAT/TUN في لحظة.
- ✅ **مُجرب ومختبر عملياً** — تم اختباره على سيرفرات VPS: المصافحة، ICMP، HTTPS، حساب الترافيك، والتسلسل يعمل بكفاءة في كلا نمطي التوجيه.

### التثبيت

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

يقوم بتثبيت اللوحة من [أحدث إصدار](https://github.com/AlexeyLCP/lucx-ui/releases/latest)، وخدمة systemd، و Xray-core و mtg (من الإصدار الرئيسي لـ 3x-ui)، وبناء موديل النواة لـ AmneziaWG عبر DKMS (`bin/install-awg-module.sh`).

### الترخيص

يعمل هذا المشروع تحت **ترخيصين** (التفاصيل في [LICENSING.md](LICENSING.md)):

| الجزء | الترخيص |
|---|---|
| كود 3x-ui الأصلي | **GPL-3.0** (وفقًا لمتطلبات المشروع الرئيسي) |
| مكونات LucX الخاصة (`internal/awg/`, `internal/lucx/`, واجهة AWG، والسكربتات) | **PolyForm Noncommercial 1.0.0** |

عملياً: **مجاني** للاستخدام الشخصي، غير التجاري، العلمي، البحثي والتعليمي. **الاستخدام التجاري** (إعادة بيع VPN، اللوحات المدفوعة، الدمج في منتج تجاري) يتطلب إذنًا كتابيًا صريحًا من المؤلف — يمكنك فتح [issue](https://github.com/AlexeyLCP/lucx-ui/issues) أو التواصل مع مالك المستودع. تضمن ترويسات `SPDX-License-Identifier` تحديد الحدود بوضوح: عدم وجود الترويسة يعني GPL-3.0.

### شكر وتقدير

- **VladufQa** — الاختبارات الميدانية على سيرفرات VPS (ruvds): المصافحات الأولى، الترافيك، والتقارير الفنية للتوجيه.
- **Kirill Rudenko** — الاختبارات (runode) و **PR #13**: خاصية needRestart لـ AWG، التوجيه بالسياسات iif، الجداول والبوابات المنفصلة، وإعادة استعادة المسارات والـ sniffing.
- **302ba (Alex)** — **PR #24**: إصلاح فقدان حقول العميل أثناء تحليل مخطط Zod.
- فريق **3x-ui** — على القاعدة الممتازة وبنية الـ Sidecar التي قمنا بمحاكاتها.

### المصادر والإلهام

- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — أساس المشروع (GPL-3.0) وبنية MTProto sidecar.
- [AmneziaVPN](https://github.com/amnezia-vpn) — بروتوكول AmneziaWG وموديل النواة.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — نمط PostUp NAT (MASQUERADE + FORWARD)، مولدات QUIC Initial، وطريقة تثبيت DKMS.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — التقاط توقيع QUIC (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) و [refraction-networking/utls](https://github.com/refraction-networking/utls) — بصمات TLS لمتصفحي Firefox/Safari المستخدمة في بروفايلات ClientHello.

### ☕ دعم المشروع

مشروع LucX-UI مجاني للاستخدام الشخصي وغير التجاري. إذا كانت اللوحة توفر لك الوقت، يمكنك دعم التطوير:

| الطريقة | التفاصيل |
|---|---|
| 🇷🇺 **YooMoney** (روبل، روسيا) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

التبرعات هي شكر وتقدير وليست عملية شراء: فهي لا تمنح ترخيصاً تجارياً ولا تغير شروط [LICENSING.md](LICENSING.md).

---

*التوثيق الأصلي لـ **3x-ui** باللغة العربية محفوظ أدناه.*

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

**3X-UI** هي لوحة تحكم ويب متقدمة ومفتوحة المصدر لإدارة خوادم [Xray-core](https://github.com/XTLS/Xray-core). توفّر واجهة نظيفة ومتعددة اللغات لنشر وتكوين ومراقبة مجموعة واسعة من بروتوكولات الوكيل وVPN — من خادم VPS واحد إلى عمليات النشر متعددة العقد.

تم بناء 3X-UI كنسخة محسّنة (fork) من مشروع X-UI الأصلي، وتضيف دعمًا أوسع للبروتوكولات، واستقرارًا محسّنًا، ومحاسبة للترافيك لكل عميل، والعديد من ميزات تحسين تجربة الاستخدام.

> [!IMPORTANT]
> هذا المشروع مخصص للاستخدام الشخصي فقط. يرجى عدم استخدامه لأغراض غير قانونية أو في بيئة إنتاجية.

## الميزات

- **اتصالات واردة متعددة البروتوكولات** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، Hysteria2، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **وسائل نقل وأمان حديثة** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، مؤمَّنة بـ TLS و XTLS و REALITY.
- **Fallback** — تقديم عدة بروتوكولات على منفذ واحد (مثل VLESS و Trojan على المنفذ 443) باستخدام ميزة fallback في Xray.
- **إدارة لكل عميل** — حصص الترافيك، تواريخ انتهاء الصلاحية، حدود IP، حالة الاتصال المباشرة، وروابط مشاركة وأكواد QR واشتراكات بنقرة واحدة.
- **إحصائيات الترافيك** — لكل اتصال وارد، ولكل عميل، ولكل اتصال صادر، مع عناصر تحكم لإعادة التعيين.
- **دعم العقد المتعددة** — إدارة وتوسيع عبر عدة خوادم من لوحة واحدة.
- **الاتصالات الصادرة والتوجيه** — WARP، NordVPN، قواعد توجيه مخصصة، موازنات تحميل، وتسلسل الوكلاء الصادرة.
- **خادم اشتراك مدمج** بصيغ إخراج متعددة و[قوالب صفحات مخصصة](docs/custom-subscription-templates.md).
- **روبوت تيليجرام** للمراقبة والإدارة عن بُعد.
- **واجهة RESTful API** مع توثيق Swagger داخل اللوحة.
- **تخزين مرن** — SQLite (افتراضي) أو PostgreSQL.
- **13 لغة لواجهة المستخدم** مع سمات داكنة وفاتحة.
- **تكامل مع Fail2ban** لفرض حدود IP لكل عميل.

## لقطات الشاشة

<details>
<summary>انقر للتوسيع</summary>

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

## البدء السريع

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

لتثبيت إصدار محدد، أضِف وسمه (مثل `v3.4.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

لتثبيت بنية **dev** المتجددة (أحدث إصدار أولي لكل التزام (commit) من `main`، وليس إصدارًا مستقرًا)، مرّر `dev-latest`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

أثناء التثبيت، يتم إنشاء اسم مستخدم وكلمة مرور ومسار وصول عشوائية. بعد التثبيت، شغّل `x-ui` لفتح قائمة الإدارة، حيث يمكنك بدء/إيقاف الخدمة، وعرض أو إعادة تعيين بيانات تسجيل الدخول، وإدارة شهادات SSL، والمزيد.

للحصول على الوثائق الكاملة، يرجى زيارة [ويكي المشروع](https://github.com/MHSanaei/3x-ui/wiki).

### التثبيت غير التفاعلي

يعمل المثبِّت أيضًا **بشكل غير تفاعلي** لـ cloud-init.
عيّن `XUI_NONINTERACTIVE=1` (أو مرّره عبر أنبوب دون TTY) وسيتولى التثبيت من البداية إلى النهاية
دون أي مطالبات، مُنشئًا بيانات اعتماد عشوائية وكاتبًا إياها في
`/etc/x-ui/install-result.env`. راجع [`deploy/`](deploy/) لـ:

- [بيانات مستخدم cloud-init](deploy/cloud-init/) — تثبيت غير تفاعلي على أي سحابة (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [ملاحظات Hetzner Cloud](deploy/marketplace/hetzner/) — نشر يعتمد على cloud-init على Hetzner

## المنصات المدعومة

**أنظمة التشغيل:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap)، Alpine و Windows.

**المعماريات:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## خيارات قاعدة البيانات

يدعم 3X-UI خلفيتين (backends) يتم اختيارهما أثناء التثبيت:

- **SQLite** (افتراضي) — ملف واحد في `/etc/x-ui/x-ui.db`. بدون إعداد، مثالي لعمليات النشر الصغيرة والمتوسطة.
- **PostgreSQL** — موصى به لأعداد العملاء الكبيرة أو الإعدادات متعددة العقد. يمكن للمثبِّت تثبيت PostgreSQL محليًا لك، أو قبول DSN لخادم موجود.

في وقت التشغيل، يتم اختيار الخلفية عبر متغيرات البيئة (يكتبها المثبِّت لك في `/etc/default/x-ui`):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### ترحيل تثبيت SQLite موجود إلى PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# ثم عيّن XUI_DB_TYPE و XUI_DB_DSN في /etc/default/x-ui وأعد التشغيل:
systemctl restart x-ui
```

يبقى ملف SQLite الأصلي دون تغيير؛ احذفه يدويًا بعد التحقق من الخلفية الجديدة.

### Docker

يستمر الأمر الافتراضي `docker compose up -d` في استخدام SQLite. للتشغيل مع خدمة PostgreSQL المرفقة، أزِل التعليق عن سطري متغيرات البيئة `XUI_DB_*` في `docker-compose.yml` وشغّل باستخدام البروفايل:

```bash
docker compose --profile postgres up -d
```

تتضمن الصورة Fail2ban (مُفعَّل افتراضيًا) لفرض **حدود IP** لكل عميل. يحظر Fail2ban المخالفين باستخدام `iptables`، الذي يتطلب صلاحية `NET_ADMIN`. يمنح `docker-compose.yml` هذه الصلاحية مسبقًا عبر `cap_add`؛ إذا شغّلت الحاوية باستخدام `docker run` بدلاً من ذلك، فأضِف الصلاحيات بنفسك، وإلا فسيتم تسجيل عمليات الحظر دون تطبيقها أبدًا:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## متغيرات البيئة

| المتغير | الوصف | الافتراضي |
| --- | --- | --- |
| `XUI_DB_TYPE` | خلفية قاعدة البيانات: `sqlite` أو `postgres` | `sqlite` |
| `XUI_DB_DSN` | سلسلة اتصال PostgreSQL (عندما `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | مجلد ملف قاعدة بيانات SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | الحد الأقصى للاتصالات المفتوحة (تجمّع PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | الحد الأقصى للاتصالات الخاملة (تجمّع PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | مسار URI الأولي للوحة الويب | `/` |
| `XUI_ENABLE_FAIL2BAN` | تفعيل فرض حدود IP المعتمد على Fail2ban | `true` |
| `XUI_LOG_LEVEL` | مستوى السجل (`debug`، `info`، `warning`، `error`) | `info` |
| `XUI_DEBUG` | تفعيل وضع التصحيح | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | تفعيل مراقب صحة النفق (يفحص عنوان URL ويعيد تشغيل xray بعد فشل متكرر؛ إعادة التشغيل تقطع جميع العملاء) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | الوكيل الذي يُرسَل عبره الفحص؛ وجّهه إلى اتصال xray وارد محلي ليختبر الفحص النفق (مثل `socks5://127.0.0.1:1080`). القيمة الفارغة تعني أن الفحص يتحقق فقط من اتصال المضيف | — |
| `XUI_TUNNEL_HEALTH_URL` | عنوان URL الذي يُفحَص لمعرفة صحة النفق | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | الفترة بين عمليات الفحص | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | مهلة كل عملية فحص | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | عدد حالات الفشل المتتالية قبل تشغيل إعادة التشغيل | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | الحد الأدنى للتأخير بين عمليات إعادة التشغيل المتتالية | `5m` |

## اللغات المدعومة

تتوفر واجهة اللوحة بـ 13 لغة:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## المساهمة

المساهمات مرحب بها. يرجى قراءة [دليل المساهمة](/CONTRIBUTING.md) قبل فتح مشكلة (issue) أو طلب سحب (pull request).

## شكر خاص إلى

- [alireza0](https://github.com/alireza0/)

## الاعتراف

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (الترخيص: **GPL-3.0**): _قواعد توجيه v2ray/xray و v2ray/xray-clients المحسنة مع النطاقات الإيرانية المدمجة وتركيز على الأمان وحظر الإعلانات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (الترخيص: **GPL-3.0**): _يحتوي هذا المستودع على قواعد توجيه V2Ray محدثة تلقائيًا بناءً على بيانات النطاقات والعناوين المحظورة في روسيا._

## أدوات المجتمع

أدوات وتكاملات بناها المجتمع حول 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (الترخيص: **MIT**): _إدارة الاتصالات الواردة والعملاء وإعدادات اللوحة وتكوين Xray كرمز باستخدام Terraform / OpenTofu._

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
