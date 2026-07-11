package nodeapp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/initcfg"
	"github.com/NetSepio/erebrus/internal/p2p"
)

func runInitCLI(args []string) error {
	var (
		access     = "public"
		profile    = ""
		publicAddr = os.Getenv("WG_ENDPOINT_HOST")
		gatewayURL = envOr("GATEWAY_URL", "https://gateway.erebrus.io")
		nodeName   = envOr("NODE_NAME", hostnameOr("erebrus-node"))
		region     = envOr("REGION", "unknown")
		zone       = envOr("ZONE", "")
		mnemonic   = os.Getenv("MNEMONIC")
		apiToken   = os.Getenv("NODE_API_TOKEN")
		envPath    = initcfg.DefaultEnvPath
		yes        = false
		appHosting = false
		appDomain  = ""
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--access", "--mode":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for %s", args[i-1])
			}
			access = args[i]
		case "--network-profile":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --network-profile")
			}
			profile = args[i]
		case "--public-address", "--wg-endpoint-host":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for public address")
			}
			publicAddr = args[i]
		case "--gateway-url":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --gateway-url")
			}
			gatewayURL = args[i]
		case "--node-name":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --node-name")
			}
			nodeName = args[i]
		case "--region":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --region")
			}
			region = args[i]
		case "--zone":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --zone")
			}
			zone = args[i]
		case "--env-file":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --env-file")
			}
			envPath = args[i]
		case "--enable-app-hosting":
			appHosting = true
		case "--domain":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --domain")
			}
			appDomain = args[i]
		case "-y", "--yes":
			yes = true
		case "-h", "--help":
			printInitHelp()
			return nil
		default:
			return fmt.Errorf("unknown argument: %s", args[i])
		}
	}

	mode, err := initcfg.ParseAccessMode(access)
	if err != nil {
		return err
	}
	if publicAddr == "" {
		return fmt.Errorf("public address is required (--public-address or WG_ENDPOINT_HOST)")
	}

	if mnemonic == "" {
		mnemonic, err = p2p.GenerateMnemonic()
		if err != nil {
			return err
		}
		fmt.Println("Generated a new node identity (12-word recovery phrase).")
		fmt.Println("Back it up now — it cannot be recovered.")
		if !yes {
			fmt.Print("Press Enter to continue...")
			fmt.Scanln()
		}
	}
	if apiToken == "" {
		apiToken = randToken()
	}

	var netProfile config.NetworkProfile
	if profile != "" {
		m, err := config.ParseModeSettings(string(mode), "", profile)
		if err != nil {
			return err
		}
		netProfile = m.NetworkProfile
	}

	opts := initcfg.Options{
		AccessMode:           mode,
		NetworkProfile:       netProfile,
		NodeName:             nodeName,
		Region:               region,
		Zone:                 zone,
		Mnemonic:             mnemonic,
		NodeAPIToken:         apiToken,
		GatewayURL:           gatewayURL,
		PublicAddress:        publicAddr,
		EnableStealth:        true,
		EnableAppHosting:     appHosting,
		AppWildcardDomain:    appDomain,
		PublicGatewayEnabled: appHosting,
	}
	if appHosting && appDomain != "" {
		opts.PublicDomain = appDomain
		opts.WildcardDomain = "*." + appDomain
	}

	if err := initcfg.WriteFile(envPath, opts); err != nil {
		return err
	}
	fmt.Printf("Wrote internal config: %s\n", envPath)
	fmt.Printf("Node API key: %s\n", apiToken)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Link for systemd: EnvironmentFile=%s\n", envPath)
	fmt.Println("  2. systemctl enable --now erebrus")
	fmt.Println("  3. erebrus status")
	return nil
}

func printInitHelp() {
	fmt.Println(`Usage: erebrus init [options]

Initialize a bare-metal node (writes internal env file — do not edit by hand).

Options:
  --access private|public          Gateway visibility (default: public)
  --network-profile bridge|host-network|native
  --public-address <ip-or-dns>     Public address clients dial (required)
  --gateway-url <url>              Control plane URL
  --node-name <name>
  --region <code>                  ISO country or custom label (e.g. US)
  --zone <zone>                    Optional placement (e.g. east, west, us-east)
  --enable-app-hosting             Public edge (public mode)
  --domain <base>                  e.g. apps.example.com
  --env-file <path>                default: /etc/erebrus/erebrus.env
  -y, --yes                        Non-interactive

After start, verify with: erebrus status`)
}

func randToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	s := base64.RawURLEncoding.EncodeToString(b)
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

func hostnameOr(def string) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return def
}

func runDoctorCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: erebrus doctor network|gateway|config")
	}
	switch args[0] {
	case "config":
		return doctorConfig()
	case "network":
		return doctorNetwork()
	case "gateway":
		return doctorGateway()
	default:
		return fmt.Errorf("unknown doctor target: %s", args[0])
	}
}

func doctorConfig() error {
	path := initcfg.DefaultEnvPath
	if p := os.Getenv("EREBRUS_ENV_FILE"); p != "" {
		path = p
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("internal config not found at %s", path)
	}
	if info.Mode().Perm() > 0o600 {
		fmt.Printf("! config permissions %o — recommend 600\n", info.Mode().Perm())
	}
	_ = os.Setenv("LOAD_CONFIG_FILE", "")
	// Load via godotenv from file would need reading - use preboot with env from file
	fmt.Printf("ok config file exists: %s (%d bytes)\n", path, info.Size())
	fmt.Println("  Run: erebrus status --preboot (with EnvironmentFile loaded) or erebrus status after start")
	return nil
}

func doctorNetwork() error {
	return runStatusCLI([]string{"--json"})
}

func doctorGateway() error {
	url := envOr("GATEWAY_URL", "")
	if url == "" {
		return fmt.Errorf("GATEWAY_URL not set")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url + "/healthz")
	if err != nil {
		return fmt.Errorf("gateway unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway healthz returned %d", resp.StatusCode)
	}
	fmt.Printf("ok gateway reachable: %s/healthz\n", url)
	return runStatusCLI(nil)
}
