#!/usr/bin/env bash
#
# Erebrus v2 node installer  —  curl -fsSL https://erebrus.io/install.sh | bash
#
# Deploy mode (how the node runs):
#   container (default) — Docker compose; WireGuard + stealth carriers in a container.
#   host                — bare metal via systemd; supports App-Hosting + wildcard DNS.
#
# Access mode (who can use the node — independent of deploy):
#   private (default) | shared | public
#   All nodes register with the gateway using their access type.
#
# Linux only (x86_64 / arm64). A node needs a STATIC, internet-routable public
# IP, real bandwidth, and open ports — the installer verifies all three.
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Constants / defaults (override via env)
# ---------------------------------------------------------------------------
REPO_URL="${EREBRUS_REPO_URL:-https://github.com/NetSepio/erebrus}"
BRANCH="${EREBRUS_BRANCH:-main}"
INSTALL_DIR="${INSTALL_DIR:-/opt/erebrus}"
STATE_DIR="${STATE_DIR:-/var/lib/erebrus}"
ENV_DIR="/etc/erebrus"
GO_VERSION="${GO_VERSION:-1.23.4}"
BUILD_TAGS="with_reality_server"

# Ports
HTTP_PORT="${HTTP_PORT:-9080}"        # tcp  REST API
WG_PORT="${WG_ENDPOINT_PORT:-51820}"  # udp  WireGuard
STEALTH_TCP_PORT="${STEALTH_TCP_PORT:-8443}"  # tcp  VLESS+REALITY (gateway prod: 443)
STEALTH_UDP_PORT="${STEALTH_UDP_PORT:-4443}"  # udp  Hysteria2 (gateway prod: 443)
# (host + app-hosting also needs 80/tcp + 443/tcp for Caddy)

# Minimum acceptable throughput for an exit node (Mbps)
MIN_DOWN_MBPS="${MIN_DOWN_MBPS:-50}"
MIN_UP_MBPS="${MIN_UP_MBPS:-20}"

# Behaviour toggles
DEPLOY="${EREBRUS_DEPLOY:-}"
ACCESS="${EREBRUS_ACCESS:-}"
PROFILE="${EREBRUS_PROFILE:-}"
ASSUME_YES="${ASSUME_YES:-false}"
SKIP_CHECKS="${SKIP_CHECKS:-false}"

LOG_FILE="/tmp/erebrus-install-$(date +%s).log"

# ---------------------------------------------------------------------------
# Output helpers
# ---------------------------------------------------------------------------
if [[ -t 1 ]]; then
  C_RESET='\033[0m'; C_R='\033[31m'; C_G='\033[32m'; C_Y='\033[33m'; C_B='\033[34m'; C_BOLD='\033[1m'
else
  C_RESET=''; C_R=''; C_G=''; C_Y=''; C_B=''; C_BOLD=''
fi
log()  { echo -e "$*"; echo "$(date '+%F %T') $*" >>"$LOG_FILE"; }
info() { log "${C_B}•${C_RESET} $*"; }
ok()   { log "${C_G}✔${C_RESET} $*"; }
warn() { log "${C_Y}!${C_RESET} $*"; }
err()  { log "${C_R}✘${C_RESET} $*"; }
die()  { err "$*"; echo "  See $LOG_FILE for details." >&2; exit 1; }

# Prompts read from the controlling terminal so they work even when the script
# is piped in (curl … | bash, where stdin is the pipe, not the keyboard). With
# no tty we fall back to defaults — pair with --yes / env vars for unattended use.
TTY="/dev/tty"; [[ -e "$TTY" ]] || TTY=""

confirm() { # confirm "question" [default y/n]
  local q="$1" def="${2:-y}" ans
  $ASSUME_YES && return 0
  [[ -z "$TTY" ]] && { [[ "$def" == "y" ]]; return; }
  local hint="[Y/n]"; [[ "$def" == "n" ]] && hint="[y/N]"
  read -rp "$(echo -e "${C_BOLD}?${C_RESET} $q $hint ") " ans <"$TTY" || true
  ans="${ans:-$def}"
  [[ "$ans" =~ ^[Yy]$ ]]
}
ask() { # ask VARNAME "prompt" "default"
  local __var="$1" __prompt="$2" __def="${3:-}" __in
  if $ASSUME_YES || [[ -z "$TTY" ]]; then printf -v "$__var" '%s' "$__def"; return; fi
  read -rp "$(echo -e "${C_BOLD}?${C_RESET} $__prompt ${__def:+(default: $__def) }") " __in <"$TTY" || true
  printf -v "$__var" '%s' "${__in:-$__def}"
}

banner() {
  echo -e "${C_B}${C_BOLD}"
  cat <<'EOF'
 ____ ____ ____ ____ ____ _  _ ____
 |___ |__/ |___ |__] |__/ |  | [__
 |___ |  \ |___ |__] |  \ |__| ___]   v2 node installer
EOF
  echo -e "${C_RESET}"
}

# ---------------------------------------------------------------------------
# Arg parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode|--deploy) DEPLOY="${2:-}"; shift 2 ;;
    --access) ACCESS="${2:-}"; shift 2 ;;
    --profile) PROFILE="${2:-}"; shift 2 ;;
    --docker|--container) DEPLOY="container"; shift ;;
    --host) DEPLOY="host"; shift ;;
    -y|--yes) ASSUME_YES=true; shift ;;
    --skip-checks) SKIP_CHECKS=true; shift ;;
    --branch) BRANCH="${2:-}"; shift 2 ;;
    -h|--help)
      cat <<'USAGE'
Erebrus v2 node installer

Usage: install.sh [options]
  --mode container|host     Deploy mode (container = Docker; host = bare metal)
  --deploy container|host   Alias for --mode
  --access private|public          Gateway visibility (default: public)
  --profile erebrus|shield|sentinel  Deployment profile (default: erebrus)
  --container | --docker    Shorthand for --mode container
  --host                    Shorthand for --mode host
  -y, --yes                 Non-interactive; accept defaults (pair with env vars)
  --skip-checks             Skip static-IP / bandwidth / port preflight
  --branch <name>           Source branch to build from (default: main)
  -h, --help                This help

Key env overrides: EREBRUS_ACCESS, EREBRUS_DEPLOY, MNEMONIC, WG_ENDPOINT_HOST,
  NODE_NAME, REGION, ZONE (auto for US if unset), EREBRUS_IMAGE, EREBRUS_BUILD_LOCAL,
  GATEWAY_URL, NODE_API_TOKEN, ENABLE_STEALTH,
  REALITY_SERVER_NAMES, HYSTERIA2_OBFS_PASSWORD, ENABLE_APP_HOSTING,
  APP_WILDCARD_DOMAIN, INSTALL_DIR, MIN_DOWN_MBPS, MIN_UP_MBPS
Linux only (x86_64/arm64). Needs a static public IP, bandwidth, and open ports.
USAGE
      exit 0 ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

# ---------------------------------------------------------------------------
# Privilege + platform
# ---------------------------------------------------------------------------
SUDO=""
require_root() {
  if [[ $EUID -ne 0 ]]; then
    command -v sudo >/dev/null 2>&1 || die "run as root (sudo not found)"
    SUDO="sudo"
    info "Using sudo for privileged steps."
  fi
}
run() { $SUDO "$@"; }

PKG=""
detect_platform() {
  [[ "$(uname -s)" == "Linux" ]] || die "Erebrus nodes are Linux-only. Detected: $(uname -s)."
  case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
  if   command -v apt-get >/dev/null 2>&1; then PKG="apt"
  elif command -v dnf     >/dev/null 2>&1; then PKG="dnf"
  elif command -v yum     >/dev/null 2>&1; then PKG="yum"
  elif command -v pacman  >/dev/null 2>&1; then PKG="pacman"
  else die "no supported package manager (apt/dnf/yum/pacman) found"; fi
  ok "Linux/$ARCH detected, package manager: $PKG"
}

pkg_install() {
  info "Installing packages: $*"
  case "$PKG" in
    apt)    run apt-get update -qq >>"$LOG_FILE" 2>&1; run apt-get install -y "$@" >>"$LOG_FILE" 2>&1 ;;
    dnf)    run dnf install -y "$@" >>"$LOG_FILE" 2>&1 ;;
    yum)    run yum install -y "$@" >>"$LOG_FILE" 2>&1 ;;
    pacman) run pacman -Sy --noconfirm "$@" >>"$LOG_FILE" 2>&1 ;;
  esac
}

ensure_tool() { command -v "$1" >/dev/null 2>&1 || pkg_install "${2:-$1}"; }

# ---------------------------------------------------------------------------
# Preflight checks
# ---------------------------------------------------------------------------
PUBLIC_IP=""
detect_public_ip() {
  local svc
  for svc in "https://api.ipify.org" "https://ifconfig.me/ip" "https://icanhazip.com" "https://ipinfo.io/ip"; do
    PUBLIC_IP="$(curl -fsS --max-time 8 "$svc" 2>/dev/null | tr -d '[:space:]' || true)"
    [[ "$PUBLIC_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] && break
    PUBLIC_IP=""
  done
  [[ -n "$PUBLIC_IP" ]] || die "could not determine public IP (no internet?)"
  ok "Public IP: $PUBLIC_IP"
}

check_static_ip() {
  # Heuristic: a directly-attached public IP appears on a local interface.
  # If it doesn't, the host is behind NAT and needs port-forwarding / a static
  # mapping for inbound traffic to reach the node.
  local local_ips
  local_ips="$(ip -o -4 addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 || true)"
  if echo "$local_ips" | grep -qx "$PUBLIC_IP"; then
    ok "Public IP is bound directly to a local interface (not behind NAT)."
  else
    warn "Public IP $PUBLIC_IP is NOT on a local interface — host appears to be behind NAT."
  warn "Inbound traffic will only reach this node if that IP is STATIC and ports"
  warn "$HTTP_PORT/tcp, $WG_PORT/udp, $STEALTH_TCP_PORT/tcp, $STEALTH_UDP_PORT/udp are forwarded here."
    confirm "Continue anyway?" n || die "aborted: a static, routable public IP is required"
  fi
  warn "Note: the installer cannot prove the IP is permanent — make sure it is STATIC,"
  warn "      otherwise the node will drop off the network when the lease changes."
}

# Measure throughput against Cloudflare's speedtest endpoints (no account needed).
check_bandwidth() {
  command -v curl >/dev/null 2>&1 || ensure_tool curl
  info "Measuring bandwidth (this takes a few seconds)…"

  local down_bytes=50000000 up_bytes=20000000 t mbps
  # Download
  t="$(curl -fsS --max-time 30 -o /dev/null -w '%{time_total}' \
        "https://speed.cloudflare.com/__down?bytes=${down_bytes}" 2>/dev/null || echo 0)"
  if [[ "$t" != "0" ]] && awk "BEGIN{exit !($t>0)}"; then
    mbps="$(awk "BEGIN{printf \"%.0f\", ($down_bytes*8)/($t*1000000)}")"
    if (( mbps < MIN_DOWN_MBPS )); then
      warn "Download ~${mbps} Mbps (recommended ≥ ${MIN_DOWN_MBPS} Mbps)."
    else
      ok "Download ~${mbps} Mbps"
    fi
  else
    warn "Could not measure download bandwidth."
  fi
  # Upload (critical for an exit node serving clients)
  t="$(head -c "$up_bytes" /dev/zero | curl -fsS --max-time 30 -o /dev/null -w '%{time_total}' \
        --data-binary @- "https://speed.cloudflare.com/__up" 2>/dev/null || echo 0)"
  if [[ "$t" != "0" ]] && awk "BEGIN{exit !($t>0)}"; then
    mbps="$(awk "BEGIN{printf \"%.0f\", ($up_bytes*8)/($t*1000000)}")"
    if (( mbps < MIN_UP_MBPS )); then
      warn "Upload ~${mbps} Mbps (recommended ≥ ${MIN_UP_MBPS} Mbps for an exit node)."
    else
      ok "Upload ~${mbps} Mbps"
    fi
  else
    warn "Could not measure upload bandwidth."
  fi
}

# Actively verify a TCP port is reachable FROM THE INTERNET: bind a temporary
# listener, then ask check-host.net (multiple external probes) to connect.
check_inbound_tcp() {
  local port="$1" label="$2"
  command -v python3 >/dev/null 2>&1 || { warn "python3 missing; skipping $label inbound check"; return; }
  if ss -lnt 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${port}\$"; then
    warn "Port $port already in use locally; skipping inbound reachability check."
    return
  fi

  python3 - "$port" >>"$LOG_FILE" 2>&1 <<'PY' &
import socket, sys, time
p=int(sys.argv[1]); s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
s.bind(("0.0.0.0",p)); s.listen(8); s.settimeout(25); t=time.time()
while time.time()-t < 25:
    try:
        c,_=s.accept(); c.close()
    except Exception:
        break
PY
  local lpid=$!
  sleep 1

  local rid res
  rid="$(curl -fsS --max-time 10 -H 'Accept: application/json' \
        "https://check-host.net/check-tcp?host=${PUBLIC_IP}:${port}&max_nodes=3" 2>/dev/null \
        | python3 -c 'import sys,json;print(json.load(sys.stdin).get("request_id",""))' 2>/dev/null || true)"
  if [[ -z "$rid" ]]; then
    warn "$label ($port/tcp): external prober unavailable — could not auto-verify. Ensure the port is open."
    kill "$lpid" 2>/dev/null || true; wait "$lpid" 2>/dev/null || true; return
  fi
  sleep 7
  res="$(curl -fsS --max-time 10 -H 'Accept: application/json' \
        "https://check-host.net/check-result/${rid}" 2>/dev/null || true)"
  kill "$lpid" 2>/dev/null || true; wait "$lpid" 2>/dev/null || true

  local connected
  connected="$(echo "$res" | python3 -c '
import sys,json
try: d=json.load(sys.stdin)
except Exception: print("err"); sys.exit()
hit=0
for v in (d or {}).values():
    if isinstance(v,list) and v and isinstance(v[0],list) and v[0] and v[0][0]==1: hit+=1
print(hit)
' 2>/dev/null || echo err)"
  if [[ "$connected" == "err" || -z "$connected" ]]; then
    warn "$label ($port/tcp): inbound check inconclusive — verify the port is open."
  elif (( connected > 0 )); then
    ok "$label ($port/tcp) reachable from the internet ($connected probes)."
  else
    err "$label ($port/tcp) NOT reachable from the internet."
    warn "Open it in your firewall/security-group (and NAT it if applicable)."
    confirm "Continue anyway?" n || die "aborted: required port $port not reachable"
  fi
}

run_preflight() {
  echo; info "${C_BOLD}Preflight checks${C_RESET}"
  detect_public_ip
  if $SKIP_CHECKS; then warn "--skip-checks set: skipping static-IP / bandwidth / port checks."; return; fi
  ensure_tool curl
  command -v ip >/dev/null 2>&1 || pkg_install iproute2 || pkg_install iproute || true
  check_static_ip
  check_bandwidth
  info "Checking inbound port reachability…"
  check_inbound_tcp "$HTTP_PORT" "REST API"
  check_inbound_tcp "$STEALTH_TCP_PORT" "VLESS+REALITY carrier"
  warn "UDP ports $WG_PORT (WireGuard) and $STEALTH_UDP_PORT (Hysteria2) can't be probed reliably —"
  warn "make sure they're open; the installer will add firewall rules where it can."
}

# ---------------------------------------------------------------------------
# Mode selection + configuration
# ---------------------------------------------------------------------------
normalize_deploy() {
  case "$1" in
    docker|container) echo "container" ;;
    host) echo "host" ;;
    *) echo "$1" ;;
  esac
}

choose_deploy() {
  DEPLOY="$(normalize_deploy "$DEPLOY")"
  [[ -n "$DEPLOY" ]] && { ok "Deploy mode: $DEPLOY"; return; }
  echo
  echo -e "${C_BOLD}Choose deploy mode:${C_RESET}"
  echo "  1) container — Docker compose (VPN + stealth). Recommended."
  echo "  2) host      — bare-metal systemd. Adds App-Hosting (wildcard DNS)."
  local c; ask c "Selection [1/2]" "1"
  case "$c" in
    1|docker|container) DEPLOY="container" ;;
    2|host) DEPLOY="host" ;;
    *) die "invalid selection: $c" ;;
  esac
  ok "Deploy mode: $DEPLOY"
}

choose_access() {
  ACCESS="${ACCESS:-private}"
  ACCESS="$(echo "$ACCESS" | tr '[:upper:]' '[:lower:]')"
  [[ -n "$ACCESS" && "$ASSUME_YES" == "true" ]] && { ok "Access mode: $ACCESS"; return; }
  if [[ -n "$ACCESS" && "$ACCESS" != "private" ]]; then
    ok "Access mode: $ACCESS"
    return
  fi
  if $ASSUME_YES; then
    ACCESS="private"
    ok "Access mode: $ACCESS"
    return
  fi
  echo
  echo -e "${C_BOLD}Choose access mode:${C_RESET}"
  echo "  1) private — your devices only (default)"
  echo "  2) shared  — friends via wallet allowlist on gateway"
  echo "  3) public  — open to entitled users on the network"
  local c; ask c "Selection [1/2/3]" "1"
  case "$c" in
    1|private) ACCESS="private" ;;
    2|shared)  ACCESS="shared" ;;
    3|public)  ACCESS="public" ;;
    *) die "invalid selection: $c" ;;
  esac
  ok "Access mode: $ACCESS"
}

choose_profile() {
  if [[ -n "$PROFILE" ]]; then
    case "$PROFILE" in
      erebrus|shield|sentinel) ok "Profile: $PROFILE"; return ;;
      *) die "invalid profile: $PROFILE (use erebrus, shield, or sentinel)" ;;
    esac
  fi
  echo
  echo -e "${C_BOLD}Choose deployment profile:${C_RESET}"
  echo "  1) Erebrus — VPN node only (default)"
  echo "  2) Erebrus Shield — node + AdGuard Home DNS protection"
  echo "  3) Erebrus Sentinel — node + Unbound licensed firewall"
  local c; ask c "Selection [1/2/3]" "1"
  case "$c" in
    1|erebrus) PROFILE="erebrus" ;;
    2|shield)  PROFILE="shield" ;;
    3|sentinel) PROFILE="sentinel" ;;
    *) die "invalid selection: $c" ;;
  esac
  ok "Profile: $PROFILE"
}

# config values
NODE_NAME=""; REGION=""; ZONE=""; WG_ENDPOINT_HOST=""; MNEMONIC="${MNEMONIC:-}"
NODE_API_TOKEN="${NODE_API_TOKEN:-}"; NODE_KEY="${NODE_KEY:-}"
EREBRUS_NODE_REGISTRATION_TOKEN="${EREBRUS_NODE_REGISTRATION_TOKEN:-${EREBRUS_ORG_ENROLLMENT_SECRET:-}}"
GATEWAY_URL="${GATEWAY_URL:-https://gateway.erebrus.io}"
ENABLE_STEALTH="${ENABLE_STEALTH:-true}"; REALITY_SERVER_NAMES="${REALITY_SERVER_NAMES:-www.microsoft.com}"
HYSTERIA2_OBFS_PASSWORD="${HYSTERIA2_OBFS_PASSWORD:-}"
ENABLE_APP_HOSTING="${ENABLE_APP_HOSTING:-false}"; APP_WILDCARD_DOMAIN="${APP_WILDCARD_DOMAIN:-}"
PUBLIC_DOMAIN="${PUBLIC_DOMAIN:-}"; WILDCARD_DOMAIN="${WILDCARD_DOMAIN:-}"
PUBLIC_GATEWAY_ENABLED="${PUBLIC_GATEWAY_ENABLED:-false}"
EREBRUS_BIN=""  # path/way to invoke binary for genmnemonic

rand_token() { head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 32; }

# For US nodes, derive east/west from ipinfo longitude when ZONE is unset.
detect_zone() {
  [[ -n "${ZONE:-}" ]] && return
  local country="${REGION:-}"
  country="$(echo "$country" | tr '[:lower:]' '[:upper:]' | tr -d '[:space:]')"
  [[ "$country" == "US" ]] || return
  local loc lon
  loc="$(curl -fsS --max-time 6 https://ipinfo.io/loc 2>/dev/null | tr -d '[:space:]' || true)"
  [[ "$loc" == *,* ]] || return
  lon="${loc#*,}"
  if awk -v lon="$lon" 'BEGIN { if (lon+0 < -102) exit 0; exit 1 }'; then
    ZONE="west"
  else
    ZONE="east"
  fi
}

gather_config() {
  echo; info "${C_BOLD}Node configuration${C_RESET}"
  REGION="${REGION:-$(curl -fsS --max-time 6 https://ipinfo.io/country 2>/dev/null | tr -d '[:space:]' || echo unknown)}"
  detect_zone
  [[ -n "${ZONE:-}" ]] && ok "Auto-detected zone: ${ZONE} (US east/west from geo)"
  ask NODE_NAME "Node name" "${NODE_NAME:-erebrus-$(hostname -s 2>/dev/null || echo node)}"
  ask ZONE "Zone (optional — e.g. east, west, us-east)" "${ZONE:-}"
  ask WG_ENDPOINT_HOST "Public endpoint host (IP or domain clients dial)" "${WG_ENDPOINT_HOST:-$PUBLIC_IP}"
  ask GATEWAY_URL "Gateway URL" "$GATEWAY_URL"
  ask EREBRUS_NODE_REGISTRATION_TOKEN "Node registration token (ere_reg_* from org owner/admin)" "${EREBRUS_NODE_REGISTRATION_TOKEN:-}"
  [[ -n "$NODE_API_TOKEN" ]] || NODE_API_TOKEN="$(rand_token)"

  case "$DEPLOY" in
    container)
      EREBRUS_MODE=container
      EREBRUS_NETWORK_PROFILE=bridge
      ;;
    host)
      EREBRUS_MODE=host
      EREBRUS_NETWORK_PROFILE=host-network
      ;;
    *) die "invalid deploy mode: $DEPLOY (use container or host)" ;;
  esac
  EREBRUS_ACCESS="${ACCESS:-private}"

  if [[ "$EREBRUS_ACCESS" == "public" ]]; then
    STEALTH_TCP_PORT=443
    STEALTH_UDP_PORT=443
    info "Public access: stealth carriers on 443/tcp and 443/udp for reachability."
  fi

  if [[ "$DEPLOY" == "host" ]]; then
    if confirm "Enable App-Hosting (expose VPN-connected apps to the internet)?" n; then
      ENABLE_APP_HOSTING="true"
      PUBLIC_GATEWAY_ENABLED="true"
      echo "  App-Hosting needs a WILDCARD DNS record you control, e.g.:"
      echo -e "      ${C_BOLD}*.apps.example.com  A  ${WG_ENDPOINT_HOST}${C_RESET}"
      echo "  The gateway then mints per-app CNAMEs under it and routes traffic in."
      ask APP_WILDCARD_DOMAIN "Wildcard base domain (e.g. apps.example.com)" "$APP_WILDCARD_DOMAIN"
      [[ -n "$APP_WILDCARD_DOMAIN" ]] || die "App-Hosting requires a wildcard domain"
      PUBLIC_DOMAIN="$APP_WILDCARD_DOMAIN"
      WILDCARD_DOMAIN="*.${APP_WILDCARD_DOMAIN}"
    fi
  fi
}

# Container image: registry default; local build fallback.
EREBRUS_IMAGE="${EREBRUS_IMAGE:-ghcr.io/netsepio/erebrus:latest}"

# Invoke the node CLI regardless of install mode (pulled/built image vs host binary).
erebrus_cli() {
  if [[ "$DEPLOY" == "container" ]]; then
    run docker run --rm "$EREBRUS_IMAGE" "$@"
  else
    /usr/local/bin/erebrus "$@"
  fi
}

# Generate a mnemonic using the freshly built binary/image if the operator
# didn't supply one. Called after the binary/image is available.
ensure_mnemonic() {
  [[ -n "$MNEMONIC" ]] && { ok "Using supplied mnemonic."; return; }
  info "Generating node identity mnemonic…"
  MNEMONIC="$(erebrus_cli genmnemonic | tr -d '\r')" || die "failed to generate mnemonic"
  [[ -n "$MNEMONIC" ]] || die "empty mnemonic generated"
  ok "Node identity generated (12-word recovery phrase). It is saved securely — BACK IT UP."
}

write_env_file() {
  local f="$1"
  run mkdir -p "$(dirname "$f")"
  run tee "$f" >/dev/null <<EOF
# Erebrus v2 node — generated $(date '+%F %T')
# See .env.example in the repo for field documentation.
RUNTYPE=release
EREBRUS_IMAGE=${EREBRUS_IMAGE}
EREBRUS_PROFILE=${PROFILE:-erebrus}
EREBRUS_ACCESS=${EREBRUS_ACCESS:-private}
EREBRUS_MODE=${EREBRUS_MODE:-container}
EREBRUS_NETWORK_PROFILE=${EREBRUS_NETWORK_PROFILE:-bridge}
SENTINEL_IMAGE=ghcr.io/netsepio/erebrus-sentinel:latest
SERVER=0.0.0.0
HTTP_PORT=${HTTP_PORT}
NODE_NAME=${NODE_NAME}
REGION=${REGION}
ZONE=${ZONE}
MNEMONIC=${MNEMONIC}
NODE_API_TOKEN=${NODE_API_TOKEN}
NODE_KEY=${NODE_KEY:-${NODE_API_TOKEN}}
GATEWAY_URL=${GATEWAY_URL}
GATEWAY_AUTO_REGISTER=true
EREBRUS_NODE_REGISTRATION_TOKEN=${EREBRUS_NODE_REGISTRATION_TOKEN}
WALLET_CHAIN=SOLANA
API_PUBLIC_URL=http://${WG_ENDPOINT_HOST}:${HTTP_PORT}

# WireGuard
WG_CONF_DIR=/etc/wireguard
WG_INTERFACE_NAME=wg0
WG_ENDPOINT_HOST=${WG_ENDPOINT_HOST}
WG_ENDPOINT_PORT=${WG_PORT}
WG_IPv4_SUBNET=10.0.0.1/16
WG_DNS=1.1.1.1
WG_POST_UP=iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o $(default_iface) -j MASQUERADE
WG_POST_DOWN=iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o $(default_iface) -j MASQUERADE

# Stealth carriers (sing-box)
ENABLE_STEALTH=${ENABLE_STEALTH}
STEALTH_TCP_PORT=${STEALTH_TCP_PORT}
STEALTH_UDP_PORT=${STEALTH_UDP_PORT}
REALITY_SERVER_NAMES=${REALITY_SERVER_NAMES}
HYSTERIA2_OBFS_PASSWORD=${HYSTERIA2_OBFS_PASSWORD}

# Public edge / app hosting
ENABLE_APP_HOSTING=${ENABLE_APP_HOSTING}
APP_WILDCARD_DOMAIN=${APP_WILDCARD_DOMAIN}
PUBLIC_DOMAIN=${PUBLIC_DOMAIN}
WILDCARD_DOMAIN=${WILDCARD_DOMAIN}
PUBLIC_GATEWAY_ENABLED=${PUBLIC_GATEWAY_ENABLED}

# Profile / firewall wiring
$(profile_env_block)

# State
STATE_DIR=${STATE_DIR}
CHAIN_REGISTRATION=off
EOF
  run chmod 600 "$f"
  ok "Wrote config: $f"
}

default_iface() { ip route show default 2>/dev/null | awk '/default/{print $5; exit}' || echo eth0; }

profile_env_block() {
  case "${PROFILE:-erebrus}" in
    shield)
      cat <<'PEOF'
FIREWALL_PROVIDER=adguard_home
FIREWALL_DNS_ADDR=adguardhome:53
SHIELD_ADMIN_URL=http://adguardhome:3000
WG_DNS=10.0.0.1
PEOF
      ;;
    sentinel)
      cat <<'PEOF'
FIREWALL_PROVIDER=unbound_erebrus
FIREWALL_DNS_ADDR=erebrus-sentinel:53
SENTINEL_API_URL=http://erebrus-sentinel:8788
WG_DNS=10.0.0.1
PEOF
      ;;
    *)
      echo "FIREWALL_PROVIDER=none"
      ;;
  esac
}

install_compose_file() {
  local dest="$1"
  local name="${PROFILE:-erebrus}"
  local src=""
  if [[ -f "$INSTALL_DIR/deploy/compose/${name}.yml" ]]; then
    src="$INSTALL_DIR/deploy/compose/${name}.yml"
  elif [[ -f "$INSTALL_DIR/src/deploy/compose/${name}.yml" ]]; then
    src="$INSTALL_DIR/src/deploy/compose/${name}.yml"
  else
    info "Fetching compose profile ${name}…"
    curl -fsSL "https://raw.githubusercontent.com/NetSepio/erebrus/${BRANCH}/deploy/compose/${name}.yml" \
      -o "$dest" >>"$LOG_FILE" 2>&1 || die "failed to fetch deploy/compose/${name}.yml"
    return
  fi
  run cp "$src" "$dest"
}

# ---------------------------------------------------------------------------
# Firewall
# ---------------------------------------------------------------------------
open_firewall() {
  local extra_tcp=()
  [[ "$ENABLE_APP_HOSTING" == "true" ]] && extra_tcp=(80 443)
  if command -v ufw >/dev/null 2>&1 && run ufw status >/dev/null 2>&1; then
    info "Opening ports via ufw…"
    run ufw allow "${HTTP_PORT}/tcp"  >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${STEALTH_TCP_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${WG_PORT}/udp"    >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${STEALTH_UDP_PORT}/udp"   >>"$LOG_FILE" 2>&1 || true
    for p in "${extra_tcp[@]}"; do run ufw allow "${p}/tcp" >>"$LOG_FILE" 2>&1 || true; done
    ok "ufw rules added."
  elif command -v firewall-cmd >/dev/null 2>&1; then
    info "Opening ports via firewalld…"
    run firewall-cmd --permanent --add-port="${HTTP_PORT}/tcp"  >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${STEALTH_TCP_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${WG_PORT}/udp"    >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${STEALTH_UDP_PORT}/udp"   >>"$LOG_FILE" 2>&1 || true
    for p in "${extra_tcp[@]}"; do run firewall-cmd --permanent --add-port="${p}/tcp" >>"$LOG_FILE" 2>&1 || true; done
    run firewall-cmd --reload >>"$LOG_FILE" 2>&1 || true
    ok "firewalld rules added."
  else
    warn "No ufw/firewalld detected. Ensure these are open in your cloud security group:"
    warn "  ${HTTP_PORT}/tcp, ${STEALTH_TCP_PORT}/tcp, ${WG_PORT}/udp, ${STEALTH_UDP_PORT}/udp ${extra_tcp:+(+ 80/tcp 443/tcp)}"
  fi
}

enable_ip_forward() {
  echo 'net.ipv4.ip_forward=1' | run tee /etc/sysctl.d/99-erebrus.conf >/dev/null
  run sysctl -p /etc/sysctl.d/99-erebrus.conf >>"$LOG_FILE" 2>&1 || true
}

# ---------------------------------------------------------------------------
# Docker install path
# ---------------------------------------------------------------------------
install_docker_mode() {
  if ! command -v docker >/dev/null 2>&1; then
    info "Installing Docker…"
    curl -fsSL https://get.docker.com | run sh >>"$LOG_FILE" 2>&1 || die "Docker install failed"
  fi
  run systemctl enable --now docker >>"$LOG_FILE" 2>&1 || true
  ensure_tool git

  local skip_build=false
  if [[ "${EREBRUS_BUILD_LOCAL:-false}" != "true" ]]; then
    info "Pulling node image ${EREBRUS_IMAGE}…"
    if run docker pull "$EREBRUS_IMAGE" >>"$LOG_FILE" 2>&1; then
      skip_build=true
      ok "Using registry image ${EREBRUS_IMAGE}"
    else
      warn "Registry pull failed; will build from source (set EREBRUS_BUILD_LOCAL=true to skip pull)."
    fi
  fi

  if ! $skip_build; then
    info "Fetching source ($BRANCH) for image build…"
    if [[ -d "$INSTALL_DIR/.git" ]]; then
      run git -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH" >>"$LOG_FILE" 2>&1
      run git -C "$INSTALL_DIR" checkout -f "$BRANCH" >>"$LOG_FILE" 2>&1
      run git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH" >>"$LOG_FILE" 2>&1
    else
      run mkdir -p "$INSTALL_DIR"
      run git clone --depth 1 -b "$BRANCH" "$REPO_URL" "$INSTALL_DIR" >>"$LOG_FILE" 2>&1
    fi
    info "Building node image (includes -tags ${BUILD_TAGS})…"
    install_compose_file "$INSTALL_DIR/docker-compose.yml"
    ( cd "$INSTALL_DIR" && run docker build -f docker/erebrus-node.Dockerfile -t "$EREBRUS_IMAGE" . >>"$LOG_FILE" 2>&1 ) || \
      ( cd "$INSTALL_DIR" && run docker build -t "$EREBRUS_IMAGE" . >>"$LOG_FILE" 2>&1 ) || die "image build failed"
  else
    run mkdir -p "$INSTALL_DIR"
  fi
  install_compose_file "$INSTALL_DIR/docker-compose.yml"

  ensure_mnemonic
  write_env_file "$INSTALL_DIR/.env"

  info "Starting node via docker compose…"
  local compose="docker compose"
  docker compose version >/dev/null 2>&1 || compose="docker-compose"
  ( cd "$INSTALL_DIR" && run $compose --env-file .env up -d >>"$LOG_FILE" 2>&1 ) || die "docker compose up failed"
  open_firewall
  ok "Docker node started."
}

# ---------------------------------------------------------------------------
# Host (bare-metal) install path
# ---------------------------------------------------------------------------
ensure_go() {
  if command -v go >/dev/null 2>&1; then return; fi
  info "Installing Go ${GO_VERSION}…"
  local tgz="go${GO_VERSION}.linux-${ARCH}.tar.gz"
  curl -fsSL "https://go.dev/dl/${tgz}" -o "/tmp/${tgz}" >>"$LOG_FILE" 2>&1 || die "Go download failed"
  run rm -rf /usr/local/go && run tar -C /usr/local -xzf "/tmp/${tgz}"
  export PATH="$PATH:/usr/local/go/bin"
}

build_host_binary() {
  ensure_tool git
  ensure_go
  info "Fetching source ($BRANCH)…"
  if [[ -d "$INSTALL_DIR/src/.git" ]]; then
    run git -C "$INSTALL_DIR/src" fetch --depth 1 origin "$BRANCH" >>"$LOG_FILE" 2>&1
    run git -C "$INSTALL_DIR/src" reset --hard "origin/$BRANCH" >>"$LOG_FILE" 2>&1
  else
    run mkdir -p "$INSTALL_DIR/src"
    run git clone --depth 1 -b "$BRANCH" "$REPO_URL" "$INSTALL_DIR/src" >>"$LOG_FILE" 2>&1
  fi
  info "Building erebrus (-tags ${BUILD_TAGS}); this can take a couple of minutes…"
  ( cd "$INSTALL_DIR/src" && run env PATH="$PATH:/usr/local/go/bin" \
      go build -tags "$BUILD_TAGS" \
      -ldflags "-X github.com/NetSepio/erebrus/internal/config.Version=2.0.0" \
      -o /usr/local/bin/erebrus-node ./cmd/erebrus-node >>"$LOG_FILE" 2>&1 ) || die "build failed"
  run ln -sf /usr/local/bin/erebrus-node /usr/local/bin/erebrus
  ok "Installed /usr/local/bin/erebrus-node (erebrus alias)"
}

install_host_mode() {
  pkg_install wireguard-tools iptables ca-certificates curl
  command -v modprobe >/dev/null 2>&1 && run modprobe wireguard >>"$LOG_FILE" 2>&1 || \
    warn "Could not load the wireguard kernel module; ensure it is available on this host."
  enable_ip_forward
  build_host_binary
  ensure_mnemonic
  run mkdir -p "$STATE_DIR" /etc/wireguard
  write_env_file "$ENV_DIR/erebrus.env"

  if [[ "$ENABLE_APP_HOSTING" == "true" ]]; then
    info "Installing Caddy for app ingress…"
    if [[ "$PKG" == "apt" ]]; then
      run bash -c 'apt-get install -y debian-keyring debian-archive-keyring apt-transport-https >/dev/null 2>&1; \
        curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg; \
        curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null; \
        apt-get update -qq' >>"$LOG_FILE" 2>&1 || true
      pkg_install caddy || warn "Caddy install failed; install it manually for app hosting."
    else
      pkg_install caddy || warn "Caddy not packaged here; install it manually for app hosting."
    fi
  fi

  info "Installing systemd service…"
  run tee /etc/systemd/system/erebrus.service >/dev/null <<EOF
[Unit]
Description=Erebrus v2 dVPN node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${ENV_DIR}/erebrus.env
ExecStart=/usr/local/bin/erebrus-node
AmbientCapabilities=CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_ADMIN
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  run systemctl daemon-reload
  run systemctl enable --now erebrus >>"$LOG_FILE" 2>&1 || die "failed to start erebrus service"
  open_firewall
  ok "Host node started (systemd: erebrus.service)."
}

# ---------------------------------------------------------------------------
# Post-install
# ---------------------------------------------------------------------------
validate_and_summary() {
  info "Validating node…"
  local out="" i
  for i in $(seq 1 15); do
    out="$(curl -fsS --max-time 4 "http://127.0.0.1:${HTTP_PORT}/api/v2/status" 2>/dev/null || true)"
    [[ -n "$out" ]] && break
    sleep 2
  done
  echo
  if [[ -n "$out" ]]; then
    ok "Node is up. Run: erebrus status (or curl /api/v2/status)"
    echo "$out" | python3 -m json.tool 2>/dev/null || echo "$out"
    if [[ -n "${GATEWAY_URL:-}" ]]; then
      if curl -fsS --max-time 6 "${GATEWAY_URL%/}/healthz" >/dev/null 2>&1; then
        ok "Gateway reachable at ${GATEWAY_URL}"
      else
        warn "Gateway not reachable at ${GATEWAY_URL} — control plane may stay offline"
      fi
    fi
  else
    warn "Node did not answer on :${HTTP_PORT} yet. Check logs:"
    [[ "$DEPLOY" == "container" ]] && echo "    cd $INSTALL_DIR && docker compose logs -f" \
                                    || echo "    journalctl -u erebrus -f"
  fi

  echo
  echo -e "${C_BOLD}${C_G}Erebrus node installed (profile=${PROFILE:-erebrus}, deploy=${DEPLOY}, access=${EREBRUS_ACCESS}).${C_RESET}"
  echo "  REST API : http://${WG_ENDPOINT_HOST}:${HTTP_PORT}/api/v2/status"
  echo "  WireGuard: ${WG_ENDPOINT_HOST}:${WG_PORT}/udp"
  echo "  Stealth  : VLESS+REALITY :${STEALTH_TCP_PORT}/tcp · Hysteria2 :${STEALTH_UDP_PORT}/udp"
  echo "  Node API key: ${NODE_API_TOKEN}"
  echo "  Verify   : erebrus status"
  if [[ "$DEPLOY" == "container" ]]; then
    echo "  Manage   : cd $INSTALL_DIR && docker compose [logs -f|restart|down]"
    echo "  Config   : $INSTALL_DIR/.env"
  else
    echo "  Manage   : systemctl [status|restart|stop] erebrus ; journalctl -u erebrus -f"
    echo "  Config   : $ENV_DIR/erebrus.env"
  fi
  if [[ "$ENABLE_APP_HOSTING" == "true" ]]; then
    echo
    echo -e "${C_BOLD}App-Hosting:${C_RESET} create this DNS record so the gateway can route apps:"
    echo -e "    ${C_BOLD}*.${APP_WILDCARD_DOMAIN}  A  ${WG_ENDPOINT_HOST}${C_RESET}"
  fi
  echo
  echo -e "${C_Y}Back up your node identity (12-word phrase) — it cannot be recovered.${C_RESET}"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  banner
  require_root
  detect_platform
  choose_deploy
  choose_access
  choose_profile
  run_preflight
  gather_config
  case "$DEPLOY" in
    container) install_docker_mode ;;
    host)      install_host_mode ;;
    *) die "invalid deploy mode: $DEPLOY" ;;
  esac
  validate_and_summary
}
main "$@"
