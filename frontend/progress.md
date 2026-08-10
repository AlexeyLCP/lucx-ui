
## Релиз v3.6.0-lucx.95 (2026-08-10) — qWDTT tunnel sidecar

**feat(tunnel): qWDTT** — третий туннельный сайдкар (WireGuard over VK TURN).

**Upstream:** SpaceNeuroX/proxy-turn-vk-android server.go (GPL-3.0). Design ref — Bebrik2283555/Ex3-ui extras.

**Backend:**
- `const Qwdtt Name = "qwdtt"` + BinaryName `qwdtt-linux-{arch}`
- `QwdttConfig` + BuildArgs + ClientURI (qwdtt://) + LegacyURI (wdtt://) + Subscription JSON
- Instance.Args for CLI-driven cores; empty ConfigText = no config file write
- Settings key `lucxTunnel_qwdtt`; state dir bin/tunnel/qwdtt-data (passwords.json, keys)
- API `/panel/api/tunnel/qwdtt/*`
- release.yml: binary from Ex3-ui v1.0 tarball (extra-qwdtt → qwdtt-linux-{arch})

**Frontend:** QwdttCard (root warning, form, URIs, sub JSON), i18n x13.

**Caveat:** needs root/CAP_NET_ADMIN; VK call hashes are operator-supplied.

**lucxVersion:** lucx.95. Tunnel sidecars complete (Naive + olcRTC + qWDTT).