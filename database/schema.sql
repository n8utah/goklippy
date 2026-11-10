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
