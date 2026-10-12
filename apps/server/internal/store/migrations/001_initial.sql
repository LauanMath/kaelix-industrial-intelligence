CREATE TABLE users (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 email text NOT NULL UNIQUE,
 password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
 token_hash bytea PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);
CREATE TABLE gateways (
 id text PRIMARY KEY,
 name text NOT NULL,
 location text NOT NULL DEFAULT '',
 token_hash bytea NOT NULL UNIQUE,
 created_at timestamptz NOT NULL DEFAULT now(),
 last_ingested_at timestamptz,
 last_heartbeat_at timestamptz,
 heartbeat jsonb
);
CREATE TABLE devices (
 id text PRIMARY KEY CHECK (id ~ '^[0-9A-F]{8}$'),
 profile text NOT NULL DEFAULT '',
 notes text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE motors (
 id text PRIMARY KEY,
 name text NOT NULL,
 location text NOT NULL DEFAULT '',
 device_id text UNIQUE REFERENCES devices(id),
 expected_interval_seconds integer NOT NULL CHECK (expected_interval_seconds BETWEEN 1 AND 86400),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE readings (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 gateway_id text NOT NULL REFERENCES gateways(id),
 gateway_session_id text NOT NULL,
 event_seq bigint NOT NULL CHECK (event_seq > 0),
 device_id text NOT NULL REFERENCES devices(id),
 ingested_at timestamptz NOT NULL DEFAULT now(),
 estimated_received_at timestamptz NOT NULL,
 queue_delay_ms bigint NOT NULL CHECK (queue_delay_ms >= 0),
 payload jsonb NOT NULL,
 UNIQUE (gateway_id, gateway_session_id, event_seq)
);
CREATE INDEX readings_history_idx ON readings(device_id, estimated_received_at DESC, id DESC);
