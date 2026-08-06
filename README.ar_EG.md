<!-- LUCX-HOOK: LucX-UI fork README — Streamlined AR README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **لوحة تحكم متقدمة لـ Xray و AmneziaWG** — مع اشتراكات موحّدة، وإدارة متعددة الخوادم، ودعم أصلي لـ AWG.

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
> **مخصص للاستخدام الشخصي، غير التجاري، العلمي، البحثي والتعليمي فقط.** الاستخدام التجاري — بما فيه إعادة بيع VPN أو اللوحات المدفوعة — يتطلب إذناً كتابياً صريحاً بموجب ترخيص PolyForm Noncommercial 1.0.0.

---

## ⚡ التثبيت السريع

تثبيت بأمر واحد على **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch إلخ)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

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

[3x-ui](https://github.com/MHSanaei/3x-ui) لوحة متعددة البروتوكولات ممتازة بواجهة أمامية حديثة React 19 + Ant Design 6. يحافظ LucX-UI على كل ما يقدمه 3x-ui ويضيف **دعماً أصلياً لـ AmneziaWG (AWG)** — وهي نسخة مقاومة للحجب من WireGuard — غير المتوفرة في 3x-ui:

| الميزة | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (sidecar للنواة عبر `awg-quick`) | ✗ | ✓ |
| AWG CPS تعتيم (TLS / DNS / SIP / QUIC + بصمات المتصفح) | ✗ | ✓ |
| AWG outbound — تسلسل VPN إلى سيرفرات AWG الرئيسية (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| إعدادات إصدار إعداد العميل المسبقة (1.5 / 2 / 3) | ✗ | ✓ |
| تشخيص AWG داخل اللوحة (التوجيه / NAT / الأقران / المصافحات) | ✗ | ✓ |
| روابط outbound الذكية للـ Cluster | ✗ | ✓ |
| واجهة أمامية React 19 + AntD 6 + Vite 8 + Zod 4 | ✓ | ✓ (موروث) |
| جميع بروتوكولات Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| مزامنة upstream بلا احتكاك (عزل LUCX-HOOK، 49 ملف) | — | ✓ |

Sidecar للنواة (مثل `mtg` لـ MTProto في 3x-ui) يعني أن AWG يعمل كواجهة نواة حقيقية — وليس shim لمساحة المستخدم — مما يتيح لـ Xray توجيه الترافيك المفكوك عبر inbound الـ TUN الخاص به، فتحصل على القوة الكاملة للتوجيه والشمّ وقواعد النطاقات لدى Xray على ترافيك AWG.

---

## 🌟 حول LucX-UI

**LucX-UI** نسخة مفرعة ومتطورة من [3x-ui](https://github.com/MHSanaei/3x-ui) (متزامنة حالياً مع **v3.6.0** upstream) تضيف دعماً أصلياً لـ **AmneziaWG (AWG)** كـ sidecar لواجهة النواة، محاكيةً بنية MTProto upstream. يحافظ على توافق 100% مع upstream عبر عزل صارم للكود ضمن `LUCX-HOOK`.

### 🛡️ ميزات AmneziaWG (AWG)
- **AWG Inbounds & Outbounds** — Sidecar للنواة (`awg-quick`)، اتصال بوضع العميل إلى سيرفرات AWG الرئيسية (`awgo-{id}`)، حلقة توفيق تلقائية كل 10 ثوانٍ، ومثبت موديل النواة عبر DKMS.
- **تعتيم متقدم** — بروفايلات Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4)، محاكاة حزم CPS (TLS, DNS, SIP, QUIC)، وبصمات TLS للمتصفحات (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — حماية هيدر AmneziaWG 3 بمفاتيح 32-بايت مُولّدة تلقائياً؛ سقف الإصدار من جانب السيرفر يقود إصدار الميزات لكل عميل.
- **إعدادات إصدار العميل المسبقة** — إنشاء إعدادات العميل لـ AWG 1.5 / 2 / 3 من inbound واحد — اختر الصيغة التي تفهمها تطبيقة عميلك.
- **التقاط التوقيع المباشر** — تحويل مصافحات QUIC الحقيقية من نطاقات front إلى قيم تعتيم I1–I5.
- **التوجيه والتشخيص** — نمطا توجيه (Kernel NAT و Route through Xray مع توجيه السياسات والشمّ) + تشخيص بنقرة واحدة من داخل اللوحة.

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

## 🔄 الانتقال من 3x-ui

يشارك LucX-UI نفس قاعدة مخطط قاعدة بيانات Xray-core / SQLite (أو PostgreSQL) مع 3x-ui، وتُنشأ جداول AWG تلقائياً عند أول تشغيل. للتثبيت فوق إعداد 3x-ui موجود، انسخ قاعدة بياناتك احتياطياً أولاً ثم شغل أمر التثبيت القياسي:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

تُبنى موديل نواة AWG تلقائياً بواسطة المثبّت (`bin/install-awg-module.sh`, DKMS). بعد التثبيت، شغّل `x-ui` في الطرفية للتأكد من إصدار موديل نواة AWG وابدأ بإضافة AWG inbounds من اللوحة.

---

## 📜 الترخيص والشروط

هذا المشروع منشور تحت **ترخيصين** (التفاصيل في [LICENSING.md](LICENSING.md)):

| المكون | الترخيص |
|---|---|
| كود 3x-ui الأصلي | **GPL-3.0** |
| مكونات LucX-UI (`internal/awg/`, `internal/lucx/`, الواجهة) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 الشكر والتقدير

- **المختبرون والمساهمون:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, فريق **3x-ui**.
- **المشاريع والإلهام:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ دعم المشروع

LucX-UI مجاني للاستخدام الشخصي. يمكنك دعم التطوير المستمر:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ للمطورين

<details>
<summary><b>المعمارية، البناء والمزامنة مع upstream (انقر للتوسعة)</b></summary>

**قاعدة المعمارية والعزل.** يعيش كود LucX بالكامل في حزم معزولة (`internal/awg/`, `internal/lucx/`)؛ والتغييرات على ملفات 3x-ui upstream تتم فقط داخل علامات `// LUCX-HOOK` / `// END LUCX-HOOK` بحيث يكون كل إصدار upstream نقلاً شبه تافه. راجع [AGENTS.md](AGENTS.md) لخريطة المعمارية الكاملة والقواعد العشر والمشكلات المعروفة وأنماط التنقيح.

**البناء من المصدر** (يتطلب Go 1.23+, Node.js 20+, gcc — Linux فقط، CGO لـ SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# pre-push hygiene: bin/check-lucx.sh  (gofumpt on the 49 LucX-owned files)
```

**إجراء المزامنة مع upstream** (تم التحقق منه v3.5.0→v3.6.0، 103 commit / 432 ملف / 7 تعارض):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolve block by block (see AGENTS.md Rule 8) — never blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # compare marker counts before/after to detect lost blocks
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

---

## ⭐ النجوم عبر الزمن

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
