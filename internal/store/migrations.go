package store

const schema = `
CREATE TABLE IF NOT EXISTS ip_rotations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    modem_id    TEXT    NOT NULL,
    old_ip      TEXT,
    new_ip      TEXT    NOT NULL,
    duration_ms INTEGER,
    rotated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS modem_events (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    modem_id  TEXT NOT NULL,
    event     TEXT NOT NULL,
    detail    TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rotations_modem ON ip_rotations(modem_id, rotated_at);
CREATE INDEX IF NOT EXISTS idx_events_modem ON modem_events(modem_id, timestamp);
`
