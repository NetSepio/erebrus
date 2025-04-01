#!/bin/bash

# Function to remove Wireguard and dependencies including binaries
remove_wireguard() {
    clear
    echo -e "\e[1;34mRemoving Wireguard and Dependencies...\e[0m"

    # Detect OS and remove Wireguard packages
    if command -v apk > /dev/null; then
        echo "Removing on Alpine Linux..."
        apk del wireguard-tools openresolv bind-tools gettext inotify-tools iptables bash
    elif command -v apt-get > /dev/null; then
        echo "Removing on Debian-based systems..."
        apt-get remove --purge -y wireguard-tools resolvconf dnsutils gettext inotify-tools iptables bash
        apt-get autoremove --purge -y
    elif command -v yum > /dev/null; then
        echo "Removing on RHEL-based systems..."
        yum remove -y wireguard-tools openresolv bind-utils gettext inotify-tools iptables bash
    elif command -v pacman > /dev/null; then
        echo "Removing on Arch-based systems..."
        pacman -Rns --noconfirm wireguard-tools openresolv bind-tools gettext inotify-tools iptables bash
    elif command -v dnf > /dev/null; then
        echo "Removing on Fedora..."
        dnf remove -y wireguard-tools openresolv bind-utils gettext inotify-tools iptables bash
    elif command -v brew > /dev/null; then
        echo "Removing on macOS..."
        brew uninstall wireguard-tools gettext coreutils iproute2mac bash
    else
        echo "Unsupported Linux distribution. Exiting."
        exit 1
    fi

    # Remove Wireguard binaries and config files from /usr/bin, /usr/sbin, /bin, /sbin, /etc
    echo "Cleaning up remaining Wireguard binaries and configurations..."
    rm -f /usr/bin/wg* /usr/sbin/wg* /bin/wg* /sbin/wg* /etc/wireguard/*

    # Remove any Wireguard kernel module (optional, may require sudo privileges)
    if lsmod | grep -q wireguard; then
        echo "Removing Wireguard kernel module..."
        modprobe -r wireguard
    fi

    # Check if Wireguard config directory exists and remove it
    if [ -d "/etc/wireguard" ]; then
        echo "Removing Wireguard configuration files..."
        rm -rf /etc/wireguard
    fi

    # Check for Wireguard network interfaces and remove if present (optional)
    if ip link show | grep -q wg; then
        echo "Removing Wireguard network interfaces..."
        ip link delete wg0
    fi

    echo -e "\e[32mWireguard and dependencies have been removed successfully.\e[0m"
}

# Execute the function
remove_wireguard
