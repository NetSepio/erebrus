#!/usr/bin/env bash
#
# Erebrus v2 node installer — curl -fsSL https://erebrus.io/install.sh | bash
#
# Runs the node as a Docker compose service. Access mode (private | public) controls
# gateway visibility and default stealth ports.
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
BUILD_TAGS="with_reality_server"

# Ports
HTTP_PORT="${HTTP_PORT:-9080}"        # tcp  REST API
WG_PORT="${WG_ENDPOINT_PORT:-51820}"  # udp  WireGuard
STEALTH_TCP_PORT="${STEALTH_TCP_PORT:-443}"  # tcp  VLESS+REALITY
STEALTH_UDP_PORT="${STEALTH_UDP_PORT:-443}"  # udp  Hysteria2

# Minimum acceptable throughput for an exit node (Mbps)
MIN_DOWN_MBPS="${MIN_DOWN_MBPS:-50}"
MIN_UP_MBPS="${MIN_UP_MBPS:-20}"

# Behaviour toggles
ACCESS="${EREBRUS_ACCESS:-${ACCESS:-}}"
PROFILE="${EREBRUS_PROFILE:-standard}"
DROP="${DROP_ENABLED:-false}"
INTERACTIVE="${INTERACTIVE:-false}"
DROP_STORAGE_MAX="${DROP_STORAGE_MAX:-10GB}"
DROP_SWARM_PORT="${DROP_SWARM_PORT:-4001}"
DROP_WEBUI_ENABLED="${DROP_WEBUI_ENABLED:-false}"
DROP_STATE="disabled"
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
    --access) ACCESS="${2:-}"; shift 2 ;;
    --profile) PROFILE="${2:-}"; shift 2 ;;
    --drop) DROP="true"; shift ;;
    --no-drop) DROP="false"; shift ;;
    -y|--yes) ASSUME_YES=true; shift ;;
    --interactive) INTERACTIVE=true; shift ;;
    --skip-checks) SKIP_CHECKS=true; shift ;;
    --branch) BRANCH="${2:-}"; shift 2 ;;
    -h|--help)
      cat <<'USAGE'
Erebrus v2 node installer

Minimal unattended install (container + standard profile; everything else derived):

  MNEMONIC="..." \
  EREBRUS_ACCESS=public \
  EREBRUS_NODE_REGISTRATION_TOKEN="ere_reg_..." \
  bash install.sh --yes --skip-checks

Optional: --drop, INSTALL_DIR, WG_ENDPOINT_HOST,
  NODE_NAME, EREBRUS_IMAGE, REGION, ZONE.

Usage: install.sh [options]
  --access private|public          Gateway visibility (required unless EREBRUS_ACCESS is set)
  --profile standard|shield|sentinel  Deployment profile (default: standard)
  --drop                    Enable the optional Kubo/IPFS Drop sidecar
  --no-drop                 Disable Drop and preserve existing Kubo data
  -y, --yes                 Non-interactive (required inputs must be set)
  --interactive             Prompt for access/profile/drop
  --skip-checks             Skip static-IP / bandwidth / port preflight
  --branch <name>           Source branch to build from (default: main)
  -h, --help                This help

Required env: MNEMONIC, EREBRUS_ACCESS (or ACCESS), EREBRUS_NODE_REGISTRATION_TOKEN
Derived: public IP/region/zone, node name, gateway URL, API token (preserved on reinstall).
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
  local port="$1" label="$2" required="${3:-false}"
  command -v python3 >/dev/null 2>&1 || { warn "python3 missing; skipping $label inbound check"; return; }
  local lpid=""
  if ss -lnt 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${port}\$"; then
    info "$label ($port/tcp) is already listening locally; checking external reachability."
  else
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
    lpid=$!
    sleep 1
  fi

  local rid res
  rid="$(curl -fsS --max-time 10 -H 'Accept: application/json' \
        "https://check-host.net/check-tcp?host=${PUBLIC_IP}:${port}&max_nodes=3" 2>/dev/null \
        | python3 -c 'import sys,json;print(json.load(sys.stdin).get("request_id",""))' 2>/dev/null || true)"
  if [[ -z "$rid" ]]; then
    warn "$label ($port/tcp): external prober unavailable — could not auto-verify. Ensure the port is open."
    [[ -n "$lpid" ]] && { kill "$lpid" 2>/dev/null || true; wait "$lpid" 2>/dev/null || true; }
    return
  fi
  sleep 7
  res="$(curl -fsS --max-time 10 -H 'Accept: application/json' \
        "https://check-host.net/check-result/${rid}" 2>/dev/null || true)"
  [[ -n "$lpid" ]] && { kill "$lpid" 2>/dev/null || true; wait "$lpid" 2>/dev/null || true; }

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
    [[ "$required" == "true" ]] && die "aborted: required port $port not reachable"
    confirm "Continue anyway?" n || die "aborted: required port $port not reachable"
  fi
}

prepare_drop_firewall() {
  [[ "$DROP" == "true" ]] || return 0
  if command -v ufw >/dev/null 2>&1 && run ufw status >/dev/null 2>&1; then
    info "Opening Drop ports via ufw before reachability checks…"
    run ufw allow "${DROP_SWARM_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${DROP_SWARM_PORT}/udp" >>"$LOG_FILE" 2>&1 || true
  elif command -v firewall-cmd >/dev/null 2>&1; then
    info "Opening Drop ports via firewalld before reachability checks…"
    run firewall-cmd --permanent --add-port="${DROP_SWARM_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${DROP_SWARM_PORT}/udp" >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --reload >>"$LOG_FILE" 2>&1 || true
  else
    warn "No ufw/firewalld detected; open Drop ports in the host and cloud firewalls."
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
  if [[ "$DROP" == "true" ]]; then
    check_inbound_tcp "$DROP_SWARM_PORT" "Kubo swarm" true
  fi
  warn "UDP ports $WG_PORT (WireGuard) and $STEALTH_UDP_PORT (Hysteria2) can't be probed reliably —"
  [[ "$DROP" == "true" ]] && warn "the same applies to Drop swarm ${DROP_SWARM_PORT}/udp."
  warn "make sure they're open; the installer will add firewall rules where it can."
}

# ---------------------------------------------------------------------------
# Mode selection + configuration
# ---------------------------------------------------------------------------
normalize_access() {
  case "$(echo "$1" | tr '[:upper:]' '[:lower:]')" in
    private|public) echo "$(echo "$1" | tr '[:upper:]' '[:lower:]')" ;;
    *) die "invalid access mode: $1 (use private or public)" ;;
  esac
}

choose_access() {
  local raw="${ACCESS:-${EREBRUS_ACCESS:-}}"
  if [[ -n "$raw" ]]; then
    ACCESS="$(normalize_access "$raw")"
    ok "Access mode: $ACCESS"
    return
  fi
  if $ASSUME_YES || [[ -z "$TTY" ]]; then
    die "EREBRUS_ACCESS (or ACCESS) is required — set private or public"
  fi
  echo
  echo -e "${C_BOLD}Choose access mode:${C_RESET}"
  echo "  1) private — your devices and org members only"
  echo "  2) public  — listed on the network for entitled users"
  local c; ask c "Selection [1/2]" "1"
  case "$c" in
    1|private) ACCESS="private" ;;
    2|public)  ACCESS="public" ;;
    *) die "invalid selection: $c" ;;
  esac
  ok "Access mode: $ACCESS"
}

choose_profile() {
  PROFILE="${PROFILE:-standard}"
  if [[ -n "$PROFILE" ]]; then
    case "$PROFILE" in
      standard|shield|sentinel) ok "Profile: $PROFILE"; return ;;
      *) die "invalid profile: $PROFILE (use standard, shield, or sentinel)" ;;
    esac
  fi
  $INTERACTIVE || { PROFILE="standard"; ok "Profile: $PROFILE"; return; }
  echo
  echo -e "${C_BOLD}Choose deployment profile:${C_RESET}"
  echo "  1) Erebrus — VPN node only (default)"
  echo "  2) Erebrus Shield — node + AdGuard Home DNS protection"
  echo "  3) Erebrus Sentinel — node + Unbound licensed firewall"
  local c; ask c "Selection [1/2/3]" "1"
  case "$c" in
    1|standard) PROFILE="standard" ;;
    2|shield)  PROFILE="shield" ;;
    3|sentinel) PROFILE="sentinel" ;;
    *) die "invalid selection: $c" ;;
  esac
  ok "Profile: $PROFILE"
}

choose_drop() {
  if [[ -n "$DROP" ]]; then
    case "$(echo "$DROP" | tr '[:upper:]' '[:lower:]')" in
      1|true|yes|on) DROP="true" ;;
      0|false|no|off) DROP="false" ;;
      *) die "invalid DROP_ENABLED value: $DROP" ;;
    esac
  elif $ASSUME_YES || [[ -z "$TTY" ]] || ! $INTERACTIVE; then
    DROP="false"
  elif confirm "Enable Erebrus Drop (Kubo/IPFS storage)?" n; then
    DROP="true"
  else
    DROP="false"
  fi
  ok "Drop: $([[ "$DROP" == "true" ]] && echo enabled || echo disabled)"
  if [[ "$DROP" == "true" ]]; then
    ok "Drop: files accessed via the authenticated Erebrus gateway"
  fi
}

# config values
NODE_NAME=""; REGION=""; ZONE=""; WG_ENDPOINT_HOST=""; MNEMONIC="${MNEMONIC:-}"
NODE_API_TOKEN="${NODE_API_TOKEN:-}"; NODE_KEY="${NODE_KEY:-}"
EREBRUS_NODE_REGISTRATION_TOKEN="${EREBRUS_NODE_REGISTRATION_TOKEN:-${EREBRUS_ORG_ENROLLMENT_SECRET:-}}"
GATEWAY_URL="${GATEWAY_URL:-https://gateway.erebrus.io}"
ENABLE_STEALTH="${ENABLE_STEALTH:-true}"; REALITY_SERVER_NAMES="${REALITY_SERVER_NAMES:-www.microsoft.com}"
HYSTERIA2_OBFS_PASSWORD="${HYSTERIA2_OBFS_PASSWORD:-}"
EREBRUS_BIN=""  # path/way to invoke binary for genmnemonic

rand_token() { head -c 24 /dev/urandom | base64 | tr -d '/+=' | head -c 32; }

read_env_value() {
  local f="$1" key="$2"
  grep -m1 "^${key}=" "$f" 2>/dev/null | sed "s/^${key}=//" | tr -d '\r' || true
}

load_existing_env_values() {
  local f="$INSTALL_DIR/.env"
  [[ -f "$f" ]] || return 0
  info "Loading existing values from $f"
  MNEMONIC="${MNEMONIC:-$(read_env_value "$f" MNEMONIC)}"
  ACCESS="${ACCESS:-${EREBRUS_ACCESS:-$(read_env_value "$f" EREBRUS_ACCESS)}}"
  EREBRUS_NODE_REGISTRATION_TOKEN="${EREBRUS_NODE_REGISTRATION_TOKEN:-$(read_env_value "$f" EREBRUS_NODE_REGISTRATION_TOKEN)}"
  NODE_NAME="${NODE_NAME:-$(read_env_value "$f" NODE_NAME)}"
  WG_ENDPOINT_HOST="${WG_ENDPOINT_HOST:-$(read_env_value "$f" WG_ENDPOINT_HOST)}"
  REGION="${REGION:-$(read_env_value "$f" REGION)}"
  ZONE="${ZONE:-$(read_env_value "$f" ZONE)}"
  NODE_API_TOKEN="${NODE_API_TOKEN:-$(read_env_value "$f" NODE_API_TOKEN)}"
  NODE_KEY="${NODE_KEY:-$(read_env_value "$f" NODE_KEY)}"
}

ensure_required_inputs() {
  load_existing_env_values
  choose_access

  if [[ -z "${EREBRUS_NODE_REGISTRATION_TOKEN:-}" ]]; then
    if [[ -n "$TTY" ]] && ! $ASSUME_YES; then
      ask EREBRUS_NODE_REGISTRATION_TOKEN "Node registration token (ere_reg_*)" ""
    else
      die "EREBRUS_NODE_REGISTRATION_TOKEN is required"
    fi
  fi
  [[ -n "$EREBRUS_NODE_REGISTRATION_TOKEN" ]] || die "EREBRUS_NODE_REGISTRATION_TOKEN is required"

  if [[ -z "${MNEMONIC:-}" ]]; then
    if [[ -n "$TTY" ]] && ! $ASSUME_YES; then
      ask MNEMONIC "Node mnemonic (12 words)" ""
    else
      die "MNEMONIC is required"
    fi
  fi
  [[ -n "$MNEMONIC" ]] || die "MNEMONIC is required"
}

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

  NODE_NAME="${NODE_NAME:-erebrus-$(hostname -s 2>/dev/null || echo node)}"
  WG_ENDPOINT_HOST="${WG_ENDPOINT_HOST:-$PUBLIC_IP}"
  GATEWAY_URL="${GATEWAY_URL:-https://gateway.erebrus.io}"
  [[ -n "$NODE_API_TOKEN" ]] || NODE_API_TOKEN="$(rand_token)"

  if $INTERACTIVE && [[ -n "$TTY" ]] && ! $ASSUME_YES; then
    ask NODE_NAME "Node name" "$NODE_NAME"
    ask ZONE "Zone (optional — e.g. east, west, us-east)" "${ZONE:-}"
    ask WG_ENDPOINT_HOST "Public endpoint IP" "$WG_ENDPOINT_HOST"
    ask GATEWAY_URL "Gateway URL" "$GATEWAY_URL"
  else
    ok "Node name: $NODE_NAME"
    ok "Endpoint host: $WG_ENDPOINT_HOST"
    ok "Gateway: $GATEWAY_URL"
  fi

  EREBRUS_NETWORK_PROFILE=bridge
  EREBRUS_ACCESS="${ACCESS:-private}"

  ok "Stealth carriers: VLESS+REALITY ${STEALTH_TCP_PORT}/tcp · Hysteria2 ${STEALTH_UDP_PORT}/udp"
}

# Container image: registry default; local build fallback.
EREBRUS_IMAGE="${EREBRUS_IMAGE:-ghcr.io/netsepio/erebrus:latest}"

# Invoke the node CLI from the container image.
erebrus_cli() {
  run docker run --rm "$EREBRUS_IMAGE" "$@"
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
  # Shield profile: generate a strong AdGuard admin password once. The node
  # configures AdGuard with it on startup and reports it to the gateway, where
  # org paid seats can view/rotate it.
  if [[ "${PROFILE:-standard}" == "shield" ]]; then
    SHIELD_ADMIN_USER="${SHIELD_ADMIN_USER:-admin}"
    SHIELD_ADMIN_PASSWORD="${SHIELD_ADMIN_PASSWORD:-$(rand_token)}"
  fi
  run tee "$f" >/dev/null <<EOF
# Erebrus v2 node — generated $(date '+%F %T')
# See .env.example in the repo for field documentation.
RUNTYPE=release
EREBRUS_IMAGE=${EREBRUS_IMAGE}
EREBRUS_PROFILE=${PROFILE:-standard}
EREBRUS_ACCESS=${EREBRUS_ACCESS:-private}
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

# Optional Drop/Kubo storage
DROP_ENABLED=${DROP}
DROP_STORAGE_MAX=${DROP_STORAGE_MAX}
DROP_SWARM_PORT=${DROP_SWARM_PORT}
DROP_WEBUI_ENABLED=${DROP_WEBUI_ENABLED}

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
  case "${PROFILE:-standard}" in
    shield)
      cat <<PEOF
FIREWALL_PROVIDER=adguard_home
FIREWALL_DNS_ADDR=adguardhome:53
SHIELD_ADMIN_URL=http://adguardhome:3000
SHIELD_ADMIN_USER=${SHIELD_ADMIN_USER:-admin}
SHIELD_ADMIN_PASSWORD=${SHIELD_ADMIN_PASSWORD}
SHIELD_UPSTREAM_DNS=1.1.1.1,1.0.0.1
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
  local name="${PROFILE:-standard}"
  case "$name" in standard) name="erebrus" ;; esac
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

install_drop_files() {
  local compose_src="" init_src=""
  if [[ -f "$INSTALL_DIR/deploy/compose/drop.yml" ]]; then
    compose_src="$INSTALL_DIR/deploy/compose/drop.yml"
    init_src="$INSTALL_DIR/deploy/compose/kubo-init.sh"
  elif [[ -f "$INSTALL_DIR/src/deploy/compose/drop.yml" ]]; then
    compose_src="$INSTALL_DIR/src/deploy/compose/drop.yml"
    init_src="$INSTALL_DIR/src/deploy/compose/kubo-init.sh"
  fi
  if [[ -n "$compose_src" && -f "$init_src" ]]; then
    run cp "$compose_src" "$INSTALL_DIR/drop.yml"
    run cp "$init_src" "$INSTALL_DIR/kubo-init.sh"
    run chmod 755 "$INSTALL_DIR/kubo-init.sh"
    return
  fi
  info "Fetching Drop Compose support…"
  curl -fsSL "https://raw.githubusercontent.com/NetSepio/erebrus/${BRANCH}/deploy/compose/drop.yml" \
    -o "$INSTALL_DIR/drop.yml" >>"$LOG_FILE" 2>&1 || die "failed to fetch deploy/compose/drop.yml"
  curl -fsSL "https://raw.githubusercontent.com/NetSepio/erebrus/${BRANCH}/deploy/compose/kubo-init.sh" \
    -o "$INSTALL_DIR/kubo-init.sh" >>"$LOG_FILE" 2>&1 || die "failed to fetch deploy/compose/kubo-init.sh"
  run chmod 755 "$INSTALL_DIR/kubo-init.sh"
}

# ---------------------------------------------------------------------------
# Firewall
# ---------------------------------------------------------------------------
open_firewall() {
  if command -v ufw >/dev/null 2>&1 && run ufw status >/dev/null 2>&1; then
    info "Opening ports via ufw…"
    run ufw allow "${HTTP_PORT}/tcp"  >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${STEALTH_TCP_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${WG_PORT}/udp"    >>"$LOG_FILE" 2>&1 || true
    run ufw allow "${STEALTH_UDP_PORT}/udp"   >>"$LOG_FILE" 2>&1 || true
    if [[ "$DROP" == "true" ]]; then
      run ufw allow "${DROP_SWARM_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
      run ufw allow "${DROP_SWARM_PORT}/udp" >>"$LOG_FILE" 2>&1 || true
    fi
    ok "ufw rules added."
  elif command -v firewall-cmd >/dev/null 2>&1; then
    info "Opening ports via firewalld…"
    run firewall-cmd --permanent --add-port="${HTTP_PORT}/tcp"  >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${STEALTH_TCP_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${WG_PORT}/udp"    >>"$LOG_FILE" 2>&1 || true
    run firewall-cmd --permanent --add-port="${STEALTH_UDP_PORT}/udp"   >>"$LOG_FILE" 2>&1 || true
    if [[ "$DROP" == "true" ]]; then
      run firewall-cmd --permanent --add-port="${DROP_SWARM_PORT}/tcp" >>"$LOG_FILE" 2>&1 || true
      run firewall-cmd --permanent --add-port="${DROP_SWARM_PORT}/udp" >>"$LOG_FILE" 2>&1 || true
    fi
    run firewall-cmd --reload >>"$LOG_FILE" 2>&1 || true
    ok "firewalld rules added."
  else
    warn "No ufw/firewalld detected. Ensure these are open in your cloud security group:"
    warn "  ${HTTP_PORT}/tcp, ${STEALTH_TCP_PORT}/tcp, ${WG_PORT}/udp, ${STEALTH_UDP_PORT}/udp"
    if [[ "$DROP" == "true" ]]; then
      warn "  Drop swarm: ${DROP_SWARM_PORT}/tcp and ${DROP_SWARM_PORT}/udp"
    fi
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
  [[ "$DROP" == "true" ]] && install_drop_files

  ensure_mnemonic
  write_env_file "$INSTALL_DIR/.env"

  info "Starting node via docker compose…"
  local -a compose=(docker compose)
  docker compose version >/dev/null 2>&1 || compose=(docker-compose)
  if [[ "$DROP" == "true" ]]; then
    local -a drop_files=(-f docker-compose.yml -f drop.yml)
    ( cd "$INSTALL_DIR" && run "${compose[@]}" --env-file .env "${drop_files[@]}" up -d >>"$LOG_FILE" 2>&1 ) || die "docker compose up failed"
    local kubo_id="" health="" i
    for i in $(seq 1 30); do
      kubo_id="$(cd "$INSTALL_DIR" && run "${compose[@]}" --env-file .env "${drop_files[@]}" ps -q kubo 2>/dev/null || true)"
      if [[ -n "$kubo_id" ]]; then
        health="$(run docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$kubo_id" 2>/dev/null || true)"
        [[ "$health" == "healthy" ]] && break
      fi
      sleep 2
    done
    if [[ "$health" == "healthy" ]]; then
      DROP_STATE="active"
      ok "Drop Kubo sidecar is healthy."
    else
      DROP_STATE="starting"
      warn "Drop Kubo sidecar is not healthy yet; VPN remains available."
    fi
  else
    if [[ -f "$INSTALL_DIR/drop.yml" ]]; then
      ( cd "$INSTALL_DIR" && run "${compose[@]}" --env-file .env -f docker-compose.yml -f drop.yml stop kubo >>"$LOG_FILE" 2>&1 ) || true
    fi
    ( cd "$INSTALL_DIR" && run "${compose[@]}" --env-file .env -f docker-compose.yml up -d >>"$LOG_FILE" 2>&1 ) || die "docker compose up failed"
  fi
  open_firewall
  ok "Docker node started."
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
    echo "    cd $INSTALL_DIR && docker compose logs -f"
  fi

  echo
  echo -e "${C_BOLD}${C_G}Erebrus node installed (profile=${PROFILE:-standard}, access=${EREBRUS_ACCESS}).${C_RESET}"
  echo "  REST API : http://${WG_ENDPOINT_HOST}:${HTTP_PORT}/api/v2/status"
  echo "  WireGuard: ${WG_ENDPOINT_HOST}:${WG_PORT}/udp"
  echo "  Stealth  : VLESS+REALITY :${STEALTH_TCP_PORT}/tcp · Hysteria2 :${STEALTH_UDP_PORT}/udp"
  echo "  Node API key: ${NODE_API_TOKEN}"
  echo "  Verify   : erebrus status"
  if [[ "$DROP" == "true" ]]; then
    echo "  Drop     : ${DROP_STATE} (gateway-only file access; swarm ${DROP_SWARM_PORT}/tcp+udp)"
  else
    echo "  Drop     : disabled (existing Kubo data preserved)"
  fi
  if [[ "$DROP" == "true" ]]; then
    echo "  Manage   : cd $INSTALL_DIR && docker compose --env-file .env -f docker-compose.yml -f drop.yml [ps|logs -f|restart]"
  else
    echo "  Manage   : cd $INSTALL_DIR && docker compose [logs -f|restart|down]"
  fi
  echo "  Config   : $INSTALL_DIR/.env"
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
  PROFILE="${PROFILE:-standard}"
  ensure_required_inputs
  choose_profile
  choose_drop
  prepare_drop_firewall
  run_preflight
  gather_config
  install_docker_mode
  validate_and_summary
}
main "$@"
