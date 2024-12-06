// The following variables will be dynamically injected by the Go code

const peaqDid = "did:peaq:testnode123";  // Placeholder for dynamic Peaq DID
const nodename = "TestVPNNode";  // Placeholder for dynamic Node Name
const ipaddress = "192.168.1.100";   // Placeholder for dynamic IP Address
const ispinfo = "TestISP";   // Placeholder for dynamic ISP Information
const region = "EU-Central";     // Placeholder for dynamic Region
const location = "Frankfurt, Germany"; // Placeholder for dynamic Location

// Log the values to check if they're injected correctly
console.log("Peaq DID:", peaqDid);
console.log("Node Name:", nodename);
console.log("IP Address:", ipaddress);
console.log("ISP Info:", ispinfo);
console.log("Region:", region);
console.log("Location:", location);

const {
  Connection,
  PublicKey,
  Keypair,
  SystemProgram,
  LAMPORTS_PER_SOL,
} = require("@solana/web3.js");
const { Program, AnchorProvider, BN } = require("@project-serum/anchor");
const fs = require("fs");
const path = require("path");
const bs58 = require("bs58"); // Import the base58 decoding library

// Program ID from your deployment
const PROGRAM_ID = new PublicKey(
  "3ypCkXQWiAFkNk7bo8bnZFxUVmVEWCqpBoY7v4vgPnHJ"
);

// Fixed IDL with correct string type definitions
const IDL = {
  version: "0.1.0",
  name: "erebrus",
  instructions: [
    {
      name: "registerVpnNode",
      accounts: [
        {
          name: "vpnNode",
          isMut: true,
          isSigner: false,
        },
        {
          name: "user",
          isMut: true,
          isSigner: true,
        },
        {
          name: "systemProgram",
          isMut: false,
          isSigner: false,
        },
      ],
      args: [
        {
          name: "userNodeNum",
          type: "u64",
        },
        {
          name: "peaqDid",
          type: "string",
        },
        {
          name: "nodename",
          type: "string",
        },
        {
          name: "ipaddress",
          type: "string",
        },
        {
          name: "ispinfo",
          type: "string",
        },
        {
          name: "region",
          type: "string",
        },
        {
          name: "location",
          type: "string",
        },
      ],
      discriminator: [254, 249, 109, 84, 232, 26, 70, 251],
    },
  ],
  accounts: [
    {
      name: "VpnNode",
      type: {
        kind: "struct",
        fields: [
          {
            name: "nodeId",
            type: "u64",
          },
          {
            name: "user",
            type: "publicKey",
          },
          {
            name: "peaqDid",
            type: "string",
          },
          {
            name: "nodename",
            type: "string",
          },
          {
            name: "ipaddress",
            type: "string",
          },
          {
            name: "ispinfo",
            type: "string",
          },
          {
            name: "region",
            type: "string",
          },
          {
            name: "location",
            type: "string",
          },
          {
            name: "status",
            type: "u8",
          },
          {
            name: "canClose",
            type: "bool",
          },
        ],
      },
      discriminator: [154, 255, 245, 194, 44, 120, 114, 244],
    },
  ],
};

async function checkBalance(connection, publicKey) {
  try {
    const balance = await connection.getBalance(publicKey);
    return balance / LAMPORTS_PER_SOL;
  } catch (error) {
    console.error("Error checking balance:", error);
    return 0;
  }
}

async function registerVpnNode() {
  try {
    // Connect to devnet
    const connection = new Connection(
      "https://rpc.devnet.soo.network/rpc",
      "confirmed"
    );

    // Load private key from environment variable (base58 string)
    const privateKeyEnv = process.env.SOLANA_PRIVATE_KEY;
    if (!privateKeyEnv) {
      console.error("Error: SOLANA_PRIVATE_KEY environment variable is not set.");
      process.exit(1);
    }

    // Decode the private key from base58 format to Uint8Array
    const privateKey = bs58.decode(privateKeyEnv);
    const wallet = Keypair.fromSecretKey(privateKey);

    console.log("Using wallet address:", wallet.publicKey.toString());

    // Check balance
    const balance = await checkBalance(connection, wallet.publicKey);
    console.log("Current balance:", balance, "SOL");

    // Setup provider
    const provider = new AnchorProvider(
      connection,
      {
        publicKey: wallet.publicKey,
        signTransaction: async (tx) => {
          tx.partialSign(wallet);
          return tx;
        },
        signAllTransactions: async (txs) => {
          return txs.map((t) => {
            t.partialSign(wallet);
            return t;
          });
        },
      },
      { commitment: "confirmed" }
    );

    // Create program interface
    const program = new Program(IDL, PROGRAM_ID, provider);

    // Generate a node number
    // const userNodeNum = new BN(1);
    let userNodeNum = new BN(Date.now()); // Timestamp in milliseconds

    // Print the userNodeNum
    console.log("Generated userNodeNum (on-chain counter):", userNodeNum.toString());


    // Find PDA
    const [vpnNodePDA] = PublicKey.findProgramAddressSync(
      [Buffer.from("vpn"), userNodeNum.toArrayLike(Buffer, "le", 8)],
      program.programId
    );

    console.log("VPN Node PDA:", vpnNodePDA.toString());

    // Register VPN node
    const nodeDetails = {
      userNodeNum: userNodeNum,
      peaqDid: "did:peaq:testnode123",
      nodename: "TestVPNNode",
      ipaddress: "192.168.1.100",
      ispinfo: "TestISP",
      region: "EU-Central",
      location: "Frankfurt, Germany",
    };

    const tx = await program.methods
      .registerVpnNode(
        nodeDetails.userNodeNum,
        nodeDetails.peaqDid,
        nodeDetails.nodename,
        nodeDetails.ipaddress,
        nodeDetails.ispinfo,
        nodeDetails.region,
        nodeDetails.location
      )
      .accounts({
        vpnNode: vpnNodePDA,
        user: wallet.publicKey,
        systemProgram: SystemProgram.programId,
      })
      .signers([wallet])
      .rpc();

    console.log("Transaction signature:", tx);

    // Fetch and display the created account
    await connection.confirmTransaction(tx);
    const vpnNodeAccount = await program.account.vpnNode.fetch(vpnNodePDA);

    console.log("Created VPN Node Account:", {
      nodeId: vpnNodeAccount.nodeId.toString(),
      user: vpnNodeAccount.user.toString(),
      peaqDid: vpnNodeAccount.peaqDid,
      nodename: vpnNodeAccount.nodename,
      ipaddress: vpnNodeAccount.ipaddress,
      ispinfo: vpnNodeAccount.ispinfo,
      region: vpnNodeAccount.region,
      location: vpnNodeAccount.location,
      status: vpnNodeAccount.status,
      canClose: vpnNodeAccount.canClose,
    });

    return {
      success: true,
      signature: tx,
      vpnNodePDA: vpnNodePDA.toString(),
      account: vpnNodeAccount,
    };
  } catch (error) {
    console.error("Error registering VPN node:", error);
    if (error.logs) {
      console.error("Program logs:", error.logs);
    }
    return {
      success: false,
      error: error.message,
      logs: error.logs,
    };
  }
}

// Execute the registration
console.log("Starting VPN node registration...");
registerVpnNode()
  .then((result) => {
    console.log("Registration result:", result);
    process.exit(result.success ? 0 : 1);
  })
  .catch((error) => {
    console.error("Fatal error:", error);
    process.exit(1);
  });