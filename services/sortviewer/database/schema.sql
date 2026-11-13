-- Sort viewer schema (SQLite). The server runs this file on every start, so every
-- statement must be safe to repeat (IF NOT EXISTS). Timestamps are DATETIME, UTC.
--
-- Ids of collections, runs and pieces are the sorter's own, so its pushes are
-- idempotent: sending the same thing twice changes nothing.

-- A collection shared from the sorter: one person's LEGO. plan is the sorter's
-- current sorting plan as JSON ({profile, bins, bucket}); it's replaced whole.
CREATE TABLE IF NOT EXISTS collections (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  plan TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(plan)),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A sort run: Start to Stop on the sorter, into one collection.
CREATE TABLE IF NOT EXISTS runs (
  id INTEGER PRIMARY KEY,
  collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  profile_name TEXT NOT NULL DEFAULT '',
  started_at DATETIME NOT NULL,
  ended_at DATETIME NULL
);

CREATE INDEX IF NOT EXISTS idx_runs_collection ON runs(collection_id, started_at);

-- A piece the sorter saw. part_id is '' for an unknown (no part found). photo is
-- the file under the data folder (the sorter's last C4 photo, shrunk), '' if none.
CREATE TABLE IF NOT EXISTS pieces (
  id TEXT PRIMARY KEY,
  collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  run_id INTEGER NULL,
  status TEXT NOT NULL DEFAULT '',
  seen_at DATETIME NOT NULL,
  sorted_at DATETIME NULL,
  part_id TEXT NOT NULL DEFAULT '',
  part_name TEXT NOT NULL DEFAULT '',
  part_category TEXT NOT NULL DEFAULT '',
  part_score REAL NOT NULL DEFAULT 0,
  part_img_url TEXT NOT NULL DEFAULT '',
  color_id TEXT NOT NULL DEFAULT '',
  color_name TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT '',
  category_name TEXT NOT NULL DEFAULT '',
  bin_layer INTEGER NOT NULL DEFAULT 0,
  bin_number INTEGER NOT NULL DEFAULT 0,
  virtual_bin BOOLEAN NOT NULL DEFAULT FALSE,
  photo TEXT NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pieces_collection ON pieces(collection_id, seen_at);
CREATE INDEX IF NOT EXISTS idx_pieces_part ON pieces(collection_id, part_id, color_id);

-- People who watch: signed in with email and a password the admin set.
CREATE TABLE IF NOT EXISTS viewers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE COLLATE NOCASE,
  name TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Which collections each viewer may see.
CREATE TABLE IF NOT EXISTS viewer_collections (
  viewer_id INTEGER NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
  collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  PRIMARY KEY (viewer_id, collection_id)
);

-- Signed-in browsers. token_hash is SHA-256 of the cookie's token. A viewer session
-- has viewer_id; an admin session (signed in through Gangway) has admin_email.
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  viewer_id INTEGER NULL REFERENCES viewers(id) ON DELETE CASCADE,
  admin_email TEXT NOT NULL DEFAULT '',
  admin_name TEXT NOT NULL DEFAULT '',
  oidc_sid TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

-- Keys the sorter pushes with. Only SHA-256 of the key is kept; the key is shown once.
CREATE TABLE IF NOT EXISTS api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  prefix TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_used_at DATETIME NULL,
  revoked_at DATETIME NULL
);
