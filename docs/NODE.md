# Running an Erebrus node

A node is a Linux host (x86_64/arm64) with a **static, internet-routable public IP**,
real bandwidth, and open ports. It serves WireGuard plus two DPI-resistant stealth
carriers, and exposes a small REST API the gateway and operators use.

## Quick install

```bash
curl -fsSL https://erebrus.io/install.sh | bash
```

The installer runs preflight checks (static IP / NAT, up+down bandwidth, inbound
port reachability).

Non-interactive:

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." \
  EREBRUS_ACCESS=public \
  EREBRUS_NODE_REGISTRATION_TOKEN="ere_reg_..." \
  bash -s -- --yes --skip-checks
```

The installer detects the public IP and uses it as `WG_ENDPOINT_HOST`. Set the
variable explicitly only to override the detected public IP; hosts behind NAT
still require port forwarding.

Add `--drop` to run the optional Kubo/IPFS storage sidecar. `--no-drop` stops
Drop while preserving its volume. Drop v1 requires Docker.

## Ports

| Port | Proto | Purpose |
|------|-------|---------|
| 9080 | tcp | REST API (`/api/v2`) + `/metrics` |
| 51820 | udp | WireGuard fast path |
| 443 | tcp | VLESS + REALITY stealth carrier (all nodes) |
| 443 | udp | Hysteria2 stealth carrier (all nodes) |
| 4001 | tcp + udp | Kubo swarm — **Drop only** |

Open the ports required by the selected features in your cloud firewall /
security group. `443/tcp` is not used by Drop. UDP can't be probed remotely, so
double-check 51820, 443/udp, and Drop's 4001/udp when enabled. The installer probes
`4001/tcp`. Kubo admin RPC `5001` and the raw Kubo gateway `8080` are
internal-only and must not be published.

## Configuration

Full reference: [`.env.example`](../.env.example). The only required values are
`MNEMONIC` (the node identity — back it up) and `WG_ENDPOINT_HOST`. The installer
generates a `MNEMONIC` and `NODE_API_TOKEN` for you if unset.

### Access modes (`EREBRUS_ACCESS`)

| Mode | Who can connect |
|------|-----------------|
| **private** | Your devices and org members only (not listed in the public directory) |
| **public** | Entitled network users (listed in the public directory) |

Set via `EREBRUS_ACCESS=private` or `EREBRUS_ACCESS=public`. Access mode controls
**who** can use the node, not stealth ports — all profiles bind carriers on
**443/tcp** and **443/udp** for reachability through restrictive networks.

### Region and zone (for multi-node directories)

| Variable | Purpose | Example |
|----------|---------|---------|
| `REGION` | Country or broad geography | `US` (installer auto-detects via ipinfo), `NO`, `SG` |
| `ZONE` | Optional sub-region for clients to pick between nodes | `east`, `west` (auto for US via ipinfo longitude), `nyc-1` |
| `NODE_NAME` | Operator-facing label | `erebrus-us-east-01` |

Both `REGION` and `ZONE` are sent to the gateway on registration and in WebSocket
`hello` / `heartbeat` (`spec.region`, `spec.zone`). The local dashboard shows them
too. Gateway-side filtering/display is a separate follow-up.

### Deployment profiles

| Profile | Compose | Extra services |
|---------|---------|----------------|
| `standard` (default) | `deploy/compose/erebrus.yml` | VPN node only |
| `shield` | `deploy/compose/shield.yml` | AdGuard Home DNS |
| `sentinel` | `deploy/compose/sentinel.yml` | erebrus-sentinel (Unbound API) |

Installer: `./install.sh --profile shield` (or interactive prompt). Sets
`EREBRUS_PROFILE`, `FIREWALL_PROVIDER`, `FIREWALL_DNS_ADDR`, and `WG_DNS` for
tunnel DNS routing.

Registration sends `deployment_profile` to the gateway; firewall rules sync via
WS `sync_firewall` when an operator calls `POST .../firewall/sync` on the gateway.

### Erebrus Drop

Drop adds the same `deploy/compose/drop.yml` override to Standard, Shield, or
Sentinel. The installer prompts for it, or accepts:

```bash
./install.sh --profile standard --drop
./install.sh --profile shield --drop
./install.sh --profile sentinel --drop
```

Kubo uses `ipfs/kubo:v0.42.0`, stores its repo in a persistent `kubo_data`
volume, and receives a deterministic identity distinct from the Erebrus node
PeerID. Files are uploaded and read only through the authenticated Erebrus
gateway. The raw Kubo `8080` and `5001` ports are never published.
See [DROP.md](DROP.md) for APIs, metrics, upgrades, and destructive cleanup.

### Gateway registration

Nodes enroll with a scoped **registration token** (`ere_reg_*`), not a permanent org
secret. Org owners/admins mint tokens via the gateway:
`POST /api/v2/orgs/{org_id}/node-registration-tokens`.

| Variable | Purpose |
|----------|---------|
| `GATEWAY_URL` | Gateway base URL (e.g. `https://gateway.erebrus.io`) |
| `EREBRUS_NODE_REGISTRATION_TOKEN` | Scoped token for `POST /api/v2/nodes/register` |
| `NODE_ID` / `NODE_TOKEN` | Persisted after registration (auto-register when unset) |

The gateway returns `node_id` = libp2p `peer_id` (same value in WS `hello.node_id`).
`EREBRUS_ORG_ENROLLMENT_SECRET` is a deprecated alias for the registration token.

On an existing US node:

```bash
# edit /opt/erebrus/.env (or your INSTALL_DIR)
ZONE=east
REGION=US
docker compose up -d
```

- **docker** config: `${INSTALL_DIR}/.env` (default `/opt/erebrus/.env`)

## Managing the node

**docker**
```bash
cd /opt/erebrus
docker compose ps
docker compose logs -f
docker compose restart
docker compose down
```

For an installer-managed Drop node, include the override in direct Compose
commands:

```bash
docker compose --env-file .env -f docker-compose.yml -f drop.yml ps
docker compose --env-file .env -f docker-compose.yml -f drop.yml logs -f kubo
```

Do not use `down -v`; it deletes persistent node and Kubo volumes.

## Verify

```bash
# Carriers advertised
curl -s http://127.0.0.1:9080/api/v2/status | jq '.protocols, .capabilities.stealth'
# → ["wireguard","vless-reality","hysteria2"]  and  true

# Drop capability, service state, and optional readiness check
curl -s http://127.0.0.1:9080/api/v2/status | \
  jq '.capabilities.drop, .capabilities.services.drop, (.readiness.checks[] | select(.id == "drop"))'

# Provision a peer and inspect the unified credential bundle
TOKEN=<NODE_API_TOKEN>
PUB=$(wg genkey | wg pubkey)
curl -s -X PUT http://127.0.0.1:9080/api/v2/peers/test \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"name\":\"test\",\"wg_public_key\":\"$PUB\"}" | jq
```

The bundle returns the WireGuard config plus `vless://` / `hysteria2://` share URIs
and a ready sing-box client profile (WireGuard tunnelled through the carrier).

## Troubleshooting

- **`reality server is not included in this build`** — the binary was built without
  `-tags with_reality_server`. Use `make build` / the provided Dockerfile.
- **WireGuard interface won't come up** — the host needs the `wireguard` kernel
  module and `NET_ADMIN`. In containers, run with `--cap-add=NET_ADMIN` (compose
  already does). Load it on the host with `modprobe wireguard` if missing.
- **Peer create returns 500 in local dev** — expected when there's no live WG
  device; the credentials endpoint still renders bundles. On a real host with
  `NET_ADMIN` it succeeds.
- **Stealth ports not reachable** — confirm the cloud firewall allows 443/tcp and
  443/udp; `ss -tlnp | grep 443` and `ss -ulnp | grep 443` show them locally.
- **Drop is `unreachable`** — inspect `docker compose ... logs kubo`. VPN
  readiness remains independent and should stay available.
- **Kubo identity conflict** — verify the node mnemonic belongs with the
  existing `kubo_data` volume. Do not delete or replace the Kubo config
  automatically; follow the recovery steps in [DROP.md](DROP.md).
