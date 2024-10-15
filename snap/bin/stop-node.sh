#!/usr/bin/env bash
x=$(pgrep -x erebrus |wc -l)
y=$(pgrep -x wg-watcher.sh|wc -l)
a="x"
b="x"

if [ $x -eq 0 ] && [ $y -eq 0 ]
then
   echo "Node is already stopped"
   exit 0
elif [ $x -gt 0 ] && [ $y -eq 0 ]
then
   pkill -9 erebrus & > /dev/null 2>&1
   if [ $? -ne 0 ];then
     a="Issue with killing erebrus process"
   fi
elif [ $x -eq 0 ] && [ $y -gt 0 ]
then
   pkill -9 wg-watcher.sh & > /dev/null 2>&1
   if [ $? -ne 0 ];then
     b="Issue with killing wg-watcher process"
   fi
else
   pkill -9 erebrus & > /dev/null 2>&1
   if [ $? -ne 0 ];then
      a="Issue with killing erebrus process"
   fi
   pkill -9 wg-watcher.sh &  > /dev/null 2>&1
   if [ $? -ne 0 ];then
      b="Issue with killing wg-watcher process"
   fi
fi

if [ "$a" == "x" ] && [ "$b" == "x" ]
then    
   echo "Node is stopped"
   exit 0
elif [ "$a" == "x" ] && [ "$b" != "x" ]
then
   echo $b
   exit 1
elif [ "$a" != "x" ] && [ "$b" == "x" ]
then
   echo $a
   exit 1
else
   echo $a
   echo $b
   exit 1
fi
   
