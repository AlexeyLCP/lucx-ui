<!-- LUCX-HOOK: LucX-UI fork README — Streamlined ES README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Panel de control avanzado de Xray y AmneziaWG** — con suscripciones unificadas, gestión multi-servidor y soporte nativo de AWG.

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

Instalación con una sola línea en **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch, etc.)**:

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

## 🛡️ ¿Por qué LucX-UI?

[3x-ui](https://github.com/MHSanaei/3x-ui) es un excelente panel multiprotocolo con un frontend moderno en React 19 + Ant Design 6. LucX-UI mantiene todo lo que ofrece 3x-ui y añade **AmneziaWG (AWG) nativo** — un fork resistente a la censura de WireGuard — que 3x-ui no posee:

| Característica | 3x-ui | LucX-UI |
|---|:---:|:---:|
| Inbound AmneziaWG (sidecar de kernel vía `awg-quick`) | ✗ | ✓ |
| Ofuscación AWG CPS (TLS / DNS / SIP / QUIC + huellas de navegador) | ✗ | ✓ |
| AWG outbound — encadenamiento VPN a servidores AWG externos (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| Presets de versión de configuración cliente (1.5 / 2 / 3) | ✗ | ✓ |
| Diagnóstico AWG en panel (enrutamiento / NAT / peers / handshakes) | ✗ | ✓ |
| Sidecar de túnel NaiveProxy (Caddy + forward_proxy, supervisado) | ✗ | ✓ |
| Credenciales NaiveProxy por cliente + `naive+https://` en suscripciones | ✗ | ✓ |
| NaiveProxy → enrutamiento Xray (puente SOCKS loopback, opcional) | ✗ | ✓ |
| Enlaces outbound de clúster inteligente | ✗ | ✓ |
| Frontend React 19 + AntD 6 + Vite 8 + Zod 4 | ✓ | ✓ (heredado) |
| Todos los protocolos Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Sincronización con upstream sin fricción (aislamiento LUCX-HOOK, 49 archivos) | — | ✓ |

Un sidecar de kernel (como el `mtg` de MTProto en 3x-ui) significa que AWG se ejecuta como una interfaz de kernel real — no un shim de espacio de usuario — por lo que Xray enruta el tráfico descifrado a través de su propio TUN inbound, dándole todo el poder de enrutamiento, sniffing y reglas de dominio de Xray sobre el tráfico AWG.

---

## 🌟 Acerca de LucX-UI

**LucX-UI** es un fork mejorado de [3x-ui](https://github.com/MHSanaei/3x-ui) (actualmente sincronizado con upstream **v3.6.0**) que añade soporte nativo de **AmneziaWG (AWG)** como sidecar de interfaz de kernel, reflejando la arquitectura de MTProto de upstream. Mantiene 100% de compatibilidad con upstream mediante el aislamiento estricto de código `LUCX-HOOK`.

### 🛡️ Características de AmneziaWG (AWG)
- **Inbounds y Outbounds AWG** — Sidecar de kernel (`awg-quick`), conexión en modo cliente a servidores AWG externos (`awgo-{id}`), ciclo de conciliación automática de 10 segundos y creador de módulos DKMS.
- **Ofuscación Avanzada** — Perfiles Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4), mimatización de paquetes CPS (TLS, DNS, SIP, QUIC) y huellas TLS de navegador (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — Protección de cabecera AmneziaWG 3 con claves de 32 bytes autogeneradas; el techo de versión del lado del servidor controla la emisión de características por cliente.
- **Presets de Versión de Cliente** — Genere configs de cliente para AWG 1.5 / 2 / 3 desde un solo inbound — elija el formato que su app cliente entienda.
- **Captura de Firma en Vivo** — Convierte saludos QUIC reales de dominios frontales en parámetros I1–I5.
- **Enrutamiento y Diagnóstico** — Dos modos (Kernel NAT y Route through Xray con policy routing y sniffing) + diagnóstico en panel con un solo clic.

### 🚇 Sidecars de túnel (NaiveProxy)
- **NaiveProxy** — Caddy con el plugin `forward_proxy` (fork de [klzgrad](https://github.com/klzgrad/forwardproxy), padding HTTP/2) se ejecuta como sidecar supervisado por el panel: Caddyfile renderizado, start/stop/restart con reconcile de recuperación ante caídas y sonda de salud de tres niveles (process → TCP → TLS).
- **Credenciales por cliente** — cada cliente habilitado del panel obtiene automáticamente un par `basic_auth` personal (derivado del secreto del panel, sin almacenamiento); deshabilitar un cliente lo revoca en el siguiente reconcile.
- **Suscripciones** — la suscripción de cada cliente incluye su enlace personal `naive+https://` junto a los de Xray/AWG (formato estándar de NekoBox / husi / Exclave), más código QR y generador de contraseñas fuertes en el panel.
- **UX del panel** — Auto TLS (Let's Encrypt) o su propio cert/key, modo raw-Caddyfile con validación `caddy adapt`, vista previa del Caddyfile, logs del proceso, upload/download del binario.
- **Enrutar a través de Xray (opcional)** — el interruptor hace que Caddy marque destinos vía un puente SOCKS loopback oculto (`upstream socks5://127.0.0.1:…`, forward_proxy nativo — sin parche) con etiqueta `lucx-tunnel-naive`, de modo que el tráfico NaiveProxy obtiene el enrutamiento / sniffing / reglas de dominio de Xray (mismo patrón que MTProto). Por defecto sigue siendo egress directo.

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

## 🔄 Migración desde 3x-ui

LucX-UI comparte la misma base de esquema de base de datos Xray-core / SQLite (o PostgreSQL) que 3x-ui, y las tablas AWG se crean automáticamente en la primera ejecución. Para instalar sobre una configuración 3x-ui existente, primero haga una copia de seguridad de su base de datos y luego ejecute el comando de instalación estándar:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

El módulo de kernel AWG se construye automáticamente mediante el instalador (`bin/install-awg-module.sh`, DKMS). Tras la instalación, ejecute `x-ui` en la consola para confirmar la versión del módulo de kernel AWG y empezar a añadir inbounds AWG desde el panel.

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
- **Proyectos e Inspiración:** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) & [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) (sidecar de túnel NaiveProxy), [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) (referencia de diseño de integración Caddyfile), [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) (referencia del concepto de sidecar de túnel: qWDTT / olcRTC), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls).

---

## ☕ Apoyar el proyecto

LucX-UI es gratuito para uso personal. Puede apoyar el desarrollo:

| Método | Detalles |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Rusia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ Para Desarrolladores

<details>
<summary><b>Arquitectura, compilación y sincronización con upstream (clic para expandir)</b></summary>

**Arquitectura y regla de aislamiento.** Todo el código de LucX vive en paquetes aislados (`internal/awg/`, `internal/lucx/`); los cambios a archivos de 3x-ui upstream van únicamente dentro de los marcadores `// LUCX-HOOK` / `// END LUCX-HOOK` para que cada release de upstream sea un port casi trivial. Consulte [AGENTS.md](AGENTS.md) para el mapa completo de arquitectura, las 10 reglas, problemas conocidos y patrones de depuración.

**Compilación desde el código fuente** (requiere Go 1.23+, Node.js 20+, gcc — solo Linux, CGO para SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# higiene pre-push: bin/check-lucx.sh  (gofumpt sobre los 49 archivos de LucX)
```

**Procedimiento de sincronización con upstream** (validado v3.5.0→v3.6.0, 103 commits / 432 archivos / 7 conflictos):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolver bloque por bloque (ver AGENTS.md Regla 8) — nunca usar --ours/--theirs de forma indiscriminada
git grep -c "LUCX-HOOK"  # comparar conteos de marcadores antes/después para detectar bloques perdidos
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

---

## ⭐ Estrellas a lo Largo del Tiempo

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
