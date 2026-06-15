package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ServiceACL is an access rule for a service.
type ServiceACL struct {
	ID        string
	ServiceID string
	Subject   string
	Action    string
	CreatedAt int64
}

// InsertServiceACL adds an ACL row.
func (s *Store) InsertServiceACL(ctx context.Context, acl ServiceACL) error {
	if acl.ID == "" {
		acl.ID = uuid.NewString()
	}
	if acl.CreatedAt == 0 {
		acl.CreatedAt = time.Now().Unix()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO service_acls(id,service_id,subject,action,created_at) VALUES(?,?,?,?,?)`,
		acl.ID, acl.ServiceID, acl.Subject, acl.Action, acl.CreatedAt)
	return err
}

// ListServiceACLs returns ACLs for a service.
func (s *Store) ListServiceACLs(ctx context.Context, serviceID string) ([]ServiceACL, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,service_id,subject,action,created_at FROM service_acls WHERE service_id=?`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServiceACL
	for rows.Next() {
		var a ServiceACL
		if err := rows.Scan(&a.ID, &a.ServiceID, &a.Subject, &a.Action, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}