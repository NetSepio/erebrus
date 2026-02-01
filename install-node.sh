#!/usr/bin/env bash
set -euo pipefail

IMAGE="ghcr.io/netsepio/erebrus:main"

status_stage1="Pending"
status_stage2="Pending"
status_stage3="Pending"

#######################################
# Header
#######################################
display_header() {
    clear
    echo -e "\e[94m"
    cat << "EOF"
 /$$$$$$$$                      /$$
| $$_____/                    | $$                                    
| $$        /$$$$$$   /$$$$$$ | $$$$$$$   /$$$$$$  /$$   /$$  /$$$$$$$
| $$$$$    /$$__  $$ /$$__  $$| $$__  $$ /$$__  $$| $$  | $$ /$$_____/
| $$__/   | $$  \__/| $$$$$$$$| $$  \ $$| $$  \__/| $$  | $$|  $$$$$$ 
| $$      | $$      | $$_____/| $$  | $$| $$      | $$  | $$ \____  $$
| $$$$$$$$| $$      |  $$$$$$$| $$$$$$$/| $$      |  $$$$$$/ /$$$$$$$/
|________/|__/       \_______/|_______/ |__/       \______/ |_______/ 
                                                   Powered by NetSepio
EOF
    echo -e "\e[0m"

    printf "\n\e[1m=== Erebrus Node Installer (Ubuntu) ===\e[0m\n"
    printf "%0.s=" {1..100}
    printf "\n"

    printf "\n\e[1mRequirements:\e[0m\n"
    printf "1. Public routable IP required.\n"
    printf "2. Ports 9080, 9002 and 51820 must be open on firewall.\n"

    printf "%0.s=" {1..100}
    printf "\n\n"

    printf "\e[1mStage 1 - Dependencies:\e[0m   [%s]\n" "$status_stage1"
    printf "\e[1mStage 2 - Configure:\e[0m       [%s]\n" "$status_stage2"
    printf "\e[1mStage 3 - Run Node:\e[0m        [%s]\n\n" "$status_stage3"
}

#######################################
# Helpers
#######################################
die() { echo "❌ $1"; exit 1; }

is_valid_ip() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

is_valid_dir() {
  [[ "$1" =~ ^/[a-zA-Z0-9/_-]+$ ]]
}

get_public_ip() {
  IP=$(curl -fsS https://api.ipify.org) || die "Cannot detect public IP"
  is_valid_ip "$IP" || die "Invalid IP detected"
  echo "$IP"
}

get_region() {
  curl -fsS https://ipinfo.io/country || echo "NA"
}

check_mnemonic_format() {
  IFS=' ' read -r -a words <<< "$1"
  local count=${#words[@]}
  [[ "$count" == "12" || "$count" == "15" || "$count" == "18" || "$count" == "21" || "$count" == "24" ]] || return 1
  for w in "${words[@]}"; do
    [[ "$w" =~ ^[a-z]+$ ]] || return 1
  done
  return 0
}

#######################################
# Check node status
#######################################
check_node_status() {
  if docker ps -f name=erebrus | grep erebrus >/dev/null 2>&1; then
    return 0
  else
    return 1
  fi
}

#######################################
# Reachability test with retry
#######################################
test_ip_reachability() {
  local host_ip=$1
  local port=9080
  local retries=2

  for ((i=1;i<=retries;i++)); do
    echo "Testing IP reachability attempt $i..."

    nc -l -p "$port" >/dev/null 2>&1 &
    listener_pid=$!
    sleep 2

    if echo "test" | nc -w 3 "$host_ip" "$port" >/dev/null 2>&1; then
      kill "$listener_pid"
      echo "✅ IP is reachable from internet."
      return 0
    else
      kill "$listener_pid"
      echo "❌ IP not reachable."
      if [[ $i -lt $retries ]]; then
        read -rp "Retry? (y/n): " retry_choice
        [[ "$retry_choice" == "y" ]] || return 1
      fi
    fi
  done
  return 1
}

#######################################
# Final message
#######################################
print_final_message() {
  if check_node_status; then
    echo "✅ Erebrus node installation completed."
    echo "🌍 API available at: http://${HOST_IP}:9080"
  else
    echo "❌ Node failed to start."
  fi
}

#######################################
# Start
#######################################
display_header
read -rp "Continue installation? (y/n): " CONFIRM
[[ "$CONFIRM" == "y" ]] || die "Cancelled"

#######################################
# Pre-check
#######################################
if check_node_status; then
  echo "⚠️ Erebrus node already running. Aborting."
  exit 0
fi

#######################################
# Stage 1 - Dependencies
#######################################
status_stage1="In Progress"
display_header

sudo apt-get update
sudo apt-get install -y docker.io docker-compose curl lsof netcat-* 

sudo systemctl enable docker
sudo systemctl start docker

if ! groups "$USER" | grep -q docker; then
  sudo usermod -aG docker "$USER"
  echo "⚠️ Log out & log in required for docker group."
fi

status_stage1="Complete"
display_header

#######################################
# Stage 2 - Configure
#######################################
status_stage2="In Progress"
display_header

read -rp "Install directory (default: $(pwd)): " INSTALL_DIR
INSTALL_DIR=${INSTALL_DIR:-$(pwd)}
is_valid_dir "$INSTALL_DIR" || die "Invalid directory"
mkdir -p "$INSTALL_DIR/wireguard"

DEFAULT_IP=$(get_public_ip)
echo "Detected IP: $DEFAULT_IP"
read -rp "Use this IP? (y/n): " USE_IP
if [[ "$USE_IP" == "n" ]]; then
  read -rp "Enter public IP: " HOST_IP
  is_valid_ip "$HOST_IP" || die "Invalid IP"
else
  HOST_IP="$DEFAULT_IP"
fi

echo "Select chain:"
select CHAIN in APT SOL EVM SUI; do
  [[ -n "$CHAIN" ]] && break
done

while true; do
  read -rsp "Enter wallet mnemonic: " WALLET_MNEMONIC
  echo
  check_mnemonic_format "$WALLET_MNEMONIC" && break
  echo "Invalid mnemonic"
done

echo "INSTALL_DIR=$INSTALL_DIR"
echo "HOST_IP=$HOST_IP"
echo "CHAIN=$CHAIN"
echo "MNEMONIC=HIDDEN"

read -rp "Confirm? (y/n): " OK
[[ "$OK" == "y" ]] || die "Aborted"

test_ip_reachability "$HOST_IP" || die "Public IP is not reachable"

ENV_FILE="$INSTALL_DIR/.env"
chmod 700 "$INSTALL_DIR"

cat > "$ENV_FILE" <<EOF
RUNTYPE=debug
SERVER=0.0.0.0
HTTP_PORT=9080
GRPC_PORT=9090
REGION=$(get_region)
DOMAIN=http://${HOST_IP}:9080
HOST_IP=${HOST_IP}
MASTERNODE_URL=https://gateway.erebrus.io
POLYGON_RPC=
SIGNED_BY=NetSepio
FOOTER=NetSepio 2024
MASTERNODE_WALLET=
GATEWAY_DOMAIN=https://gateway.erebrus.io
LOAD_CONFIG_FILE=false
MASTERNODE_PEERID=/ip4/130.211.28.223/tcp/9001/p2p/12D3KooWJSMKigKLzehhhmppTjX7iQprA7558uU52hqvKqyjbELf
MNEMONIC_APTOS=${WALLET_MNEMONIC}
CHAIN_NAME=${CHAIN}

WG_CONF_DIR=/etc/wireguard
WG_CLIENTS_DIR=/etc/wireguard/clients
WG_INTERFACE_NAME=wg0.conf
WG_ENDPOINT_HOST=${HOST_IP}
WG_ENDPOINT_PORT=51820
WG_IPv4_SUBNET=10.0.0.1/16
WG_IPv6_SUBNET=fd9f:0000::10:0:0:1/64
WG_DNS=1.1.1.1
WG_ALLOWED_IP_1=0.0.0.0/0
WG_ALLOWED_IP_2=::/0
WG_PRE_UP=echo WireGuard PreUp
WG_POST_UP=iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
WG_PRE_DOWN=echo WireGuard PreDown
WG_POST_DOWN=iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
PASETO_EXPIRATION_IN_HOURS=168
AUTH_EULA=I Accept the NetSepio Terms of Service https://netsepio.com/terms.html
EOF

chmod 600 "$ENV_FILE"

status_stage2="Complete"
display_header

#######################################
# Stage 3 - Run Node
#######################################
status_stage3="In Progress"
display_header

docker pull "$IMAGE"

docker run -d \
  --name erebrus \
  --restart unless-stopped \
  -p 9080:9080/tcp \
  -p 9002:9002/tcp \
  -p 51820:51820/udp \
  --cap-add=NET_ADMIN \
  --sysctl="net.ipv4.conf.all.src_valid_mark=1" \
  --sysctl="net.ipv6.conf.all.forwarding=1" \
  -v "$INSTALL_DIR/wireguard:/etc/wireguard" \
  --env-file "$ENV_FILE" \
  "$IMAGE"

status_stage3="Complete"
display_header

print_final_message
