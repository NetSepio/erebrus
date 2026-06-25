package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ServiceRow is the DB representation of a registered service.
type ServiceRow struct {
	ID           string
	Name         string
	Type         string
	Protocol     string
	InternalAddr string
	Port         int
	OwnerPeerID  string
	OwnerDID     string
	Visibility   string
	AuthMode     string
	Tags         string
	Public       int
	PublicHost   string
	CreatedAt    int64
	UpdatedAt    int64
}

// UpsertService inserts or replaces a service by id.
func (s *Store) UpsertService(ctx context.Context, row ServiceRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO services(id,name,type,protocol,internal_addr,port,owner_peer_id,owner_did,
		 visibility,auth_mode,tags,public,public_hostname,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, type=excluded.type, protocol=excluded.protocol,
		   internal_addr=excluded.internal_addr, port=excluded.port,
		   visibility=excluded.visibility, auth_mode=excluded.auth_mode, tags=excluded.tags,
		   public=excluded.public, public_hostname=excluded.public_hostname, updated_at=excluded.updated_at`,
		row.ID, row.Name, row.Type, row.Protocol, row.InternalAddr, row.Port,
		row.OwnerPeerID, row.OwnerDID, row.Visibility, row.AuthMode, row.Tags,
		row.Public, row.PublicHost, row.CreatedAt, row.UpdatedAt)
	return err
}

// ListServices returns all registered services.
func (s *Store) ListServices(ctx context.Context) ([]ServiceRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,type,protocol,internal_addr,port,owner_peer_id,owner_did,
		        visibility,auth_mode,tags,public,public_hostname,created_at,updated_at
		 FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

// GetService fetches a service by id.
func (s *Store) GetService(ctx context.Context, id string) (*ServiceRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id,name,type,protocol,internal_addr,port,owner_peer_id,owner_did,
		        visibility,auth_mode,tags,public,public_hostname,created_at,updated_at
		 FROM services WHERE id=?`, id)
	out, err := scanService(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: service", ErrNotFound)
	}
	return out, err
}

// GetServiceByName fetches the first service matching name.
func (s *Store) GetServiceByName(ctx context.Context, name string) (*ServiceRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id,name,type,protocol,internal_addr,port,owner_peer_id,owner_did,
		        visibility,auth_mode,tags,public,public_hostname,created_at,updated_at
		 FROM services WHERE name=? LIMIT 1`, name)
	out, err := scanService(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: service", ErrNotFound)
	}
	return out, err
}

// DeleteService removes a service by id.
func (s *Store) DeleteService(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: service", ErrNotFound)
	}
	return nil
}

// SetServicePublic updates public exposure fields.
func (s *Store) SetServicePublic(ctx context.Context, id, hostname string, public bool) error {
	pub := 0
	if public {
		pub = 1
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE services SET public=?, public_hostname=?, updated_at=? WHERE id=?`,
		pub, hostname, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: service", ErrNotFound)
	}
	return nil
}

func scanServices(rows *sql.Rows) ([]ServiceRow, error) {
	var out []ServiceRow
	for rows.Next() {
		var r ServiceRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Protocol, &r.InternalAddr, &r.Port,
			&r.OwnerPeerID, &r.OwnerDID, &r.Visibility, &r.AuthMode, &r.Tags,
			&r.Public, &r.PublicHost, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanService(scan func(dest ...any) error) (*ServiceRow, error) {
	var r ServiceRow
	err := scan(&r.ID, &r.Name, &r.Type, &r.Protocol, &r.InternalAddr, &r.Port,
		&r.OwnerPeerID, &r.OwnerDID, &r.Visibility, &r.AuthMode, &r.Tags,
		&r.Public, &r.PublicHost, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
