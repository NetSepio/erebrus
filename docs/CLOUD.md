# Cloud deployment checklist

Operators do **not** hand-edit `.env` files. Use the installer, then verify with `docker compose logs` or `curl /api/v2/status`.

## Access modes

| Mode | Who can connect |
|------|-----------------|
| **private** | You, your devices, and org members (not listed publicly) |
| **public** | Entitled network users (listed in the public directory) |

`EREBRUS_NETWORK_PROFILE` controls Docker container networking: `bridge` (default) or `host-network`.

## Ports (docker / private)

| Port | Proto |
|------|-------|
| 9080 | tcp |
| 51820 | udp |
| 8443 | tcp |
| 4443 | udp |
| 4001 | tcp + udp (Drop only) |

Public Docker nodes use stealth on **443/tcp** and **443/udp**.

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