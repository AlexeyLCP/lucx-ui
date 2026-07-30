#!/bin/bash
DB=/etc/x-ui/x-ui.db
echo "===== web settings ====="
sqlite3 "$DB" "SELECT key,value FROM settings WHERE key LIKE 'web%';" 2>&1

BP=$(sqlite3 "$DB" "SELECT value FROM settings WHERE key='webBasePath';" 2>/dev/null)
PORT=$(sqlite3 "$DB" "SELECT value FROM settings WHERE key='webPort';" 2>/dev/null)
[ -z "$PORT" ] && PORT=2053
echo "basePath='${BP}' port='${PORT}'"

BASE="http://127.0.0.1:${PORT}${BP%/}"
echo "===== probes against ${BASE} ====="
probe() {
  code=$(curl -s -k -o /dev/null -w '%{http_code}' "$1")
  echo "$2 -> HTTP $code"
}
probe "${BASE}/" "GET /"
probe "${BASE}/panel/api/inbounds/list" "inbounds/list (upstream route, reference)"
probe "${BASE}/panel/api/awg-outbounds/list" "awg-outbounds/list"
probe "${BASE}/panel/api/awg-outbounds/status/1" "awg-outbounds/status/1"
probe "${BASE}/panel/api/inbounds/3/awgDiagnostics" "awgDiagnostics (our other endpoint)"

echo "===== is the route table registered at all? (grep binary) ====="
strings /usr/local/x-ui/x-ui | grep -c 'awg-outbounds' || echo "0 occurrences"
strings /usr/local/x-ui/x-ui | grep 'awg-outbounds' | head -5
