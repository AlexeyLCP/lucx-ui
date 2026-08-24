# 07 — Debug tunnels

Extracted from AGENTS.md. This file is project law.

---

### Pattern 1m: tunnel-sidecar lives on after inbound delete (“spams logs even though I deleted it”) — FIXED (lucx.115)
- **Cause:** dual source of truth for tunnel cores. Besides inbounds (`olcrtc-{id}` / `naive-{id}`) the legacy path lives on: settings blob `lucxTunnel_{naive,olcrtc,qwdtt}` + Tunnels-page card (Start/Stop/Save) + manager key without prefix (`olcrtc`). `reconcile{Naive,Olcrtc}Inbounds` with NO inbounds fell back to the blob and `Ensure`’d the legacy core: a blob with `enabled:true` resurrected the process every tick (10 s). Deleting the inbound only tore down `{core}-{id}`. How the blob becomes `enabled:true` after lucx.102 migration: migration writes `migratedToInbound` and REMOVES `enabled`, but the legacy Start/Save button on the Tunnels page re-saves the blob as a struct without the marker and with `enabled:true`. Two adjacent gaps: with empty `want`, `Reconcile{Naive,Olcrtc}` wasn’t called → orphan `{core}-{id}` weren’t swept; a migrated blob was treated as legitimate desired-state. Same footgun on all three cores (naive/olcrtc/qwdtt).
- **Symptom (VladufQa, 13.08.2026):** deleted olcRTC inbound — panel keeps spamming olcrtc `[ice] TRACE` logs, process holds the client (STUN from client IP).
- **Fix (lucx.115, `internal/web/service/tunnel.go`):** (1) `tunnelBlobMigrated(key)` reads the marker; all three reconciles’ fallback under a migrated blob force `Enabled=false`; (2) fallback instead of bare `Ensure` calls `Reconcile{Naive,Olcrtc}` with the legacy instance in want → orphan `{core}-{id}` are swept even with an empty inbound list; (3) `legacyLifecycleBlocked` — Start/Restart/Save of legacy endpoints refuse (“manage on the Inbounds page”) if the blob is migrated OR a protocol inbound exists; Stop is NOT blocked (zombie-kill button).
- **Diagnostics:** `ps aux | grep -E 'olcrtc|caddy-naive|qwdtt'` — config path in argv reveals the key: `tunnel/olcrtc.yaml` = legacy core, `tunnel/olcrtc-N.yaml` = inbound orphan. Blob state: `sqlite3 /etc/x-ui/x-ui.db "select value from settings where key like 'lucxTunnel_%'"` (`enabled:true` without marker = zombie state).
- **Healing an already-hit host without update:** Tunnels → core card → Stop (persists enabled=false), or `pkill -f olcrtc-linux` / `pkill -f caddy-naive` — current reconcile without the fix will restart it, with the fix it won’t.
- **Lesson:** if a feature migrates storage (settings blob → inbound), the reconcile fallback to old storage must respect the migration marker, and old-path lifecycle endpoints must refuse after migration. Otherwise deleting the NEW entity resurrects the OLD one. Orphan-key sweep must work even with empty `want`.

### Pattern 1o: olcRTC “worked, then broke” after panel update — upstream wire-break + unpinned master — FIXED (lucx.132)
- **Symptom:** olcRTC tunnel (any provider — Telemost/Jitsi/WB) worked; after `x-ui update` / web update clients stop connecting though config/room/provider didn’t change. Report NoName (16.08.2026): “Friday it flew through Yandex, yesterday it stopped”.
- **Cause:** `release.yml` built olcrtc from **unpinned `master`**. Upstream on 14.08.2026 merged PR #140 “Refactor/global overhaul” (252 files), fully rewriting the crypto layer: old format (raw key → XChaCha20-Poly1305, frame `[24B nonce][ct][tag]`) replaced by “OLC2” (HKDF-SHA256 directional keys, frame `magic "OLC2"|counter|16B prefix|ct|tag`, replay-window, AAD). **No fallback to the old format** — their readme: “no compatibility fallback… Upgrade both endpoints together”. Server after update speaks OLC2, client app (owenclave/olcbox) stays on old crypto → no packet passes auth → tunnel is dead. YAML/URI schemas are compatible — only the data plane breaks.
- **Diagnostics:** binary version isn’t printed (`usage: olcrtc <config.yaml>` on any flag). Check the date of `/usr/local/x-ui/bin/olcrtc-linux-amd64` (matches last panel update) and sidecar logs `journalctl -u x-ui | grep -i olcrtc` (prefix `olcrtc: <label> |`). Key signal: server was updated, client app was not.
- **Fix (lucx.132):** pin `OLCRTC_REF` in release.yml to `3339cd36716885e583429f97e73462cde4984e2e` (last master before PR #140 = lucx.112–118 binary, verified on Telemost). Clone via `git init + fetch --depth 1 <SHA> + checkout FETCH_HEAD`: `git clone --branch <SHA>` on GitHub does NOT work (“Remote branch … not found”), fetch by SHA does.
- **Healing an already-hit host without lucx.132:** pull `olcrtc-linux-amd64` from tarball `v3.6.0-lucx.118`, replace `/usr/local/x-ui/bin/`, restart the inbound; or install an OLC2 build on the client (olcbox nightly from 16.08.2026+; owenclave has no OLC2 yet).
- **Lesson:** external sidecar binaries built from someone else’s unpinned `master` are a bomb: any upstream wire-breaking merge silently rides into the next release. ALL cores are pinned (mieru/TrustTunnel/caddy-naive already were; olcrtc was the exception). Lift the olcrtc pin only when clients ship OLC2 builds AND e2e has been run on a real provider.

### Pattern 1p: TrustTunnel “listens, no traffic” + outbound AWG — FIXED (lucx.133)
- **Symptom:** process `trusttunnel-N` UP, TCP/UDP :443 listening, client connects, no internet. Log: `trusttunnel egress : target tag [ SW ] not found, skipping injection`. `ss` shows no loopback SOCKS (`routeXrayPort`).
- **Cause:** `injectAwgOutbounds` ran after the SOCKS inject. `injectSocksEgress` on an unknown tag **exited entirely** — SOCKS never came up, while TOML already wrote `socks5 = 127.0.0.1:<port>`. AWG-TUN doesn’t do that (bridge always).
- **Fix (lucx.133):** awgo tags are injected before egress; unknown tag → warning + SOCKS without force-route.
- **Workaround without update:** turn off “via Xray” or clear outbound (empty = catch-all, SOCKS comes up).
- **Lesson:** a bridge injector must not be all-or-nothing because of an optional force-route. The rule target must exist **before** lookup.

### Pattern 1q: attach qWDTT → client update “empty client ID” — FIXED (lucx.144)
- **Symptom (Tuna, 20.08.2026):** after attaching qWDTT, Clients save fails `POST /panel/api/clients/update/fox` → `empty client ID`. Same for olcRTC.
- **Cause:** qWDTT/olcRTC are `shareOnlySidecar` — membership is `client_inbounds` only, inbound settings have no `clients[]`. Attach/detach skipped that JSON; `UpdateInboundClient` still scanned settings, missed the email, returned `empty client ID`.
- **Fix (lucx.144):** `UpdateInboundClient` no-op for share-only; `Update` skips those inbounds and still writes `ClientRecord` when nothing else is attached. Tests `TestUpdateAfterShareOnlyAttach`, `TestUpdateShareOnlyOnlyClient`.
- **Lesson:** if attach/detach have a protocol-specific path, **update must too**. Walking “every inbound id” through the Xray-clients JSON path breaks share-only sidecars.

### Pattern 1r: TrustTunnel `client_random_prefix=%2F` — NekoBox+ drops it — FIXED (lucx.145)
- **Symptom (doc. bravn, lucx.144):** Throne/NekoBox+ link has `client_random_prefix=3eb5d634%2Fffffffff`; NekoBox+ does not write the prefix.
- **Cause:** lucx.142 appended the prefix via `url.QueryEscape`, which encodes `/` as `%2F`. The value is only hex or hex/mask (`ValidClientRandomPrefix`) — `/` is safe raw in the query. NekoBox+ does not URL-decode that param.
- **Fix:** `ClientURI` writes `client_random_prefix=` as-is. Test forbids `%2F`. TLV `tt://?` already carried `/` in binary (unchanged).

### Pattern 1t: update `gunzip failed` / `Text file busy` on caddy-naive / trusttunnel — FIXED (lucx.164)
- **Symptom:** `x-ui update` / web update prints `/dev/fd/63: line 126: bin/caddy-naive-linux-amd64: Text file busy` and `gunzip failed` (same for `trusttunnel-linux-*` or any live sidecar). Curl progress bars succeed. Panel comes up.
- **Cause:** lucx.161 moved sidecar fetch AFTER `systemctl start x-ui`. Reconcile execs the old `bin/<core>-linux-*`. `gzip -dc > dest` opens that inode O_WRONLY → ETXTBSY. Non-running sidecars (clients, unused cores) unpack fine.
- **Fix (lucx.164):** write `${name}.new`, `mv -f` onto dest (directory entry swaps; old process keeps old inode), `pkill -f ${name}` so the next reconcile tick execs the new file.
- **Healing an already-hit host without lucx.164:** panel is already new. Only the binaries that printed `gunzip failed` stayed old. Either leave them (if that core did not change) or:
  `pkill -f caddy-naive-linux-amd64; pkill -f trusttunnel-linux-amd64`
  then re-run `x-ui update` once lucx.164 is on `main` (menu curls fresh `update.sh`).
- **Lesson:** never write over an executable that may be running. Same rule the Go Cores download already follows (`dst+".download"` + `Rename`).

### Pattern 1u: qWDTT paste of the full panel link hangs on DTLS — FIXED (lucx.167)
- **Symptom (VladufQa, 24.08.2026):** SpaceNeuroX 1.4.2. Compact `wdtt://ip:dtls:wg:local:pass:hash` connects. A lone `qwdtt://config?…` connects. Pasting the panel's two-line block (modern + legacy) hangs on `[DTLS] Handshake…` / step DTLS 10s.
- **Cause:** `genQwdttLink` (frontend + `/sub/`) concatenated `qwdtt://config?…\nwdtt://…`. Client `SubscriptionImport.parsePayload` does `startsWith("qwdtt://config")` then `Uri.parse` of the **whole** clipboard. The second line rides into `pass` → wrong password → DTLS timeout. Official APK export is a single `qwdtt://config?` line (`ExportProfileSheet`).
- **Fix (lucx.167):** emit only `qwdtt://config?`. `LegacyURI` stays a separate copy on the Tunnels card. Tests forbid a newline / `wdtt://` in the share string.
- **Healing without lucx.167:** paste only the first line, or only the `wdtt://` line, never both.
- **Lesson:** one client, one clipboard URI. Two formats for two apps (TrustTunnel TLV + Throne) is fine; two formats for the same SpaceNeuroX importer is not.

### Pattern 1s: qWDTT DTLS handshake timeout 10s — stale sidecar vs client 1.4.2 — FIXED (lucx.163)
- **Symptom (VladufQa, 22.08.2026):** SpaceNeuroX client log: DNS/VK/WRAP/TURN green, then `[DTLS] Handshake…` and `step DTLS did not finish in 10s`. Operator asked whether listen should be `0.0.0.0:56000` or the server IP.
- **Cause:** two independent footguns. (1) **Listen vs SubHost:** `-listen` is the server bind (`0.0.0.0:56000`); `subHost` is the advertised `peer` (`PUBLIC_IP:56000`). Putting `0.0.0.0` in `subHost` makes the client DTLS to itself. (2) **Wire skew:** we shipped Ex3-ui `v1.0` extra-qwdtt (~qWDTT 1.4.0, 09.08). Client `v1.4.2` (19.08) commit `Harden server authentication and transport` bumped pion/dtls 3.1.2→3.1.5 and changed WRAP/auth. Same timeout is also upstream issue #31 (even 1.4↔1.4 around 15.08).
- **Fix (lucx.163):** pin `qwdtt-linux-*` to SpaceNeuroX SHA `6c2f7a627d9fc0b54240035818ff86b6d2b6c76f` (`v1.4.2` `./server`). Live binary is `third_party/sidecars/linux-amd64/qwdtt-linux-amd64.gz` (install/update fetch). `release.yml` / `sourcecraft-release.sh` build from that SHA, not Ex3-ui. CLI flags (`-listen/-password/-dns/…`) unchanged.
- **Diagnostics:** `ps aux | grep qwdtt`; `ss -ulnp | grep 56000`; inbound `listenAddr=0.0.0.0:56000` and `subHost=<public>:56000`; client APK ≥ 1.4.2. Binary date under `/usr/local/x-ui/bin/qwdtt-linux-amd64` older than 19.08.2026 = this bug.
- **Healing an already-hit host without lucx.163:** replace `/usr/local/x-ui/bin/qwdtt-linux-amd64` with a `v1.4.2` `./server` build (Cores upload or gunzip the third_party blob), restart the inbound. Confirm `subHost` is the public IP, re-issue the `qwdtt://` link.
- **Lesson:** same as Pattern 1o. A sidecar taken from a third-party panel tarball ages in silence. Pin the upstream SHA of the protocol we claim to speak; bump only with a matching client.
