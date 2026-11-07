CREATE TABLE IF NOT EXISTS gateway_admin (
 gateway_id TEXT PRIMARY KEY,
 admin_key TEXT NOT NULL
);
PRAGMA user_version=4;
