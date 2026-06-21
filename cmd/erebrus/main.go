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
	dnspkg "github.com/NetSepio/erebrus/internal/dns"
	"github.com/NetSepio/erebrus/internal/edge"
	"github.com/NetSepio/erebrus/internal/gatewayclient"
	"github.com/NetSepio/erebrus/internal/node"
	"github.com/NetSepio/erebrus/internal/p2p"
	"github.com/NetSepio/erebrus/internal/readiness"
	"github.com/NetSepio/erebrus/internal/registrar"
	"github.com/NetSepio/erebrus/internal/services"
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
		case "templates":
			if err := runTemplatesCLI(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "templates:", err)
				os.Exit(1)
			}
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
		case "status":
			if err := runStatusCLI(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "status:", err)
				os.Exit(1)
			}
			return
		case "init":
			if err := runInitCLI(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "init:", err)
				os.Exit(1)
			}
			return
		case "doctor":
			if err := runDoctorCLI(os.Args[2:]); err != nil {
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
	wgErr := wgm.Init(ctx)
	wgOK := wgErr == nil
	if !wgOK {
		// Non-fatal: the conf is written; the interface may need NET_ADMIN.
		slog.Warn("wireguard interface init incomplete", "err", wgErr)
	}

	// Stealth carriers (sing-box VLESS+REALITY / Hysteria2). Init always runs so
	// credential bundles can advertise carrier params; Start is a no-op when
	// disabled. A start failure (e.g. port in use) is non-fatal — WireGuard
	// still serves the fast path.
	stealthMgr := stealth.New(cfg, st)
	stealthOK := false
	if err := stealthMgr.Init(ctx); err != nil {
		slog.Warn("stealth init failed; carriers unavailable", "err", err)
	} else if err := stealthMgr.Start(ctx); err != nil {
		slog.Warn("stealth carriers failed to start", "err", err)
	} else {
		stealthOK = cfg.EnableStealth
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

	// Private DNS (optional).
	svcReg := &services.Registry{St: st}
	if cfg.PrivateDNSEnabled {
		dnsCfg := dnspkg.Config{
			Enabled:    true,
			Domain:     cfg.PrivateDNSDomain,
			ListenAddr: dnspkg.DefaultListenAddr(cfg.WGIPv4Subnet, cfg.PrivateDNSAddr),
			Upstream:   cfg.UpstreamDNS,
			QueryLogs:  cfg.DNSQueryLogs,
		}
		if err := dnsCfg.Validate(); err != nil {
			slog.Warn("private DNS disabled", "err", err)
		} else {
			go func() {
				if err := dnspkg.New(dnsCfg, svcReg).Start(ctx); err != nil {
					slog.Warn("private DNS stopped", "err", err)
				}
			}()
		}
	}

	// Core service + HTTP API.
	svc := node.New(cfg, st, wgm, stealthMgr, metrics)
	apiServer := api.NewServer(cfg, svc, api.Identity{PeerID: peerID, DID: did})
	svc.SetAPIStatusHook(apiServer.SetStatus)

	// Public edge proxy (Gateway Mode only, opt-in).
	if cfg.Mode.IsPublic() && cfg.PublicGatewayEnabled {
		edgeProxy := &edge.Proxy{Reg: svcReg, St: st, WildcardDomain: cfg.WildcardDomain}
		edgeSrv := &http.Server{
			Addr:              ":9081",
			Handler:           edgeProxy.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("public edge proxy listening", "addr", edgeSrv.Addr)
			if err := edgeSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Warn("edge proxy error", "err", err)
			}
		}()
		defer func() {
			shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = edgeSrv.Shutdown(shut)
		}()
	}

	// Gateway control plane (WebSocket + PASETO). Best-effort when configured.
	var gwClient *gatewayclient.Client
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
			gwClient = gatewayclient.New(cfg.GatewayURL, nodeID, nodeToken, bridge, bridge, bridge.Status)
			go gwClient.Run(ctx)
		} else {
			slog.Warn("gateway URL set but node credentials missing — WS control plane disabled")
		}
	}

	apiServer.SetReadinessProvider(func() readiness.Input {
		gwReg := false
		gwConn := false
		if cfg.GatewayEnabled() {
			if id, tok, err := gatewayclient.LoadCredentials(ctx, st); err == nil && id != "" && tok != "" {
				gwReg = true
			}
			if gwClient != nil {
				gwConn = gwClient.Connected()
			}
		}
		return readiness.Input{
			Cfg:                cfg,
			IdentityConfigured: true,
			GatewayRegistered:  gwReg,
			GatewayConnected:   gwConn,
			WireGuardOK:        wgOK,
			StealthListening:   stealthOK,
		}
	})

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
