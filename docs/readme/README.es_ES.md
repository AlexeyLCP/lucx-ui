<!-- LUCX-HOOK: LucX-UI fork README — Streamlined ES README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **Panel avanzado de Xray** — AmneziaWG (kernel + nativo, hasta 3.1), importar AWG existente, túneles supervisados y sidecar outbounds (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy), suscripciones Clash / Amnezia `vpn://` / Happ, RoscomVPN geo y enrutamiento Happ.

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="../../LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
</p>

<p align="center">
  <a href="README.en_US.md">English</a> |
  <a href="../../README.md">Русский</a> |
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

Instalación opcional desde Yandex (SourceCraft) si GitHub no está disponible. Sin tokens ni git — panel, geo y scripts en un solo paquete:

```bash
mkdir -p /tmp/lucx-dist && curl -fsSL https://codeload.sourcecraft.tech/alexeylcp/lucx-ui/tarball/refs/heads/dist | tar -xz --strip-components=1 -C /tmp/lucx-dist && sudo bash /tmp/lucx-dist/install.sh --yandex
```

Después `x-ui update` usa la misma fuente (`/etc/x-ui/install-source`).

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

[3x-ui](https://github.com/MHSanaei/3x-ui) es un excelente panel multiprotocolo con un frontend moderno en React 19 + Ant Design 6. LucX-UI mantiene todo lo de 3x-ui y añade lo que upstream no tiene: **AmneziaWG de kernel** (junto al `amneziawg` nativo de upstream), **importación de AWG existente**, **sidecars de túnel** (NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy), **suscripciones ampliadas** (Clash Meta AWG, Amnezia `vpn://`, Happ) y **packs RoscomVPN geo + perfiles Happ** (el geodata browser ya está en upstream desde [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0):

<details>
<summary><b>Comparación con 3x-ui</b></summary>

| Característica | 3x-ui | LucX-UI |
|---|:---:|:---:|
| Inbound AmneziaWG (sidecar de kernel vía `awg-quick`) | ✗ | ✓ |
| Inbound AmneziaWG nativo (`amneziawg`, userspace) | ✓ | ✓ |
| Importar AWG existente del host (awg-multi / toolza3 / Docker) | ✗ | ✓ |
| Kernel AWG sin módulo → amneziawg-go embebido | ✗ | ✓ |
| Velocidad en vivo de clientes/inbounds AWG en el panel | ✗ | ✓ |
| Ofuscación AWG CPS (TLS / DNS / SIP / QUIC + huellas de navegador) | ✗ | ✓ |
| AWG outbound — encadenamiento VPN a servidores AWG externos (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1 (`RandomTrailers` / `DisableCookies`, anti-DPI) | ✗ | ✓ |
| Presets de versión de configuración cliente (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| Diagnóstico AWG en panel (enrutamiento / NAT / peers / handshakes) | ✗ | ✓ |
| Sidecar de túnel NaiveProxy (Caddy + forward_proxy, supervisado) | ✗ | ✓ |
| Credenciales NaiveProxy por cliente + `naive+https://` en suscripciones | ✗ | ✓ |
| NaiveProxy → enrutamiento Xray (puente SOCKS loopback, opcional) | ✗ | ✓ |
| Sidecar olcRTC (WebRTC vía salas meet, supervisado) | ✗ | ✓ |
| Sidecar qWDTT (WireGuard sobre VK TURN, supervisado) | ✗ | ✓ |
| Sidecar mieru (`mita`, tráfico por cliente, supervisado) | ✗ | ✓ |
| Sidecar TrustTunnel (protocolo AdGuard VPN, tipo HTTPS, supervisado) | ✗ | ✓ |
| Sidecar outbounds (cliente Naive / mieru / TrustTunnel → SOCKS, routing y pools) | ✗ | ✓ |
| AWG en Clash Meta + suscripción Amnezia `/awg/` (`.conf` / `vpn://`) | ✗ | ✓ |
| Geodata browser — categorías geosite/geoip desde el panel | ✓ | ✓ |
| Pack geo RoscomVPN (`geoip/geosite_ROSCOM.dat`) | ✗ | ✓ |
| Perfiles de enrutamiento Happ (RoscomVPN deeplink + custom) | ✗ | ✓ |
| Enlaces outbound de clúster inteligente | ✗ | ✓ |
| Frontend React 19 + AntD 6 + Vite 8 + Zod 4 | ✓ | ✓ (heredado) |
| Todos los protocolos Xray (VLESS / VMess / Trojan / Shadowsocks / ...) | ✓ | ✓ |
| Inbound proxy WEB de Telegram (`tproxy`, t.me/webproxy) | ✗ | ✓ |
| Sincronización con upstream sin fricción (aislamiento LUCX-HOOK) | — | ✓ |

</details>

Un sidecar de kernel (como el `mtg` de MTProto en 3x-ui) significa que AWG se ejecuta como una interfaz de kernel real — no un shim de espacio de usuario — por lo que Xray enruta el tráfico descifrado a través de su propio TUN inbound, dándole todo el poder de enrutamiento, sniffing y reglas de dominio de Xray sobre el tráfico AWG. Sin módulo, el mismo inbound LucX `awg` corre sobre amneziawg-go embebido. El protocolo nativo de upstream `amneziawg` sigue en el panel a su lado.

---

## 🌟 Acerca de LucX-UI

**LucX-UI** es un fork mejorado de [3x-ui](https://github.com/MHSanaei/3x-ui) (sincronizado con upstream **v3.7.0**). Además de los protocolos Xray de stock: **AmneziaWG** en dos modos — sidecar de kernel `awg` (como MTProto/`mtg`) y el `amneziawg` nativo de upstream, hasta **AWG 3.1**; **importación** de awg-multi / toolza3 / Docker; **túneles supervisados** (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel), **suscripciones ampliadas** (Clash Meta AWG, Amnezia `/awg/` + `vpn://`, Happ), **proxy WEB de Telegram** (`tproxy`) y **geo RoscomVPN de stock** (el browser de categorías es compartido con upstream v3.7.0). Compatibilidad 100% con upstream vía aislamiento `LUCX-HOOK`.

<details>
<summary><b>🛡️ Características de AmneziaWG (AWG)</b></summary>

- **Inbounds y Outbounds AWG** — Sidecar de kernel (`awg-quick`), conexión en modo cliente a servidores AWG externos (`awgo-{id}`), ciclo de conciliación automática de 10 segundos y creador de módulos DKMS.
- **Dos motores** — `AmneziaWG (kernel)` vía `awg-quick` si hay módulo, y el `amneziawg` nativo de upstream. Sin módulo, los inbounds LucX `awg` corren sobre amneziawg-go embebido (SOCKS hacia Xray); el camino de kernel no cambia cuando el módulo está.
- **Importar AWG existente** — Banner en Inbounds: awg-multi / toolza3 / Docker Amnezia. Claves, IPs, puerto y ofuscación se copian tal cual; la iface de kernel se renombra en el sitio (los handshakes siguen).
- **Velocidad en vivo** — Columnas de velocidad en Clients / Inbounds para AWG (las stats de Xray no lo ven).
- **Ofuscación Avanzada** — Perfiles Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4), mimatización de paquetes CPS (TLS, DNS, SIP, QUIC) y huellas TLS de navegador (Chrome, Firefox, Safari).
- **AWG3 / HeaderProtectionKey** — Protección de cabecera AmneziaWG 3 con claves de 32 bytes autogeneradas; el techo de versión del lado del servidor controla la emisión de características por cliente.
- **AWG 3.1** — `RandomTrailers` (cola de paquete aleatoria, anti-DPI por tamaño) y `DisableCookies`; el módulo de kernel y las herramientas se actualizan automáticamente a v3.1 al actualizar el panel.
- **Presets de Versión de Cliente** — Genere configs de cliente para AWG 1.5 / 2 / 3 / 3.1 desde un solo inbound — elija el formato que su app cliente entienda.
- **Captura de Firma en Vivo** — Convierte saludos QUIC reales de dominios frontales en parámetros I1–I5.
- **Enrutamiento y Diagnóstico** — Dos modos (Kernel NAT y Route through Xray con policy routing y sniffing) + diagnóstico en panel con un solo clic.

</details>

<details>
<summary><b>🚇 Sidecars de túnel (NaiveProxy, olcRTC, qWDTT, mieru, TrustTunnel, Telegram WEB proxy)</b></summary>

- **NaiveProxy** — Caddy con el plugin `forward_proxy` (fork de [klzgrad](https://github.com/klzgrad/forwardproxy), padding HTTP/2) se ejecuta como sidecar supervisado por el panel: Caddyfile renderizado, start/stop/restart con reconcile de recuperación ante caídas y sonda de salud de tres niveles (process → TCP → TLS).
- **Credenciales por cliente** — cada cliente habilitado del panel obtiene automáticamente un par `basic_auth` personal (derivado del secreto del panel, sin almacenamiento); deshabilitar un cliente lo revoca en el siguiente reconcile.
- **Suscripciones** — la suscripción de cada cliente incluye su enlace personal `naive+https://` junto a los de Xray/AWG (formato estándar de NekoBox / husi / Exclave), más código QR y generador de contraseñas fuertes en el panel.
- **UX del panel** — Auto TLS (Let's Encrypt) o su propio cert/key, modo raw-Caddyfile con validación `caddy adapt`, vista previa del Caddyfile, logs del proceso, upload/download del binario.
- **Enrutar a través de Xray (opcional)** — el interruptor hace que Caddy marque destinos vía un puente SOCKS loopback oculto (`upstream socks5://127.0.0.1:…`, forward_proxy nativo — sin parche) con etiqueta `lucx-tunnel-naive`, de modo que el tráfico NaiveProxy obtiene el enrutamiento / sniffing / reglas de dominio de Xray (mismo patrón que MTProto). Por defecto sigue siendo egress directo.
- **olcRTC** — túnel TCP-over-WebRTC vía sala de videollamada legal ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)).
- **qWDTT** — WireGuard por TURN de VK Calls ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)).
- **mieru** — proxy resistente a la censura sobre un protocolo propio en vez de TLS ([enfein/mieru](https://github.com/enfein/mieru) `mita`, GPL-3.0). Multicliente con credenciales HMAC por cliente del panel, tráfico y estado online por cliente, y enlace `mierus://`. Clientes: mieru CLI, mihomo, Clash Verge Rev, husi, Exclave.
- **TrustTunnel** — el protocolo de AdGuard VPN ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel), Apache-2.0): tráfico indistinguible de HTTPS (HTTP/1.1 + HTTP/2 + QUIC). Reutiliza el certificado ACME del panel (requiere un dominio con cert emitido) y emite un deep-link `tt://?` para los clientes Flutter / CLI.
- **Proxy WEB de Telegram (`tproxy`)** — `tproxy-server` + MTProxy oficial + Caddy TLS reverse_proxy en `hostname:443`, enlace `t.me/webproxy`. El enrutado vía Xray está **aparado** (egress directo de MTProxy; ver lucx.211).
- **Sidecar outbounds** — modo cliente Naive / mieru / TrustTunnel: pega un enlace (`naive+https://` / `mierus://` / `tt://`), el tag aparece en reglas de routing y pools de balanceo (igual que AWG outbound). Desactivar = blackhole (fail-closed, no filtra a `direct`). Binarios de cliente en el tar.gz.

</details>

<details>
<summary><b>📦 Suscripciones, geodata y enrutamiento de clientes</b></summary>

- **Suscripción Amnezia** — `/awg/{subId}` devuelve `.conf` puro o `vpn://…`.
- **AWG en Clash Meta** — peers vía `amnezia-wg-option`.
- **Geodata browser** — explorar `geoip*.dat` / `geosite*.dat` desde el UI de routing (en upstream desde [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0, [STRENCH0](https://github.com/STRENCH0)).
- **Pack RoscomVPN geo** — `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` ([roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip) / [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)).
- **Perfiles Happ** — Settings → Happ: deeplink RoscomVPN ([roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)).

</details>

<details>
<summary><b>🚀 Características Base de 3x-ui</b></summary>

- **Protocolos:** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN.
- **Seguridad y Transportes:** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks.
- **Gestión:** Cuotas de tráfico, límites IP (Fail2ban), estado en línea, suscripciones, bot de Telegram, API REST, multinodo, SQLite / PostgreSQL.

</details>

<details>
<summary><b>📸 Capturas de pantalla</b></summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../../media/01-overview-dark.png">
  <img alt="Overview" src="../../media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../../media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="../../media/02-add-inbound-light.png">
</picture>

</details>

---

## 🔄 Migración desde 3x-ui y AWG existente

LucX-UI comparte la misma base de esquema de base de datos Xray-core / SQLite (o PostgreSQL) que 3x-ui, y las tablas AWG se crean automáticamente en la primera ejecución. Para instalar sobre una configuración 3x-ui existente, primero haga una copia de seguridad de su base de datos y luego ejecute el comando de instalación estándar:

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

El módulo de kernel AWG se construye automáticamente mediante el instalador (`bin/install-awg-module.sh`, DKMS). Tras la instalación, ejecute `x-ui` en la consola para confirmar la versión del módulo de kernel AWG y empezar a añadir inbounds AWG desde el panel.

**Tras instalar:** los endpoints de suscripción (`/sub/`, `/json/`, `/clash/`, `/awg/`) escuchan en un **puerto aparte** (por defecto **2096**), no el del panel — el reverse proxy debe reenviarlo también. Guarde grupos geo personalizados con un **nombre de archivo distinto** — los nombres de stock (`geoip.dat` / `geosite.dat` y `_IR` / `_RU` / `_ROSCOM`) se sobrescriben al actualizar geofile.

<details>
<summary><b>Desde AWG existente en el host</b></summary>

Si el servidor ya ejecuta **awg-multi**, **toolza3** o **Docker Amnezia**, el panel **no derriba** las ifaces ajenas `awg0`/`awg1`. En Inbounds aparece el banner **Importar AWG existente**: previsualizar peers → un inbound por interfaz. Claves / IPs / puerto / ofuscación se copian tal cual. Una iface de kernel se renombra en el sitio (`awg{id}`) — los handshakes siguen. Userspace/Docker: detenga el gestor antiguo; esos clientes se reconectan una vez.

Sin módulo de kernel, los inbounds LucX `awg` siguen levantándose sobre amneziawg-go embebido. El protocolo nativo de upstream `amneziawg` está en el panel a su lado.

</details>

---

## 📜 Licencia y Términos

Este proyecto se publica bajo **dos licencias** para el código propio, más binarios/datos de terceros bajo sus términos upstream (matriz completa en [LICENSING.md](../../LICENSING.md)):

<details>
<summary><b>Matriz de licencias</b></summary>

| Componente | Licencia |
|---|---|
| Código base original 3x-ui | **GPL-3.0** |
| Componentes LucX-UI (`internal/awg/`, `internal/lucx/`, páginas LucX del frontend) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*` (Caddy) | **Apache-2.0** |
| Plugin `forward_proxy` ([klzgrad](https://github.com/klzgrad/forwardproxy)) | **MIT** |
| NaiveProxy / `bin/naive-client-*` ([klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)) | **BSD-3-Clause** |
| `bin/olcrtc-*` ([openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)) | **WTFPL** |
| `bin/qwdtt-*` ([SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)) | **GPL-3.0** |
| `bin/mieru-*` (`mita`, [enfein/mieru](https://github.com/enfein/mieru)) | **GPL-3.0** |
| `bin/trusttunnel-*` ([TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)) | **Apache-2.0** |
| AmneziaWG kernel module & tools ([amnezia-vpn](https://github.com/amnezia-vpn)) | **GPL-2.0** (módulo; se instala en el host) |
| Geo `.dat` de stock (Loyalsoldier / IR / RU / ROSCOM) | Upstream de cada dataset (ver LICENSING.md) |

Los binarios de túnel son **procesos hijos** — el panel no los enlaza. El GPL de qWDTT aplica a ese binario y sus fuentes, no al código PolyForm de LucX.

</details>

---

## 🤝 Agradecimientos y Créditos

Gracias a todos los proyectos y personas open-source.

<details>
<summary><b>Probadores y colaboradores</b></summary>

- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13)), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, equipo **[3x-ui](https://github.com/MHSanaei/3x-ui)**.

</details>

<details>
<summary><b>PRs upstream portados</b></summary>

- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) geodata browser.

</details>

<details>
<summary><b>Proyectos e inspiración</b></summary>

[MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) · [amnezia-vpn](https://github.com/amnezia-vpn) · [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) / [forwardproxy](https://github.com/klzgrad/forwardproxy) · [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) · [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) · [enfein/mieru](https://github.com/enfein/mieru) · [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) · [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) · [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) · [hydraponique](https://github.com/hydraponique) RoscomVPN ([geoip](https://github.com/hydraponique/roscomvpn-geoip) / [geosite](https://github.com/hydraponique/roscomvpn-geosite) / [routing](https://github.com/hydraponique/roscomvpn-routing)) · [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) · [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules) · [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) · [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) · [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) · [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) · [refraction-networking/utls](https://github.com/refraction-networking/utls)

</details>

---

## ☕ Apoyar el proyecto

LucX-UI es gratuito para uso personal. **¿Te gustó? Pon una ⭐** al repositorio — ayuda a que otros lo encuentren. Las donaciones son opcionales:

<details>
<summary><b>Donaciones</b></summary>

| Método | Detalles |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🇷🇺 **YooMoney** (RUB, Rusia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

</details>

---

## 🛠️ Para Desarrolladores

<details>
<summary><b>Arquitectura, compilación y sincronización con upstream (clic para expandir)</b></summary>

**Arquitectura y regla de aislamiento.** Todo el código de LucX vive en paquetes aislados (`internal/awg/`, `internal/lucx/`); los cambios a archivos de 3x-ui upstream van únicamente dentro de los marcadores `// LUCX-HOOK` / `// END LUCX-HOOK` para que cada release de upstream sea un port casi trivial. Consulte [AGENTS.md](../../AGENTS.md) para el mapa completo de arquitectura, las 10 reglas, problemas conocidos y patrones de depuración.

**Compilación desde el código fuente** (requiere Go 1.27+, Node.js 24+, gcc — solo Linux, CGO para SQLite):

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# higiene pre-push: bin/check-lucx.sh  (LUCX-HOOK + internal/awg|lucx)
```

**Procedimiento de sincronización con upstream** (base actual — upstream **v3.7.0**; fusionar tags/main de upstream, no el viejo v3.5→v3.6):

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# resolver bloque por bloque (ver AGENTS.md Regla 8) — nunca usar --ours/--theirs de forma indiscriminada
git grep -c "LUCX-HOOK"  # comparar conteos de marcadores antes/después para detectar bloques perdidos
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

<!-- END LUCX-HOOK -->
