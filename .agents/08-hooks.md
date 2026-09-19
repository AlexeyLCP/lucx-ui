# 08 — LucX overlay catalog (next upstream merge)

Three piles. Merge `origin/main`, then restore pile 1 and 2 onto origin files. Keep pile 3 as whole files. Never `--ours` a whole origin file (Rule 8).

| Pile | What | Detector |
|---|---|---|
| 1 | Origin file + `LUCX-HOOK` | `git grep -c LUCX-HOOK` count drop |
| 2 | Origin file we patched **without** a marker | count stays; `git diff origin/main -- path` goes to zero |
| 3 | No origin twin | file missing on `origin/main` |

```
git grep -c LUCX-HOOK > /tmp/hooks-before
git merge --no-commit --no-ff origin/main
git grep -c LUCX-HOOK > /tmp/hooks-after
# drop = lost pile 1. Then walk pile 2: git diff origin/main -- <file>
```

i18n: extra keys in all 13 `internal/web/translation/*.json` have **no** HOOK. Dead-keys CI requires the same key set as en-US.

`frontend/src/generated/*` — `npm run gen`, never hand-merge.

## Traps (v3.8.0)

| Where | Must keep |
|---|---|
| `internal/sub/service.go` `getInboundsBySubId` | lucx protocols in the SQL `IN` list |
| `internal/sub/service.go` `ResolveRequest` | never use `X-Real-IP` as share host |
| `internal/sub/service.go` `amneziaWGConfigText` | always emit `S3`/`S4` (zero is real); drop unportable I-fields |
| `internal/web/service/inbound.go` UpdateInbound | sidecar in-place `Update`, snapshot **old** protocol on del |
| `internal/web/service/client_inbound_apply.go` | share-only: no `clients[]`, no `MarshalIndent` of settings |
| `internal/web/service/client_crud.go` Update | do not broadcast keys/AllowedIPs across tunnel inbounds |
| `internal/web/service/inbound_node.go` | auto-tag port change → same row + new tag |
| `frontend/.../InboundFormModal.tsx` | **one** Form+Tabs; `AwgInboundIdProvider`; lucx protocols in the list |
| `frontend/.../qr/QrPanel.tsx` | Happ QR cutoff **2953**, not 2000 (no HOOK) |
| `frontend/.../clients/ClientInfoModal.tsx` | kernel AWG ConfigBlock (download .conf), not only ClientQrModal |
| `install.sh` / `update.sh` after fail2ban | `bash bin/install-awg-module.sh` (lost in v3.8 overlay, lucx.248) |

---

## 1 — Overlay with HOOK

### Install / Docker / CI

| File | Why |
|---|---|
| `install.sh` | fork URLs, Yandex geo, geo before first start, AWG after fail2ban |
| `x-ui.sh` | install-source, RoscomVPN geo, AWG module menu |
| `DockerInit.sh` | RoscomVPN geo |
| `Dockerfile` | LucX image |
| `docker-compose.yml` | GHCR |
| `.github/FUNDING.yml` | donations to LucX author |
| `.github/workflows/docker.yml` | GHCR, no Docker Hub, amd64+arm64 |
| `.github/workflows/release.yml` | lucx tag suffix, slim tarball, sidecars, arm64, stable |
| `.github/workflows/mutation.yml` | mutate LucX packages only |

### Go

| File | Why |
|---|---|
| `internal/config/config.go` + `_test.go` | `lucxVersion` suffix |
| `internal/database/db.go` | KeepAlive widen, `awg_outbounds`, prune hidden SOCKS |
| `internal/database/model/model.go` | lucx protocols, KeepAliveValue |
| `internal/mtproto/manager.go` | lucx touch |
| `internal/sub/service.go` | GetLink/GetSubs lucx protocols, AWG conf, host, I-fields |
| `internal/sub/controller.go` | `/awg/` routes, Happ |
| `internal/sub/clash_service.go` | kernel AWG clash proxy |
| `internal/sub/sub.go` | AWG sub URL |
| `internal/web/web.go` | AWG/tunnel cadence, stop, upload limit |
| `internal/web/entity/entity.go` | favicon, log days, Happ ROSCOM, AWG sub URI |
| `internal/web/runtime/local.go` | AWG/sidecar Add/Del/Update |
| `internal/web/job/xray_traffic_job.go` | AWG speed into broadcast |
| `internal/web/controller/api.go` | lucx API routes |
| `internal/web/controller/inbound.go` | AWG/sidecar save, I-field warn |
| `internal/web/controller/client.go` | awgBody / subBody |
| `internal/web/controller/server.go` | version / geo / cores |
| `internal/web/controller/node.go` | lucx node capability |
| `internal/web/controller/dist.go` | lucx static |
| `internal/web/controller/xray_setting.go` | lucx xray settings |
| `internal/web/service/inbound.go` | AWG/sidecar create/update, deploy gate |
| `internal/web/service/inbound_node.go` | share-only node sync, auto-tag |
| `internal/web/service/inbound_migration.go` | lucx protocol migrate |
| `internal/web/service/inbound_protocol.go` + `_test.go` | lucx protocol helpers |
| `internal/web/service/inbound_sublink.go` | lucx sub links |
| `internal/web/service/client_crud.go` | one keypair, no IP/key broadcast |
| `internal/web/service/client_inbound_apply.go` | skip UUID / share-only settings |
| `internal/web/service/client_bulk.go` | tunnel field strip |
| `internal/web/service/client_link.go` | lucx link export |
| `internal/web/service/client_lookup.go` | lucx lookup |
| `internal/web/service/xray.go` + `xray_config_inject_test.go` | inject AWG TUN/outbounds |
| `internal/web/service/setting.go` | lucx settings keys |
| `internal/web/service/server.go` | lucx server |
| `internal/web/service/node.go` + `node_contract.go` | lucx node |
| `internal/web/service/port_conflict.go` | lucx transports |
| `internal/web/service/panel/panel.go` + `panel_test.go` | release notes / version |
| `internal/web/service/tgbot/tgbot_client.go` + `tgbot_send.go` | lucx bot |

### Frontend (HOOK)

| File | Why |
|---|---|
| `frontend/src/schemas/primitives/protocol.ts` | lucx protocol enum |
| `frontend/src/schemas/protocols/inbound/index.ts` | union lucx inbound schemas |
| `frontend/src/schemas/protocols/inbound/wireguard.ts` | lucx keepalive |
| `frontend/src/schemas/client.ts` / `setting.ts` / `xray.ts` / `node.ts` | lucx fields |
| `frontend/src/lib/xray/inbound-defaults.ts` | default AWG/sidecar settings |
| `frontend/src/lib/xray/inbound-link.ts` | genAwgLink / sidecar URIs |
| `frontend/src/lib/xray/link-label.tsx` | lucx labels |
| `frontend/src/lib/xray/protocol-capabilities.ts` | lucx caps |
| `frontend/src/pages/inbounds/form/InboundFormModal.tsx` | one form + lucx tabs |
| `frontend/src/pages/inbounds/form/protocols/index.ts` | export lucx fields |
| `frontend/src/pages/inbounds/list/InboundList.tsx` `RowActions.tsx` `helpers.ts` | lucx list |
| `frontend/src/pages/inbounds/info/InboundInfoModal.tsx` `helpers.ts` | lucx info |
| `frontend/src/pages/inbounds/InboundsPage.tsx` `useInbounds.ts` | lucx page |
| `frontend/src/pages/clients/ClientFormModal.tsx` `ClientQrModal.tsx` `ClientInfoModal.tsx` `ClientsPage.tsx` `wireguardConfig.ts` | AWG QR/.conf |
| `frontend/src/pages/sub/SubPage.tsx` | AMNEZIA / vpn:// |
| `frontend/src/pages/xray/XrayPage.tsx` `balancers/BalancersTab.tsx` `basics/constants.ts` `routing/*` | kernel AWG outbounds + routing |
| `frontend/src/pages/settings/GeneralTab.tsx` | lucx settings |
| `frontend/src/pages/index/GeodataSection.tsx` `VersionModal.tsx` | ROSCOM, lucx version |
| `frontend/src/pages/login/LoginPage.tsx` `LoginPage.css` | lucx login |
| `frontend/src/layouts/AppSidebar.tsx` | Tunnels menu |
| `frontend/src/routes.tsx` | `/panel/tunnels` |
| `frontend/src/api/queryKeys.ts` `queries/useOutboundTags.ts` | lucx keys |
| `frontend/src/hooks/useTheme.tsx` `useXraySetting.ts` | Sand/Graphite |
| `frontend/src/models/status.ts` | lucx status |
| `frontend/src/styles/page-shell.css` | lucx chrome |
| `frontend/src/env.d.ts` | lucx env |
| `frontend/src/pages/api-docs/endpoints.ts` | lucx API docs |
| `frontend/src/test/inbound-link.test.ts` `wireguard-client-config.test.ts` `link-label.test.ts` `rule-form-preserve-fields.test.tsx` | lucx tests |

---

## 2 — Overlay **without** HOOK

Origin twins. Merge taking origin deletes these with **no count drop**. Re-apply from `git show our-pre-merge:path`. Wrap new edits in HOOK when touching them.

### Install / CI

| File | Why |
|---|---|
| `update.sh` | `AlexeyLCP/lucx-ui` URLs, sha256 sidecar, AWG after fail2ban |
| `.github/workflows/ci.yml` | lucx CI bits |
| `.github/workflows/smoke.yml` | lucx smoke |
| `.github/dependabot.yml` | lucx ignore/grouping |
| `.gitattributes` `.gitignore` | lucx paths |

### Go

| File | Why |
|---|---|
| `internal/sub/json_service.go` | lucx JSON sub bits |
| `internal/web/service/inbound_amneziawg.go` | lucx patches on userspace AmneziaWG |
| `internal/web/service/inbound_clients.go` | lucx client JSON |
| `internal/web/service/inbound_mtproto.go` | lucx mtproto |
| `internal/web/service/inbound_flow_restore.go` | persistIntendedFlow |
| `internal/web/service/client_wireguard.go` | lucx WG/AWG overlap |
| `internal/amneziawg/params.go` | lucx param bits |
| `internal/amneziawgnet/device.go` | lucx device bits |
| `tools/openapigen/main.go` | lucx OpenAPI fields |

Tests on origin twins (no HOOK): `internal/sub/forwarded_trust_test.go`, `service_amneziawg_test.go`, `json_service_test.go`, `clash_service_test.go`, `service_test.go`; `internal/web/service/node_tag_sync_test.go`, `inbound_amneziawg_test.go`, `inbound_migration_test.go`, `client_attach_test.go`, `client_wireguard_test.go`, `port_conflict_test.go`, and siblings in the same packages. If origin has the test file, keep our extra cases.

### Frontend

| File | Why |
|---|---|
| `frontend/src/pages/inbounds/qr/QrPanel.tsx` | Happ QR 2953 + vpn:// resolve |
| `frontend/src/pages/inbounds/qr/QrCodeModal.tsx` | lucx QR variants |
| `frontend/src/pages/settings/HappSettingsContent.tsx` | ROSCOM Happ source |
| `frontend/src/pages/settings/SettingsPage.tsx` | Cores tab |
| `frontend/src/pages/settings/SubscriptionFormatsTab.tsx` `SubscriptionGeneralTab.tsx` | AWG sub URI |
| `frontend/src/hooks/useClients.ts` | lucx client fields |
| `frontend/src/lib/xray/inbound-form-adapter.ts` | lucx protocol form |
| `frontend/src/models/dbinbound.ts` `setting.ts` | lucx model fields |
| `frontend/src/pages/clients/amneziawgConfig.ts` | lucx userspace conf |
| `frontend/src/pages/clients/ClientBulkAddModal.tsx` `FilterDrawer.tsx` `SubLinksModal.tsx` | lucx client UI |
| `frontend/src/pages/inbounds/list/InboundStatsModal.tsx` `types.ts` `useInboundColumns.tsx` | lucx columns |
| `frontend/src/pages/index/IndexPage.tsx` `OverviewActionBar.tsx` `PanelUpdateModal.tsx` `PanelUpdateModal.css` | lucx update UI |
| `frontend/src/pages/nodes/NodesPage.tsx` | lucx node capability |
| `frontend/src/layouts/PanelLayout.tsx` `AppSidebar.css` | lucx chrome |
| `frontend/src/main.tsx` | lucx boot |
| `frontend/src/components/command-palette/CommandPalette.tsx` | lucx palette entries |
| `frontend/src/schemas/status.ts` | lucx status |
| `frontend/package.json` `vitest.config.ts` `.oxlintrc.json` `.storybook/preview.tsx` | lucx scripts/tests |

### i18n

All 13 `internal/web/translation/*.json` — LucX keys in every locale. No HOOK. Copy new keys from `en-US.json` into the other 12.

---

## 3 — Ours, no origin twin

Keep the directory. Origin will not have these.

| Dir / file | What |
|---|---|
| `internal/awg/` | kernel AWG sidecar |
| `internal/lucx/` | tunnel sidecars, geodata, parser |
| `internal/database/migrate_awg*.go` `migrate_geodata.go` `migrate_naive_inbound.go` `migrate_tunnel_*.go` `repair_tunnel_fields.go` | LucX migrations |
| `internal/database/model/awg_outbound.go` `sidecar_outbound.go` | LucX tables |
| `internal/sub/awg_*.go` `roscomvpn.go` `service_{awg,naive,qwdtt,trusttunnel}_test.go` `clash_awg_test.go` | LucX sub |
| `internal/web/controller/awg.go` `awg_outbound.go` `lucx.go` `tunnel.go` `sidecar_outbound.go` | LucX API |
| `internal/web/job/awg_job.go` `awg_speed_buffer.go` `tunnel_job.go` `log_retention_job.go` | LucX jobs |
| `internal/web/favicon/` | panel tab icon |
| `internal/web/service/inbound_lucx.go` `client_awg.go` `awg_*.go` `tunnel.go` `sidecar_outbound.go` `lucx_online.go` `network_tuning.go` `client_tunnel_creds.go` | LucX service |
| `frontend/src/pages/tunnels/` | Tunnels page |
| `frontend/src/pages/inbounds/form/protocols/{awg,naive,olcrtc,qwdtt,csqtt,mieru,trusttunnel,anytls,tproxy,cover}.tsx` | lucx forms |
| `frontend/src/pages/xray/awg-outbounds/` `sidecar-outbounds/` | lucx outbound UI |
| `frontend/src/pages/settings/CoresTab.tsx` | sidecar binaries |
| `frontend/src/pages/inbounds/AwgImportBanner.tsx` | AWG import |
| `frontend/src/lib/awg/` `lib/mieru/` `lib/sub/` `lib/xray/awg-*.ts` | lucx libs |
| `frontend/src/api/{awg-import,awg-outbounds,sidecar-outbounds,tunnels}.ts` | lucx API |
| `frontend/src/schemas/{awg-*,tunnel,sidecar-outbound}.ts` `protocols/inbound/{awg,naive,…}.ts` | lucx zod |
| `frontend/src/test/awg-*.ts*` `qwdtt-*.ts` `csqtt-*.ts` `vpnuri.test.ts` `sub-*.ts` `cores-*.tsx` | lucx tests |
| `bin/` | install-awg, check-lucx, pack-sidecars, sourcecraft |
| `third_party/sidecars/` `third_party/patches/` | sidecar blobs |
| `.github/workflows/upstream-watch.yml` | watch origin releases |
| `.sourcecraft/` | Yandex CI |
| `.agents/` `docs/progress.md` `docs/LICENSING.md` | fork docs |
| `LICENSE-PolyForm-Noncommercial.txt` | LucX license |

PolyForm vs GPL split: Rule 10 / `docs/LICENSING.md`.

---

## Refresh

- New HOOK on an origin file → pile 1, same commit.
- Patch on an origin file without HOOK → pile 2 **and** wrap it in HOOK if you can.
- New LucX-only file → pile 3 (directory is enough).
