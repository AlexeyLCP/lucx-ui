# Linux kernel WireGuard managed by LucX

## Intent
Manage standard Linux WireGuard clients in the same panel as kernel AWG, retaining official WireGuard client compatibility and existing Xray destination routing. The operator requested implementation in a personal fork, production validation on their own gateway, and an upstream pull request.

## Design decisions
Add an explicit `kernel` opt-in to the existing `wireguard` inbound settings. Absent/false keeps the existing implementation and exported client profiles unchanged. The WireGuard form labels the choice as a Linux kernel backend switch. This avoids adding a second protocol/client model and preserves subscription compatibility. It is an implementation choice, not an upstream requirement.

Kernel settings add `address` (server CIDR), `tproxyPort`, `fwMark`, and `kernel` while retaining `secretKey`, `mtu`, `dns`, `clients` and legacy `peers`. Kernel mode initially routes TCP/UDP through Xray TPROXY; direct egress is expressed using existing Xray routing rules. It must not silently fall back to userspace.

Place the runtime in `internal/lucx/wireguard/`; upstream integration stays in LUCX-HOOK blocks. The runtime uses Linux wireguard and ip/wg, never amneziawg. Interface names are `lxwg<ID>`, separate from foreign wg0 and LucX AWG interfaces. Generated files live in a dedicated LucX-owned directory. Peer changes use `wg syncconf`; device changes may recreate only that interface. Kernel state feeds per-client traffic, handshake-based online status, and existing live-speed broadcasts without counting routed inbound traffic twice.

TPROXY uses interface-scoped rules, isolated routing tables/marks, transparent loopback listeners and priority after ordinary conntrack-mark restore chains. Preserve Xray rule order; do not prepend a forced outbound. Drop unsupported forwarding/IPv6 rather than leaking it to the host default route. Validate IPv4 server subnet, peer subnets, keys, ports, interface ownership and resource collisions. Initial kernel TPROXY support is IPv4; unsupported IPv6 peers receive an explicit validation error. This limit must appear in the UI/docs.

Provide an explicit import/preview operation for a standard wg-quick server configuration. Reject arbitrary shell hooks with an actionable message, or require them to be removed after operator review; never run imported hooks. Preserve keys, PSKs, IPs, port and MTU. Server configs do not contain client private keys: importing cannot recover those; existing profiles remain valid, and full export requires separately supplied private keys. Do not stop or adopt a foreign interface automatically. Production transfer from existing wg0 is a separate explicit deployment operation with backup/rollback; no startup database migration imports foreign interfaces.

## Safety and compatibility
No automatic rewriting of existing WireGuard/AWG settings or client records. Update existing schemas only with optional fields. Kernel mode is local Linux only; unsupported platforms/nodes fail clearly. Disabled/expired/quota-exhausted clients must be absent from the live peer set. Disable/delete cleans only resources owned by that inbound. Never log private keys, PSKs, profiles, or subscription IDs.

## Validation and rollout
Use failing unit/regression tests before implementation, real temporary SQLite databases for integration, existing frontend unit tests, typecheck/lint/build, Go build/vet/tests and an independent review. Build Linux release artifacts in the personal fork's CI, not on production.

Deploy only to the operator's authorized gateway. Back up database, binaries, unit files, firewall/routing and wg0 config; schedule rollback before cutover. Start with a new synthetic kernel WG inbound and independent test key; verify handshake, UDP DNS, HTTPS, egress routing, peer enable/disable, live accounting and multiple rekeys. Transfer external wg0 ownership only in a separate provisioning cutover; feature installation leaves it intact. Preserve AWG and transit. Document service interruption and provider-path limitations. Open an upstream PR with the final implementation and measured validation.

## Non-goals
Kernel WG outbounds, obfuscation, replacing the AWG runtime, new routing/balancer policy, automatic migration of every host, protocol performance guarantees, and changes to unrelated proxy services.

## Runtime contracts

- Install an owned nft guard before bringing a new device up. The IPv4 prerouting chain runs at mangle + 2: exclude the server subnet, TPROXY TCP/UDP to loopback, then drop unmatched traffic. Forwarding from the managed interface is dropped; IPv6 ingress is dropped. Missing Xray listeners therefore fail closed. Reconcile reinstalls owned nft state after a flush.
- Resource names: device `lxwg<ID>`, nft table `lxwg_<ID>`, route table `51000 + ID`, mark `0x50000000 + ID`, ip-rule priority `10001`, config directory `<XUI_DB_FOLDER>/wireguard`. IDs are restricted to 1..65535. Refuse occupied/unowned devices, configs, route tables, marks and conflicting transparent listener ports. WireGuard transport binds wildcard because the Linux module cannot bind the UDP listener to an individual local IP; the UI rejects non-wildcard Listen in kernel mode.
- TPROXY listeners bind 127.0.0.1 only. Reject collisions with inbound ports (TCP or UDP), other kernel transparent ports and current host listeners, except a listener already owned by the same inbound during an update.
- Reject duplicate public keys and overlapping peer AllowedIPs, non-host addresses, the server address, and IPv6. If both legacy peers and clients are populated, reject kernel opt-in with a clear instruction to resolve ambiguity. Keepalive accepts a numeric value or decimal string in 0..65535; ranges are rejected, never silently collapsed.
- Sample usage before changing peers. First observation of an already-running device establishes a zero-charge baseline; new interfaces start at zero. Counter decreases reset the baseline without negative deltas. Kernel accounts per-client traffic; Xray accounts routed inbound totals. Final peer deltas must be persisted when peers are removed.
- Import is preview-only at the parsing endpoint; the normal authenticated AddInbound operation is the explicit commit. Preview sets enable=false and maps peers to stable operator-editable names. Public-key-only imports retain missing private keys and cannot offer downloadable full client profiles; never generate replacement keys for an existing public key.
- Production cutover must preserve the old Xray TPROXY tag/sniffing policy or explicitly translate every reference. External wg-quick/vpn-admin ownership must be disabled before enabling the imported peer set. Synthetic deployment is mandatory before transferring any live interface. If ownership cannot be transferred safely, leave existing wg0 externally managed and report the remaining migration step rather than disrupt it.
