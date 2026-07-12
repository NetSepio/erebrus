# Erebrus Drop implementation context

This document records the architecture decisions, invariants, and validation
baseline for the Drop/Kubo node implementation merged in
[PR #36](https://github.com/NetSepio/erebrus/pull/36) as merge commit
`af0b0c7d4d60a8b3dc9a32704447485e4e30c6e6`. Use
[`DROP.md`](DROP.md) for operator procedures and this document when changing or
integrating the implementation.

## Architecture

```text
Erebrus gateway
        |
        | authenticated node-private API
        v
erebrus-node ---- internal HTTP RPC ----> kubo:5001
                                         |
                                         +-- optional public gateway :8080
                                         +-- public swarm :4001/tcp+udp
```

- Kubo is an optional `ipfs/kubo:v0.42.0` sidecar in the same Compose network
  as the node.
- Kubo RPC `5001/tcp` is Compose-internal and must never be host-published.
- The swarm port defaults to `4001/tcp+udp` and is published whenever Drop is
  enabled.
- Direct CID retrieval on `8080/tcp` is an independent, explicit opt-in.
- Kubo failure degrades Drop only; it does not make VPN readiness fail.
- The `kubo_data` volume persists the Kubo repository, identity, blocks, and
  pins across restarts, upgrades, and Drop disablement.

## Public retrieval decision

The safe default is:

```dotenv
DROP_ENABLED=true
DROP_PUBLIC_GATEWAY_ENABLED=false
```

In this mode, end users upload, read, pin-check, and unpin through the
authenticated Erebrus gateway-to-node API. The host does not publish Kubo
gateway port `8080`.

Operators can intentionally enable public CID reads with:

```bash
./install.sh --mode container --drop --drop-public-gateway --yes
```

This adds `deploy/compose/drop-public-gateway.yml`, opens and probes
`8080/tcp`, and makes locally available content readable without Erebrus
authentication:

```text
http://<node-public-ip>:8080/ipfs/<cid>
```

Uploads, pin checks, unpins, metadata authorization, and quota decisions remain
authenticated gateway operations even when direct reads are public.

## Security invariants

- Never publish Kubo RPC `5001`.
- Never reuse the Erebrus libp2p private key as the Kubo identity.
- Keep `Gateway.NoFetch=true` so `8080` does not become an unrestricted
  recursive gateway.
- Keep `Gateway.NoDNSLink=true` because the public contract is CID-based.
- Keep `Gateway.ExposeRoutingAPI=false`; `/routing/v1` must not be exposed on
  the public gateway.
- Treat CID possession and public `8080` access as retrieval, not
  authorization.
- Encrypt private content client-side before adding it to Drop.
- Protect the node API on `9080/tcp` with TLS or gateway-only network access.
- Apply bandwidth limits, abuse controls, and takedown procedures before
  enabling public `8080` on production nodes.

## Identity contract

The Erebrus PeerID derivation is unchanged. Kubo uses a separate deterministic
identity derived from the same mnemonic with:

```text
hardened child: m/1'
domain:         erebrus/drop/kubo/v1
```

For the standard deterministic test mnemonic:

```text
Erebrus PeerID: 12D3KooWHXVETqmop8y1iD6XHZujC64269bg1qmwkpDaygtYZMH8
Kubo PeerID:    12D3KooWRsWLnKUXEbV7yqXDUDu9cBdYFGktUrwJXPW1z9T1eSqR
```

The serialized Kubo private key is handed to the persistent volume during
first-run initialization, is not written to `.env` or logs, and is removed
after installation. Existing identities are preserved. A mismatch fails Drop
initialization without replacing the persisted Kubo identity.

## Node API contract

All Drop routes require the node key and an exact-purpose gateway PASETO.

| Purpose | Route |
|---------|-------|
| `drop_status` | `GET /api/v2/drop/status` |
| `drop_upload` | `PUT /api/v2/drop/uploads/{upload_id}` |
| `drop_read` | `GET /api/v2/drop/objects/{cid}` |
| `drop_pin_check` | `GET /api/v2/drop/pins/{cid}` |
| `drop_unpin` | `DELETE /api/v2/drop/pins/{cid}` |
| `drop_webui` | `/api/v2/drop/webui/*` |

Uploads require `X-Erebrus-Declared-Size`, accept optional
`X-Erebrus-SHA256`, stream `application/octet-stream`, enforce the object limit
during streaming, and add to Kubo with `pin=true`. Reads validate the CID and
enforce the byte limit while streaming `ipfs cat`.

The public status and gateway capability contracts expose:

```json
{
  "enabled": true,
  "accepts_public_uploads": true,
  "public_gateway_enabled": false,
  "webui_available": false
}
```

`public_gateway_enabled` reports the operator's host-publication choice; it
does not indicate that Kubo RPC is public.

## Installer bootstrap

An unattended Drop installation requires the node registration token:

```bash
curl -fsSL https://erebrus.io/install.sh | \
  MNEMONIC="..." \
  EREBRUS_NODE_REGISTRATION_TOKEN="ere_reg_..." \
  bash -s -- --mode docker --drop --yes
```

The installer detects the public IP and uses it as `WG_ENDPOINT_HOST`. Set
`WG_ENDPOINT_HOST` explicitly only when the node should advertise a DNS name or
the detected address must be overridden. NAT port forwarding remains an
operator responsibility.

## Long-running Kubo policy

Kubo runs with:

```text
--enable-gc
Datastore.GCPeriod=30m
Datastore.StorageGCWatermark=80
```

The Compose service also supplies:

- persistent storage;
- `restart: unless-stopped`;
- a healthcheck;
- a two-minute graceful shutdown period;
- a 65,536 file-descriptor limit;
- a 512-process limit;
- rotated `json-file` logs capped at three 10 MB files.

`DROP_STORAGE_MAX` is a soft datastore target, not a filesystem quota. Pinned
blocks cannot be reclaimed. The gateway must unpin content when its final
authorized reference is deleted, and operators must leave disk headroom for
uploads, compaction, and garbage collection.

## Gateway integration responsibilities

The separate Erebrus gateway must:

- reserve quota atomically before upload;
- make upload IDs idempotent;
- authorize reads against object metadata before issuing `drop_read`;
- reference-count duplicate CIDs;
- perform compensating unpins after failed metadata transactions;
- issue bounded-lifetime exact-purpose tokens;
- distinguish private authenticated reads from optional public CID reads.

These metadata, quota, billing, and authorization responsibilities are outside
this node repository.

## Validation baseline

The merged implementation passed:

```text
make vet
make test
make build-all
bash -n install.sh
bash -n scripts/install.sh
sh -n deploy/compose/kubo-init.sh
git diff --check
```

Compose configuration was validated for Standard, Shield, Sentinel, and
development profiles in both private and public-gateway modes.

The real Kubo lifecycle validation covered:

- authenticated streaming upload and pin;
- authenticated node read;
- private mode with no host `8080`;
- public direct retrieval through `:8080/ipfs/<cid>`;
- private-only RPC `5001`;
- `/routing/v1` returning `404`;
- persistence across Erebrus and Kubo restart;
- automatic reclamation after unpin and GC;
- Drop degradation without VPN readiness failure.

## Implementation map

- `internal/drop/`: Kubo client, identity derivation, and Drop service.
- `internal/api/drop.go`: authenticated node-private routes.
- `internal/gatewayauth/`: exact-purpose PASETO verification.
- `internal/gatewayclient/`: capability and heartbeat contracts.
- `deploy/compose/drop.yml`: reusable private-by-default sidecar.
- `deploy/compose/drop-public-gateway.yml`: optional `8080` publication.
- `deploy/compose/kubo-init.sh`: identity and Kubo configuration.
- `install.sh`: prompts, preflight, firewall, and Compose selection.
- `docs/node-api.openapi.yaml`: REST contract.
- `docs/ws-protocol.md`: gateway capability contract.
