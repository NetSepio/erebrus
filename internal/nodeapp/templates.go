package nodeapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/services"
	"github.com/NetSepio/erebrus/internal/store"
	"github.com/NetSepio/erebrus/internal/templates"
)

func runTemplatesCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: erebrus templates list|install <name>")
	}
	switch args[0] {
	case "list":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(templates.Catalog())
	case "install":
		if len(args) < 2 {
			return fmt.Errorf("usage: erebrus templates install <name>")
		}
		cfg := config.Load()
		st, err := store.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer st.Close()
		reg := &services.Registry{St: st}
		svc, err := templates.Install(context.Background(), reg, args[1])
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(svc)
	default:
		return fmt.Errorf("unknown templates subcommand %q", args[0])
	}
}
