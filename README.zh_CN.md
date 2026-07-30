<!-- LUCX-HOOK: LucX-UI fork README — ZH lead section, license, credits, sources. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **仅限个人、非商业、科学、研究和教育用途。** 商业用途（包括 VPN 转售、付费面板或基于此代码构建的订阅服务）需要获得原作者的明确书面许可。请勿用于非法目的。

---

## 关于 LucX-UI

**LucX-UI** 是 [3x-ui](https://github.com/MHSanaei/3x-ui) (v3.6.0) 的分支，具有原生 **AmneziaWG (AWG)** 支持。AWG 作为内核接口 sidecar 运行——完全镜像上游 MTProto (mtg) 的架构：面板负责生命周期和流量统计，Xray 可选择性进行路由。

### 新增与支持的功能

- ✅ **AWG 入站 (Inbounds)** — 基于 `awg-quick` 的内核 sidecar：创建、每 10 秒 reconcile 调和、孤立接口清理、内核模块 DKMS 安装程序。
- ✅ **AWG 出站 (Outbounds / 客户端模式)** — 面板可直接连接至上游 AmneziaWG 服务器：在 Xray 菜单中有专属标签页，粘贴已有的 `.conf`，并由 reconcile 循环管理 `awgo-{id}` 内核接口。Xray 配置中会注入带有 `sockopt.interface` 的 `freedom` 出站，以便路由规则和负载均衡器将流量送往上游 VPN。
- ✅ **混淆控制** — Lite/Standard/Pro 预设 (Jc/Jmin/Jmax/S1–S4/H1–H4) 以及 CPS 数据包伪装：TLS、DNS、SIP、QUIC。
- ✅ **浏览器 TLS 指纹** — Chrome (GREASE)、Firefox 120+ (NSS 排序与 padding)、Safari 16+ (Apple 排序与 TLS 1.1)。适用于 TLS 和 QUIC。
- ✅ **实时主机签名抓取** — 将前置域名的真实 QUIC 握手自动转换为 I1–I5 混淆参数。
- ✅ **客户端管理** — 二维码、`.conf` 配置文件下载、按节点/客户端统计流量 (`awg show transfer`)。
- ✅ **两种路由模式:**
  - **Kernel NAT** — 内核直接转发；NAT 规则在 iptables 被刷新后由 reconcile 自动恢复。
  - **Route through Xray (通过 Xray 路由)** — 流量通过 TUN 入站、策略路由和 sniffing 识别，完整流经 Xray 的路由管线（域名/geosite 规则、负载均衡器、链式出站）。
- ✅ **面板内置诊断** — 入站表单中的一键诊断按钮：实时检测接口状态、ip_forward、节点握手、NAT/TUN 规则，快速排查故障。
- ✅ **实战验证** — 在测试 VPS 上经过充分验证：握手、ICMP、HTTPS、流量统计、级联路由及两种路由模式均正常工作。

### 安装

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

从 [最新 Release](https://github.com/AlexeyLCP/lucx-ui/releases/latest) 安装面板、systemd 服务、Xray-core 和 mtg（来自 3x-ui 上游），并通过 DKMS 自动编译安装 AmneziaWG 内核模块 (`bin/install-awg-module.sh`)。

### 许可证

本项目采用 **双许可** 机制（详情见 [LICENSING.md](LICENSING.md)）：

| 项目组件 | 许可协议 |
|---|---|
| 原始 3x-ui 代码 | **GPL-3.0** （遵循上游要求） |
| LucX 独创组件 (`internal/awg/`, `internal/lucx/`, AWG 前端及脚本) | **PolyForm Noncommercial 1.0.0** |

在实际使用中：个人、非商业、科学、研究和教育用途 **完全免费**。**商业用途**（VPN 转售、付费面板服务、嵌入商业产品）必须获得作者的明确书面许可——请提交 [issue](https://github.com/AlexeyLCP/lucx-ui/issues) 或联系仓库所有者。文件中的 `SPDX-License-Identifier` 标头明确区分了许可边界：无标头即为 GPL-3.0。

### 致谢

- **VladufQa** — 实战 VPS (ruvds) 测试：首次握手、流量统计、级联转发及路由 bug 反馈。
- **Kirill Rudenko** — 测试 (runode) 与 **PR #13**：AWG needRestart、iif 策略路由、独立 inbound 路由表/网关、reconcile 路由恢复及 sniffing — 让“通过 Xray 路由”真正稳定运行。
- **302ba (Alex)** — **PR #24**：修复 Zod schema 解析时客户端字段丢失的问题。
- **3x-ui** 团队 — 感谢他们优秀的代码基础与我们所镜像的 sidecar 架构。

### 借鉴来源与致谢

- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — Fork 基础 (GPL-3.0)，MTProto sidecar 架构参考。
- [AmneziaVPN](https://github.com/amnezia-vpn) — AmneziaWG 协议本身及内核模块。
- [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script) — PostUp NAT 模式 (MASQUERADE + FORWARD)、无密码学依赖的 QUIC Initial 生成器及 DKMS 安装思路。
- [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager) — QUIC 签名抓取移植 (`internal/awg/signature/`) 及 TLS 兼容性提示。
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) 和 [refraction-networking/utls](https://github.com/refraction-networking/utls) — ClientHello 预设背后的 Firefox/Safari 代表性 TLS 指纹。

### ☕ 支持本项目

LucX-UI 对个人和非商业用途完全免费。如果面板帮您节省了时间，欢迎支持开发：

| 方式 | 详情 |
|---|---|
| 🇷🇺 **YooMoney** (卢布, 俄罗斯) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

捐赠是对项目的感谢，不属于购买行为：捐赠不代表授予商业许可证，也不改变 [LICENSING.md](LICENSING.md) 中的条款。

---

*以下为原 **3x-ui** 中文文档，完整保留以供参考。*

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

**3X-UI** 是一个先进的开源 Web 控制面板，用于管理 [Xray-core](https://github.com/XTLS/Xray-core) 服务器。它提供简洁、多语言的界面，用于部署、配置和监控各种代理与 VPN 协议——从单台 VPS 到多节点部署。

3X-UI 作为原始 X-UI 项目的增强分支（fork），增加了更广泛的协议支持、更好的稳定性、按客户端的流量统计以及许多提升使用体验的功能。

> [!IMPORTANT]
> 本项目仅供个人使用。请勿将其用于非法目的，也请勿在生产环境中使用。

## 功能特性

- **多协议入站** — VLESS、VMess、Trojan、Shadowsocks、WireGuard、Hysteria2、HTTP、SOCKS (Mixed)、Dokodemo-door / Tunnel 和 TUN。
- **现代传输与安全** — TCP (Raw)、mKCP、WebSocket、gRPC、HTTPUpgrade 和 XHTTP，并通过 TLS、XTLS 和 REALITY 加密。
- **回落 (Fallback)** — 通过 Xray 的 fallback 功能在单个端口上提供多种协议（例如在 443 端口上同时使用 VLESS 和 Trojan）。
- **按客户端管理** — 流量配额、到期日期、IP 限制、实时在线状态，以及一键分享链接、二维码和订阅。
- **流量统计** — 按入站、按客户端、按出站统计，并支持重置控制。
- **多节点支持** — 从单一面板管理并扩展到多台服务器。
- **出站与路由** — WARP、NordVPN、自定义路由规则、负载均衡器和出站代理链。
- **内置订阅服务器**，支持多种输出格式和[自定义页面模板](docs/custom-subscription-templates.md)。
- **Telegram 机器人**，用于远程监控和管理。
- **RESTful API**，带有面板内置的 Swagger 文档。
- **灵活的存储** — SQLite（默认）或 PostgreSQL。
- **13 种界面语言**，支持深色和浅色主题。
- **Fail2ban 集成**，用于强制执行按客户端的 IP 限制。

## 截图

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
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

若要安装特定版本，请在命令后附加对应的标签（例如 `v3.4.0`）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

若要安装滚动更新的 **dev** 版本（来自 `main` 的最新逐次提交预发布版本，而非稳定版本），请传入 `dev-latest`：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

安装过程中会生成随机的用户名、密码和访问路径。安装完成后，运行 `x-ui` 打开管理菜单，您可以在其中启动/停止服务、查看或重置登录凭据、管理 SSL 证书等。

完整文档请参阅 [项目Wiki](https://github.com/MHSanaei/3x-ui/wiki)。

### 无人值守安装

安装程序也可以**非交互式**运行，适用于 cloud-init。
设置 `XUI_NONINTERACTIVE=1`（或在无 TTY 的情况下通过管道传入），它就会全程
零提示地完成端到端安装，生成随机凭据并写入
`/etc/x-ui/install-result.env`。请参阅 [`deploy/`](deploy/)：

- [Cloud-init user-data](deploy/cloud-init/) — 在任意云平台上无人值守安装（Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle）
- [Hetzner Cloud 说明](deploy/marketplace/hetzner/) — 在 Hetzner 上基于 cloud-init 的部署

## 支持的平台

**操作系统：** Ubuntu、Debian、Armbian、Fedora、CentOS、RHEL、AlmaLinux、Rocky Linux、Oracle Linux、Amazon Linux、Virtuozzo、Arch、Manjaro、Parch、openSUSE (Tumbleweed / Leap)、Alpine 和 Windows。

**架构：** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`。

## 数据库选项

3X-UI 支持两种后端，可在安装时选择：

- **SQLite**（默认）— 位于 `/etc/x-ui/x-ui.db` 的单个文件。无需配置，适合中小型部署。
- **PostgreSQL** — 推荐用于大量客户端或多节点设置。安装程序可以为您在本地安装 PostgreSQL，或接受指向现有服务器的 DSN。

运行时通过环境变量选择后端（安装程序会为您写入 `/etc/default/x-ui`）：

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### 将现有的 SQLite 安装迁移到 PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# 然后在 /etc/default/x-ui 中设置 XUI_DB_TYPE 和 XUI_DB_DSN 并重启：
systemctl restart x-ui
```

源 SQLite 文件保持不变；在确认新后端正常工作后，请手动删除它。

### Docker

默认的 `docker compose up -d` 仍使用 SQLite。若要使用捆绑的 PostgreSQL 服务运行，请取消注释 `docker-compose.yml` 中的两行 `XUI_DB_*` 环境变量，并使用该 profile 启动：

```bash
docker compose --profile postgres up -d
```

该镜像捆绑了 Fail2ban（默认启用），用于强制执行按客户端的 **IP 限制**。Fail2ban 使用 `iptables` 封禁违规者，这需要 `NET_ADMIN` 权限。`docker-compose.yml` 已通过 `cap_add` 授予该权限；如果您改用 `docker run` 启动容器，请自行添加这些权限，否则封禁只会被记录而永远不会生效：

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `XUI_DB_TYPE` | 数据库后端：`sqlite` 或 `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL 连接字符串（当 `XUI_DB_TYPE=postgres` 时） | — |
| `XUI_DB_FOLDER` | SQLite 数据库文件所在目录 | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | 最大打开连接数（PostgreSQL 连接池） | — |
| `XUI_DB_MAX_IDLE_CONNS` | 最大空闲连接数（PostgreSQL 连接池） | — |
| `XUI_INIT_WEB_BASE_PATH` | Web 面板的初始 URI 路径 | `/` |
| `XUI_ENABLE_FAIL2BAN` | 启用基于 Fail2ban 的 IP 限制 | `true` |
| `XUI_LOG_LEVEL` | 日志级别（`debug`、`info`、`warning`、`error`） | `info` |
| `XUI_DEBUG` | 启用调试模式 | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | 启用隧道健康监控（探测某个 URL，在连续多次失败后重启 xray；重启会断开所有客户端） | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | 探测请求所经过的代理；将其指向本地 xray 入站，使探测能够测试隧道（例如 `socks5://127.0.0.1:1080`）。留空表示探测仅检查主机连通性 | — |
| `XUI_TUNNEL_HEALTH_URL` | 用于检测隧道健康状况的探测 URL | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | 两次探测之间的间隔 | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | 单次探测的超时时间 | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | 触发重启前的连续失败次数 | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | 两次连续重启之间的最小间隔 | `5m` |

## 支持的语言

面板界面提供 13 种语言：

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## 贡献

欢迎贡献。在提交 issue 或 pull request 之前，请阅读[贡献指南](/CONTRIBUTING.md)。

## 特别感谢

- [alireza0](https://github.com/alireza0/)

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (许可证: **GPL-3.0**): _增强的 v2ray/xray 和 v2ray/xray-clients 路由规则，内置伊朗域名，专注于安全性和广告拦截。_
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (许可证: **GPL-3.0**): _此仓库包含基于俄罗斯被阻止域名和地址数据自动更新的 V2Ray 路由规则。_

## 社区工具

社区围绕 3x-ui 构建的工具和集成。

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (许可证: **MIT**): _使用 Terraform / OpenTofu 通过代码管理入站、客户端、面板设置和 Xray 配置。_

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
