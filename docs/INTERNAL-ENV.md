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
| `DROP_PUBLIC_GATEWAY_DOMAIN` | Optional DNS name for a TLS public CID gateway; empty means no public reads |

When `DROP_PUBLIC_GATEWAY_DOMAIN` is set, a pinned Traefik sidecar terminates
TLS on `443/tcp` and proxies only `/ipfs/*` to the internal Kubo gateway on
`8080`. Kubo RPC (`5001`) and the raw Kubo gateway are never host-published.
Public CID reads are advertised as `https://<domain>/ipfs/<cid>`.

See [`.env.example`](../.env.example) for the full internal template.