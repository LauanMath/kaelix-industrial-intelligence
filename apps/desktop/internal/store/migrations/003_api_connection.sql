CREATE TABLE api_connection (
 id INTEGER PRIMARY KEY CHECK(id=1),
 endpoint TEXT NOT NULL DEFAULT '',
 email TEXT NOT NULL DEFAULT '',
 token TEXT NOT NULL DEFAULT '',
 expires_at TEXT NOT NULL DEFAULT ''
);
INSERT INTO api_connection(id) VALUES(1);
PRAGMA user_version=3;
