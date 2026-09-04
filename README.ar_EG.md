<!-- LUCX-HOOK: LucX-UI fork README — Streamlined AR README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **لوحة Xray متقدمة** — AmneziaWG (نواة + أصلي، حتى 3.1)، استيراد AWG الموجود، أنفاق تحت الإشراف و sidecar outbounds (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy)، اشتراكات Clash / Amnezia `vpn://` / Happ، RoscomVPN geo وتوجيه Happ.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
</p>

<p align="center">
  <a href="README.en_US.md">English</a> |
  <a href="README.md">Русский</a> |
  <a href="README.fa_IR.md">فارسی</a> |
  <b>العربية</b> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **مخصص للاستخدام الشخصي، غير التجاري، العلمي، البحثي والتعليمي فقط.** الاستخدام التجاري — بما فيه إعادة بيع VPN أو اللوحات المدفوعة — يتطلب إذناً كتابياً صريحاً بموجب ترخيص PolyForm Noncommercial 1.0.0.

---

## ⚡ التثبيت السريع

تثبيت بأمر واحد على **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch إلخ)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

تثبيت اختياري من Yandex (SourceCraft) عندما يتعذّر الوصول إلى GitHub. بلا توكنات ولا git — اللوحة وgeo والسكربتات في حزمة واحدة:

```bash
mkdir -p /tmp/lucx-dist && curl -fsSL https://codeload.sourcecraft.tech/alexeylcp/lucx-ui/tarball/refs/heads/dist | tar -xz --strip-components=1 -C /tmp/lucx-dist && sudo bash /tmp/lucx-dist/install.sh --yandex
```

لاحقاً يستخدم `x-ui update` المصدر نفسه (`/etc/x-ui/install-source`).

<details>
<summary><b>🛠️ التثبيت والتكوين المتقدم (Cloud-Init, Docker, PostgreSQL, متغيرات البيئة)</b></summary>

### التثبيت التلقائي (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
تُحفظ بيانات الدخول في `/etc/x-ui/install-result.env`.

### Docker مع PostgreSQL
```bash
docker compose --profile postgres up -d
```

### متغيرات البيئة الرئيسية (`/etc/default/x-ui`)
| المتغير | الوصف | الافتراضي |
| --- | --- | --- |
| `XUI_DB_TYPE` | محرك قاعدة البيانات (`sqlite` أو `postgres`) | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL DSN | — |
| `XUI_ENABLE_FAIL2BAN` | تفعيل Fail2ban لحظر IP | `true` |
| `XUI_LOG_LEVEL` | مستوى السجلات (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ لماذا LucX-UI؟

[3x-ui](https://github.com/MHSanaei/3x-ui) لوحة متعددة البروتوكولات ممتازة بواجهة React 19 + Ant Design 6. يحافظ LucX-UI على كل ما في 3x-ui ويضيف ما لا يملكه upstream: **AmneziaWG للنواة** (إلى جانب `amneziawg` الأصلي لدى upstream)، **استيراد AWG الموجود**، **sidecars للأنفاق** (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy)، **اشتراكات أوسع** (Clash Meta AWG، Amnezia `vpn://`، Happ) و**حزم RoscomVPN geo + ملفات Happ** (متصفح geodata في upstream منذ [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0):

<details>
<summary><b>مقارنة مع 3x-ui</b></summary>

| الميزة | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (sidecar للنواة عبر `awg-quick`) | ✗ | ✓ |
| AmneziaWG الأصلي (`amneziawg`، مساحة المستخدم) | ✓ | ✓ |
| استيراد AWG الموجود على المضيف (awg-multi / toolza3 / Docker) | ✗ | ✓ |
| Kernel AWG بدون وحدة → amneziawg-go مدمج | ✗ | ✓ |
| سرعة AWG الحية للعملاء والـ inbounds في اللوحة | ✗ | ✓ |
| AWG CPS تعتيم (TLS / DNS / SIP / QUIC + بصمات المتصفح) | ✗ | ✓ |
| AWG outbound — تسلسل VPN إلى سيرفرات AWG الرئيسية (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1 (`RandomTrailers` / `DisableCookies`، ضد DPI) | ✗ | ✓ |
| إعدادات إصدار إعداد العميل المسبقة (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| تشخيص AWG داخل اللوحة (التوجيه / NAT / الأقران / المصافحات) | ✗ | ✓ |
| Sidecar نفق NaiveProxy (Caddy + forward_proxy، تحت إشراف اللوحة) | ✗ | ✓ |
| بيانات اعتماد NaiveProxy لكل عميل + `naive+https://` في الاشتراكات | ✗ | ✓ |
| NaiveProxy → توجيه Xray (جسر SOCKS loopback، اختياري) | ✗ | ✓ |
| Sidecar olcRTC (WebRTC عبر غرف meet، تحت الإشراف) | ✗ | ✓ |
| Sidecar qWDTT (WireGuard عبر VK TURN، تحت الإشراف) | ✗ | ✓ |
| Sidecar mieru (`mita`، ترافيك لكل عميل، تحت الإشراف) | ✗ | ✓ |
| Sidecar TrustTunnel (بروتوكول AdGuard VPN، يشبه HTTPS، تحت الإشراف) | ✗ | ✓ |
| Sidecar outbounds (عميل Naive / mieru / TrustTunnel → SOCKS، التوجيه والمجمّعات) | ✗ | ✓ |
| AWG في Clash Meta + اشتراك Amnezia `/awg/` (`.conf` / `vpn://`) | ✗ | ✓ |
| Geodata browser — اختيار فئات geosite/geoip من اللوحة | ✓ | ✓ |
| حزمة RoscomVPN geo (`geoip/geosite_ROSCOM.dat`) | ✗ | ✓ |
| ملفات توجيه Happ (RoscomVPN deeplink + مخصص) | ✗ | ✓ |
| روابط outbound الذكية للـ Cluster | ✗ | ✓ |
| واجهة أمامية React 19 + AntD 6 + Vite 8 + Zod 4 | ✓ | ✓ (موروث) |
| جميع بروتوكولات Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| inbound وكيل ويب تلغرام (`tproxy`، t.me/webproxy) | ✗ | ✓ |
| مزامنة upstream بلا احتكاك (عزل LUCX-HOOK) | — | ✓ |

</details>

Sidecar للنواة (مثل `mtg` لـ MTProto في 3x-ui) يعني أن AWG يعمل كواجهة نواة حقيقية — وليس shim لمساحة المستخدم — مما يتيح لـ Xray توجيه الترافيك المفكوك عبر inbound الـ TUN الخاص به، فتحصل على القوة الكاملة للتوجيه والشمّ وقواعد النطاقات لدى Xray على ترافيك AWG. بلا وحدة — نفس inbound LucX `awg` يعمل على amneziawg-go المدمج. بروتوكول upstream الأصلي `amneziawg` يبقى في اللوحة بجانبه.

---

## 🌟 حول LucX-UI

**LucX-UI** نسخة محسّنة من [3x-ui](https://github.com/MHSanaei/3x-ui) (متزامنة مع upstream **v3.7.0**). فوق بروتوكولات Xray الافتراضية: **AmneziaWG** بوضعَين — sidecar للنواة `awg` (كـ MTProto/`mtg`) و`amneziawg` الأصلي لدى upstream، حتى **AWG 3.1**؛ **استيراد** awg-multi / toolza3 / Docker؛ **أنفاق تحت إشراف اللوحة** (NaiveProxy، olcRTC، qWDTT، mieru، TrustTunnel)، **اشتراكات موسّعة** (Clash Meta AWG، Amnezia `/awg/` + `vpn://`، Happ)، **وكيل ويب تلغرام** (`tproxy`) و**geo RoscomVPN الافتراضي** (متصفح الفئات مشترك مع upstream v3.7.0). توافق 100% مع upstream عبر عزل `LUCX-HOOK`.

<details>
<summary><b>🛡️ ميزات AmneziaWG (AWG)</b></summary>

- **AWG Inbounds & Outbounds** — Sidecar للنواة (`awg-quick`)، اتصال بوضع العميل إلى سيرفرات AWG الرئيسية (`awgo-{id}`)، حلقة توفيق تلقائية كل 10 ثوانٍ، ومثبت موديل النواة عبر DKMS.
- **محرّكان** — `AmneziaWG (kernel)` عبر `awg-quick` عند وجود الوحدة، و`amneziawg` الأصلي لدى upstream. بلا وحدة — تعمل inboundات LucX `awg` على amneziawg-go المدمج (SOCKS إلى Xray)؛ مسار النواة لا يتغيّر عندما تكون الوحدة موجودة.
- **استيراد AWG الموجود** — لافتة في Inbounds: awg-multi / toolza3 / Docker Amnezia. تُنسَخ المفاتيح وIP والمنفذ والتعتيم كما هي؛ تُعاد تسمية واجهة النواة في مكانها (المصافحات تبقى).
- **سرعة حية** — أعمدة السرعة في Clients / Inbounds لـ AWG (إحصاءات Xray لا تراه).
- **تعتيم متقدم** — بروفايلات Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4)، محاكاة حزم CPS (TLS, DNS, SIP, QUIC)، وبصمات TLS للمتصفحات (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — حماية هيدر AmneziaWG 3 بمفاتيح 32-بايت مُولّدة تلقائياً؛ سقف الإصدار من جانب السيرفر يقود إصدار الميزات لكل عميل.
- **AWG 3.1** — `RandomTrailers` (ذيل حزمة عشوائي، ضد DPI حسب الحجم) و`DisableCookies`؛ تُرقّى وحدة النواة والأدوات تلقائياً إلى v3.1 عند تحديث اللوحة.
- **إعدادات إصدار العميل المسبقة** — إنشاء إعدادات العميل لـ AWG 1.5 / 2 / 3 / 3.1 من inbound واحد — اختر الصيغة التي تفهمها تطبيقة عميلك.
- **التقاط التوقيع المباشر** — تحويل مصافحات QUIC الحقيقية من نطاقات front إلى قيم تعتيم I1–I5.
- **التوجيه والتشخيص** — نمطا توجيه (Kernel NAT و Route through Xray مع توجيه السياسات والشمّ) + تشخيص بنقرة واحدة من داخل اللوحة.

</details>

<details>
<summary><b>🚇 Sidecar-ات الأنفاق (NaiveProxy، olcRTC، qWDTT، mieru، TrustTunnel، Telegram WEB proxy)</b></summary>

- **NaiveProxy** — Caddy مع إضافة `forward_proxy` (نسخة [klzgrad](https://github.com/klzgrad/forwardproxy)، حشو HTTP/2) يعمل كـ sidecar تحت إشراف اللوحة: Caddyfile مُولَّد، start/stop/restart مع reconcile لإحياء الأعطال، وفحص صحة ثلاثي المستويات (process → TCP → TLS).
- **بيانات اعتماد لكل عميل** — كل عميل مفعّل في اللوحة يحصل تلقائياً على زوج `basic_auth` شخصي (مشتق من سر اللوحة، دون تخزين)؛ تعطيل العميل يُلغي الاعتماد في reconcile التالي.
- **الاشتراكات** — يحمل اشتراك كل عميل رابطه الشخصي `naive+https://` إلى جانب روابط Xray/AWG (الصيغة القياسية لـ NekoBox / husi / Exclave)، مع رمز QR ومولّد كلمات مرور قوية في اللوحة.
- **تجربة اللوحة** — Auto TLS (Let's Encrypt) أو شهادتك/مفتاحك الخاصان، وضع raw-Caddyfile مع تحقق `caddy adapt`، معاينة Caddyfile، سجلات العملية، رفع/تنزيل الثنائي.
- **التوجيه عبر Xray (اختياري)** — المفتاح يجعل Caddy يتصل بالوجهات عبر جسر SOCKS loopback مخفي (`upstream socks5://127.0.0.1:…`، forward_proxy أصلي — بلا تصحيح) بالوسم `lucx-tunnel-naive`، فيحصل ترافيك NaiveProxy على التوجيه / الشمّ / قواعد النطاقات الكاملة لدى Xray (نفس نمط MTProto). الافتراضي يبقى خروجاً مباشراً.
- **olcRTC** — نفق TCP-over-WebRTC عبر غرفة مكالمة فيديو قانونية ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)).
- **qWDTT** — WireGuard عبر TURN لـ VK Calls ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)).
- **mieru** — بروكسي مقاوم للرقابة عبر بروتوكول خاص بدلاً من TLS ([enfein/mieru](https://github.com/enfein/mieru) `mita`، GPL-3.0). متعدد العملاء ببيانات HMAC لكل عميل لوحة، ترافيك وحالة اتصال لكل عميل، ورابط مشاركة `mierus://`. العملاء: mieru CLI، mihomo، Clash Verge Rev، husi، Exclave.
- **TrustTunnel** — بروتوكول AdGuard VPN ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)، Apache-2.0): الترافيك لا يمكن تمييزه عن HTTPS (HTTP/1.1 + HTTP/2 + QUIC). يعيد استخدام شهادة ACME الخاصة باللوحة (يتطلب نطاقاً بشهادة صادرة) ويُصدر deep-link `tt://?` لعملاء Flutter / CLI.
- **وكيل ويب تلغرام (`tproxy`)** — `tproxy-server` + MTProxy الرسمي + Caddy TLS reverse_proxy على `hostname:443`، رابط المشاركة `t.me/webproxy`. التوجيه عبر Xray **موقوف** حالياً (خروج MTProxy مباشر؛ انظر lucx.211).
- **Sidecar outbounds** — وضع عميل Naive / mieru / TrustTunnel: الصق رابط المشاركة (`naive+https://` / `mierus://` / `tt://`)، يظهر الوسم في قواعد التوجيه ومجمّعات الموازنة (مثل AWG outbound). التعطيل = blackhole (fail-closed، لا يتسرّب إلى `direct`). ثنائيات العميل في tar.gz.

</details>

<details>
<summary><b>📦 الاشتراكات و geodata وتوجيه العملاء</b></summary>

- **اشتراك Amnezia** — `/awg/{subId}` يُرجع `.conf` صرفاً أو `vpn://…`.
- **AWG في Clash Meta** — peers عبر `amnezia-wg-option`.
- **Geodata browser** — تصفح `geoip*.dat` / `geosite*.dat` من واجهة التوجيه (في upstream منذ [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0، [STRENCH0](https://github.com/STRENCH0)).
- **حزمة RoscomVPN geo** — `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip) / [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)).
- **ملفات Happ** — Settings → Happ: deeplink RoscomVPN ([roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

</details>

<details>
<summary><b>🚀 الميزات الأساسية لـ 3x-ui</b></summary>

- **البروتوكولات:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **الأمان والنقل:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **الإدارة:** حصص الترافيك، حدود IP (Fail2ban)، حالة الاتصال، الاشتراكات، بوت تلجرام، REST API، السيرفرات المتعددة، SQLite / PostgreSQL.

</details>

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

## 🔄 الانتقال من 3x-ui وAWG الموجود

يشارك LucX-UI نفس قاعدة مخطط قاعدة بيانات Xray-core / SQLite (أو PostgreSQL) مع 3x-ui، وتُنشأ جداول AWG تلقائياً عند أول تشغيل. للتثبيت فوق إعداد 3x-ui موجود، انسخ قاعدة بياناتك احتياطياً أولاً ثم شغل أمر التثبيت القياسي:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

تُبنى موديل نواة AWG تلقائياً بواسطة المثبّت (`bin/install-awg-module.sh`, DKMS). بعد التثبيت، شغّل `x-ui` في الطرفية للتأكد من إصدار موديل نواة AWG وابدأ بإضافة AWG inbounds من اللوحة.

**بعد التثبيت:** نقاط نهاية الاشتراك (`/sub/`، `/json/`، `/clash/`، `/awg/`) تستمع على **منفذ منفصل** (الافتراضي **2096**)، وليس منفذ اللوحة — يجب أن يمرّر الـ reverse proxy ذلك أيضاً. أبقِ مجموعات geo المخصّصة تحت **اسم ملف منفصل** — الأسماء الافتراضية (`geoip.dat` / `geosite.dat` و`_IR` / `_RU` / `_ROSCOM`) تُستبدل عند تحديث geofile.

<details>
<summary><b>من AWG الموجود على المضيف</b></summary>

إذا كان السيرفر يشغّل مسبقاً **awg-multi** أو **toolza3** أو **Docker Amnezia**، فاللوحة **لا تحذف** واجهات `awg0`/`awg1` الأجنبية. تظهر في Inbounds لافتة **«استيراد AWG الموجود»**: معاينة الأقران → inbound واحد لكل واجهة. تُنسَخ المفاتيح / IP / المنفذ / التعتيم كما هي. تُعاد تسمية واجهة النواة في مكانها (`awg{id}`) — المصافحات تبقى. Userspace/Docker: أوقف المدير القديم؛ يعيد أولئك العملاء الاتصال مرة واحدة.

بلا وحدة نواة، تُرفع inboundات LucX `awg` على amneziawg-go المدمج. بروتوكول upstream الأصلي `amneziawg` متاح في اللوحة بجانبه.

</details>

---

## 📜 الترخيص والشروط

يُنشر هذا المشروع تحت **ترخيصين** للكود الخاص به، بالإضافة إلى ثنائيات/بيانات طرف ثالث وفق شروط upstream (المصفوفة الكاملة في [LICENSING.md](LICENSING.md)):

<details>
<summary><b>مصفوفة التراخيص</b></summary>

| المكون | الترخيص |
|---|---|
| كود 3x-ui الأصلي | **GPL-3.0** |
| مكونات LucX-UI (`internal/awg/`، `internal/lucx/`، صفحات LucX للواجهة) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| إضافة `forward_proxy` ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy / `bin/naive-client-*` ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/mieru-*` (`mita`، [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| وحدة AmneziaWG والأدوات ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (الوحدة؛ تُثبَّت على المضيف) |
| ملفات geo `.dat` الافتراضية (Loyalsoldier / IR / RU / ROSCOM) | شروط كل مجموعة بيانات (انظر LICENSING.md) |

ثنائيات الأنفاق هي **عمليات فرعية** — اللوحة لا تربطها. GPL الخاص بـ qWDTT ينطبق على ذلك الثنائي ومصادره، وليس على كود LucX تحت PolyForm.

</details>

---

## 🤝 الشكر والتقدير

شكراً لكل المشاريع والأشخاص في المصدر المفتوح.

<details>
<summary><b>المختبرون والمساهمون</b></summary>

- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13)), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, فريق **[3x-ui](https://github.com/MHSanaei/3x-ui)**.

</details>

<details>
<summary><b>PR-ات upstream المُرحَّلة</b></summary>

- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) geodata browser.

</details>

<details>
<summary><b>المشاريع والإلهام</b></summary>

[MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) · [amnezia-vpn](https://github.com/amnezia-vpn) · [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) / [forwardproxy](https://github.com/klzgrad/forwardproxy) · [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) · [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) · [enfein/mieru](https://github.com/enfein/mieru) · [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) · [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) · [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) · [hydraponique](https://github.com/hydraponique) RoscomVPN ([geoip](https://github.com/hydraponique/roscomvpn-geoip) / [geosite](https://github.com/hydraponique/roscomvpn-geosite) / [routing](https://github.com/hydraponique/roscomvpn-routing)) · [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) · [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules) · [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) · [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) · [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) · [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) · [refraction-networking/utls](https://github.com/refraction-networking/utls)

</details>

---

## ☕ دعم المشروع

LucX-UI مجاني للاستخدام الشخصي. **أعجبك؟ ضع ⭐** على المستودع — يساعد الآخرين على اكتشافه. التبرعات اختيارية:

<details>
<summary><b>التبرعات</b></summary>

| Method | Details |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

</details>

---

## 🛠️ للمطورين

<details>
<summary><b>المعمارية، البناء والمزامنة مع upstream (انقر للتوسعة)</b></summary>

**قاعدة المعمارية والعزل.** يعيش كود LucX بالكامل في حزم معزولة (`internal/awg/`, `internal/lucx/`)؛ والتغييرات على ملفات 3x-ui upstream تتم فقط داخل علامات `// LUCX-HOOK` / `// END LUCX-HOOK` بحيث يكون كل إصدار upstream نقلاً شبه تافه. راجع [AGENTS.md](AGENTS.md) لخريطة المعمارية الكاملة والقواعد العشر والمشكلات المعروفة وأنماط التنقيح.

**البناء من المصدر** (يتطلب Go 1.27+, Node.js 24+, gcc — Linux فقط، CGO لـ SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# pre-push hygiene: bin/check-lucx.sh  (LUCX-HOOK + internal/awg|lucx)
```

**إجراء المزامنة مع upstream** (الأساس الحالي — upstream **v3.7.0**؛ ادمج tags/main للـ upstream، لا v3.5→v3.6 القديمة):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolve block by block (see AGENTS.md Rule 8) — never blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # compare marker counts before/after to detect lost blocks
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

<!-- END LUCX-HOOK -->
