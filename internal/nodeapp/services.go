package nodeapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/services"
	"github.com/NetSepio/erebrus/internal/store"
)

func runServicesCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: erebrus services list|inspect <id>|remove <id>|serve --name <name> --port <port> [--type <type>]")
	}
	cfg := config.Load()
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()
	reg := &services.Registry{St: st}
	ctx := context.Background()

	switch args[0] {
	case "list":
		items, err := reg.List(ctx)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	case "inspect":
		if len(args) < 2 {
			return fmt.Errorf("usage: erebrus services inspect <service-id>")
		}
		svc, err := reg.Get(ctx, args[1])
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(svc)
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: erebrus services remove <service-id>")
		}
		return reg.Remove(ctx, args[1])
	default:
		return fmt.Errorf("unknown services subcommand %q", args[0])
	}
}

func runServeCLI(args []string) error {
	name, port, typ := "", 0, ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			name = args[i+1]
			i++
		case "--port":
			p, err := strconv.Atoi(args[i+1])
			if err != nil {
				return err
			}
			port = p
			i++
		case "--type":
			typ = args[i+1]
			i++
		}
	}
	if name == "" || port == 0 {
		return fmt.Errorf("usage: erebrus serve --name <name> --port <port> [--type <type>]")
	}
	cfg := config.Load()
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()
	reg := &services.Registry{St: st}
	svc, err := reg.Publish(context.Background(), services.Service{
		Name: name, Port: port, Type: typ,
	})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(svc)
}
