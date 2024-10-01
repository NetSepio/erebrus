package core

// import (
// 	"context"
// 	"fmt"

// 	"github.com/gagliardetto/solana-go"
// 	"github.com/gagliardetto/solana-go/programs/system"
// 	"github.com/gagliardetto/solana-go/rpc"
// )

// func SolanaRegister() {
// 	// Connect to Solana RPC node
// 	client := rpc.New(rpc.MainNetBeta_RPC)

// 	// Define the public key of your smart contract
// 	programID := solana.MustPublicKeyFromBase58("YourSmartContractPublicKeyHere")

// 	// Define the sender's wallet keypair
// 	sender := solana.NewWalletFromPrivateKeyBase58("YourPrivateKeyHere")

// 	// Create an instruction to call the `register_vpn_node` function
// 	params := []interface{}{
// 		"device_id_example",
// 		"did_example",
// 		"node_name_example",
// 		"ip_address_example",
// 		"isp_info_example",
// 		"region_example",
// 		"location_example",
// 	}
// 	instruction := system.NewInvokeInstruction(programID, params)

// 	// Create a transaction
// 	tx, err := solana.NewTransaction(
// 		[]solana.Instruction{
// 			instruction,
// 		},
// 		sender.PublicKey(),
// 	)
// 	if err != nil {
// 		fmt.Println("Failed to create transaction:", err)
// 		return
// 	}

// 	// Sign the transaction
// 	_, err = tx.Sign(
// 		func(key solana.PublicKey) *solana.PrivateKey {
// 			if sender.PublicKey().Equals(key) {
// 				return &sender.PrivateKey
// 			}
// 			return nil
// 		},
// 	)
// 	if err != nil {
// 		fmt.Println("Failed to sign transaction:", err)
// 		return
// 	}

// 	// Send the transaction
// 	txSig, err := client.SendTransaction(context.Background(), tx)
// 	if err != nil {
// 		fmt.Println("Failed to send transaction:", err)
// 		return
// 	}

// 	fmt.Println("Transaction sent with signature:", txSig)
// }
