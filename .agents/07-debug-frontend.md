# 07 — Debug frontend

Extracted from AGENTS.md. This file is project law.

---

### Pattern 3: Frontend doesn’t see the AWG protocol
- **Cause:** Registration forgotten in one of: `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`.
- **Fix:** `grep -rn "awg\|Awg\|AWG" frontend/src/` — check all 5 registration points.

### Pattern 7: CI go-test red on `TestBuildFirefoxHello_NoGrease`/`TestBuildSafariHello_NoGrease` (pre-existing flaky)
- **Cause:** CI runs `go test -shuffle=on` (random seed every run). The `NoGrease` tests assert that Safari/Firefox ClientHello **does not contain** GREASE patterns `0a0a`/`fafa` in hex. But `buildSafariHello`/`buildFirefoxHello` (`cps.go`) **do write** GREASE via `greaseValue()` — `rng.Intn` over 16 values `[0x0A0A … 0xFAFA]`. Test passes in 14/16 cases (when rng doesn’t pick `0x0A0A`/`0xFAFA`), fails in ~1/10 shuffle runs. Reproducible locally: `go test ./internal/awg/cps/... -count=20 -shuffle=on`. **Not related to AWG changes** — a separate bug in obfuscation logic.
- **Fix (temporary):** `gh run rerun <id> --failed` — a different shuffle seed passes with high probability.
- **Fix (root, TODO separate issue):** either the test should check GREASE only in extension positions (not whole hex), or `buildSafariHello`/`buildFirefoxHello` shouldn’t write GREASE via rng (real Safari/Firefox fingerprints don’t use GREASE — only Chrome). Tried `SetRand(t *testing.T, …)` with `t.Cleanup` to isolate global `rng` — didn’t help (logic problem, not pollution); reverted. Do NOT rush a fix — this is CPS obfuscation domain logic.
- **Lesson:** on a CI job failure first check whether it’s your regression. Reproduce locally with the exact shuffle seed from the CI log (`-test.shuffle <N>`). If it passes locally and isn’t related to your files — it’s flaky, rerun is justified.

### Pattern 7b: ~~CI frontend red on storybook a11y `ConfigBlock.stories.tsx → Collapsed`~~ — FIXED (lucx.58)
- **Cause (refined from CI log, attempt 1 run 30816344911):** axe reported `insufficient color contrast of 2.29 (foreground #a6a6a6, background #f8f8f8)` on `<code class="config-block-text">`. `#a6a6a6` is the final text color (#595959 from `body.light .config-block-text`) **blended with the background at ~54% opacity**: the a11y addon runs axe right after play(), while antd Collapse is still in its expand fade-in. Timing race: axe hits either mid-fade (fail) or after (pass).
- **Fix (lucx.58):** `token.motion: false` in the storybook ConfigProvider decorator (`.storybook/preview.tsx`) — antd animations in stories disabled, expand is instant, axe always sees the final state. Production animations untouched (decorator is storybook-only). Verified with 3 consecutive runs. CSS not touched — `--ant-*` variables resolve correctly (antd cssVar mode scopes variables, doesn’t rename the prefix).
- **Lesson:** storybook a11y + appear animations = flaky color-contrast failures (axe measures mid-fade and sees a blended color). Disabling motion in the test decorator is the standard fix; don’t chase CSS if final contrast is fine.
- **Lesson:** if CI frontend fails on the SINGLE test `storybook ... ConfigBlock → Collapsed` with `color-contrast` — that’s this flake; check your tests are green and rerun, don’t spend time hunting a regression in your code.

### Pattern 8: “can disable outbound, can’t enable” + awgo vs inbound-client subnet collision — FIXED (lucx.69)
- **Cause 1 (enable button):** frontend `awgOutboundsApi.enable` didn’t pass `JSON_HEADERS`; `http-init.ts` serializes the body as JSON **only** with `Content-Type: application/json`, else form-urlencoded (`enable=true`). Backend `_ = c.ShouldBindJSON(&body)` silently failed to parse the form body → `body.Enable=false` → every enable became a disable. **Lesson:** any frontend POST with a JSON body must pass `JSON_HEADERS`; in the controller don’t swallow bind errors (`_ =`) — check and fail loudly.
- **Cause 2 (subnet collision):** an awgo outbound takes its address from the provider .conf (often 10.8.0.0/24). If AWG inbound clients sit in the same /24 (legacy wrong-subnet), reverse path breaks → flood of `ERROR XRAY: proxy/tun: connection was refused`. Old `checkAwgSubnetConflict` only checked the inbound server address and only on INBOUND save.
- **Fix (lucx.69):** (1) frontend enable → `JSON_HEADERS`; (2) backend `ShouldBind` with `json:"enable" form:"enable"` + error check; (3) new guard `AwgOutboundService.checkSubnetConflict`→`awgOutboundSubnetClash` in outbound Add/Update: checks awgo address against both the server subnet and **client IPs** of every AWG inbound (`awgSettingsClientIPs`), /32-/128 exempt.
- **Diagnosing “proxy/tun: connection was refused”:** errors SPAM only with a live awgo outbound carrying traffic and vanish when the outbound is disabled/removed → first check awgo subnet against AWG inbound client subnets (`ip route`, `awg show`). This is NOT a TUN/Xray bug per se, it’s a routing collision. SECOND common cause (lucx.72, VladufQa): **broken/multiple DNS on an inbound** that goes into the outbound — clients resolve bad IPs and the server dials them → RST/timeout; fix with one working DNS (1.1.1.1) on the inbound. Outbound ParseConf since lucx.72 takes only the first DNS from “a, b”.
- **Lesson:** locally on Windows package `internal/web/service` won’t build (no gcc → CGO `sqlite3.Backup`); check pure logic with a standalone program, full test — GitHub Actions CI (see Test Commands).

### Pattern 10: address in links/subscription = the subscriber’s own IP (X-Real-IP) — FIXED (lucx.125)
- **Symptom:** on an inbound without its own address (wildcard listen, no node, no managed host — typical AWG form) the subscription has a foreign address: `Endpoint` in `.conf`/`vpn://` or `server:` in the Clash profile equals the WAN/local IP of whoever downloaded the subscription. For the same operator’s VLESS/Reality/WS the address is correct — simply because those inbounds have their own address (host entry, listen, or shareAddr). Reports Aleksandr SacredX: lucx.89 (`.conf`), lucx.124 (Clash).
- **Cause:** `ResolveRequest` (internal/sub/service.go) took `host` in order `X-Forwarded-Host` → **`X-Real-IP`** → `Host`. Nginx per our own docs sets `proxy_set_header X-Real-IP $remote_addr` — i.e. the CLIENT address, and it’s “routable”, so `PrepareForRequest` didn’t reject it. Then `resolveInboundAddress` falls through to `s.address` as the last chain link (shareAddr/node/listen → subDomain/webDomain → request address) — and substitutes the subscriber’s IP. Same order was in `resolveHost` (internal/web/controller/inbound.go) — from there the admin’s IP landed in panel links and QR.
- **Fix (lucx.90, partial):** `AwgEndpointHost` — only for `/awg/`. Other formats (raw `/sub/`, `/json/`, `/clash/`) and panel links kept leaking.
- **Fix (lucx.125, full):** shared `requestServerHost(c, trusted)` — trusted `X-Forwarded-Host`, else host from `Host`; `X-Real-IP` is never read. `ResolveRequest` and `AwgEndpointHost` moved onto it; `X-Real-IP` branch removed from controller `resolveHost`. Tests: `TestResolveRequest_HostNeverRealIP`, `TestGetProxies_AwgWithoutOwnAddressUsesSubscriptionHost`, `TestResolveHostNeverUsesRealIp`.
- **Diagnostics (one command, repro without a client):** `curl -s -H 'X-Real-IP: 203.0.113.77' https://<sub-domain>/clash/<subId> | grep -B2 'type: wireguard'` — if `server:` shows `203.0.113.77`, the panel still leaks.
- **Lesson:** `X-Real-IP`/`X-Forwarded-For` answer “who came”, not “where to connect”. In any code that builds a SERVER ADDRESS for the client, those headers are forbidden; only `X-Forwarded-Host` (from a trusted proxy) and `Host` are allowed.

### Pattern 11: Client Info vpn:// button identical on every AWG inbound — FIXED (lucx.155)

- **Cause:** the button sat inside `awgConfigs.map` but copied `linksBuilt.amneziaVpn` (`/awg/{subId}?format=vpn`). That endpoint returns every attached inbound. `.conf` already used `cfg.text` per inbound.
- **Fix:** `GetAwg` / `awgBody` / public `/awg/` take `inboundId`. The button appends it. Omit the param → all profiles, as before.
- **Seen on:** Kirill, 2026-08-22.
