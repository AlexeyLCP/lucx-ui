#!/bin/bash
# LucX-UI: deploy v3.6.0-lucx.48 to a test server with full rollback material.
# Upstream v3.6.0 adds DB migrations (migrateTgIDIndex, BackupSQLite) that run
# on first start against a live DB, so the DB is copied BEFORE the new binary
# ever touches it. Never overwrite an existing backup set.
set -uo pipefail

TAG="v3.6.0-lucx.48"
REPO="AlexeyLCP/lucx-ui"
STAMP=$(date -u +%Y%m%d-%H%M%S)
BK="/root/lucx-backup-${STAMP}"
DB=/etc/x-ui/x-ui.db

echo "===== 1. BACKUP -> ${BK} ====="
mkdir -p "$BK"
cp -a /usr/local/x-ui/x-ui "$BK/x-ui.bin"
"$BK/x-ui.bin" -v > "$BK/version.txt" 2>&1 || true
echo "binary: $(cat "$BK/version.txt")"

systemctl stop x-ui
sleep 2
systemctl is-active x-ui || echo "service stopped (expected)"

# DB must be copied with the service down: sqlite WAL/journal could otherwise
# be mid-transaction and the copy would need recovery.
cp -a "$DB" "$BK/x-ui.db"
for ext in -wal -shm; do
    [ -f "${DB}${ext}" ] && cp -a "${DB}${ext}" "$BK/x-ui.db${ext}"
done
sqlite3 "$BK/x-ui.db" "PRAGMA integrity_check;" > "$BK/db-integrity.txt" 2>&1
echo "db backup integrity: $(cat "$BK/db-integrity.txt")"
sqlite3 "$BK/x-ui.db" "SELECT id,protocol,port,enable FROM inbounds;" > "$BK/inbounds.txt" 2>&1
sqlite3 "$BK/x-ui.db" "SELECT id,inbound_id,email,up,down FROM client_traffics;" > "$BK/traffics.txt" 2>&1
sqlite3 "$BK/x-ui.db" "SELECT id,tag,enable FROM awg_outbounds;" > "$BK/awg_outbounds.txt" 2>&1
echo "--- saved traffic counters (rollback reference) ---"
cat "$BK/traffics.txt"

echo "===== 2. DOWNLOAD ${TAG} ====="
cd /tmp
rm -rf /tmp/lucx-new && mkdir /tmp/lucx-new && cd /tmp/lucx-new
URL="https://github.com/${REPO}/releases/download/${TAG}/x-ui-linux-amd64.tar.gz"
if ! curl -fL --retry 5 --retry-delay 3 --connect-timeout 15 -o pkg.tar.gz "$URL"; then
    echo "FATAL: download failed"; systemctl start x-ui; exit 1
fi
ls -la pkg.tar.gz
tar -xzf pkg.tar.gz
ls -la x-ui/
./x-ui/x-ui -v || { echo "FATAL: new binary does not run"; systemctl start x-ui; exit 1; }
NEWVER=$(./x-ui/x-ui -v)
echo "new binary reports: ${NEWVER}"
if [ "$NEWVER" != "3.6.0-lucx.48" ]; then
    echo "FATAL: version mismatch, expected 3.6.0-lucx.48"; systemctl start x-ui; exit 1
fi

echo "===== 3. INSTALL BINARY ====="
install -m 755 ./x-ui/x-ui /usr/local/x-ui/x-ui
# x-ui.sh is the management CLI; keep it in step with the binary.
if [ -f ./x-ui/x-ui.sh ]; then
    install -m 755 ./x-ui/x-ui.sh /usr/bin/x-ui
fi
/usr/local/x-ui/x-ui -v

echo "===== 4. START ====="
systemctl start x-ui
sleep 12
systemctl is-active x-ui || { echo "SERVICE FAILED TO START"; journalctl -u x-ui -n 60 --no-pager; exit 1; }

echo "===== 5. POST-START LOG (migrations!) ====="
journalctl -u x-ui --since "-2min" --no-pager | tail -60

echo "===== BACKUP DIR: ${BK} ====="
echo "rollback: systemctl stop x-ui; cp -a ${BK}/x-ui.bin /usr/local/x-ui/x-ui; cp -a ${BK}/x-ui.db ${DB}; systemctl start x-ui"
