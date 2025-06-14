#!/usr/bin/env bash
# Initialize logging
init_logging() {
    LOG_DIR="/tmp"
    LOG_FILE="${LOG_DIR}/erebrus_install.log"
    
    # Clear previous log and start fresh
    > "$LOG_FILE"
    log_info "=== Erebrus Node Installation Started ==="
    log_info "Installation directory: $INSTALL_DIR"
    log_info "Installation mode: ${INSTALLATION_MODE:-binary}"
    log_info "Timestamp: $(date)"
}

# Centralized logging functions
log_info() {
    local message="$1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $message" >> "$LOG_FILE"
}

log_error() {
    local message="$1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $message" >> "$LOG_FILE"
}

log_success() {
    local message="$1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [SUCCESS] $message" >> "$LOG_FILE"
}

log_warning() {
    local message="$1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARNING] $message" >> "$LOG_FILE"
}

format_status() {
    local status="$1"
    case "$status" in
        "✔ Complete") echo "[\033[32m$status\033[0m]" ;;
        "✘ Skipped")  echo "[\033[33m$status\033[0m]" ;;
        "✘ Failed")   echo "[\033[31m$status\033[0m]" ;;
        "In Progress")echo "[\033[34m$status\033[0m]" ;;
        "Pending")    echo "[$status]" ;;
        *)            echo "[$status]" ;;
    esac
}

display_header() {
    # clear everything including scrollback buffer
    printf '\033[2J\033[3J\033[H'
    local header_buffer=""
    
    # Add the logo to buffer
    header_buffer+="$(tput clear)$(tput civis)"
    header_buffer+="\e[94m"
    header_buffer+=$(cat << "EOF"
/$$$$$$$$                      /$$
| $$_____/                    | $$                                    
| $$        /$$$$$$   /$$$$$$ | $$$$$$$   /$$$$$$  /$$   /$$  /$$$$$$$
| $$$$$    /$$__  $$ /$$__  $$| $$__  $$ /$$__  $$| $$  | $$ /$$_____/
| $$__/   | $$  \__/| $$$$$$$$| $$  \ $$| $$  \__/| $$  | $$|  $$$$$$ 
| $$      | $$      | $$_____/| $$  | $$| $$      | $$  | $$ \____  $$
| $$$$$$$$| $$      |  $$$$$$$| $$$$$$$/| $$      |  $$$$$$/ /$$$$$$$/
|________/|__/       \_______/|_______/ |__/       \______/ |_______/ 
EOF
)
    header_buffer+="\e[0m\n\n"
    header_buffer+="\033[1m\033[4mErebrus Node Software Installer v1.1\033[0m\n"
    printf '─%.0s' {1..80}

    # Add separator and requirements
    header_buffer+=$(printf '─%.0s' {1..100})
    header_buffer+="\n\e[1mRequirements:\e[0m\n"
    header_buffer+="→ Erebrus node needs static public IP that is routable from internet & controlled by you.\n"
    header_buffer+="→ Ports 9080, 9002, 9003, 51820, and 8088 must be open on your firewall and/or host system.\n"
    header_buffer+=$(printf '─%.0s' {1..100})
    header_buffer+="\n"

    # Add status lines
    header_buffer+="\033[1m🔧 Configure Node:       \033[0m$(format_status "${STAGE_STATUS[0]}")\n"
    header_buffer+="\033[1m📦 Install Packages:     \033[0m$(format_status "${STAGE_STATUS[1]}")\n"
    header_buffer+="\033[1m🚀 Run Node:             \033[0m$(format_status "${STAGE_STATUS[2]}")\n"
    
    # Add final separator
    header_buffer+=$(printf '─%.0s' {1..100})
    header_buffer+="\n"
    
    # Print the entire buffer at once
    echo -e "$header_buffer"
    
    # Save cursor position after printing everything
    tput sc
    log_info "Header displayed successfully"
}

# Function to clear all subprocess output
function clear_subprocess_output() {
    # Go to saved position after header (status lines)
    tput rc
    
    # Move down 4 lines (3 status lines + 1 separator line)
    # tput cud
    
    # Clear everything from current position to end of screen
    tput ed
    log_info "Subprocess output cleared"
}

# Function to show spinner
show_spinner() {
    local pid=$1
    local msg=$2
    local delay=0.2
    local spinstr='|/-\'
    
    log_info "Starting subprocess: $msg"

    # Disable keyboard input echoing and save terminal settings
    stty -echo
    local old_tty_settings=$(stty -g)
    
    # Print the initial message with brackets and spinner placeholder
    printf "\n%s [ ]" "$msg"
    printf "\b\b"  # Move cursor back inside the brackets
    
    # Start the spinner
    while kill -0 $pid 2>/dev/null; do
        local temp=${spinstr#?}
        printf "%c\b" "$spinstr"  # Print spinner char and move back
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        
        # Clear any input to prevent line breaks
        read -t 0.1 -n 10000 discard 2>/dev/null || true
    done
    
    # Get the exit status of the process
    wait $pid
    local exit_status=$?
    
    # Update with Done/Failed in brackets and add newline
    if [ $exit_status -eq 0 ]; then
        printf "\033[32mSuccess\033[0m]\n"
        log_success "Subprocess completed: $msg"
    else
        printf "\033[31mFailed\033[0m]\n"
        log_error "Subprocess failed: $msg (exit code: $exit_status)"
    fi
    
    # Restore terminal settings
    stty "$old_tty_settings"
    stty echo
    
    return $exit_status
}

# Function to check and create installation directory
create_install_directory() {
    local base_dir="$1"
    
    if [[ -z "$base_dir" ]]; then
        log_error "create_install_directory: base_dir parameter is required"
        return 1
    fi
    
    # Set installation directory to base_dir/erebrus
    INSTALL_DIR="${base_dir}/erebrus"
    log_info "Setting installation directory to: $INSTALL_DIR"
    
    # Check if directory already exists
    if [ -d "$INSTALL_DIR" ]; then
        log_info "Installation directory already exists: $INSTALL_DIR"
        return 0
    fi
    
    log_info "Creating installation directory: $INSTALL_DIR"
    
    # Try to create without sudo first
    if mkdir -p "$INSTALL_DIR/wireguard" 2>/dev/null && chown -R $(id -u -n):$(id -g -n) "$INSTALL_DIR" 2>/dev/null; then
        log_success "Installation directory created successfully: $INSTALL_DIR"
        return 0
    else
        # Try with sudo
        printf "Creating directory '%s' requires elevated permissions.\n" "$INSTALL_DIR"
        if sudo mkdir -p "$INSTALL_DIR/wireguard" && sudo chown -R $(whoami):$(whoami) "$INSTALL_DIR"; then
            printf "Directory '%s' created successfully.\n" "$INSTALL_DIR"
            log_success "Installation directory created with sudo: $INSTALL_DIR"
            return 0
        else
            printf "Error: Failed to create directory '%s'.\n" "$INSTALL_DIR"
            log_error "Failed to create installation directory: $INSTALL_DIR"
            return 1
        fi
    fi
}

# Function to get the public IP address
get_public_ip() {
    log_info "Attempting to get public IP address"
    local ip=$(curl -s ifconfig.io 2>>"$LOG_FILE")
    if [[ -n "$ip" ]]; then
        log_success "Public IP detected: $ip"
        echo "$ip"
    else
        log_error "Failed to detect public IP address"
        echo ""
    fi
}

# Function to get region
get_region() {
    log_info "Attempting to get region"
    local region=$(curl -s ifconfig.io/country_code 2>>"$LOG_FILE")
    if [[ -n "$region" ]]; then
        log_success "Region detected: $region"
        echo "$region"
    else
        log_error "Failed to detect region"
        echo "US"
    fi
}

# Function to check if Docker is installed
is_docker_installed() {
    log_info "Checking if Docker is installed"
    if command -v docker > /dev/null && command -v docker-compose > /dev/null; then
        log_success "Docker is already installed"
        return 0
    else
        log_info "Docker is not installed"
        return 1
    fi
}

# Function to test if the IP is directly reachable from the internet
test_ip_reachability() {
    local host_ip=$1
    local port=9080
    local max_retries=2
    local retry=0
    local user_retry_choice=""
    local listener_pid=""

    log_info "Testing IP reachability for $host_ip:$port"

    # This function does the actual test and returns success/failure. It doesn't print any messages
    do_ip_test() {
        local host_ip=$1
        local port=$2
        local listener_pid=""
        
        # Check if port is already in use
        if sudo lsof -i :$port > /dev/null 2>&1; then
            log_warning "Port $port is already in use, skipping reachability test"
            return 0  # Consider this a success to continue installation
        fi
        
        # Start a netcat listener in the background
        nc -l $port > /dev/null 2>&1 &
        listener_pid=$!
        
        # Verify the listener started successfully
        sleep 1
        if ! kill -0 "$listener_pid" 2>/dev/null; then
            log_error "Failed to start netcat listener on port $port"
            return 1
        fi
        
        sleep 1  # Give the listener more time to bind to the port

        # Try to connect to the listener using netcat
        if echo "test" | nc -w 3 $host_ip $port > /dev/null 2>&1; then
            # Kill the listener if it's still running
            if [ -n "$listener_pid" ] && kill -0 "$listener_pid" 2>/dev/null; then
                kill "$listener_pid" > /dev/null 2>&1
            fi
            log_success "IP reachability test passed for $host_ip:$port"
            return 0
        else
            # Kill the listener if it's still running
            if [ -n "$listener_pid" ] && kill -0 "$listener_pid" 2>/dev/null; then
                kill "$listener_pid" > /dev/null 2>&1
            fi
            log_error "IP reachability test failed for $host_ip:$port"
            return 1
        fi
    }

    while [ $retry -le $max_retries ]; do
        # Run the test with spinner
        do_ip_test "$host_ip" "$port" &
        show_spinner $! "→ Verifying IP reachability"
        local test_result=$?
        
        if [ $test_result -eq 0 ]; then
            return 0  # Success
        else
            # If we have retries left, ask the user if they want to retry
            if [ $retry -lt $max_retries ]; then
                printf "\nThe IP address %s is not reachable from internet. IP reachability test failed.\n" "$host_ip"
                printf "Make sure port 9002 and 9080 are open on your firewall and/or host system and try again.\n"
                
                read -p "Would you like to retry? (y/n): " user_retry_choice
                if [ "$user_retry_choice" != "y" ]; then
                    log_error "User chose not to retry IP reachability test"
                    return 1
                fi
            else
                printf "\nYou do not have a public IP that is routable and reachable from internet.\n"
                log_error "IP reachability test failed after $max_retries attempts"
                return 1
            fi
        fi
        
        ((retry++))
        log_warning "IP reachability test failed for $host_ip:$port - retry $retry"
    done

    log_error "IP reachability test failed completely"
    return 1
}

# Docker check_node_status() has been deprecated. See bottom of script if needed.
check_node_status() {
    log_info "Checking node status"
    local container_running=0
    local port_9080_listening=0
    local port_9002_listening=0
    local port_8088_listening=0
    local service_responding=0

    # Check if container 'erebrus' is running (more precise check)
    if [ "$INSTALLATION_MODE" = "container" ]; then
        if sudo docker ps --format "table {{.Names}}" | grep -q "^erebrus$"; then
            container_running=1
            log_info "Erebrus container is running"
        else
            log_info "Erebrus container is not running"
        fi
    fi

    # Check specific ports more efficiently
    if sudo lsof -i :9080 -sTCP:LISTEN >/dev/null 2>&1; then
        port_9080_listening=1
        log_info "Port 9080 is listening"
    else
        log_info "Port 9080 is not listening"
    fi

    if sudo lsof -i :9002 -sTCP:LISTEN >/dev/null 2>&1; then
        port_9002_listening=1
        log_info "Port 9002 is listening"
    else
        log_info "Port 9002 is not listening"
    fi

    if sudo lsof -i :8088 -sTCP:LISTEN >/dev/null 2>&1; then
        port_8088_listening=1
        log_info "Xray Port 8088 is listening"
    else
        log_info "Xray Port 8088 is not listening"
    fi

    # HTTP health check to verify service is actually responding
    if [ "$port_9080_listening" -eq 1 ]; then
        if curl -s --connect-timeout 5 --max-time 10 "http://localhost:9080" >/dev/null 2>&1 || \
           curl -s --connect-timeout 5 --max-time 10 "http://localhost:9080/health" >/dev/null 2>&1 || \
           curl -s --connect-timeout 5 --max-time 10 "http://localhost:9080/api" >/dev/null 2>&1; then
            service_responding=1
            log_info "Erebrus service is responding to HTTP requests"
        else
            log_warning "Port 9080 is listening but service is not responding to HTTP requests"
        fi
    fi

    # Determine overall status
    local status_ok=0
    if [ "$INSTALLATION_MODE" = "container" ]; then
        if [[ "$container_running" -eq 1 && "$port_9080_listening" -eq 1 && "$port_9002_listening" -eq 1 ]]; then
            status_ok=1
        fi
    else
        if [[ "$port_9080_listening" -eq 1 && "$port_9002_listening" -eq 1 && "$port_8088_listening" -eq 1 ]]; then
            status_ok=1
        fi
    fi

    # Check if service is also responding
    if [[ "$status_ok" -eq 1 && "$service_responding" -eq 1 ]]; then
        log_success "Node status check passed - Service is fully operational"
        return 0
    elif [[ "$status_ok" -eq 1 ]]; then
        log_success "Node status check passed - Ports are listening"
        return 0
    else
        log_error "Node status check failed"
        return 1
    fi
}

validate_post_install() {
    echo "🔍 Preparing to validate installation..."
    (sleep 5 && check_node_status) &
    show_spinner $! "→ Validating installation"
    return $?
}

check_mnemonic_format() {
    log_info "Validating mnemonic format"
    local mnemonic="$1"
    # Split the mnemonic into an array of words
    IFS=' ' read -r -a words <<< "$mnemonic"

    # Define the required number of words in the mnemonic (12, 15, 18, 21, or 24 typically for BIP39)
    local required_words=(12 15 18 21 24)

    # Check if the mnemonic has the correct number of words
    local num_words=${#words[@]}
    if ! [[ " ${required_words[*]} " =~ " $num_words " ]]; then
        log_error "Invalid mnemonic: wrong number of words ($num_words). Expected: 12, 15, 18, 21, or 24"
        return 1
    fi

    # Check if each word in the mnemonic is valid
    for word in "${words[@]}"; do
        if [[ ! "$word" =~ ^[a-zA-Z]+$ ]]; then
            log_error "Invalid mnemonic: word '$word' contains non-alphabetic characters"
            return 1
        fi
    done
    log_success "Mnemonic format validation passed ($num_words words)"
    return 0
}

print_final_message() {
    log_info "Generating final installation message"
    
    # Check if any enabled stages failed
    local has_failures=false
    local enabled_stages=0
    local completed_stages=0
    
    for i in {0..2}; do
        local status="${STAGE_STATUS[$i]}"
        if [[ "$status" == "✘ Failed" || "$status" == "✘ Blocked" ]]; then
            has_failures=true
        fi
        if [[ "$status" != "✘ Skipped" ]]; then
            enabled_stages=$((enabled_stages + 1))
            if [[ "$status" == "✔ Complete" ]]; then
                completed_stages=$((completed_stages + 1))
            fi
        fi
    done
    
    if [[ "$has_failures" == true ]]; then
        printf "\e[31mInstallation failed due to stage failures.\e[0m\n"
        printf "See $LOG_FILE for details.\n"
        log_error "Installation failed - One or more stages failed"
    elif [[ $enabled_stages -eq 0 ]]; then
        printf "\e[33mNo stages were enabled to run.\e[0m\n"
        log_warning "No stages were enabled to run"
    elif [[ $completed_stages -eq $enabled_stages ]]; then
        printf "\e[32mErebrus node installation is finished.\e[0m\n"
        printf "Erebrus Node API is accessible at http://${HOST_IP}:9080\n"
        printf "Refer \e[4mhttps://github.com/NetSepio/erebrus/blob/main/docs/docs.md\e[0m for API documentation.\n"
        printf "\nYou can now manage the node using the \e[1merebrus\e[0m command. Try:\n"
        printf "  \e[36merebrus status\e[0m\n"
        printf "\n\e[32mAll stages completed successfully!\e[0m\n\n"
        log_success "Installation completed successfully - Node is running"
    else
        printf "\e[33mSome enabled stages did not complete successfully.\e[0m\n"
        printf "See $LOG_FILE for details.\n"
        log_warning "Some enabled stages did not complete successfully"
    fi
}

# Stage #1 - Configure Node environment variables
configure_node() {
    log_info "=== Starting Stage 1: Configure Node ==="
    echo "📋 Configuring node..."
    
    # Prompt for installation directory and validate input
    read -p "Enter installation directory (default: current directory): " INSTALL_DIR_INPUT
    # Set base directory from input or use current default
    BASE_DIR=${INSTALL_DIR_INPUT:-$(pwd)}
    echo "Installation directory set to "$BASE_DIR""
    log_info "User input for installation directory: $INSTALL_DIR_INPUT"

    # Create the installation directory
    if ! create_install_directory "$BASE_DIR"; then
        log_error "Failed to create installation directory"
        return 1
    fi

    DEFAULT_HOST_IP=$(get_public_ip)

    # Prompt for Public IP
    printf "\nAutomatically detected public IP: ${DEFAULT_HOST_IP}\n"
    read -p "Do you want to use this public IP? (default: y) (y/n): " use_default_host_ip
    log_info "User choice for public IP: $use_default_host_ip"
    if [ "$use_default_host_ip" = "n" ]; then
        read -p "Enter your public IP (default: ${DEFAULT_HOST_IP}): " HOST_IP
        HOST_IP=${HOST_IP:-$DEFAULT_HOST_IP}
        log_info "User provided custom IP: $HOST_IP"
    else
        HOST_IP=${DEFAULT_HOST_IP}
        log_info "Using detected IP: $HOST_IP"
    fi

    DEFAULT_DOMAIN="http://${HOST_IP}:9080"

    # Prompt for Node Details
    while [[ -z "$NODE_NAME" ]]; do
        read -p "Enter your node name: " NODE_NAME
        if [[ -z "$NODE_NAME" ]]; then
            echo "❌ Node name cannot be empty. Please try again."
        fi
    done
    log_info "Node name set: $NODE_NAME"
    printf "Select a configuration type from the list below:\n"
    PS3="Select a config type (e.g. 1): "
    options=("ASTRO - Coming soon" "BEACON" "TITAN - Coming soon" "NEXUS" "ZENETH")

    while true; do
        select choice in "${options[@]}"; do
            case "$choice" in
                "ASTRO - Coming soon"|"TITAN - Coming soon")
                    echo "This configuration will be in upcoming updates. Please choose another option."
                    break  # Restart the select prompt
                    ;;
                "BEACON"|"NEXUS"|"ZENETH")
                    CONFIG="$choice"
                    echo "You selected: $CONFIG"
                    log_info "Configuration type selected: $CONFIG"
                    break 2  # Exit both select and while loops
                    ;;
                *)
                    echo "Invalid choice. Please select a valid config type."
                    ;;
            esac
        done
    done

    read -p "Enable Xray (default: n) (y/n): " enable_xray
    enable_xray=${enable_xray:-n}  # default to 'n' if empty
    log_info "Xray enable choice: $enable_xray"

    if [[ "$enable_xray" =~ ^[yY]$ ]]; then
        XRAY_ENABLED="true"
        printf "\033[0;32mXray will be enabled on this node.\033[0m\n"
        log_info "Xray enabled"
    else
        XRAY_ENABLED="false"
        printf "\033[0;31mXray will be disabled on this node.\033[0m\n"
        log_info "Xray disabled"
    fi

    # Prompt for Chain
    printf "Select valid chain from list below:\n"
    PS3="Select a chain (e.g. 1): "
    options=("SOLANA" "PEAQ")
    select CHAIN in "${options[@]}"; do
        if [ -n "$CHAIN" ]; then
            log_info "Chain selected: $CHAIN"
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
            printf "Wrong mnemonic, try again with correct mnemonic.\n"
        fi
    done

    # Prompt for Config Type
    printf "Select an access type from list below:\n"
    PS3="Select an access type (e.g. 1): "
    options=("public" "private")
    select ACCESS in "${options[@]}"; do
        if [ -n "$ACCESS" ]; then
            log_info "Access type selected: $ACCESS"
            break
        else
            echo "Invalid choice. Please select a valid access type."
        fi
    done

    # Write environment variables to .env file
    bash -c "cat > ${INSTALL_DIR}/.env" <<EOL
# Application Configuration
RUNTYPE=released
SERVER=0.0.0.0
HTTP_PORT=9080
GRPC_PORT=9003
LIBP2P_PORT=9002
XRAY_ENABLED=${XRAY_ENABLED}
REGION=$(get_region)
NODE_NAME=${NODE_NAME}
DOMAIN=${DEFAULT_DOMAIN}
HOST_IP=${HOST_IP}
GATEWAY_DOMAIN=https://gateway.erebrus.io
POLYGON_RPC=
SIGNED_BY=NetSepio
FOOTER=NetSepio 2024
GATEWAY_WALLET=0x0
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
WG_IPv4_SUBNET=10.0.0.1/16
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
    log_success "Environment file created successfully: ${INSTALL_DIR}/.env"
    return 0
}

#Stage #2 - Install Dependencies
function install_dependencies(){
    log_info "=== Starting Stage 2: Install Dependencies ==="
    # Ensure installation directory exists (in case Stage 1 was skipped)
    if ! create_install_directory "$BASE_DIR"; then
        log_error "Failed to ensure installation directory exists"
        return 1
    fi

    if [[ "$INSTALLATION_MODE" == "container" ]]; then
        install_dependencies_docker_mode &
        show_spinner $! "→ Installing Docker..."
        local status=$?
    else
        install_dependencies_binary_mode &
        show_spinner $! "→ Installing dependencies"
        local deps_status=$?       
         
        download_erebrus_binary &
        show_spinner $! "→ Downloading Erebrus binary"
        local binary_status=$?       

        if [[ "$XRAY_ENABLED" == "true" ]]; then
            download_xray_binary &
            show_spinner $! "→ Downloading Xray binary"
            local xray_status=$?       
            fi
    fi
    # Return overall status (non-zero if any subprocess failed)
    [[ $deps_status -eq 0 && $binary_status -eq 0 && ${xray_status:-0} -eq 0 ]]
    log_info "=== Finished Stage 2: Install Dependencies ==="
    return $?
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

install_dependencies_docker_mode() {
    log_info "=== Starting install_dependencies_docker_mode ==="
    printf "   → Checking Docker installation...\n"
    if is_docker_installed; then
        printf "   ✓ Docker already installed\n"
        sleep 2
    else
        printf "   → Installing Docker...\n"
        if command -v apt-get > /dev/null; then
            (sudo apt-get update -qq && sudo apt-get install -y containerd docker.io && sudo apt-get install netcat-* -y && sudo apt-get install lsof -y  >> "$LOG_FILE" 2>&1) &
        elif command -v yum > /dev/null; then
            (sudo yum install yum-utils -y && sudo yum install nmap-ncat.x86_64 -y && sudo yum install lsof -y && sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo && yum install -y docker >> "$LOG_FILE" 2>&1 && sudo systemctl start docker && sudo systemctl enable docker >> "$LOG_FILE" 2>&1) &
        elif command -v pacman > /dev/null; then
            (sudo pacman -Sy --noconfirm docker >> "$LOG_FILE" 2>&1 && sudo systemctl start docker && sudo systemctl enable docker >> "$LOG_FILE" 2>&1) &
        elif command -v dnf > /dev/null; then
            printf "   → Installing Docker on Fedora...\n"
            (sudo dnf install dnf-plugins-core && dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo && dnf install -y docker-ce docker-ce-cli containerd.io  >> "$LOG_FILE" 2>&1) &
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            printf "   → Installing Docker on macOS...\n"
            if ! command -v brew > /dev/null; then
                printf "   ✗ Homebrew not found. Please install Homebrew first.\n"
                exit 1
            fi
            (brew install --cask docker >> "$LOG_FILE" 2>&1 && open /Applications/Docker.app) &
            printf "   ✓ Docker installation complete\n"
        else
            printf "   ✗ Unsupported Linux distribution.\n"
            exit 1
        fi
        printf "   ✓ Docker installation complete\n"
    fi

    if docker --version > /dev/null 2>&1; then
        printf "   → Configuring Docker group...\n"
        # Created docker group if not exits
        if ! group_exists "docker"; then
            create_group "docker";
        fi
        if add_user_to_group "docker"; then
            if [[ $? -ne 0 ]]; then
                printf "   ✗ Failed to create group, docker configuration failed.\n"
                exit 1
            fi
        fi
        printf "   ✓ Docker configuration complete\n"
    fi
    log_info "=== Finished install_dependencies_docker_mode ==="
}

function install_dependencies_binary_mode() {
    log_info "=== Starting install_dependencies_binary_mode ==="
    create_erebrus_folder
    CURRENT_DIR=$(pwd)

    INSTALL_FAILED=false

    # Detect OS and install dependencies
    if command -v apk > /dev/null; then
        apk update >> "$LOG_FILE" 2>&1
        apk add --no-cache bash openresolv bind-tools wireguard-tools gettext inotify-tools iptables >> "$LOG_FILE" 2>&1 || INSTALL_FAILED=true
    elif command -v apt-get > /dev/null; then
        sudo apt-get update -qq >> "$LOG_FILE" 2>&1
        sudo apt-get install -y bash resolvconf dnsutils wireguard-tools gettext inotify-tools iptables systemd netcat-* lsof >> "$LOG_FILE" 2>&1 || INSTALL_FAILED=true
    elif command -v yum > /dev/null; then
        sudo yum install -y bash openresolv bind-utils wireguard-tools gettext inotify-tools iptables nmap-ncat lsof >> "$LOG_FILE" 2>&1 || INSTALL_FAILED=true
    elif command -v pacman > /dev/null; then
        sudo pacman -Sy --noconfirm bash openresolv bind-tools wireguard-tools gettext inotify-tools iptables netcat lsof >> "$LOG_FILE" 2>&1 || INSTALL_FAILED=true
    elif command -v dnf > /dev/null; then
        sudo dnf install -y bash openresolv bind-utils wireguard-tools gettext inotify-tools iptables nmap-ncat lsof >> "$LOG_FILE" 2>&1 || INSTALL_FAILED=true
    elif command -v brew > /dev/null; then
        brew install bash wireguard-tools gettext coreutils iproute2mac curl netcat lsof >> "$LOG_FILE" 2>&1 || INSTALL_FAILED=true
    else
        echo "   ✗ Unsupported Linux distribution. Exiting." | tee -a "$LOG_FILE"
        exit 1
    fi

    if [ "$INSTALL_FAILED" = true ]; then
        log_error "Some dependencies failed to install."
    fi
    log_info "=== Finished install_dependencies_binary_mode ==="
}

function download_xray_binary() {
    log_info "=== Starting download_xray_binary ==="
    XRAY_REPO="NetSepio/erebrus-xray"
    DOWNLOAD_DIR="${INSTALL_DIR}"

    # Detect OS and ARCH (same logic as erebrus binary)
    OS=$(uname | tr '[:upper:]' '[:lower:]') # "linux" or "darwin"
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        arm64 | aarch64) ARCH="arm64" ;;
        *) 
            log_error "Unsupported architecture: $ARCH"
            echo "   ✗ Unsupported architecture: $ARCH"
            return 1 
            ;;
    esac

    XRAY_BINARY_NAME="erebrus-xray-${OS}-${ARCH}"
    XRAY_PATH="$DOWNLOAD_DIR/$XRAY_BINARY_NAME"
    
    log_info "Detected OS: $OS, Architecture: $ARCH"
    log_info "Target binary: $XRAY_BINARY_NAME"

    # Check if binary already exists and is executable
    if [[ -f "$XRAY_PATH" && -x "$XRAY_PATH" ]]; then
        log_info "Erebrus-Xray binary already exists at $XRAY_PATH"
        echo "$XRAY_PATH" > "${DOWNLOAD_DIR}/xray_binary_path"
        log_success "Downloading latest Erebrus-Xray binary"
        log_info "=== Finished download_xray_binary ==="
    fi

    # Try to fetch latest release tag with better error handling
    log_info "Fetching latest Xray release information..."
    LATEST_XRAY_TAG=$(curl -s --connect-timeout 10 --max-time 30 https://api.github.com/repos/$XRAY_REPO/releases/latest 2>>"$LOG_FILE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$LATEST_XRAY_TAG" ]]; then
        log_warning "Could not fetch latest release tag from GitHub API, trying fallback method..."
        # Fallback: try to get the latest tag directly
        LATEST_XRAY_TAG=$(curl -s --connect-timeout 10 --max-time 30 "https://api.github.com/repos/$XRAY_REPO/tags" 2>>"$LOG_FILE" | grep '"name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
        
        if [[ -z "$LATEST_XRAY_TAG" ]]; then
            log_warning "GitHub API failed, using default tag 'latest'..."
            LATEST_XRAY_TAG="latest"
        fi
    fi

    log_info "Using Xray release tag: $LATEST_XRAY_TAG"
    XRAY_DOWNLOAD_URL="https://github.com/$XRAY_REPO/releases/download/$LATEST_XRAY_TAG/$XRAY_BINARY_NAME"
    log_info "Download URL: $XRAY_DOWNLOAD_URL"

    # Remove existing file if present
    if [[ -f "$XRAY_PATH" ]]; then
        rm -f "$XRAY_PATH"
        log_info "Removed existing Xray binary file"
    fi

    # Download with better error handling
    log_info "Downloading Xray binary..."
    if curl -L --connect-timeout 10 --max-time 300 -o "$XRAY_PATH" "$XRAY_DOWNLOAD_URL" >> "$LOG_FILE" 2>&1; then
        log_info "Download completed successfully"
        chmod +x "$XRAY_PATH"
        
        if [[ -f "$XRAY_PATH" && -s "$XRAY_PATH" ]]; then
            local file_size=$(stat -f%z "$XRAY_PATH" 2>/dev/null || stat -c%s "$XRAY_PATH" 2>/dev/null || echo "unknown")
            echo "$XRAY_PATH" > "${DOWNLOAD_DIR}/xray_binary_path"
            log_success "Erebrus-Xray binary downloaded successfully to $XRAY_PATH (size: $file_size bytes)"
        else
            log_error "Downloaded file is missing or empty"
            return 1
        fi
    else
        log_error "Failed to download Erebrus-Xray binary from $XRAY_DOWNLOAD_URL"
        return 1
    fi
    
    log_info "=== Finished download_xray_binary ==="
    return 0
}

function download_erebrus_binary() {
    log_info "=== Starting download_erebrus_binary ==="
    kill_port_erebrus
    REPO="NetSepio/erebrus"
    DOWNLOAD_DIR="${INSTALL_DIR}"
    #ERROR_LOG="$DOWNLOAD_DIR/erebrus_error.log"

    # Detect OS and ARCH
    OS=$(uname | tr '[:upper:]' '[:lower:]') # "linux" or "darwin"
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        arm64 | aarch64) ARCH="arm64" ;;
        *) echo "   ✗ Unsupported architecture: $ARCH" | tee "$ERROR_LOG"; log_error "Unsupported architecture: $ARCH"; return 1 ;;
    esac

    BINARY_NAME="erebrus-${OS}-${ARCH}"
    BINARY_PATH="$DOWNLOAD_DIR/$BINARY_NAME"

    # Fetch latest release tag
    LATEST_TAG=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$LATEST_TAG" ]]; then
        echo "   ✗ Failed to fetch the latest release tag." | tee "$ERROR_LOG"
        log_error "Failed to fetch the latest release tag."
        return 1
    fi

    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$BINARY_NAME"

    if [[ -f "$BINARY_PATH" ]]; then
        rm -f "$BINARY_PATH"
    fi

    curl -L -o "$BINARY_PATH" "$DOWNLOAD_URL" >> "$LOG_FILE" 2>&1

    if [[ $? -ne 0 ]]; then
        echo "   ✗ Download failed!" | tee "$ERROR_LOG"
        log_error "Download failed!"
        return 1
    fi

    chmod +x "$BINARY_PATH"

    if [[ ! -f "$BINARY_PATH" ]]; then
        echo "   ✗ Error: $BINARY_NAME not found in $DOWNLOAD_DIR!" | tee "$ERROR_LOG"
        log_error "Error: $BINARY_NAME not found in $DOWNLOAD_DIR!"
        return 1
    fi

    echo "$BINARY_PATH" > "${DOWNLOAD_DIR}/erebrus_binary_path"
    log_success "Erebrus binary downloaded successfully to $BINARY_PATH"
    log_info "=== Finished download_erebrus_binary ==="
    return 0
}

run_erebrus_container() {
    log_info "=== Starting run_erebrus_container ==="
    printf "   → Starting Erebrus container...\n"
    ENV_FILE="${INSTALL_DIR}/.env"
    sleep 2
    if [ ! -f "$ENV_FILE" ]; then
        printf "   ✗ The .env file does not exist at path: %s\n" "$ENV_FILE"
        printf "   Make sure the .env file exists and try again.\n"
        log_error "The .env file does not exist at path: $ENV_FILE"
        exit 1
    fi
    (sudo docker run -d -p 9080:9080/tcp -p 9002:9002/tcp -p 51820:51820/udp \
        --cap-add=NET_ADMIN --cap-add=SYS_MODULE \
        --sysctl="net.ipv4.conf.all.src_valid_mark=1" \
        --sysctl="net.ipv6.conf.all.forwarding=1" \
        --restart unless-stopped -v "${INSTALL_DIR}/wireguard:/etc/wireguard" \
        --name erebrus --env-file "${ENV_FILE}" ghcr.io/netsepio/erebrus:main >> "$LOG_FILE" 2>&1) &
    wait $!
    printf "   ✓ Erebrus container started\n"
    log_success "Erebrus container started"
    log_info "=== Finished run_erebrus_container ==="
}

run_erebrus_binary() {
    log_info "=== Starting run_erebrus_binary ==="
    local path="${INSTALL_DIR}/erebrus_binary_path"
    
    if [[ -f "$path" ]]; then
        local binary=$(cat "$path")
        # Change to the installation directory before running the binary
        cd "${INSTALL_DIR}" || {
            log_error "Failed to change to installation directory: ${INSTALL_DIR}"
            return 1
        }
        
        # Run the binary with sudo (should work now that we ensured credentials)
        sudo "$binary" > "${INSTALL_DIR}/erebrus.log" 2>&1 &
        EREBRUS_PID=$!    
        # Change back to original directory
        cd - > /dev/null
        
        if kill -0 "$EREBRUS_PID" 2>/dev/null; then
            log_success "Erebrus started with (PID: $EREBRUS_PID)"
            return 0
        else 
            log_error "Erebrus binary failed to start"
            return 1
        fi
    else
        log_error "Erebrus binary path not found"
        return 1
    fi
    log_info "=== Finished run_erebrus_binary ==="
}

function run_xray_binary() {
    log_info "=== Starting run_xray_binary ==="
    
    # Create config file first
    create_xray_config || {
        log_error "Failed to create Xray configuration"
        return 1
    }
    
    local path="${INSTALL_DIR}/xray_binary_path"
    if [[ -f "$path" ]]; then
        local binary=$(cat "$path")
        local config_path="${INSTALL_DIR}/config.json"
        "$binary" -c "$config_path" > "${INSTALL_DIR}/xray.log" 2>&1 &
        XRAY_PID=$!
        sleep 2

        if kill -0 "$XRAY_PID" 2>/dev/null; then
            log_success "Erebrus-Xray started (PID: $XRAY_PID) with config at $config_path"
            return 0
        else
            log_error "Erebrus-Xray process exited or failed to start"
            return 1
        fi
    else
        log_error "Xray binary path not found"
        return 1
    fi
    
    log_info "=== Finished run_xray_binary ==="
    return 0
}

# Function to create the "erebrus" folder in the current directory
function create_erebrus_folder() {
    CURRENT_DIR=$(pwd)
    FOLDER_NAME="erebrus"
    mkdir -p "$CURRENT_DIR/$FOLDER_NAME"
    if ! [ -d "$CURRENT_DIR/$FOLDER_NAME" ]; then
        return 1
    fi
}

function kill_port_erebrus() {
    local ports=(9080 9002 8088)
    
    for port in "${ports[@]}"; do
        local pids
        pids=$(sudo lsof -t -i :$port 2>/dev/null)
        
        if [[ -n "$pids" ]]; then
            log_info "Killing processes on port $port: $pids"
            echo "$pids" | xargs kill -9 2>/dev/null
        fi
    done
}

confirm_installation() {
    read -p "Do you want to continue with installation? (default: y) (y/n): " confirm
    
    # Clear the prompt line immediately after user input
    printf "\033[1A\033[2K"  # Move up one line and clear it
    
    confirm=${confirm:-y}
    if [[ "$confirm" != [Yy] ]]; then
        echo "Installation cancelled."
        exit 1
    fi
    
    if check_node_status; then
        printf "\e[33mErebrus node is already installed and running.\e[0m\n"
        printf "Refer \e[4mhttps://github.com/NetSepio/erebrus/blob/main/docs/docs.md\e[0m for API documentation.\n\n"

        while true; do
            read -p "Do you want to reinstall the node? (y/n): " confirm_reinstallation
            # Clear this prompt too
            printf "\033[1A\033[2K"
            
            case "$confirm_reinstallation" in
                [Yy])
                    break
                    ;;
                [Nn]) 
                    printf "\e[31mInstallation aborted by user\e[0m\n"
                    exit 0
                    ;;
                *) 
                    echo "Please select valid option"
                    ;;
            esac
        done
    fi
}

function create_xray_config() {
    log_info "=== Starting create_xray_config ==="
    # Create config.json file
    local config_file="$INSTALL_DIR/config.json"
    
    cat > "$config_file" <<EOL
    {
    "log": {
        "loglevel": "warning"
    },
    "inbounds": [
        {
        "port": 8088,
        "protocol": "http",
        "settings": {
            "accounts": [{"user1": "password1"}],
            "userLevel": 0,
            "authMethod": "paseto",
            "pasetoPublicKey": "1eefa289f60b539496b735936dde395ee38d776696c0a1948a8bea5fc8997940"
        },
        "streamSettings": {
            "network": "tcp"
        }
        }
    ],
    "outbounds": [
        {
        "protocol": "freedom",

        "settings": {}
        }
    ]
    }
EOL
    
    if [ ! -f "$config_file" ]; then
        log_error "Failed to create Xray config file at $config_file"
        return 1
    fi
    
    log_success "Created Xray config file at $config_file"
    log_info "=== Finished create_xray_config ==="
    return 0
}

run_node() {
    log_info "=== Starting Stage 3: Run Node ==="
    if [[ "$INSTALLATION_MODE" == "container" ]]; then
        run_erebrus_container &
        show_spinner $! "→ Starting Erebrus container"
        local container_status=$?
    else 
        run_erebrus_binary &
        show_spinner $! "→ Starting Erebrus node"
        local binary_status=$?
        
        if [[ $binary_status -eq 0 && "$XRAY_ENABLED" == "true" ]]; then
            run_xray_binary &
            show_spinner $! "→ Starting Erebrus-Xray"
            local xray_status=$?
        fi
    fi
    
    # Return overall status
    [[ ${container_status:-0} -eq 0 && ${binary_status:-0} -eq 0 && ${xray_status:-0} -eq 0 ]]
    log_info "=== Finished Stage 3: Run Node ==="
    return $?
}

# Function to mark disabled stages as skipped before running any stages
mark_disabled_stages() {
    log_info "Checking which stages are enabled/disabled..."
    
    # Check each stage and mark as skipped if disabled
    for i in {0..2}; do
        local stage_num=$((i + 1))
        if ! is_stage_enabled $stage_num; then
            STAGE_STATUS[$i]="✘ Skipped"
            log_info "Stage $stage_num is disabled/commented - marked as skipped"
        fi
    done
}

# Function to check if a stage function is available/enabled
is_stage_enabled() {
    local stage_num=$1
    case $stage_num in
        1) declare -f configure_node > /dev/null ;;
        2) declare -f install_dependencies > /dev/null ;;
        3) declare -f run_node > /dev/null ;;
        *) return 1 ;;
    esac
}

# Function to check if previous stage was successful
check_previous_stage() {
    local current_stage=$1
    local previous_stage=$((current_stage - 1))
    
    if [[ $previous_stage -ge 0 ]]; then
        local prev_status="${STAGE_STATUS[$previous_stage]}"
        if [[ "$prev_status" != "✔ Complete" && "$prev_status" != "✘ Skipped" ]]; then
            log_error "Stage $((current_stage + 1)) cannot run: Stage $((previous_stage + 1)) was not successful (Status: $prev_status)"
            return 1
        fi
    fi
    return 0
}

create_manage_script() {
    log_info "Installing node management script"
    show_spinner $! "→ Installing node management script"
    cat > ${INSTALL_DIR}/manage.sh <<'EOF'
#!/bin/bash
#Erebrus Node Management Script

# Ensure script runs with sudo/root
if [[ "$EUID" -ne 0 ]]; then
  exec sudo "$0" "$@"
fi

DEBUG=false
ARGS=()
FOLLOW_LOGS=false


print_help() {
  cat <<HELP_TEXT
Erebrus Node Management
Usage: erebrus [OPTIONS] ACTION [SERVICE]

Actions:
  start       Start the service(s)
  stop        Stop the service(s)
  status      Show status of the service(s)
  restart     Restart the service(s)
  log         Show logs of the service(s)

Services:
  node        Manage erebrus-node binary
  xray        Manage erebrus-xray binary
  (if SERVICE is omitted, action applies to both node and xray)

Options:
  -v, --verbose  Enable debug output
  -h, --help     Show this help message
  -f             Follow logs in real-time (use with 'log' action)

Examples:
  erebrus start node
  erebrus stop xray
  erebrus status
  erebrus restart -v node
  erebrus log node
  erebrus log xray -f
  erebrus log
HELP_TEXT
}

# Parse arguments and flags
for arg in "$@"; do
  case "$arg" in
    -v|--verbose) DEBUG=true ;;
    -h|--help)
      print_help
      exit 0
      ;;
    -f) FOLLOW_LOGS=true ;;
    *) ARGS+=("$arg") ;;
  esac
done

set -- "${ARGS[@]}"

log_debug() {
  if $DEBUG; then
    printf "\e[36m[DEBUG]\e[0m %s\n" "$1"
  fi
}

load_env_file() {
  INSTALL_DIR="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"
  local env_file="$INSTALL_DIR/.env"

  if [[ -f "$env_file" ]]; then
    while IFS='=' read -r key val; do
      key="$(echo "$key" | xargs)"
      val="$(echo "$val" | sed -E 's/^ *| *$//g')"
      [[ "$key" == \#* || -z "$key" ]] && continue
      val="${val%\"}"
      val="${val#\"}"
      val="${val%\'}"
      val="${val#\'}"
      export "$key=$val"
    done < "$env_file"
  fi
}

show_logs() {
  local service="$1"
  local log_args="-n 50"
  $FOLLOW_LOGS && log_args="$log_args -f"

  if [[ "$service" == "node" ]]; then
    tail $log_args "$INSTALL_DIR/erebrus.log"
  elif [[ "$service" == "xray" ]]; then
    tail $log_args "$INSTALL_DIR/xray.log"
  elif [[ -z "$service" ]]; then
    printf "\e[36m--- erebrus-node log ---\e[0m\n"
    tail $log_args "$INSTALL_DIR/erebrus.log" &
    pid1=$!
    printf "\e[36m--- erebrus-xray log ---\e[0m\n"
    tail $log_args "$INSTALL_DIR/xray.log" &
    pid2=$!
    wait $pid1 $pid2
  else
    printf "\e[31mUnknown service for log: %s\e[0m\n" "$service"
    exit 1
  fi
}

load_env_file

EREBRUS_PATH=$(cat $INSTALL_DIR/erebrus_binary_path 2>/dev/null)
XRAY_PATH=$(cat $INSTALL_DIR/xray_binary_path 2>/dev/null)
if [[ ! -x "$EREBRUS_PATH" ]]; then
  echo "Error: erebrus_binary_path is missing or not executable"
  exit 1
fi

if [[ "$XRAY_ENABLED" == "true" && ! -x "$XRAY_PATH" ]]; then
  echo "Error: xray_binary_path is missing or not executable"
  exit 1
fi

get_pids() {
  local binary="$1"
  local binary_name
  binary_name=$(basename "$binary")
  pgrep -f "$binary_name" | paste -sd ' ' -
}

start_service() {
  local name="$1"
  local binary="$2"

  log_debug "Starting $name with binary: $binary"

  local pids
  pids=$(get_pids "$binary")
  if [[ -n "$pids" ]]; then
    printf "\e[32m%s is already running (PIDs: %s)\e[0m\n" "$name" "$(echo "$pids" | paste -sd ',' -)"
  else
    if [[ "$name" == "erebrus-node" ]]; then
      "$binary" > "$INSTALL_DIR/erebrus.log" 2>&1 &
    elif [[ "$name" == "erebrus-xray" ]]; then
      "$binary" -c "$INSTALL_DIR/config.json" > "$INSTALL_DIR/xray.log" 2>&1 &
    else
      "$binary" > /dev/null 2>&1 &
    fi
    printf "\e[32m%s started (PID: %s)\e[0m\n" "$name" "$!"
  fi
}

stop_service() {
  local name="$1"
  local binary="$2"

  log_debug "Stopping $name with binary: $binary"

  local pids
  pids=$(get_pids "$binary")
  if [[ -n "$pids" ]]; then
    echo "$pids" | xargs kill
    printf "\e[31m%s stopped (PIDs: %s)\e[0m\n" "$name" "$pids"
  else
    printf "\e[31m%s is not running\e[0m\n" "$name"
  fi
}

status_service() {
  local name="$1"
  local binary="$2"

  log_debug "Checking status of $name with binary: $binary"

  local pids
  pids=$(get_pids "$binary")
  if [[ -n "$pids" ]]; then
    printf "\e[32m%s is running (PIDs: %s)\e[0m\n" "$name" "$pids"
  else
    printf "\e[31m%s is not running\e[0m\n" "$name"
  fi
}

ACTION="$1"
SERVICE="$2"

run_action() {
  local action="$1"
  local service="$2"
  local binary name

  if [[ "$action" != "log" ]]; then
    case "$service" in
      node)
        binary="$EREBRUS_PATH"
        name="erebrus-node"
        ;;
      xray)
        if [[ "$XRAY_ENABLED" != "true" ]]; then
          printf "\e[31mXray is not installed on this node\e[0m\n"
          return
        fi
        binary="$XRAY_PATH"
        name="erebrus-xray"
        ;;
      *)
        printf "\e[31mUnknown service: %s\e[0m\n" "$service"
        exit 1
        ;;
    esac
  fi

  case "$action" in
    start) start_service "$name" "$binary" ;;
    stop) stop_service "$name" "$binary" ;;
    status) status_service "$name" "$binary" ;;
    restart)
      stop_service "$name" "$binary"
      sleep 1
      start_service "$name" "$binary"
      ;;
    log) show_logs "$service" ;;
    *)
      printf "\e[31mInvalid action: %s\e[0m\n" "$action"
      exit 1
      ;;
  esac
}

if [[ -z "$ACTION" ]]; then
  print_help
  exit 1
fi

if [[ -z "$SERVICE" ]]; then
  run_action "$ACTION" node
  run_action "$ACTION" xray
else
  run_action "$ACTION" "$SERVICE"
fi
EOF

    chmod +x ${INSTALL_DIR}/manage.sh
    sudo ln -s ${INSTALL_DIR}/manage.sh /usr/local/bin/erebrus
    log_info "manage.sh script created and made executable."
    return $?
}

# Run stage1
# For each run_stage function, change the order of operations:
run_stage_1() {
    if declare -f configure_node > /dev/null; then
        STAGE_STATUS[0]="In Progress"
        display_header  # Update header BEFORE running the function
        
        if configure_node; then
            STAGE_STATUS[0]="✔ Complete"
            display_header  # Update header AFTER status change
            echo "✅ Stage 1: Node configuration completed!"
            log_success "Stage 1: Node configuration completed!"
        else
            STAGE_STATUS[0]="✘ Failed"
            display_header  # Update header AFTER status change
            echo "❌ Stage 1: Node configuration failed!"
            log_error "Stage 1: Node configuration failed!"
        fi
        
        sleep 3
    else
        STAGE_STATUS[0]="✘ Skipped"
        display_header
        echo "⏭️ Stage 1: Configuration skipped"
        log_info "Stage 1: Configuration skipped"
        sleep 3
    fi
}

# Run stage2
run_stage_2() {
    # Check if previous stage was successful
    if ! check_previous_stage 1; then
        STAGE_STATUS[1]="✘ Blocked"
        display_header
        echo "🚫 Stage 2: Dependencies installation blocked due to previous stage failure"
        log_error "Stage 2: Dependencies installation blocked due to previous stage failure"
        sleep 2
        return 1
    fi
    
    if declare -f install_dependencies > /dev/null; then
        STAGE_STATUS[1]="In Progress"   
        display_header     
        if install_dependencies; then
            if create_manage_script; then
                STAGE_STATUS[1]="✔ Complete"
                display_header
                echo "✅ Stage 2: Dependencies installed successfully & node management script installed!"
                log_success "Stage 2: Dependencies installed successfully and node management script installed!"
            else
                STAGE_STATUS[2]="✘ Failed"
                display_header
                echo "❌ Stage 2: Node management script installation failed"
                log_error "Stage 2: Installing dependencies completed, but Node management script installation failed"
            fi
        else
            STAGE_STATUS[1]="✘ Failed"
            echo "❌ Stage 2: Dependencies installation failed!"
            log_error "Stage 2: Dependencies installation failed!"
        fi
        
        sleep 3
        # clear_subprocess_output  # Add this line
    else
        STAGE_STATUS[1]="✘ Skipped"
        display_header
        echo "⏭️ Stage 2: Dependencies installation skipped"
        log_info "Stage 2: Dependencies installation skipped"
        sleep 3
        # clear_subprocess_output  # Add this line
    fi
}

# Run stage3
run_stage_3() {
    # Check if previous stage was successful
    if ! check_previous_stage 2; then
        STAGE_STATUS[2]="✘ Blocked"
        display_header
        echo "🚫 Stage 3: Node startup blocked due to previous stage failure"
        log_error "Stage 3: Node startup blocked due to previous stage failure"
        sleep 3
        return 1
    fi
    
    if declare -f run_node > /dev/null; then
        STAGE_STATUS[2]="In Progress"
        display_header
        if run_node; then
            if validate_post_install; then
                STAGE_STATUS[2]="✔ Complete"
                display_header
                echo "✅ Stage 3: Node started and validated successfully!"
                log_success "Stage 3: Node started and validated successfully!"
            else
                STAGE_STATUS[2]="✘ Failed"
                display_header
                echo "❌ Stage 3: Node started but validation failed"
                log_error "Stage 3: Node started but validation failed"
            fi
        else
            STAGE_STATUS[2]="✘ Failed"
            display_header
            echo "❌ Stage 3: Failed to start node"
            log_error "Stage 3: Failed to start node"
        fi
        
        sleep 3
    else
        STAGE_STATUS[2]="✘ Skipped"
        display_header
        echo "⏭️ Stage 3: Node startup skipped"
        log_info "Stage 3: Node startup skipped"
        sleep 3
    fi
}

# Cleanup function to restore terminal on exit
cleanup() {
    tput cnorm  # Show cursor
}

# Set trap to cleanup on exit
trap cleanup EXIT

#####################################################################################################################
# Main script execution starts here
#####################################################################################################################
STAGE_STATUS=("Pending" "Pending" "Pending")
INSTALLATION_MODE="binary"  #valid options "binary" , "container"
XRAY_ENABLED="false"

# Set default directories
BASE_DIR=$(pwd)
INSTALL_DIR="$BASE_DIR/erebrus"

init_logging
display_header  # Show header once
confirm_installation
mark_disabled_stages # Mark disabled stages as skipped before running any stages

# Only update header once after marking disabled stages
display_header

# Run Stages
run_stage_1
run_stage_2
run_stage_3

# Final status update
for i in {0..2}; do
    if [[ "${STAGE_STATUS[$i]}" == "Pending" ]]; then
        STAGE_STATUS[$i]="✘ Skipped"
    fi
done
display_header

# Print final message
echo ""
print_final_message

# Show cursor again
tput cnorm
if [ -n "$BASH_VERSION" ]; then
  hash -r
elif [ -n "$ZSH_VERSION" ]; then
  rehash
fi
