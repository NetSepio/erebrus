# Host your Erebrus node

## Install and Deploy using Docker
  
1. Install docker using this [script](https://github.com/NetSepio/erebrus/blob/main/docs/setup.md) (For Ubuntu server). Or refer the official [documentation](https://docs.docker.com/engine/install) 

2. create a .env file in same directory and define the environment for erebrus . you can use template from [.sample-env](https://github.com/NetSepio/erebrus/blob/main/.sample-env). Make sure to put the correct server URL. Example:
```
NODE_NAME=blazing_icarus"
DOMAIN=http://255.255.255.255:9080
HOST_IP=255.255.255.255
WG_ENDPOINT_HOST=255.255.255.255
```
replace `255.255.255.255` with the server IP address

3. Open incoming request to ports: TCP Ports `9080`(http),` 9090`(gRPC),` 9002`(p2p) and UDP port `51820` of your server to communicate with the gateway

4. Pull the ererbus docker image
```
docker pull ghcr.io/netsepio/erebrus:main
```
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