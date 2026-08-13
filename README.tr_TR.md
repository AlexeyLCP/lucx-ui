<!-- LUCX-HOOK: LucX-UI fork README — Streamlined TR README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Gelişmiş Xray paneli** — yerel AmneziaWG (3.1'e kadar), denetimli tüneller (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel), Clash / Amnezia `vpn://` / Happ abonelikleri, geodata browser ve RoscomVPN yönlendirme.

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

[3x-ui](https://github.com/MHSanaei/3x-ui), modern React 19 + Ant Design 6 ön yüzüne sahip mükemmel bir çoklu protokol panelidir. LucX-UI, 3x-ui'nin sunduğu her şeyi korur ve upstream'de olmayanları ekler: **yerel AmneziaWG (AWG, 3.1'e kadar)**, **tünel sidecar'ları** (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel), **zengin abonelikler** (Clash Meta AWG, Amnezia `vpn://`, Happ) ve **geodata araçları** (panel browser + RoscomVPN paketleri):

| Özellik | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG inbound (çekirdek sidecar'ı `awg-quick` ile) | ✗ | ✓ |
| AWG CPS gizleme (TLS / DNS / SIP / QUIC + tarayıcı parmak izleri) | ✗ | ✓ |
| AWG outbound — üst AWG sunucularına VPN zincirleme (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1 (`RandomTrailers` / `DisableCookies`, anti-DPI) | ✗ | ✓ |
| İstemci yapılandırması sürüm ön ayarları (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| Panel içi AWG teşhisi (yönlendirme / NAT / eşler / el sıkışmalar) | ✗ | ✓ |
| NaiveProxy tünel sidecar'ı (Caddy + forward_proxy, denetimli) | ✗ | ✓ |
| İstemci başına NaiveProxy kimlik bilgileri + aboneliklerde `naive+https://` | ✗ | ✓ |
| NaiveProxy → Xray yönlendirme (SOCKS loopback köprüsü, isteğe bağlı) | ✗ | ✓ |
| olcRTC tünel sidecar'ı (meet odaları üzerinden WebRTC, denetimli) | ✗ | ✓ |
| qWDTT tünel sidecar'ı (VK TURN üzerinde WireGuard, denetimli) | ✗ | ✓ |
| mieru tünel sidecar'ı (`mita`, istemci başına trafik, denetimli) | ✗ | ✓ |
| TrustTunnel sidecar'ı (AdGuard VPN protokolü, HTTPS benzeri, denetimli) | ✗ | ✓ |
| Clash Meta'ta AWG + Amnezia aboneliği `/awg/` (`.conf` / `vpn://`) | ✗ | ✓ |
| Geodata browser — panelden geosite/geoip kategorileri | ✗* | ✓ |
| RoscomVPN geo paketi (`geoip/geosite_ROSCOM.dat`) | ✗ | ✓ |
| Happ yönlendirme profilleri (RoscomVPN deeplink + özel) | ✗ | ✓ |
| Akıllı Cluster outbound bağlantıları | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 ön yüz | ✓ | ✓ (devralınan) |
| Tüm Xray protokolleri (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Sürtünmesiz üst senkronizasyon (LUCX-HOOK izolasyonu, 49 dosya) | — | ✓ |

\* Upstream [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) (henüz birleştirilmedi) — LucX-UI'ye taşındı.

Bir çekirdek sidecar'ı (3x-ui'nin MTProto `mtg`'si gibi), AWG'nin gerçek bir çekirdek arabirimi olarak çalıştığı (kullanıcı alanı shim'i değil) anlamına gelir; böylece Xray çözülmüş trafiği kendi TUN inbound'u üzerinden yönlendirir ve AWG trafiğinde Xray'ın tam yönlendirme, sniffing ve alan adı kural gücünü kullanırsınız.

---

## 🌟 LucX-UI Hakkında

**LucX-UI**, [3x-ui](https://github.com/MHSanaei/3x-ui)'nun geliştirilmiş fork'udur (upstream **v3.6.0** ile senkron). Stok Xray protokollerinin ötesinde: yerel **AmneziaWG** (çekirdek sidecar, MTProto/`mtg` ile aynı fikir, artık **AWG 3.1**'e kadar), panel denetimli **tüneller** (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel), genişletilmiş **abonelikler** (Clash Meta AWG, Amnezia `/awg/` + `vpn://`, Happ) ve RoscomVPN listeli **geodata browser**. Katı `LUCX-HOOK` ile %100 upstream uyumu.

### 🛡️ AmneziaWG (AWG) Özellikleri
- **AWG Inbound & Outbound** — Çekirdek sidecar'ı (`awg-quick`), üst AWG sunucularına istemci modunda bağlanma (`awgo-{id}`), 10 saniyelik otomatik uzlaştırma döngüsü ve DKMS modül derleyicisi.
- **Gelişmiş Gizleme** — Lite/Standard/Pro profilleri (Jc/Jmin/Jmax/S1–S4/H1–H4), CPS paket taklidi (TLS, DNS, SIP, QUIC) ve tarayıcı TLS parmak izleri (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — Otomatik üretilen 32 baytlık anahtarlarla AmneziaWG 3 başlık koruması; sunucu tarafı sürüm tavanı, istemci başına özellik emisyonunu denetler.
- **AWG 3.1** — `RandomTrailers` (rastgele paket kuyruğu, boyuta göre anti-DPI) ve `DisableCookies`; panel güncellemesinde çekirdek modülü ve araçlar otomatik olarak v3.1'e yükselir.
- **İstemci Sürüm Ön Ayarları** — Tek bir inbound'dan AWG 1.5 / 2 / 3 / 3.1 için istemci yapılandırmaları üretin — istemci uygulamanızın anladığı biçimi seçin.
- **Canlı İmza Yakalama** — Ön alan adlarından gerçek QUIC el sıkışmalarını I1–I5 gizleme parametrelerine dönüştürür.
- **Yönlendirme & Teşhis** — İki yönlendirme modu (Kernel NAT ve politika yönlendirmeli & sniffing'li Route through Xray) + panel içi tek tıkla teşhis.

### 🚇 Tünel Sidecar'ları (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel)
- **NaiveProxy** — `forward_proxy` eklentili Caddy ([klzgrad](https://github.com/klzgrad/forwardproxy) fork'u, HTTP/2 padding) panel denetimli bir sidecar olarak çalışır: render edilmiş Caddyfile, crash-revive reconcile ile start/stop/restart ve üç seviyeli sağlık sondası (process → TCP → TLS).
- **İstemci başına kimlik bilgileri** — paneldeki her etkin istemci otomatik olarak kişisel bir `basic_auth` çifti alır (panel sırrından türetilir, saklanmaz); istemciyi devre dışı bırakmak bir sonraki reconcile'da iptal eder.
- **Abonelikler** — her istemcinin aboneliği Xray/AWG bağlantılarının yanında kişisel `naive+https://` bağlantısını taşır (NekoBox / husi / Exclave standart formatı), ayrıca panelde QR kod ve güçlü parola üreteci vardır.
- **Panel UX** — Auto TLS (Let's Encrypt) veya kendi cert/key'iniz, `caddy adapt` doğrulamalı raw-Caddyfile modu, Caddyfile önizlemesi, süreç logları, ikili yükleme/indirme.
- **Xray üzerinden yönlendir (isteğe bağlı)** — anahtar, Caddy'nin gizli loopback SOCKS köprüsü üzerinden hedeflere bağlanmasını sağlar (`upstream socks5://127.0.0.1:…`, yerel forward_proxy — yama yok), etiket `lucx-tunnel-naive`; böylece NaiveProxy trafiği tam Xray yönlendirme / sniffing / alan adı kurallarını alır (MTProto ile aynı). Varsayılan doğrudan çıkıştır.
- **olcRTC** — yasal görüntülü arama odası üzerinden TCP-over-WebRTC tüneli ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)).
- **qWDTT** — VK Calls TURN üzerinden WireGuard ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)).
- **mieru** — TLS yerine özel bir protokol üzerinden sansüre dayanıklı proxy ([enfein/mieru](https://github.com/enfein/mieru) `mita`, GPL-3.0). Panel istemcisi başına HMAC kimlik bilgileriyle çok istemcili, istemci başına trafik & çevrimiçi durumu ve `mierus://` paylaşım bağlantısı. İstemciler: mieru CLI, mihomo, Clash Verge Rev, husi, Exclave.
- **TrustTunnel** — AdGuard VPN protokolü ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel), Apache-2.0): trafik HTTPS'den ayırt edilemez (HTTP/1.1 + HTTP/2 + QUIC). Panelin ACME sertifikasını yeniden kullanır (sertifikası verilmiş bir alan adı gerekir) ve Flutter / CLI istemcileri için `tt://?` deep-link üretir.

### 📦 Abonelikler, geodata ve istemci yönlendirme
- **Amnezia aboneliği** — `/awg/{subId}` saf `.conf` veya `vpn://…` döner.
- **Clash Meta'ta AWG** — `amnezia-wg-option` ile peer'ler.
- **Geodata browser** — routing UI'dan `geoip*.dat` / `geosite*.dat` gezinme ([PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / [STRENCH0](https://github.com/STRENCH0) portu).
- **RoscomVPN geo paketi** — `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip) / [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)).
- **Happ profilleri** — Settings → Happ: RoscomVPN deeplink ([roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

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

Bu proje kendi kodu için **iki lisans** ve üçüncü taraf ikili/veriler için upstream koşulları altında yayınlanır (tam matris: [LICENSING.md](LICENSING.md)):

| Bileşen | Lisans |
|---|---|
| Orijinal 3x-ui kod tabanı | **GPL-3.0** |
| LucX-UI bileşenleri (`internal/awg/`, `internal/lucx/`, LucX ön yüz sayfaları) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| `forward_proxy` eklentisi ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/mieru-*` (`mita`, [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| AmneziaWG kernel module & tools ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (modül; host'a kurulur) |
| Stok geo `.dat` (Loyalsoldier / IR / RU / ROSCOM) | Her veri setinin upstream'i (bkz. LICENSING.md) |

Tünel ikilileri **alt süreçlerdir** — panel onlara link etmez. qWDTT GPL'si o ikiliye ve kaynaklarına aittir, LucX PolyForm koduna değil.

---

## 🤝 Teşekkürler ve Katkıda Bulunanlar

Tüm açık kaynak projelere ve insanlara teşekkürler.

### Test edenler & katkıda bulunanlar
- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13)), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, **[3x-ui](https://github.com/MHSanaei/3x-ui)** ekibi.

### Taşınan / dayanak upstream PR'lar
- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) geodata browser.

### Projeler & ilham
[MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) · [amnezia-vpn](https://github.com/amnezia-vpn) · [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) / [forwardproxy](https://github.com/klzgrad/forwardproxy) · [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) · [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) · [enfein/mieru](https://github.com/enfein/mieru) · [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) · [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) · [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) · [hydraponique](https://github.com/hydraponique) RoscomVPN ([geoip](https://github.com/hydraponique/roscomvpn-geoip) / [geosite](https://github.com/hydraponique/roscomvpn-geosite) / [routing](https://github.com/hydraponique/roscomvpn-routing)) · [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) · [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules) · [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) · [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) · [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) · [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) · [refraction-networking/utls](https://github.com/refraction-networking/utls)

---

## ☕ Projeyi Destekleyin

LucX-UI kişisel kullanım için ücretsizdir. **Beğendiniz mi? ⭐ verin** — projenin bulunmasına yardımcı olur. Bağışlar isteğe bağlıdır:

| Method | Details |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
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
