package store

import (
	"context"
	"time"
)

// AddServiceDomain maps a custom domain to a service.
func (s *Store) AddServiceDomain(ctx context.Context, serviceID, domain string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO service_domains(service_id,domain,created_at) VALUES(?,?,?)
		 ON CONFLICT(service_id,domain) DO NOTHING`,
		serviceID, domain, time.Now().Unix())
	return err
}

// RemoveServiceDomain removes a custom domain mapping.
func (s *Store) RemoveServiceDomain(ctx context.Context, serviceID, domain string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM service_domains WHERE service_id=? AND domain=?`, serviceID, domain)
	return err
}

// GetServiceByDomain finds a service id for a custom domain.
func (s *Store) GetServiceByDomain(ctx context.Context, domain string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT service_id FROM service_domains WHERE domain=?`, domain).Scan(&id)
	return id, err
}

// ListServiceDomains returns custom domains for a service.
func (s *Store) ListServiceDomains(ctx context.Context, serviceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT domain FROM service_domains WHERE service_id=? ORDER BY domain`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
