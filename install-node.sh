#!/usr/bin/env bash

# Function to display header and stage status
display_header() {
    clear
    brand_color="\e[94m"
    reset_color="\e[0m"
    echo -e "${brand_color}"
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
    echo -e "${reset_color}"
    printf "\n\e[1m\e[4m=== Erebrus Node Software Installer - Version 1.0 ===\e[0m\n"
    printf "%0.s=" {1..120}  # Print a line separator of 80 characters
    printf "\n"
    printf "\n\e[1mRequirements:\e[0m\n"
    printf "1. Erebrus node needs public IP that is routable from internet.\n"
    printf "   Node software requires public IP to funtion properly.\n"
    printf "2. Ports 9080, 9002 and 51820 must be open on your firewall and/or host system.\n"
    printf "   Ensure these ports are accessible to run the Erebrus Node software.\n"
    printf "%0.s=" {1..120}  # Print a line separator of 80 characters
    printf "\n"
    printf "\n\e[1mStage 1 - Configure Node:\e[0m\t       [${status_stage1}\e[0m]\n"
    printf "\e[1mStage 2 - Install Dependencies:\e[0m\t       [${status_stage2}\e[0m]\n"
    printf "\e[1mStage 3 - Run Node:\e[0m\t               [${status_stage3}\e[0m]\n\n"
}


# Function to show spinner
show_spinner() {
    local pid=$1
    local delay=0.2
    local spinstr='|/-\'
    tput civis
    printf " ["
    while [ "$(ps a | awk '{print $1}' | grep $pid)" ]; do
        local temp=${spinstr#?}
        printf "%c]" "$spinstr"
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b\b"
    done
    printf " \t"
    tput civis
}

# Function to check if Docker is installed
is_docker_installed() {
    if command -v docker > /dev/null && command -v docker-compose > /dev/null; then
        return 0
    else
        return 1
    fi
}

# Check if given group name exists in system
function group_exists() {
  if command -v getent >/dev/null 2>&1; then
    # Use getent if available (common in Linux)
    if getent group "$1" >/dev/null 2>&1; then
      return 0
    else
      return 1
    fi
  # Check using dscl (might be more reliable on macOS)
  elif command -v dscl >/dev/null 2>&1; then
    if dscl . -list /Groups | grep "$1" >/dev/null 2>&1; then
      return 0
    else
      return 1
    fi
  fi
}

function create_group() {
    # Create group, takes group name as an argument $1
        if command -v groupadd; then
            sudo groupadd "$1"
            [[ $? -eq 0 ]] && return 0 || return 1
        elif command -v dscl; then
            dscl . -create /Groups/"$1"
            [[ $? -eq 0 ]] && return 0 || return 1
        fi
}

#Create docker group and add user to the group
function add_user_to_group() {
    # Add current user to docker group
    if command -v usermod; then
        if ! groups "$USER" | grep "$1"; then
            sudo usermod -aG "$1" "$USER"  # Use sudo and usermod for Linux
            [[ $? -eq 0 ]] && return 0 || return 1
        fi
    elif command -v dscl; then
        if ! dscl . -read /Groups/"$1" | grep GroupMembership | grep "$USER"; then
            dscl . -append /Groups/"$1" GroupMembership "$USER"  # Use dscl for macOS
            [[ $? -eq 0 ]] && return 0 || return 1
        fi
    fi
}


# Function to install Docker and Docker Compose based on distribution
install_dependencies() {
    clear
    status_stage1="\e[34mIn Progress\e[0m"
    display_header
    printf "\e[1mInstalling Docker...\e[0m"
    if is_docker_installed; then
        printf " \e[32m[Already installed]\e[0m\n"
        sleep 2
    else
        if command -v apt-get > /dev/null; then
            (sudo apt-get update -qq && sudo apt-get install -y containerd docker.io && sudo apt-get install netcat-* -y && sudo apt-get install lsof -y  > /dev/null 2> error.log) &
            show_spinner $!
            printf " \e[32mComplete\e[0m\n"
        elif command -v yum > /dev/null; then
            (sudo yum install yum-utils -y && sudo yum install nmap-ncat.x86_64 -y && sudo yum install lsof -y && sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo && yum install -y docker > /dev/null 2>&1 && sudo systemctl start docker && sudo systemctl enable docker > /dev/null 2> error.log) &
            show_spinner $!
            printf " \e[32mComplete\e[0m\n"
        elif command -v pacman > /dev/null; then
            (sudo pacman -Sy --noconfirm docker > /dev/null 2>&1 && sudo systemctl start docker && sudo systemctl enable docker > /dev/null 2> error.log) &
            show_spinner $!
            printf " \e[32mComplete\e[0m\n"
        elif command -v dnf > /dev/null; then
            printf "Installing Docker on Fedora..."
            (sudo dnf install dnf-plugins-core && dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo && dnf install -y docker-ce docker-ce-cli containerd.io  > /dev/null 2>&1 && sudo systemctl start docker && sudo systemctl enable docker > /dev/null 2> error.log) &
            show_spinner $!
            printf " \e[32mComplete\e[0m\n"
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            printf "Installing Docker on macOS..."
            if ! command -v brew > /dev/null; then
                printf "Homebrew not found. Please install Homebrew first.\n"
                exit 1
            fi
            (brew install --cask docker > /dev/null 2> error.log && open /Applications/Docker.app) &
            show_spinner $!
            printf " \e[32mComplete\e[0m\n"
            printf "Please ensure Docker is running from the macOS toolbar.\n"
        else
            printf "Unsupported Linux distribution.\n"
            exit 1
        fi
    fi

    if docker --version > /dev/null 2>&1; then
        # Created docker group if not exits
        if ! group_exists "docker"; then
            create_group "docker";
        fi
        if add_user_to_group "docker"; then
            if [[ $? -ne 0 ]]; then
                printf "Failed to create group, docker configuration failed."
                exit 1
            fi
        fi
        status_stage2="\e[32m$green_tick Complete\e[0m"
        error_stage2=""
    else
        status_stage2="\e[31mFailed\e[0m"
        error_stage2="\e[31mFailed to install Docker.\e[0m\n"
    fi
    display_header

    # Install Wireguard dependencies
    install_dependencies_wireguard
}

# Function to get the public IP address
get_public_ip() {
    echo $(curl -s ifconfig.io)
}

# Function to get region
get_region() {
    echo $(curl -s ifconfig.io/country_code)
}

# Function to test if the IP is directly reachable from the internet
test_ip_reachability() {
    local host_ip=$1
    local port=9080
    local max_retries=1
    local retry=0
    local user_retry_choice=""
    local spinner_pid

    while [ $retry -le $max_retries ]; do
        display_header
        printf "\n\e[1mTesting IP reachability from internet...\e[0m"
        show_spinner $$ &  # Start the spinner
        spinner_pid=$!

        # Start a netcat listener in the background
        (nc -l $port > /dev/null 2>&1 &)
        listener_pid=$!
        sleep 2  # Give the listener time to start

        # Try to connect to the listener using netcat
        if echo "test" | nc -w 3 $host_ip $port > /dev/null 2>&1; then
            kill $listener_pid > /dev/null 2>&1
            kill $spinner_pid  # Stop the spinner
            printf "\b\b\e[32mComplete\e[0m]\n"
            sleep 3
            return 0
        else
            if [ $retry -lt $max_retries ]; then
                printf "\nThe IP address %s is not reachable from internet. IP reachability test failed.\n" "$host_ip"
                printf "Make sure port 9002 and 9080 are open on your firewall and/or host system and try again.\n"
                
                kill $spinner_pid  # Stop the spinner to interact with user
                
                read -p "Would you like to retry? (y/n): " user_retry_choice
                if [ "$user_retry_choice" != "y" ]; then
                    kill $listener_pid > /dev/null 2>&1
                    return 1
                fi
            else
                printf "\b\b\e[31mFailed\e[0m\n"
                printf "\nYou do not have a public IP that is routable and reachable from internet.\n"
                kill $listener_pid > /dev/null 2>&1
                kill $spinner_pid  # Stop the spinner
                return 1
            fi
        fi
        ((retry++))
    done

    printf "\nFailed to verify IP reachability after multiple attempts. Exiting.\n"
    kill $spinner_pid  # Stop the spinner
    exit 1
}


check_node_status() {
    local container_running=0
    local port_9080_listening=0
    local port_9002_listening=0

    # Check if container 'erebrus' is running
    if sudo docker ps -f name=erebrus | grep "erebrus" >/dev/null; then
        container_running=1
    fi

    # Check if ports 9080 and 9002 are listening using lsof command
    local lsof_output=$(sudo lsof -nP -iTCP -sTCP:LISTEN)
    if echo "${lsof_output}" | grep ":9080.*LISTEN" >/dev/null; then
        port_9080_listening=1
    fi
    if echo "${lsof_output}" | grep ":9002.*LISTEN" >/dev/null; then
        port_9002_listening=1
    fi

    # Return 0 if container is running and both ports are listening
    if [ "${container_running}" -eq 1 ] && [ "${port_9080_listening}" -eq 1 ] && [ "${port_9002_listening}" -eq 1 ]; then
        return 0  # Container is running and ports are listening
    else
        return 1  # Either container is not running or ports are not listening
    fi
}

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

print_final_message() {
    if check_node_status; then
        printf "\e[32mErebrus node installation is finished.\e[0m\n"
        printf "Erebrus Node API is accessible at http://${HOST_IP}:9080\n"
        printf "Refer \e[4mhttps://github.com/NetSepio/erebrus/blob/main/docs/docs.md\e[0m for API documentation.\n"
        printf "\n\e[32mAll stages completed successfully!\e[0m\n\n"
    else
        printf "\e[31mFailed to run Erebrus node.\e[0m\n"
    fi
}


# Function to configure Node environment variables
configure_node() {
    clear
    printf "\n\e[1mConfiguring Node environment variables...\e[0m\n"
    status_stage1="\e[34mIn Progress\e[0m" # status_stage2
    display_header

    # Prompt for installation directory and validate input
    while true; do
        read -p "Enter installation directory (default: current directory): " INSTALL_DIR
        INSTALL_DIR=${INSTALL_DIR:-$(pwd)}
        sudo mkdir -p "$INSTALL_DIR/wireguard" && sudo chown -R $(whoami):$(whoami) "$INSTALL_DIR"
        # Check if directory exists
        if [ ! -d "$INSTALL_DIR" ]; then
            printf "Error: Directory '%s' does not exist.\n" "$INSTALL_DIR"
            # Ask user for confirmation for creation with sudo
            read -p "Do you want to create the directory (you may be prompted for your password)? (y/N): " CREATE_DIR
            # Check user confirmation (case-insensitive)
            if [[ $CREATE_DIR =~ ^[Yy]$ ]]; then
            # Create directory with sudo, setting user and group to current user
            sudo mkdir -p "$INSTALL_DIR/wireguard" && sudo chown -R $(whoami):$(whoami) "$INSTALL_DIR"
            if [[ $? -eq 0 ]]; then
                printf "Directory '%s' created successfully.\n" "$INSTALL_DIR"
                break
            else
                printf "Error: Failed to create directory. Please check your permissions.\n"
            fi
            else
            printf "Directory creation skipped.\n"
            fi
        else
            # Directory exists, break the loop
            break
        fi
    done

    DEFAULT_HOST_IP=$(get_public_ip)

    # Prompt for Public IP
    printf "\nAutomatically detected public IP: ${DEFAULT_HOST_IP}\n"
    read -p "Do you want to use this public IP? (y/n): " use_default_host_ip
    if [ "$use_default_host_ip" = "n" ]; then
        read -p "Enter your public IP (default: ${DEFAULT_HOST_IP}): " HOST_IP
        HOST_IP=${HOST_IP:-$DEFAULT_HOST_IP}
    else
        HOST_IP=${DEFAULT_HOST_IP}
    fi

    DEFAULT_DOMAIN="http://${HOST_IP}:9080"

    # Prompt for Node Details
    read -p "Enter your node name: " NODE_NAME

    # Prompt for Config Type
    # printf "Select a configuration type from list below:\n"
    # PS3="Select a config type (e.g. 1): "
    # options=("MINI" "STANDARD" "HPC")
    # select CONFIG in "${options[@]}"; do
    #     if [ -n "$CONFIG" ]; then
    #         break
    #     else
    #         echo "Invalid choice. Please select a valid config type."
    #     fi
    # done

    #!/bin/sh

    printf "Select a configuration type from the list below:\n"
    PS3="Select a config type (e.g. 1): "
    options=("ASTRO Comming soon" "BEACON" "TITAN Comming soon" "NEXUS" "ZENETH")

    while true; do
        select CONFIG in "${options[@]}"; do
            case "$CONFIG" in
                "ASTRO"|"TITAN")
                    echo "This configuration will be in upcoming updates. Please choose another option."
                    break  # Break out of select loop, restart while loop
                    ;;
                "BEACON"|"NEXUS"|"ZENETH")
                    echo "You selected: $CONFIG"
                    exit 0  # Exit script on valid selection
                    ;;
                *)
                    echo "Invalid choice. Please select a valid config type."
                    ;;
            esac
        done
    done


    # Prompt for Chain
    printf "Select valid chain from list below:\n"
    PS3="Select a chain (e.g. 1): "
    options=("SOLANA" "PEAQ")
    select CHAIN in "${options[@]}"; do
        if [ -n "$CHAIN" ]; then
            break
        else
            echo "Invalid choice. Please select a valid chain."
        fi
    done

    while true; do
        read -p "Enter your wallet mnemonic: " WALLET_MNEMONIC
        if check_mnemonic_format "$WALLET_MNEMONIC"; then
            break
        else
            printf "Wrong mnemonic, try agian with correct mnemonic.\n"
        fi
    done

        # Prompt for Config Type
    printf "Select an access type from list below:\n"
    PS3="Select an access type (e.g. 1): "
    options=("public" "private")
    select ACCESS in "${options[@]}"; do
        if [ -n "$ACCESS" ]; then
            break
        else
            echo "Invalid choice. Please select a valid access type."
        fi
    done


    # Display and confirm user-provided variables
    printf "\n\e[1mUser Provided Configuration:\e[0m\n"
    printf "INSTALL DIR=%s\n" "${INSTALL_DIR}"
    printf "REGION=$(get_region)\n"
    printf "NODE_NAME=%s\n" "${NODE_NAME}"
    printf "HOST_IP=%s\n" "${HOST_IP}"
    printf "DOMAIN=%s\n" "${DEFAULT_DOMAIN}"
    printf "CHAIN=%s\n" "${CHAIN}"
    printf "CONFIG=%s\n" "${CONFIG}"
    printf "MNEMONIC=%s\n" "${WALLET_MNEMONIC}"
    printf "ACCESS_TYPE=%s\n" "${ACCESS}"
    read -p "Confirm configuration (y/n): " confirm
    if [ "${confirm}" != "y" ]; then
        printf "Configuration not confirmed. Exiting.\n"
        exit 1
    fi

    # Validate and test IP reachability
    test_ip_reachability "$HOST_IP"
    if [ $? -eq 1 ]; then
        status_stage1="\e[31mFailed\e[0m\n"
        error_stage1="\e[31mFailed to configure Erebrus node.\e[0m\n"
        return 1
    else
    # Write environment variables to .env file
    sudo tee ${INSTALL_DIR}/.env  <<EOL

# Application Configuration
RUNTYPE=released
SERVER=0.0.0.0
HTTP_PORT=9080
GRPC_PORT=9090
LIBP2P_PORT=9002
REGION=$(get_region)
NODE_NAME=${NODE_NAME}
DOMAIN=${DEFAULT_DOMAIN}
HOST_IP=${HOST_IP}
GATEWAY_DOMAIN=https://gateway.erebrus.io
POLYGON_RPC=
SIGNED_BY=NetSepio
FOOTER=NetSepio 2024
GATEWAY_WALLET=0x0
GATEWAY_DOMAIN=https://gateway.erebrus.io
LOAD_CONFIG_FILE=false
GATEWAY_PEERID=/ip4/178.156.141.248/tcp/9001/p2p/12D3KooWJSMKigKLzehhhmppTjX7iQprA7558uU52hqvKqyjbELf
CHAIN_NAME=${CHAIN}
NODE_TYPE=VPN
NODE_CONFIG=${CONFIG}
MNEMONIC=${WALLET_MNEMONIC}
CONTRACT_ADDRESS=0x291eC3328b56d5ECebdF993c3712a400Cb7569c3
RPC_URL=https://evm.peaq.network
NODE_ACCESS=${ACCESS}


# WireGuard Configuration
WG_CONF_DIR=/etc/wireguard
WG_CLIENTS_DIR=/etc/wireguard/clients
WG_INTERFACE_NAME=wg0.conf

# WireGuard Specifications
WG_ENDPOINT_HOST=${HOST_IP}
WG_ENDPOINT_PORT=51820
WG_IPv4_SUBNET=10.0.0.1/24
WG_IPv6_SUBNET=fd9f:0000::10:0:0:1/64
WG_DNS=1.1.1.1
WG_ALLOWED_IP_1=0.0.0.0/0
WG_ALLOWED_IP_2=::/0
WG_PRE_UP=echo WireGuard PreUp
WG_POST_UP=iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
WG_PRE_DOWN=echo WireGuard PreDown
WG_POST_DOWN=iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
SERVICE_CONF_DIR=./erebrus

# Authentication & Policies
PASETO_EXPIRATION_IN_HOURS=168
AUTH_EULA=I Accept the Erebrus Terms of Service https://erebrus.io/terms



# Caddy Specifications
CADDY_CONF_DIR=/etc/caddy # /etc/caddy
CADDY_INTERFACE_NAME=Caddyfile

EOL
        status_stage2="\e[32m$green_tick Complete\e[0m"
        #display_header
    fi
}

# Function to run the Node container
run_node() {
    clear
    printf "\n\e[1mRunning Erebrus Node...\e[0m"
    status_stage3="\e[34mIn Progress\e[0m"
    display_header
    printf "Starting Erebrus Node... "
    ENV_FILE="${INSTALL_DIR}/.env"
    sleep 2
    if [ ! -f "$ENV_FILE" ]; then
        printf "\e[31mError:\e[0m The .env file does not exist at path: %s\n" "$ENV_FILE"
        printf "Make sure the .env file exists and try again.\n"
        exit 1
    fi
    (sudo docker run -d -p 9080:9080/tcp -p 9002:9002/tcp -p 51820:51820/udp \
        --cap-add=NET_ADMIN --cap-add=SYS_MODULE \
        --sysctl="net.ipv4.conf.all.src_valid_mark=1" \
        --sysctl="net.ipv6.conf.all.forwarding=1" \
        --restart unless-stopped -v "${INSTALL_DIR}/wireguard:/etc/wireguard" \
        --name erebrus --env-file "${ENV_FILE}" ghcr.io/netsepio/erebrus:main > /dev/null 2> error.log) &
    show_spinner $!
    wait $!

    if [ $? -eq 0 ]; then
        status_stage3="\e[32m$green_tick Complete\e[0m"
        error_stage3=""
    else
        status_stage3="\e[31mFailed\e[0m"
        error_stage3="\e[31mFailed to run Erebrus node. See error.log for details.\e[0m\n"
    fi
    display_header
}

#######################################
# Main script execution starts here
status_stage1="\e[33mPending\e[0m"
status_stage2="\e[33mPending\e[0m"
status_stage3="\e[33mPending\e[0m"
green_tick="\u2714"
skipped_symbol="\u26D4"
clear
display_header

read -p "Do you want to continue with installation? (y/n): " confirm_installation
if [ "${confirm_installation}" != "y" ]; then
    printf "Installation canceled.\n"
    exit 1
fi

if check_node_status; then
    status_stage1="\e[33m$skipped_symbol Skipped\e[0m"
    status_stage2="\e[33m$skipped_symbol Skipped\e[0m"
    status_stage3="\e[33m$skipped_symbol Skipped\e[0m"
    display_header
    printf "\e[31mErebrus node is already installed and running. Aborting installation.\e[0m\n"
    printf "Refer \e[4mhttps://github.com/NetSepio/erebrus/blob/main/docs/docs.md\e[0m for API documentation.\n\n"
    exit 0
fi

configure_node # install_dependencies
if [ -n "${error_stage1}" ]; then
    printf "%s${error_stage1}"
    exit 1
else
    install_dependencies # configure_node
    if [ -n "${error_stage2}" ]; then
        printf "%s${error_stage2}"
        exit 1
    else
        download_and_run_binary_file
        if [ -n "${error_stage3}" ]; then
            printf "%s${error_stage3}"
            exit 1
        else
            print_final_message
        fi
    fi
fi

# Main script
# if [[ "$CONFIG_TYPE" == "STANDARD" || "$CONFIG_TYPE" == "HPC" ]]; then
#   echo "Configuration Type: $CONFIG_TYPE"
#   check_and_create_folders
# else
#   exit 1
# fi

# Function to check and create directories
check_and_create_folders() {
  # Define the folder paths
  HOME_DIR="$HOME"
  EREBRUS_FOLDER="$HOME_DIR/.erebrus"

  # Check and create .erebrus folder
  if [ ! -d "$EREBRUS_FOLDER" ]; then
    echo "Creating $EREBRUS_FOLDER..."
    mkdir -p "$EREBRUS_FOLDER"
  else
    echo "$EREBRUS_FOLDER already exists."
  fi
}

install_dependencies_wireguard() {
    clear
    echo -e "\e[1;34mInstalling Wireguard Dependencies...\e[0m"

    # Detect OS
    if command -v apk > /dev/null; then
        echo "Installing on Alpine Linux..."
        apk update && apk add --no-cache bash openresolv bind-tools wireguard-tools gettext inotify-tools iptables
    elif command -v apt-get > /dev/null; then
        echo "Installing on Debian-based systems..."
        apt-get update -qq && apt-get install -y bash resolvconf dnsutils wireguard-tools gettext inotify-tools iptables systemd && apt-get install --reinstall -y systemd
    elif command -v yum > /dev/null; then
        echo "Installing on RHEL-based systems..."
        yum install -y bash openresolv bind-utils wireguard-tools gettext inotify-tools iptables
    elif command -v pacman > /dev/null; then
        echo "Installing on Arch-based systems..."
        pacman -Sy --noconfirm bash openresolv bind-tools wireguard-tools gettext inotify-tools iptables
    elif command -v dnf > /dev/null; then
        echo "Installing on Fedora..."
        dnf install -y bash openresolv bind-utils wireguard-tools gettext inotify-tools iptables
    elif command -v brew > /dev/null; then
        echo "Installing on macOS..."
        brew install bash wireguard-tools gettext coreutils iproute2mac
    else
        echo "Unsupported Linux distribution. Exiting."
        exit 1
    fi

    echo -e "\e[32mAll dependencies installed successfully.\e[0m"
}

download_and_run_binary_file() {
    REPO="NetSepio/erebrus"
    BINARY_NAME="erebrus"
    DOWNLOAD_DIR="$(pwd)" # Set to the current directory
    ERROR_LOG="$DOWNLOAD_DIR/erebrus_error.log"

    echo "Checking and installing curl..."
    apt-get update && apt-get install -y curl

    echo "Fetching latest release info..."
    LATEST_TAG=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$LATEST_TAG" ]]; then
        echo "Failed to fetch the latest release tag."
        echo "Failed to fetch the latest release tag." > "$ERROR_LOG"
        return
    fi

    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$BINARY_NAME"

    echo "Downloading $BINARY_NAME from $DOWNLOAD_URL..."
    curl -L -o "$DOWNLOAD_DIR/$BINARY_NAME" "$DOWNLOAD_URL"

    if [[ $? -ne 0 ]]; then
        echo "Download failed!" | tee "$ERROR_LOG"
        return
    fi

    chmod +x "$DOWNLOAD_DIR/$BINARY_NAME"

    echo "$BINARY_NAME has been installed successfully!"

    # Run the binary and capture errors if any
    echo "Running $BINARY_NAME..."
    "$DOWNLOAD_DIR./$BINARY_NAME" 2> "$ERROR_LOG"

    if [[ $? -ne 0 ]]; then
        echo "Error encountered while running $BINARY_NAME. Check $ERROR_LOG for details."
    fi
}

# Run the function to check and create folders after retrieving the environment variables
check_and_create_folders