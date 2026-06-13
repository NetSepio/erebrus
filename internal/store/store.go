// Package store is the node-local persistence layer, backed by SQLite
// (modernc.org/sqlite, pure Go — CGO-free). It replaces the v1 per-UUID JSON
// files and gives us atomic multi-protocol provisioning and race-free IP
// allocation.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a peer does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Peer is a provisioned VPN client on this node.
type Peer struct {
	ID             string
	Name           string
	Wallet         string
	WGPublicKey    string
	WGAllowedIP    string // CIDR, e.g. 10.0.0.7/32
	WGPresharedKey string
	ProxyUUID      string
	ProxyPassword  string
	Enabled        bool
	CreatedAt      int64
	UpdatedAt      int64
	ExpiresAt      int64
}

// Open opens (creating if necessary) the SQLite database at path and applies
// migrations. Busy timeout + WAL keep concurrent reads smooth.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite is single-writer; cap connections to avoid lock churn.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(string(b)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// --- node_settings ---

// GetSetting returns a setting value; ("", nil) if absent.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM node_settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetSetting upserts a setting.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO node_settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// --- peers ---

// GetPeer returns a peer by id.
func (s *Store) GetPeer(ctx context.Context, id string) (*Peer, error) {
	row := s.db.QueryRowContext(ctx, selectCols+` WHERE id = ?`, id)
	p, err := scanPeer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ListPeers returns all peers ordered by creation time.
func (s *Store) ListPeers(ctx context.Context) ([]*Peer, error) {
	rows, err := s.db.QueryContext(ctx, selectCols+` ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Peer
	for rows.Next() {
		p, err := scanPeer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePeer removes a peer. Idempotent: deleting a missing peer is not an error.
func (s *Store) DeletePeer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM peers WHERE id = ?`, id)
	return err
}

// UpsertPeer creates or updates a peer, allocating a WireGuard IP from subnet
// on first creation. The whole operation runs in one immediate transaction so
// IP allocation is race-free even under concurrent calls. On update, the
// allocated IP and generated proxy credentials are preserved.
//
// gen supplies freshly generated values used only when creating a new peer.
func (s *Store) UpsertPeer(ctx context.Context, in *Peer, subnet string, gen GeneratedCreds) (*Peer, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	existing, err := txGetPeer(ctx, tx, in.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	now := time.Now().Unix()
	if existing != nil {
		// Update: preserve IP and proxy credentials, refresh mutable fields.
		existing.Name = in.Name
		existing.Wallet = in.Wallet
		existing.WGPublicKey = in.WGPublicKey
		existing.WGPresharedKey = in.WGPresharedKey
		existing.Enabled = in.Enabled
		existing.ExpiresAt = in.ExpiresAt
		existing.UpdatedAt = now
		if _, err := tx.ExecContext(ctx,
			`UPDATE peers SET name=?, wallet=?, wg_public_key=?, wg_preshared_key=?,
			 enabled=?, updated_at=?, expires_at=? WHERE id=?`,
			existing.Name, existing.Wallet, existing.WGPublicKey, existing.WGPresharedKey,
			boolToInt(existing.Enabled), existing.UpdatedAt, existing.ExpiresAt, existing.ID); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Create: allocate the next free IP within the transaction.
	allocated, err := txAllocateIP(ctx, tx, subnet)
	if err != nil {
		return nil, err
	}
	p := &Peer{
		ID:             in.ID,
		Name:           in.Name,
		Wallet:         in.Wallet,
		WGPublicKey:    in.WGPublicKey,
		WGAllowedIP:    allocated,
		WGPresharedKey: in.WGPresharedKey,
		ProxyUUID:      gen.ProxyUUID,
		ProxyPassword:  gen.ProxyPassword,
		Enabled:        in.Enabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      in.ExpiresAt,
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO peers(id,name,wallet,wg_public_key,wg_allowed_ip,wg_preshared_key,
		 proxy_uuid,proxy_password,enabled,created_at,updated_at,expires_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Wallet, p.WGPublicKey, p.WGAllowedIP, p.WGPresharedKey,
		p.ProxyUUID, p.ProxyPassword, boolToInt(p.Enabled), p.CreatedAt, p.UpdatedAt, p.ExpiresAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

// GeneratedCreds carries freshly minted credentials for a new peer.
type GeneratedCreds struct {
	ProxyUUID     string
	ProxyPassword string
}

const selectCols = `SELECT id,name,wallet,wg_public_key,wg_allowed_ip,wg_preshared_key,
 proxy_uuid,proxy_password,enabled,created_at,updated_at,expires_at FROM peers`

type scanner interface {
	Scan(dest ...any) error
}

func scanPeer(sc scanner) (*Peer, error) {
	var p Peer
	var enabled int
	err := sc.Scan(&p.ID, &p.Name, &p.Wallet, &p.WGPublicKey, &p.WGAllowedIP, &p.WGPresharedKey,
		&p.ProxyUUID, &p.ProxyPassword, &enabled, &p.CreatedAt, &p.UpdatedAt, &p.ExpiresAt)
	if err != nil {
		return nil, err
	}
	p.Enabled = enabled != 0
	return &p, nil
}

func txGetPeer(ctx context.Context, tx *sql.Tx, id string) (*Peer, error) {
	return scanPeer(tx.QueryRowContext(ctx, selectCols+` WHERE id = ?`, id))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
