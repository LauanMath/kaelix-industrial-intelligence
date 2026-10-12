-- Preserve snapshots from the earlier, incompatible heartbeat shape.
CREATE TABLE legacy_gateway_heartbeats (
 gateway_id text PRIMARY KEY REFERENCES gateways(id),
 received_at timestamptz,
 payload jsonb NOT NULL
);
INSERT INTO legacy_gateway_heartbeats SELECT id,last_heartbeat_at,heartbeat FROM gateways WHERE heartbeat IS NOT NULL;
UPDATE gateways SET heartbeat=NULL,last_heartbeat_at=NULL;
ALTER TABLE gateways ADD COLUMN active_session_id text;
CREATE TABLE gateway_sessions (
 gateway_id text NOT NULL REFERENCES gateways(id),
 session_id text NOT NULL,
 first_seen_at timestamptz NOT NULL DEFAULT now(),
 retired boolean NOT NULL DEFAULT false,
 last_heartbeat_seq bigint NOT NULL DEFAULT 0,
 latest_heartbeat jsonb,
 last_heartbeat_at timestamptz,
 PRIMARY KEY(gateway_id,session_id)
);
ALTER TABLE gateways ADD CONSTRAINT gateways_active_session_fk FOREIGN KEY(id,active_session_id) REFERENCES gateway_sessions(gateway_id,session_id);
CREATE TABLE gateway_heartbeats (
 gateway_id text NOT NULL,
 session_id text NOT NULL,
 heartbeat_seq bigint NOT NULL CHECK(heartbeat_seq BETWEEN 1 AND 4294967295),
 payload jsonb NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(gateway_id,session_id,heartbeat_seq),
 FOREIGN KEY(gateway_id,session_id) REFERENCES gateway_sessions(gateway_id,session_id)
);
ALTER TABLE readings ALTER COLUMN estimated_received_at DROP NOT NULL;
DROP INDEX readings_history_idx;
CREATE INDEX readings_history_idx ON readings(device_id,(COALESCE(estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz)) DESC,id DESC);
