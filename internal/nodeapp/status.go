package nodeapp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/readiness"
)

func runStatusCLI(args []string) error {
	preboot := false
	jsonOut := false
	url := fmt.Sprintf("http://127.0.0.1:%s/api/v2/status", envOr("HTTP_PORT", "9080"))

	for _, a := range args {
		switch a {
		case "--preboot":
			preboot = true
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag: %s", a)
		}
	}

	if preboot {
		cfg := config.Load()
		if err := cfg.Validate(); err != nil {
			return err
		}
		rep := readiness.Preboot(cfg)
		return printStatus(rep, jsonOut)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("node not reachable at %s: %w (is erebrus running?)", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	if jsonOut {
		fmt.Println(string(body))
		var out struct {
			Readiness readiness.Report `json:"readiness"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return err
		}
		if !out.Readiness.OK {
			os.Exit(1)
		}
		return nil
	}

	var out struct {
		AccessMode string `json:"access_mode"`
		Identity   struct {
			PeerID        string `json:"peer_id"`
			DID           string `json:"did"`
			WalletChain   string `json:"wallet_chain"`
			WalletLabel   string `json:"wallet_chain_label"`
			WalletAddress string `json:"wallet_address"`
		} `json:"identity"`
		Endpoints struct {
			WireGuard struct {
				PublicKey string `json:"public_key"`
				Endpoint  string `json:"endpoint"`
			} `json:"wireguard"`
		} `json:"endpoints"`
		Readiness readiness.Report `json:"readiness"`
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}

	fmt.Printf("Access: %s\n", out.AccessMode)
	if hint, ok := out.Capabilities["access_hint"].(string); ok && hint != "" {
		fmt.Printf("  %s\n", hint)
	}
	if region, ok := out.Capabilities["region_label"].(string); ok && region != "" {
		fmt.Printf("Region: %s\n", region)
	}
	if zone, ok := out.Capabilities["zone_label"].(string); ok && zone != "" {
		fmt.Printf("Zone: %s\n", zone)
	}
	fmt.Printf("Peer ID: %s\n", out.Identity.PeerID)
	fmt.Printf("DID: %s\n", out.Identity.DID)
	if out.Identity.WalletAddress != "" {
		label := out.Identity.WalletLabel
		if label == "" {
			label = out.Identity.WalletChain
		}
		fmt.Printf("Wallet (%s): %s\n", label, out.Identity.WalletAddress)
	}
	if out.Endpoints.WireGuard.PublicKey != "" {
		fmt.Printf("WireGuard public key: %s\n", out.Endpoints.WireGuard.PublicKey)
		if out.Endpoints.WireGuard.Endpoint != "" {
			fmt.Printf("WireGuard endpoint: %s\n", out.Endpoints.WireGuard.Endpoint)
		}
	}
	fmt.Printf("Readiness: %s\n", readiness.SummaryLine(out.Readiness))
	for _, c := range out.Readiness.Checks {
		mark := "ok"
		if !c.OK {
			mark = "FAIL"
		}
		opt := ""
		if c.Optional {
			opt = " (optional)"
		}
		fmt.Printf("  [%s] %s%s", mark, c.ID, opt)
		if c.Detail != "" {
			fmt.Printf(" — %s", c.Detail)
		}
		fmt.Println()
	}
	for _, w := range out.Readiness.Warnings {
		fmt.Printf("  ! %s\n", w)
	}
	if !out.Readiness.OK {
		os.Exit(1)
	}
	return nil
}

func printStatus(rep readiness.Report, jsonOut bool) error {
	if jsonOut {
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Printf("Readiness (preboot): %s\n", readiness.SummaryLine(rep))
		for _, c := range rep.Checks {
			mark := "ok"
			if !c.OK {
				mark = "FAIL"
			}
			fmt.Printf("  [%s] %s — %s\n", mark, c.ID, c.Detail)
		}
	}
	if !rep.OK {
		os.Exit(1)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}