<!-- LUCX-HOOK: LucX-UI fork README — Unified TR README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
  <a href="README.ar_EG.md">العربية</a> |
  <a href="README.zh_CN.md">中文</a> |
  <a href="README.es_ES.md">Español</a> |
  <b>Türkçe</b>
</p>

> [!WARNING]
> **Yalnızca kişisel, ticari olmayan, bilimsel, araştırma ve eğitim amaçlı kullanım içindir.** Ticari kullanım — VPN satışı, ücretli paneller veya bu kod üzerine inşa edilen abonelik hizmetleri dahil — yazarın açık yazılı iznini gerektirir. Yasadışı amaçlarla kullanmayın.

---

## LucX-UI Hakkında

**LucX-UI**, yerel **AmneziaWG (AWG)** desteğine sahip gelişmiş bir [Xray-core](https://github.com/XTLS/Xray-core) web yönetim panelidir. [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) projesinin geliştirilmiş bir çatallaması (fork) olarak oluşturulmuştur. AWG, çekirdek arabirimi (kernel-interface) sidecar olarak çalışır — üst projenin MTProto (mtg) için kullandığı mimariyle tam bir simetri oluşturur: panel yaşam döngüsünü ve trafik muhasebesini yönetir, Xray ise isteğe bağlı olarak trafiği yönlendirir.

### Ana Özellikler

#### 🛡️ AmneziaWG (AWG) Geliştirmeleri
- **AWG Inbound'lar** — `awg-quick` tabanlı çekirdek sidecar'ı: oluşturma, her 10 saniyede bir reconcile (uzlaştırma), yetim arabirim temizliği ve DKMS çekirdek modülü yükleyicisi.
- **AWG Outbound'lar (İstemci Modu)** — Panel doğrudan üst seviye bir AmneziaWG sunucusuna bağlanabilir: Xray ayarlarında özel sekme, hazır `.conf` dosyası yapıştırma ve reconcile döngüsüyle yönetilen `awgo-{id}` çekirdek arabirimi. Xray yapılandırmasına `sockopt.interface` içeren `freedom` outbound enjekte edilir, böylece yönlendirme kuralları trafiği üst VPN'e iletebilir.
- **Gizleme (Obfuscation) Kontrolü** — Lite/Standard/Pro profilleri (Jc/Jmin/Jmax/S1–S4/H1–H4) ve CPS paket taklidi: TLS, DNS, SIP ve QUIC.
- **Tarayıcı TLS Parmak İzleri** — Chrome (GREASE), Firefox 120+ (NSS sıralaması, padding) ve Safari 16+ (Apple sıralaması, TLS 1.1) parmak izleri.
- **Canlı Sunucudan İmza Yakalama** — Ön alandan alınan gerçek QUIC el sıkışmasını I1–I5 gizleme parametrelerine dönüştürür.
- **İstemci Yönetimi** — QR kodları, `.conf` indirme ve istemci bazında trafik takibi (`awg show transfer`).
- **İki Yönlendirme Modu**:
  - **Kernel NAT** — Doğrudan çekirdek yönlendirmesi; NAT kuralları iptables sıfırlansa bile reconcile döngüsüyle kendi kendini onarır.
  - **Route through Xray** — Trafik, TUN inbound, politika yönlendirmesi ve sniffing yardımıyla Xray'in tüm yönlendirme hattından geçer (alan adı/geosite kuralları, yük dengeleyiciler, zincirleme outbound'lar).
- **Panel İçi Teşhis (Diagnostics)** — Inbound formunda tek tıkla teşhis: arabirim durumu, ip_forward, eşler/el sıkışmaları, NAT/TUN kuralları anında görünür.

#### 🚀 Temel 3x-ui Özellikleri
- **Çoklu protokol desteği** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Karma) ve TUN.
- **Modern taşıma ve güvenlik** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade ve XHTTP; TLS, XTLS ve REALITY korumalı.
- **Geri Düşüş (Fallbacks)** — Xray'in fallback özelliği sayesinde tek bir port üzerinden birden fazla protokol sunun.
- **İstemci Yönetimi** — Trafik kotaları, son kullanım tarihleri, IP limitleri, canlı çevrimiçi durum ve tek tıkla paylaşım bağlantıları/QR/abonelikler.
- **Trafik İstatistikleri** — Bağlantı noktası, istemci ve giden trafik bazında ayrı ayrı izleme.
- **Çoklu Düğüm (Multi-node) Desteği** — Birden fazla sunucuyu tek bir panelden yönetin.
- **Giden Trafik & Yönlendirme** — WARP, NordVPN, özel yönlendirme kuralları ve zincirleme vekil sunucular.
- **Abonelik Sunucusu** — Özel sayfa şablonları ve çoklu çıktı formatları ile.
- **Telegram Botu** — Uzaktan izleme ve yönetim için.
- **RESTful API** — Panel içi Swagger dokümantasyonu ile.
- **Esnek Depolama** — SQLite (varsayılan) veya PostgreSQL.
- **Fail2ban Entegrasyonu** — İstemci başına IP limitlerini uygulamak için.

### Ekran Görüntüleri

<details>
<summary>Genişletmek için tıklayın</summary>

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

## Hızlı Başlangıç

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Paneli [en son sürümden](https://github.com/AlexeyLCP/lucx-ui/releases/latest), systemd servisini, Xray-core ve mtg'yi yükler ve DKMS aracılığıyla AmneziaWG çekirdek modülünü derler (`bin/install-awg-module.sh`).

Kurulum sırasında rastgele bir kullanıcı adı, parola ve erişim yolu oluşturulur. Kurulumdan sonra yönetim menüsünü açmak için `x-ui` çalıştırın.

### Otomatik (Sessiz) Kurulum

Yükleyici, cloud-init için **etkileşimsiz modda** çalışabilir. `XUI_NONINTERACTIVE=1` olarak ayarlayın; kurulum herhangi bir soru sormadan tamamlanır ve bilgileri `/etc/x-ui/install-result.env` dosyasına yazar. Rehberler için [`deploy/`](deploy/) klasörüne bakın.

## Desteklenen Platformlar

**İşletim Sistemleri:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine ve Windows.

**Mimariler:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Veritabanı Seçenekleri

3X-UI iki veritabanı motorunu destekler:

- **SQLite** (varsayılan) — `/etc/x-ui/x-ui.db` tek dosyası.
- **PostgreSQL** — Yüksek istemci sayıları veya dağıtılmış düğümler için önerilir.

`/etc/default/x-ui` içerisindeki ortam değişkenleri:
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Docker

Docker üzerinde PostgreSQL kullanmak için `docker-compose.yml` içerisindeki `XUI_DB_*` satırlarının yorumunu kaldırın ve çalıştırın:
```bash
docker compose --profile postgres up -d
```

## Ortam Değişkenleri

| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `XUI_DB_TYPE` | Veritabanı türü: `sqlite` veya `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL bağlantı adresi (`XUI_DB_TYPE=postgres` iken) | — |
| `XUI_DB_FOLDER` | SQLite veritabanı dosya dizini | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | IP limiti için Fail2ban entegrasyonu | `true` |
| `XUI_LOG_LEVEL` | Günlük detay seviyesi (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Tünel sağlık izleyicisini etkinleştir | `false` |

## Lisans ve Şartlar

Bu proje **çift lisans** altındadır (ayrıntılar [LICENSING.md](LICENSING.md) dosyasında):

| Bileşen | Lisans |
|---|---|
| Orijinal 3x-ui kodu | **GPL-3.0** (üst projenin gerektirdiği şekilde) |
| LucX bileşenleri (`internal/awg/`, `internal/lucx/`, AWG ön yüzü, betikler) | **PolyForm Noncommercial 1.0.0** |

Kişisel, ticari olmayan, bilimsel, araştırma ve eğitim amaçlı kullanım için **ücretsizdir**. **Ticari kullanım** (VPN satışı, ücretli paneller, ticari ürünlere entegrasyon) yazarın açık yazılı iznini gerektirir: bir [issue](https://github.com/AlexeyLCP/lucx-ui/issues) açın veya depo sahibiyle iletişime geçin. Dosyalardaki `SPDX-License-Identifier` başlıkları sınırları net bir şekilde belirler.

## Katkıda Bulunma

Katkılar memnuniyetle karşılanır. Lütfen bir issue veya PR açmadan önce [Katkı Rehberini](/CONTRIBUTING.md) okuyun.

## Teşekkürler ve Kaynaklar

### Test Edenler & Katkıda Bulunanlar
- **VladufQa** — Canlı VPS (ruvds) testleri: ilk el sıkışmalar, trafik, zincirleme yönlendirme ve hata bildirimleri.
- **Kirill Rudenko** — Testler (runode) ve **PR #13**: AWG needRestart, iif politika yönlendirmesi, inbound başına tablolar/ağ geçitleri, rota onarımı ve sniffing.
- **302ba (Alex)** — **PR #24**: Zod şeması ayrıştırılırken istemci alanlarının kaybolması hatasının düzeltilmesi.
- **alireza0** — Üst proje katkıcısı.
- **3x-ui ekibi** — Harika bir temel ve yansıttığımız sidecar mimarisi için.

### Kaynaklar ve İlham Alınan Projeler
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — Çatallama tabanı (GPL-3.0), MTProto sidecar mimarisi.
- [AmneziaVPN](https://github.com/amnezia-vpn) — AmneziaWG protokolü ve çekirdek modülü.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — PostUp NAT deseni (MASQUERADE + FORWARD), QUIC Initial üreteçleri ve DKMS yaklaşımı.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — QUIC imza yakalama (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) & [refraction-networking/utls](https://github.com/refraction-networking/utls) — Tarayıcı TLS parmak izleri.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) & [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) — Yönlendirme kuralları veri setleri.

### Topluluk Araçları
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Lisans: **MIT**): Panel ayarlarını kod olarak yönetin.

## ☕ Projeyi Destekleyin

LucX-UI kişisel ve ticari olmayan kullanım için ücretsizdir. Panel zaman kazanmanızı sağlıyorsa geliştirmeyi destekleyebilirsiniz:

| Yöntem | Detaylar |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Rusya) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Bağışlar bir teşekkürdür, satın alma değildir: ticari lisans sağlamaz ve [LICENSING.md](LICENSING.md) şartlarını değiştirmez.

## Yıldız Tablosu

[![Zaman içerisindeki yıldız sayısı](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)

<!-- END LUCX-HOOK -->
