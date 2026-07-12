# Erebrus

Erebrus is a decentralized VPN (DePIN) that protects your privacy and security with no hidden tracking or logging. Anyone worldwide can run a node — on a physical server or a VM — and earn incentives, helping build a censorship-resistant network.

For more details visit [erebrus.io](https://erebrus.io).

## Features

- WireGuard fast path with a SQLite-backed, race-free peer store.
- **Stealth carriers** for restrictive networks: when WireGuard's UDP is throttled or DPI-blocked, the same tunnel is wrapped in an embedded sing-box transport that looks like ordinary internet traffic:
  - **VLESS + REALITY** (`:8443/tcp`) — presents as a real TLS session to a borrowed SNI.
  - **Hysteria2** (`:4443/udp`) — QUIC/HTTP3 with optional Salamander obfuscation.
- libp2p identity + DID (`did:erebrus:<peerId>`) derived from a mnemonic.
- Optional **Erebrus Drop** storage: a pinned Kubo/IPFS sidecar with a separate
  mnemonic-derived libp2p identity and persistent local pins.
- HTTP REST API (`/api/v2`) and Prometheus `/metrics`.
- Optional App-Hosting: expose a VPN-connected app to the public internet (host mode).

## Install a node

Linux only (x86_64 / arm64). A node needs a **static, internet-routable public IP**, real bandwidth, and open ports (`9080/tcp`, `51820/udp`, `8443/tcp`, `4443/udp`). Drop nodes additionally publish `8080/tcp` for CID retrieval and `4001/tcp+udp` for the IPFS swarm. The installer verifies the required TCP ports.

```bash
curl -fsSL https://erebrus.io/install.sh | bash
```

You'll be asked to pick a mode:

- **docker** (recommended) — zero-hassle: WireGuard + stealth carriers in a container.
- **host** — bare-metal via systemd; additionally supports **App-Hosting** (needs a wildcard DNS record, e.g. `*.apps.example.com → <node-ip>`, so the gateway can mint per-app CNAMEs).

Non-interactive example:

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." WG_ENDPOINT_HOST="vpn.example.com" bash -s -- --mode docker --drop --yes
```

Drop is optional, works with the Standard, Shield, and Sentinel Docker profiles,
and is not supported by host mode in v1. Use `--no-drop` to stop the sidecar
without deleting its persistent data.

## Build from source

The REALITY server requires a build tag, wired into the Makefile and Dockerfile:

```bash
make build      # go build -tags with_reality_server -o erebrus ./cmd/erebrus
make test
```

## Dashboard

Every node serves a local dashboard at `http://<node>:9080/` — intro, live stats
(connected users, bandwidth, throughput, uptime), and the API reference. It reads
only public, coarse aggregates (`/api/v2/status`, `/api/v2/stats`).

## Docs

- [docs/NODE.md](docs/NODE.md) — running, configuring, and managing a node (ports, env reference, troubleshooting).
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — package layout and the stealth carrier topology.
- [docs/SECURITY-AUDIT.md](docs/SECURITY-AUDIT.md) — data-capture inventory, threat model, and operator hardening.
- [docs/DROP.md](docs/DROP.md) — Drop installation, private APIs, metrics, and safe storage operations.
- [docs/node-api.openapi.yaml](docs/node-api.openapi.yaml) — the `/api/v2` REST contract.

The REST surface lives under `/api/v2` (status, stats, peers CRUD, credentials,
and gateway-private Drop operations); node status is public at
`GET /api/v2/status`.
