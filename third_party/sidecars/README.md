# Tunnel sidecar binaries

gzipped amd64 binaries fetched by `install.sh` / `update.sh` into `/usr/local/x-ui/bin/`.
Not packed into `x-ui-linux-amd64.tar.gz` (SourceCraft 100 MB release cap).

| File | Upstream |
|---|---|
| `caddy-naive-linux-amd64.gz` | klzgrad/forwardproxy `v2.11.2-naive` |
| `naive-client-linux-amd64.gz` | klzgrad/naiveproxy `v150.0.7871.63-1` |
| `olcrtc-linux-amd64.gz` | openlibrecommunity/olcrtc `3339cd36` |
| `qwdtt-linux-amd64.gz` | Bebrik2283555/Ex3-ui `v1.0` extra-qwdtt |
| `mieru-linux-amd64.gz` | enfein/mieru `v3.35.0` mita |
| `mieru-client-linux-amd64.gz` | enfein/mieru `v3.35.0` mieru |
| `trusttunnel-linux-amd64.gz` | TrustTunnel/TrustTunnel `v1.0.33` |
| `trusttunnel-client-linux-amd64.gz` | TrustTunnel/TrustTunnelClient `v1.0.49` |

Licenses stay with those projects. Refresh from a LucX release tarball:

```bash
python -c "..."  # or extract from x-ui-linux-amd64.tar.gz and gzip -9
```
