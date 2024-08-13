#!/bin/bash

version="1.0.0"

case "$1" in
  -h | --help)
    echo "Usage: $0 [OPTION]"
    echo ""
    echo "Options:"
    echo "  -h, --help        Print this help message."
    echo "  -c, --configure   Install dependencies & Configure the Erebrus node application."
    echo "  -v, --version     Display the version of this script."
    echo ""
    echo "Example:"
    echo "  $0 --install"
    echo "  $0 --version"
    exit 0
    ;;
  -c | --configure)
    ${SNAP}/install-node.sh
    exit 0
    ;;
  -v | --version)
    echo "Erebus Node"
    echo "Version: $version"
    exit 0
    ;;
  -s | --status)
    if docker ps --filter name=erebrus | grep -q erebrus; then
      echo "Erebrus node is running..."
    else
      echo "Erebrus node is not running..."
    fi
    ;;
  *)
    echo "Invalid option: $1"
    exit 1
    ;;
esac

