// Package services implements the private service registry.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/store"
	"github.com/google/uuid"
)

// Service is a published private (or public) service on the VPN.
type Service struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Protocol     string   `json:"protocol"`
	InternalAddr string   `json:"internal_addr"`
	Port         int      `json:"port"`
	OwnerPeerID  string   `json:"owner_peer_id"`
	OwnerDID     string   `json:"owner_did"`
	Visibility   string   `json:"visibility"`
	AuthMode     string   `json:"auth_mode"`
	Tags         []string `json:"tags"`
	CreatedAt    int64    `json:"created_at"`
	UpdatedAt    int64    `json:"updated_at"`
}

// Registry persists and queries services.
type Registry struct {
	St *store.Store
}

// Publish registers or updates a service.
func (r *Registry) Publish(ctx context.Context, s Service) (*Service, error) {
	if s.Name == "" || s.Port <= 0 {
		return nil, fmt.Errorf("name and port are required")
	}
	now := time.Now().Unix()
	if s.ID == "" {
		s.ID = "svc_" + strings.ReplaceAll(s.Name, " ", "-") + "_" + uuid.NewString()[:8]
	}
	if s.Protocol == "" {
		s.Protocol = "http"
	}
	if s.Visibility == "" {
		s.Visibility = "private"
	}
	if s.AuthMode == "" {
		s.AuthMode = "vpn-peer"
	}
	if s.InternalAddr == "" {
		s.InternalAddr = fmt.Sprintf("127.0.0.1:%d", s.Port)
	}
	s.CreatedAt = now
	s.UpdatedAt = now
	tags, _ := json.Marshal(s.Tags)
	if err := r.St.UpsertService(ctx, store.ServiceRow{
		ID: s.ID, Name: s.Name, Type: s.Type, Protocol: s.Protocol,
		InternalAddr: s.InternalAddr, Port: s.Port,
		OwnerPeerID: s.OwnerPeerID, OwnerDID: s.OwnerDID,
		Visibility: s.Visibility, AuthMode: s.AuthMode, Tags: string(tags),
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}); err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns all services.
func (r *Registry) List(ctx context.Context) ([]Service, error) {
	rows, err := r.St.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToService(row))
	}
	return out, nil
}

// Get returns one service by id.
func (r *Registry) Get(ctx context.Context, id string) (*Service, error) {
	row, err := r.St.GetService(ctx, id)
	if err != nil {
		return nil, err
	}
	s := rowToService(*row)
	return &s, nil
}

// Remove deletes a service.
func (r *Registry) Remove(ctx context.Context, id string) error {
	return r.St.DeleteService(ctx, id)
}

// FindByName resolves a service for DNS (first match on name).
func (r *Registry) FindByName(ctx context.Context, name string) (*Service, error) {
	row, err := r.St.GetServiceByName(ctx, name)
	if err != nil {
		return nil, err
	}
	s := rowToService(*row)
	return &s, nil
}

func rowToService(row store.ServiceRow) Service {
	var tags []string
	_ = json.Unmarshal([]byte(row.Tags), &tags)
	return Service{
		ID: row.ID, Name: row.Name, Type: row.Type, Protocol: row.Protocol,
		InternalAddr: row.InternalAddr, Port: row.Port,
		OwnerPeerID: row.OwnerPeerID, OwnerDID: row.OwnerDID,
		Visibility: row.Visibility, AuthMode: row.AuthMode, Tags: tags,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
