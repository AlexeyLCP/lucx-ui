# 08 — LUCX-HOOK overlay catalog

Files that exist on `origin/main` and carry LucX patches. After `git merge origin/main`, restore `LUCX-HOOK`…`END LUCX-HOOK` here. Never `--ours` a whole file (Rule 8).

**Not this catalog:** `internal/awg/`, `internal/lucx/`, and LucX-only files at the bottom — they have no origin twin; keep them.

## How

```
git grep -c LUCX-HOOK > /tmp/hooks-before
git merge --no-commit --no-ff origin/main
git grep -c LUCX-HOOK > /tmp/hooks-after
# any count drop = lost hook. Re-apply from pre-merge:
#   git show HEAD:path  (our blocks) onto origin version of the same file
```

i18n: extra keys in all 13 `internal/web/translation/*.json` have **no** HOOK markers. Dead-keys CI still requires them.

## Traps (v3.8.0)

These look like one-line origin edits; dropping them reds CI:

| Where | Must keep |
|---|---|
| `internal/sub/service.go` `getInboundsBySubId` | lucx protocols in the SQL `IN` list |
| `internal/sub/service.go` `ResolveRequest` | never use `X-Real-IP` as share host |
| `internal/sub/service.go` `amneziaWGConfigText` | always emit `S3`/`S4` (zero is real); drop unportable I-fields |
| `internal/web/service/inbound.go` UpdateInbound | sidecar in-place `Update`, snapshot **old** protocol on del |
| `internal/web/service/client_inbound_apply.go` | share-only: no `clients[]`, no `MarshalIndent` of settings |
| `internal/web/service/client_crud.go` Update | do not broadcast keys/AllowedIPs across tunnel inbounds |
| `internal/web/service/inbound_node.go` | auto-tag port change → same row + new tag |
| `frontend/.../InboundFormModal.tsx` | **one** Form+Tabs; wrap `AwgInboundIdProvider`; lucx protocols in the list |
| `frontend/.../qr/QrPanel.tsx` | Happ QR cutoff 2953 (version-40 L), not 2000 |

## Overlay files

### Install / Docker / CI

| File | Why |
|---|---|
| `install.sh` | fork URLs, Yandex geo bundle, geo before first start |
| `x-ui.sh` | install-source, RoscomVPN geo, AWG module menu |
| `DockerInit.sh` | RoscomVPN geo |
| `Dockerfile` | LucX image bits |
| `docker-compose.yml` | GHCR image |
| `.github/FUNDING.yml` | donations to LucX author |
| `.github/workflows/docker.yml` | GHCR, no Docker Hub, amd64+arm64 |
| `.github/workflows/release.yml` | lucx tag suffix, slim tarball, sidecars, arm64, stable not prerelease |
| `.github/workflows/mutation.yml` | mutate LucX packages only |

### Go — config / DB / sub

| File | Why |
|---|---|
| `internal/config/config.go` | `lucxVersion` suffix on dashboard / dev builds |
| `internal/config/config_test.go` | same |
| `internal/database/db.go` | KeepAlive text widen, `awg_outbounds`, prune hidden SOCKS children |
| `internal/database/model/model.go` | lucx protocols, KeepAliveValue, AWG fields |
| `internal/mtproto/manager.go` | lucx touch if any |
| `internal/sub/service.go` | GetLink/GetSubs lucx protocols, AWG conf, host, I-fields |
| `internal/sub/controller.go` | `/awg/` routes, Happ |
| `internal/sub/clash_service.go` | kernel AWG clash proxy |
| `internal/sub/sub.go` | AWG sub URL / settings |

### Go — web

| File | Why |
|---|---|
| `internal/web/web.go` | AWG/tunnel cadence, stop, upload limit |
| `internal/web/entity/entity.go` | favicon, log retention, Happ ROSCOM, AWG sub URI |
| `internal/web/runtime/local.go` | AWG/sidecar Add/Del/Update |
| `internal/web/job/xray_traffic_job.go` | fold AWG speed into broadcast |
| `internal/web/controller/api.go` | lucx API routes |
| `internal/web/controller/inbound.go` | AWG/sidecar save, I-field warn |
| `internal/web/controller/client.go` | awgBody / subBody |
| `internal/web/controller/server.go` | version / geo / cores |
| `internal/web/controller/node.go` | lucx node capability |
| `internal/web/controller/dist.go` | lucx static |
| `internal/web/controller/xray_setting.go` | lucx xray settings |
| `internal/web/service/inbound.go` | AWG/sidecar create/update, deploy gate, hints |
| `internal/web/service/inbound_node.go` | share-only node sync, auto-tag port change |
| `internal/web/service/inbound_migration.go` | lucx protocol migrate |
| `internal/web/service/inbound_protocol.go` | lucx protocol helpers |
| `internal/web/service/inbound_sublink.go` | lucx sub links |
| `internal/web/service/client_crud.go` | tunnel keypair once, no IP/key broadcast |
| `internal/web/service/client_inbound_apply.go` | skip UUID / share-only settings |
| `internal/web/service/client_bulk.go` | tunnel field strip |
| `internal/web/service/client_link.go` | lucx link export |
| `internal/web/service/client_lookup.go` | lucx lookup |
| `internal/web/service/xray.go` | inject AWG TUN/outbounds/routing |
| `internal/web/service/setting.go` | lucx settings keys |
| `internal/web/service/server.go` | lucx server bits |
| `internal/web/service/node.go` | lucx node contract |
| `internal/web/service/node_contract.go` | lucx capability |
| `internal/web/service/port_conflict.go` | lucx protocol transports |
| `internal/web/service/panel/panel.go` | release notes / version |
| `internal/web/service/tgbot/tgbot_client.go` | lucx bot bits |
| `internal/web/service/tgbot/tgbot_send.go` | same |
| tests next to the above | keep HOOK assertions |

### Frontend

| File | Why |
|---|---|
| `frontend/src/schemas/primitives/protocol.ts` | awg/naive/olcrtc/qwdtt/mieru/trusttunnel/anytls/tproxy/cover |
| `frontend/src/schemas/protocols/inbound/index.ts` | union those schemas |
| `frontend/src/schemas/protocols/inbound/wireguard.ts` | lucx keepalive |
| `frontend/src/schemas/client.ts` | lucx client fields |
| `frontend/src/schemas/setting.ts` | AWG sub URI, Happ ROSCOM, favicon, log days |
| `frontend/src/schemas/xray.ts` | lucx xray |
| `frontend/src/schemas/node.ts` | lucx capability |
| `frontend/src/lib/xray/inbound-defaults.ts` | default AWG/sidecar settings |
| `frontend/src/lib/xray/inbound-link.ts` | genAwgLink / sidecar URIs |
| `frontend/src/lib/xray/link-label.tsx` | lucx protocol labels |
| `frontend/src/lib/xray/protocol-capabilities.ts` | lucx caps |
| `frontend/src/pages/inbounds/form/InboundFormModal.tsx` | one form, lucx protocol tabs, AwgInboundIdProvider |
| `frontend/src/pages/inbounds/form/protocols/index.ts` | export lucx fields |
| `frontend/src/pages/inbounds/list/*` | lucx list/actions |
| `frontend/src/pages/inbounds/info/*` | lucx info |
| `frontend/src/pages/inbounds/InboundsPage.tsx` | lucx page |
| `frontend/src/pages/inbounds/useInbounds.ts` | lucx |
| `frontend/src/pages/clients/*` | AWG QR/.conf, tunnel clients |
| `frontend/src/pages/sub/SubPage.tsx` | AMNEZIA / vpn:// |
| `frontend/src/pages/xray/*` | kernel AWG outbounds in Outbounds, routing |
| `frontend/src/pages/settings/GeneralTab.tsx` | lucx settings |
| `frontend/src/pages/index/GeodataSection.tsx` | ROSCOM |
| `frontend/src/pages/index/VersionModal.tsx` | lucx version |
| `frontend/src/pages/login/*` | lucx login chrome |
| `frontend/src/layouts/AppSidebar.tsx` | Tunnels menu |
| `frontend/src/routes.tsx` | `/panel/tunnels` |
| `frontend/src/api/queryKeys.ts` | lucx keys |
| `frontend/src/api/queries/useOutboundTags.ts` | lucx tags |
| `frontend/src/hooks/useTheme.tsx` | Sand/Graphite |
| `frontend/src/hooks/useXraySetting.ts` | lucx |
| `frontend/src/models/status.ts` | lucx status |
| `frontend/src/styles/page-shell.css` | lucx chrome |
| `frontend/src/env.d.ts` | lucx env |
| `frontend/src/pages/api-docs/endpoints.ts` | lucx API docs |
| `frontend/src/test/inbound-link.test.ts` | lucx vpn:// |
| `frontend/src/test/wireguard-client-config.test.ts` | AWG conf |
| `frontend/src/test/link-label.test.ts` | lucx labels |
| `frontend/src/test/rule-form-preserve-fields.test.tsx` | lucx routing fields |
| `frontend/src/pages/inbounds/qr/QrPanel.tsx` | Happ QR 2953 (may lack HOOK marker — still ours) |

`QrPanel.tsx` is origin; the 2953 cutoff may sit outside a HOOK. Treat as overlay anyway.

## LucX-only (no origin twin)

Keep as whole files. Do not merge from origin.

- `internal/web/service/inbound_lucx.go`, `client_awg.go`, `awg_host.go`
- `internal/web/controller/awg.go`, `awg_outbound.go`
- `internal/web/job/awg_job.go`
- `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `awg-inbound-id-context.ts`
- `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx`
- `frontend/src/pages/settings/CoresTab.tsx`
- `frontend/src/lib/sub/fetchBody.ts`
- `.github/workflows/upstream-watch.yml`
- packages `internal/awg/`, `internal/lucx/` (and other PolyForm files in Rule 10)

## Refresh

A file is overlay iff it has `LUCX-HOOK` **and** exists on `origin/main`. If you add a HOOK to a new origin file, add a row here in the same commit.
