<!-- LUCX-HOOK: LucX-UI fork README — Unified ES README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **Solo para uso personal, no comercial, científico, de investigación y educativo.** El uso comercial, incluida la reventa de VPN, paneles de pago o servicios de suscripción basados en este código, requiere un permiso explícito por escrito del autor. No lo utilice para fines ilegales.

---

## Acerca de LucX-UI

**LucX-UI** es un panel de control web avanzado para administrar servidores [Xray-core](https://github.com/XTLS/Xray-core), creado como un fork mejorado de [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) con soporte nativo de **AmneziaWG (AWG)**. AWG funciona como un sidecar a nivel de interfaz de kernel, reflejando exactamente la arquitectura utilizada en el proyecto original para MTProto (mtg): el panel gestiona el ciclo de vida y la contabilidad de tráfico, y Xray puede enrutar el tráfico opcionalmente.

### Características Principales

#### 🛡️ Mejoras de AmneziaWG (AWG)
- **Inbounds AWG** — Sidecar de kernel mediante `awg-quick`: creación, conciliación automática cada 10 segundos, limpieza de interfaces huérfanas e instalador de módulos del kernel vía DKMS.
- **Outbounds AWG (Modo Cliente)** — El panel puede conectarse directamente a un servidor AmneziaWG de nivel superior: pestaña dedicada en la sección Xray, pegado de archivos `.conf` e interfaz de kernel `awgo-{id}` gestionada por el ciclo de conciliación. Inyecta un outbound `freedom` con `sockopt.interface` en Xray para enrutar el tráfico a través del VPN superior.
- **Control de Ofuscación** — Perfiles Lite/Standard/Pro (Jc/Jmin/Jmax/S1–S4/H1–H4) y mimatización de paquetes CPS: TLS, DNS, SIP y QUIC.
- **Huellas TLS de Navegador** — Chrome (GREASE), Firefox 120+ (orden NSS y padding) y Safari 16+ (orden Apple y TLS 1.1) para TLS y QUIC.
- **Captura de Firma en Vivo** — Convierte un saludo QUIC real de un dominio frontal en parámetros I1–I5.
- **Gestión de Clientes** — Códigos QR, descarga de `.conf` y contabilidad de tráfico individual por cliente (`awg show transfer`).
- **Dos Modos de Enrutamiento**:
  - **Kernel NAT** — Reenvío directo por el kernel; las reglas NAT se autoreparan tras un reinicio de iptables.
  - **Route through Xray** — El tráfico fluye a través de todo el canal de enrutamiento de Xray (reglas de dominio/geosite, balanceadores, outbounds en cadena) mediante un inbound TUN con enrutamiento por políticas y sniffing.
- **Diagnósticos Integrados** — Comprobación con un solo clic en el formulario de inbound: estado de interfaz, ip_forward, clientes/saludos y reglas NAT/TUN.

#### 🚀 Características Base de 3x-ui
- **Inbounds multiprotocolo** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixto) y TUN.
- **Transportes y seguridad modernos** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade y XHTTP, protegidos con TLS, XTLS y REALITY.
- **Fallbacks** — Sirve múltiples protocolos en un solo puerto (ej. VLESS y Trojan en el 443).
- **Gestión de clientes** — Cuotas de tráfico, fechas de expiración, límites de IP, estado en línea y enlaces/QR/suscripciones en un clic.
- **Estadísticas de tráfico** — Por inbound, por cliente y por outbound.
- **Soporte multinodo** — Gestione y escale en múltiples servidores desde un solo panel.
- **Enrutamiento y outbounds** — WARP, NordVPN, reglas personalizadas, balanceadores y cadenas de proxies.
- **Servidor de suscripciones** con plantillas personalizadas.
- **Bot de Telegram** para monitoreo remoto.
- **API RESTful** con documentación Swagger integrada.
- **Almacenamiento flexible** — SQLite (predeterminado) o PostgreSQL.
- **Integración con Fail2ban** para aplicar límites de IP por cliente.

### Capturas de Pantalla

<details>
<summary>Haga clic para expandir</summary>

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

## Inicio Rápido

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Instala el panel desde la [última versión](https://github.com/AlexeyLCP/lucx-ui/releases/latest), el servicio systemd, Xray-core y mtg, y compila el módulo de kernel AmneziaWG mediante DKMS (`bin/install-awg-module.sh`).

Durante la instalación se generan credenciales aleatorias. Tras la instalación, ejecute `x-ui` para abrir el menú.

### Instalación desatendida

El instalador admite **modo no interactivo** para cloud-init. Establezca `XUI_NONINTERACTIVE=1` para una instalación automática sin preguntas, guardando los datos en `/etc/x-ui/install-result.env`. Consulte [`deploy/`](deploy/).

## Plataformas Soportadas

**Sistemas operativos:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine y Windows.

**Arquitecturas:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Opciones de Base de Datos

3X-UI soporta dos motores de base de datos:

- **SQLite** (predeterminado) — archivo `/etc/x-ui/x-ui.db`.
- **PostgreSQL** — recomendado para gran cantidad de clientes o nodos distribuidos.

Variables en `/etc/default/x-ui`:
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Docker

Para usar PostgreSQL en Docker, descomente las líneas `XUI_DB_*` en `docker-compose.yml` y ejecute:
```bash
docker compose --profile postgres up -d
```

## Variables de Entorno

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `XUI_DB_TYPE` | Motor de BD: `sqlite` o `postgres` | `sqlite` |
| `XUI_DB_DSN` | Cadena de conexión PostgreSQL (cuando `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Directorio de la base de datos SQLite | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | Activa Fail2ban para límite de IP | `true` |
| `XUI_LOG_LEVEL` | Nivel de registros (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Activa el monitor de salud del túnel | `false` |

## Licencia y Términos

Este proyecto se publica bajo **dos licencias** (detalles en [LICENSING.md](LICENSING.md)):

| Componente | Licencia |
|---|---|
| Código original 3x-ui | **GPL-3.0** (requerido por el proyecto base) |
| Componentes LucX (`internal/awg/`, `internal/lucx/`, frontend AWG, scripts) | **PolyForm Noncommercial 1.0.0** |

**Gratuito** para uso personal, no comercial, científico, de investigación y educativo. El **uso comercial** (reventa de VPN, paneles de pago, integración comercial) requiere permiso por escrito del autor: abra un [issue](https://github.com/AlexeyLCP/lucx-ui/issues) o contacte al propietario. Los encabezados `SPDX-License-Identifier` definen los límites.

## Contribuciones

Las contribuciones son bienvenidas. Lea la [Guía de Contribución](/CONTRIBUTING.md) antes de enviar un issue o PR.

## Agradecimientos y Créditos

### Probadores y Colaboradores
- **VladufQa** — Pruebas en VPS real (ruvds): primeros saludos, tráfico, cascadas y reportes de enrutamiento.
- **Kirill Rudenko** — Pruebas (runode) y **PR #13**: AWG needRestart, enrutamiento iif, tablas/gateways independientes, recuperación de rutas y sniffing.
- **302ba (Alex)** — **PR #24**: Corrección de pérdida de campos del cliente al procesar el esquema Zod.
- **alireza0** — Colaborador del proyecto base.
- El equipo de **3x-ui** — Por la excelente base y arquitectura de sidecar que tomamos de modelo.

### Fuentes e Inspiración
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — Base del fork (GPL-3.0), referencia de arquitectura sidecar MTProto.
- [AmneziaVPN](https://github.com/amnezia-vpn) — Protocolo AmneziaWG y módulo de kernel.
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — Patrón PostUp NAT (MASQUERADE + FORWARD), generadores QUIC Initial e instalación DKMS.
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — Captura de firmas QUIC (`internal/awg/signature/`).
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) y [refraction-networking/utls](https://github.com/refraction-networking/utls) — Perfiles TLS de navegador.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) y [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) — Reglas de enrutamiento.

### Herramientas de la Comunidad
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Licencia: **MIT**): Gestión de inbounds y configuración del panel mediante código.

## ☕ Apoyar el proyecto

LucX-UI es gratuito para uso personal y no comercial. Si el panel le ahorra tiempo, puede apoyar el desarrollo:

| Método | Detalles |
|---|---|
| 🇷🇺 **YooMoney** (RUB, Rusia) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

Las donaciones son un agradecimiento, no una compra: no otorgan licencia comercial ni modifican los términos de [LICENSING.md](LICENSING.md).

## Estrellas a lo Largo del Tiempo

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)

<!-- END LUCX-HOOK -->
