// Package nodeapp runs the Erebrus VPN node runtime and operator CLI.
package nodeapp

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/p2p"
	"github.com/NetSepio/erebrus/internal/telemetry"
	"github.com/joho/godotenv"
)

// Main is the shared entrypoint for erebrus-node and the legacy erebrus binary.
func Main(args []string) {
	if len(args) > 1 {
		switch args[1] {
		case "genmnemonic":
			m, err := p2p.GenerateMnemonic()
			if err != nil {
				fmt.Fprintln(os.Stderr, "genmnemonic:", err)
				os.Exit(1)
			}
			fmt.Println(m)
			return
		case "version", "--version", "-v":
			fmt.Println(config.Version)
			return
		case "templates":
			if err := runTemplatesCLI(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "templates:", err)
				os.Exit(1)
			}
			return
		case "serve":
			if err := runServeCLI(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "serve:", err)
				os.Exit(1)
			}
			return
		case "services":
			if err := runServicesCLI(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "services:", err)
				os.Exit(1)
			}
			return
		case "rotate":
			if len(args) < 3 || args[2] != "carriers" {
				fmt.Fprintln(os.Stderr, "usage: erebrus-node rotate carriers [--grace-period 24h] [--peer <peer-id>]")
				os.Exit(2)
			}
			if err := runRotateCarriers(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "rotate:", err)
				os.Exit(1)
			}
			return
		case "status":
			if err := runStatusCLI(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "status:", err)
				os.Exit(1)
			}
			return
		case "init":
			if err := runInitCLI(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "init:", err)
				os.Exit(1)
			}
			return
		case "doctor":
			if err := runDoctorCLI(args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "doctor:", err)
				os.Exit(1)
			}
			return
		}
	}

	if os.Getenv("LOAD_CONFIG_FILE") == "" {
		_ = godotenv.Load()
	}

	cfg := config.Load()
	telemetry.InitLogger(cfg.RunType == "debug")

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	for _, w := range cfg.Mode.Warnings {
		slog.Warn(w)
	}
	slog.Info("runtime settings",
		"profile", cfg.ErebrusProfile,
		"access", cfg.Mode.RuntimeMode,
		"deploy", cfg.Mode.Deploy,
		"network_profile", cfg.Mode.NetworkProfile,
		"firewall_provider", cfg.FirewallProvider,
		"api_bind", fmt.Sprintf("%s:%s", cfg.BindAddr, cfg.HTTPPort),
	)

	if err := Run(cfg); err != nil {
		slog.Error("node exited with error", "err", err)
		os.Exit(1)
	}
}