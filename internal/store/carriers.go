package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CarrierCredential is a rotated carrier secret record (hash only).
type CarrierCredential struct {
	ID         string
	Transport  string
	SecretHash string
	CreatedAt  int64
	ExpiresAt  int64 // 0 = no expiry
	Active     bool
	Scope      string
	PeerID     string
}

// InsertCarrierCredential records a carrier credential hash.
func (s *Store) InsertCarrierCredential(ctx context.Context, c CarrierCredential) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	now := time.Now().Unix()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	active := 0
	if c.Active {
		active = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO carrier_credentials(id,transport,secret_hash,created_at,expires_at,active,scope,peer_id)
		 VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.Transport, c.SecretHash, c.CreatedAt, c.ExpiresAt, active, c.Scope, c.PeerID)
	return err
}

// DeactivateExpiredCarrierCredentials marks expired rows inactive.
func (s *Store) DeactivateExpiredCarrierCredentials(ctx context.Context, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE carrier_credentials SET active=0 WHERE active=1 AND expires_at > 0 AND expires_at <= ?`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListActiveCarrierCredentials returns active credential records.
func (s *Store) ListActiveCarrierCredentials(ctx context.Context) ([]CarrierCredential, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,transport,secret_hash,created_at,expires_at,active,scope,peer_id
		 FROM carrier_credentials WHERE active=1 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CarrierCredential
	for rows.Next() {
		var c CarrierCredential
		var active int
		if err := rows.Scan(&c.ID, &c.Transport, &c.SecretHash, &c.CreatedAt, &c.ExpiresAt, &active, &c.Scope, &c.PeerID); err != nil {
			return nil, err
		}
		c.Active = active == 1
		out = append(out, c)
	}
	return out, rows.Err()
}