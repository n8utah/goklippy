# Sorter collections

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-13

## Why
We sort LEGO for other people (first up: a friend's collection) and hand it back sorted.
Right now every piece goes into one shared pile of records, so there's no way to say
"these pieces, counts and unknowns belong to this person's collection".

## What
- A collection is a named set of LEGO being sorted, e.g. "Sam's collection". Create, rename
  and archive them.
- When starting a sort, pick which collection the pieces go into. Every piece seen in that
  run, identified or not, belongs to it.
- Pieces, counts and unknowns are tracked per collection, and add up across several runs into
  the same collection.
- The pieces page (and counts/catalog views) can be filtered by collection.
- A collection has a sorting plan (the profile's bins and which categories go in each), so we
  can say what groups it will come back in. The plan can change partway through; the
  collection always shows its current plan, and a change is synced to the viewer site.
- Unknowns can be cleared from a collection. The unknown bin usually gets fed back through for
  another try; clearing its unknowns first means the re-run doesn't count them twice.
- Existing pieces and runs are test data and get cleared out, so nothing exists without a
  collection.

## Done when
- [ ] Collections can be created, renamed and archived.
- [ ] Starting a sort requires a chosen collection, shown while sorting; it can't change
      mid-run (stop and start a new run instead).
- [ ] Each piece records its collection; existing test pieces, runs and photos are cleared.
- [ ] Pieces page filters by collection; counts and unknowns show per collection.
- [ ] A collection's unknowns can be cleared before re-running the unknown bin.
- [ ] A collection shows its current sorting plan (bins and their categories), and the plan
      can be changed partway through.
- [ ] With the chute left out and a profile active, each piece still gets the bin it would
      have gone to, marked virtual, so layouts and plans can be tested without the chute.

## Plan

**Data** (`database/schema.sql`, applied with `mage atlasPush` after the clear-out below)
- New `collections (id, name UNIQUE, archived_at NULL, created_at)`.
- `sort_runs.collection_id INTEGER NOT NULL REFERENCES collections(id)`. Pieces stay linked to
  their run, so a piece's collection is its run's; no column on `pieces`.
- `bin_categories` gains `collection_id` in its key: `(collection_id, profile_id, layer, bin,
  category_id)`. Bins fill up as categories arrive, so today a second collection sorted with
  the same profile would inherit the first one's bin layout. Keying by collection gives each
  collection its own layout (its sorting plan), and switching profile partway through keeps
  each profile's layout for that collection.
- A collection's **current plan** = the profile of its latest run + that collection/profile's
  `bin_categories`.
- **Unknown** = a piece in the collection with no part (`part_id = ''`): unidentified, ID
  failed, or lost before identification.

**Clearing the test data** (one-off, on the sorter's `private/goklippy.db`, after a backup):
delete runs, pieces, piece photos (files too), reviews, catalog counts, bin assignments and
bin empties. Incidents are kept. Done by hand once, not shipped as code.

**Backend**
- `models/collections_store.go`: create, rename, archive/unarchive, list with counts
  (runs, pieces, identified, unknowns), get one, clear unknowns (deletes those pieces, their
  reviews, and returns photo paths so logic removes the files).
- `models/records_store.go`: `StartRun` takes the collection; `ListRuns` returns it;
  `PieceFilter` and catalog filters gain `CollectionID` (join through `sort_runs`); `Stats`,
  `KitPieces` take an optional collection.
- `models/sorting_store.go`: `BinCategories`/`ReplaceBinCategories`/`SetBinCategories` take the
  collection as well as the profile.
- `logic/sorter`: Start needs a collection id (`ErrNoCollection` without one, archived ones
  refused); the running run's collection is kept on the sort state and loaded with its own bin
  layout; the snapshot carries `collection {id, name}`. Clearing a collection's unknowns is
  refused while that collection is being sorted.
- `routes/sorter_collection_routes.go`: `GET/POST /sorter/collections`,
  `GET/PATCH /sorter/collections/:id` (rename, archive), `GET /sorter/collections/:id/plan`,
  `POST /sorter/collections/:id/clear-unknowns`. `POST /sorter/commands` takes
  `collection_id` with `start`. `?collection=` on pieces, runs, catalog and stats.

**Frontend**
- New `CollectionsPage`: list (name, runs, pieces, unknowns, last sorted), create, rename,
  archive; a collection's detail shows counts, unknowns (with photos, and Clear unknowns), and
  its current plan (each bin and its categories, with how many pieces landed there).
- `SortPage`: a collection picker beside Start (remembers the last one; Start disabled until
  one is picked); the running collection's name shown while sorting, picker locked.
- `PiecesPage`: collection filter on both tabs, alongside run (runs list limited to that
  collection); exports name the collection.
- `RecordsPage` runs list and `BinsPage` show / follow the collection.

**Tests**: models tests for collections, filtering, unknown clearing and per-collection bin
layouts; sorter tests that Start needs a collection and loads its layout; update
`sort_e2e_test.go`. Then try it on the bench with the simulator: two collections, a profile
switch, clear unknowns.

**Switching collections** (Nate, 2026-11-13): starting a run for a different collection than
the last run's means the bins were emptied and we start over; nothing is added to bins left
from another collection. So on Start, when the collection differs from the previous run's:
every bin is marked emptied (`bin_empties`), and the collection's bin layout for that profile
is cleared, so it builds a fresh one. Consecutive runs of the same collection carry on with
its layout. Bins page counts stay machine-wide (they describe what's physically in the bins).

## Notes
- Needed before [sort-sharing-push](sort-sharing-push.md) and [sort-viewer-app](sort-viewer-app.md).
- Today: `sort_runs` → `pieces`; the plan is `sorting_profiles` + `bin_categories`. A collection
  likely hangs off `sort_runs` (a run belongs to one collection), with pieces reached through
  their run. To decide when planned.
- 2026-11-13, Nate: the plan can change during a collection's sort; the latest plan is the
  one shown and synced. Existing pieces/runs were all test data and can be cleared.
- 2026-11-13, Nate: the collection is set per run. To switch collections, stop and start a
  new run.
- 2026-11-13, Nate: with the chute left out, still say which bin each piece would have gone to
  (a virtual bin), to help test everything else.
- 2026-11-13, progress: built in `1b3f314` (backend) and `ac3b02d` (UI). Switching is done
  by picking the collection (Run › Sort or Run › Collections), not on Start, so a picked
  collection's bins can be preassigned before starting. Backend tests pass, including the
  chute-left-out virtual bins; tried against the simulator's API. Not yet looked at in a browser.
- 2026-11-13, sorter database: backed up to `private/backups/goklippy-before-collections-20261113.db`,
  test runs/pieces/photos/catalog/bin assignments cleared (incidents kept), schema applied.
  `mage atlasPush` currently fails on SQLite's built-in `app_users` email index (Atlas tries to
  drop `app_users_email`), so the change was applied from Atlas's own plan without the
  index lines.
- Left: look the pages over on the real machine, then close with `/task-done`.
