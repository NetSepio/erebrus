package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/services"
)

func runServicePublish(reg *services.Registry, ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: erebrus services publish <service-id> [--public]")
	}
	id := args[0]
	public := false
	for _, a := range args[1:] {
		if a == "--public" {
			public = true
		}
	}
	cfg := config.Load()
	svc, err := reg.Get(ctx, id)
	if err != nil {
		return err
	}
	if !cfg.Mode.IsPublic() {
		return fmt.Errorf("public publish requires public access mode")
	}
	domain := cfg.PublicDomain
	if domain == "" {
		domain = cfg.AppWildcardDomain
	}
	hostname := ""
	if public && domain != "" {
		hostname = fmt.Sprintf("%s.%s", svc.Name, strings.TrimPrefix(domain, "*."))
	}
	st := reg.St
	if err := st.SetServicePublic(ctx, id, hostname, public); err != nil {
		return err
	}
	if public {
		svc.Visibility = "public"
		svc.AuthMode = "public"
	}
	svc.Public = public
	svc.PublicHost = hostname
	_, err = reg.Publish(ctx, *svc)
	return err
}

func runServiceUnpublish(reg *services.Registry, ctx context.Context, id string) error {
	return reg.St.SetServicePublic(ctx, id, "", false)
}
