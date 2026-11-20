# Changelog

Dates are the day the work landed on `main`. Entries under **Unreleased** are on
`feat/zta-plan` and not merged yet.

## Unreleased — zero-trust environment

### TL;DR

- Shield applies the gateway's firewall snapshots and acknowledges them, and
  seeds default blocklists the first time it comes up.
- The VLESS bootstrap carrier can only reach the configured gateway, and the
  credential it retires at rotation keeps working for a grace window so a
  blocked client can still collect its replacement.
- A node can anchor its identity on chain without paying anything: it signs a
  consent proof and the gateway submits and pays for the write. Off by default.
- Carrier rotation recovers from a restart that could not bind, instead of
  leaving the carrier down while reporting success.

### Added

- **Opt-in on-chain registration.** `CHAIN_REGISTRATION=on` lets the node
  publish its identity — peer id, DID, IP hash, region and operator wallet — to
  the registry. The node requests a challenge from the gateway, rebuilds every
  identity field from its own configuration and signs only a proof that
  describes this node; it never signs bytes it was handed, because this key
  anchors its identity on chain. The gateway submits the transaction and pays
  the fee, so the operator wallet needs no balance. A two-letter `REGION` is
  required, and `install.sh` asks once during setup.
- **Signed uptime attestations.** While registration is on, the node signs one
  observation per five-minute bucket over the control channel. The gateway keeps
  one per bucket and commits only an epoch root on chain.
- **Registration memory.** The outcome of the last sponsored registration is
  persisted against a fingerprint of the identity it covered, so a node that
  reconnects does not spend the operator's sponsorship budget again. The
  fingerprint changes with the identity, which is also when re-registering is
  the point.
- **Restricted bootstrap carrier.** The carrier publishes a second VLESS
  credential whose route is pinned to the configured gateway, so a client that
  has nothing but that credential can reach the gateway API and nothing else.
- **Shield default blocklists** are seeded on first setup, so a fresh node
  filters sensibly before an operator configures anything.

### Changed

- `CHAIN_REGISTRATION` is now a plain opt-in switch (`off` by default, or
  `on`/`true`/`1`/`yes`) rather than naming a chain; the chain always follows
  `WALLET_CHAIN`. An existing node that upgrades keeps its current behaviour
  until the operator turns it on. **No new environment variables.**
- Carrier rotation takes a grace period and keeps the retired bootstrap
  credential valid for it. A client whose only route to the gateway is that
  carrier cannot fetch a replacement descriptor once its credential stops being
  accepted, and the audit archive holds hashes only, so it cannot serve one. The
  retired identity is routed exactly like the live one — to the gateway, not
  WireGuard — and only one generation ever overlaps.

### Fixed

- **A failed restart left the carrier down while rotation reported success.**
  Restarts were keyed on the live sing-box instance, which a start that could not
  bind had already cleared, so every later attempt became a no-op. The manager
  tracks whether the carrier should be listening and the next rotation retries
  it.
- **Shield did not apply or acknowledge gateway firewall snapshots.** The node
  now translates a snapshot into AdGuard rules and rewrites, applies it
  atomically and reports the applied version back, so the gateway can tell drift
  from a policy that is in place.
