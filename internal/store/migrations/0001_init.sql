-- Erebrus node v2 local state.
CREATE TABLE IF NOT EXISTS peers (
    id               TEXT PRIMARY KEY,            -- gateway-issued VPN client UUID
    name             TEXT NOT NULL,
    wallet           TEXT NOT NULL DEFAULT '',
    wg_public_key    TEXT NOT NULL UNIQUE,
    wg_allowed_ip    TEXT NOT NULL UNIQUE,        -- e.g. 10.0.0.7/32
    wg_preshared_key TEXT NOT NULL DEFAULT '',
    proxy_uuid       TEXT NOT NULL UNIQUE,        -- VLESS user id (Phase 2)
    proxy_password   TEXT NOT NULL DEFAULT '',    -- Hysteria2 password (Phase 2)
    enabled          INTEGER NOT NULL DEFAULT 1,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    expires_at       INTEGER NOT NULL DEFAULT 0   -- unix seconds; 0 = never
);

CREATE INDEX IF NOT EXISTS idx_peers_enabled ON peers(enabled);

-- Key/value for node-level settings: WG server keypair, REALITY keys, ports.
CREATE TABLE IF NOT EXISTS node_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
