# Erebrus Node — Security & Data-Capture Audit (v2.0)

_Scope: the `erebrus` node (this repo) and its trust boundaries with clients,
the gateway, and the host. Last reviewed 2026-06-26 against the v2 codebase.
This is an internal pre-release review, not a third-party pentest — an external
audit is still recommended before a large public launch._

---

## 1. Architecture & trust boundaries

```
  client ── WireGuard / VLESS+REALITY / Hysteria2 ──▶ NODE ──▶ internet
                                                       │
                          HTTPS + WebSocket (control)  ▼
                                                    GATEWAY
```

| Boundary | Carries | Trust |
|---|---|---|
| client ↔ node (data plane) | the user's traffic | end-to-end via WireGuard keys; node is the exit |
| gateway → node (`/api/v2/peers`) | provisioning + credential bundles | bearer `NODE_API_TOKEN` |
| node → gateway (WS) | identity, heartbeat, per-client byte deltas | node PASETO |
| node ↔ host | SQLite DB, `config.env`, WG kernel iface | host root |

The node is the **exit point**: it necessarily sees the source (client) and can
observe destination IPs of forwarded packets at the network layer. The design
goal is to **store and transmit as little of that as possible**.

---

## 2. Data-capture inventory (privacy posture)

### Stored on the node (SQLite, `STATE_DIR/erebrus.db`)
- **Per peer:** id, name, wallet address, WG public key, assigned tunnel IP,
  WG preshared key, generated proxy UUID/password, timestamps, expiry.
- **Node settings:** WG server private/public key, REALITY private/public key +
  short-id, VLESS UUID, Hysteria2 password, Hysteria2 self-signed cert + key.

### NOT stored (by design)
- No traffic content, destination IPs/domains, connection logs, or DNS queries.
- No per-flow records. The node keeps **no activity log**.

### Transmitted to the gateway (authenticated WS)
- Identity (`peer_id`, `did`, `ip_hash`), spec (cpu/mem/region/**raw IP**),
  capabilities, endpoints (+ public keys).
- Heartbeat: cpu/mem %, **cumulative interface rx/tx**, self-speedtest.
- `usage_report` (60s): **per-client rx/tx byte deltas + last handshake**, keyed
  by the gateway-issued client UUID.

> **Metadata note (important):** the gateway can join client UUID → wallet, so
> *per-wallet bandwidth and online-time metadata exists at the gateway*, even
> though the node logs nothing. This is inherent to metered DePIN billing.
> It is **traffic metadata, not content or destinations**. Document this in the
> user-facing privacy policy. If stronger privacy is wanted later, aggregate
> usage before it leaves the node.

### Logs (slog JSON → stderr)
- Node identity and operational warnings/errors only. Tokens, client keys, and
  request bodies are **not** logged. Internal error strings are no longer echoed
  to API clients (see F4).

### Third parties
- **DNS** defaults to `1.1.1.1` (Cloudflare) → see F5.

---

## 3. Open findings

Severity: 🔴 high · 🟠 medium · 🟡 low. Only items that still need operator action,
roadmap work, or explicit acceptance.

| # | Severity | Finding | Action |
|---|---|---|---|
| F3 | 🔴 | Node API + credential bundles served over plaintext HTTP | Operator — TLS or firewall `:9080` to gateway only |
| F5 | 🟠 | DNS sent to a third party (Cloudflare) by default | Operator: local resolver; roadmap: node-internal DNS |
| F6 | 🟠 | Key material at rest unencrypted in SQLite / `config.env` | Operator: FDE, access control; repo: `0600` perms |
| F7 | 🟡 | `/metrics` and `/api/v2/stats` are public (coarse aggregates only) | Operator: firewall scrapers if sensitive |
| F8 | 🟠 | No application-level rate limiting | Operator: reverse proxy / fail2ban / cloud UDP protection |
| F10 | 🟡 | Shared node-wide carrier secret; partial rotation only | Roadmap: full carrier-secret rotation command |
| F11 | 🟡 | Hysteria2 self-signed cert + client `insecure` | Accepted — inner WG payload stays confidential |

### F3 — Plaintext node API (OPERATOR — top priority)
`:9080` is plain HTTP. The `NODE_API_TOKEN` and full credential bundles
(client WG config, share URIs) traverse it in cleartext; an on-path attacker
between gateway and node could steal the token (→ full peer control) or
intercept bundles. **Mitigations:**
- Terminate TLS in front of the node (Caddy/nginx/Cloudflare) **or**
- restrict `:9080` to the gateway only (cloud firewall / private network / a
  management WireGuard link), never exposing it to the public internet.
The installer's preflight opens `:9080`; production deployments should put it
behind TLS or a firewall. _Roadmap: gateway↔node mTLS / PASETO-signed calls._

### F5 — DNS leakage (OPERATOR / ROADMAP)
With `WG_DNS=1.1.1.1`, clients' DNS resolves at Cloudflare. Operators wanting
no third party should run a local resolver and set `WG_DNS` to it; the
node-internal DNS (Phase 5, `miekg/dns`) will make this the default for
app-hosting nodes.

### F6 — Secrets at rest (PARTIALLY MITIGATED)
The DB holds the WG server private key, REALITY key, Hy2 cert key and per-peer
PSKs; `config.env` holds the mnemonic. Host compromise ⇒ node impersonation.
WireGuard's forward secrecy protects *past* sessions (ephemeral session keys),
but a stolen static key lets an attacker impersonate the node going forward.
**Mitigations in repo:** `STATE_DIR` is `0700`; the DB (+WAL/SHM) is now forced
to `0600`; the installer writes `config.env` `0600`. **Operator:** use full-disk
encryption; restrict host access; rotate the mnemonic ⇒ new node identity.

### F7 — Public metrics/stats (BY DESIGN)
`/metrics` (Prometheus) and `/api/v2/stats` (the dashboard's coarse aggregates:
connected count, cumulative bytes, uptime) are unauthenticated. They expose **no
per-client data**. Operators who consider even aggregates sensitive should
firewall `:9080` to trusted scrapers, or front it with auth.

### F8 — No rate limiting (OPERATOR)
There is no app-level throttle on the API or the data-plane listeners. Risks:
token brute-force (bounded by the 401 fail-closed + strong token), address-pool
exhaustion via mass provisioning (requires the node token, held only by the
gateway), and UDP floods on WG/Hy2. **Mitigations:** provisioning is
gateway-gated by entitlement; put a rate-limiting reverse proxy and/or
fail2ban in front; rely on the cloud provider's UDP flood protection.

### F10 — Carrier secret rotation (ROADMAP)
The VLESS UUID / Hy2 password are node-wide and shared with every client. A leak
lets a holder reach the WG door (not the VPN itself). `rotate_reality` rotates
REALITY short-ids, but not the VLESS UUID / Hy2 password — add a full
carrier-secret rotation command.

### F11 — Hysteria2 self-signed TLS (ACCEPTED)
Hy2 uses a self-signed cert; clients connect with `insecure`. An active MITM on
`:4443` sees only the **inner WireGuard-encrypted payload** (the client pins the
node's WG public key from the bundle), so confidentiality holds. REALITY (the
TCP carrier) resists MITM by design.

### Resolved in codebase (no action)

- **F1** — API auth no longer fails open when `NODE_API_TOKEN` is unset (release fails closed).
- **F2** — Token compare uses `crypto/subtle.ConstantTimeCompare`.
- **F4** — API errors are generic; detail logged server-side only.
- **F9** — Stealth `direct` outbound pinned to `127.0.0.1:<wg-port>`; WG auth still required.
- **SQL injection / command injection / IP races / peer name injection** — reviewed safe in v2.

---

## 4. Operator hardening checklist

- [ ] Set a strong `NODE_API_TOKEN` (the installer generates 32 bytes) — never blank.
- [ ] Do **not** expose `:9080` to the public internet: TLS-terminate it, or
      firewall it to the gateway only.
- [ ] Open only what's needed: `51820/udp`, `8443/tcp`, `4443/udp` publicly.
- [ ] Enable full-disk encryption; keep `config.env` and `STATE_DIR` `0600/0700`.
- [ ] Run on a dedicated host/VM; minimise other services.
- [ ] Consider a local DNS resolver (avoid the Cloudflare default).
- [ ] Keep the OS + `wireguard` module patched; rebuild the image for sing-box CVEs.
- [ ] Back up the mnemonic securely; rotating it changes the node identity.

## 5. Release-readiness notes

**Done:** reproducible `-tags with_reality_server` builds; gitleaks CI; `/healthz`;
node self-speedtest on heartbeats (`internal/speedtest`).

**Operator awareness (not blockers):**

- WireGuard teardown on SIGTERM relies on `wg-quick down` / `PostDown` — document for restarts.
- Container needs `NET_ADMIN`; compose uses `cap_add` rather than `privileged`.
