# Internal environment reference (developers / installers)

End operators should use `erebrus status`, not this document.

## File locations

| Path | Used by |
|------|---------|
| `/opt/erebrus/.env` | Docker install (`docker compose --env-file`) |
| `/etc/erebrus/erebrus.env` | Bare metal (`systemd EnvironmentFile`) |

## Required bootstrap

| Variable | Purpose |
|----------|---------|
| `MNEMONIC` | Node identity (12-word phrase) |
| `WG_ENDPOINT_HOST` | Public address clients dial |

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

Kubo RPC URL, RPC/gateway ports, service name, and image version are fixed
application/Compose defaults rather than environment settings.

See [`.env.example`](../.env.example) for the full internal template.