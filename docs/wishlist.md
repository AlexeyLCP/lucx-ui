# LucX-UI Wishlist

## UI documentation rule

Every new protocol, backend selector, plugin permission, binary, and advanced
setting must ship with a plain-language label and help text. Explain what it
changes, when to use it, compatibility/rollback behavior, and any requirement
that can make the feature unavailable. Do not expose a bare technical option
without an operator-facing description.

## Plugin UX requirements

When the plugin runtime is implemented, its UI must explain: source and version,
permissions (API, network, subscription seams), process lifecycle, storage
location, logs, restart behavior, and fallback behavior when a plugin times out
or exits. The panel must show an explicit warning before enabling a plugin that
can change subscriptions or access network credentials.

This is the working list of user requests. Items are deliberately kept separate
from the changelog until they are implemented or rejected.

## Planned

1. Balancer-only inbound visibility.
2. Dynamic subscription membership for balancer outbounds.
3. MTProto `limit IP` compatibility with the mtg log format, if the current
   online-stats path proves insufficient on a real deployment.
4. AWG-outbound status, handshake, traffic counters and route-aware ping.

## Research

1. MASQUE Apple Network Relay export. Scope is explicitly **MDM/DDM JSON only**:
   LucX-UI does not promise a standalone `.mobileconfig` until Apple documents
   and a real iOS/macOS test confirms that delivery path. The generated
   configuration must be installed through a compatible MDM/DDM service; Apple
   device compatibility and the absence of a VPN indicator remain manual checks.
2. MTProto ZIG / the implementation described as “запретом 2”.
3. Telegram Web Proxy performance on mobile and desktop, with and without VPN.
4. `better-csqtt`: https://github.com/AllIsA1/better-csqtt

## Implemented operator notes

### Apple Network Relay

MASQUE inbounds can export an Apple Network Relay DDM JSON document from the
inbound form. The export includes HTTP/2/HTTP/3 relay URLs, domain/FQDN routing,
DNS failover, UI toggle, relay UUID, and public-key asset references. It is
deliberately not a `.mobileconfig`: Apple documents this configuration as a
declarative management payload, so an MDM/DDM service must deliver it. The
current Apple documentation lists iOS/iPadOS/macOS/visionOS 27 as the minimum
supported OS family. Device behavior and VPN indicator presentation still need
verification on real Apple hardware.

### Telemt and MTProto Zig

Telemt is a separate Telegram MTProto core. Keep its TLS domain stable after
sharing links, because changing it changes newly generated links. Use the
Telemt card in Settings → Cores to install a pinned sidecar binary and inspect
its logs/status.

MTProto Zig is an opt-in Telegram WEB backend pinned to upstream `v1.15.4`.
Legacy `tproxy-server + MTProxy` remains the default. Switching to MTProto Zig
can require new WEB links and must be tested with the target Telegram Desktop
client and a real TLS/WEB relay topology; the panel's process status alone is
not an end-to-end WEB compatibility test.

Implementation started as a native LucX integration: persistent members in
`csqtt_members`, a 30-second sync job, per-client links in `GetLink`, and the
existing shared link as fallback. The standalone plugin runtime is not needed
for this implementation.

`better-csqtt` was checked on 2026-10-31. Its own test suite passes and its
Linux amd64/arm64 binaries are present upstream. It is a LucX process plugin,
not a CSQTT core patch: it requires `lucx-plugin.json` loading, plugin
credentials, a Unix-socket process supervisor and the `sub/after` seam API.
The current LucX-UI tree has no plugin runtime or seam dispatcher, so the
plugin cannot be applied here yet. The next step is to add or merge the plugin
runtime first, then install `better-csqtt` and verify per-client links, member
creation/deletion, disabled clients and fallback behavior.

## Excluded for now

- Telegram bot notifications and custom Telegram bots.
- Mass AWG configuration ZIP export.
- Insi/HAPP interface changes.
