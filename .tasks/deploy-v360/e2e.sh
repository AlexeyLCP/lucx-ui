#!/bin/bash
# End-to-end AWG check: the previous script only proved the interface is UP.
# What matters is whether a packet entering awg3 actually reaches the internet
# through the Xray TUN (routeThroughXray mode), and whether the reconcile loop
# restores the chain after an Xray restart (the post-restart window fixed in
# lucx.34 via ensureAwgRouting).

echo "===== ip_forward ====="
cat /proc/sys/net/ipv4/ip_forward

echo "===== routed chain BEFORE restart ====="
ip rule show | grep -i awg
ip route show table 1003

echo "===== does the AWG subnet reach the internet via tun3? ====="
# Source the ping from the AWG gateway address so it follows the same
# policy-routing path a real peer's traffic takes (iif awg3 -> table 1003).
ping -c 3 -W 3 -I 10.8.0.1 1.1.1.1 2>&1 | tail -4
echo "--- HTTPS through the same source ---"
curl -s --interface 10.8.0.1 --max-time 12 -o /dev/null -w 'https via 10.8.0.1 -> %{http_code} in %{time_total}s\n' https://1.1.1.1 2>&1

echo "===== TUN inbound present in the live Xray config ====="
python3 - <<'PY' 2>/dev/null || grep -o '"protocol": *"tun"' /usr/local/x-ui/bin/config.json
import json
cfg = json.load(open('/usr/local/x-ui/bin/config.json'))
for ib in cfg.get('inbounds', []):
    if ib.get('protocol') == 'tun':
        print('tun inbound:', ib.get('tag'), 'settings:', json.dumps(ib.get('settings', {}))[:200])
        print('  sniffing:', json.dumps(ib.get('sniffing', {})))
for ob in cfg.get('outbounds', []):
    so = (ob.get('streamSettings') or {}).get('sockopt') or {}
    if so.get('interface'):
        print('awg outbound:', ob.get('tag'), '-> iface', so.get('interface'), 'sendThrough', ob.get('sendThrough'))
PY

echo
echo "===== RESTART XRAY (post-restart window test, lucx.34 fix) ====="
# Restarting the whole service is the harsher variant: tun3 is destroyed and
# must be recreated, then ensureAwgRouting must restore the route immediately
# rather than waiting up to 10s for the next reconcile tick.
systemctl restart x-ui
sleep 8
echo "--- immediately after restart (t+8s) ---"
ip -brief link show | grep -E 'awg3|tun3' || echo "MISSING IFACES"
ip rule show | grep -i awg || echo "NO IP RULE"
ip route show table 1003 || echo "NO ROUTE IN TABLE 1003"

echo "--- ping right after restart ---"
ping -c 2 -W 3 -I 10.8.0.1 1.1.1.1 2>&1 | tail -3

sleep 14
echo "--- after a full reconcile tick (t+22s) ---"
ip rule show | grep -i awg
ip route show table 1003
awg show 2>&1 | grep -E 'interface:|latest handshake|transfer'

echo "===== errors after restart ====="
journalctl -u x-ui --since "-2min" --no-pager | grep -iE 'error|panic|fatal|reconcile failed' | tail -15 || echo "NO ERRORS"

echo "===== service ====="
systemctl is-active x-ui
/usr/local/x-ui/x-ui -v
