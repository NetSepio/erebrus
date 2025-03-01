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
	"os/exec"
	"runtime"
	"strings"
	"bytes"
	"mime/multipart"
	"strconv"
	"time"
	
	"github.com/NetSepio/erebrus/contract"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2phost "github.com/libp2p/go-libp2p/core/host"
	// "github.com/libp2p/go-libp2p/core/peer"
	bip39 "github.com/tyler-smith/go-bip39"
	bip32 "github.com/tyler-smith/go-bip32"
	log "github.com/sirupsen/logrus"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/disk"
	"crypto/ecdsa"
	"context"
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
	TotalRAM       uint64   `json:"total_ram"`
	UsedRAM        uint64   `json:"used_ram"`
	FreeRAM        uint64   `json:"free_ram"`
	TotalDisk      uint64   `json:"total_disk"`
	UsedDisk       uint64   `json:"used_disk"`
	FreeDisk       uint64   `json:"free_disk"`
	CPUUsage       float64  `json:"cpu_usage"`
	Version        string   `json:"version"`
	CodeHash       string   `json:"code_hash"`
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
func makeBasicHost() (libp2phost.Host, error) {
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

var libp2pHost libp2phost.Host

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

	fmt.Printf("%s✅ IPFS Upload Successful%s\n", colorGreen, colorReset)
	fmt.Printf("%s🔗 IPFS Hash:%s %s\n", colorPurple, colorReset, ipfsResp.Hash)
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

	// Get memory info
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return "", fmt.Errorf("failed to get memory stats: %v", err)
	}

	// Get disk usage
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return "", fmt.Errorf("failed to get disk stats: %v", err)
	}

	// Get version and code hash
	codeHash, version := GetCodeHashAndVersion()

	metadata := SystemMetadata{
		OS:              runtime.GOOS,
		Architecture:    runtime.GOARCH,
		NumCPU:         runtime.NumCPU(),
		Hostname:       hostname,
		LocalIPs:       localIPs,
		Environment:    environment,
		GoVersion:      runtime.Version(),
		RuntimeVersion: runtime.Version(),
		// Memory in bytes
		TotalRAM:       memInfo.Total,
		UsedRAM:        memInfo.Used,
		FreeRAM:        memInfo.Free,
		// Disk space in bytes
		TotalDisk:      diskInfo.Total,
		UsedDisk:       diskInfo.Used,
		FreeDisk:       diskInfo.Free,
		CPUUsage:       getCPUUsage(),
		// Version info
		Version:        version,
		CodeHash:       codeHash,
	}

	// Print the metadata in a formatted way
	fmt.Printf("\n%s%s%s\n", colorYellow, "═══════════ System Metadata ═══════════", colorReset)
	fmt.Printf("%s• OS/Arch:%s %s/%s\n", colorCyan, colorReset, metadata.OS, metadata.Architecture)
	fmt.Printf("%s• Hostname:%s %s\n", colorCyan, colorReset, metadata.Hostname)
	fmt.Printf("%s• Environment:%s %s\n", colorCyan, colorReset, metadata.Environment)
	fmt.Printf("%s• Local IPs:%s %s\n", colorCyan, colorReset, strings.Join(metadata.LocalIPs, ", "))
	fmt.Printf("%s• CPU:%s %d cores (Usage: %.2f%%)\n", colorCyan, colorReset, metadata.NumCPU, metadata.CPUUsage)
	fmt.Printf("%s• Memory:%s %.2f/%.2f GB (%.2f GB free)\n", colorCyan, colorReset, 
		float64(metadata.UsedRAM)/1024/1024/1024,
		float64(metadata.TotalRAM)/1024/1024/1024,
		float64(metadata.FreeRAM)/1024/1024/1024)
	fmt.Printf("%s• Disk:%s %.2f/%.2f GB (%.2f GB free)\n", colorCyan, colorReset,
		float64(metadata.UsedDisk)/1024/1024/1024,
		float64(metadata.TotalDisk)/1024/1024/1024,
		float64(metadata.FreeDisk)/1024/1024/1024)
	fmt.Printf("%s• Go Version:%s %s\n", colorCyan, colorReset, metadata.GoVersion)
	fmt.Printf("%s• Runtime Version:%s %s\n", colorCyan, colorReset, metadata.RuntimeVersion)
	fmt.Printf("%s• Version:%s %s\n", colorCyan, colorReset, metadata.Version)
	fmt.Printf("%s• Code Hash:%s %s\n", colorCyan, colorReset, metadata.CodeHash)
	fmt.Printf("%s%s%s\n", colorYellow, "══════════════════════════════════════", colorReset)

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

// Helper function to get CPU usage
func getCPUUsage() float64 {
	// Get CPU usage percentage
	percentage, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0.0
	}
	if len(percentage) > 0 {
		return percentage[0]
	}
	return 0.0
}

func generateNFTMetadata(nodeName string, nodeSpec string, nodeConfig string) (string, error) {
	configValue := os.Getenv("NODE_CONFIG")
	if configValue == "" {
		configValue = "STANDARD"
	}

	accessValue := os.Getenv("NODE_ACCESS")
	if accessValue == "" {
		accessValue = "public"
	}

	metadata := NFTMetadata{
		Name: fmt.Sprintf("%s | Erebrus Node", nodeName),
		Description: "This Soulbound NFT is more than just a token—it's a declaration of digital sovereignty. " +
			"As an Erebrus Node, it stands as an unyielding pillar of privacy and security, forging a path " +
			"beyond the reach of Big Tech's surveillance and censorship. This is not just technology; it's a " +
			"revolution. Welcome to the frontlines of digital freedom. Thank you for being a part of the movement.",
		Image:       "ipfs://bafybeig6unjraufdpiwnzrqudl5vy3ozep2pzc3hwiiqd4lgcjfhaockpm",
		ExternalURL: "https://erebrus.io",
		Attributes: []NFTAttribute{
			{TraitType: "name", Value: nodeName},
			{TraitType: "spec", Value: "erebrus"},
			{TraitType: "config", Value: configValue},
			{TraitType: "access", Value: accessValue},
			{TraitType: "status", Value: "registered"},
		},
	}

	nftMetadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("%s❌ Failed to marshal NFT metadata: %v%s", colorRed, err, colorReset)
	}

	// Log NFT metadata in a colorful box
	fmt.Printf("\n%s%s%s\n", colorYellow, "═══════════ NFT Metadata ═══════════", colorReset)
	fmt.Printf("%s• Name:%s %s\n", colorCyan, colorReset, metadata.Name)
	fmt.Printf("%s• Description:%s %s\n", colorCyan, colorReset, metadata.Description)
	fmt.Printf("%s• Image:%s %s\n", colorCyan, colorReset, metadata.Image)
	fmt.Printf("%s• External URL:%s %s\n", colorCyan, colorReset, metadata.ExternalURL)
	fmt.Printf("%s• Attributes:%s\n", colorCyan, colorReset)
	for _, attr := range metadata.Attributes {
		fmt.Printf("  %s◦ %s:%s %s\n", colorPurple, attr.TraitType, colorReset, attr.Value)
	}
	fmt.Printf("%s%s%s\n\n", colorYellow, "══════════════════════════════════", colorReset)

	return string(nftMetadataJSON), nil
}

func RegisterNodeOnPeaq() error {
	if strings.ToLower(os.Getenv("CHAIN_NAME")) != "peaq" {
		return nil
	}

	// Connect to the Ethereum client
	rpcURL := os.Getenv("RPC_URL")
	client, err := ethclient.Dial(rpcURL)
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

	// Get wallet details from mnemonic
	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		return fmt.Errorf("%s❌ MNEMONIC not found in environment variables%s", colorRed, colorReset)
	}

	privateKey, ownerAddress, err := deriveWalletFromMnemonic(mnemonic)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to derive wallet from mnemonic: %v%s", colorRed, err, colorReset)
	}
	fmt.Printf("\n%s%s%s\n", colorYellow, "═══════════ Wallet Details ═══════════", colorReset)
	fmt.Printf("%s• Mnemonic:%s %s\n", colorCyan, colorReset, mnemonic)
	fmt.Printf("%s• Private Key:%s %x\n", colorCyan, colorReset, ethcrypto.FromECDSA(privateKey))
	fmt.Printf("%s• Wallet Address:%s %s\n", colorCyan, colorReset, ownerAddress)
	fmt.Printf("%s%s%s\n\n", colorYellow, "══════════════════════════════════", colorReset)


	// Get chain ID from RPC URL
	chainID, err := getChainID(rpcURL)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to get chain ID: %v%s", colorRed, err, colorReset)
	}

	// Create auth with derived wallet
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to create transactor: %v%s", colorRed, err, colorReset)
	}

	// Get node address from wallet
	nodeAddress := auth.From

	// Prepare registration parameters
	nodeName := os.Getenv("NODE_NAME")
	nodeSpec := "erebrus"
	nodeConfig := os.Getenv("NODE_CONFIG")
	
	// Get system metadata and upload to IPFS
	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%sUploading System Metadata to IPFS:%s\n", colorCyan, colorReset)
	metadata, err := getSystemMetadata()
	if err != nil {
		return fmt.Errorf("%s❌ Failed to get system metadata: %v%s", colorRed, err, colorReset)
	}
	
	// Generate NFT metadata and upload to IPFS
	nftMetadataJSON, err := generateNFTMetadata(nodeName, nodeSpec, nodeConfig)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to generate NFT metadata: %v%s", colorRed, err, colorReset)
	}

	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%sUploading NFT Metadata to IPFS:%s\n", colorCyan, colorReset)
	// Upload NFT metadata to IPFS
	nftMetadata, err := uploadToIPFS(nftMetadataJSON)
	if err != nil {
		return fmt.Errorf("%s❌ Failed to upload NFT metadata to IPFS: %v%s", colorRed, err, colorReset)
	}

	// Use the derived owner address
	owner := ownerAddress

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
		if strings.Contains(err.Error(), "Node already exists") {
			fmt.Printf("\n%s%s%s\n", colorYellow, "═══════════ Node Status ═══════════", colorReset)
			fmt.Printf("%s• Status:%s Already Registered\n", colorCyan, colorReset)
			fmt.Printf("%s• Node ID:%s %s\n", colorCyan, colorReset, nodeID)
			fmt.Printf("%s%s%s\n\n", colorYellow, "═════════════════════════════════", colorReset)
		} else {
			return fmt.Errorf("Failed to register node: %v", err)
		}
	} else {
		fmt.Printf("\n%s%s%s\n", colorYellow, "═══════════ Node Registration ═══════════", colorReset)
		fmt.Printf("%s• Status:%s Registration Successful\n", colorCyan, colorReset)
		fmt.Printf("%s• Node ID:%s %s\n", colorCyan, colorReset, nodeID)
		fmt.Printf("%s• Transaction:%s %s\n", colorCyan, colorReset, tx.Hash().Hex())
		fmt.Printf("%s%s%s\n\n", colorYellow, "══════════════════════════════════════", colorReset)
	}

	// Start periodic checkpoints regardless of whether the node was just registered or already existed
	log.WithFields(log.Fields{
		"nodeID": nodeID,
		"interval": "1 minute",
	}).Info("Starting periodic checkpoint creation")

	CreatePeriodicCheckpoints(nodeID, client, instance, auth)

	return nil
}

func CreatePeriodicCheckpoints(nodeID string, client *ethclient.Client, instance *contract.Contract, auth *bind.TransactOpts) {
	ticker := time.NewTicker(15 * time.Minute)
	
	// Log the start of checkpoint creation
	log.WithFields(log.Fields{
		"nodeID": nodeID,
		"interval": "15 minutes",
	}).Info("Periodic checkpoint creation initialized")

	go func() {
		// Create first checkpoint immediately
		createCheckpoint(nodeID, instance, auth)

		// Then create checkpoints periodically
		for range ticker.C {
			createCheckpoint(nodeID, instance, auth)
		}
	}()
}

// SystemMetrics represents the system metrics for checkpoints
type SystemMetrics struct {
	Timestamp        int64         `json:"timestamp"`
	ConnectedClients int           `json:"connected_clients"`
	ClientStats      []ClientStats `json:"client_stats"`
	Uptime          string        `json:"uptime"`
}

// ClientStats represents the bandwidth statistics of a WireGuard client
type ClientStats struct {
	Client string `json:"client"`
	RX     string `json:"rx"`
	TX     string `json:"tx"`
}

var (
	programStartTime = time.Now() // Store program start time
)

// getSystemMetrics collects system metrics including WireGuard stats
func getSystemMetrics() (*SystemMetrics, error) {
	// Get WireGuard client stats
	clients, err := getBandwidthStats()
	if err != nil {
		log.WithError(err).Warn("Failed to get bandwidth stats")
	}

	// Calculate program uptime
	uptime := time.Since(programStartTime)
	uptimeStr := formatDuration(uptime)

	metrics := &SystemMetrics{
		Timestamp:        time.Now().Unix(),
		ConnectedClients: len(clients),
		ClientStats:      clients,
		Uptime:          uptimeStr,
	}

	return metrics, nil
}

// formatDuration converts duration to a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 || len(parts) == 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}

	return fmt.Sprintf("up %s", strings.Join(parts, ", "))
}

// getBandwidthStats fetches the bandwidth stats of WireGuard clients
func getBandwidthStats() ([]ClientStats, error) {
	var clients []ClientStats

	// Get the latest handshakes
	cmd := exec.Command("bash", "-c", "wg show wg0 latest-handshakes")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	nowCmd := exec.Command("date", "+%s")
	nowOut, err := nowCmd.Output()
	if err != nil {
		return nil, err
	}

	now, err := strconv.Atoi(strings.TrimSpace(string(nowOut)))
	if err != nil {
		return nil, err
	}

	var activeClients []string
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		handshakeTime, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		if handshakeTime > 0 && (now-handshakeTime) < 120 {
			activeClients = append(activeClients, fields[0])
		}
	}

	// Get the transfer stats
	cmd = exec.Command("wg", "show", "wg0", "transfer")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	transferStats := out.String()
	for _, client := range activeClients {
		for _, line := range strings.Split(transferStats, "\n") {
			if strings.Contains(line, client) {
				fields := strings.Fields(line)
				if len(fields) < 3 {
					continue
				}

				rxBytes, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					continue
				}
				txBytes, err := strconv.ParseFloat(fields[2], 64)
				if err != nil {
					continue
				}

				rxMB := rxBytes / 1024 / 1024
				txMB := txBytes / 1024 / 1024

				clients = append(clients, ClientStats{
					Client: client,
					RX:     strconv.FormatFloat(rxMB, 'f', 4, 64) + " MB",
					TX:     strconv.FormatFloat(txMB, 'f', 4, 64) + " MB",
				})
			}
		}
	}

	return clients, nil
}

// For wallet derivation logging
func deriveWalletFromMnemonic(mnemonic string) (*ecdsa.PrivateKey, common.Address, error) {
	walletAddress, privateKey, err := GenerateEthereumWalletAddress(mnemonic)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to generate wallet: %v", err)
	}
	
	address := common.HexToAddress(walletAddress)
	return privateKey, address, nil
}

func getChainID(rpcURL string) (*big.Int, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %v", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	return chainID, nil
}

func createCheckpoint(nodeID string, instance *contract.Contract, auth *bind.TransactOpts) {
	startTime := time.Now()

	// Get system metrics
	metrics, err := getSystemMetrics()
	if err != nil {
		log.WithFields(log.Fields{
			"nodeID": nodeID,
			"error":  err,
		}).Error("Failed to get system metrics")
		return
	}

	// Convert metrics to JSON
	dataJSON, err := json.Marshal(metrics)
	if err != nil {
		log.WithFields(log.Fields{
			"nodeID": nodeID,
			"error":  err,
		}).Error("Failed to marshal checkpoint data")
		return
	}

	// Get wallet details from mnemonic
	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		log.Error("MNEMONIC not found in environment variables")
		return
	}

	privateKey, _, err := deriveWalletFromMnemonic(mnemonic)
	if err != nil {
		log.WithError(err).Error("Failed to derive wallet from mnemonic")
		return
	}

	// Get chain ID from RPC URL
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Error("RPC_URL not found in environment variables")
		return
	}

	chainID, err := getChainID(rpcURL)
	if err != nil {
		log.WithError(err).Error("Failed to get chain ID")
		return
	}

	// Create new auth with the derived private key and chain ID
	newAuth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.WithError(err).Error("Failed to create transaction auth")
		return
	}

	// Copy over any existing auth settings
	if auth != nil {
		newAuth.GasLimit = auth.GasLimit
		newAuth.GasPrice = auth.GasPrice
		newAuth.Nonce = auth.Nonce
	}

	// Create checkpoint transaction
	tx, err := instance.CreateCheckpoint(newAuth, nodeID, string(dataJSON))
	if err != nil {
		log.WithFields(log.Fields{
			"nodeID": nodeID,
			"error":  err,
		}).Error("Failed to create checkpoint")
		return
	}

	duration := time.Since(startTime)

	fmt.Printf("\n%s%s%s\n", colorYellow, "═══════════ Checkpoint Created ═══════════", colorReset)
	fmt.Printf("%s• Node ID:%s %s\n", colorCyan, colorReset, nodeID)
	fmt.Printf("%s• Time:%s %s\n", colorCyan, colorReset, startTime.Format(time.RFC3339))
	fmt.Printf("%s• Duration:%s %s\n", colorCyan, colorReset, duration)
	fmt.Printf("%s• Transaction:%s %s\n", colorCyan, colorReset, tx.Hash().Hex())
	fmt.Printf("%s%s%s\n\n", colorYellow, "══════════════════════════════════════", colorReset)
}

// GetNodeStatus retrieves the current status of the node from the contract
func GetNodeStatus() (*NodeStatus, error) {
	if strings.ToLower(os.Getenv("CHAIN_NAME")) != "peaq" {
		return nil, fmt.Errorf("Chain not configured")
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(os.Getenv("RPC_URL"))
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the Ethereum client: %v", err)
	}

	// Create a new instance of the contract
	contractAddress := common.HexToAddress(os.Getenv("CONTRACT_ADDRESS"))
	instance, err := contract.NewContract(contractAddress, client)
	if err != nil {
		return nil, fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	// Get the node ID
	nodeID, err := GeneratePeaqDID()
	if err != nil {
		return nil, fmt.Errorf("Failed to generate Peaq DID: %v", err)
	}

	// Get node data from the contract
	node, err := instance.Nodes(&bind.CallOpts{}, nodeID)
	if err != nil {
		return nil, fmt.Errorf("Failed to get node data: %v", err)
	}

	// Get latest checkpoint
	checkpoint, err := instance.Checkpoint(&bind.CallOpts{}, nodeID)
	if err != nil {
		log.WithError(err).Warn("Failed to get checkpoint data")
	}

	return &NodeStatus{
		ID:        nodeID,
		Name:      node.Name,
		Spec:      node.Spec,
		Config:    node.Config,
		IPAddress: node.IpAddress,
		Region:    node.Region,
		Location:  node.Location,
		Owner:     node.Owner,
		TokenID:   node.TokenId,
		Status:    node.Status,
		Checkpoint: checkpoint,
	}, nil
}

// NodeStatus represents the current status of a node
type NodeStatus struct {
	ID         string
	Name       string
	Spec       string
	Config     string
	IPAddress  string
	Region     string
	Location   string
	Owner      common.Address
	TokenID    *big.Int
	Status     uint8
	Checkpoint string
}

// GetStatusText returns the text representation of the node status
func (ns *NodeStatus) GetStatusText() string {
	statusText := []string{"Offline", "Online", "Maintenance", "Deactivated"}
	if ns.Status < uint8(len(statusText)) {
		return statusText[ns.Status]
	}
	return "Unknown"
}

// GetStatusEmoji returns the emoji representation of the node status
func (ns *NodeStatus) GetStatusEmoji() string {
	statusEmoji := []string{"🔴", "🟢", "🟡", "⚫"}
	if ns.Status < uint8(len(statusEmoji)) {
		return statusEmoji[ns.Status]
	}
	return "❓"
}

// DeactivateNode deactivates the node in the contract
func DeactivateNode() error {
	if strings.ToLower(os.Getenv("CHAIN_NAME")) != "peaq" {
		return fmt.Errorf("Chain not configured")
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(os.Getenv("RPC_URL"))
	if err != nil {
		return fmt.Errorf("Failed to connect to the Ethereum client: %v", err)
	}

	// Create a new instance of the contract
	contractAddress := common.HexToAddress(os.Getenv("CONTRACT_ADDRESS"))
	instance, err := contract.NewContract(contractAddress, client)
	if err != nil {
		return fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	// Get the node ID
	nodeID, err := GeneratePeaqDID()
	if err != nil {
		return fmt.Errorf("Failed to generate Peaq DID: %v", err)
	}

	// Create auth options for the transaction
	privateKey, err := ethcrypto.HexToECDSA(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		return fmt.Errorf("Failed to create private key: %v", err)
	}

	chainID, ok := new(big.Int).SetString(os.Getenv("CHAIN_ID"), 10)
	if !ok {
		return fmt.Errorf("Failed to parse CHAIN_ID")
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("Failed to create transactor: %v", err)
	}

	// Call deactivateNode function
	tx, err := instance.DeactivateNode(auth, nodeID)
	if err != nil {
		return fmt.Errorf("Failed to deactivate node: %v", err)
	}

	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🔄 Node Deactivation%s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🆔 Node ID:%s %s\n", colorCyan, colorReset, nodeID)
	fmt.Printf("%s📝 Transaction Hash:%s %s\n", colorCyan, colorReset, tx.Hash().Hex())
	fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)

	return nil
}

// ActivateNode sets the node status to Online
func ActivateNode() error {
	if strings.ToLower(os.Getenv("CHAIN_NAME")) != "peaq" {
		return fmt.Errorf("Chain not configured")
	}

	// Connect to the Ethereum client
	client, err := ethclient.Dial(os.Getenv("RPC_URL"))
	if err != nil {
		return fmt.Errorf("Failed to connect to the Ethereum client: %v", err)
	}

	// Create a new instance of the contract
	contractAddress := common.HexToAddress(os.Getenv("CONTRACT_ADDRESS"))
	instance, err := contract.NewContract(contractAddress, client)
	if err != nil {
		return fmt.Errorf("Failed to instantiate contract: %v", err)
	}

	// Get the node ID
	nodeID, err := GeneratePeaqDID()
	if err != nil {
		return fmt.Errorf("Failed to generate Peaq DID: %v", err)
	}

	// Create auth options for the transaction
	privateKey, err := ethcrypto.HexToECDSA(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		return fmt.Errorf("Failed to create private key: %v", err)
	}

	chainID, ok := new(big.Int).SetString(os.Getenv("CHAIN_ID"), 10)
	if !ok {
		return fmt.Errorf("Failed to parse CHAIN_ID")
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("Failed to create transactor: %v", err)
	}

	// Call updateNodeStatus function with Online status (1)
	tx, err := instance.UpdateNodeStatus(auth, nodeID, 1) // 1 represents Online status
	if err != nil {
		return fmt.Errorf("Failed to activate node: %v", err)
	}

	fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🔄 Node Activation%s\n", colorGreen, colorReset)
	fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
	fmt.Printf("%s🆔 Node ID:%s %s\n", colorCyan, colorReset, nodeID)
	fmt.Printf("%s📝 Transaction Hash:%s %s\n", colorCyan, colorReset, tx.Hash().Hex())
	fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)

	return nil
}

