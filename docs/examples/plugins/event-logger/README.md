# event-logger — a working stage 3b example plugin

A minimal plugin that shows the events contract end to end: the panel starts
it (as a systemd unit when the host allows one), passes the unix socket path
in `LUCX_PLUGIN_SOCKET`, and POSTs eventbus events to `/_lucx/events`. This
plugin logs every received event to stdout, which lands in the plugin's
`plugin.log` — open the Plugins page and watch the log.

## Build

Static binary for the host architecture:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o event-logger .
```

## Package

A ZIP containing `manifest.json` (must exist at the archive root) and the
binary named by `entrypoint` (`event-logger` for amd64):

```
manifest.json
event-logger
```

Install through **Settings → Plugin Manager** (ZIP upload). The manifest
subscribes to `cpu.high`, `memory.high`, `xray.crash`, `outbound.down`,
`outbound.up` and `login.attempt`. Note the publish sources: the panel only
runs the minute-cadence CPU/memory samplers when a notifier (Telegram or
email) subscribes to those alarms; login attempts fire on every login form
submission; `xray.crash` fires when the Xray process dies — kill it on the
host to see an event land immediately in the log.

## The contract, in one page

- The socket path arrives in `LUCX_PLUGIN_SOCKET`; `unlink` it and bind it
  yourself, then `chmod 0600`.
- Serve `POST /_lucx/events`; answer any 2xx quickly. Delivery is best
  effort: 5 second timeout, no retry, one at a time per plugin, ordered.
- Payloads are `{id, type, source, time, data}`; `data` carries the raw
  event payload with Go field names (`{"Percent":42.0,"Threshold":0}` for
  cpu.high, a string for xray.crash, `{"Username","IP","Time","Status",
  "Reason"}` for login.attempt).
- Only the declared `events` names are delivered; vocabulary mirrors the
  panel's eventbus publishers. Unknown names are refused at install.
- The panel counts `delivered` and `failed` per plugin in the status.
