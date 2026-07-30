#!/bin/bash
DB=/etc/x-ui/x-ui.db
Q() { sqlite3 "$DB" "$1" 2>/dev/null; }

echo "===== VERSION ====="
/usr/local/x-ui/x-ui -v 2>/dev/null

echo "===== DB FILE ====="
ls -la "$DB"

echo "===== INBOUNDS ====="
Q "SELECT id, protocol, port, enable, remark FROM inbounds;"

echo "===== AWG INBOUND SETTINGS ====="
Q "SELECT id, substr(settings,1,400) FROM inbounds WHERE protocol='awg';"

echo "===== AWG_OUTBOUNDS ====="
Q "SELECT id, tag, enable, remark FROM awg_outbounds;"

echo "===== CLIENT_TRAFFICS (count + sample) ====="
Q "SELECT count(*) FROM client_traffics;"
Q "SELECT id, inbound_id, email, up, down, enable FROM client_traffics LIMIT 10;"

echo "===== CLIENTS: comment field populated? (PR24 regression target) ====="
Q "SELECT count(*) FROM clients;"
Q "SELECT id, email, comment FROM clients LIMIT 10;"

echo "===== EXISTING INDEXES (migrateTgIDIndex target) ====="
Q "SELECT name, tbl_name, sql FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_autoindex%';"

echo "===== tgId columns present? ====="
Q "PRAGMA table_info(clients);" | grep -i tg
Q "PRAGMA table_info(client_traffics);" | grep -i tg

echo "===== INTEGRITY ====="
Q "PRAGMA integrity_check;"

echo "===== AWG RUNTIME: awg show ====="
awg show 2>&1 | head -40

echo "===== INTERFACES ====="
ip -brief link show | grep -E 'awg|tun' || echo "none"

echo "===== IP RULES ====="
ip rule show | grep -i awg || echo "no awg rules"

echo "===== ROUTE TABLES 1000+ ====="
for t in $(ip rule show | grep -oP 'lookup \K1[0-9]{3}' | sort -u); do
  echo "-- table $t --"; ip route show table "$t"
done

echo "===== NAT RULES ====="
iptables -t nat -S 2>/dev/null | grep -i -E 'awg|MASQ' || echo "no nat awg rules"

echo "===== SERVICE ====="
systemctl is-active x-ui
systemctl show x-ui -p ExecStart --value

echo "===== RECENT AWG LOG ====="
journalctl -u x-ui -n 40 --no-pager | grep -i -E 'awg|error|panic' | tail -20 || echo "no awg log lines"

echo "===== DISK ====="
df -h / | tail -1
echo "===== OS ====="
cat /etc/os-release | grep -E 'PRETTY|VERSION_ID'
uname -r
echo "===== iptables present? (Pattern 1b) ====="
command -v iptables && iptables --version | head -1 || echo "IPTABLES MISSING"
echo "===== awg-quick present? ====="
command -v awg-quick || echo "AWG-QUICK MISSING"
