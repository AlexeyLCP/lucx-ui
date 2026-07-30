<!-- LUCX-HOOK: LucX-UI fork README — Streamlined ZH README. Keep in sync with LICENSING.md and AGENTS.md. -->
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
> **仅限个人、非商业、科学和教育用途。** 商业用途（包括 VPN 专售或付费面板）需要根据 PolyForm Noncommercial 1.0.0 获得作者的书面许可。

---

## ⚡ 快速开始

在 **Linux (Ubuntu / Debian / CentOS / AlmaLinux / Arch 等)** 上的一键安装脚本：

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

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

## 🌟 关于 LucX-UI

**LucX-UI** 是一款用于管理 [Xray-core](https://github.com/XTLS/Xray-core) 服务器的高级 Web 控制面板，基于 [3x-ui](https://github.com/MHSanaei/3x-ui) 进行增强分叉，原生支持 **AmneziaWG (AWG)** 内核接口 Sidecar。

### 🛡️ AmneziaWG (AWG) 特性
- **AWG 入站与出站** —— 内核 Sidecar (`awg-quick`)、客户端模式连接上游 AWG 服务器 (`awgo-{id}`)、10 秒自动协调循环及 DKMS 内核模块构建器。
- **高级混淆控制** —— Lite/Standard/Pro 预设 (Jc/Jmin/Jmax/S1–S4/H1–H4)、CPS 数据包伪装 (TLS、DNS、SIP、QUIC) 及浏览器 TLS 指纹 (Chrome、Firefox、Safari)。
- **真实签名抓取 (Live Capture)** —— 将真实域名的 QUIC 握手实时转换为 I1–I5 混淆参数。
- **路由与诊断** —— 双路由模式 (Kernel NAT 与带策略路由及 sniffing 的 Route through Xray) + 面板内一键诊断。

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

## 📜 许可证与条款

本项目遵循**双重许可证**（详情参阅 [LICENSING.md](LICENSING.md)）：

| 组件 | 许可证 |
|---|---|
| 原始 3x-ui 代码库 | **GPL-3.0** |
| LucX-UI 组件 (`internal/awg/`, `internal/lucx/`, 前端) | **PolyForm Noncommercial 1.0.0** |

---

## 🤝 致谢与来源

- **测试者与贡献者：** **VladufQa**, **Kirill Rudenko** (PR #13), **302ba (Alex)** (PR #24), **alireza0**, **3x-ui 团队**。
- **项目与灵感：** [3x-ui](https://github.com/MHSanaei/3x-ui), [AmneziaVPN](https://github.com/amnezia-vpn), [pumbaX/awg-multi-script](https://github.com/pumbaX/awg-multi-script), [hoaxisr/awg-manager](https://github.com/hoaxisr/awg-manager), [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client), [refraction-networking/utls](https://github.com/refraction-networking/utls)。

---

## ☕ 支持本项目

LucX-UI 个人使用完全免费。您可以支持后续开发：

| 方式 | 详情 |
|---|---|
| 🇷🇺 **YooMoney** (卢布, 俄罗斯) | [yoomoney.ru/to/41001989176429](https://yoomoney.ru/to/41001989176429) |
| 💎 **USDT (TON)** | `UQC48dE4i35bjEU4jljx0h1CGeXMu77eKZwN5W4gbcibmqDs` |
| 💠 **USDT (ERC-20)** | `0xA49aBc042c5BB3d682788D3DEB2eAC833343a873` |

---

## ⭐ 随时间变化的星标数

[![Stargazers over time](https://starchart.cc/AlexeyLCP/lucx-ui.svg?variant=adaptive)](https://starchart.cc/AlexeyLCP/lucx-ui)

<!-- END LUCX-HOOK -->
