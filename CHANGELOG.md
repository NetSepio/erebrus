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
  descriptor valid for it. A client whose only route to the gateway is that
  carrier cannot fetch a replacement once its own stops being accepted, and the
  audit archive holds hashes only, so it cannot serve one. The descriptor is
  kept whole — the credential *and* the REALITY short-id it was published with —
  routed exactly like the live one, to the gateway rather than WireGuard, and
  only one generation ever overlaps. A short-id rotation on its own now retires
  the outgoing descriptor the same way, so a command meant to change an
  identifier does not lock out the clients the carrier exists for.

### Fixed

- **A rotated bootstrap descriptor was unusable.** Rotation kept the retired
  credential but replaced the REALITY short-id it had been published with, and
  the short-id is checked during the handshake — before VLESS authentication is
  reached at all. So the grace window held a credential that could not connect,
  which is the one case it exists for: the client that has no other way to
  collect the new descriptor.
- **The grace window did not expire on the running listener.** A retired
  descriptor was dropped only the next time something rebuilt the carrier, which
  in practice meant the next rotation — days past the deadline it was retired
  with. The deadline now rebuilds the listener itself.
- **A reconnect asked the gateway to sponsor a second registry write.** The
  nonce of a registration in flight was per connection, so reconnecting started
  over: the gateway sponsored another paid write of the same facts and charged
  it to the operator's allowance of six a day, which a handful of reconnects
  could exhaust — leaving the node unable to register at all. The nonce is now
  kept with the identity it covers, across connections and restarts, and a
  reconnecting node asks after that job instead.
- **A failed rotation dropped the descriptor clients were holding.** A rotation
  writes the new short-id before the listener restarts, so a restart that cannot
  bind leaves a credential the network never saw. Retiring that one on the next
  rotation discarded the descriptor still in clients' hands — inside its own
  grace window — so a successful retry locked out exactly the blocked clients
  the window exists for. What gets a window is now the descriptor the listener
  actually served.
- **A restart after a failed rotation dropped the published descriptor.** Which
  descriptor the listener last bound with lived only in memory, so a process
  that came back after a rotation that could not bind read the unpublished
  credential as the served one — and retired it on the next rotation, dropping
  the descriptor clients were holding inside its grace window. It is recorded in
  the node's own store now, and written on every successful start — and only
  there. A rotation used to record its credentials as served once the restart
  returned, and a restart is a no-op for a carrier that is deliberately down:
  the rotate-carriers tool opens a manager over the node's store without a
  listener of its own, so two rotations through it put a descriptor no listener
  had ever published into the grace window and dropped the one clients were
  really holding.
- **A reconnect after a failed gateway lookup spent another sponsorship.** The
  gateway answered `unavailable` both for a sponsorship it has no record of and
  for a lookup it could not complete, and the node discarded its nonce on
  either. A transient database error therefore had the node ask for a second
  paid write of a registration already queued. A lookup that failed now answers
  `deferred`, which the node keeps polling.
- **A failed restart left the carrier down while rotation reported success.**
  Restarts were keyed on the live sing-box instance, which a start that could not
  bind had already cleared, so every later attempt became a no-op. The manager
  tracks whether the carrier should be listening and the next rotation retries
  it.
- **Shield did not apply or acknowledge gateway firewall snapshots.** The node
  now translates a snapshot into AdGuard rules and rewrites, applies it
  atomically and reports the applied version back, so the gateway can tell drift
  from a policy that is in place.
