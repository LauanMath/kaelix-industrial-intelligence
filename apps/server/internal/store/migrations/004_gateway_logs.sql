CREATE TABLE gateway_logs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 gateway_id text NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
 gateway_session_id text NOT NULL,
 log_seq bigint NOT NULL CHECK (log_seq BETWEEN 1 AND 4294967295),
 received_at timestamptz NOT NULL DEFAULT now(),
 payload jsonb NOT NULL,
 UNIQUE (gateway_id, gateway_session_id, log_seq)
);
CREATE INDEX gateway_logs_cursor_idx ON gateway_logs(gateway_id, id);
