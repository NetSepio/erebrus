# Host your Erebrus node

## Install and Deploy using Docker
  
1. Install docker [(setup docs)](https://github.com/NetSepio/erebrus/blob/main/docs/setup.md).

2. create a .env file in same directory and define the environment for erebrus . you can use template from [.sample-env](https://github.com/NetSepio/erebrus/blob/main/.sample-env). Make sure to put the correct server URL.

3. Open incoming request to ports: TCP Ports 9080(http), 9090(gRPC), 9001(p2p) and UDP port 51820 of your server to communicate with the gateway  

4. Pull the ererbus docker image

5. Run the Image

```
docker run -d -p 9080:9080/tcp -p 9002:9002/tcp -p 51820:51820/udp \
--cap-add=NET_ADMIN --cap-add=SYS_MODULE \
--sysctl="net.ipv4.conf.all.src_valid_mark=1" \
--sysctl="net.ipv6.conf.all.forwarding=1" \
--restart unless-stopped \
-v ~/wireguard/:/etc/wireguard/ \
--name erebrus --env-file .env \
ghcr.io/netsepio/erebrus:main

```
