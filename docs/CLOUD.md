# Cloud deployment checklist

Operators do **not** hand-edit `.env` files. Use the installer or `erebrus init`, then verify with `erebrus status`.

## Access modes

| Mode | Who can connect |
|------|-----------------|
| **private** | You, your devices, and org members (not listed publicly) |
| **public** | Entitled network users (listed in the public directory) |

Deployment profile (`EREBRUS_NETWORK_PROFILE`) is separate: `bridge` for Docker, `host-network` for bare metal.

## Ports (docker / private)

| Port | Proto |
|------|-------|
| 9080 | tcp |
| 51820 | udp |
| 8443 | tcp |
| 4443 | udp |
| 443 | tcp (Drop TLS CID gateway only, when a domain is configured) |
| 4001 | tcp + udp (Drop only) |

Public bare-metal nodes use stealth on **443/tcp** and **443/udp**.

## Install (docker)

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." \
  EREBRUS_ACCESS=public \
  EREBRUS_NODE_REGISTRATION_TOKEN="ere_reg_..." \
  bash -s -- --yes --skip-checks
```

The installer detects the public IP for `WG_ENDPOINT_HOST`. Set it explicitly
only when the node should advertise a DNS name or a different public address.

Omit `--drop` or pass `--no-drop` for VPN-only deployment. Drop publishes swarm
on `4001/tcp+udp`; add `--drop-public-gateway-domain <domain>` only when
unauthenticated direct CID retrieval on `https://<domain>/ipfs/<cid>` is
intended and DNS points to the node. Admin RPC `5001` and the raw Kubo gateway
`8080` remain private.

## Install (bare metal)

```bash
sudo erebrus init --access private --public-address <ip> --yes
# configure systemd EnvironmentFile=/etc/erebrus/erebrus.env
sudo systemctl enable --now erebrus
```

## Verify

```bash
erebrus status
curl -s http://127.0.0.1:9080/api/v2/status | jq '.readiness'
```

Back up your **node identity** (12-word phrase) from installer/init output — it is never shown again in status.