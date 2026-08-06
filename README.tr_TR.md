<!-- LUCX-HOOK: LucX-UI fork README — Streamlined TR README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Gelişmiş Xray ve AmneziaWG kontrol paneli** — birleşik abonelikler, çoklu sunucu yönetimi ve yerel AWG desteğiyle.

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
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <b>Türkçe</b>
</p>

> [!WARNING]
> **Yalnızca kişisel, ticari olmayan, bilimsel, araştırma ve eğitim amaçlı kullanım içindir.** Ticari kullanım — VPN satışı veya ücretli paneller dahil — PolyForm Noncommercial 1.0.0 kapsamında açık yazılı izin gerektirir.

---

## ⚡ Hızlı Başlangıç

**Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch vb.)** üzerinde tek satırla kurulum:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ Gelişmiş Kurulum & Yapılandırma (Cloud-Init, Docker, PostgreSQL, Ortam Değişkenleri)</b></summary>

### Otomatik Kurulum (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
Giriş bilgileri `/etc/x-ui/install-result.env` dosyasına kaydedilir.

### PostgreSQL ile Docker
```bash
docker compose --profile postgres up -d
```

### Önemli Ortam Değişkenleri (`/etc/default/x-ui`)
| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `XUI_DB_TYPE` | Veritabanı motoru (`sqlite` veya `postgres`) | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL DSN | — |
| `XUI_ENABLE_FAIL2BAN` | Fail2ban IP kısıtlamasını etkinleştir | `true` |
| `XUI_LOG_LEVEL` | Log seviyesi (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ Neden LucX-UI?

[3x-ui](https://github.com/MHSanaei/3x-ui), modern React 19 + Ant Design 6 ön yüzüne sahip mükemmel bir çoklu protokol panelidir. LucX-UI, 3x-ui'nin sunduğu her şeyi korur ve 3x-ui'de bulunmayan **yerel AmneziaWG (AWG)** — sansüre dirençli bir WireGuard fork'u — ekler:

| Özellik | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (çekirdek sidecar'ı `awg-quick` ile) | ✗ | ✓ |
| AWG CPS gizleme (TLS / DNS / SIP / QUIC + tarayıcı parmak izleri) | ✗ | ✓ |
| AWG outbound — üst AWG sunucularına VPN zincirleme (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| İstemci yapılandırması sürüm ön ayarları (1.5 / 2 / 3) | ✗ | ✓ |
| Panel içi AWG teşhisi (yönlendirme / NAT / eşler / el sıkışmalar) | ✗ | ✓ |
| Akıllı Cluster outbound bağlantıları | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 ön yüz | ✓ | ✓ (devralınan) |
| Tüm Xray protokolleri (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Sürtünmesiz üst senkronizasyon (LUCX-HOOK izolasyonu, 49 dosya) | — | ✓ |

Bir çekirdek sidecar'ı (3x-ui'nin MTProto `mtg`'si gibi), AWG'nin gerçek bir çekirdek arabirimi olarak çalıştığı (kullanıcı alanı shim'i değil) anlamına gelir; böylece Xray çözülmüş trafiği kendi TUN inbound'u üzerinden yönlendirir ve AWG trafiğinde Xray'ın tam yönlendirme, sniffing ve alan adı kural gücünü kullanırsınız.

---

## 🌟 LucX-UI Hakkında

**LucX-UI**, [3x-ui](https://github.com/MHSanaei/3x-ui)'nun (şu anda üst **v3.6.0** sürümüyle senkronize) yerel **AmneziaWG (AWG)** desteğini, üst MTProto mimarisini yansıtan bir çekirdek arabirimi sidecar'ı olarak ekleyen geliştirilmiş bir fork'udur. Katı `LUCX-HOOK` kod izolasyonu ile %100 üst uyumluluğunu korur.

### 🛡️ AmneziaWG (AWG) Özellikleri
- **AWG Inbound & Outbound** — Çekirdek sidecar'ı (`awg-quick`), üst AWG sunucularına istemci modunda bağlanma (`awgo-{id}`), 10 saniyelik otomatik uzlaştırma döngüsü ve DKMS modül derleyicisi.
- **Gelişmiş Gizleme** — Lite/Standard/Pro profilleri (Jc/Jmin/Jmax/S1–S4/H1–H4), CPS paket taklidi (TLS, DNS, SIP, QUIC) ve tarayıcı TLS parmak izleri (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — Otomatik üretilen 32 baytlık anahtarlarla AmneziaWG 3 başlık koruması; sunucu tarafı sürüm tavanı, istemci başına özellik emisyonunu denetler.
- **İstemci Sürüm Ön Ayarları** — Tek bir inbound'dan AWG 1.5 / 2 / 3 için istemci yapılandırmaları üretin — istemci uygulamanızın anladığı biçimi seçin.
- **Canlı İmza Yakalama** — Ön alan adlarından gerçek QUIC el sıkışmalarını I1–I5 gizleme parametrelerine dönüştürür.
- **Yönlendirme & Teşhis** — İki yönlendirme modu (Kernel NAT ve politika yönlendirmeli & sniffing'li Route through Xray) + panel içi tek tıkla teşhis.

### 🚀 Temel 3x-ui Özellikleri
- **Protokoller:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Taşımalar & Güvenlik:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Yönetim:** Trafik kotaları, IP limitleri (Fail2ban), canlı çevrimiçi durum, abonelikler, Telegram botu, REST API, Çoklu Düğüm desteği, SQLite / PostgreSQL.

<details>
<summary><b>📸 Ekran Görüntüleri</b></summary>

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

## 🔄 3x-ui'den Geçiş

LucX-UI, 3x-ui ile aynı Xray-core / SQLite (veya PostgreSQL) veritabanı şema tabanını paylaşır ve AWG tabloları ilk çalıştırmada otomatik olarak oluşturulur. Mevcut bir 3x-ui kurulumunun üzerine yüklemek için önce veritabanınızı yedekleyin ve standart kurulum komutunu çalıştırın:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

AWG çekirdek modülü, yükleyici tarafından otomatik olarak derlenir (`bin/install-awg-module.sh`, DKMS). Kurulumdan sonra, AWG çekirdek modülü sürümünü doğrulamak için konsolda `x-ui` komutunu çalıştırın ve panelden AWG inbound'ları eklemeye başlayın.

---

## 📜 Lisans ve Şartlar

Bu proje **iki lisans** altında yayınlanır (detaylar [LICENSING.md](LICENSING.md) içinde):

| Bileşen | Lisans |
|---|---|
| Orijinal 3x-ui kod tabanı | **GPL-3.0** |
| LucX-UI bileşenleri (`internal/awg/`, `internal/lucx/`, ön yüz) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Teşekkürler ve Katkıda Bulunanlar

- **Test Edenler & Katkıda Bulunanlar:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, **3x-ui ekibi**.
- **Projeler & İlham Kaynakları:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Projeyi Destekleyin

LucX-UI kişisel kullanım için ücretsizdir. Süregelen geliştirmeyi destekleyebilirsiniz:

| Method | Details |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Russia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ Geliştiriciler İçin

<details>
<summary><b>Mimari, derleme ve üst senkronizasyon (genişletmek için tıklayın)</b></summary>

**Mimari ve izolasyon kuralı.** Tüm LucX kodu izole paketlerde yaşar (`internal/awg/`, `internal/lucx/`); üst 3x-ui dosyalarındaki değişiklikler yalnızca `// LUCX-HOOK` / `// END LUCX-HOOK` işaretçileri içinde yapılır, böylece her üst sürüm yakın-zamanlı bir port olur. Tam mimari haritası, 10 kural, bilinen sorunlar ve hata ayıklama desenleri için [AGENTS.md](AGENTS.md) dosyasına bakın.

**Kaynaktan derleyin** (Go 1.23+, Node.js 20+, gcc gerekli — yalnızca Linux, SQLite için CGO):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# pre-push hygiene: bin/check-lucx.sh  (gofumpt on the 49 LucX-owned files)
```

**Üst senkronizasyon prosedürü** (v3.5.0→v3.6.0 doğrulandı, 103 commit / 432 dosya / 7 çakışma):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolve block by block (see AGENTS.md Rule 8) — never blanket --ours/--theirs
git grep -c "LUCX-HOOK"  # compare marker counts before/after to detect lost blocks
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

---

## ⭐ Zaman İçinde Yıldızlar

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
