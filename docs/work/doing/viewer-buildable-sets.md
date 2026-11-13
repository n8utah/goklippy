# Buildable sets on the viewer site

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-13

## Why
Once a collection is sorted, the obvious next question from whoever owns it is "what can I
build with this?". The viewer site already has every identified piece, by part and colour.

## What
- A **Sets** tab on each collection: LEGO sets ranked by how much of each the collection
  covers, as % and "missing N", with the right colours and, separately, any colour.
- Opening a set shows its parts: needed, have, missing, with pictures.
- Filters: search (name, number, theme) and a minimum set size so 3-piece polybags don't
  crowd the top.
- Set data comes from Rebrickable's public downloads (no API key), kept in its own SQLite
  file next to the site's database and refreshed by the admin (and monthly by itself).

## Done when
- [ ] The site downloads Rebrickable's set data on first start and builds `sets.db` without
      blocking requests; the admin can refresh it and sees when it was last built.
- [ ] `GET /api/collections/:id/sets` ranks sets for a collection (same access rules as the
      other collection views); `GET /api/collections/:id/sets/:set` lists a set's parts with
      have/need.
- [ ] Mold variants and alternates (Rebrickable `M`/`A` relationships) count as the same part;
      prints don't. Spare parts, stickers and minifigures aren't counted.
- [ ] BrickLink colours (what Brickognize gives) map to Rebrickable colours by name.
- [ ] The Sets tab and set detail work on a phone and in both themes.
- [ ] Go tests cover the import (from small CSV fixtures) and the matching.

## Plan
- `database/catalog.sql`: schema of the separate set database (colors, themes, parts with a
  canonical part per mold/alternate group, sets with total parts needed, set_parts aggregated
  per canonical part and colour, part images).
- `backend/models/catalog.go`: open (read-only use), a builder that writes a new file, and
  the ranking / set-detail queries. The collection's counts are passed in as JSON
  (`json_each`), so the two databases never need attaching.
- `backend/logic/sets.go`: matching (collection counts → catalog queries), the catalog's
  lifecycle (build into `sets.db.new`, swap under a lock), monthly refresh.
- `backend/logic/setimport.go`: downloads and parses Rebrickable's CSVs.
- Routes: viewer `GET .../sets`, `GET .../sets/:set`; admin `GET /api/admin/sets`,
  `POST /api/admin/sets/refresh`.
- Frontend: `Sets` tab in CollectionPage (`components/SetsView.js`, `SetDetail.js`), a
  "Set data" panel in Admin.

## Notes
- Part ids: Brickognize gives BrickLink part numbers, which match Rebrickable's for nearly all
  unprinted parts. Ones that don't are reported as "not in the set data" for now; a
  Rebrickable API key could map them later (`/lego/parts/?bricklink_id=`).
