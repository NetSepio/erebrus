
# Erebrus

Erebrus, a decentralized VPN, that ensures your privacy, security, and transparent data practices. It's open-source, ensuring no hidden data tracking or logging. Complementing it, our DePIN initiative lets anyone worldwide participate as a node, contributing either physical servers or virtual machines and earning incentives. With this, we pave the way for safer, decentralized Wi-Fi hotspots, making unreliable public Wi-Fi a thing of the past.

For more details visit [here](https://erebrus.io).  

## Features

- Easy Client and Server management.
- LibP2P integration for peer discovery
- Supports REST and gRPC (QUIC upcoming).
- Email VPN configuration to clients easily.
- Solana blockchain integration for secure node registration and management.

## Deploy Erebrus Node

- Refer docs here [setup docs](https://github.com/NetSepio/erebrus/blob/main/docs/node.md).

## Get Started

To deploy Erebrus, you need to follow the documentation given below,

- First you will need to setup Wireguard and Watcher, for that use [setup docs](https://github.com/NetSepio/erebrus/blob/main/docs/setup.md).
- After setup , you will have choices for deploying Erebrus. Refer [Deploy docs](https://github.com/NetSepio/erebrus/blob/main/docs/deploy.md)


### Solana Integration

Erebrus uses the Solana blockchain for node registration and management. To set up the Solana integration:

1. Install the Solana CLI tools.
2. Set up a Solana wallet with sufficient SOL for transaction fees.
3. Set the following environment variables:
   - `SOLANA_RPC_URL`: URL of the Solana RPC node (optional, defaults to MainNet Beta)
   - `SMART_CONTRACT_PUBLIC_KEY`: Public key of the Erebrus smart contract on Solana
   - `SENDER_PRIVATE_KEY`: Private key of the Solana wallet used for transactions
   - `NODE_NAME`: Name of your VPN node


## API Docs

Download Postman collection for Erebrus from [here](https://github.com/NetSepio/erebrus/blob/main/docs/Erebrus.postman_collection.json). There are two types of docs available:

- You can refer docs from github [here](https://github.com/NetSepio/erebrus/blob/main/docs/docs.md)
- There is a web based doc available on Erebrus route /docs.You can refer it after deployment

## Solana Smart Contract

Erebrus interacts with a custom Solana smart contract for node management. The `register_vpn_node` function in the contract handles node registration with the following parameters:
- Device ID
- DID (Decentralized Identifier)
- Node Name
- IP Address
- ISP Information
- Region
- Location
