# Cloud deployment checklist

Operators do **not** hand-edit `.env` files. Use the installer or `erebrus init`, then verify with `erebrus status`.

## Access modes

| Mode | Who can connect |
|------|-----------------|
| **private** | You and your devices only |
| **shared** | You plus wallet addresses you allow on the gateway |
| **public** | Entitled network users (host earnings via gateway — future) |

Deployment profile (`EREBRUS_NETWORK_PROFILE`) is separate: `bridge` for Docker, `host-network` for bare metal.

## Ports (docker / private)

| Port | Proto |
|------|-------|
| 9080 | tcp |
| 51820 | udp |
| 8443 | tcp |
| 4443 | udp |
| 8080 | tcp (Drop CID gateway only) |
| 4001 | tcp + udp (Drop only) |

Public bare-metal nodes use stealth on **443/tcp** and **443/udp**.

## Install (docker)

```bash
curl -fsSL https://raw.githubusercontent.com/NetSepio/erebrus/v2/install.sh | \
  WG_ENDPOINT_HOST="<public-ip>" \
  bash -s -- --mode docker --drop --yes
```

Omit `--drop` or pass `--no-drop` for VPN-only deployment. Drop publishes swarm
on `4001/tcp+udp`; add `--drop-public-gateway` only when unauthenticated direct
CID retrieval on `8080/tcp` is intended. Admin RPC `5001` remains private.

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