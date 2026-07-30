<!-- LUCX-HOOK: LucX-UI fork README — TR lead section, license, credits, sources. Keep in sync with LICENSING.md and AGENTS.md. -->
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

**LucX-UI**, yerel **AmneziaWG (AWG)** desteğine sahip bir [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) çatallamasıdır (fork). AWG, çekirdek arabirimi (kernel-interface) sidecar olarak çalışır — üst projenin MTProto (mtg) için kullandığı mimariyle tam bir simetri oluşturur: panel yaşam döngüsünü ve trafik muhasebesini yönetir, Xray ise isteğe bağlı olarak trafiği yönlendirir.

### Eklenen ve Çalışan Özellikler

- ✅ **AWG Inbound'lar** — `awg-quick` tabanlı çekirdek sidecar'ı: oluşturma, her 10 saniyede bir reconcile (uzlaştırma), yetim arabirim temizliği, DKMS çekirdek modülü yükleyicisi.
- ✅ **AWG Outbound'lar (İstemci Modu)** — Panel, üst seviye AmneziaWG sunucusuna bağlanabilir: Xray altında özel sekme, mevcut `.conf` yapıştırma ve reconcile döngüsüyle yönetilen `awgo-{id}` çekirdek arabirimi. Xray yapılandırmasına `sockopt.interface` içeren `freedom` outbound enjekte edilir, böylece yönlendirme kuralları ve yük dengeleyiciler trafiği üst VPN'e iletebilir.
- ✅ **Obfuscation (Karıştırma/Gizleme)** — Lite/Standard/Pro profilleri (Jc/Jmin/Jmax/S1–S4/H1–H4) ve CPS paket taklidi: TLS, DNS, SIP, QUIC.
- ✅ **Tarayıcı TLS Parmak İzleri** — Chrome (GREASE), Firefox 120+ (NSS sıralaması, padding), Safari 16+ (Apple sıralaması, TLS 1.1). TLS ve QUIC için.
- ✅ **Canlı Sunucudan İmza Yakalama** — Ön alan adından gerçek QUIC el sıkışmasını I1–I5 değerlerine dönüştürür.
- ✅ **İstemci Yönetimi** — QR kodları, `.conf` indirme, istemci bazında trafik takibi (`awg show transfer`).
- ✅ **İki Yönlendirme Modu:**
  - **Kernel NAT** — Doğrudan çekirdek yönlendirmesi; NAT kuralları iptables sıfırlansa bile reconcile döngüsüyle kendi kendini onarır.
  - **Xray Üzerinden Yönlendirme (Route through Xray)** — Trafik, TUN inbound, politika yönlendirmesi ve sniffing yardımıyla Xray'in tüm yönlendirme hattından geçer (alan adı/geosite kuralları, yük dengeleyiciler, zincirleme outbound'lar).
- ✅ **Panel İçi Teşhis (Diagnostics)** — Inbound formunda tek tıkla teşhis: arabirim durumu, ip_forward, eşler/el sıkışmaları, NAT/TUN kuralları anında görünür.
- ✅ **Saha Testlerinden Geçti** — Test VPS'lerinde başarıyla test edildi: el sıkışma, ICMP, HTTPS, trafik takibi, zincirleme ve her iki yönlendirme modu.

### Kurulum

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Paneli [en son sürümden](https://github.com/AlexeyLCP/lucx-ui/releases/latest), systemd servisini, Xray-core ve mtg'yi (3x-ui üst sürümünden) yükler ve DKMS aracılığıyla AmneziaWG çekirdek modülünü derler (`bin/install-awg-module.sh`).

### Lisans

Bu proje **çift lisans** altındadır (ayrıntılar [LICENSING.md](LICENSING.md) dosyasında):

| Bileşen | Lisans |
|---|---|
| Orijinal 3x-ui kodu | **GPL-3.0** (üst projenin gerektirdiği şekilde) |
| LucX bileşenleri (`internal/awg/`, `internal/lucx/`, AWG ön yüzü, betikler) | **PolyForm Noncommercial 1.0.0** |

Pratikte: Kişisel, ticari olmayan, bilimsel, araştırma ve eğitim amaçlı kullanım için **ücretsizdir**. **Ticari kullanım** (VPN satışı, ücretli paneller, ticari ürünlere entegrasyon) yazarın açık yazılı iznini gerektirir: bir [issue](https://github.com/AlexeyLCP/lucx-ui/issues) açın veya depo sahibiyle iletişime geçin. Dosyalardaki `SPDX-License-Identifier` başlıkları sınırları net bir şekilde belirler: başlık yoksa GPL-3.0'dır.

### Teşekkürler

- **VladufQa** — Canlı VPS (ruvds) testleri: ilk el sıkışmalar, trafik, zincirleme yönlendirme ve hata bildirimleri.
- **Kirill Rudenko** — Testler (runode) ve **PR #13**: AWG needRestart, iif politika yönlendirmesi, inbound başına tablolar/ağ geçitleri, rota onarımı ve sniffing.
- **302ba (Alex)** — **PR #24**: Zod şeması ayrıştırılırken istemci alanlarının kaybolması hatasının düzeltilmesi.
- **3x-ui** ekibi — Harika bir temel ve yansıttığımız sidecar mimarisi için.

### Kaynaklar ve İlham Alınan Projeler

- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — Çatallama tabanı (GPL-3.0), MTProto sidecar mimarisi.
- [AmneziaVPN](https://github.com/amnezia-vpn) — AmneziaWG protokolü ve çekirdek modülü.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — PostUp NAT deseni (MASQUERADE + FORWARD), QUIC Initial üreteçleri ve DKMS yaklaşımı.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — QUIC imza yakalama (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) & [refraction-networking/utls](https://github.com/refraction-networking/utls) — ClientHello profillerimiz için Firefox/Safari TLS parmak izleri.

### ☕ Projeyi Destekleyin

LucX-UI kişisel ve ticari olmayan kullanım için ücretsizdir. Panel zaman kazanmanızı sağlıyorsa geliştirmeyi destekleyebilirsiniz:

| Yöntem | Detaylar |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Rusya) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Bağışlar bir teşekkürdür, satın alma değildir: ticari lisans sağlamaz ve [LICENSING.md](LICENSING.md) şartlarını değiştirmez.

---

*Aşağıda orijinal **3x-ui** Türkçe dokümantasyonu yer almaktadır.*

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

**3X-UI**, [Xray-core](https://github.com/XTLS/Xray-core) sunucularını yönetmek için geliştirilmiş profesyonel, açık kaynaklı bir web kontrol panelidir. Tek bir sanal sunucudan (VPS) çok düğümlü (multi-node) dağıtımlara kadar çok çeşitli proxy ve VPN protokollerini kurmak, yapılandırmak ve izlemek için temiz, çok dilli bir arayüz sağlar.

Orijinal X-UI projesinin geliştirilmiş bir çatallaması (fork) olarak inşa edilen 3X-UI; çok daha geniş protokol desteği, artırılmış kararlılık, kullanıcı başına trafik hesaplama ve kullanım kolaylığı sağlayan birçok yeni özellik sunar.

> [!IMPORTANT]
> Bu proje yalnızca kişisel kullanım için tasarlanmıştır. Lütfen yasadışı amaçlar için veya üretim (production) ortamında kullanmayın.

## Özellikler

- **Çoklu protokol destekli gelen bağlantılar (Inbounds)** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Karma), Dokodemo-door / Tunnel ve TUN.
- **Modern aktarımlar (transports) ve güvenlik** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade ve XHTTP; TLS, XTLS ve REALITY ile güvene alınmıştır.
- **Geri Dönüş (Fallbacks)** — Xray'in fallback desteğini kullanarak tek bir port üzerinde birden fazla protokole (ör. 443 üzerinde hem VLESS hem Trojan) hizmet verin.
- **Kullanıcı başına yönetim** — Trafik kotaları, bitiş tarihleri, IP sınırları, canlı çevrimiçi (online) durumu ve tek tıkla paylaşım bağlantıları, QR kodları ve abonelikler.
- **Trafik istatistikleri** — Gelen bağlantı (Inbound), istemci ve giden bağlantı (Outbound) bazında istatistikler ve sıfırlama kontrolleri.
- **Çoklu düğüm (Multi-node) desteği** — Tek bir panel üzerinden birden fazla sunucuyu yönetin ve ölçeklendirin.
- **Giden bağlantı (Outbound) ve yönlendirme** — WARP, NordVPN, özel yönlendirme kuralları, yük dengeleyiciler (load balancers) ve giden bağlantı proxy zincirleme (proxy chaining).
- **Dahili abonelik sunucusu** (Birden fazla çıktı formatı ve [özel sayfa şablonları](docs/custom-subscription-templates.md) ile).
- Uzaktan izleme ve yönetim için **Telegram botu**.
- Panel içi Swagger dokümantasyonuna sahip **RESTful API**.
- **Esnek depolama** — SQLite (varsayılan) veya PostgreSQL.
- Koyu ve açık tema seçenekleriyle **13 farklı UI dili**.
- Kullanıcı başına IP limitlerini zorunlu kılmak için **Fail2ban entegrasyonu**.

## Ekran Görüntüleri

<details>
<summary>Genişletmek için tıklayın</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Genel Bakış" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Gelen Bağlantılar (Inbounds)" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Kullanıcı Ekle" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Yapılandırmalar" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Hızlı Başlangıç

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

Belirli bir sürümü kurmak için, etiketini (ör. `v3.4.0`) ekleyin:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

Sürekli güncellenen **dev** sürümünü (kararlı bir sürüm değil; `main` dalından her commit'te oluşturulan en son ön sürüm) kurmak için `dev-latest` değerini geçirin:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

Kurulum sırasında rastgele bir kullanıcı adı, şifre ve erişim yolu oluşturulur. Kurulumdan sonra, hizmeti başlatabileceğiniz/durdurabileceğiniz, giriş bilgilerinizi görüntüleyebileceğiniz veya sıfırlayabileceğiniz, SSL sertifikalarını yönetebileceğiniz ve çok daha fazlasını yapabileceğiniz yönetim menüsünü açmak için terminalde `x-ui` komutunu çalıştırın.

Tam dokümantasyon için lütfen [proje Wiki sayfasını](https://github.com/MHSanaei/3x-ui/wiki) ziyaret edin.

### Etkileşimsiz kurulum

Yükleyici, cloud-init için **etkileşimsiz** olarak da çalışır.
`XUI_NONINTERACTIVE=1` ayarlayın (veya TTY olmadan boru hattına aktarın); kurulum baştan
sona hiçbir soru sormadan tamamlanır, rastgele kimlik bilgileri oluşturup bunları
`/etc/x-ui/install-result.env` dosyasına yazar. Şunlar için [`deploy/`](deploy/) klasörüne bakın:

- [Cloud-init user-data](deploy/cloud-init/) — herhangi bir bulutta etkileşimsiz kurulum (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Hetzner Cloud notları](deploy/marketplace/hetzner/) — Hetzner üzerinde cloud-init tabanlı dağıtım

## Desteklenen Platformlar

**İşletim sistemleri:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine ve Windows.

**Mimariler:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Veritabanı Seçenekleri

3X-UI kurulum sırasında seçilebilecek iki arka uç (backend) destekler:

- **SQLite** (varsayılan) — `/etc/x-ui/x-ui.db` konumunda tek bir dosya. Kurulum gerektirmez, küçük ve orta ölçekli dağıtımlar için idealdir.
- **PostgreSQL** — Yüksek kullanıcı sayıları veya çoklu düğüm (multi-node) kurulumları için önerilir. Yükleyici sizin için yerel olarak PostgreSQL kurabilir veya mevcut bir sunucuya DSN bağlantısı kabul edebilir.

Çalışma anında veritabanı türü ortam değişkenleri (environment variables) ile seçilir (yükleyici bunları sizin için `/etc/default/x-ui` dosyasına yazar):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Mevcut bir SQLite Kurulumunu PostgreSQL'e Taşıma

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# ardından /etc/default/x-ui içindeki XUI_DB_TYPE ve XUI_DB_DSN değerlerini ayarlayıp yeniden başlatın:
systemctl restart x-ui
```

Kaynak SQLite dosyasına dokunulmaz; yeni veritabanının düzgün çalıştığını doğruladıktan sonra eski SQLite dosyasını manuel olarak silebilirsiniz.

### Docker

Varsayılan `docker compose up -d` komutu SQLite kullanmaya devam eder. Birlikte paketlenmiş PostgreSQL servisi ile çalıştırmak için, `docker-compose.yml` dosyasındaki iki `XUI_DB_*` değişken satırının yorumunu kaldırın ve profille başlatın:

```bash
docker compose --profile postgres up -d
```

Docker imajı, kullanıcı başına **IP limitlerini** zorunlu kılmak için Fail2ban ile (varsayılan olarak etkindir) paketlenmiştir. Fail2ban, ihlalcileri `iptables` ile engeller ve bunun için `NET_ADMIN` yetkisine ihtiyaç duyar. `docker-compose.yml` bunu zaten `cap_add` üzerinden vermektedir; ancak konteyneri bunun yerine `docker run` ile başlatırsanız bu yetkileri kendiniz eklemelisiniz, aksi takdirde yasaklamalar günlüğe kaydedilir ancak uygulanmaz:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Ortam Değişkenleri (Environment Variables)

| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `XUI_DB_TYPE` | Veritabanı türü: `sqlite` veya `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL bağlantı dizesi (eğer `XUI_DB_TYPE=postgres` ise) | — |
| `XUI_DB_FOLDER` | SQLite veritabanı dizini | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maksimum açık bağlantı sayısı (PostgreSQL havuzu) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maksimum boşta bekleme bağlantısı (PostgreSQL havuzu) | — |
| `XUI_INIT_WEB_BASE_PATH` | Web paneli için başlangıç URI yolu | `/` |
| `XUI_ENABLE_FAIL2BAN` | Fail2ban tabanlı IP limit uygulamasını etkinleştir | `true` |
| `XUI_LOG_LEVEL` | Günlük (Log) ayrıntı seviyesi (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Hata ayıklama (debug) modunu etkinleştir | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Tünel sağlık izleyicisini etkinleştir (bir URL'yi yoklar ve tekrarlanan başarısızlıklardan sonra xray'i yeniden başlatır; yeniden başlatma tüm istemcilerin bağlantısını düşürür) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Yoklamanın gönderildiği proxy; yoklamanın tüneli test etmesi için bunu yerel bir xray gelen bağlantısına yönlendirin (ör. `socks5://127.0.0.1:1080`). Boş bırakılırsa yoklama yalnızca ana makine bağlantısını kontrol eder | — |
| `XUI_TUNNEL_HEALTH_URL` | Tünel sağlığı için yoklanan URL | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Yoklamalar arasındaki aralık | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Yoklama başına zaman aşımı | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Yeniden başlatma tetiklenmeden önceki ardışık başarısızlık sayısı | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Ardışık yeniden başlatmalar arasındaki minimum gecikme | `5m` |

## Desteklenen Diller

Panel arayüzü 13 farklı dilde mevcuttur:

İngilizce · Farsça · Arapça · Çince (Basitleştirilmiş) · Çince (Geleneksel) · İspanyolca · Rusça · Ukraynaca · Türkçe · Vietnamca · Japonca · Endonezce · Portekizce (Brezilya)

## Katkıda Bulunma

Katkılarınızı her zaman bekliyoruz. Bir sorun (issue) açmadan veya pull request (PR) göndermeden önce lütfen [Katkıda Bulunma Kılavuzunu](/CONTRIBUTING.md) okuyun.

## Özel Teşekkürler

- [alireza0](https://github.com/alireza0/)

## Teşekkür & Atıf

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Lisans: **GPL-3.0**): _Geliştirilmiş v2ray/xray ve v2ray/xray-clients yönlendirme (routing) kuralları; yerleşik İran alan adları ile güvenlik ve reklam engelleme odaklıdır._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Lisans: **GPL-3.0**): _Bu depo, Rusya'daki engellenen alan adları ve adreslere dayalı otomatik olarak güncellenen V2Ray yönlendirme kurallarını içerir._

## Topluluk Araçları

3x-ui çevresindeki topluluk tarafından oluşturulmuş araçlar ve entegrasyonlar.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Lisans: **MIT**): _Gelen bağlantılarnı, kullanıcıları, panel ayarlarını ve Xray yapılandırmasını Terraform / OpenTofu ile kod olarak (as code) yönetin._

## Projeyi Destekleyin

**Eğer bu proje size faydalı olduysa, bir yıldız verebilirsiniz**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Bana Bir Kahve Ismarla" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="NOWPayments üzerinden Kripto Bağış Butonu">
</a>

## Yıldız Tablosu

[![Zaman içerisindeki yıldız sayısı](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
