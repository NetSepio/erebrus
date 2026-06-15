CREATE TABLE IF NOT EXISTS service_domains (
    service_id TEXT NOT NULL,
    domain     TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (service_id, domain),
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_service_domains_domain ON service_domains(domain);