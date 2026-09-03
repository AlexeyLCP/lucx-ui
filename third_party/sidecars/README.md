# Tunnel sidecar binaries

gzipped amd64 binaries fetched by `install.sh` / `update.sh` into `/usr/local/x-ui/bin/`.
Not packed into `x-ui-linux-amd64.tar.gz` (SourceCraft 100 MB release cap).

| File | Upstream |
|---|---|
| `caddy-naive-linux-amd64.gz` | klzgrad/forwardproxy `v2.11.2-naive` |
| `naive-client-linux-amd64.gz` | klzgrad/naiveproxy `v150.0.7871.63-1` |
| `olcrtc-linux-amd64.gz` | openlibrecommunity/olcrtc `ebe518a2` (OLC2) |
| `qwdtt-linux-amd64.gz` | SpaceNeuroX/proxy-turn-vk-android `v1.4.2` (`6c2f7a62`) `./server` |
| `mieru-linux-amd64.gz` | enfein/mieru `v3.35.0` mita |
| `mieru-client-linux-amd64.gz` | enfein/mieru `v3.35.0` mieru |
| `trusttunnel-linux-amd64.gz` | TrustTunnel/TrustTunnel `v1.0.33` |
| `trusttunnel-client-linux-amd64.gz` | TrustTunnel/TrustTunnelClient `v1.0.49` |
| `anytls-linux-amd64.gz` | anytls/anytls-go `v0.0.13` `anytls-server` + LucX `-cert/-key` overlay |
| `tproxy-linux-amd64` / `mtproxy-linux-amd64` | telegramdesktop/tproxy-server `52a5feb7` + TelegramMessenger/MTProxy `f36d8af7` — packed by `.github/workflows/release.yml` into the GitHub amd64 tarball (not stored as gz here) |

Licenses stay with those projects. Refresh from a LucX release tarball:

```bash
python -c "..."  # or extract from x-ui-linux-amd64.tar.gz and gzip -9
```
