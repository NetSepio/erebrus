package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"bytes"
	"mime/multipart"

	"github.com/NetSepio/erebrus/contract"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	// "github.com/libp2p/go-libp2p/core/peer"
	bip39 "github.com/tyler-smith/go-bip39"
	bip32 "github.com/tyler-smith/go-bip32"
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
	Loc     string `json:"loc"`
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

type IPFSResponse struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

// Custom reader for deterministic key generation
type reader struct {
	seed []byte
	pos  int
}

func (r *reader) Read(p []byte) (n int, err error) {
	copy(p, r.seed)
	return len(r.seed), nil
}

func bytesReader(seed []byte) *reader {
	return &reader{seed: seed}
}

// makeBasicHost creates a LibP2P host with a deterministic peer ID using mnemonics
func makeBasicHost() (host.Host, error) {
	// Get mnemonic from environment variable or use default
	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		log.Warn("MNEMONIC not set, using default mnemonic")
		mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	}

	// Convert mnemonic to a BIP-32 seed
	seed := bip39.NewSeed(mnemonic, "")

	// Derive a master key from the seed
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %v", err)
	}

	// Derive a child key
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild)
	if err != nil {
		return nil, fmt.Errorf("failed to derive child key: %v", err)
	}

	// Convert the private key to an Ed25519 key
	hashedKey := sha256.Sum256(childKey.Key)
	priv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Ed25519, 256, bytesReader(hashedKey[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to generate libp2p key: %v", err)
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9002"),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	host, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %v", err)
	}

	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🌟 LibP2P Host Created%s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🆔 Peer ID:%s %s\n", colorCyan, colorReset, host.ID().String())
	fmt.Printf("%s📡 Addresses:%s\n", colorCyan, colorReset)
	for _, addr := range host.Addrs() {
		fmt.Printf("   %s%s%s\n", colorBlue, addr.String(), colorReset)
	}
	fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)

	return host, nil
}

var libp2pHost host.Host

func GeneratePeaqDID() (string, error) {
	var err error
	if libp2pHost == nil {
		libp2pHost, err = makeBasicHost()
		if err != nil {
			return "", fmt.Errorf("%s❌ Failed to create LibP2P host: %v%s", colorRed, err, colorReset)
		}
	}

	peerID := libp2pHost.ID().String()
	return fmt.Sprintf("did:netsepio:%s", peerID), nil
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

// isCloudEnvironment detects if the system is running in a cloud environment.
func isCloudEnvironment() string {
	// Check for hypervisor UUID, used by major cloud providers
	if content, err := os.ReadFile("/sys/hypervisor/uuid"); err == nil {
		uuid := strings.ToLower(strings.TrimSpace(string(content)))
		if strings.HasPrefix(uuid, "ec2") || strings.HasPrefix(uuid, "google") {
			return "cloud"
		}
	}

	// Check for cloud-init presence
	if _, err := os.Stat("/var/lib/cloud"); err == nil {
		return "cloud"
	}

	return "consumer"
}

func uploadToIPFS(data string) (string, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", "data.json")
	if err != nil {
		return "", fmt.Errorf("error creating form file: %v", err)
	}

	_, err = io.Copy(fw, bytes.NewReader([]byte(data)))
	if err != nil {
		return "", fmt.Errorf("error copying data: %v", err)
	}

	w.Close()

	req, err := http.NewRequest("POST", "https://ipfs.erebrus.io/api/v0/add", &b)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var ipfsResp IPFSResponse
	if err := json.Unmarshal(body, &ipfsResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s📤 IPFS Upload Attempt%s\n", colorBlue, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s📦 Data Size:%s %d bytes\n", colorCyan, colorReset, len(data))

	// After successful upload
	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s✅ IPFS Upload Successful%s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s📝 File Name:%s %s\n", colorCyan, colorReset, ipfsResp.Name)
	fmt.Printf("%s📏 File Size:%s %s bytes\n", colorCyan, colorReset, ipfsResp.Size)
	fmt.Printf("%s🔗 IPFS Hash:%s %s\n", colorCyan, colorReset, ipfsResp.Hash)
	fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)

	return fmt.Sprintf("ipfs://%s", ipfsResp.Hash), nil
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

	environment := isCloudEnvironment()
	fmt.Println("environment", environment)

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

	// Upload to IPFS
	ipfsPath, err := uploadToIPFS(string(metadataJSON))
	if err != nil {
		return "", fmt.Errorf("failed to upload metadata to IPFS: %v", err)
	}

	return ipfsPath, nil
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

	nodeID, err := GeneratePeaqDID()
	if err != nil {
		return fmt.Errorf("%s❌ Failed to generate Peaq DID: %v%s", colorRed, err, colorReset)
	}

	// Create auth options for the transaction
	privateKey, err := ethcrypto.HexToECDSA(os.Getenv("PRIVATE_KEY"))
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
	
	// Get system metadata and upload to IPFS
	metadata, err := getSystemMetadata()
	if err != nil {
		return fmt.Errorf("%s❌ Failed to get system metadata: %v%s", colorRed, err, colorReset)
	}
	
	// Generate NFT metadata and upload to IPFS
	nftMetadataJSON, err := generateNFTMetadata(nodeName, nodeSpec, nodeConfig)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to generate NFT metadata: %v%s", colorRed, err, colorReset)
	}

	nftMetadata, err := uploadToIPFS(nftMetadataJSON)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to upload NFT metadata to IPFS: %v%s", colorRed, err, colorReset)
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
		ipInfo.Loc,     // location (coordinates)
		metadata,       // metadata
		nftMetadata,    // nftMetadata
		owner,          // _owner
	)
	if err != nil {
		errorMsg := fmt.Sprintf("\n%s%s%s\n", colorRed, "====================================", colorReset)
		errorMsg += fmt.Sprintf("%s❌ Registration Error%s\n", colorRed, colorReset)
		errorMsg += fmt.Sprintf("%s%s%s\n", colorRed, "====================================", colorReset)
		errorMsg += fmt.Sprintf("%s🚫 Error:%s %v\n", colorRed, colorReset, err)
		errorMsg += fmt.Sprintf("%s%s%s\n\n", colorRed, "====================================", colorReset)
		
		fmt.Print(errorMsg)
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
	fmt.Printf("%s📍 Coordinates:%s %s%s%s\n", colorCyan, colorReset, colorCyan, ipInfo.Loc, colorReset)
	fmt.Printf("%s💻 Environment:%s %s%s%s\n", colorCyan, colorReset, colorPurple, isCloudEnvironment(), colorReset)
	fmt.Printf("%s🔧 System Metadata IPFS:%s %s%s%s\n", colorCyan, colorReset, colorBlue, metadata, colorReset)
	fmt.Printf("%s🎨 NFT Metadata IPFS:%s %s%s%s\n", colorCyan, colorReset, colorPurple, nftMetadata, colorReset)
	fmt.Printf("%s👤 Owner Address:%s %s%s%s\n", colorCyan, colorReset, colorYellow, owner.Hex(), colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s✅ Registration Complete! %s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)

	// Structured logging with all details
	log.WithFields(log.Fields{
		"txHash":           tx.Hash().Hex(),
		"nodeID":          nodeID,
		"nodeName":        nodeName,
		"nodeSpec":        nodeSpec,
		"nodeConfig":      nodeConfig,
		"ipAddress":       ipInfo.IP,
		"region":          ipInfo.Region,
		"coordinates":     ipInfo.Loc,
		"environment":     isCloudEnvironment(),
		"systemMetadata": metadata,
		"nftMetadata":    nftMetadata,
		"ownerAddress":   owner.Hex(),
		"contractAddress": contractAddress.Hex(),
		"chainID":        chainID.String(),
	}).Info(fmt.Sprintf("%s🚀 Node registration transaction submitted successfully! 🎉%s", colorGreen, colorReset))

	log.WithFields(log.Fields{
		"systemMetadataIPFS": metadata,
		"nftMetadataIPFS":   nftMetadata,
	}).Info(fmt.Sprintf("%s📤 Metadata uploaded to IPFS successfully%s", colorGreen, colorReset))

	return nil
}

