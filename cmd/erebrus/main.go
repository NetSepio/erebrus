// Command erebrus is the Erebrus v2 VPN node. It serves an HTTP REST API
// (/api/v2), manages WireGuard peers backed by SQLite, derives its identity
// and DID from a mnemonic, and advertises on the libp2p DHT. The v1 gRPC
// server, libp2p status pubsub, Docker-agent and Caddy subsystems are gone.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NetSepio/erebrus/internal/api"
	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/gatewayclient"
	"github.com/NetSepio/erebrus/internal/node"
	"github.com/NetSepio/erebrus/internal/p2p"
	"github.com/NetSepio/erebrus/internal/registrar"
	"github.com/NetSepio/erebrus/internal/stealth"
	"github.com/NetSepio/erebrus/internal/store"
	"github.com/NetSepio/erebrus/internal/telemetry"
	"github.com/NetSepio/erebrus/internal/transport/probe"
	"github.com/NetSepio/erebrus/internal/wg"
	"github.com/joho/godotenv"
)

func main() {
	// Lightweight CLI subcommands used by the installer and operators. These run
	// without loading the full node configuration.
	if len(os.Args) > 1 {
		switch os.Args[1] {
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
		case "serve":
			if err := runServeCLI(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "serve:", err)
				os.Exit(1)
			}
			return
		case "services":
			if err := runServicesCLI(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "services:", err)
				os.Exit(1)
			}
			return
		case "rotate":
			if len(os.Args) < 3 || os.Args[2] != "carriers" {
				fmt.Fprintln(os.Stderr, "usage: erebrus rotate carriers [--grace-period 24h] [--peer <peer-id>]")
				os.Exit(2)
			}
			if err := runRotateCarriers(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "rotate:", err)
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
	slog.Info("runtime mode",
		"mode", cfg.Mode.RuntimeMode,
		"network_profile", cfg.Mode.NetworkProfile,
		"api_bind", fmt.Sprintf("%s:%s", cfg.BindAddr, cfg.HTTPPort),
	)

	if err := run(cfg); err != nil {
		slog.Error("node exited with error", "err", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if best, ok := probe.Select(ctx, &probe.LocalProber{
		StealthEnabled: cfg.EnableStealth,
		WGPort:         cfg.WGEndpointPortInt(),
		VLESSPort:      cfg.VLESSPortInt(),
		Hysteria2Port:  cfg.Hysteria2PortInt(),
	}, cfg.EnableStealth); ok {
		slog.Info("transport ladder", "preferred", best.Kind, "score", best.Score)
	}

	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	// Identity / DID from the mnemonic.
	peerID, did, err := p2p.PeerIDFromMnemonic(cfg.Mnemonic)
	if err != nil {
		return fmt.Errorf("derive identity: %w", err)
	}
	slog.Info("node identity", "peer_id", peerID, "did", did, "node", cfg.NodeName)

	// Store.
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	// Metrics.
	metrics := telemetry.NewMetrics()

	// WireGuard.
	wgm := wg.New(cfg, st, wg.NewController())
	if err := wgm.Init(ctx); err != nil {
		// Non-fatal: the conf is written; the interface may need NET_ADMIN.
		slog.Warn("wireguard interface init incomplete", "err", err)
	}

	// Stealth carriers (sing-box VLESS+REALITY / Hysteria2). Init always runs so
	// credential bundles can advertise carrier params; Start is a no-op when
	// disabled. A start failure (e.g. port in use) is non-fatal — WireGuard
	// still serves the fast path.
	stealthMgr := stealth.New(cfg, st)
	if err := stealthMgr.Init(ctx); err != nil {
		slog.Warn("stealth init failed; carriers unavailable", "err", err)
	} else if err := stealthMgr.Start(ctx); err != nil {
		slog.Warn("stealth carriers failed to start", "err", err)
	} else {
		if cfg.EnableStealth {
			slog.Info("stealth carriers listening", "vless_port", cfg.VLESSPort, "hysteria2_port", cfg.Hysteria2Port)
		}
		defer stealthMgr.Close()
	}

	// libp2p host (identity + DID + DHT advertise). Best-effort.
	p2pNode, err := p2p.Start(ctx, cfg.Mnemonic, cfg.P2PListenPort, cfg.GatewayPeerMultiaddr)
	if err != nil {
		slog.Warn("libp2p host failed to start", "err", err)
	} else {
		defer p2pNode.Close()
	}

	// On-chain registration (noop in v2.0).
	reg := registrar.New(cfg.ChainRegistration)
	if err := reg.Register(ctx, registrar.NodeIdentity{
		PeerID:  peerID,
		DID:     did,
		IPHash:  registrar.HashIP(cfg.WGEndpointHost),
		Region:  cfg.Region,
		Wallet:  "",
		Version: cfg.Version,
	}); err != nil {
		slog.Warn("registrar register failed", "err", err)
	}

	// Core service + HTTP API.
	svc := node.New(cfg, st, wgm, stealthMgr, metrics)
	apiServer := api.NewServer(cfg, svc, api.Identity{PeerID: peerID, DID: did})
	svc.SetAPIStatusHook(apiServer.SetStatus)

	// Gateway control plane (WebSocket + PASETO). Best-effort when configured.
	if cfg.GatewayEnabled() {
		nodeID, nodeToken, err := gatewayclient.LoadCredentials(ctx, st)
		if err != nil {
			slog.Warn("load gateway credentials failed", "err", err)
		}
		if nodeID == "" {
			nodeID = cfg.NodeID
		}
		if nodeToken == "" {
			nodeToken = cfg.NodeToken
		}
		if (nodeID == "" || nodeToken == "") && cfg.GatewayAutoRegister {
			reg, err := gatewayclient.Register(ctx, gatewayclient.RegistrationInput{
				GatewayURL:  cfg.GatewayURL,
				AuthEULA:    cfg.AuthEULA,
				WalletChain: cfg.WalletChain,
				Mnemonic:    cfg.Mnemonic,
				PeerID:      peerID,
				DID:         did,
				Name:        cfg.NodeName,
				Region:      cfg.Region,
				APIBaseURL:  cfg.PublicAPIBaseURL(),
				APIToken:    cfg.NodeAPIToken,
			})
			if err != nil {
				slog.Warn("gateway registration failed", "err", err)
			} else {
				nodeID, nodeToken = reg.NodeID, reg.NodeToken
				if err := gatewayclient.SaveCredentials(ctx, st, nodeID, nodeToken); err != nil {
					slog.Warn("persist gateway credentials failed", "err", err)
				} else {
					slog.Info("gateway registered", "node_id", nodeID)
				}
			}
		}
		if nodeID != "" && nodeToken != "" {
			bridge := node.NewGatewayBridge(svc, peerID, did, nodeID)
			gw := gatewayclient.New(cfg.GatewayURL, nodeID, nodeToken, bridge, bridge, bridge.Status)
			go gw.Run(ctx)
		} else {
			slog.Warn("gateway URL set but node credentials missing — WS control plane disabled")
		}
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.BindAddr, cfg.HTTPPort),
		Handler:           apiServer.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("HTTP API listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}
