# Running an Erebrus node

A node is a Linux host (x86_64/arm64) with a **static, internet-routable public IP**,
real bandwidth, and open ports. It serves WireGuard plus two DPI-resistant stealth
carriers, and exposes a small REST API the gateway and operators use.

## Quick install

```bash
curl -fsSL https://erebrus.io/install.sh | bash
```

The installer runs preflight checks (static IP / NAT, up+down bandwidth, inbound
port reachability), then asks for an install **mode**:

| Mode | What you get | Use it when |
|------|--------------|-------------|
| **docker** | WireGuard + stealth carriers in a container (compose). | You just want to run a VPN node. Recommended. |
| **host** | Bare-metal via `systemd`. Adds **App-Hosting** (expose a VPN-connected app to the internet). | You want app/port exposure and can set a wildcard DNS record. |

Non-interactive:

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." WG_ENDPOINT_HOST="vpn.example.com" bash -s -- --mode docker --yes
```

## Ports

| Port | Proto | Purpose |
|------|-------|---------|
| 9080 | tcp | REST API (`/api/v2`) + `/metrics` |
| 51820 | udp | WireGuard fast path |
| 8443 | tcp | VLESS + REALITY stealth carrier |
| 4443 | udp | Hysteria2 stealth carrier |
| 80, 443 | tcp | Caddy ingress — **host mode + App-Hosting only** |

Open all of these in your cloud firewall / security group. UDP can't be probed
remotely, so double-check 51820 and 4443.

## Configuration

Full reference: [`.env.example`](../.env.example). The only required values are
`MNEMONIC` (the node identity — back it up) and `WG_ENDPOINT_HOST`. The installer
generates a `MNEMONIC` and `NODE_API_TOKEN` for you if unset.

- **docker** config: `${INSTALL_DIR}/.env` (default `/opt/erebrus/.env`)
- **host** config: `/etc/erebrus/erebrus.env`

## Managing the node

**docker**
```bash
cd /opt/erebrus
docker compose ps
docker compose logs -f
docker compose restart
docker compose down
```

**host**
```bash
systemctl status erebrus
journalctl -u erebrus -f
systemctl restart erebrus
```

## Verify

```bash
# Carriers advertised
curl -s http://127.0.0.1:9080/api/v2/status | jq '.protocols, .capabilities.stealth'
# → ["wireguard","vless-reality","hysteria2"]  and  true

# Provision a peer and inspect the unified credential bundle
TOKEN=<NODE_API_TOKEN>
PUB=$(wg genkey | wg pubkey)
curl -s -X PUT http://127.0.0.1:9080/api/v2/peers/test \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"name\":\"test\",\"wg_public_key\":\"$PUB\"}" | jq
```

The bundle returns the WireGuard config plus `vless://` / `hysteria2://` share URIs
and a ready sing-box client profile (WireGuard tunnelled through the carrier).

## App-Hosting (host mode)

Create a wildcard DNS record pointing at the node:

```
*.apps.example.com   A   <node-public-ip>
```

The gateway then mints per-app CNAMEs under it and routes public traffic through the
node to the chosen VPN client's port. (Route automation lands with the gateway; the
installer prepares the host — Caddy + the wildcard domain.)

## Troubleshooting

- **`reality server is not included in this build`** — the binary was built without
  `-tags with_reality_server`. Use `make build` / the provided Dockerfile.
- **WireGuard interface won't come up** — the host needs the `wireguard` kernel
  module and `NET_ADMIN`. In containers, run with `--cap-add=NET_ADMIN` (compose
  already does). Load it on the host with `modprobe wireguard` if missing.
- **Peer create returns 500 in local dev** — expected when there's no live WG
  device; the credentials endpoint still renders bundles. On a real host with
  `NET_ADMIN` it succeeds.
- **Stealth ports not reachable** — confirm the cloud firewall allows 8443/tcp and
  4443/udp; `ss -tlnp | grep 8443` and `ss -ulnp | grep 4443` show them locally.
