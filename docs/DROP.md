# Erebrus Drop operations

Erebrus Drop is an optional Kubo/IPFS sidecar for persistent, content-addressed
storage. It is available with the Standard, Shield, and Sentinel Docker
profiles. Host mode is not supported in v1.

Drop failure affects only storage. WireGuard, stealth carriers, gateway
registration, and overall VPN readiness continue independently.

## Install and configure

The interactive installer asks:

```text
Enable Erebrus Drop (Kubo/IPFS storage)? [y/N]
Allow public CID retrieval directly from this node on 8080/tcp? [y/N]
```

The public CID gateway defaults off. With it disabled, uploads, reads, pin
checks, and unpins remain available only through the authenticated Erebrus
gateway-to-node API.

For unattended installs:

```bash
./install.sh --mode container --profile standard --drop --yes
./install.sh --mode container --profile shield --drop --yes
./install.sh --mode container --profile sentinel --drop --yes
./install.sh --mode container --profile standard --drop --drop-public-gateway --yes
```

The only operator-facing settings are:

```dotenv
DROP_ENABLED=false
DROP_STORAGE_MAX=10GB
DROP_SWARM_PORT=4001
DROP_WEBUI_ENABLED=false
DROP_PUBLIC_GATEWAY_ENABLED=false
```

`DROP_WEBUI_ENABLED=true` is valid only for private nodes. Kubo uses fixed
defaults: service `kubo`, private RPC `http://kubo:5001`, CID gateway
`8080/tcp`, repo path `/var/lib/erebrus-kubo` in the node, and image
`ipfs/kubo:v0.42.0`. Port `8080` is host-published only when
`DROP_PUBLIC_GATEWAY_ENABLED=true`.

Before creating the Kubo container, the installer opens and externally probes
`DROP_SWARM_PORT/tcp`. It also opens and probes `8080/tcp` when direct public
CID retrieval is selected. A confirmed TCP failure aborts the install unless
the operator explicitly uses `--skip-checks`. UDP cannot be reliably probed, so
the installer requires the operator to verify `DROP_SWARM_PORT/udp` in the host
and cloud firewalls.

For repository development:

```bash
DROP_ENABLED=true docker compose --profile drop up -d
```

This keeps direct CID access private. To publish `8080/tcp`:

```bash
DROP_ENABLED=true DROP_PUBLIC_GATEWAY_ENABLED=true docker compose \
  -f docker-compose.yml \
  -f deploy/compose/drop-public-gateway.yml \
  --profile drop up -d
```

Installer-managed deployments use the optional override:

```bash
docker compose --env-file .env -f docker-compose.yml -f drop.yml up -d
```

When public CID retrieval is enabled, append
`-f drop-public-gateway.yml`.

## Network and identity safety

- Publish `DROP_SWARM_PORT` as both TCP and UDP.
- Publish gateway `8080/tcp` only after the operator explicitly enables public
  direct CID retrieval.
- Never publish Kubo admin RPC `5001`.
- The node is the only caller of Kubo admin RPC.
- Kubo gateway `NoFetch` is enabled so the read-only public gateway serves
  locally available blocks instead of acting as an unrestricted recursive
  gateway.
- Kubo gateway DNSLink and `/routing/v1` exposure are disabled. Public port
  `8080` is limited to gateway content handling and does not expose the Kubo
  delegated-routing service.
- Kubo receives a deterministic libp2p identity derived from hardened child
  `m/1'` and domain `erebrus/drop/kubo/v1`.
- The Kubo PeerID is intentionally different from the Erebrus node PeerID.
- The serialized Kubo private key is handed through the shared volume only for
  first-run initialization. It is not written to `.env` or logs and the
  temporary handoff is removed after Kubo installs it.
- Existing Kubo identities are preserved. A mismatch creates
  `.erebrus-identity-conflict`, leaves the existing config unchanged, and keeps
  Drop operations unavailable.
- Kubo anonymous telemetry is disabled in the supplied Compose definitions.

When `DROP_PUBLIC_GATEWAY_ENABLED=true`, pinned content is available directly
by CID without Erebrus authentication:

```text
http://<node-public-ip>:8080/ipfs/<cid>
```

This path is public by design. Keep the option disabled when every read must
pass through Erebrus gateway authorization.

## Persistence, disable, and removal

The Compose volume `kubo_data` contains the Kubo repository, identity, and pins.
Normal upgrades reuse it.

To disable Drop safely, rerun the installer with the same deployment profile:

```bash
./install.sh --mode container --profile standard --no-drop --yes
```

This writes `DROP_ENABLED=false` and stops Kubo without removing its container
data. Do not use `docker compose down -v`; that can delete both Erebrus and Kubo
volumes.

Permanent Kubo removal is a separate destructive operation:

1. Disable Drop and back up any required CIDs.
2. Find the exact volume:
   ```bash
   docker volume ls --filter label=com.docker.compose.volume=kubo_data
   ```
3. Confirm no Kubo container uses it, then explicitly remove only that volume:
   ```bash
   docker volume rm <exact-kubo-volume-name>
   ```

Never delete only `/data/ipfs/config` to resolve an identity conflict. Either
restore the mnemonic associated with the existing volume or deliberately
remove the entire Kubo volume after accepting loss of its identity and pins.

Kubo runs with automatic garbage collection enabled. Every 30 minutes it checks
the repository and reclaims unpinned blocks after usage reaches 80% of
`DROP_STORAGE_MAX`. The limit is a soft Kubo datastore threshold, so operators
must retain disk headroom for repository metadata and container logs. The
supplied sidecar also rotates logs, raises the file-descriptor limit, bounds
process count, and allows two minutes for clean shutdown.

## Status and readiness

Public `GET /api/v2/status` includes:

```json
{
  "capabilities": {
    "drop": {
      "enabled": true,
      "accepts_public_uploads": true,
      "public_gateway_enabled": false,
      "webui_available": false
    },
    "services": {
      "vpn": "active",
      "drop": "active"
    }
  },
  "readiness": {
    "checks": [
      {
        "id": "drop",
        "ok": true,
        "optional": true,
        "detail": "active"
      }
    ]
  }
}
```

Drop states are:

| State | Meaning |
|-------|---------|
| `disabled` | Operator disabled Drop |
| `starting` | Identity is prepared and Kubo health is pending |
| `active` | Kubo RPC and repository stats are healthy |
| `degraded` | Identity initialization or repository statistics failed |
| `full` | Repository usage reached the configured storage maximum |
| `unreachable` | Kubo RPC cannot be reached |

The Drop readiness check is always optional. `degraded`, `full`, or
`unreachable` can change Drop behavior but cannot make VPN readiness false.
New uploads are accepted only while Drop is `active`; reads, pin inspection,
and cleanup remain available in `degraded` or `full` state when Kubo is still
reachable.

Gateway-private `GET /api/v2/drop/status` returns Kubo version, repository size,
storage maximum, object count, and the same state.

## Node-private API

All Drop routes require both:

1. `X-Erebrus-Node-Key` with the node key.
2. A gateway-issued PASETO bearer token targeted to this node and carrying the
   exact purpose listed below.

Debug mode does not bypass exact-purpose Drop authorization.

| Method and path | Purpose | Behavior |
|-----------------|---------|----------|
| `GET /api/v2/drop/status` | `drop_status` | Current Drop health and capacity |
| `PUT /api/v2/drop/uploads/{upload_id}` | `drop_upload` | Stream, verify, add, and recursively pin |
| `GET /api/v2/drop/objects/{cid}` | `drop_read` | Stream one object |
| `GET /api/v2/drop/pins/{cid}` | `drop_pin_check` | Check recursive pin state |
| `DELETE /api/v2/drop/pins/{cid}` | `drop_unpin` | Remove a recursive pin idempotently |
| `/api/v2/drop/webui[/…]` | `drop_webui` | Private reverse proxy to Kubo WebUI |

Uploads use `Content-Type: application/octet-stream` and require
`X-Erebrus-Declared-Size`. `X-Erebrus-SHA256` is an optional lowercase or
uppercase hexadecimal SHA-256 digest. Upload and download bodies are streamed,
not buffered, and the v1 single-object maximum is 1,000,000,000 bytes. CIDs are
validated before Kubo calls.

The WebUI proxy removes bearer, node-key, origin, and referrer headers before
forwarding. API errors are generic and do not return raw Kubo internals.

## Metrics

The public Prometheus endpoint `/metrics` adds:

| Metric | Labels | Meaning |
|--------|--------|---------|
| `drop_uploads_total` | `result`, `scope` | Upload attempts by outcome |
| `drop_upload_bytes_total` | `scope` | Bytes successfully uploaded |
| `drop_download_bytes_total` | `scope` | Bytes successfully streamed to callers |
| `drop_node_operations_total` | `operation`, `result` | Upload, read, pin-check, and unpin outcomes |

These metrics contain counts and byte totals only. They do not expose CIDs,
upload IDs, authorization values, private keys, or organization data.

## Troubleshooting

```bash
docker compose --env-file .env -f docker-compose.yml -f drop.yml ps
docker compose --env-file .env -f docker-compose.yml -f drop.yml logs -f kubo
# Only when DROP_PUBLIC_GATEWAY_ENABLED=true:
curl -I "http://127.0.0.1:8080/ipfs/<cid>"
curl -s http://127.0.0.1:9080/api/v2/status | \
  jq '.capabilities.drop, .capabilities.services.drop, (.readiness.checks[] | select(.id == "drop"))'
```

- `starting`: Kubo may be waiting for the identity handoff or completing repo
  initialization.
- `unreachable`: inspect Kubo health and the shared Compose network.
- `full`: unpin data through the authorized API and allow automatic GC to
  reclaim it, or increase `DROP_STORAGE_MAX` and recreate Kubo.
- `degraded` with an identity conflict: verify the mnemonic and volume pairing;
  the node will not overwrite the existing Kubo identity.
