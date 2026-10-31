# Sidecar plugins (MVP)

LucX-UI can install and supervise user-supplied sidecar plugins. This first
release intentionally exposes only process management. Plugins do not receive
panel API access, database access, Telegram bot tokens, or subscription seams.

## Package

Upload a ZIP from **Settings → Cores → Sidecar plugins**. The archive must have
a `manifest.json` at its root:

```json
{
  "id": "example-sidecar",
  "name": "Example sidecar",
  "version": "1.0.0",
  "apiVersion": 1,
  "type": "sidecar",
  "entrypoint": "bin/example-sidecar",
  "architectures": ["amd64", "arm64"],
  "permissions": {
    "network": [],
    "panelApi": [],
    "bot": []
  }
}
```

`entrypoint` must stay inside the plugin directory. The current host
architecture must be listed. The package is limited to 200 MB and must contain
an executable at the declared path after extraction.

## Runtime contract

The plugin runs as a separate process with its working directory set to its
installation directory. LucX-UI provides:

- `LUCX_PLUGIN_ID`;
- `LUCX_PLUGIN_VERSION`;
- `LUCX_PLUGIN_DATA`, a private writable data directory.

Logs are stored in `plugin.log`; the panel provides status, logs, start, stop,
restart, install, and delete operations. Plugin files live below
`bin/plugins/<id>/`.

This MVP does not sandbox arbitrary native code. Install only packages you
trust. The next plugin iteration will add explicit permission enforcement,
health checks, resource limits, and authenticated panel seams before plugins
can replace bots or read panel data.
