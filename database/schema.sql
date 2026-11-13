-- Base schema for goklippy (SQLite; Atlas source of truth).
-- The backend runs this file on startup, so every statement must be safe to repeat
-- (IF NOT EXISTS). Changes to existing tables go through `mage atlasPush`.
-- Timestamps are DATETIME so the driver reads them back as time values.

CREATE TABLE IF NOT EXISTS app_users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS auth_refresh_tokens (
  token_id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  expires_at DATETIME NOT NULL,
  revoked_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auth_refresh_tokens_user_id ON auth_refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_refresh_tokens_expires_at ON auth_refresh_tokens(expires_at);

-- LEGO sorter settings: one JSON document per name ("machine", "routines").
-- JSON keeps tuning changes free of schema changes; the backend owns the shape.
CREATE TABLE IF NOT EXISTS sorter_settings (
  name TEXT PRIMARY KEY,
  data TEXT NOT NULL CHECK (json_valid(data)),
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Sorting profiles in Hive's sorting_profile.json format. `data` is the file as
-- uploaded (it can run to tens of MB); `summary` is what the lists show, so they
-- never parse `data`. At most one profile is active.
CREATE TABLE IF NOT EXISTS sorting_profiles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT 'upload',
  summary TEXT NOT NULL CHECK (json_valid(summary)),
  data TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sorting_profiles_one_active ON sorting_profiles(active) WHERE active;

-- A sort run: from Start to Stop.
CREATE TABLE IF NOT EXISTS sort_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NULL REFERENCES sorting_profiles(id) ON DELETE SET NULL,
  profile_name TEXT NOT NULL DEFAULT '',
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at DATETIME NULL
);

-- Every piece the machine saw. id is a UUID made when the piece first appears.
-- problems is a JSON array of {kind, message, at}; prediction is Brickognize's answer.
CREATE TABLE IF NOT EXISTS pieces (
  id TEXT PRIMARY KEY,
  run_id INTEGER NULL REFERENCES sort_runs(id) ON DELETE SET NULL,
  status TEXT NOT NULL,
  seen_at DATETIME NOT NULL,
  identified_at DATETIME NULL,
  distributed_at DATETIME NULL,
  part_id TEXT NOT NULL DEFAULT '',
  part_name TEXT NOT NULL DEFAULT '',
  part_category TEXT NOT NULL DEFAULT '',
  part_score REAL NOT NULL DEFAULT 0,
  color_id TEXT NOT NULL DEFAULT '',
  color_name TEXT NOT NULL DEFAULT '',
  color_score REAL NOT NULL DEFAULT 0,
  category TEXT NOT NULL DEFAULT '',
  category_name TEXT NOT NULL DEFAULT '',
  bin_layer INTEGER NOT NULL DEFAULT 0,
  bin_number INTEGER NOT NULL DEFAULT 0,
  route_reason TEXT NOT NULL DEFAULT '',
  piece_mm REAL NOT NULL DEFAULT 0,
  problems TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(problems)),
  prediction TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_pieces_run ON pieces(run_id, seen_at);
CREATE INDEX IF NOT EXISTS idx_pieces_seen ON pieces(seen_at);
CREATE INDEX IF NOT EXISTS idx_pieces_part ON pieces(part_id, color_id, seen_at);

-- Photos of a piece, as files under the data folder. used marks the ones sent to
-- Brickognize.
CREATE TABLE IF NOT EXISTS piece_images (
  piece_id TEXT NOT NULL REFERENCES pieces(id) ON DELETE CASCADE,
  idx INTEGER NOT NULL,
  path TEXT NOT NULL,
  used BOOLEAN NOT NULL DEFAULT FALSE,
  taken_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (piece_id, idx)
);

-- A person's check of a piece's identification. part_correct and color_correct are
-- NULL until answered; color_id is the right color when the predicted one was wrong;
-- issues is a JSON array of flags about the photos.
CREATE TABLE IF NOT EXISTS piece_reviews (
  piece_id TEXT PRIMARY KEY REFERENCES pieces(id) ON DELETE CASCADE,
  part_correct BOOLEAN NULL,
  color_correct BOOLEAN NULL,
  color_id TEXT NOT NULL DEFAULT '',
  issues TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(issues)),
  reviewed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The catalog: every part and colour identified, across all runs, with Brickognize's
-- picture of the part. Kept apart from pieces (whose records and photos can be pruned)
-- and cleared on its own. catalog_runs counts each one per run; catalog_counted is the
-- pieces already counted, so filling it from the records never counts one twice.
CREATE TABLE IF NOT EXISTS catalog_parts (
  part_id TEXT NOT NULL,
  color_id TEXT NOT NULL DEFAULT '',
  part_name TEXT NOT NULL DEFAULT '',
  color_name TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT '',
  img_url TEXT NOT NULL DEFAULT '',
  seen INTEGER NOT NULL DEFAULT 0,
  best_score REAL NOT NULL DEFAULT 0,
  first_seen DATETIME NOT NULL,
  last_seen DATETIME NOT NULL,
  PRIMARY KEY (part_id, color_id)
);

CREATE TABLE IF NOT EXISTS catalog_runs (
  part_id TEXT NOT NULL,
  color_id TEXT NOT NULL DEFAULT '',
  run_id INTEGER NOT NULL,
  seen INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (part_id, color_id, run_id)
);

CREATE TABLE IF NOT EXISTS catalog_counted (
  piece_id TEXT PRIMARY KEY
);

-- Problems the machine stopped for, and how each was dealt with.
CREATE TABLE IF NOT EXISTS incidents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  piece_id TEXT NULL,
  opened_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at DATETIME NULL,
  resolution TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_incidents_opened ON incidents(opened_at);

-- Every channel at the moment an incident opened: its camera's picture (a file under the
-- data folder; empty for C1, which has no camera) and its state as JSON (what it was
-- doing, its summary, what its camera saw). Cleared on their own or with the incident.
CREATE TABLE IF NOT EXISTS incident_cameras (
  incident_id INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  channel TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  taken_at DATETIME NOT NULL,
  state TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(state)),
  PRIMARY KEY (incident_id, channel)
);

-- When each bin was last emptied; its count is the pieces delivered since.
CREATE TABLE IF NOT EXISTS bin_empties (
  layer INTEGER NOT NULL,
  bin INTEGER NOT NULL,
  emptied_at DATETIME NOT NULL,
  PRIMARY KEY (layer, bin)
);

CREATE INDEX IF NOT EXISTS idx_pieces_bin ON pieces(bin_layer, bin_number, distributed_at);

-- Which categories each bin takes, per profile, so switching profiles keeps each
-- profile's bins. Layer and bin count from 1; position keeps a bin's categories in
-- the order they were given.
CREATE TABLE IF NOT EXISTS bin_categories (
  profile_id INTEGER NOT NULL REFERENCES sorting_profiles(id) ON DELETE CASCADE,
  layer INTEGER NOT NULL,
  bin INTEGER NOT NULL,
  category_id TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (profile_id, layer, bin, category_id)
);
