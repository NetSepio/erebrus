package nodeapp

import (
	"context"
	"fmt"

	"github.com/NetSepio/erebrus/internal/store"
)

func runServiceDomain(st *store.Store, ctx context.Context, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: erebrus services domain add|remove <service-id> <domain>")
	}
	switch args[0] {
	case "add":
		return st.AddServiceDomain(ctx, args[1], args[2])
	case "remove":
		return st.RemoveServiceDomain(ctx, args[1], args[2])
	default:
		return fmt.Errorf("unknown domain subcommand %q", args[0])
	}
}
