CREATE INDEX readings_recent_idx ON readings(ingested_at DESC,id DESC);
