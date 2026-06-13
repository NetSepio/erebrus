# Node architecture (v2)

The node is a single Go binary (`cmd/erebrus`) plus a SQLite state file. It has no
external service dependencies at runtime.

## Packages

| Package | Responsibility |
|---------|----------------|
| `internal/config` | Environment-derived configuration + helpers. |
| `internal/store` | SQLite persistence: peers, node settings/secrets, race-free IP allocation. |
| `internal/wg` | WireGuard server: keypair, interface/peer config rendering, live sync via `wgctrl`. |
| `internal/stealth` | Embedded sing-box: VLESS+REALITY and Hysteria2 carriers + client profile/URI generation. |
| `internal/p2p` | libp2p identity + DID derived from the mnemonic; DHT advertise. |
| `internal/registrar` | On-chain registration interface (no-op in v2.0; Solana later). |
| `internal/node` | Core service tying store + wg + stealth together; builds credential bundles. |
| `internal/api` | Gin REST surface under `/api/v2` + Prometheus `/metrics`. |
| `internal/telemetry` | Structured logging + metrics. |

## Stealth topology ("WireGuard is the endpoint")

When WireGuard's UDP is throttled or DPI-blocked, the **same** WireGuard tunnel is
wrapped in a carrier that looks like ordinary internet traffic:

```
client ──WireGuard(UDP)──────────────────────────────▶ :51820  (fast path)

client ──WG inside VLESS+REALITY(TCP:8443)──┐
                                            ├─▶ sing-box ─▶ 127.0.0.1:51820 ─▶ WireGuard
client ──WG inside Hysteria2(QUIC:4443)─────┘
```

Key properties:

- **One shared carrier secret per node.** REALITY keypair/short-id, the VLESS UUID,
  the Hysteria2 password and a self-signed Hysteria2 cert are generated once and
  persisted in SQLite `node_settings`.
- **Not an open proxy.** The carriers' `direct` outbound is pinned to
  `127.0.0.1:<wg-port>`, so a carrier connection can only ever reach the local
  WireGuard listener — never arbitrary internet hosts.
- **Auth stays in WireGuard.** The shared secret only gets you to the WG door; you
  still need a registered WireGuard key to get a tunnel. No per-peer sing-box user
  management, so the sing-box instance never restarts on peer churn.

The credential bundle a client receives therefore contains the WireGuard config
**and** the carrier share URIs + a complete sing-box client profile that nests
WireGuard inside the chosen carrier.

## Build tag

The sing-box REALITY *server* is gated behind `with_reality_server`. The binary
**must** be built with it (`make build`, the Dockerfile, and CI all set it) or the
REALITY inbound fails to start at runtime.
