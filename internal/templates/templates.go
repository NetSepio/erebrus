// Package templates provides one-command service setup presets.
package templates

import (
	"context"
	"fmt"

	"github.com/NetSepio/erebrus/internal/services"
)

// Template describes a built-in service preset.
type Template struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Ports             []int  `json:"ports"`
	Protocol          string `json:"protocol"`
	DefaultVisibility string `json:"default_visibility"`
	DefaultAuth       string `json:"default_auth"`
	Description       string `json:"description"`
}

// Catalog returns built-in templates.
func Catalog() []Template {
	return []Template{
		{Name: "drop-room", Type: "webdav", Ports: []int{8787}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "WebDAV/file sharing"},
		{Name: "ollama", Type: "ai.llm", Ports: []int{11434}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "Local Ollama LLM API"},
		{Name: "openai-compatible-llm", Type: "ai.llm", Ports: []int{8080}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "OpenAI-compatible local endpoint"},
		{Name: "nextcloud", Type: "web", Ports: []int{8080}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "Nextcloud"},
		{Name: "home-assistant", Type: "web", Ports: []int{8123}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "Home Assistant"},
		{Name: "dashboard", Type: "web", Ports: []int{3000}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "Local web dashboard"},
		{Name: "static-site", Type: "web", Ports: []int{8080}, Protocol: "http", DefaultVisibility: "private", DefaultAuth: "vpn-peer", Description: "Simple static site"},
	}
}

// Find returns a template by name.
func Find(name string) (*Template, error) {
	for _, t := range Catalog() {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unknown template %q", name)
}

// Install registers a service from a template.
func Install(ctx context.Context, reg *services.Registry, name string) (*services.Service, error) {
	tpl, err := Find(name)
	if err != nil {
		return nil, err
	}
	port := tpl.Ports[0]
	return reg.Publish(ctx, services.Service{
		Name:       tpl.Name,
		Type:       tpl.Type,
		Port:       port,
		Protocol:   tpl.Protocol,
		Visibility: tpl.DefaultVisibility,
		AuthMode:   tpl.DefaultAuth,
	})
}
