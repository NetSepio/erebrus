# Drop public CID gateway implementation context

This document captures the current design for the optional public CID gateway
that is part of the `erebrus-gateway` frozen mirror. It is intended for the
node and gateway teams when back-porting or cross-checking protocol changes.

## Why replace a boolean with a domain

The previous `DROP_PUBLIC_GATEWAY_ENABLED=true` toggle exposed a raw Kubo
read-only gateway on `8080/tcp`. That required operators to open a second public
port and produced `http://<node-ip>:8080/ipfs/<cid>` URLs, which are hard to
TLS-terminate and unsuitable for browser-based consumers. The new design
requires an explicit DNS hostname and advertises only a reachable HTTPS URL.

## Configuration contract

- `DROP_PUBLIC_GATEWAY_ENABLED` is removed. There is no boolean public gateway.
- `DROP_PUBLIC_GATEWAY_DOMAIN` is an optional string. An empty value means the
  public gateway is disabled and file reads stay behind the authenticated
  Erebrus gateway.
- A non-empty value must be a valid DNS hostname: no scheme, port, path, query,
  fragment, credentials, `localhost`, or IP literal. The node normalizes it to
  lowercase and validates it in `Config.Validate()`.
- The canonical URL is always `https://<domain>`. It is never `http://` and
  never includes `:5001` or `:8080`.

## TLS termination and reverse proxy

- `deploy/compose/drop-public-gateway.yml` adds a pinned `traefik:v3.7.7`
  sidecar.
- Traefik publishes only `443/tcp`.
- It terminates TLS and uses TLS-ALPN-01 ACME to obtain certificates.
- It routes only `Host(<domain>) && PathPrefix(/ipfs/)` to the internal
  `http://kubo:8080` service.
- CORS is handled by a Traefik `headers` middleware with permissive settings:
  `*`, methods `GET`, `HEAD`, `OPTIONS`, `*` allowed headers, and a 300-second
  preflight max-age. `OPTIONS` requests are answered by the middleware.
- The raw Kubo `8080` and `5001` ports are never published to the host.
- `80/tcp` is intentionally not published because TLS-ALPN-01 ACME only
  requires `443/tcp`.

## Kubo hardening stays in place

- `Gateway.NoFetch=true` (no recursive fetching).
- `Gateway.NoDNSLink=true`.
- `Gateway.ExposeRoutingAPI=false`.
- Automatic GC and the existing runtime safeguards (file descriptor limits,
  `pids_limit`, `stop_grace_period`, log rotation) are preserved.

## Reachability gating

The node does not advertise a public gateway URL until it can prove the
endpoint is reachable with valid TLS:

- `internal/drop/gateway.go` contains a deterministic probe CID and a probe
  function that performs a `GET` to `https://<domain>/ipfs/<cid>`.
- The probe uses `InsecureSkipVerify=false` and a `CheckRedirect` that refuses
  to follow redirects (a 3xx is still treated as reachable).
- Any `5xx` response is treated as unreachable, because it means the TLS
  endpoint is up but the backend is not ready.
- `internal/drop/service.go` runs the probe every 30 seconds in a dedicated
  goroutine.
- `Service.PublicGatewayURL()` returns the URL only when the probe is currently
  passing and Kubo is in an operational state (`active`, `degraded`, or `full`).

## Capability contract

- `internal/gatewayclient/messages.go` and `internal/api/server.go` expose the
  gateway as `public_gateway_url` (`string`) with `json:"public_gateway_url,omitempty"`).
- It is omitted when the gateway is disabled, the domain is invalid, or the
  endpoint is not reachable.
- `docs/node-api.openapi.yaml` and `docs/ws-protocol.md` are updated to match.

## Installer and firewall changes

- `install.sh` prompts for a domain instead of a `yes/no` public gateway.
- Unattended installs use `DROP_PUBLIC_GATEWAY_DOMAIN=<domain>` or
  `--drop-public-gateway-domain <domain>`.
- `prepare_drop_firewall()` and `open_firewall()` open `443/tcp` only when a
  domain is configured.
- Preflight checks DNS resolution against the detected public IP and probes
  `443/tcp` for reachability.

## Testing and validation

- Unit tests cover domain normalization, URL construction, `omitempty` JSON
  serialization, and the public-gateway URL gating logic.
- The repository validation checklist includes `gofmt`, `make vet`, `make test`,
  `make build-all`, `bash -n install.sh`, `sh -n deploy/compose/kubo-init.sh`,
  Docker Compose config validation for Standard/Shield/Sentinel/development,
  and OpenAPI parsing.
