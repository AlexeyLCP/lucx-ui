<!-- LUCX-HOOK: LucX-UI fork README — Streamlined TR README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **Yalnızca kişisel, ticari olmayan, bilimsel ve eğitim amaçlı kullanım içindir.** Ticari kullanım (VPN satışı veya ücretli paneller) PolyForm Noncommercial 1.0.0 kapsamında yazarın açık yazılı iznini gerektirir.

---

## ⚡ Hızlı Başlangıç

**Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch vb.)** üzerinde tek tıkla kurulum:

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

### Temel Ortam Değişkenleri (`/etc/default/x-ui`)
| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `XUI_DB_TYPE` | Veritabanı motoru (`sqlite` veya `postgres`) | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL adresi | — |
| `XUI_ENABLE_FAIL2BAN` | IP kısıtlaması için Fail2ban | `true` |
| `XUI_LOG_LEVEL` | Log seviyesi (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🌟 LucX-UI Hakkında

**LucX-UI**, yerel **AmneziaWG (AWG)** çekirdek arabirimi desteğiyle [3x-ui](https://github.com/MHSanaei/3x-ui) projesinden geliştirilmiş gelişmiş bir [Xray-core](https://github.com/XTLS/Xray-core) web yönetim panelidir.

### 🛡️ AmneziaWG (AWG) Özellikleri
- **AWG Inbound & Outbound** — Çekirdek sidecar'ı (`awg-quick`), üst AWG sunucularına istemci modunda bağlanma (`awgo-{id}`), 10 saniyelik otomatik uzlaştırma döngüsü ve DKMS modül derleyicisi.
- **Gelişmiş Gizleme** — Lite/Standard/Pro profilleri (Jc/Jmin/Jmax/S1–S4/H1–H4), CPS paket taklidi (TLS, DNS, SIP, QUIC) ve tarayıcı TLS parmak izleri (Chrome, Firefox, Safari).
- **Canlı İmza Yakalama** — Ön alan adından gerçek QUIC el sıkışmasını I1–I5 gizleme parametrelerine dönüştürme.
- **Yönlendirme & Teşhis** — İki yönlendirme modu (Kernel NAT ve politika yönlendirmeli Route through Xray) + panel içi tek tıkla teşhis.

### 🚀 Temel 3x-ui Özellikleri
- **Protokoller:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Güvenlik & Taşıma:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
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

## 📜 Lisans ve Şartlar

Bu proje **çift lisans** altındadır (detaylar [LICENSING.md](LICENSING.md) dosyasında):

| Bileşen | Lisans |
|---|---|
| Orijinal 3x-ui kod tabanı | **GPL-3.0** |
| LucX-UI bileşenleri (`internal/awg/`, `internal/lucx/`, ön yüz) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Teşekkürler ve Kaynaklar

- **Test Edenler & Katkıda Bulunanlar:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, **3x-ui ekibi**.
- **Projeler & İlham Kaynakları:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Projeyi Destekleyin

LucX-UI kişisel kullanım için ücretsizdir. Geliştirmeyi destekleyebilirsiniz:

| Yöntem | Detaylar |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Rusya) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ Yıldız Tablosu

[![Zaman içerisindeki yıldız sayısı](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
