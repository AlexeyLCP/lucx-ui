#!/bin/bash
# LucX-UI Install Script
# https://github.com/AlexeyLCP/lucx-ui

red='\033[0;31m'; green='\033[0;32m'; blue='\033[0;34m'; yellow='\033[0;33m'; plain='\033[0m'
[[ $EUID -ne 0 ]] && echo -e "${red}Run as root${plain}" && exit 1

if [[ -f /etc/os-release ]]; then source /etc/os-release; release=$ID; fi
echo "OS: $release"

arch() { case "$(uname -m)" in x86_64|x64|amd64) echo 'amd64';; arm64|aarch64) echo 'arm64';; armv7*) echo 'armv7';; *) echo 'amd64';; esac; }
echo "Arch: $(arch)"

LUCX_HOME="${LUCX_HOME:-/usr/local/lucx-ui}"
XRAY_VER="26.3.27"
REPO="https://github.com/AlexeyLCP/lucx-ui"

echo -e "${green}Step 1/6: Installing dependencies...${plain}"
case "$release" in
    ubuntu|debian|armbian) apt-get update -qq && apt-get install -y -q curl tar unzip cron git ca-certificates ;;
    centos|fedora|rhel|almalinux) yum install -y -q curl tar unzip cronie git ca-certificates ;;
    arch|manjaro) pacman -Syu --noconfirm curl tar unzip cronie git ca-certificates ;;
    alpine) apk add curl tar unzip dcron git ca-certificates ;;
    *) apt-get update -qq && apt-get install -y -q curl tar unzip cron git ca-certificates ;;
esac

echo -e "${green}Step 2/6: Setting up directories...${plain}"
mkdir -p $LUCX_HOME/bin /etc/lucx-ui /var/log/lucx-ui
mkdir -p /etc/amnezia/amneziawg /etc/telemt /var/run/telemt /var/lib/telemt

echo -e "${green}Step 3/6: Downloading Xray...${plain}"
curl -4fLRo /tmp/xray.zip "https://github.com/XTLS/Xray-core/releases/download/v${XRAY_VER}/Xray-linux-64.zip" 2>/dev/null || \
  curl -4fLRo /tmp/xray.zip "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip"
unzip -o /tmp/xray.zip -d $LUCX_HOME/bin/ xray > /dev/null 2>&1
chmod +x $LUCX_HOME/bin/xray
ln -sf $LUCX_HOME/bin/xray $LUCX_HOME/bin/lucx-xray-linux-$(arch)
rm -f /tmp/xray.zip

echo -e "${green}Step 4/6: Downloading GeoIP...${plain}"
curl -4fLRo $LUCX_HOME/bin/geoip.dat "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" 2>/dev/null || true
curl -4fLRo $LUCX_HOME/bin/geosite.dat "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" 2>/dev/null || true

echo -e "${green}Step 5/6: Downloading LucX-UI...${plain}"
# Write config
cat > /etc/lucx-ui/config.json << EOF
{"log":{"loglevel":"info","access":"/var/log/lucx-ui/access.log","error":"/var/log/lucx-ui/error.log"},"db":{"type":"sqlite","path":"/etc/lucx-ui/x-ui.db"},"web":{"port":2053,"listen":"0.0.0.0","basePath":"/","certFile":"","keyFile":""},"xray":{"bin":"$LUCX_HOME/bin/lucx-xray-linux-$(arch)"}}
EOF

# Download binary
tag="${1:-v0.1.0}"
if [[ "$tag" == "latest" ]]; then
  tag=$(curl -sL "https://api.github.com/repos/AlexeyLCP/lucx-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  [[ -z "$tag" ]] && tag="v0.1.0"
fi
echo -e "${green}Version: ${tag}${plain}"

url="https://github.com/AlexeyLCP/lucx-ui/releases/download/${tag}/lucx-ui-linux-$(arch).tar.gz"
if curl -4fLRo /tmp/lucx-ui.tar.gz "$url" 2>/dev/null; then
  tar xzf /tmp/lucx-ui.tar.gz -C /tmp/
  cp /tmp/lucx-ui/lucx-ui $LUCX_HOME/ 2>/dev/null || true
  chmod +x $LUCX_HOME/lucx-ui 2>/dev/null || true
  rm -f /tmp/lucx-ui.tar.gz
fi

# If binary not found, build from source
if [[ ! -f $LUCX_HOME/lucx-ui ]]; then
  echo -e "${yellow}Building from source...${plain}"
  if ! command -v go > /dev/null 2>&1; then
    curl -4fLRo /tmp/go.tar.gz "https://go.dev/dl/go1.24.6.linux-$(arch).tar.gz"
    tar -C /usr/local -xzf /tmp/go.tar.gz; rm -f /tmp/go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    grep -q '/usr/local/go/bin' /etc/profile || echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
  fi
  if ! command -v node > /dev/null 2>&1; then
    curl -fsSL https://deb.nodesource.com/setup_22.x | bash - > /dev/null 2>&1
    apt-get install -y -q nodejs > /dev/null 2>&1 || true
  fi
  git clone "$REPO" /tmp/lucx-build 2>/dev/null
  if [[ -d /tmp/lucx-build ]]; then
    cd /tmp/lucx-build
    cd frontend && npm install --silent 2>/dev/null && npm run build 2>/dev/null && cd ..
    export PATH=$PATH:/usr/local/go/bin
    CGO_ENABLED=0 go build -o lucx-ui -ldflags="-s -w" . 2>/dev/null
    cp lucx-ui $LUCX_HOME/; chmod +x $LUCX_HOME/lucx-ui
    cd /; rm -rf /tmp/lucx-build
  fi
fi

echo -e "${green}Step 6/6: Starting service...${plain}"
cat > /etc/systemd/system/lucx-ui.service << EOF
[Unit]
Description=LucX-UI Panel
After=network.target
[Service]
Type=simple
ExecStart=$LUCX_HOME/lucx-ui
WorkingDirectory=$LUCX_HOME
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable lucx-ui > /dev/null 2>&1
systemctl restart lucx-ui 2>/dev/null || service lucx-ui restart 2>/dev/null
sleep 3

IP=$(curl -s https://api.ipify.org 2>/dev/null || echo 'YOUR_IP')
if systemctl is-active --quiet lucx-ui 2>/dev/null; then
  echo ""
  echo -e "┌───────────────────────────────────────────────────────┐"
  echo -e "│  ${green}LucX-UI Installation Complete!${plain}                      │"
  echo -e "│                                                       │"
  echo -e "│  Panel:   ${blue}http://${IP}:2053/${plain}"
  echo -e "│  Config:  ${blue}/etc/lucx-ui/config.json${plain}"
  echo -e "│  CLI:     ${blue}lucx-ui${plain} (start|stop|status|restart)"
  echo -e "│  Docs:    ${blue}https://github.com/AlexeyLCP/lucx-ui${plain}"
  echo -e "│                                                       │"
  echo -e "│  ${yellow}First login: admin / admin${plain}"
  echo -e "└───────────────────────────────────────────────────────┘"
  echo ""
  echo -e "${green}LucX engines:${plain}"
  echo -e "  AWG configs:    /etc/amnezia/amneziawg/"
  echo -e "  Telemt configs: /etc/telemt/"
  echo -e "  GeoIP:          $LUCX_HOME/bin/geoip.dat"
  echo -e "  GeoSite:        $LUCX_HOME/bin/geosite.dat"
else
  echo -e "${red}Service failed to start.${plain}"
  echo -e "${yellow}Check: journalctl -u lucx-ui${plain}"
  exit 1
fi
