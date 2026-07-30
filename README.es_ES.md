<!-- LUCX-HOOK: LucX-UI fork README — Streamlined ES README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
  <b>Español</b> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **Solo para uso personal, no comercial, científico y educativo.** El uso comercial (reventa de VPN o paneles de pago) requiere permiso por escrito bajo PolyForm Noncommercial 1.0.0.

---

## ⚡ Inicio Rápido

Instalación rápida en **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch, etc.)**:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

<details>
<summary><b>🛠️ Instalación Avanzada y Configuración (Cloud-Init, Docker, PostgreSQL, Variables)</b></summary>

### Instalación no interactiva (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
Las credenciales se guardan en `/etc/x-ui/install-result.env`.

### Docker con PostgreSQL
```bash
docker compose --profile postgres up -d
```

### Variables de entorno principales (`/etc/default/x-ui`)
| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `XUI_DB_TYPE` | Motor de BD (`sqlite` o `postgres`) | `sqlite` |
| `XUI_DB_DSN` | Conexión PostgreSQL | — |
| `XUI_ENABLE_FAIL2BAN` | Activar Fail2ban para límite IP | `true` |
| `XUI_LOG_LEVEL` | Nivel de registros (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🌟 Acerca de LucX-UI

**LucX-UI** es un panel de control web multiprotocolo avanzado para administrar servidores [Xray-core](https://github.com/XTLS/Xray-core), creado como un fork mejorado de [3x-ui](https://github.com/MHSanaei/3x-ui) con integración nativa de **AmneziaWG (AWG)**.

El proyecto añade soporte resistente a la censura de AmneziaWG como un sidecar a nivel de kernel, reflejando la arquitectura de MTProto. Ofrece ajustes precisos de ofuscación, imitación de huellas TLS de navegador, modo cliente (AWG Outbounds), diagnósticos integrados y dos modos de enrutamiento (Kernel NAT y Route through Xray) manteniendo total compatibilidad con 3x-ui.

### 🛡️ Características de AmneziaWG (AWG)
- **Inbounds y Outbounds AWG** — Sidecar de kernel (`awg-quick`), conexión a servidores AWG externos (`awgo-{id}`), ciclo de conciliación de 10 segundos y creador de módulos DKMS.
- **Ofuscación Avanzada** — Perfiles Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4), mimatización de paquetes CPS (TLS, DNS, SIP, QUIC) y huellas TLS de navegador (Chrome, Firefox, Safari).
- **Captura de Firma en Vivo** — Convierte saludos QUIC reales en parámetros I1–I5.
- **Enrutamiento y Diagnóstico** — Dos modos (Kernel NAT y Route through Xray con policy routing y sniffing) + diagnóstico en panel con un solo clic.

### 🚀 Características Base de 3x-ui
- **Protocolos:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Seguridad y Transportes:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Gestión:** Cuotas de tráfico, límites IP (Fail2ban), estado en línea, suscripciones, bot de Telegram, API REST, multinodo, SQLite / PostgreSQL.

<details>
<summary><b>📸 Capturas de pantalla</b></summary>

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

## 📜 Licencia y Términos

Este proyecto se publica bajo **dos licencias** (detalles en [LICENSING.md](LICENSING.md)):

| Componente | Licencia |
|---|---|
| Código base original 3x-ui | **GPL-3.0** |
| Componentes LucX-UI (`internal/awg/`, `internal/lucx/`, frontend) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 Agradecimientos y Créditos

- **Probadores y Colaboradores:** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, el equipo de **3x-ui**.
- **Proyectos e Inspiración:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Apoyar el proyecto

LucX-UI es gratuito para uso personal. Puede apoyar el desarrollo:

| Método | Detalles |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Rusia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ Estrellas a lo Largo del Tiempo

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
