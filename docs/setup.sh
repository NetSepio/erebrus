#!/bin/bash
# setup erebrus node on ubuntu/debian server 

# Update the package index
sudo apt-get update
sudo apt-get install ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Configure Docker to be used without root
sudo groupadd docker
sudo usermod -aG docker $USER
newgrp docker

# Start Docker services
sudo systemctl start docker
sudo systemctl enable docker

docker run -d -p 9080:9080/tcp -p 51820:51820/udp --cap-add=NET_ADMIN --cap-add=SYS_MODULE --sysctl="net.ipv4.conf.all.src_valid_mark=1" --sysctl="net.ipv6.conf.all.forwarding=1" --restart unless-stopped --name erebrus --env-file .env ghcr.io/netsepio/erebrus:main
