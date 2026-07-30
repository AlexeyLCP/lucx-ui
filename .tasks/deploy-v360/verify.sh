#!/bin/bash
DB=/etc/x-ui/x-ui.db
Q() { sqlite3 "$DB" "$1" 2>&1; }
BK=$(ls -d /root/lucx-backup-* | tail -1)

echo "===== VERSION ====="
/usr/local/x-ui/x-ui -v

echo "===== DB INTEGRITY AFTER MIGRATIONS ====="
Q "PRAGMA integrity_check;"

echo "===== migrateTgIDIndex: did the tg_id index appear? ====="
Q "SELECT name, tbl_name, sql FROM sqlite_master WHERE type='index' AND (name LIKE '%tg%' OR sql LIKE '%tg_id%');"

echo "===== DATA PRESERVED: inbounds ====="
Q "SELECT id, protocol, port, enable FROM inbounds;"
echo "--- before (backup) ---"
cat "$BK/inbounds.txt"

echo "===== DATA PRESERVED: traffic counters (must be >= backup) ====="
Q "SELECT id, inbound_id, email, up, down FROM client_traffics;"
echo "--- before (backup) ---"
cat "$BK/traffics.txt"

echo "===== DATA PRESERVED: awg_outbounds ====="
Q "SELECT id, tag, enable, remark FROM awg_outbounds;"
echo "--- before (backup) ---"
cat "$BK/awg_outbounds.txt"

echo "===== PR24 REGRESSION: client comment survives a round-trip? ====="
Q "SELECT id, email, comment FROM clients;"
echo "--- comment inside AWG inbound settings JSON ---"
sqlite3 "$DB" "SELECT settings FROM inbounds WHERE protocol='awg';" 2>/dev/null | grep -o '\"comment\"[^,]*' | head -5

echo "===== AWG RUNTIME ====="
awg show 2>&1 | grep -E 'interface:|listening port:|peer:|latest handshake|transfer|allowed ips'

echo "===== INTERFACES ====="
ip -brief link show | grep -E 'awg|tun'

echo "===== ROUTED CHAIN (routeThroughXray) ====="
ip rule show | grep -i awg || echo "NO AWG IP RULE"
for t in $(ip rule show | grep -oP 'lookup \K1[0-9]{3}' | sort -u); do
  echo "-- table $t --"; ip route show table "$t"
done

echo "===== NAT ====="
iptables -t nat -S | grep -E 'MASQ' || echo "no masquerade"

echo "===== MSS CLAMP / FORWARD (reconcile-maintained) ====="
iptables -S FORWARD | grep -E 'awg|tun' || echo "no forward rules"

echo "===== XRAY CONFIG: TUN inbound + awg outbounds injected? ====="
grep -o '"protocol": *"tun"' /usr/local/x-ui/bin/config.json | head -3 || echo "no tun inbound in config.json"
grep -o '"interface": *"awgo-[0-9]*"' /usr/local/x-ui/bin/config.json | head -5 || echo "no awgo sockopt in config.json"
grep -o '"tag": *"[^"]*"' /usr/local/x-ui/bin/config.json | head -20

echo "===== API: AWG outbound endpoints alive? (session-less = expect 401/redirect, NOT 404) ====="
for ep in list status/1 ; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:2053/panel/api/awg-outbounds/${ep}")
  echo "GET /panel/api/awg-outbounds/${ep} -> HTTP ${code}"
done

echo "===== ERRORS SINCE START ====="
journalctl -u x-ui --since "-6min" --no-pager | grep -iE 'error|panic|fatal|failed' | grep -v 'ignore_errors' | tail -20 || echo "NO ERRORS"

echo "===== AWG RECONCILE HEALTH (should be no repeating failures) ====="
journalctl -u x-ui --since "-6min" --no-pager | grep -ci 'reconcile failed' || echo "0 reconcile failures"

echo "===== ONLINE / HANDSHAKE ====="
awg show awg3 latest-handshakes 2>&1
echo "--- transfer ---"
awg show awg3 transfer 2>&1

echo "===== SERVICE ====="
systemctl is-active x-ui
systemctl show x-ui -p NRestarts --value
