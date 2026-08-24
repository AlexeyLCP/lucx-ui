# Licensing — LucX-UI

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) and uses **two licenses**, depending on which part of the code you are looking at.

## 1. Upstream 3x-ui code — GPL-3.0

All original 3x-ui code remains under the **GNU General Public License v3.0** (see [LICENSE](LICENSE)), as required by the upstream project. This includes every file not listed below, and every upstream file that carries inline `LUCX-HOOK` integration blocks (the surrounding file stays GPL; the hook blocks themselves are small integration glue).

## 2. LucX-specific components — PolyForm Noncommercial 1.0.0

The components **authored by the LucX-UI project** are licensed under the **PolyForm Noncommercial License 1.0.0** (see [LICENSE-PolyForm-Noncommercial.txt](LICENSE-PolyForm-Noncommercial.txt)):

- `internal/awg/` — the entire AWG sidecar (manager, process, instance, traffic, diagnostics, NAT/orphan helpers, `cps/`, `signature/`)
- `internal/lucx/` — Smart Cluster packages (`parser/`, `nodetype/`, `outbound_link/`) and the tunnel sidecar package (`tunnel/`)
- `internal/database/migrate_awg.go` and its test
- `internal/web/controller/awg.go` — AWG API endpoints
- `internal/web/controller/tunnel.go` — tunnel sidecar API endpoints
- `internal/web/job/awg_job.go` — AWG reconcile cron
- `internal/web/job/tunnel_job.go` — tunnel sidecar reconcile cron
- `internal/web/service/client_awg.go` — AWG client provisioning
- `internal/web/service/awg_import.go` — import existing host AWG
- `frontend/src/api/awg-import.ts`, `frontend/src/schemas/awg-import.ts`, `frontend/src/pages/inbounds/AwgImportBanner.tsx`
- `internal/web/service/tunnel.go` — tunnel sidecar service
- `frontend/src/schemas/protocols/inbound/awg.ts` — AWG Zod schema
- `frontend/src/schemas/tunnel.ts` — tunnel sidecar Zod schemas
- `frontend/src/api/tunnels.ts` — tunnel sidecar API client
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` — AWG form
- `frontend/src/pages/inbounds/form/awg-inbound-id-context.ts`
- `frontend/src/pages/tunnels/TunnelsPage.tsx` — tunnel sidecar page
- `frontend/src/pages/clients/wireguardConfig.ts` — client `.conf` builder
- `bin/install-awg-module.sh` — DKMS kernel-module installer
- `bin/check-lucx.sh`, `bin/pre-push` — development scripts

Every such file carries an SPDX header:

```
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
```

**What this means in practice:**

- **Free** for personal, educational, research, charity, and government use — run as many panels as you like.
- **Commercial use requires permission.** Reselling VPN access (paid panels/subscriptions built on this code), offering it as a paid service, or embedding these components in a commercial product requires explicit written permission from the LucX-UI author.
- You **cannot** sublicense these components or strip the license headers.
- The GPL-3.0 obligations for the upstream 3x-ui code apply to the project as a whole regardless.

## 3. Third-party binaries & data (not LucX-UI code)

The panel **supervises external processes** and ships optional geo datasets. Nothing below is linked into the LucX-UI Go binary; licenses stay with their upstream authors.

| Artifact / project | Role | License |
|---|---|---|
| [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) | Base panel | **GPL-3.0** |
| LucX-owned paths listed in §2 | AWG / tunnels / Smart Cluster | **PolyForm Noncommercial 1.0.0** |
| `bin/caddy-naive-linux-*` — [Caddy](https://github.com/caddyserver/caddy) | NaiveProxy sidecar runtime | **Apache-2.0** |
| [klzgrad/forwardproxy](https://github.com/klzgrad/forwardproxy) (`naive` branch) | Caddy `forward_proxy` plugin | **MIT** |
| [klzgrad/naiveproxy](https://github.com/klzgrad/naiveproxy) | Protocol / client reference | **BSD-3-Clause** |
| `bin/olcrtc-linux-*` — [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc) | olcRTC WebRTC tunnel core | **WTFPL** |
| `bin/qwdtt-linux-*` — [SpaceNeuroX/proxy-turn-vk-android](https://github.com/SpaceNeuroX/proxy-turn-vk-android) server | qWDTT (WG over VK TURN) | **GPL-3.0** |
| `bin/mieru-linux-*` — [enfein/mieru](https://github.com/enfein/mieru) `mita` | mieru server | **GPL-3.0** |
| `bin/trusttunnel-linux-*` — [TrustTunnel/TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) `trusttunnel_endpoint` | TrustTunnel endpoint | **Apache-2.0** |
| [amnezia-vpn](https://github.com/amnezia-vpn) kernel module & tools | AmneziaWG / AWG3 (installed by host scripts, not the panel binary) | **GPL-2.0** (kernel module) |
| `bin/geoip.dat` / `geosite.dat` — [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) | Stock geodata | Upstream (see repo) |
| `bin/geoip_IR.dat` / `geosite_IR.dat` — [chocolate4u/Iran-v2ray-rules](https://github.com/chocolate4u/Iran-v2ray-rules) | IR geodata | Upstream (see repo) |
| `bin/geoip_RU.dat` / `geosite_RU.dat` — [runetfreedom/russia-v2ray-rules-dat](https://github.com/runetfreedom/russia-v2ray-rules-dat) | RU geodata | Upstream (see repo) |
| `bin/geoip_ROSCOM.dat` / `geosite_ROSCOM.dat` — [hydraponique/roscomvpn-geoip](https://github.com/hydraponique/roscomvpn-geoip), [roscomvpn-geosite](https://github.com/hydraponique/roscomvpn-geosite) | RoscomVPN RKN lists | Upstream (see repo) |
| Happ routing deeplinks — [hydraponique/roscomvpn-routing](https://github.com/hydraponique/roscomvpn-routing) | Optional client routing profiles | Upstream (see repo) |

**Notes**

- Tunnel binaries are **child processes**. LucX code under `internal/lucx/tunnel/` (PolyForm) only writes configs, spawns/kills, and probes health.
- qWDTT is GPL-3.0 **as an external program**. Shipping the binary does not relicense LucX PolyForm sources; operators who redistribute qWDTT must still honour GPL-3.0 for that binary and its sources.
- Geo `.dat` files are data packs refreshed from upstream releases; their license/terms follow the linked repositories.
- Design references (not shipped as code): [elector1337/3x-ui-naive](https://github.com/elector1337/3x-ui-naive), [Bebrik2283555/Ex3-ui](https://github.com/Bebrik2283555/Ex3-ui), geodata browser port of [MHSanaei/3x-ui#6165](https://github.com/MHSanaei/3x-ui/pull/6165) (STRENCH0).

## Why the split?

3x-ui is GPL-3.0, and a fork cannot be relicensed as a whole. At the same time, the AWG sidecar and Smart Cluster are original work the author wants to keep non-commercial — usable and forkable by anyone for themselves, but not repackaged into a paid VPN business without a conversation first. Per-file SPDX headers make the boundary unambiguous; if a file has no SPDX header, it is upstream GPL-3.0.

## Contact

For commercial licensing, open an issue at <https://github.com/AlexeyLCP/lucx-ui/issues> or contact the repository owner.
