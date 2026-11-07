-- version/config stay null until a user saves a configuration; the row can exist before
-- that to remember what the transmitter reports.
CREATE TABLE device_configs (
 device_id text PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
 version integer CHECK (version BETWEEN 1 AND 65535),
 config jsonb,
 updated_at timestamptz,
 radiated_version integer CHECK (radiated_version BETWEEN 0 AND 65535),
 radiated_at timestamptz,
 reported_version integer CHECK (reported_version BETWEEN 0 AND 65535),
 reported_at timestamptz,
 reported_session_id text,
 reported_event_seq bigint,
 reported_boot_count bigint,
 reported_sequence bigint,
 reported_command_seq integer CHECK (reported_command_seq BETWEEN 0 AND 65535),
 applied_at timestamptz,
 CHECK ((version IS NULL) = (config IS NULL))
);
CREATE TABLE device_commands (
 device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 seq integer NOT NULL CHECK (seq BETWEEN 1 AND 65535),
 kind text NOT NULL CHECK (kind IN ('reboot','factory_reset','capture_now')),
 state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','radiated','applied')),
 created_at timestamptz NOT NULL DEFAULT now(),
 radiated_at timestamptz,
 applied_at timestamptz,
 PRIMARY KEY (device_id, seq)
);
CREATE INDEX device_commands_open_idx ON device_commands(device_id, seq) WHERE state <> 'applied';
-- Wi-Fi passwords inside config are sealed with AES-GCM; the key is outside the database.
CREATE TABLE gateway_configs (
 gateway_id text PRIMARY KEY REFERENCES gateways(id) ON DELETE CASCADE,
 version integer CHECK (version BETWEEN 1 AND 65535),
 config jsonb,
 updated_at timestamptz,
 reported_version integer CHECK (reported_version BETWEEN 0 AND 65535),
 reported_at timestamptz,
 applied_at timestamptz,
 firmware_version text,
 last_sync_at timestamptz,
 CHECK ((version IS NULL) = (config IS NULL))
);
CREATE TABLE waveforms (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 gateway_id text NOT NULL REFERENCES gateways(id),
 gateway_session_id text NOT NULL,
 event_seq bigint NOT NULL CHECK (event_seq BETWEEN 1 AND 4294967295),
 device_id text NOT NULL REFERENCES devices(id),
 ingested_at timestamptz NOT NULL DEFAULT now(),
 estimated_received_at timestamptz,
 queue_delay_ms bigint NOT NULL CHECK (queue_delay_ms >= 0),
 metadata jsonb NOT NULL,
 samples bytea NOT NULL,
 UNIQUE (gateway_id, gateway_session_id, event_seq)
);
CREATE INDEX waveforms_history_idx ON waveforms(device_id,(COALESCE(estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz)) DESC,id DESC);
CREATE TABLE collections (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text NOT NULL,
 device_id text NOT NULL REFERENCES devices(id),
 condition text NOT NULL DEFAULT '',
 notes text NOT NULL DEFAULT '',
 started_at timestamptz NOT NULL,
 ended_at timestamptz,
 CHECK (ended_at IS NULL OR ended_at > started_at)
);
CREATE INDEX collections_recent_idx ON collections(started_at DESC, id DESC);
-- The binary of a replaced firmware is dropped; its row stays as history.
CREATE TABLE gateway_firmware (
 id text PRIMARY KEY,
 gateway_id text NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
 version text NOT NULL,
 sha256 text NOT NULL,
 size bigint NOT NULL CHECK (size > 0),
 data bytea,
 state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','offered','downloaded','applied','failed','superseded')),
 reason text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 offered_at timestamptz,
 downloaded_at timestamptz,
 finished_at timestamptz
);
CREATE INDEX gateway_firmware_recent_idx ON gateway_firmware(gateway_id, created_at DESC);
