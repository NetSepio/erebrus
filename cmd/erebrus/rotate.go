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

	// Carrier rotation is a local DB operation: it only needs the state store
	// and the stealth secrets (node_settings). It must NOT run full node
	// validation (WG_ENDPOINT_HOST/MNEMONIC) or bind the carrier ports — doing
	// so would clash with an already-running node.
	cfg := config.Load()
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()

	stealthMgr := stealth.New(cfg, st)
	if err := stealthMgr.Init(context.Background()); err != nil { // loads/creates secrets, no listeners
		return err
	}

	rot := &carriers.Rotator{St: st, Stealth: stealthMgr}
	if err := rot.Rotate(context.Background(), carriers.Options{GracePeriod: grace, PeerID: peerID}); err != nil {
		return err
	}
	fmt.Println("carrier secrets rotated. Restart the node to serve the new credentials; old ones remain valid for the grace period.")
	return nil
}
