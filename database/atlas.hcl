// Atlas config for the SQLite schema. schema.sql is the single source of truth: the
// backend runs it on startup (new tables), and `mage atlasPush` applies changes to
// existing tables. Paths are relative to this folder.
env "local" {
  src = "file://schema.sql"
  url = "sqlite://../private/goklippy.db"
  dev = "sqlite://dev?mode=memory"
}
