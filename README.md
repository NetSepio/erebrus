# Erebrus

Erebrus is a decentralized VPN (DePIN) that protects your privacy and security with no hidden tracking or logging. Anyone worldwide can run a node — on a physical server or a VM — and earn incentives, helping build a censorship-resistant network.

For more details visit [erebrus.io](https://erebrus.io).

## Features

- WireGuard fast path with a SQLite-backed, race-free peer store.
- **Stealth carriers** for restrictive networks: when WireGuard's UDP is throttled or DPI-blocked, the same tunnel is wrapped in an embedded sing-box transport that looks like ordinary internet traffic:
  - **VLESS + REALITY** (`:8443/tcp`) — presents as a real TLS session to a borrowed SNI.
  - **Hysteria2** (`:4443/udp`) — QUIC/HTTP3 with optional Salamander obfuscation.
- libp2p identity + DID (`did:erebrus:<peerId>`) derived from a mnemonic.
- HTTP REST API (`/api/v2`) and Prometheus `/metrics`.
- Optional App-Hosting: expose a VPN-connected app to the public internet (host mode).

## Install a node

Linux only (x86_64 / arm64). A node needs a **static, internet-routable public IP**, real bandwidth, and open ports (`9080/tcp`, `51820/udp`, `8443/tcp`, `4443/udp`). The installer verifies all three.

```bash
curl -fsSL https://erebrus.io/install.sh | bash
```

You'll be asked to pick a mode:

- **docker** (recommended) — zero-hassle: WireGuard + stealth carriers in a container.
- **host** — bare-metal via systemd; additionally supports **App-Hosting** (needs a wildcard DNS record, e.g. `*.apps.example.com → <node-ip>`, so the gateway can mint per-app CNAMEs).

Non-interactive example:

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." WG_ENDPOINT_HOST="vpn.example.com" bash -s -- --mode docker --yes
```

## Build from source

The REALITY server requires a build tag, wired into the Makefile and Dockerfile:

```bash
make build      # go build -tags with_reality_server -o erebrus ./cmd/erebrus
make test
```

## API & docs

REST surface is under `/api/v2` (status, peers CRUD, credentials). Node status is public at `GET /api/v2/status`.
