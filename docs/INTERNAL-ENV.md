# Internal environment reference (developers / installers)

End operators should use `erebrus status`, not this document.

## File locations

| Path | Used by |
|------|---------|
| `/opt/erebrus/.env` | Docker install (`docker compose --env-file`) |

## Required bootstrap

| Variable | Purpose |
|----------|---------|
| `MNEMONIC` | Node identity (12-word phrase) |
| `EREBRUS_ACCESS` | `private` or `public` (gateway directory visibility) |
| `WG_ENDPOINT_HOST` | Public IP address clients dial |

## Release-only

| Variable | Purpose |
|----------|---------|
| `NODE_API_TOKEN` | Bearer for peer API |

## Optional Drop

| Variable | Purpose |
|----------|---------|
| `DROP_ENABLED` | Run the Docker-only Kubo sidecar integration |
| `DROP_STORAGE_MAX` | Kubo repository storage limit, default `10GB` |
| `DROP_SWARM_PORT` | Published Kubo TCP/UDP swarm port, default `4001` |
| `DROP_WEBUI_ENABLED` | Enable the exact-purpose private WebUI proxy |

Kubo RPC (`5001`) and the raw Kubo gateway (`8080`) are never host-published.
Files are accessed only through the authenticated Erebrus API.

See [`.env.example`](../.env.example) for the full internal template.