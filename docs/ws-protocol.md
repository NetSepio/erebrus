# Erebrus Node ↔ Gateway WebSocket Protocol

This document is the source contract for the hand-mirrored Go message types in
the Erebrus node and gateway repositories. Changes must remain additive within
protocol v2 so older nodes and gateways can ignore fields they do not know.

Every frame uses the envelope:

```json
{"type":"hello","data":{}}
```

## Node hello

The node sends `hello` whenever the WebSocket connects. Drop-capable nodes add
an optional `capabilities.drop` object and a coarse `services.drop` state:

```json
{
  "type": "hello",
  "data": {
    "node_id": "12D3...",
    "version": "2.0.0",
    "capabilities": {
      "access_mode": "public",
      "app_hosting": false,
      "wildcard_domain": "",
      "drop": {
        "enabled": true,
        "accepts_public_uploads": true,
        "public_gateway_enabled": false,
        "webui_available": false
      }
    },
    "services": {
      "vpn": "active",
      "drop": "active"
    }
  }
}
```

`capabilities.drop` is omitted by nodes that do not implement Drop.
`public_gateway_enabled` reports whether direct unauthenticated
`http://<node-ip>:8080/ipfs/<cid>` reads are host-published. When false, file
operations remain available through the authenticated Erebrus gateway.

## Node heartbeat

Drop-capable nodes add an optional `drop` object to `heartbeat`. The Kubo
version is also mirrored in `versions.kubo`, while `services.drop` preserves
the existing coarse service model.

```json
{
  "type": "heartbeat",
  "data": {
    "ts": 1765584000,
    "status": "online",
    "versions": {
      "node": "2.0.0",
      "kubo": "0.42.0"
    },
    "services": {
      "vpn": "active",
      "drop": "active"
    },
    "drop": {
      "state": "active",
      "kubo_version": "0.42.0",
      "repo_size_bytes": 1048576,
      "storage_max_bytes": 10000000000,
      "num_objects": 12
    }
  }
}
```

Drop service states are:

```text
disabled | starting | active | degraded | full | unreachable
```

The gateway uses exact capacity for admission control. Public discovery must
project only eligibility and a coarse `available | low | full` capacity state.
Neither message may contain the Kubo RPC URL, credentials, private keys, or
private organization data.
