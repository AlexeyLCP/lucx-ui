# 03 — Environments

Extracted from AGENTS.md. This file is project law.

---

## Deploy

- ⚠️ **GCP prod (`lucx`, 34.88.71.12, GCP Finland) is CLOSED** (2026-08-03, owner decision): Google projects wound down, server is no longer a deploy target. **Do NOT connect there, update it, or treat it as “unreachable prod that needs bringing up”.** The only live project server is `lucx-test2` (below). SSH alias `lucx` in `~/.ssh/config` is historical — do not use.
- **Target:** `lucx-test2` (144.31.157.106, poor-rose-snake.play2go.cloud) — the only test/verification server.
- **Service:** `x-ui.service` (systemd)
- **Procedure:** `x-ui update` on the server (pulls latest release + new `update.sh`: SHA-gate for AWG module rebuild + kernel-gate) → verify `systemctl status x-ui` + logs. Clean install — `install.sh` (see Release & Install).
- **AWG runtime check:** `awg show` should list active interfaces; `ip link show awgN` for TUN

### Test servers (SSH aliases in `~/.ssh/config`, user `root`, key `~/.ssh/id_ed25519`)

| Alias | IP | Host | Purpose |
|---|---|---|---|
| (no alias — `root@144.31.224.212`) | 144.31.224.212 | skinny-azure-snail.play2go.cloud | **Only test stand** (since 2026-08-05): install tests, web update, AWG runtime, release checks. Debian 13, key-auth |

- **144.31.157.106 (lucx-test2)** — reinstalled by owner 2026-08-05, SSH unstable (drops on kex); **no longer used** — stand moved to 144.31.224.212 per Alexey (“forget other test boxes”).
- **Testers:** VladufQa, Kirill Rudenko — update themselves via `x-ui update` or reinstall; do not touch their panels without asking.
- **Keenetic (experimental, since 2026-10-25 / lucx.316):** router install via `bin/lucx-keenetic.sh` (Entware wrapper, adapted from own-open-source/x-ui-keenetic, GPL-3.0) + `docs/content/docs/en/guide/install-keenetic.mdx`. aarch64/amd64 only (no mips/armv7 releases). No DKMS — AWG runs on userspace amneziawgnet; wgobfs fronts stock KeeneticOS kernel WireGuard (needs `ip-full` + `wireguard-tools`). First hardware report (2026-10-25): install worked, but the tarball's Linux `x-ui.sh` had overwritten the wrapper (`x-ui` / `S99x-ui` dead) and the dashboard disk widget read `/` (firmware RAM overlay) instead of the OPKG volume. Wrapper fix is in the script on `main` (re-fetch it); disk figure is lucx.319.

### Stand state (2026-11-13, live check of lucx.361–363)

- The stand runs the **dev build** (`dev-latest` → lucx.363), installed by unpacking
  `releases/download/dev-latest/x-ui-linux-amd64.tar.gz` over `/usr/local/x-ui`
  (DB, certs and `/etc/x-ui` untouched). The old binary is backed up as
  `/root/x-ui.bin.bak-<epoch>`; the DB before the test as `/root/x-ui.db.bak-<epoch>`.
- The panel DB migrated **lucx.342 → lucx.363** without a hiccup (Rule 0b overlay path).
- The panel cert `/root/cert/idoctor.mom/fullchain.pem` **expired 2026-11-10** and
  `idoctor.mom` now resolves to another host, so the cover inbound (id 1) stays
  disabled ("certificate expired") — renew the cert before using that inbound.
- Test inbounds left in place (tags `in-zig-tcp`, `in-mieru-tcp`,
  `in-tproxy-standalone`; ports 52888 / 20101 / 8443) plus the client
  `mieru-e2e@test` (sub id `e2emieru`) and the setting `subClashEnable=true`.
  The Clash subscription is served over **https** on :2096 (`/clash/<subid>`).
  Remove with: stop x-ui → delete those rows → start x-ui.

---
