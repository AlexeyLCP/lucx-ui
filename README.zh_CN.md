<!-- LUCX-HOOK: LucX-UI fork README — Unified ZH README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
  <b>中文</b> |
  <a href="README.es_ES.md">Español</a> |
  <a href="README.tr_TR.md">Türkçe</a>
</p>

> [!WARNING]
> **仅限个人、非商业、科学、研究和教育用途。** 商业用途（包括 VPN 专售、付费面板或基于此代码构建的订阅服务）需要获得作者的明确书面许可。请勿用于非法目的。

---

## 关于 LucX-UI

**LucX-UI** 是一款用于管理 [Xray-core](https://github.com/XTLS/Xray-core) 服务器的高级 Web 控制面板，基于 [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) 进行增强分叉，原生支持 **AmneziaWG (AWG)**。AWG 作为内核接口 Sidecar 运行 —— 完全对齐上游 MTProto (mtg) 的架构：面板负责生命周期和流量统计，Xray 可选进行路由。

### 主要特性

#### 🛡️ AmneziaWG (AWG) 增强
- **AWG 入站 (Inbounds)** —— 基于 `awg-quick` 的内核 Sidecar：创建、每 10 秒自动协调 (reconcile)、清理残留接口及 DKMS 内核模块安装器。
- **AWG 出站 (Outbounds / 客户端模式)** —— 面板可直接连接上游 AmneziaWG 服务器：Xray 设置中的独立标签页、粘贴 `.conf` 文件，并由 reconcile 循环管理 `awgo-{id}` 内核接口。在 Xray 配置中注入带 `sockopt.interface` 的 `freedom` 出站，以便路由规则和负载均衡器将流量转发至上游 VPN。
- **混淆控制 (Obfuscation)** —— Lite/Standard/Pro 预设 (Jc/Jmin/Jmax/S1–S4/H1–H4) 以及 CPS 数据包伪装：TLS、DNS、SIP 和 QUIC。
- **浏览器 TLS 指纹** —— 针对 TLS 和 QUIC 的 Chrome (GREASE)、Firefox 120+ (NSS 排序与 padding) 和 Safari 16+ (Apple 排序与 TLS 1.1) 指纹。
- **抓取真实主机签名** —— 将真实域名的 QUIC 握手实时转换为 I1–I5 混淆参数。
- **客户端管理** —— QR 码、下载 `.conf` 文件及按节点流量统计 (`awg show transfer`)。
- **两种路由模式**:
  - **Kernel NAT** —— 直接内核转发；NAT 规则在 iptables 清除后通过 reconcile 循环自动恢复。
  - **Route through Xray** —— 流量通过 TUN 入站、策略路由和 sniffing，进入 Xray 完整的路由管道（域名/geosite 规则、负载均衡器、链式出站）。
- **面板内置诊断** —— 入站表单中的一键诊断功能：检查接口状态、ip_forward、对等节点/握手及 NAT/TUN 规则。

#### 🚀 3x-ui 核心特性
- **多协议入站** —— VLESS、VMess、Trojan、Shadowsocks、WireGuard、Hysteria2、HTTP、SOCKS (混合) 和 TUN。
- **现代传输与安全协议** —— TCP (Raw)、mKCP、WebSocket、gRPC、HTTPUpgrade 和 XHTTP，支持 TLS、XTLS 和 REALITY。
- **回落 (Fallbacks)** —— 利用 Xray 的回落功能在单端口上提供多种协议。
- **客户端管理** —— 流量限额、到期时间、IP 限制、在线状态以及一键分享链接、二维码和订阅。
- **流量统计** —— 按入站、客户端和出站统计流量。
- **多节点支持** —— 从单个面板管理和扩展多个服务器。
- **出站与路由** —— WARP、NordVPN、自定义路由规则、负载均衡器和代理链。
- **订阅服务器** —— 支持多种输出格式和自定义页面模板。
- **Telegram 机器人** —— 用于远程监控和管理。
- **RESTful API** —— 提供面板内 Swagger 文档。
- **灵活存储** —— SQLite (默认) 或 PostgreSQL。
- **Fail2ban 集成** —— 用于实施客户端 IP 限制。

### 截图

<details>
<summary>点击展开</summary>

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

## 快速开始

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

从 [最新发布版本](https://github.com/AlexeyLCP/lucx-ui/releases/latest) 安装面板、systemd 服务、Xray-core 和 mtg，并通过 DKMS 编译 AmneziaWG 内核模块 (`bin/install-awg-module.sh`)。

安装过程中会随机生成用户名、密码和访问路径。安装完成后，运行 `x-ui` 打开管理菜单。

### 无人值守安装

安装程序支持用于 cloud-init 的**非交互式模式**。设置 `XUI_NONINTERACTIVE=1` 即可全自动安装，凭据保存在 `/etc/x-ui/install-result.env`。参见 [`deploy/`](deploy/) 了解 cloud-init 指南。

## 支持的平台

**操作系统:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine 和 Windows。

**架构:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`。

## 数据库选项

3X-UI 支持两种后端：

- **SQLite** (默认) —— 单个文件 `/etc/x-ui/x-ui.db`。
- **PostgreSQL** —— 推荐用于海量客户端或多节点部署。

在 `/etc/default/x-ui` 中的环境变量：
```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Docker

在 Docker 中使用 PostgreSQL 时，请取消 `docker-compose.yml` 中的 `XUI_DB_*` 环境变量注释，并运行：
```bash
docker compose --profile postgres up -d
```

## 环境变量

| 变量 | 描述 | 默认值 |
| --- | --- | --- |
| `XUI_DB_TYPE` | 数据库后端：`sqlite` 或 `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL 连接字符串 (当 `XUI_DB_TYPE=postgres` 时) | — |
| `XUI_DB_FOLDER` | SQLite 数据库文件目录 | `/etc/x-ui` |
| `XUI_ENABLE_FAIL2BAN` | 启用 Fail2ban IP 限制 | `true` |
| `XUI_LOG_LEVEL` | 日志级别 (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_TUNNEL_HEALTH_MONITOR` | 启用隧道健康监控 | `false` |

## 许可证与条款

本项目遵循**双重许可证**（详情参阅 [LICENSING.md](LICENSING.md)）：

| 组件 | 许可证 |
|---|---|
| 原始 3x-ui 代码 | **GPL-3.0**（按上游要求） |
| LucX 独占组件 (`internal/awg/`, `internal/lucx/`, AWG 前端及脚本) | **PolyForm Noncommercial 1.0.0** |

**免费**用于个人、非商业、科学、研究和教育用途。**商业用途**（VPN 专售、付费面板、商业嵌入）需要作者的明确书面许可：请提交 [issue](https://github.com/AlexeyLCP/lucx-ui/issues) 或联系仓库所有者。每个文件头的 `SPDX-License-Identifier` 明确定义了界限：无头文件即为 GPL-3.0。

## 贡献指南

欢迎参与贡献。在提交 issue 或 pull request 之前，请阅读 [贡献指南](/CONTRIBUTING.md)。

## 致谢与来源

### 测试者与贡献者
- **VladufQa** —— 真实 VPS 测试 (ruvds)：首次握手、流量测试、级联路由及 Bug 反馈。
- **Kirill Rudenko** —— 测试 (runode) 与 **PR #13**：AWG needRestart、iif 策略路由、独立网关/路由表、路由恢复和 sniffing。
- **302ba (Alex)** —— **PR #24**：修复解析 Zod 模式时客户端字段丢失的问题。
- **alireza0** —— 上游贡献者。
- **3x-ui 团队** —— 感谢提供优秀的基石和 Sidecar 架构。

### 来源与灵感
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) —— Fork 基础 (GPL-3.0)，MTProto Sidecar 架构参考。
- [AmneziaVPN](https://github.com/amnezia-vpn) —— AmneziaWG 协议及内核模块。
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) —— PostUp NAT 模式 (MASQUERADE + FORWARD)、无密码学库的 QUIC Initial 生成器及 DKMS 安装方法。
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) —— QUIC 签名抓取 (`internal/awg/signature/`)。
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) 与 [refraction-networking/utls](https://github.com/refraction-networking/utls) —— 用于 ClientHello 预设的浏览器 TLS 指纹。
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) 与 [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) —— 路由规则数据集。

### 社区工具
- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (许可证: **MIT**): 通过代码管理入站、客户端和面板设置。

## ☕ 支持本项目

LucX-UI 对个人和非商业用途完全免费。如果面板帮您节省了时间，欢迎支持开发：

| 方式 | 详情 |
|---|---|
| 🇷🇺 **YooMoney** (卢布, 俄罗斯) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

捐赠是对项目的感谢，不属于购买行为：捐赠不代表授予商业许可证，也不改变 [LICENSING.md](LICENSING.md) 中的条款。

## 随时间变化的星标数

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)

<!-- END LUCX-HOOK -->
