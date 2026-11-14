-- Set catalog schema (SQLite): every LEGO set's parts, from Rebrickable's public
-- downloads. It lives in its own file (sets.db) next to the site's database, is built
-- whole into a new file and swapped in, and is only read otherwise. Ids are
-- Rebrickable's for colors and themes; parts and sets get ids while building.

-- What was built, when: built_at (RFC 3339), sets, source.
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- name_key is the name folded for matching BrickLink's colour names (which are what the
-- sorter has): lower case, letters and digits only.
CREATE TABLE IF NOT EXISTS colors (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  name_key TEXT NOT NULL,
  rgb TEXT NOT NULL DEFAULT '',
  is_trans BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_colors_name_key ON colors(name_key);

CREATE TABLE IF NOT EXISTS themes (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  parent_id INTEGER NULL
);

-- canon_id is the part that stands for every mold variant and alternate of this one
-- (itself if it has none); set_parts and images use it.
CREATE TABLE IF NOT EXISTS parts (
  id INTEGER PRIMARY KEY,
  part_num TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  category_id INTEGER NOT NULL DEFAULT 0,
  canon_id INTEGER NOT NULL
);

-- need is how many parts the set takes as counted here (no spares, stickers or
-- minifigures), so it can be lower than Rebrickable's num_parts.
CREATE TABLE IF NOT EXISTS sets (
  id INTEGER PRIMARY KEY,
  set_num TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  year INTEGER NOT NULL DEFAULT 0,
  theme_id INTEGER NOT NULL DEFAULT 0,
  img_url TEXT NOT NULL DEFAULT '',
  need INTEGER NOT NULL DEFAULT 0
);

-- A set's parts: its latest inventory, merged per canonical part and colour.
CREATE TABLE IF NOT EXISTS set_parts (
  set_id INTEGER NOT NULL,
  part_id INTEGER NOT NULL,
  color_id INTEGER NOT NULL,
  qty INTEGER NOT NULL,
  PRIMARY KEY (set_id, part_id, color_id)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS idx_set_parts_part ON set_parts(part_id, color_id);

-- set_parts summed over colours, for matching in any colour: filled from set_parts
-- when the catalog is finished.
CREATE TABLE IF NOT EXISTS set_shapes (
  set_id INTEGER NOT NULL,
  part_id INTEGER NOT NULL,
  qty INTEGER NOT NULL,
  PRIMARY KEY (set_id, part_id)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS idx_set_shapes_part ON set_shapes(part_id);

-- A picture of a canonical part in a colour (Rebrickable's).
CREATE TABLE IF NOT EXISTS images (
  part_id INTEGER NOT NULL,
  color_id INTEGER NOT NULL,
  img_url TEXT NOT NULL,
  PRIMARY KEY (part_id, color_id)
) WITHOUT ROWID;
