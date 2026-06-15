package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/carriers"
	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/stealth"
	"github.com/NetSepio/erebrus/internal/store"
	"github.com/NetSepio/erebrus/internal/wg"
)

func runRotateCarriers(args []string) error {
	grace := 24 * time.Hour
	peerID := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--grace-period":
			if i+1 >= len(args) {
				return fmt.Errorf("--grace-period requires a value")
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid grace period: %w", err)
			}
			grace = d
			i++
		case "--peer":
			if i+1 >= len(args) {
				return fmt.Errorf("--peer requires a value")
			}
			peerID = args[i+1]
			i++
		case "carriers":
			continue
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag %s", args[i])
			}
		}
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()

	wgm := wg.New(cfg, st, wg.NewController())
	_ = wgm.Init(context.Background())
	stealthMgr := stealth.New(cfg, st)
	if err := stealthMgr.Init(context.Background()); err != nil {
		return err
	}
	if cfg.EnableStealth {
		_ = stealthMgr.Start(context.Background())
		defer stealthMgr.Close()
	}

	rot := &carriers.Rotator{St: st, Stealth: stealthMgr}
	return rot.Rotate(context.Background(), carriers.Options{GracePeriod: grace, PeerID: peerID})
}