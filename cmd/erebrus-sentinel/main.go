// Command erebrus-sentinel runs the Unbound-powered licensed firewall sidecar API.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/NetSepio/erebrus/internal/sentinel/api"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println("erebrus-sentinel/0.1.0")
			return
		}
	}
	addr := os.Getenv("SENTINEL_LISTEN_ADDR")
	if addr == "" {
		addr = ":8788"
	}
	srv := api.New(addr, api.ConfigDir())
	slog.Info("starting erebrus-sentinel", "addr", addr, "conf_dir", api.ConfigDir())
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("sentinel stopped", "err", err)
		os.Exit(1)
	}
}
