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

See [`.env.example`](../.env.example) for the full internal template.