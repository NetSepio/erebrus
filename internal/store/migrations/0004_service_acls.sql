CREATE TABLE IF NOT EXISTS service_acls (
    id         TEXT PRIMARY KEY,
    service_id TEXT NOT NULL,
    subject    TEXT NOT NULL,
    action     TEXT NOT NULL DEFAULT 'connect',
    created_at INTEGER NOT NULL,
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_service_acls_service ON service_acls(service_id);