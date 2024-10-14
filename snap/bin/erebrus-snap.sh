#!/bin/bash

version="1.0.0"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

case "$1" in
  -h | --help)
    echo "Usage: sudo erebrus-node [OPTION]"
    echo ""
    echo "Options:"
    echo "  -h, --help        Print this help message."
    echo "  -c, --configure   Install dependencies & Configure the Erebrus node application."
    echo "  -v, --version     Display the version of this script."
    echo "  --start   start the service"
    echo "  --stop    stop the service"
    echo "  --restart Rstart the service"
    echo "  --status  Get service status"
    echo ""
    echo "Example:"
    echo "  sudo erebrus-node --configure"
    echo "  sudo erebrus-node  --version"
    exit 0
    ;;
  -c | --configure)
    ${SNAP}/setup-node.sh 
    exit 0
    ;;
  -v | --version)
    echo "Erebus Node"
    echo "Version: $version"
    exit 0
    ;;
  -s | --status)
    ${SNAP}/status-node.sh
    exit 0
    ;;
  --start)
    printf "${NC}Starting Erebrus Node .............\n"
    ${SNAP}/start-node.sh >> ${SNAP_COMMON}/erebrus.log 2>&1
    if [ $? -eq 0 ];then
       printf "${GREEN}Erebrus Node is started successfully\n"
       sleep 10
       ${SNAP}/status-node.sh
    else
       printf "${RED}Ereburs Node could not be started\n"
    fi
    exit 0
    ;;
  --stop)
    printf "${NC}Stopping Erebrus Node ............\n"
    ${SNAP}/stop-node.sh >> ${SNAP_COMMON}/erebrus.log 2>&1
    if [ $? -eq 0 ];then
	printf "${GREEN}Ereburs Node stopped successfully\n"
    else
        printf "${RED}Could not stop Erebrus Node\n"
    fi
    exit 0
    ;;
  --restart)
    printf "${NC}Stopping Erebrus Node ............\n"
    ${SNAP}/stop-node.sh >> ${SNAP_COMMON}/erebrus.log 2>&1
    if [ $? -eq 0 ];then
        printf "${GREEN}Ereburs Node stopped successfully\n"
    else
        printf "${RED}Could not stop Erebrus Node\n"
    fi

    printf "${NC}Starting Erebrus Node .............\n"
    ${SNAP}/start-node.sh >> ${SNAP_COMMON}/erebrus.log 2>&1
    if [ $? -eq 0 ];then
       printf "${GREEN}Erebrus Node is started successfully\n"
       sleep 10
       ${SNAP}/status-node.sh
    else
       printf "${RED}Ereburs Node could not be started\n"
    fi
    exit 0
    ;;
  *)
    echo "Invalid option: $1"
    exit 1
    ;;
esac

