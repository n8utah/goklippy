# Built-in sorting profiles

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-13
Finished: 2026-11-13

## Why
The only way to get a sorting profile today is to build a `sorting_profile.json` in Hive and
upload it (tens of MB). The sorter has none stored, so there's nothing to sort by without a trip
to Hive first. Most sorts just want "plates here, bricks there" or "by colour", which needs no
part lists at all: Brickognize already returns each part's BrickLink category and we store it on
every piece (`pieces.part_category`).

Separately, the active profile is machine-wide, so a collection just sorts by whatever profile
happened to be active. Now that a collection has a sorting plan, it should say which profile it
uses.

## What
- Two built-in profile types, created from Rules › Profiles with no file:
  - **By part type**: the category is Brickognize's BrickLink category grouped by the part
    before the comma ("Plate", "Plate, Round" and "Plate, Modified" all go to **Plate**; likewise
    Brick, Slope, Tile, Wedge, Technic, Wheel, ...). Today's catalog of ~25 categories comes to
    about a dozen groups.
  - **By colour**: one category per colour.
- Pieces with no part type (or no colour, for the colour profile) go to misc, as they do now.
- Fine-tuning stays on the Bins page: a bin can already take several categories (e.g. Tile +
  Wedge in one bin).
- Uploading Hive profiles keeps working as it does today.
- A collection picks its profile. A run can't start without one: starting a run sorts by the
  collection's profile. There's no machine-wide "active" profile any more; anything that used it
  (e.g. the "where would this go?" dry run) uses a collection's profile instead.
- Changing a collection's profile partway through asks whether the bins were emptied:
  - **Emptied:** every bin is marked emptied and the new profile starts a fresh bin layout.
  - **Not emptied:** the bins already holding pieces are left alone; the new profile only gets
    the unused bins, and sorting continues.

## Done when
- [x] "By part type" and "By colour" are always on the Profiles page, with nothing to upload,
      alongside uploaded ones.
- [x] Sorting with them routes each piece by its Brickognize part type (grouped) or its colour;
      pieces without one go to misc.
- [x] Bins can be given these categories on the Bins page, and pre-assigned from them.
- [x] A collection has a profile, chosen when it's created and changeable later.
- [x] Starting a run requires the collection to have a profile, and the run sorts by it.
- [x] The machine-wide active profile is gone (no Activate, no `active` column); the Bins page,
      dry run, plan and home page follow the current collection's profile.
- [x] Changing the current collection's profile asks "bins emptied, or use the unused bins and
      continue?" and does that; bins holding pieces get no new categories in the second case.
- [x] A profile a collection uses can't be deleted.
- [x] Hive uploads still import and route as before.

## Plan

**Model.** The profile moves from the machine onto the collection. The sorter's "active
profile" becomes the current collection's profile: picking a collection, or changing its
profile, loads that profile. Bin layouts stay keyed by (collection, profile), as now.

**Built-in profiles** (`logic/sorter/profile/builtin.go`)
- Two fixed kinds, stored as small rows in `sorting_profiles` with `source = 'builtin'` and
  `data = {"builtin": "part_type"}` / `{"builtin": "color"}`. The backend makes sure both rows
  exist at startup; they can't be deleted. `Parse` recognises the `builtin` key.
- Routing gets the whole identification, not just part and colour:
  `Route(Piece{Part, Color, ColorName, PartCategory}, kitFull)`. Hive profiles ignore the
  extra fields; built-ins use them.
- **By part type:** category = Brickognize's category up to the first comma ("Plate, Round" →
  `Plate`); the id is that text. A built-in list of the usual families (Brick, Plate, Tile,
  Slope, Wedge, Technic, Wheel, Hinge, Bracket, Arch, Panel, Window, Door, Minifigure, Plant,
  ...) is the profile's category list, so bins can be given or pre-assigned them before any
  piece arrives; a family not on the list still works and is listed once a bin has it.
- **By colour:** category `color_<id>`, named from Brickognize's colour name. Names already
  seen are read back from the catalog at load, so the Bins page shows "Red", not "Color 5"
  (Hive's sort-the-rest-by-colour gets the same names).
- No part type / no colour → misc, as now.

**Data** (`database/schema.sql`)
- `collections.profile_id INTEGER NULL REFERENCES sorting_profiles(id)`: NULL only for
  collections made before this; new ones must have one. Existing collections get the
  currently active profile when the change is applied.
- Drop `sorting_profiles.active` and its one-active index.
- Applied by hand on the sorter after a backup, the way sorter-collections was
  (`mage atlasPush` trips over the `app_users` email index).

**Backend**
- `models/sorting_store.go`: drop `ActivateProfile`/`ActiveProfileID`; `EnsureBuiltinProfiles`;
  `ListProfiles` returns which collections use each profile; `DeleteProfile` refuses one in use.
  `models/collections_store.go`: `CreateCollection` takes a profile; `SetCollectionProfile`;
  the collection's `profile_id`/name in `GetCollection`/`ListCollections`; colour names from
  `catalog_parts`.
- `logic/sorter/sorting.go`: `NewSortingService` and `SelectCollection` load the current
  collection's profile; `Activate` goes; new `ChangeProfile(collection, profile, binsEmptied)`:
  refused while a run is sorting into that collection (stop first, as with switching
  collections). If it's the current collection: emptied → every bin marked emptied and a fresh
  layout; not emptied → fresh layout for the new profile, and bins still holding pieces stay
  out of it. For any other collection it just records the profile (its bins start afresh
  when it's picked anyway). `PreassignBins` moves here from activation (skips bins holding
  pieces).
- `logic/sorter/bins.go` `ChooseBin`: a bin with pieces in it and no category under the current
  profile isn't free. That's the "use the unused bins" rule, and it also stops a bin
  you've stripped by hand from taking a new category before it's emptied.
- `logic/sorter/machine_sort.go`: Start without a profile → "this collection has no sorting
  profile; pick one on Run › Collections".
- `collections.go` `Plan`: uses the collection's profile, not its latest run's.
- Callers of `Route` pass the part category and colour name (`platter.go`, `identify.go`, the
  `/sorter/route` dry run).
- Routes: `POST /sorter/profiles/:id/activate` goes; `POST /sorter/collections` takes
  `profile_id` (required); `PATCH /sorter/collections/:id` takes
  `{profile_id, bins: "emptied" | "continue"}`; `POST /sorter/bins/preassign`.

**Frontend**
- `ProfilesPage`: built-ins first ("Built-in" badge, no delete), then uploads; each shows the
  collections using it. Activate and the keep/reset/pre-assign menu go.
- `CollectionsPage`: a profile picker in the create dialog (required); the detail shows the
  profile with **Change**. For the current collection the change dialog asks "I emptied the
  bins" / "Keep what's in the bins, use the unused ones and carry on".
- `BinsPage`: a **Pre-assign from the profile** button beside Reset.
- `SortPage`: Start disabled, with a link, when the picked collection has no profile.
- `helpers/api.js`: drop `activateSortingProfile`; add the collection profile change and
  pre-assign calls.

**Tests**: profile package table tests for both built-ins (families, commas, no category, no
colour); sorting tests for loading the collection's profile, change with emptied/continue
(occupied bins skipped), Start without a profile, deleting a profile in use; models tests for
the new columns; update `sort_e2e_test.go`. Then on the simulator (its parts carry
categories): sort into a collection by part type, switch it to colour with and without
emptying, check the Bins and Plan pages in a browser.

**Overlap with other work**: sort-viewer-app is all in `services/sortviewer/`, nothing shared.
sorter-collections is built and only waiting on a look in the browser; this changes its
files (`collections.go`, `collections_store.go`, `CollectionsPage`), so I'll pull before
starting and commit in small steps.

## Notes
- 2026-11-13, done: built-in By part type / By colour profiles, each collection sorts by its own profile (no machine-wide active profile), profile change asks emptied or continue, pre-assign on the Bins page. Commits 24f1543 (backend), 8284d34 (UI); sorter database migrated by hand. Closed by Nate.
- Builds on [sorter-collections](../doing/sorter-collections.md) (collections and their
  per-collection bin layouts).
- The plan already listed "local profiles without Hive" as not done yet
  (`docs/lego-sorter-server-plan.md`, phase 6).
- Routing today takes only part and colour (`profile.Route`); by part type also needs the
  piece's Brickognize category.
- Out of scope for now: exact (ungrouped) BrickLink category, part type + colour combined, a
  rule editor, Hive sync.
- 2026-11-13, Nate: plan approved. Built-ins are always there (one of each, can't be deleted);
  a profile change needs the run stopped first; changing a non-current collection's profile
  doesn't ask about the bins.
- 2026-11-13, progress: built in `24f1543` (backend) and `8284d34` (UI). Go tests pass; tried
  on a simulated backend against a copy of the sorter's database (part-type sort, change to
  colour keeping full bins, change back with bins emptied). Left: back up and migrate the
  sorter's database (adds `collections.profile_id`, drops `sorting_profiles.active`; existing
  collections get the active profile), restart the backend, and look the pages over.
- 2026-11-13, sorter database: backed up to
  `private/backups/goklippy-before-builtin-profiles-20261113-143417.db`, then migrated by hand
  (add `collections.profile_id`, drop `sorting_profiles.active` and its index; no collections
  or profiles existed yet). Backend restarted; both built-ins are there. Left: look the pages
  over, then `/task-done`.
- 2026-11-13, Nate: no machine-wide active profile once collections choose their own.
- 2026-11-13, Nate: changing a collection's profile asks whether the bins were emptied; if not,
  the new profile uses only the unused bins and the sort continues.
