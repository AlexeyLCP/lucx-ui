import{F as e,M as t,n,t as r,v as i}from"./_plugin-vue_export-helper-DFNIO2FK.js";import{B as a,E as o,F as s,G as c,Ht as l,K as u,Kt as d,R as f,U as p,V as m,W as h,X as g,Y as _,at as v,ct as y,dt as b,ft as x,it as S,j as C,k as w,m as T,nt as E,o as D,q as O,st as k,t as A,tt as j,v as M,xt as N,yt as P}from"./vendor-antd-Bn7Itjxz.js";import{A as F,B as ee,F as I,G as te,Y as L,d as R,k as ne,p as re,q as z,rt as B,t as V,u as H,x as ie}from"./vendor-icons-DGdjm5SI.js";import{t as U}from"./axios-init-Dpl98Vsr.js";import{t as ae}from"./AppSidebar-BkpZBvG5.js";function W(e){if(!e)return``;let t=e=>e.replace(/&/g,`&amp;`).replace(/</g,`&lt;`).replace(/>/g,`&gt;`),n=``,r=0;for(;r<e.length;){let i=e.indexOf(`<code>`,r);if(i===-1){n+=t(e.slice(r));break}n+=t(e.slice(r,i));let a=e.indexOf(`</code>`,i+6);if(a===-1){n+=t(e.slice(i));break}n+=`<code>`+t(e.slice(i+6,a))+`</code>`,r=a+7}return n}var G=[{id:`authentication`,title:`Authentication`,description:`Two authentication modes are supported. UI sessions use a cookie set by the login endpoint. Programmatic clients (bots, scripts, remote panels) authenticate with a Bearer token taken from Settings → Security → API Token. Both work for every endpoint under /panel/api/*.`,endpoints:[{method:`POST`,path:`/login`,summary:`Authenticate with username + password and receive a session cookie. Required before any cookie-based API call.`,params:[{name:`username`,in:`body`,type:`string`,desc:`Panel admin username.`},{name:`password`,in:`body`,type:`string`,desc:`Panel admin password.`},{name:`twoFactorCode`,in:`body`,type:`string`,desc:`OTP code when 2FA is enabled. Omit otherwise.`}],body:`{
  "username": "admin",
  "password": "admin",
  "twoFactorCode": "123456"
}`,response:`{
  "success": true,
  "msg": "Logged in successfully"
}`,errorResponse:`{
  "success": false,
  "msg": "Wrong username or password"
}`},{method:`POST`,path:`/logout`,summary:`Clear the session cookie. Requires the CSRF header for browser sessions.`,response:`{
  "success": true
}`},{method:`GET`,path:`/csrf-token`,summary:`Mint a CSRF token for the current session. The SPA replays it in the X-CSRF-Token header on unsafe requests. Bearer-token callers can skip this — the middleware short-circuits CSRF for authenticated API requests.`,response:`{
  "success": true,
  "obj": "csrf-token-string"
}`},{method:`POST`,path:`/getTwoFactorEnable`,summary:`Returns whether 2FA is enabled on the panel — used by the login page to decide whether to show the OTP field.`,response:`{
  "success": true,
  "obj": false
}`}]},{id:`inbounds`,title:`Inbounds`,description:`Manage inbound configurations and their clients. All endpoints live under /panel/api/inbounds and require a logged-in session or Bearer token. Link-generating endpoints honour forwarded headers only when the request comes from a configured trusted proxy.`,endpoints:[{method:`GET`,path:`/panel/api/inbounds/list`,summary:`List every inbound owned by the authenticated user, including each inbound’s clientStats traffic counters.`,response:`{
  "success": true,
  "obj": [
    {
      "id": 1,
      "userId": 1,
      "up": 0,
      "down": 0,
      "total": 0,
      "remark": "VLESS-443",
      "enable": true,
      "expiryTime": 0,
      "listen": "",
      "port": 443,
      "protocol": "vless",
      "settings": "{\\"clients\\":[...]}",
      "streamSettings": "{...}",
      "tag": "inbound-443",
      "sniffing": "{...}",
      "clientStats": [...]
    }
  ]
}`},{method:`GET`,path:`/panel/api/inbounds/get/:id`,summary:`Fetch a single inbound by numeric ID.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`}]},{method:`GET`,path:`/panel/api/inbounds/getClientTraffics/:email`,summary:`Traffic counters for a client identified by email.`,params:[{name:`email`,in:`path`,type:`string`,desc:`Client email (unique across the panel).`}],response:`{
  "success": true,
  "obj": {
    "email": "user1",
    "up": 1048576,
    "down": 2097152,
    "total": 10737418240,
    "expiryTime": 1735689600000
  }
}`},{method:`GET`,path:`/panel/api/inbounds/getClientTrafficsById/:id`,summary:`Traffic counters for a client identified by its UUID/password.`,params:[{name:`id`,in:`path`,type:`string`,desc:`Client subId / UUID.`}],response:`{
  "success": true,
  "obj": {
    "email": "user1",
    "up": 1048576,
    "down": 2097152,
    "total": 10737418240,
    "expiryTime": 1735689600000
  }
}`},{method:`POST`,path:`/panel/api/inbounds/add`,summary:`Create a new inbound. Send the full inbound payload (protocol, port, settings JSON, streamSettings JSON, sniffing JSON, remark, expiryTime, total, enable).`,body:`{
  "enable": true,
  "remark": "VLESS-443",
  "listen": "",
  "port": 443,
  "protocol": "vless",
  "expiryTime": 0,
  "total": 0,
  "settings": "{\\"clients\\":[{\\"id\\":\\"...\\",\\"email\\":\\"user1\\"}],\\"decryption\\":\\"none\\",\\"fallbacks\\":[]}",
  "streamSettings": "{\\"network\\":\\"tcp\\",\\"security\\":\\"reality\\",\\"realitySettings\\":{...}}",
  "sniffing": "{\\"enabled\\":true,\\"destOverride\\":[\\"http\\",\\"tls\\"]}"
}`,errorResponse:`{
  "success": false,
  "msg": "Port 443 is already in use"
}`},{method:`POST`,path:`/panel/api/inbounds/del/:id`,summary:`Delete an inbound by ID. Also removes its associated client stats rows.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`}]},{method:`POST`,path:`/panel/api/inbounds/update/:id`,summary:`Replace an inbound’s configuration. Body shape mirrors /add. Heavy on inbounds with thousands of clients — prefer /setEnable for enable-only flips.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`}]},{method:`POST`,path:`/panel/api/inbounds/setEnable/:id`,summary:`Toggle only the enable flag without serialising the whole settings JSON. Recommended for UI switches on large inbounds.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`}],body:`{
  "enable": false
}`},{method:`POST`,path:`/panel/api/inbounds/clientIps/:email`,summary:`List source IPs that have connected with the given client’s credentials. Returns an array of "ip (timestamp)" strings.`,params:[{name:`email`,in:`path`,type:`string`,desc:`Client email.`}]},{method:`POST`,path:`/panel/api/inbounds/clearClientIps/:email`,summary:`Reset the recorded IP list for a client.`,params:[{name:`email`,in:`path`,type:`string`,desc:`Client email.`}]},{method:`POST`,path:`/panel/api/inbounds/addClient`,summary:`Add one or more clients to an existing inbound. The settings field is the JSON-encoded settings.clients array of the target inbound.`,body:`{
  "id": 1,
  "settings": "{\\"clients\\":[{\\"id\\":\\"uuid-here\\",\\"email\\":\\"newuser\\",\\"limitIp\\":0,\\"totalGB\\":0,\\"expiryTime\\":0,\\"enable\\":true,\\"flow\\":\\"\\"}]}"
}`},{method:`POST`,path:`/panel/api/inbounds/:id/copyClients`,summary:`Copy selected clients from one inbound into another. Useful for duplicating user lists across protocols.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Target inbound ID.`},{name:`sourceInboundId`,in:`body`,type:`number`,desc:`Inbound ID to read clients from.`},{name:`clientEmails`,in:`body`,type:`string[]`,desc:`Emails of clients to copy. Empty means all clients.`},{name:`flow`,in:`body`,type:`string`,desc:`Override the flow field on copied clients (e.g. "xtls-rprx-vision"). Empty to keep source flow.`}]},{method:`POST`,path:`/panel/api/inbounds/:id/delClient/:clientId`,summary:`Delete a client by its UUID/password from a specific inbound.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`},{name:`clientId`,in:`path`,type:`string`,desc:`Client UUID / password.`}]},{method:`POST`,path:`/panel/api/inbounds/updateClient/:clientId`,summary:`Update a single client without rewriting the whole settings JSON. Send the target inbound payload with the new client values.`,params:[{name:`clientId`,in:`path`,type:`string`,desc:`Client UUID / password.`}],body:`{
  "id": 1,
  "settings": "{\\"clients\\":[{\\"id\\":\\"uuid-here\\",\\"email\\":\\"user1\\",\\"limitIp\\":2,\\"totalGB\\":10737418240,\\"expiryTime\\":1735689600000,\\"enable\\":true}]}"
}`},{method:`POST`,path:`/panel/api/inbounds/:id/resetTraffic`,summary:`Zero out upload + download counters for a single inbound. Does not touch per-client counters.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`}]},{method:`POST`,path:`/panel/api/inbounds/:id/resetClientTraffic/:email`,summary:`Zero out upload + download counters for one client.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`},{name:`email`,in:`path`,type:`string`,desc:`Client email.`}]},{method:`POST`,path:`/panel/api/inbounds/resetAllTraffics`,summary:`Reset upload + download counters on every inbound. Destructive — accounting history is lost.`},{method:`POST`,path:`/panel/api/inbounds/resetAllClientTraffics/:id`,summary:`Reset traffic for every client in one inbound.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`}]},{method:`POST`,path:`/panel/api/inbounds/delDepletedClients/:id`,summary:`Delete clients in this inbound whose traffic cap or expiry has elapsed. Pass id=-1 to sweep every inbound.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID, or -1 for all inbounds.`}]},{method:`POST`,path:`/panel/api/inbounds/import`,summary:`Bulk-import an inbound from a JSON blob (e.g. one exported via the UI). The body uses form encoding with a single "data" field.`,params:[{name:`data`,in:`body (form)`,type:`string`,desc:`JSON-encoded inbound payload.`}]},{method:`POST`,path:`/panel/api/inbounds/onlines`,summary:`List the emails of currently connected clients (last seen within the heartbeat window).`,response:`{
  "success": true,
  "obj": ["user1", "user2"]
}`},{method:`POST`,path:`/panel/api/inbounds/lastOnline`,summary:`Map of client email → last-seen unix timestamp.`,response:`{
  "success": true,
  "obj": [
    { "email": "user1", "lastOnline": 1700000000 },
    { "email": "user2", "lastOnline": 1699999000 }
  ]
}`},{method:`GET`,path:`/panel/api/inbounds/getSubLinks/:subId`,summary:`Return every protocol URL (vless://, vmess://, trojan://, ss://, hysteria://, hy2://) for clients matching the subscription ID. Same result set as /sub/<subId>, but as a JSON array — no base64. When an inbound has streamSettings.externalProxy set, one URL is emitted per external proxy. Empty array when the subId has no enabled clients.`,params:[{name:`subId`,in:`path`,type:`string`,desc:`Subscription ID, taken from the client's subId field.`}],response:`{
  "success": true,
  "obj": [
    "vless://uuid@host:443?security=reality&...#user1",
    "vmess://eyJ2IjoyLC..."
  ]
}`},{method:`GET`,path:`/panel/api/inbounds/getClientLinks/:id/:email`,summary:`Return the URL(s) for one client on one inbound — the same string the Copy URL button copies in the panel UI. Supported protocols: vmess, vless, trojan, shadowsocks, hysteria, hysteria2. If streamSettings.externalProxy is set, returns one URL per external proxy. Protocols without a URL form (socks, http, mixed, wireguard, dokodemo, tunnel) return an empty array.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`},{name:`email`,in:`path`,type:`string`,desc:`Client email.`}],response:`{
  "success": true,
  "obj": [
    "vless://uuid@host:443?...#user1"
  ]
}`},{method:`POST`,path:`/panel/api/inbounds/updateClientTraffic/:email`,summary:`Manually adjust a client’s upload + download counters. Useful for migrations from external accounting systems.`,params:[{name:`email`,in:`path`,type:`string`,desc:`Client email.`}],body:`{
  "upload": 1073741824,
  "download": 5368709120
}`},{method:`POST`,path:`/panel/api/inbounds/:id/delClientByEmail/:email`,summary:`Delete a client identified by email rather than UUID.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Inbound ID.`},{name:`email`,in:`path`,type:`string`,desc:`Client email.`}]}]},{id:`server`,title:`Server`,description:`System status, log retrieval, certificate generators, Xray binary management, and backup/restore. All under /panel/api/server.`,endpoints:[{method:`GET`,path:`/panel/api/server/status`,summary:`Real-time machine snapshot: CPU, memory, swap, disk, network IO, load averages, open connections, Xray state. Cached and refreshed every 2 seconds in the background.`,response:`{
  "success": true,
  "obj": {
    "cpu": 12.5,
    "mem": { "current": 2147483648, "total": 8589934592 },
    "swap": { "current": 0, "total": 4294967296 },
    "disk": { "current": 53687091200, "total": 268435456000 },
    "netIO": { "up": 1073741824, "down": 2147483648 },
    "xray": { "state": "running", "version": "v25.10.31" },
    "tcpCount": 42,
    "load": { "load1": 0.5, "load5": 0.3, "load15": 0.2 }
  }
}`},{method:`GET`,path:`/panel/api/server/cpuHistory/:bucket`,summary:`Legacy: aggregated CPU history. Use /history/cpu/:bucket instead — same data with a uniform {t, v} shape.`,params:[{name:`bucket`,in:`path`,type:`number`,desc:`Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.`}]},{method:`GET`,path:`/panel/api/server/history/:metric/:bucket`,summary:`Aggregated time-series for one metric. Returns an array of {t, v} samples covering the last ~6 hours.`,params:[{name:`metric`,in:`path`,type:`string`,desc:`cpu | mem | netUp | netDown | online | load1 | load5 | load15.`},{name:`bucket`,in:`path`,type:`number`,desc:`Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.`}],response:`{
  "success": true,
  "obj": [
    { "t": 1700000000, "v": 12.5 },
    { "t": 1700000002, "v": 13.1 }
  ]
}`},{method:`GET`,path:`/panel/api/server/xrayMetricsState`,summary:"Xray runtime metrics state — whether the xray config has a `metrics` block, which expvar keys are flowing, and the current snapshot values for each. Returns an empty state when metrics are not configured."},{method:`GET`,path:`/panel/api/server/xrayMetricsHistory/:metric/:bucket`,summary:`Time-series history for one Xray runtime metric over the last ~6 hours. Same {t, v} shape as /history/:metric/:bucket.`,params:[{name:`metric`,in:`path`,type:`string`,desc:`xrAlloc | xrSys | xrHeapObjects | xrNumGC | xrPauseNs.`},{name:`bucket`,in:`path`,type:`number`,desc:`Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.`}]},{method:`GET`,path:`/panel/api/server/xrayObservatory`,summary:`Latest snapshot from the Xray observatory — per-outbound latency, health status, and last-probe time. Only populated when the Xray config has an observatory configured.`},{method:`GET`,path:`/panel/api/server/xrayObservatoryHistory/:tag/:bucket`,summary:`Time-series of observatory probe results for one outbound tag. Same {t, v} shape as the other history endpoints.`,params:[{name:`tag`,in:`path`,type:`string`,desc:`Outbound tag from the observatory config.`},{name:`bucket`,in:`path`,type:`number`,desc:`Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.`}]},{method:`GET`,path:`/panel/api/server/getXrayVersion`,summary:`List Xray binary versions available for install on this host.`,response:`{
  "success": true,
  "obj": ["v25.10.31", "v25.9.15", "v25.8.1"]
}`},{method:`GET`,path:`/panel/api/server/getPanelUpdateInfo`,summary:`Check whether a newer 3x-ui release is available on GitHub.`},{method:`GET`,path:`/panel/api/server/getConfigJson`,summary:`Return the assembled Xray config that’s currently running on this host.`,response:`{
  "success": true,
  "obj": {
    "log": { "loglevel": "warning" },
    "inbounds": [...],
    "outbounds": [...],
    "routing": { "rules": [...] }
  }
}`},{method:`GET`,path:`/panel/api/server/getDb`,summary:`Stream the SQLite database file as an attachment. Use as a manual backup.`},{method:`GET`,path:`/panel/api/server/getNewUUID`,summary:`Generate a fresh UUID v4. Convenience helper for client IDs.`,response:`{
  "success": true,
  "obj": "550e8400-e29b-41d4-a716-446655440000"
}`},{method:`GET`,path:`/panel/api/server/getNewX25519Cert`,summary:`Generate a new X25519 keypair for Reality.`,response:`{
  "success": true,
  "obj": {
    "privateKey": "uN9qLfV3zH8w...",
    "publicKey": "5v8xPqR2sM7k..."
  }
}`},{method:`GET`,path:`/panel/api/server/getNewmldsa65`,summary:`Generate a new ML-DSA-65 keypair (post-quantum signature). Returns {privateKey, publicKey, seed}.`,response:`{
  "success": true,
  "obj": {
    "privateKey": "mdsa65priv...",
    "publicKey": "mdsa65pub...",
    "seed": "random-seed..."
  }
}`},{method:`GET`,path:`/panel/api/server/getNewmlkem768`,summary:`Generate a new ML-KEM-768 keypair (post-quantum KEM). Returns {clientKey, serverKey}.`,response:`{
  "success": true,
  "obj": {
    "clientKey": "mlkem768-client...",
    "serverKey": "mlkem768-server..."
  }
}`},{method:`GET`,path:`/panel/api/server/getNewVlessEnc`,summary:`Generate VLESS encryption auth options. Returns an auths array each with id, label, encryption, and decryption fields.`,response:`{
  "success": true,
  "obj": {
    "auths": [
      { "id": 0, "label": "Auth #0", "encryption": "aes-256-gcm", "decryption": "" }
    ]
  }
}`},{method:`POST`,path:`/panel/api/server/stopXrayService`,summary:`Stop the Xray binary. All proxies go offline immediately.`,errorResponse:`{
  "success": false,
  "msg": "Xray is not running"
}`},{method:`POST`,path:`/panel/api/server/restartXrayService`,summary:`Reload Xray with the current config. Typically required after structural inbound or routing changes.`,errorResponse:`{
  "success": false,
  "msg": "Xray config is invalid: ..."
}`},{method:`POST`,path:`/panel/api/server/installXray/:version`,summary:`Download and install the specified Xray version. Pass "latest" for the newest release.`,params:[{name:`version`,in:`path`,type:`string`,desc:`Xray tag (e.g. v25.10.31) or "latest".`}]},{method:`POST`,path:`/panel/api/server/updatePanel`,summary:`Self-update the panel to the latest version. The server restarts on success.`},{method:`POST`,path:`/panel/api/server/updateGeofile`,summary:`Refresh the default GeoIP / GeoSite data files. Body can include a fileName, or use the /:fileName variant.`,params:[{name:`fileName`,in:`body (form)`,type:`string`,desc:`Filename to update (e.g. geoip.dat, geosite.dat). Omit to update all defaults.`}],body:`fileName=geoip.dat`},{method:`POST`,path:`/panel/api/server/updateGeofile/:fileName`,summary:`Refresh a single Geo file by filename (e.g. geoip.dat, geosite.dat).`,params:[{name:`fileName`,in:`path`,type:`string`,desc:`Filename of the data file to refresh.`}]},{method:`POST`,path:`/panel/api/server/logs/:count`,summary:`Return the last N lines of the panel’s own log.`,params:[{name:`count`,in:`path`,type:`number`,desc:`Number of trailing log lines.`}],body:`{
  "level": "info",
  "syslog": false
}`,response:`{
  "success": true,
  "obj": "2025/01/01 12:00:00 [INFO] Server started\\n2025/01/01 12:00:01 [INFO] Xray is running"
}`},{method:`POST`,path:`/panel/api/server/xraylogs/:count`,summary:`Return the last N lines of the Xray process log.`,params:[{name:`count`,in:`path`,type:`number`,desc:`Number of trailing log lines.`},{name:`filter`,in:`body (form)`,type:`string`,desc:`Keyword filter — only lines containing this string.`},{name:`showDirect`,in:`body (form)`,type:`string`,desc:`"true" to include direct (freedom) traffic lines.`},{name:`showBlocked`,in:`body (form)`,type:`string`,desc:`"true" to include blocked (blackhole) traffic lines.`},{name:`showProxy`,in:`body (form)`,type:`string`,desc:`"true" to include proxy traffic lines.`}],body:`filter=error&showDirect=false&showBlocked=true&showProxy=true`,response:`{
  "success": true,
  "obj": "2025/01/01 12:00:00 rejected  vless  proxy  example.com  reason: no valid user\\n2025/01/01 12:00:01 direct  freedom  ok"
}`},{method:`POST`,path:`/panel/api/server/importDB`,summary:`Restore the panel DB from an uploaded SQLite file (multipart form, field name "db"). The panel restarts after restore. Destructive.`,params:[{name:`db`,in:`body (multipart)`,type:`file`,desc:`SQLite database file to upload.`}]},{method:`POST`,path:`/panel/api/server/getNewEchCert`,summary:`Generate a new ECH (Encrypted Client Hello) keypair and config list for the given SNI.`,params:[{name:`sni`,in:`body (form)`,type:`string`,desc:`Server Name Indication to generate the ECH config for.`}],body:`sni=example.com`,response:`{
  "success": true,
  "obj": {
    "echKeySet": "...",
    "echServerKeys": [...],
    "echConfigList": "..."
  }
}`}]},{id:`nodes`,title:`Nodes`,description:`Manage remote 3x-ui panels acting as nodes for a central panel. All endpoints under /panel/api/nodes.`,endpoints:[{method:`GET`,path:`/panel/api/nodes/list`,summary:`List every configured node with its connection details, health, and last heartbeat patch.`,response:`{
  "success": true,
  "obj": [
    {
      "id": 1,
      "name": "de-fra-1",
      "scheme": "https",
      "host": "node1.example.com",
      "port": 2053,
      "status": "online",
      "cpu": 23.5,
      "mem": 45.1
    }
  ]
}`},{method:`GET`,path:`/panel/api/nodes/get/:id`,summary:`Fetch a single node by ID.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Node ID.`}]},{method:`POST`,path:`/panel/api/nodes/add`,summary:`Register a new remote node. Provide its URL, apiToken, and optional label/notes.`,body:`{
  "name": "de-fra-1",
  "scheme": "https",
  "host": "node1.example.com",
  "port": 2053,
  "basePath": "/",
  "apiToken": "abcdef..."
}`},{method:`POST`,path:`/panel/api/nodes/update/:id`,summary:`Replace a node’s connection details. Same body shape as /add.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Node ID.`}],body:`{
  "name": "de-fra-1",
  "scheme": "https",
  "host": "node1.example.com",
  "port": 2053,
  "basePath": "/",
  "apiToken": "abcdef..."
}`},{method:`POST`,path:`/panel/api/nodes/del/:id`,summary:`Delete a node. Inbounds bound to it are not auto-migrated.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Node ID.`}]},{method:`POST`,path:`/panel/api/nodes/setEnable/:id`,summary:`Pause or resume traffic sync with this node.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Node ID.`}],body:`{
  "enable": true
}`},{method:`POST`,path:`/panel/api/nodes/test`,summary:`Probe a node without saving it. Uses the body as connection details and returns whether the handshake succeeds.`,body:`{
  "scheme": "https",
  "host": "node1.example.com",
  "port": 2053,
  "basePath": "/",
  "apiToken": "abcdef..."
}`,response:`{
  "success": true,
  "obj": {
    "status": "online",
    "cpu": 12.5,
    "mem": 45.2
  }
}`},{method:`POST`,path:`/panel/api/nodes/probe/:id`,summary:`Probe an existing node, updating its cached health state.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Node ID.`}]},{method:`GET`,path:`/panel/api/nodes/history/:id/:metric/:bucket`,summary:`Aggregated metric history for a node — same shape as /server/history, scoped to one node.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Node ID.`},{name:`metric`,in:`path`,type:`string`,desc:`cpu | mem.`},{name:`bucket`,in:`path`,type:`number`,desc:`Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.`}]}]},{id:`custom-geo`,title:`Custom Geo`,description:`Manage user-supplied GeoIP / GeoSite source files. All endpoints under /panel/api/custom-geo.`,endpoints:[{method:`GET`,path:`/panel/api/custom-geo/list`,summary:`List configured custom geo sources with their type, alias, URL, status, and last-download timestamp.`},{method:`GET`,path:`/panel/api/custom-geo/aliases`,summary:`List geo aliases currently usable in routing rules — both built-in defaults and the user-configured ones.`},{method:`POST`,path:`/panel/api/custom-geo/add`,summary:`Register a custom geo source. Alias is auto-normalised; URL must point to a .dat / .json blob.`,body:`{
  "type": "geoip",
  "alias": "myips",
  "url": "https://example.com/geo/my.dat"
}`},{method:`POST`,path:`/panel/api/custom-geo/update/:id`,summary:`Replace a custom geo source. Same body shape as /add.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Custom geo source ID.`}]},{method:`POST`,path:`/panel/api/custom-geo/delete/:id`,summary:`Remove a custom geo source and its cached file.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Custom geo source ID.`}]},{method:`POST`,path:`/panel/api/custom-geo/download/:id`,summary:`Re-download one custom geo source on demand.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Custom geo source ID.`}]},{method:`POST`,path:`/panel/api/custom-geo/update-all`,summary:`Re-download every configured custom geo source. Errors are reported per-source in the response.`}]},{id:`backup`,title:`Backup`,description:`Operations that interact with the configured Telegram bot.`,endpoints:[{method:`POST`,path:`/panel/api/backuptotgbot`,summary:`Send a fresh DB backup to every Telegram chat configured as an admin recipient. No body, no params.`}]},{id:`settings`,title:`Settings`,description:`Panel configuration and user credentials. All endpoints live under /panel/setting and require a logged-in session or Bearer token.`,endpoints:[{method:`POST`,path:`/panel/setting/all`,summary:`Return every panel setting: web server, Telegram bot, subscription, security, LDAP. The full JSON blob that the Settings page edits.`,response:`{
  "success": true,
  "obj": {
    "webPort": 2053,
    "webCertFile": "",
    "webKeyFile": "",
    "webBasePath": "/",
    "subPort": 10882,
    "subPath": "/sub/",
    "tgBotEnable": false,
    "tgBotToken": "",
    ...
  }
}`},{method:`POST`,path:`/panel/setting/defaultSettings`,summary:`Return the computed default settings based on the request host. Useful to preview what a fresh install would use.`},{method:`POST`,path:`/panel/setting/update`,summary:`Persist every setting at once. The body mirrors the shape returned by /all. Invalid values (bad ports, missing cert pairs, etc.) are rejected before write.`,body:`{
  "webPort": 2053,
  "webBasePath": "/",
  "subPort": 10882,
  "subPath": "/sub/",
  "tgBotEnable": false,
  ...
}`},{method:`POST`,path:`/panel/setting/updateUser`,summary:`Change the panel admin username and password. Requires the current credentials for verification. The session is refreshed with the new values on success.`,params:[{name:`oldUsername`,in:`body`,type:`string`,desc:`Current admin username.`},{name:`oldPassword`,in:`body`,type:`string`,desc:`Current admin password.`},{name:`newUsername`,in:`body`,type:`string`,desc:`Desired new username.`},{name:`newPassword`,in:`body`,type:`string`,desc:`Desired new password.`}],body:`{
  "oldUsername": "admin",
  "oldPassword": "admin",
  "newUsername": "newadmin",
  "newPassword": "newpass"
}`},{method:`POST`,path:`/panel/setting/restartPanel`,summary:`Restart the entire 3x-ui process after a 3-second grace period. The connection drops immediately; the panel comes back online ~5-10 seconds later.`},{method:`GET`,path:`/panel/setting/getDefaultJsonConfig`,summary:`Return the built-in default Xray JSON config template that ships with this panel version.`}]},{id:`api-tokens`,title:`API Tokens`,description:`Manage Bearer tokens used for programmatic auth (bots, central panels acting on this node, CI). Each token has a unique name and an enabled flag — disable to revoke without deleting, delete to revoke permanently. Tokens are stored plaintext so the SPA can show them on demand. Send one as <code>Authorization: Bearer &lt;token&gt;</code> on any /panel/api/* request.`,endpoints:[{method:`GET`,path:`/panel/setting/apiTokens`,summary:`List every API token, enabled or not.`,response:`{
  "success": true,
  "obj": [
    {
      "id": 1,
      "name": "default",
      "token": "abcdef-12345-...",
      "enabled": true,
      "createdAt": 1736000000
    }
  ]
}`},{method:`POST`,path:`/panel/setting/apiTokens/create`,summary:`Mint a new API token. Name must be unique and 1-64 characters; the token string is server-generated.`,params:[{name:`name`,in:`body`,type:`string`,desc:`Human-readable label, e.g. "central-panel-a".`}],body:`{
  "name": "central-panel-a"
}`,response:`{
  "success": true,
  "obj": {
    "id": 2,
    "name": "central-panel-a",
    "token": "new-token-string",
    "enabled": true,
    "createdAt": 1736000000
  }
}`,errorResponse:`{
  "success": false,
  "msg": "a token with that name already exists"
}`},{method:`POST`,path:`/panel/setting/apiTokens/delete/:id`,summary:`Permanently delete a token. Any caller using it stops authenticating immediately.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Token row ID.`}],response:`{
  "success": true
}`},{method:`POST`,path:`/panel/setting/apiTokens/setEnabled/:id`,summary:`Toggle a token enabled/disabled without deleting it. Disabled tokens are rejected by checkAPIAuth on the next request.`,params:[{name:`id`,in:`path`,type:`number`,desc:`Token row ID.`},{name:`enabled`,in:`body`,type:`boolean`,desc:`New enabled state.`}],body:`{
  "enabled": false
}`,response:`{
  "success": true
}`}]},{id:`xray-settings`,title:`Xray Settings`,description:`Xray configuration template, outbound management, Warp/Nord integration, and config testing. All endpoints under /panel/xray.`,endpoints:[{method:`POST`,path:`/panel/xray/`,summary:`Return the Xray config template (JSON string), available inbound tags, client reverse tags, and the configured outbound test URL in one response.`,response:`{
  "success": true,
  "obj": {
    "xraySetting": "{...raw xray config...}",
    "inboundTags": "[\\"inbound-443\\"]",
    "clientReverseTags": "[]",
    "outboundTestUrl": "https://www.google.com/generate_204"
  }
}`},{method:`GET`,path:`/panel/xray/getDefaultJsonConfig`,summary:`Return the built-in default Xray config shipped with the panel (identical to /panel/setting/getDefaultJsonConfig).`},{method:`GET`,path:`/panel/xray/getOutboundsTraffic`,summary:`Return traffic statistics for every outbound. Each outbound shows up/down/total counters.`},{method:`GET`,path:`/panel/xray/getXrayResult`,summary:`Return the most recent Xray process stdout/stderr output. Useful to check for startup errors or runtime warnings.`},{method:`POST`,path:`/panel/xray/update`,summary:`Save the Xray JSON config template and optionally the outbound test URL. Both are sent as form fields.`,params:[{name:`xraySetting`,in:`body (form)`,type:`string`,desc:`Full Xray JSON config template.`},{name:`outboundTestUrl`,in:`body (form)`,type:`string`,desc:`URL used for outbound reachability tests. Defaults to https://www.google.com/generate_204.`}]},{method:`POST`,path:`/panel/xray/warp/:action`,summary:`Manage Cloudflare Warp integration. The action parameter selects the operation.`,params:[{name:`action`,in:`path`,type:`string`,desc:`data — return Warp stats (quota, remaining). del — delete Warp data. config — return current Warp config. reg — register a new Warp endpoint (sends privateKey, publicKey). license — set a Warp+ license key (sends license).`},{name:`privateKey`,in:`body (form)`,type:`string`,desc:`Required when action=reg.`},{name:`publicKey`,in:`body (form)`,type:`string`,desc:`Required when action=reg.`},{name:`license`,in:`body (form)`,type:`string`,desc:`Required when action=license.`}]},{method:`POST`,path:`/panel/xray/nord/:action`,summary:`Manage NordVPN integration. The action parameter selects the operation.`,params:[{name:`action`,in:`path`,type:`string`,desc:`countries — list available countries. servers — list servers in a country (sends countryId). reg — get NordVPN credentials (sends token). setKey — store NordVPN API key (sends key). data — return current NordVPN connection data. del — delete NordVPN data.`},{name:`countryId`,in:`body (form)`,type:`string`,desc:`Required when action=servers.`},{name:`token`,in:`body (form)`,type:`string`,desc:`Required when action=reg.`},{name:`key`,in:`body (form)`,type:`string`,desc:`Required when action=setKey.`}]},{method:`POST`,path:`/panel/xray/resetOutboundsTraffic`,summary:`Reset traffic counters for a specific outbound by tag.`,params:[{name:`tag`,in:`body (form)`,type:`string`,desc:`Outbound tag to reset (e.g. "proxy", "direct").`}],body:`tag=proxy`},{method:`POST`,path:`/panel/xray/testOutbound`,summary:`Test an outbound configuration. Sends the outbound JSON (required), optionally all outbounds (to resolve sockopt.dialerProxy dependencies), and a mode flag.`,params:[{name:`outbound`,in:`body (form)`,type:`string`,desc:`JSON-encoded single outbound to test (required).`},{name:`allOutbounds`,in:`body (form)`,type:`string`,desc:`JSON array of all outbounds — used to resolve dialerProxy chains.`},{name:`mode`,in:`body (form)`,type:`string`,desc:`"tcp" for a fast dial-only probe (parallel-safe). Default/empty uses a full HTTP probe through a temp xray instance.`}],body:`outbound={"protocol":"freedom","settings":{}}&mode=tcp`}]},{id:`subscription`,title:`Subscription Server`,description:`A separate HTTP/HTTPS server that serves proxy subscription links (standard, JSON, and Clash) to clients. The server listens on its own port (default 10882) and is configured in Settings → Subscription. Paths are configurable; defaults are shown below. All subscription endpoints set response headers for client apps to read traffic/expiry info.`,subHeader:[{name:`Subscription-Userinfo`,desc:`Traffic and expiry: <code>upload=N; download=N; total=N; expire=TS</code>`},{name:`Profile-Title`,desc:`Base64-encoded subscription display name`},{name:`Profile-Web-Page-Url`,desc:`Link to the subscription info page`},{name:`Support-Url`,desc:`Support contact URL configured in settings`},{name:`Profile-Update-Interval`,desc:`Suggested polling interval in minutes (e.g. <code>10</code>)`},{name:`Announce`,desc:`Base64-encoded announcement string`},{name:`Routing-Enable`,desc:`<code>true</code> or <code>false</code> — whether routing rules are included`},{name:`Routing`,desc:`Global routing rules for client apps that support them (e.g. Happ)`}],endpoints:[{method:`GET`,path:`/{subPath}:subid`,summary:`Return base64-encoded subscription links for all enabled clients matching the subscription ID. When the request has an Accept: text/html header or ?html=1, renders a styled info page instead. Default path: /sub/:subid.`,params:[{name:`subid`,in:`path`,type:`string`,desc:`Client subscription ID.`}]},{method:`GET`,path:`/{jsonPath}:subid`,summary:`Return subscription as a JSON array of proxy configs (one per enabled client). Only when JSON subscription is enabled in settings. Default path: /json/:subid.`,params:[{name:`subid`,in:`path`,type:`string`,desc:`Client subscription ID.`}]},{method:`GET`,path:`/{clashPath}:subid`,summary:`Return subscription as a Clash/Mihomo-compatible YAML config. Only when Clash subscription is enabled in settings. Default path: /clash/:subid.`,params:[{name:`subid`,in:`path`,type:`string`,desc:`Client subscription ID.`}]}]},{id:`websocket`,title:`WebSocket`,description:`Real-time status updates via WebSocket. Connect once at <code>ws://<panel>/ws</code> to receive a stream of JSON messages without polling. Requires an authenticated session cookie (Bearer token auth is not supported). Each message has a <code>type</code> field that identifies the payload shape.`,endpoints:[{method:`GET`,path:`/ws`,summary:`Upgrade an HTTP connection to a WebSocket. Requires an authenticated session cookie (Bearer token auth is not supported here). Returns 101 Switching Protocols on success. The server then pushes JSON messages described below.`},{method:`WS`,path:`→ type: status`,summary:`Server health snapshot pushed every 2 seconds. Contains CPU, memory, swap, disk, network IO, load, and Xray state — same shape as <code>GET /panel/api/server/status</code>.`,response:`{
  "type": "status",
  "data": { "cpu": 12.5, "mem": { "current": 2147483648, "total": 8589934592 }, "xray": { "state": "running" } }
}`},{method:`WS`,path:`→ type: xrayState`,summary:`Xray process state change. Fired when Xray starts, stops, or encounters an error.`,response:`{
  "type": "xrayState",
  "data": "running"
}`},{method:`WS`,path:`→ type: notification`,summary:`In-panel toast notification. Fired on Xray stop/restart, DB import, panel restart, etc.`,response:`{
  "type": "notification",
  "title": "Xray service restarted",
  "body": "Xray has been restarted successfully",
  "severity": "success"
}`},{method:`WS`,path:`→ type: invalidate`,summary:`Instructs the UI to re-fetch a resource. Fired when another admin session modifies data (e.g. toggling inbound enable).`,response:`{
  "type": "invalidate",
  "resource": "inbounds"
}`}]}],K={GET:`blue`,POST:`green`,PUT:`orange`,PATCH:`orange`,DELETE:`red`,WS:`purple`},q={class:`code-block-wrapper`},J={class:`code-toolbar`},Y={class:`lang-badge`},X=[`title`],oe=[`innerHTML`],Z=r({__name:`CodeBlock`,props:{code:{type:String,default:``},lang:{type:String,default:`json`}},setup(e){let t=e,n=P(!1);function r(e){return e.replace(/&/g,`&amp;`).replace(/</g,`&lt;`).replace(/>/g,`&gt;`)}function i(e){return r(e).replace(/("(?:[^"\\]|\\.)*")\s*(:)|("(?:[^"\\]|\\.)*")|(-?\d+\.?\d*(?:[eE][+-]?\d+)?)\b|(true|false)|(null)|([{}[\]])/g,(e,t,n,r,i,a,o)=>n?`<span class="json-key">${t}</span>${n}`:r?`<span class="json-string">${r}</span>`:i?`<span class="json-number">${i}</span>`:a?`<span class="json-boolean">${a}</span>`:o?`<span class="json-null">${o}</span>`:e)}let a=p(()=>t.lang===`json`?i(t.code):r(t.code));async function o(){try{await navigator.clipboard.writeText(t.code),n.value=!0,T.success(`Copied`),setTimeout(()=>{n.value=!1},2e3)}catch{T.error(`Copy failed`)}}return(t,r)=>(S(),O(`div`,q,[h(`div`,J,[h(`span`,Y,d(e.lang.toUpperCase()),1),h(`button`,{class:l([`copy-btn`,{copied:n.value}]),onClick:o,title:n.value?`Copied`:`Copy`},[n.value?(S(),c(N(w),{key:0})):(S(),c(N(D),{key:1}))],10,X)]),h(`pre`,{class:l([`code-block`,`lang-${e.lang}`])},[h(`code`,{innerHTML:a.value},null,8,oe)],2)]))}},[[`__scopeId`,`data-v-b44b9ff8`]]),se={class:`endpoint-row`},ce={class:`endpoint-header`},le={class:`endpoint-path`},Q=[`innerHTML`],ue={key:1,class:`endpoint-block`},de={key:2,class:`endpoint-block`},fe={key:3,class:`endpoint-block`},pe={key:4,class:`endpoint-block`},me=r({__name:`EndpointRow`,props:{endpoint:{type:Object,required:!0}},setup(e){let t=e,n=p(()=>K[t.endpoint.method]||`default`),r=p(()=>Array.isArray(t.endpoint.params)&&t.endpoint.params.length>0),i=[{title:`Name`,dataIndex:`name`,key:`name`,width:180},{title:`In`,dataIndex:`in`,key:`in`,width:100},{title:`Type`,dataIndex:`type`,key:`type`,width:120},{title:`Description`,dataIndex:`desc`,key:`desc`}];return(t,a)=>{let o=k(`a-tag`),s=k(`a-table`);return S(),O(`div`,se,[h(`div`,ce,[g(o,{color:n.value,class:`method-tag`},{default:b(()=>[_(d(e.endpoint.method),1)]),_:1},8,[`color`]),h(`code`,le,d(e.endpoint.path),1)]),e.endpoint.summary?(S(),O(`p`,{key:0,class:`endpoint-summary`,innerHTML:N(W)(e.endpoint.summary)},null,8,Q)):u(``,!0),r.value?(S(),O(`div`,ue,[a[0]||(a[0]=h(`div`,{class:`block-label`},`Parameters`,-1)),g(s,{columns:i,"data-source":e.endpoint.params,pagination:!1,size:`small`,"row-key":`name`},null,8,[`data-source`])])):u(``,!0),e.endpoint.body?(S(),O(`div`,de,[a[1]||(a[1]=h(`div`,{class:`block-label`},`Request body`,-1)),g(Z,{code:e.endpoint.body,lang:`json`},null,8,[`code`])])):u(``,!0),e.endpoint.response?(S(),O(`div`,fe,[a[2]||(a[2]=h(`div`,{class:`block-label`},`Response`,-1)),g(Z,{code:e.endpoint.response,lang:`json`},null,8,[`code`])])):u(``,!0),e.endpoint.errorResponse?(S(),O(`div`,pe,[a[3]||(a[3]=h(`div`,{class:`block-label error-label`},`Error response`,-1)),g(Z,{code:e.endpoint.errorResponse,lang:`json`},null,8,[`code`])])):u(``,!0)])}}},[[`__scopeId`,`data-v-b4ad3257`]]),he=[`id`],ge={class:`section-header-left`},_e={class:`section-title`},ve={class:`endpoint-count`},ye=[`innerHTML`],be={key:1,class:`sub-header-block`},xe=[`innerHTML`],Se={class:`endpoints`},Ce=r({__name:`EndpointSection`,props:{section:{type:Object,required:!0},icon:{type:Object,default:null},collapsed:{type:Boolean,default:!1}},emits:[`toggle`],setup(e,{emit:t}){let n=e,r=t,i=p(()=>n.section.endpoints.length===1?`1 endpoint`:`${n.section.endpoints.length} endpoints`);return(t,n)=>{let a=k(`a-table`);return S(),O(`section`,{id:e.section.id,class:`api-section`},[h(`div`,{class:`section-header`,onClick:n[0]||(n[0]=e=>r(`toggle`))},[h(`div`,ge,[e.collapsed?(S(),c(N(M),{key:1,class:`collapse-icon`})):(S(),c(N(C),{key:0,class:`collapse-icon`})),e.icon?(S(),c(y(e.icon),{key:2,class:`section-icon`})):u(``,!0),h(`h2`,_e,d(e.section.title),1)]),h(`span`,ve,d(i.value),1)]),e.section.description&&!e.collapsed?(S(),O(`p`,{key:0,class:`section-description`,innerHTML:N(W)(e.section.description)},null,8,ye)):u(``,!0),e.section.subHeader&&!e.collapsed?(S(),O(`div`,be,[n[1]||(n[1]=h(`div`,{class:`block-label`},`Response headers`,-1)),g(a,{columns:[{title:`Header`,dataIndex:`name`,key:`name`,width:240},{title:`Description`,dataIndex:`desc`,key:`desc`}],"data-source":e.section.subHeader,pagination:!1,size:`small`,"row-key":`name`},{bodyCell:b(({column:e,text:t})=>[e.dataIndex===`desc`?(S(),O(`span`,{key:0,innerHTML:N(W)(t)},null,8,xe)):(S(),O(m,{key:1},[_(d(t),1)],64))]),_:1},8,[`data-source`])])):u(``,!0),x(h(`div`,Se,[(S(!0),O(m,null,v(e.section.endpoints,(e,t)=>(S(),c(me,{key:t,endpoint:e},null,8,[`endpoint`]))),128))],512),[[f,!e.collapsed]])],8,he)}}},[[`__scopeId`,`data-v-01f99ee0`]]),we={class:`docs-wrapper`},Te={class:`token-card-head`},Ee={class:`token-card-title`},De={class:`toolbar`},Oe={key:0,class:`match-count`},ke={class:`toc-nav`},Ae={class:`toc-links`},je=[`href`,`onClick`],Me={class:`toc-text`},Ne={class:`toc-badge`},Pe=`curl -X GET \\
  -H "Authorization: Bearer YOUR_API_TOKEN" \\
  -H "Accept: application/json" \\
  https://your-panel.example.com/panel/api/inbounds/list`,Fe=r({__name:`ApiDocsPage`,setup(n){let r=window.X_UI_BASE_PATH||``,i=window.location.pathname,s=`${r}panel/settings#security`,f=P(``),x=P(new Set),C=P(``),w={authentication:re,inbounds:ie,server:L,nodes:z,"custom-geo":I,backup:R,settings:H,"api-tokens":F,"xray-settings":V,subscription:ne,websocket:B},T=p(()=>{let e=f.value.toLowerCase().trim();return e?G.map(t=>{let n=t.endpoints.filter(t=>t.path.toLowerCase().includes(e)||t.summary?.toLowerCase().includes(e)||t.method.toLowerCase().includes(e));return{...t,endpoints:n}}).filter(e=>e.endpoints.length>0):G}),D=p(()=>G.reduce((e,t)=>e+t.endpoints.length,0)),A=p(()=>T.value.reduce((e,t)=>e+t.endpoints.length,0));function M(e){return x.value.has(e)}function U(e){let t=new Set(x.value);t.has(e)?t.delete(e):t.add(e),x.value=t}function W(){x.value=new Set}function K(){x.value=new Set(G.map(e=>e.id))}function q(e){let t=document.getElementById(e);t&&(t.scrollIntoView({behavior:`smooth`,block:`start`}),window.location.hash!==`#${e}`&&history.replaceState(null,``,`#${e}`))}function J(){let e=window.location.hash.slice(1);if(!e)return;let t=document.getElementById(e);t&&t.scrollIntoView({behavior:`auto`,block:`start`})}let Y=null;function X(){let e=document.querySelector(`.toc-nav`),t=e?e.offsetHeight:56,n=``;for(let e of T.value){let r=document.getElementById(e.id);r&&r.getBoundingClientRect().top<=t+20&&(n=e.id)}C.value=n}return E(()=>{Y=X,window.addEventListener(`scroll`,Y,{passive:!0}),window.addEventListener(`hashchange`,J),requestAnimationFrame(()=>{J(),X()})}),j(()=>{Y&&window.removeEventListener(`scroll`,Y),window.removeEventListener(`hashchange`,J)}),(n,p)=>{let x=k(`a-button`),E=k(`a-card`),j=k(`a-input-search`),P=k(`a-space`),I=k(`a-layout-content`),L=k(`a-layout`),R=k(`a-config-provider`);return S(),c(R,{theme:N(t)},{default:b(()=>[g(L,{class:l([`api-docs-page`,{"is-dark":N(e).isDark,"is-ultra":N(e).isUltra}])},{default:b(()=>[g(ae,{"base-path":N(r),"request-uri":N(i)},null,8,[`base-path`,`request-uri`]),g(L,{class:`content-shell`},{default:b(()=>[g(I,{class:`content-area`},{default:b(()=>[h(`div`,we,[p[10]||(p[10]=h(`header`,{class:`docs-header`},[h(`h1`,{class:`docs-title`},`API Documentation`),h(`p`,{class:`docs-lead`},[_(` The 3x-ui panel exposes a REST API under `),h(`code`,null,`/panel/api/`),_(`. Authenticate with the panel session cookie, or with the `),h(`code`,null,`Authorization: Bearer <token>`),_(` header below. Every endpoint returns a uniform `),h(`code`,null,`{ success, msg, obj }`),_(` envelope unless otherwise noted. `)])],-1)),g(E,{class:`token-card`,size:`small`},{default:b(()=>[h(`div`,Te,[h(`div`,Ee,[g(N(F)),p[1]||(p[1]=h(`span`,null,`API Tokens`,-1))]),g(x,{type:`primary`,size:`small`,href:s},{default:b(()=>[...p[2]||(p[2]=[_(` Manage tokens `,-1)])]),_:1})]),h(`p`,{class:`token-hint`},[p[3]||(p[3]=_(` Create, enable, or revoke named Bearer tokens in `,-1)),h(`a`,{href:s},`Settings → Security`),p[4]||(p[4]=_(`. Send each request as `,-1)),p[5]||(p[5]=h(`code`,null,`Authorization: Bearer <token>`,-1)),p[6]||(p[6]=_(`. Token-authenticated callers skip CSRF and don't need a session cookie. Deleting a token revokes it immediately — running bots will need a new one. `,-1))])]),_:1}),g(E,{class:`curl-card`,size:`small`,title:`Quick example`},{default:b(()=>[g(Z,{code:Pe,lang:`text`})]),_:1}),h(`div`,De,[g(j,{value:f.value,"onUpdate:value":p[0]||(p[0]=e=>f.value=e),placeholder:`Search endpoints by path, method, or description…`,"allow-clear":``,class:`search-bar`},{prefix:b(()=>[g(N(o))]),_:1},8,[`value`]),f.value?(S(),O(`span`,Oe,d(A.value)+` / `+d(D.value)+` endpoints `,1)):u(``,!0),g(P,{size:`small`},{default:b(()=>[g(x,{size:`small`,onClick:W},{icon:b(()=>[g(N(ee))]),default:b(()=>[p[7]||(p[7]=_(` Expand all `,-1))]),_:1}),g(x,{size:`small`,onClick:K},{icon:b(()=>[g(N(te))]),default:b(()=>[p[8]||(p[8]=_(` Collapse all `,-1))]),_:1})]),_:1})]),h(`nav`,ke,[p[9]||(p[9]=h(`span`,{class:`toc-label`},`On this page:`,-1)),h(`div`,Ae,[(S(!0),O(m,null,v(T.value,e=>(S(),O(`a`,{key:e.id,class:l([`toc-link`,{active:C.value===e.id}]),href:`#${e.id}`,onClick:a(t=>q(e.id),[`prevent`])},[(S(),c(y(w[e.id]),{class:`toc-icon`})),h(`span`,Me,d(e.title),1),h(`span`,Ne,d(e.endpoints.length),1)],10,je))),128))])]),(S(!0),O(m,null,v(T.value,e=>(S(),c(Ce,{key:e.id,section:e,icon:w[e.id],collapsed:M(e.id),onToggle:t=>U(e.id)},null,8,[`section`,`icon`,`collapsed`,`onToggle`]))),128))])]),_:1})]),_:1})]),_:1},8,[`class`])]),_:1},8,[`theme`])}}},[[`__scopeId`,`data-v-5faac377`]]);U(),i();var $=document.getElementById(`message`);$&&T.config({getContainer:()=>$}),s(Fe).use(A).use(n).mount(`#app`);
//# sourceMappingURL=apiDocs-ByPHoiZD.js.map