# LucX-UI Construct — Session Snapshot (May 16, 2026 ~13:00 MSK)

## Quick Resume

```
Продолжи работу над lucx-ui. Прочитай CONSTRUCT.md, изучи историю коммитов, проверь состояние тестов и сервера.
```

## Repository

- **GitHub:** https://github.com/AlexeyLCP/lucx-ui (public)
- **Branch:** `lucx-ui-phase1` → `main`
- **Commits:** 85+
- **Release:** v0.1.5-pre-MVP (prerelease, 43MB tarball)
- **Local path:** `/home/lcp/3x-ui`
- **Test server:** vps-finland-lucx (GCP, 34.88.118.168, Debian 12)
- **Server panel path:** `/usr/local/x-ui/`
- **Server CLI:** `/usr/bin/x-ui`
- **Server systemd:** `x-ui.service` (NOT lucx-ui.service — binary kept as `x-ui`)

## Architecture

All new code isolated in `internal/lucx/` (Go, 31 files) and `frontend/src/lucx/` (Vue/JS, 9 files). Changes to 3x-ui files wrapped in `// LUCX-HOOK` / `// END LUCX-HOOK` markers. 152 markers total.

## Known Issues / Recent Fixes

1. **GitHub Releases CDN**: Direct download URL `github.com/.../releases/download/...` sometimes returns 404. Fix: API-based download (`/releases/tags/{ver}` → `browser_download_url`). Script: `install-lucx.sh`.

2. **Xray 0 bytes in tarball**: Local `cp /usr/local/bin/xray` fails when xray not installed. Fix: ALWAYS download Xray from GitHub: `curl https://github.com/XTLS/Xray-core/releases/download/v26.3.27/Xray-linux-64.zip`.

3. **`/releases/latest` returns 404 for prereleases**: Fix: use `/releases` (list all) + `head -1`.

4. **AWG client private key**: `crypto.subtle.generateKey('X25519')` produces CryptoKeyPair objects that break JSON serialization. Fix: plain `crypto.getRandomValues(32)` → manual base64 encoding. Never use Web Crypto key objects in JSON.

5. **AWG QR disabled then re-enabled**: Too-dense QR → `noQR: true` → reverted to normal QR (fits in version 40).

6. **Protocol dropdown order**: AWG and TELEMT moved to TOP of `Protocols` object in `inbound.js` via property order (JS preserves insertion order).

## Test Commands

```bash
cd /home/lcp/3x-ui/frontend && npm install && npm run build && cd ..
export PATH=/home/lcp/.local/go/bin:$PATH
export GOTOOLCHAIN=auto
go test ./internal/lucx/... ./internal/lucx/integration/... -count=1 -v
```

## Deploy to Server

```bash
export PATH=/home/lcp/.local/go/bin:$PATH
export GOTOOLCHAIN=auto
cd /home/lcp/3x-ui/frontend && npm run build && cd ../..
go build -C /home/lcp/3x-ui -o /tmp/x-ui -ldflags="-s -w" .
gcloud compute scp /tmp/x-ui vps-finland-lucx:/tmp/x-ui --zone=europe-north1-a
gcloud compute ssh vps-finland-lucx --zone=europe-north1-a --command="sudo systemctl stop x-ui && sudo cp /tmp/x-ui /usr/local/x-ui/x-ui && sudo chmod +x /usr/local/x-ui/x-ui && sudo systemctl start x-ui && sleep 2 && echo DEPLOYED"
```

## GitHub Release Build

```bash
rm -rf /tmp/lucx-rel && mkdir -p /tmp/lucx-rel/x-ui/bin
npm -C /home/lcp/3x-ui/frontend run build
go build -C /home/lcp/3x-ui -o /tmp/lucx-rel/x-ui/x-ui -ldflags="-s -w" .
curl -sL https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh -o /tmp/lucx-rel/x-ui/x-ui.sh
chmod +x /tmp/lucx-rel/x-ui/x-ui.sh
cd /tmp && curl -fsSL "https://github.com/XTLS/Xray-core/releases/download/v26.3.27/Xray-linux-64.zip" -o xray.zip && unzip -o xray.zip xray
cp /tmp/xray /tmp/lucx-rel/x-ui/bin/xray-linux-amd64 && chmod +x /tmp/lucx-rel/x-ui/bin/xray-linux-amd64
curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" -o /tmp/lucx-rel/x-ui/bin/geoip.dat
curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" -o /tmp/lucx-rel/x-ui/bin/geosite.dat
cp /home/lcp/3x-ui/install-lucx.sh /tmp/lucx-rel/
cd /tmp/lucx-rel && tar czf /tmp/lucx-ui-linux-amd64.tar.gz .
export PATH="/home/lcp/.local/gh/usr/bin:$PATH"
gh release create v0.1.5-pre-MVP --repo AlexeyLCP/lucx-ui --title "LucX-UI v0.1.5-pre-MVP" --prerelease --notes "..." /tmp/lucx-ui-linux-amd64.tar.gz
```
