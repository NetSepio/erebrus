package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/google/uuid"
)

func SolanaRegister() {
	// Connect to Solana RPC node
	rpcURL := os.Getenv("SOLANA_RPC_URL")
	if rpcURL == "" {
		rpcURL = rpc.MainNetBeta_RPC
	}
	client := rpc.New(rpcURL)

	// Define the public key of your smart contract
	programID := solana.MustPublicKeyFromBase58(os.Getenv("SMART_CONTRACT_PUBLIC_KEY"))

	// Define the sender's wallet keypair
	sender := solana.NewWalletFromPrivateKeyBase58(os.Getenv("SENDER_PRIVATE_KEY"))

	// Generate attributes
	deviceID := generateDeviceID()
	did := generateDID()
	nodeName := os.Getenv("NODE_NAME")
	ipAddress, ispInfo, region, location := getIPInfo()

	// Create an instruction to call the `register_vpn_node` function
	params := []interface{}{
		deviceID,
		did,
		nodeName,
		ipAddress,
		ispInfo,
		region,
		location,
	}
	instruction := system.NewInvokeInstruction(programID, params)

	// Create a transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			instruction,
		},
		sender.PublicKey(),
	)
	if err != nil {
		fmt.Println("Failed to create transaction:", err)
		return
	}

	// Sign the transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if sender.PublicKey().Equals(key) {
				return &sender.PrivateKey
			}
			return nil
		},
	)
	if err != nil {
		fmt.Println("Failed to sign transaction:", err)
		return
	}

	// Send the transaction
	txSig, err := client.SendTransaction(context.Background(), tx)
	if err != nil {
		fmt.Println("Failed to send transaction:", err)
		return
	}

	fmt.Println("Transaction sent with signature:", txSig)
}

func generateDeviceID() string {
	// Generate a unique device ID (e.g., using UUID)
	return uuid.New().String()
}

func generateDID() string {
	// Generate a Decentralized Identifier (DID)
	// This is a simplified example; you might want to use a proper DID method
	return fmt.Sprintf("did:erebrus:%s", uuid.New().String())
}

func getIPInfo() (string, string, string, string) {
	// Use ipinfo.io to get IP information
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ipinfo.io")
	if err != nil {
		log.Printf("Failed to get IP info: %v", err)
		return "", "", "", ""
	}
	defer resp.Body.Close()

	var info struct {
		IP       string `json:"ip"`
		Hostname string `json:"hostname"`
		City     string `json:"city"`
		Region   string `json:"region"`
		Country  string `json:"country"`
		Loc      string `json:"loc"`
		Org      string `json:"org"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.Printf("Failed to decode IP info: %v", err)
		return "", "", "", ""
	}

	ipAddress := info.IP
	ispInfo := info.Org
	region := fmt.Sprintf("%s, %s", info.City, info.Country)
	location := info.Loc

	return ipAddress, ispInfo, region, location
}
