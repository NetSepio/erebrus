#!/bin/bash

# Function to display header and stage status
display_header() {
    clear
    printf "\e[1m\e[4m=== Node Software Installation Script (Version 3) ===\e[0m\n"
    printf "%0.s=" {1..80}  # Print a line separator of 80 characters
    printf "\n"
    printf "\n\e[1mRequirements:\e[0m\n"
    printf "1. Erebrus node needs public IP that is routable from internet.\n"
    printf "   Node software requires public IP to funtion properly.\n"
    printf "2. Ports 9080 and 9002 must be open on your firewall and/or host system.\n"
    printf "   Ensure these ports are accessible to run the Erebrus Node software.\n"
    printf "%0.s=" {1..80}  # Print a line separator of 80 characters
    printf "\n"
    printf "\n\e[1mStage 1 - Install Dependencies:\e[0m\t       [${status_stage1}\e[0m]\n"
    printf "\e[1mStage 2 - Configure Node:\e[0m\t       [${status_stage2}\e[0m]\n"
    printf "\e[1mStage 3 - Run Node:\e[0m\t               [${status_stage3}\e[0m]\n\n"
}

# Function to show spinner
show_spinner() {
    local pid=$1
    local delay=0.2
    local spinstr='|/-\'
    printf " ["
    while [ "$(ps a | awk '{print $1}' | grep $pid)" ]; do
        local temp=${spinstr#?}
        printf "%c" "$spinstr"
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b"
    done
    printf " ]\b\b\b\b\b\b\t"
}

# Function to install Docker and Docker Compose based on distribution
install_dependencies() {
    clear
    printf "\e[1mInstalling Docker and Docker Compose...\e[0m\n"
    status_stage1="\e[34mIn Progress\e[0m"
    display_header

    if command -v apt-get > /dev/null; then
        printf "Installing Docker on Debian/Ubuntu..."
        (sudo apt-get update -qq && sudo apt-get install -y docker docker-compose > /dev/null 2>&1) &
        show_spinner $!
        printf " \e[32mComplete\e[0m\n"
    elif command -v yum > /dev/null; then
        printf "Installing Docker on Red Hat/CentOS..."
        (sudo yum install -y docker docker-compose > /dev/null 2>&1 && sudo systemctl start docker && sudo systemctl enable docker) &
        show_spinner $!
        printf " \e[32mComplete\e[0m\n"
    elif command -v pacman > /dev/null; then
        printf "Installing Docker on Arch Linux..."
        (sudo pacman -Sy --noconfirm docker docker-compose > /dev/null 2>&1 && sudo systemctl start docker && sudo systemctl enable docker) &
        show_spinner $!
        printf " \e[32mComplete\e[0m\n"
    elif command -v dnf > /dev/null; then
        printf "Installing Docker on Fedora..."
        (sudo dnf install -y docker docker-compose > /dev/null 2>&1 && sudo systemctl start docker && sudo systemctl enable docker) &
        show_spinner $!
        printf " \e[32mComplete\e[0m\n"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        printf "Installing Docker on macOS..."
        if ! command -v brew > /dev/null; then
            printf "Homebrew not found. Please install Homebrew first.\n"
            exit 1
        fi
        (brew install --cask docker > /dev/null 2>&1 && open /Applications/Docker.app) &
        show_spinner $!
        printf " \e[32mComplete\e[0m\n"
        printf "Please ensure Docker is running from the macOS toolbar.\n"
    else
        printf "Unsupported Linux distribution.\n"
        exit 1
    fi

    if docker --version > /dev/null 2>&1 && docker-compose --version > /dev/null 2>&1; then
        status_stage1="\e[32mComplete\e[0m"
        error_stage1=""
    else
        status_stage1="\e[31mFailed\e[0m"
        error_stage1="\e[31mFailed to install Docker and Docker Compose.\e[0m\n"
    fi
    display_header
}

# Function to get the public IP address
get_public_ip() {
    curl -s ifconfig.me
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
        printf "\n\e[1mChecking IP reachability from internet...\e[0m"
        show_spinner $$ &  # Start the spinner
        spinner_pid=$!

        # Start a netcat listener in the background
        (nc -l $port > /dev/null 2>&1 &)
        listener_pid=$!
        sleep 2  # Give the listener time to start

        # Try to connect to the listener using netcat
        if echo "test" | nc -w 3 $host_ip $port > /dev/null 2>&1; then
            kill $listener_pid
            kill $spinner_pid  # Stop the spinner
            printf " \e[32mComplete\e[0m\n"
            printf "\nIP address %s is reachable from the internet on port %d.\n" "$host_ip" "$port"
            return 0
        else
            if [ $retry -lt $max_retries ]; then
                printf "\nThe IP address %s is not reachable from internet. IP reachability test failed.\n" "$host_ip"
                printf "Make sure port 9002 and 9080 are open on your firewall and/or host system and try again.\n"
                
                kill $spinner_pid  # Stop the spinner to interact with user
                
                read -p "Would you like to retry? (y/n): " user_retry_choice
                if [ "$user_retry_choice" != "y" ]; then
                    kill $listener_pid
                    return 1
                fi
            else
                printf "\e[31mFailed\e[0m\n"
                printf "\nYou do not have a public IP that is routable and reachable from internet.\n"
                kill $listener_pid
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


# Function to determine default network interface
get_default_interface() {
    if command -v ip >/dev/null; then
        ip route get 1.1.1.1 | grep -oP '(?<=dev )(\S+)' | sort -u
    elif command -v ifconfig >/dev/null; then
        ifconfig | awk '/^[a-z]/ { iface=$1; next } /inet / { print iface; }' | sort -u
    fi
}

# Function to prompt user to select network interface
select_interface() {
    PS3="Select an interface to run Erebrus VPN service on: "
    select INTERFACE in $(get_default_interface); do
        if [ -n "$INTERFACE" ]; then
            echo "$INTERFACE"
            break
        else
            echo "Invalid choice. Please select a valid interface."
        fi
    done
}

check_node_status() {
    local container_running=0
    local port_9080_listening=0
    local port_9002_listening=0

    # Check if container 'erebrus' is running
    if docker ps -f name=erebrus | grep erebrus >/dev/null; then
        container_running=1
    fi

    # Check if ports 9080 and 9002 are listening using lsof command
    local lsof_output=$(lsof -nP -iTCP -sTCP:LISTEN)
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
    status_stage2="\e[34mIn Progress\e[0m"
    display_header

    # Prompt for installation directory and validate input
    while true; do
        read -p "Enter installation directory (default: current directory): " INSTALL_DIR
        INSTALL_DIR=${INSTALL_DIR:-$(pwd)}

        if [ ! -d "$INSTALL_DIR" ]; then
            printf "Error: Directory '%s' does not exist. Please enter a valid directory.\n" "$INSTALL_DIR"
        else
            break
        fi
    done

    DEFAULT_HOST_IP=$(get_public_ip)
    DEFAULT_DOMAIN="http://${DEFAULT_HOST_IP}:9080"

    # Prompt for network interface
    printf "\nAutomatically detected HOST_IP: ${DEFAULT_HOST_IP}\n"
    read -p "Do you want to use this as the default HOST_IP? (y/n): " use_default_host_ip
    if [ "$use_default_host_ip" = "n" ]; then
        read -p "Enter HOST_IP (default: ${DEFAULT_HOST_IP}): " HOST_IP
        HOST_IP=${HOST_IP:-$DEFAULT_HOST_IP}
    else
        HOST_IP=${DEFAULT_HOST_IP}
    fi

    # Prompt for network interface
    INTERFACE=$(select_interface)

    # Display and confirm user-provided variables
    printf "\n\e[1mUser Provided Configuration:\e[0m\n"
    printf "INSTALL DIR=%s\n" "${INSTALL_DIR}"
    printf "REGION=CA\n"
    printf "HOST_IP=%s\n" "${HOST_IP}"
    printf "DOMAIN=%s\n" "${DEFAULT_DOMAIN}"
    printf "INTERFACE=%s\n" "${INTERFACE}"

    read -p "Confirm configuration (y/n): " confirm
    if [ "${confirm}" != "y" ]; then
        printf "Configuration not confirmed. Exiting.\n"
        exit 1
    fi

    # Validate and test IP reachability
    test_ip_reachability "$HOST_IP"
    if [ $? -eq 1 ]; then
        status_stage2="\e[31mFailed\e[0m\n"
        error_stage2="\e[31mFailed to configure Erebrus node.\e[0m\n"
        return 1
    else
    # Write environment variables to .env file
    cat <<EOL > "${INSTALL_DIR}/.env"
RUNTYPE=debug
SERVER=0.0.0.0
HTTP_PORT=9080
GRPC_PORT=9090
MASTERNODE_URL=https://gateway.erebrus.io
WG_CONF_DIR=/etc/wireguard
WG_CLIENTS_DIR=/etc/wireguard/clients
WG_INTERFACE_NAME=wg0.conf
WG_ENDPOINT_HOST=34.130.230.82
WG_ENDPOINT_PORT=51820
WG_IPv4_SUBNET=10.0.0.1/24
WG_IPv6_SUBNET=fd9f:0000::10:0:0:1/64
WG_DNS=1.1.1.1
WG_ALLOWED_IP_1=0.0.0.0/0
WG_ALLOWED_IP_2=::/0
WG_PRE_UP=echo WireGuard PreUp
WG_POST_UP=iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o ${INTERFACE} -j MASQUERADE
WG_PRE_DOWN=echo WireGuard PreDown
WG_POST_DOWN=iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o ${INTERFACE} -j MASQUERADE
PASETO_EXPIRATION_IN_HOURS=168
AUTH_EULA=I Accept the NetSepio Terms of Service https://netsepio.com/terms.html for accessing the application. Challenge ID:
SIGNED_BY=NetSepio
FOOTER=NetSepio 2024
GATEWAY_DOMAIN=https://gateway.erebrus.io
REGION=CA
HOST_IP=${HOST_IP}
DOMAIN=${DEFAULT_DOMAIN}
INTERFACE=${INTERFACE}
EOL
        status_stage2="\e[32mComplete\e[0m"
        #display_header
    fi
}

# Function to run the Node container
run_node() {
    clear
    printf "\n\e[1mRunning Erebrus Node...\e[0m"
    status_stage3="\e[34mIn Progress\e[0m"
    display_header

    printf "Starting Erebrus Node...\n"
    (docker run -d -p 9080:9080/tcp -p 9002:9002/tcp -p 51820:51820/udp \
        --cap-add=NET_ADMIN --cap-add=SYS_MODULE \
        --sysctl="net.ipv4.conf.all.src_valid_mark=1" \
        --sysctl="net.ipv6.conf.all.forwarding=1" \
        --restart unless-stopped -v "${INSTALL_DIR}:/install_dir" \
        --name erebrus --env-file "${INSTALL_DIR}/.env" ghcr.io/netsepio/erebrus:main > /dev/null 2> error.log) &
    show_spinner $!

    wait $!

    if [ $? -eq 0 ]; then
        status_stage3="\e[32mComplete\e[0m"
        error_stage3=""
    else
        status_stage3="\e[31mFailed\e[0m"
        error_stage3="\e[31mFailed to run Erebrus node. See error.log for details.\e[0m\n"
    fi
    display_header
}

# Main script execution starts here

clear
display_header

read -p "Do you want to continue with installation? (y/n): " confirm_installation
if [ "${confirm_installation}" != "y" ]; then
    printf "Installation canceled.\n"
    exit 1
fi

if check_node_status; then
    status_stage1="\e[33mSkipped\e[0m"
    status_stage2="\e[33mSkipped\e[0m"
    status_stage3="\e[33mSkipped\e[0m"
    display_header
    printf "\e[31mErebrus node is already installed and running. Aborting installation.\e[0m\n"
    printf "Refer \e[4mhttps://github.com/NetSepio/erebrus/blob/main/docs/docs.md\e[0m for API documentation.\n\n"
    exit 0
fi

status_stage1="\e[33mPending\e[0m"
status_stage2="\e[33mPending\e[0m"
status_stage3="\e[33mPending\e[0m"

install_dependencies
if [ -n "${error_stage1}" ]; then
    printf "%s${error_stage1}"
    exit 1
else
    configure_node
    if [ -n "${error_stage2}" ]; then
        printf "%s${error_stage2}"
        exit 1
    else
        run_node
        if [ -n "${error_stage3}" ]; then
            printf "%s${error_stage3}"
            exit 1
        else
            print_final_message
        fi
    fi
fi
