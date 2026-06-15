-- Carrier credential rotation audit trail (hashes only, no plaintext history).
CREATE TABLE IF NOT EXISTS carrier_credentials (
    id          TEXT PRIMARY KEY,
    transport   TEXT NOT NULL,
    secret_hash TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL DEFAULT 0,
    active      INTEGER NOT NULL DEFAULT 1,
    scope       TEXT NOT NULL DEFAULT 'node',
    peer_id     TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_carrier_credentials_active ON carrier_credentials(active, expires_at);