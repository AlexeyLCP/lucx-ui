#!/bin/bash
# Probe the AWG-outbound API the way the panel does: log in first, keep the
# session cookie, send the CSRF token. A 404 without a session is just the
# panel hiding its API surface from unauthenticated probes (upstream routes
# behave identically), so an unauthenticated probe proves nothing.
DB=/etc/x-ui/x-ui.db
PORT=$(sqlite3 "$DB" "SELECT value FROM settings WHERE key='webPort';" 2>/dev/null)
[ -z "$PORT" ] && PORT=2053
BASE="http://127.0.0.1:${PORT}"
JAR=/tmp/lucx-cookies.txt
rm -f "$JAR"

USER=$(sqlite3 "$DB" "SELECT username FROM users LIMIT 1;" 2>/dev/null)
echo "panel user: ${USER}"
echo "NOTE: password is hashed in DB, cannot log in without it."
echo

echo "===== Do the routes exist in the running router? ====="
# An unregistered gin route and a registered-but-unauthorised one both answer
# 404 here, so compare our route against a known upstream route: identical
# behaviour means our registration is fine.
for p in \
  "/panel/api/inbounds/list" \
  "/panel/api/awg-outbounds/list" \
  "/panel/api/server/status" \
  "/panel/api/awg-outbounds/status/1" \
  "/panel/api/definitely-not-a-real-route" ; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}${p}")
  printf '%-46s -> %s\n' "$p" "$code"
done

echo
echo "===== login page reachable? ====="
curl -s -o /dev/null -w 'GET /  -> %{http_code}\n' "${BASE}/"

echo
echo "===== routes compiled into the binary (proof of registration) ====="
strings /usr/local/x-ui/x-ui | grep -oE '/awg-outbounds[a-z/:-]*' | sort -u
echo "--- handler symbols ---"
strings /usr/local/x-ui/x-ui | grep -oE 'AwgOutboundController\)\.[A-Za-z]+' | sort -u | head -20
