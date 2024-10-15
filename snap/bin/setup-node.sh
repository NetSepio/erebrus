#!/usr/bin/env bash
#
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'
env_file=${SNAP_COMMON}/.env

function configure_env() { 
REGION=$(curl -s ifconfig.io/country_code)
HOST_IP=$(curl -s ifconfig.io)
DEFAULT_DOMAIN="http://${HOST_IP}:9080"
NIC=$(ip route | grep "default" |cut -d " " -f 5)

check_mnemonic_format() {
    local mnemonic="$1"
    # Split the mnemonic into an array of words
    IFS=' ' read -r -a words <<< "$mnemonic" 

    # Define the required number of words in the mnemonic (12, 15, 18, 21, or 24 typically for BIP39)
    local required_words=(12 15 18 21 24)

    # Check if the mnemonic has the correct number of words
    local num_words=${#words[@]}
    if ! [[ " ${required_words[*]} " =~ " $num_words " ]]; then
        return 1
    fi

    # Check if each word in the mnemonic is valid
    for word in "${words[@]}"; do
        if [[ ! "$word" =~ ^[a-zA-Z]+$ ]]; then
            return 1
        fi
    done
    return 0
}

# Prompt for Chain
    printf "${NC}Select valid chain from list below:\n"
    PS3="Select a chain (e.g. 1): "    options=("APT" "SOL" "EVM" "SUI")
    select CHAIN in "${options[@]}"; do
        if [ -n "$CHAIN" ]; then
            break
        else
            echo "${NC}Invalid choice. Please select a valid chain."
        fi
    done

    while true; do
        read -p "Enter your wallet mnemonic: " WALLET_MNEMONIC 
        if check_mnemonic_format "$WALLET_MNEMONIC"; then
            break
        else
            printf "${NC}Wrong mnemonic, try agian with correct mnemonic.\n"
        fi
    done


mkdir ${SNAP_COMMON}/wireguard
tee $1  <<EOL
# Application Configuration
export RUNTYPE=debug
export SERVER=0.0.0.0
export HTTP_PORT=9080
export GRPC_PORT=9090
export REGION=${REGION}
export DOMAIN=${DEFAULT_DOMAIN}
export HOST_IP=${HOST_IP}
export MASTERNODE_URL=https://gateway.erebrus.io
export POLYGON_RPC=
export SIGNED_BY=NetSepio
export FOOTER="NetSepio 2024"
export MASTERNODE_WALLET=
export GATEWAY_DOMAIN=https://gateway.erebrus.io
export LOAD_CONFIG_FILE=false
export MASTERNODE_PEERID=/ip4/130.211.28.223/tcp/9001/p2p/12D3KooWJSMKigKLzehhhmppTjX7iQprA7558uU52hqvKqyjbELf
export MNEMONIC_APTOS="${WALLET_MNEMONIC}"
export CHAIN_NAME=${CHAIN}

# Wireguard Configuration
export WG_CONF_DIR=/etc/wireguard
export WG_CLIENTS_DIR=/etc/wireguard/clients
export WG_INTERFACE_NAME=wg0.conf
export WG_ENDPOINT_HOST=${HOST_IP}
export WG_ENDPOINT_PORT=51820
export WG_IPv4_SUBNET=10.0.0.1/24
export WG_IPv6_SUBNET=fd9f:0000::10:0:0:1/64
export WG_DNS=1.1.1.1
export WG_ALLOWED_IP_1=0.0.0.0/0
export WG_ALLOWED_IP_2=::/0
export WG_PRE_UP=echo WireGuard PreUp
export WG_POST_UP="iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o ${NIC} -j MASQUERADE"
export WG_PRE_DOWN=echo WireGuard PreDown
export WG_POST_DOWN="iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o ${NIC} -j MASQUERADE"
export PASETO_EXPIRATION_IN_HOURS=168
export AUTH_EULA="I Accept the NetSepio Terms of Service https://netsepio.com/terms.html for accessing the application. Challenge ID:"
EOL
printf "${GREEN}Node configued successfully.${NC}\n"
}

function stop_node(){
  printf "${NC}Stopping Erebrus Node ............\n"
    ${SNAP}/stop-node.sh >> ${SNAP_COMMON}/erebrus.log 2>&1
    if [ $? -eq 0 ];then
        printf "${GREEN}Ereburs Node stopped successfully\n"
    else
        printf "${RED}Could not stop Erebrus Node\n"
    fi
}

function start_node(){
 printf "${NC}Starting Erebrus Node .............\n"
    ${SNAP}/start-node.sh >> ${SNAP_COMMON}/erebrus.log 2>&1
    if [ $? -eq 0 ];then
       printf "${GREEN}Erebrus Node is started successfully\n"
       sleep 10
       ${SNAP}/status-node.sh
    else
       printf "${RED}Ereburs Node could not be started\n"
    fi
}

function check_file() {
   if [ -s $1 ]
   then
      printf "${NC}Environment file is already present\n"
      ${SNAP}/status-node.sh
      if [ $? -eq 0 ]
      then 
	 read -p "Reconfigure Node (yes|y) :" reconfigure
         if [ "$reconfigure" == "yes" ];then
	    configure_env $env_file
            stop_node
            start_node
	 elif [ "$reconfigure" == "y" ];then
	    configure_env $env_file
            stop_node
	    start_node
	 else
	    exit 1      	 
	 fi
       else
         configure_env $env_file	   
         sleep 5
         start_node
      fi
   else
      configure_env $env_file
      sleep 5
      start_node
   fi	   
}


check_file $env_file

