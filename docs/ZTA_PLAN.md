# Zero-Trust Architecture — node work order (`feat/zta-plan`)

Companion work orders on the same branch name:
`erebrus-gateway/docs/ZTA_PLAN.md`, `erebrus-vpn/docs/ZTA_HANDOVER.md`,
`erebrus-mcp/docs/ZTA_PLAN.md`. Master narrative:
`erebrus-vpn/docs/ZERO_TRUST_PLAN.md`.

Of the seven tracks, three touch this repo: **T3** (bootstrap carrier), **T4**
(Shield policy + DoH), **T5** (on-chain registration). Two deliberately do not:

- **T2 two-hop needs no node change.** `internal/netcidr/cidr.go:129-131`
  already installs `FORWARD` + `MASQUERADE` for the whole client subnet, so an
  entry node already forwards UDP to a second node's `:51820`. Nesting happens in
  the client's sing-box config. Resist the urge to add a "relay mode".
- **T6 agent egress** is an `erebrus-mcp` + gateway concern; a node sees an agent
  as an ordinary WireGuard peer.

**No new env vars.** `GATEWAY_URL` (`internal/config/config.go:131`),
`CHAIN_REGISTRATION` (`:169`), `WALLET_CHAIN` (`:136`), `SHIELD_ADMIN_URL`,
`FIREWALL_DNS_ADDR` and `WG_DNS` already cover everything below.

---

## T3 — A destination-restricted bootstrap carrier

### The constraint to preserve

`internal/stealth/stealth.go` → `serverOptions()` pins **every** carrier
connection to `127.0.0.1:<wg-port>` with a `RouteOptionsActionOptions` override.
That pin is what stops the node being an open proxy for anyone holding the shared
carrier secret. It stays. We add exactly one hole, and we make the hole useless
for anything but reaching our own control plane.

### Changes

- `internal/stealth/secrets.go` — a second credential, `keyVLESSBootstrapUUID`,
  created alongside the existing ones in `loadOrCreateSecrets`. Rotated by
  `RotateAllSecrets` like the rest.
- `internal/stealth/stealth.go`:
  - `vlessInbound()` gains a second `option.VLESSUser{Name: "bootstrap"}`.
  - `serverOptions()` inserts a rule **before** the override rule: match
    `AuthUser: ["bootstrap"]` **and** `Domain: [host(cfg.GatewayURL)]` **and**
    `Port: [443]` → route `direct`. Everything else from that user still falls
    through to the override, so a leaked bootstrap credential buys a proxy to
    the Erebrus gateway and nothing else.
  - Skip the rule entirely when `GatewayURL` is empty (`GatewayEnabled()` is
    already there at `internal/config/config.go:333`) — a standalone node should
    not carry a hole to a gateway it does not use.
- Hysteria2: leave it out of v1. VLESS/REALITY on tcp/443 is the carrier that
  survives the most aggressive filtering, and one hole is easier to reason about
  than two.
- `internal/gatewayclient/messages.go` — report the bootstrap UUID in `hello`
  so the gateway can publish descriptors. This is a `docs/ws-protocol.md` change
  (FROZEN v2.0): edit that doc in the gateway repo first, then
  `internal/nodehub/messages.go` there, then this mirror. Unknown fields are
  dropped by the gateway's re-marshal, so a node-only change is silently a no-op.
- Tests: extend `internal/stealth/stealth_test.go` — assert the rule order
  (bootstrap rule strictly before the override), that a non-gateway destination
  from the bootstrap user still gets overridden to the WG port, and that an empty
  `GATEWAY_URL` emits no bootstrap rule at all.

---

## T4 — Shield: make org policy real, then serve DoH

### 4a. `syncShield` currently throws the policy away

`internal/firewall/client.go:108`:

```go
func (c *Client) syncShield(ctx context.Context, _ SyncPayload) error {
	…
	return c.postAuth(ctx, base+"/control/cache_clear", []byte(`{}`))
}
```

The payload parameter is literally discarded. Every gateway-managed firewall
rule for a Shield node has been a no-op. Sentinel has the translation
(`internal/sentinel/policy/policy.go:114-145`); Shield has none.

**The gateway side is already done.** `store.BuildFirewallSyncPayload`
(`internal/store/firewall_policy.go:30`) builds a fully populated payload and
`internal/api/firewall.go:121` sends it: rules normalised to the actions
`block` / `allow` / `rewrite`, with `upstream_resolver_config` rules already
split out into `Upstreams`. So this fix needs **no gateway change at all** — the
data has been arriving and getting dropped.

Implement the AdGuard translation, mirroring the Sentinel switch case-for-case
so the two providers cannot drift:

| Incoming | AdGuard |
| --- | --- |
| action `block` | `\|\|domain^` via `POST /control/filtering/set_rules` |
| action `block`, target starts `*.` | strip the `*.`, same `\|\|domain^` (AdGuard's `\|\|` already covers subdomains) |
| action `allow` | `@@\|\|domain^` |
| action `rewrite` | `POST /control/rewrite/add` (`/control/rewrite/delete` for removals), value in `SyncRule.Value` |

Note the wildcard case is **not** distinguishable by action: the gateway
normalises both `domain_block` and `wildcard_domain_block` to `"block"` and
keeps the distinction in `rule_type`. Sentinel detects it from the `*.` target
prefix (`internal/sentinel/policy/policy.go:124`) — do the same here rather than
switching on `rule_type`, so the two providers behave identically on the same
input. Accept the long-form action spellings defensively too, as Sentinel does.

- `set_rules` replaces the whole custom-rule set, so build the full list from the
  payload every sync and send once. That makes the node convergent: whatever the
  gateway says is the complete truth, and a rule deleted upstream disappears
  here. Keep the cache_clear afterwards.
- Apply `p.Upstreams` through the existing `/control/dns_config` path. The
  stock-upstream guard in `ensureShieldUpstreams` stays as-is — it only fires
  when nobody has chosen resolvers, which is still the right behaviour.
- Report the applied `policy_version` in the command result (gateway adds the
  field) so the dashboard can show drift rather than assume success.
- Extend `internal/firewall/client_test.go` with a fake AdGuard asserting the
  emitted rule text per action, including the wildcard strip and the
  rules-are-replaced-not-appended property.

### 4b. Default protection out of the box

Extend `ConfigureAdmin` to seed blocklists via `/control/filtering/add_url` on
first configure, so a Shield node with no org rules still blocks malware and ads.
Today a fresh Shield node filters nothing until someone writes a rule.

### 4c. DoH on the node

**New: `POST|GET /api/v2/dns-query`** (RFC 8484) in `internal/api/`. Two callers,
one handler:

- The **gateway relay** (`/api/v2/doh/:key` there) calls it over the existing
  PASETO node auth — put it in the `authed` group in
  `internal/api/server.go:119`, purpose `dns_query` to match the Drop pattern.
- **In-tunnel clients** call it directly at the tunnel gateway IP.

Implementation is small: decode the wire-format query, exchange with
`cfg.FirewallDNSAddr` using `miekg/dns` (already a dependency), return
`application/dns-message`. Lift the exchange out of `internal/dns/forwarder.go`
into a shared helper rather than copying it.

**The no-app-update win**: sing-box accepts `https://host/dns-query` as a DNS
server address, and the client passes `bundle.dns` straight through to its config
builder. So a Shield node setting `WG_DNS` to its own DoH URL upgrades existing
clients with no app release. Two caveats before relying on it:

1. The client pins sing-box 1.11.15, which **rejects unknown DNS fields at config
   decode**. Validate the resulting client config with
   `bin/sing-box/<os>/sing-box check` in the app repo before shipping a node that
   advertises a DoH URL.
2. TLS inside the tunnel can use the self-signed cert from
   `internal/stealth/tlscert.go`, but the client must be told to trust it —
   confirm how 1.11.15 wants that expressed before committing to the design.
   If it is awkward, plain DNS to the tunnel gateway is not a real exposure
   (it never leaves the tunnel), and the public relay in 4b carries the value.

ODoH (node as target, gateway as relay) is the follow-up: key config in the
settings store, same pattern as `internal/stealth/secrets.go`, no env var.

---

## T5 — On-chain node registration

Everything needed is already stubbed. `internal/registrar/registrar.go` defines
the interface and freezes `NodeIdentity`; `CHAIN_REGISTRATION` (`off | solana`)
is already read; `internal/wallet/wallet.go` already derives and signs on both
Solana and EVM from the node mnemonic.

- Accept `ethereum` alongside `solana` in `registrar.New`, matching
  `wallet.ResolveChain`'s aliases. Keep `off` the default.
- **The node does not submit transactions and does not hold gas.** It signs the
  `NodeIdentity` payload and sends the signature to the gateway, which submits as
  fee payer. Requiring operators to fund a wallet before their node can join
  would be the single biggest onboarding regression we could ship.
  So the "solana"/"ethereum" registrar implementations are: canonicalise the
  payload, sign with the node wallet, hand it to `internal/gatewayclient`, and
  record the returned tx hash locally for `erebrus status`.
- Payload canonicalisation must be byte-identical to what the contract verifies.
  Define it once (a `Canonical()` method on `NodeIdentity` returning the exact
  preimage) and reference that function from the contract tests in the gateway
  repo. A mismatch here is a silent "signature invalid" at 3am.
- `UpdateStatus` maps to `setStatus` on the registry; throttle it — status flaps
  should not become transactions. Only `online ⇄ draining ⇄ deregistered` go on
  chain, never a heartbeat.
- `HashIP` already exists and is the right thing to publish (`sha3-256` of the
  public IPv4). Never publish the raw IP.
- Tests: `internal/registrar/registrar_test.go` — canonical preimage is stable
  across runs and field orders, unknown mode still falls back to noop with a
  warning, and `off` emits nothing.

---

## Order of work

1. **T4a** — the `syncShield` fix. Smallest change, closes a silently-broken
   feature, and nothing else in T4 is worth anything without it.
2. **T4b** default blocklists.
3. **T3** bootstrap carrier (coordinate the ws-protocol edit with the gateway).
4. **T5** registrar signing.
5. **T4c** DoH, then ODoH.
