# Cloud deployment checklist

Operators do **not** hand-edit `.env` files. Use the installer, then verify with `docker compose logs` or `curl /api/v2/status`.

## Access modes

| Mode | Who can connect |
|------|-----------------|
| **private** | You, your devices, and org members (not listed publicly) |
| **public** | Entitled network users (listed in the public directory) |

`EREBRUS_NETWORK_PROFILE` controls Docker container networking: `bridge` (default) or `host-network`.

## Ports (docker)

| Port | Proto | Purpose |
|------|-------|---------|
| 9080 | tcp | REST API |
| 51820 | udp | WireGuard |
| 443 | tcp | VLESS + REALITY (all nodes) |
| 443 | udp | Hysteria2 (all nodes) |
| 4001 | tcp + udp | Drop swarm only |

All nodes expose stealth on **443/tcp** and **443/udp**. `EREBRUS_ACCESS`
controls directory visibility, not carrier ports.

## Install

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." \
  EREBRUS_ACCESS=public \
  EREBRUS_NODE_REGISTRATION_TOKEN="ere_reg_..." \
  bash -s -- --yes --skip-checks
```

The installer detects the public IP for `WG_ENDPOINT_HOST`. Set it explicitly
only when the node should advertise a different public IP address.

Omit `--drop` or pass `--no-drop` for VPN-only deployment. Drop publishes swarm
on `4001/tcp+udp`. Files are accessed only through the authenticated Erebrus
API; admin RPC `5001` and the raw Kubo gateway `8080` remain private.

## Verify

```bash
curl -s http://127.0.0.1:9080/api/v2/status | jq '.readiness'
```

Back up your **node identity** (12-word phrase) from installer output — it is never shown again in status.