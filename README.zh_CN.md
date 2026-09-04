<!-- LUCX-HOOK: LucX-UI fork README — Streamlined ZH README. Keep in sync with LICENSING.md and AGENTS.md. -->
# LucX-UI

> **高级 Xray 控制面板** — AmneziaWG（内核 + 原生，至 3.1）、导入已有 AWG、受监管隧道与 Sidecar 出站（NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy）、Clash / Amnezia `vpn://` / Happ 订阅、RoscomVPN geo 与 Happ 路由。

<p align="center">
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases"><img src="https://img.shields.io/github/v/release/AlexeyLCP/lucx-ui" alt="Release"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/AlexeyLCP/lucx-ui/release.yml.svg" alt="Build"></a>
  <a href="https://github.com/AlexeyLCP/lucx-ui/releases/latest"><img src="https://img.shields.io/github/downloads/AlexeyLCP/lucx-ui/total.svg" alt="Downloads"></a>
  <a href="LICENSING.md"><img src="https://img.shields.io/badge/license-GPL--3.0%20%2B%20PolyForm--NC-blue" alt="License"></a>
  <a href="https://yoomoney.ru/to/41001989176429"><img src="https://img.shields.io/badge/donate-☕-yellow" alt="Donate"></a>
</p>

<p align="center">
  <a href="README.en_US.md">English</a> |
  <a href="README.md">Русский</a> |
  <a href="README.fa_IR.md">فارسی</a> |
  <a href="README.ar_EG.md">العربية</a> |
  <b>中文</b> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **仅限个人、非商业、科学和教育用途。** 商业用途（包括 VPN 专售或付费面板）需要根据 PolyForm Noncommercial 1.0.0 获得作者的书面许可。

---

## ⚡ 快速开始

在 **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch 等)** 上的一键安装脚本：

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

可选：GitHub 不可达时从 Yandex（SourceCraft）安装。无需 token 或 git — 面板、geo 与脚本打成一个包：

```bash
mkdir -p /tmp/lucx-dist && curl -fsSL https://codeload.sourcecraft.tech/alexeylcp/lucx-ui/tarball/refs/heads/dist | tar -xz --strip-components=1 -C /tmp/lucx-dist && sudo bash /tmp/lucx-dist/install.sh --yandex
```

之后 `x-ui update` 使用同一来源（`/etc/x-ui/install-source`）。

<details>
<summary><b>🛠️ 高级安装与配置（Cloud-Init、Docker、PostgreSQL、环境变量）</b></summary>

### 无人值守安装 (Cloud-Init)
```bash
XUI_NONINTERACTIVE=1 bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```
凭据保存在 `/etc/x-ui/install-result.env` 中。

### Docker 与 PostgreSQL
```bash
docker compose --profile postgres up -d
```

### 核心环境变量 (`/etc/default/x-ui`)
| 变量 | 描述 | 默认值 |
| --- | --- | --- |
| `XUI_DB_TYPE` | 数据库引擎 (`sqlite` 或 `postgres`) | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL 连接串 | — |
| `XUI_ENABLE_FAIL2BAN` | 启用 Fail2ban IP 限制 | `true` |
| `XUI_LOG_LEVEL` | 日志级别 (`debug`, `info`, `warning`, `error`) | `info` |

</details>

---

## 🛡️ 为什么选择 LucX-UI？

[3x-ui](https://github.com/MHSanaei/3x-ui) 是一款出色的多协议面板，前端采用现代化的 React 19 + Ant Design 6。LucX-UI 保留 3x-ui 的全部能力，并补充上游没有的部分：**内核 AmneziaWG**（与上游原生 `amneziawg` 并存）、**导入已有 AWG**、**隧道 Sidecar**（NaiveProxy · olcRTC · qWDTT · mieru · TrustTunnel · Telegram WEB proxy）、**更丰富的订阅**（Clash Meta AWG、Amnezia `vpn://`、Happ）以及 **RoscomVPN geo 包 + Happ 配置**（geodata browser 已随 [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0 进入上游）：

| 特性 | 3x-ui | LucX-UI |
|---|:---:|:---:|
| AmneziaWG 入站（通过 `awg-quick` 的内核 Sidecar） | ✗ | ✓ |
| 原生 AmneziaWG 入站（`amneziawg`，用户态） | ✓ | ✓ |
| 导入主机上已有的 AWG（awg-multi / toolza3 / Docker） | ✗ | ✓ |
| 无内核模块时 kernel AWG → 内置 amneziawg-go | ✗ | ✓ |
| 面板内 AWG 客户端/入站实时速率 | ✗ | ✓ |
| AWG CPS 混淆（TLS / DNS / SIP / QUIC + 浏览器指纹） | ✗ | ✓ |
| AWG outbound —— VPN 链式连接上游 AWG 服务器 (`awgo-N`) | ✗ | ✓ |
| AWG3 / HeaderProtectionKey | ✗ | ✓ |
| AWG 3.1（`RandomTrailers` / `DisableCookies` 抗 DPI） | ✗ | ✓ |
| 客户端配置版本预设 (1.5 / 2 / 3 / 3.1) | ✗ | ✓ |
| 面板内 AWG 诊断（路由 / NAT / peers / 握手） | ✗ | ✓ |
| NaiveProxy 隧道 Sidecar（Caddy + forward_proxy，面板监管） | ✗ | ✓ |
| 每客户端 NaiveProxy 凭证 + 订阅中的 `naive+https://` | ✗ | ✓ |
| NaiveProxy → Xray 路由（SOCKS loopback 桥接，可选） | ✗ | ✓ |
| olcRTC 隧道 Sidecar（WebRTC 会议房间，面板监管） | ✗ | ✓ |
| qWDTT 隧道 Sidecar（经 VK TURN 的 WireGuard，面板监管） | ✗ | ✓ |
| mieru 隧道 Sidecar（`mita`，每客户端流量，面板监管） | ✗ | ✓ |
| TrustTunnel Sidecar（AdGuard VPN 协议，类 HTTPS，面板监管） | ✗ | ✓ |
| Sidecar 出站（Naive / mieru / TrustTunnel 客户端 → SOCKS，路由与负载均衡） | ✗ | ✓ |
| AWG 接入 Clash Meta + Amnezia 订阅 `/awg/`（`.conf` / `vpn://`） | ✗ | ✓ |
| Geodata browser — 在面板中选择 geosite/geoip 分类 | ✓ | ✓ |
| RoscomVPN geo 包（`geoip/geosite_ROSCOM.dat`，RKN 列表） | ✗ | ✓ |
| Happ 路由配置（RoscomVPN deeplink + 自定义） | ✗ | ✓ |
| 智能集群 outbound 链接 | ✗ | ✓ |
| React 19 + AntD 6 + Vite 8 + Zod 4 前端 | ✓ | ✓（继承） |
| 所有 Xray 协议（VLESS / VMess / Trojan / Shadowsocks / ...） | ✓ | ✓ |
| Telegram WEB proxy inbound（`tproxy`，t.me/webproxy） | ✗ | ✓ |
| 无摩擦上游同步（LUCX-HOOK 隔离） | — | ✓ |

内核 Sidecar（就像 3x-ui 的 MTProto `mtg` 一样）意味着 AWG 作为真正的内核接口运行 —— 而非用户态垫片 —— 因此 Xray 通过自身的 TUN inbound 路由解密后的流量，让您在 AWG 流量上获得 Xray 完整的路由、嗅探与域名规则能力。没有模块时，同一个 LucX `awg` 入站会走内置 amneziawg-go。上游原生协议 `amneziawg` 仍在面板中并列可选。

---

## 🌟 关于 LucX-UI

**LucX-UI** 是 [3x-ui](https://github.com/MHSanaei/3x-ui) 的增强分叉（已同步上游 **v3.7.0**）。在原有 Xray 协议之外提供：两种 **AmneziaWG** —— 内核 Sidecar `awg`（思路同 MTProto/`mtg`）与上游原生 `amneziawg`，现已至 **AWG 3.1**；**导入** awg-multi / toolza3 / Docker；面板监管的 **隧道**（NaiveProxy、olcRTC、qWDTT、mieru、TrustTunnel）、扩展 **订阅**（Clash Meta AWG、Amnezia `/awg/` + `vpn://`、Happ）、**Telegram WEB proxy**（`tproxy`）以及 **预置 RoscomVPN geo**（分类浏览器与上游 v3.7.0 共用）。通过严格 `LUCX-HOOK` 隔离保持与上游 100% 兼容。

### 🛡️ AmneziaWG (AWG) 特性
- **AWG 入站与出站** —— 内核 Sidecar (`awg-quick`)、客户端模式连接上游 AWG 服务器 (`awgo-{id}`)、10 秒自动协调循环及 DKMS 内核模块构建器。
- **双引擎** —— 面板同时提供 `AmneziaWG (kernel)`（有模块时走 `awg-quick`）与上游原生 `amneziawg`。无模块时 LucX `awg` 入站走内置 amneziawg-go（SOCKS 进入 Xray）；有模块时内核路径不变。
- **导入已有 AWG** —— 入站页横幅：awg-multi / toolza3 / Docker Amnezia。密钥、IP、端口与混淆原样复制；内核接口就地改名（握手不断）。
- **实时速率** —— Clients / Inbounds 的 AWG 速率列（Xray stats 看不到 AWG）。
- **高级混淆控制** —— Lite/Standard/Pro 预设 (Jc/Jmin/Jmax/S1–S4/H1–H4)、CPS 数据包伪装 (TLS、DNS、SIP、QUIC) 及浏览器 TLS 指纹 (Chrome、Firefox、Safari)。
- **AWG3 / HeaderProtectionKey** —— AmneziaWG 3 头部保护，自动生成 32 字节密钥；服务端版本上限按客户端控制特性的下发。
- **AWG 3.1** —— `RandomTrailers`（随机包尾，按包大小抗 DPI）与 `DisableCookies`；面板更新时内核模块与工具自动升级至 v3.1。
- **客户端版本预设** —— 从单个入站为 AWG 1.5 / 2 / 3 / 3.1 生成客户端配置，挑选您的客户端应用可识别的格式。
- **真实签名抓取 (Live Capture)** —— 将真实域名的 QUIC 握手实时转换为 I1–I5 混淆参数。
- **路由与诊断** —— 双路由模式 (Kernel NAT 与带策略路由及 sniffing 的 Route through Xray) + 面板内一键诊断。

### 🚇 隧道 Sidecar（NaiveProxy、olcRTC、qWDTT、mieru、TrustTunnel、Telegram WEB proxy）
- **NaiveProxy** —— 带 `forward_proxy` 插件的 Caddy（[klzgrad](https://github.com/klzgrad/forwardproxy) 分叉，HTTP/2 padding）作为面板监管的 Sidecar 运行：渲染 Caddyfile、start/stop/restart 与崩溃自愈 reconcile，以及三级健康探测（process → TCP → TLS）。
- **每客户端凭证** —— 每个已启用的面板客户端自动获得个人 `basic_auth` 凭据对（由面板密钥派生，不落库）；禁用客户端会在下一次 reconcile 时吊销。
- **订阅** —— 每个客户端的订阅除 Xray/AWG 外还携带其个人 `naive+https://` 链接（NekoBox / husi / Exclave 标准格式），面板内另有二维码与强密码生成器。
- **面板 UX** —— Auto TLS（Let's Encrypt）或自有证书/密钥、带 `caddy adapt` 校验的 raw-Caddyfile 模式、Caddyfile 预览、进程日志、二进制上传/下载。
- **通过 Xray 路由（可选）** —— 开关使 Caddy 经隐藏 loopback SOCKS 桥接拨号（`upstream socks5://127.0.0.1:…`，原生 forward_proxy，无需补丁），标签 `lucx-tunnel-naive`，使 NaiveProxy 流量获得完整 Xray 路由/嗅探/域名规则（与 MTProto 相同）。默认仍为直连出口。
- **olcRTC** —— 经合法视频会议房间的 TCP-over-WebRTC 隧道（[openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)）：Jitsi / Yandex Telemost / WB Stream。
- **qWDTT** —— 经 VK Calls TURN 的 WireGuard（[SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)）。
- **mieru** —— 基于自定义协议而非 TLS 的抗审查代理（[enfein/mieru](https://github.com/enfein/mieru) `mita`，GPL-3.0）。多客户端、每客户端 HMAC 凭证、每客户端流量与在线统计、`mierus://` 分享链接。客户端：mieru CLI、mihomo、Clash Verge Rev、husi、Exclave。
- **TrustTunnel** —— AdGuard VPN 协议（[TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)，Apache-2.0）：流量与 HTTPS 无异（HTTP/1.1 + HTTP/2 + QUIC）。复用面板 ACME 证书（需已签发证书的域名），输出 `tt://?` deep-link 供 Flutter / CLI 客户端使用。
- **Telegram WEB proxy (`tproxy`)** —— `tproxy-server` + 官方 MTProxy + Caddy TLS reverse_proxy，监听 `hostname:443`，分享链接 `t.me/webproxy`。经 Xray 路由目前**搁置**（MTProxy 直连出口；见 lucx.211）。
- **Sidecar 出站** —— 客户端模式 Naive / mieru / TrustTunnel：粘贴分享链接（`naive+https://` / `mierus://` / `tt://`），标签会出现在路由规则与负载均衡池中（与 AWG 出站相同）。禁用 = blackhole（故障关闭，不会泄漏到 `direct`）。客户端二进制随 tar 包提供。

### 📦 订阅、Geodata 与客户端路由
- **Amnezia 订阅** — `/awg/{subId}` 返回纯 AmneziaWG `.conf`（或 `?format=vpn` → `vpn://…`）。
- **Clash Meta 中的 AWG** — 通过 `amnezia-wg-option` 输出。
- **Geodata browser** — 从路由 UI 浏览 `geoip*.dat` / `geosite*.dat`（自 [PR #6165](https://github.com/MHSanaei/3x-ui/pull/6165) / v3.7.0 进入上游，[STRENCH0](https://github.com/STRENCH0)）。
- **RoscomVPN geo 包** — 库存 `geoip_ROSCOM.dat` / `geosite_ROSCOM.dat`（[roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip) / [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite)）。
- **Happ 路由配置** — Settings → Happ：内置 RoscomVPN deeplink（[roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing)）。

### 🚀 3x-ui 核心特性
- **协议支持：** VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS, TUN。
- **传输与安全：** REALITY, TLS, XTLS, gRPC, WebSocket, XHTTP, Fallbacks。
- **面板管理：** 流量限额、IP 限制 (Fail2ban)、在线状态、订阅服务、Telegram 机器人、REST API、多节点支持、SQLite / PostgreSQL。

<details>
<summary><b>📸 面板截图</b></summary>

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

## 🔄 从 3x-ui 与已有 AWG 迁移

LucX-UI 与 3x-ui 共享相同的 Xray-core / SQLite（或 PostgreSQL）数据库架构基础，AWG 表会在首次运行时自动创建。要在已有的 3x-ui 安装上覆盖安装，请先备份数据库，然后运行标准安装命令：

```bash
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

AWG 内核模块由安装脚本 (`bin/install-awg-module.sh`, DKMS) 自动构建。安装完成后，在控制台运行 `x-ui` 以确认 AWG 内核模块版本，并从面板开始添加 AWG 入站。

**安装后：** 订阅端点（`/sub/`、`/json/`、`/clash/`、`/awg/`）监听**独立端口**（默认 **2096**），不是面板端口 — 反向代理也必须转发该端口。自定义 geo 分组请使用**单独文件名** — 预置名（`geoip.dat` / `geosite.dat` 以及 `_IR` / `_RU` / `_ROSCOM`）会在 geofile 更新时被覆盖。

### 从主机上已有的 AWG

若服务器上已在运行 **awg-multi**、**toolza3** 或 **Docker Amnezia**，面板**不会拆除**外来的 `awg0`/`awg1`。入站页会出现 **「导入已有 AWG」** 横幅：预览对等端 → 每个接口一个入站。密钥 / IP / 端口 / 混淆原样复制。内核接口就地改名（`awg{id}`），握手不断。用户态/Docker：先停掉旧管理器，客户端会重连一次。

没有内核模块时，LucX `awg` 入站仍会通过内置 amneziawg-go 启动。上游原生协议 `amneziawg` 在面板中并列可用。

---

## 📜 许可证与条款

本项目自有代码遵循**双重许可证**，第三方二进制/数据遵循各自上游条款（完整矩阵见 [LICENSING.md](LICENSING.md)）：

| 组件 | 许可证 |
|---|---|
| 原始 3x-ui 代码库 | **GPL-3.0** |
| LucX-UI 组件 (`internal/awg/`、`internal/lucx/`、LucX 前端页面) | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-*`（Caddy） | **Apache-2.0** |
| `forward_proxy` 插件（[klzgrad](https://github.com/klzgrad/forwardproxy)） | **MIT** |
| NaiveProxy / `bin/naive-client-*`（[klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy)） | **BSD-3-Clause** |
| `bin/olcrtc-*`（[openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)） | **WTFPL** |
| `bin/qwdtt-*`（[SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android)） | **GPL-3.0** |
| `bin/mieru-*`（`mita`，[enfein/mieru](https://github.com/enfein/mieru)） | **GPL-3.0** |
| `bin/trusttunnel-*`（[TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel)） | **Apache-2.0** |
| AmneziaWG 内核模块与工具（[amnezia-vpn](https://github.com/amnezia-vpn)） | **GPL-2.0**（模块；安装在主机） |
| 预置 geo `.dat`（Loyalsoldier / IR / RU / ROSCOM） | 各数据集上游（见 LICENSING.md） |

隧道二进制为**子进程**——面板不链接它们。qWDTT 的 GPL 适用于该二进制及其源码，不适用于 LucX 的 PolyForm 代码。

---

## 🤝 致谢与来源

感谢所有开源项目与贡献者。

### 测试者与贡献者
- **VladufQa**, **Kirill Rudenko** ([PR #13](https://github.com/AlexeyLCP/lucx-ui/pull/13)), **302ba (Alex)** ([PR #24](https://github.com/AlexeyLCP/lucx-ui/pull/24)), **Aleksandr SacredX**, **alireza0**, **[3x-ui](https://github.com/MHSanaei/3x-ui)** 团队。

### 移植 / 依赖的上游 PR
- **[STRENCH0](https://github.com/STRENCH0)** — [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) geodata browser。

### 项目与灵感
[MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) · [amnezia-vpn](https://github.com/amnezia-vpn) · [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) / [forwardproxy](https://github.com/klzgrad/forwardproxy) · [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) · [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) · [enfein/mieru](https://github.com/enfein/mieru) · [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) · [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive) · [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui) · [hydraponique](https://github.com/hydraponique) RoscomVPN（[geoip](https://github.com/hydraponique/roscomvpn-geoip) / [geosite](https://github.com/hydraponique/roscomvpn-geosite) / [routing](https://github.com/hydraponique/roscomvpn-routing)） · [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) · [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules) · [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) · [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) · [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) · [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) · [refraction-networking/utls](https://github.com/refraction-networking/utls)

---

## ☕ 支持本项目

LucX-UI 个人使用完全免费。**喜欢的话请点 ⭐** — 能帮助更多人发现本项目。捐赠可选：

| 方式 | 详情 |
|---|---|
| ⭐ **GitHub Star** | [Star AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) |
| 🇷🇺 **YooMoney** (卢布, 俄罗斯) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## 🛠️ 开发者指南

<details>
<summary><b>架构、构建与上游同步（点击展开）</b></summary>

**架构与隔离规则。** 所有 LucX 代码都位于隔离的包中（`internal/awg/`, `internal/lucx/`）；对上游 3x-ui 文件的修改仅放在 `// LUCX-HOOK` / `// END LUCX-HOOK` 标记之间，从而使每次上游发布都近乎平凡的移植。请参阅 [AGENTS.md](AGENTS.md) 了解完整的架构图、10 条规则、已知问题与调试模式。

**源码构建**（需 Go 1.27+、Node.js 24+、gcc —— 仅 Linux，CGO 用于 SQLite）：

```bash
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
# 推送前清理：bin/check-lucx.sh  (LUCX-HOOK + internal/awg|lucx)
```

**上游同步流程**（当前基线 — 上游 **v3.7.0**；合并上游 tags/main，不要用旧的 v3.5→v3.6）：

```bash
git fetch origin --tags
git merge --no-commit --no-ff origin/main
# 逐块解决（见 AGENTS.md 规则 8）—— 切勿一刀切使用 --ours/--theirs
git grep -c "LUCX-HOOK"  # 对比前后标记数量以检测丢失的块
go build ./... && go vet ./... && go test ./internal/awg/... ./internal/lucx/...
```

</details>

<!-- END LUCX-HOOK -->
