#!/usr/bin/env bash
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

x=$(pgrep -x erebrus |wc -l)
y=$(pgrep -x wg-watcher.sh|wc -l)

printf "${NC}Checking Node status\n"
sleep 5 
if [ $x -ge 1 ] && [ $y -ge 1 ]
then
   printf "${GREEN}Erebrus Node is running\n"

ip=$(curl -s ifconfig.io)


printf "${NC}Checking port connectivity ...............\n"

timeout 10s nc -zv $ip 9080 >> /dev/null 2>&1
if [ $? -eq 0 ]
then
   printf "${GREEN}Port 9080 is accessible\n"
else
   printf "${RED}Port 9080 is not accessible\n"
fi

timeout 10s nc -zv $ip 9002 >> /dev/null 2>&1
if [ $? -eq 0 ]
then
   printf "${GREEN}Port 9002 is accessible\n"
else
   printf "${RED}Port 9002 is not accessible\n"
fi

timeout 10s nc -zvu $ip 51820 >> /dev/null 2>&1 
if [ $? -eq 0 ]
then
   printf "${GREEN}Port 51820 is accessible\n"
else
   printf "${RED}Port 51820 is not accessible\n"
fi
exit 0
else
   printf "${RED}Erebrus Node is not running\n"
   exit 1
fi
