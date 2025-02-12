package core

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/NetSepio/erebrus/contract"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorReset  = "\033[0m"
)

type PeaqIPInfo struct {
	IP      string `json:"ip"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

type SystemMetadata struct {
	OS              string   `json:"os"`
	Architecture    string   `json:"architecture"`
	NumCPU         int      `json:"num_cpu"`
	Hostname       string   `json:"hostname"`
	LocalIPs       []string `json:"local_ips"`
	Environment    string   `json:"environment"` // "cloud" or "local"
	GoVersion      string   `json:"go_version"`
	RuntimeVersion string   `json:"runtime_version"`
}

type NFTAttribute struct {
	TraitType string `json:"trait_type"`
	Value     string `json:"value"`
}

type NFTMetadata struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Image       string         `json:"image"`
	ExternalURL string        `json:"externalUrl"`
	Attributes  []NFTAttribute `json:"attributes"`
}

func init() {
	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		FullTimestamp:    true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

func getLocalIPs() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips, nil
}

func getSystemMetadata() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	localIPs, err := getLocalIPs()
	if err != nil {
		localIPs = []string{"unknown"}
	}

	//would soon add code to detect if environment is cloud or local
	environment := "local"

	metadata := SystemMetadata{
		OS:              runtime.GOOS,
		Architecture:    runtime.GOARCH,
		NumCPU:         runtime.NumCPU(),
		Hostname:       hostname,
		LocalIPs:       localIPs,
		Environment:    environment,
		GoVersion:      runtime.Version(),
		RuntimeVersion: runtime.Version(),
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal system metadata: %v", err)
	}

	return string(metadataJSON), nil
}

func GeneratePeaqDID(length int) (string, error) {
	if length <= 0 {
		length = 51
	}
	const validChars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(validChars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %v", err)
		}
		result[i] = validChars[randomIndex.Int64()]
	}
	return fmt.Sprintf("did:netsepio:%s", string(result)), nil
}

func generateNFTMetadata(nodeName string, nodeSpec string, nodeConfig string) (string, error) {
	specValue := fmt.Sprintf("erebrus.%s", strings.ToLower(nodeConfig))

	configValue := os.Getenv("EREBRUS_NODE_CONFIG")
	if configValue == "" {
		configValue = "public" // default is public
	}

	metadata := NFTMetadata{
		Name: fmt.Sprintf("%s | Erebrus Node", nodeName),
		Description: "This Soulbound NFT is more than just a token—it's a declaration of digital sovereignty. " +
			"As an Erebrus Node, it stands as an unyielding pillar of privacy and security, forging a path " +
			"beyond the reach of Big Tech's surveillance and censorship. This is not just technology; it's a " +
			"revolution. Welcome to the frontlines of digital freedom. Thank you for being a part of the movement.",
		Image:       "https://ipfs.io/ipfs/bafybeig6unjraufdpiwnzrqudl5vy3ozep2pzc3hwiiqd4lgcjfhaockpm",
		ExternalURL: "https://erebrus.io",
		Attributes: []NFTAttribute{
			{
				TraitType: "name",
				Value:     nodeName,
			},
			{
				TraitType: "spec",
				Value:     specValue,
			},
			{
				TraitType: "config",
				Value:     configValue,
			},
			{
				TraitType: "status",
				Value:     "registered",
			},
		},
	}

	nftMetadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("%s❌ Failed to marshal NFT metadata: %v%s", colorRed, err, colorReset)
	}

	return string(nftMetadataJSON), nil
}

func RegisterNodeOnPeaq() error {
	if os.Getenv("CHAIN_NAME") != "peaq" {
		return nil
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(os.Getenv("RPC_URL"))
	if err != nil {
		return fmt.Errorf("%s❌ Failed to connect to the Ethereum client: %v%s", colorRed, err, colorReset)
	}

	// Create a new instance of the contract
	contractAddress := common.HexToAddress(os.Getenv("CONTRACT_ADDRESS"))
	instance, err := contract.NewContract(contractAddress, client)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to instantiate contract: %v%s", colorRed, err, colorReset)
	}

	nodeID, err := GeneratePeaqDID(51)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to generate Peaq DID: %v%s", colorRed, err, colorReset)
	}

	// Create auth options for the transaction
	privateKey, err := crypto.HexToECDSA(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		log.Fatalf("%s❌ Failed to create private key: %v%s", colorRed, err, colorReset)
	}

	chainID, ok := new(big.Int).SetString(os.Getenv("CHAIN_ID"), 10)
	if !ok {
		log.Fatalf("%s❌ Failed to parse CHAIN_ID%s", colorRed, colorReset)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("%s❌ Failed to create transactor: %v%s", colorRed, err, colorReset)
	}

	// Get node address from wallet
	nodeAddress := auth.From

	// Prepare registration parameters
	nodeName := os.Getenv("NODE_NAME")
	nodeSpec := os.Getenv("NODE_TYPE")
	nodeConfig := os.Getenv("NODE_CONFIG")
	
	// Get system metadata
	metadata, err := getSystemMetadata()
	if err != nil {
		return fmt.Errorf("%s❌ Failed to get system metadata: %v%s", colorRed, err, colorReset)
	}
	
	// Generate NFT metadata
	nftMetadata, err := generateNFTMetadata(nodeName, nodeSpec, nodeConfig)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to generate NFT metadata: %v%s", colorRed, err, colorReset)
	}

	owner := common.HexToAddress(os.Getenv("OWNER"))

	// Get IP info from ipinfo.io
	resp, err := http.Get("https://ipinfo.io/json")
	if err != nil {
		return fmt.Errorf("%s❌ Failed to get IP info: %v%s", colorRed, err, colorReset)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read IP info response: %v", err)
	}

	var ipInfo PeaqIPInfo
	if err := json.Unmarshal(body, &ipInfo); err != nil {
		return fmt.Errorf("failed to parse IP info: %v", err)
	}
	
	fmt.Println(ipInfo)
	// Register the node
	tx, err := instance.RegisterNode(
		auth,
		nodeAddress,    // _addr
		nodeID,         // id
		nodeName,       // name
		nodeSpec,       // spec
		nodeConfig,     // config
		ipInfo.IP,      // ipAddress
		ipInfo.Region,  // region
		ipInfo.City,    // location
		metadata,       // metadata
		nftMetadata,    // nftMetadata
		owner,          // _owner
	)
	if err != nil {
		// Print colored error to stdout
		fmt.Printf("\n%s❌ Error: Failed to register node: %v%s\n\n", colorRed, err, colorReset)
		// Log error in JSON format
		return fmt.Errorf("Failed to register node: %v", err)
	}

	// Print colored success message to stdout
	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🌟 Node Registration Details 🌟%s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s📝 Transaction Hash:%s %s%s%s\n", colorCyan, colorReset, colorGreen, tx.Hash().Hex(), colorReset)
	fmt.Printf("%s🆔 Node ID:%s %s%s%s\n", colorCyan, colorReset, colorBlue, nodeID, colorReset)
	fmt.Printf("%s📛 Node Name:%s %s%s%s\n", colorCyan, colorReset, colorPurple, nodeName, colorReset)
	fmt.Printf("%s🌐 IP Address:%s %s%s%s\n", colorCyan, colorReset, colorYellow, ipInfo.IP, colorReset)
	fmt.Printf("%s🗺  Region:%s %s%s%s\n", colorCyan, colorReset, colorCyan, ipInfo.Region, colorReset)
	fmt.Printf("%s📍 Location:%s %s%s%s\n", colorCyan, colorReset, colorCyan, ipInfo.City, colorReset)
	fmt.Printf("%s🔧 System Metadata:%s %s%s%s\n", colorCyan, colorReset, colorBlue, metadata, colorReset)
	fmt.Printf("%s🎨 NFT Metadata:%s %s%s%s\n", colorCyan, colorReset, colorPurple, nftMetadata, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s✅ Registration Complete! %s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)

	// Log success in JSON format
	log.WithFields(log.Fields{
		"txHash":      tx.Hash().Hex(),
		"nodeID":      nodeID,
		"nodeName":    nodeName,
		"ipAddress":   ipInfo.IP,
		"region":      ipInfo.Region,
		"location":    ipInfo.City,
	}).Info("Node registration successful")

	return nil
}

