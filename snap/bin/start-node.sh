#!/usr/bin/env bash
RED='\033[0;31m'
GREEN='\033[0;32m'
sysctl -w net.ipv4.ip_forward=1

source ${SNAP_COMMON}/.env

x=$(pgrep -x erebrus |wc -l)
y=$(pgrep -x wg-watcher.sh|wc -l)

if [ $x -ge 1 ] && [ $y -ge 1 ]
then
   echo "Node is already running"
   exit 0
else
  set -eo pipefail
  erebrus &
  if [ $? -eq 0 ];then  
  	wg-watcher.sh &
	if [ $? -eq 0 ];then
	   exit 0
	else 
	   exit 1
	fi
  else
    exit 1
  fi  
fi

