CREATE TABLE IF NOT EXISTS services (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    type          TEXT NOT NULL DEFAULT '',
    protocol      TEXT NOT NULL DEFAULT 'http',
    internal_addr TEXT NOT NULL,
    port          INTEGER NOT NULL,
    owner_peer_id TEXT NOT NULL DEFAULT '',
    owner_did     TEXT NOT NULL DEFAULT '',
    visibility    TEXT NOT NULL DEFAULT 'private',
    auth_mode     TEXT NOT NULL DEFAULT 'vpn-peer',
    tags          TEXT NOT NULL DEFAULT '',
    public        INTEGER NOT NULL DEFAULT 0,
    public_hostname TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_services_name ON services(name);
CREATE INDEX IF NOT EXISTS idx_services_visibility ON services(visibility);