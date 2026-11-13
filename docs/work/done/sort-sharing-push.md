# Push sorting updates to the viewer site

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-13
Finished: 2026-11-13

## Why
The collection's owner wants to watch their LEGO being sorted from home. The sorter runs on
the bench; the [viewer site](sort-viewer-app.md) runs on a server. The sorter has to send it
what's happening.

## What
- In the sorter's settings: the viewer site's URL and an API key (made in the viewer site).
  A test button to check they work.
- Per collection, a switch for whether it's shared to the viewer site.
- While sorting a shared collection, the sorter pushes: run started/stopped, each piece as it's
  identified (part, colour, category, bin, confidence, unknown or not), and one photo: the
  last one taken of it on C4. The official part picture (the catalog/Brickognize image) goes with it, so viewers
  see our photo next to the reference one.
- The collection's sorting plan (bins and their categories) is pushed when sharing starts and
  again whenever it changes.
- Corrections made later on the sorter (a review fixing the part or colour) are pushed too,
  and replace what the site shows.
- If the site can't be reached, updates queue up and are sent when it's back; sorting never
  waits on the site.
- Shows on the sorter whether sharing is working and how far behind it is.

## Done when
- [x] URL and API key can be set and tested from the sorter's settings.
- [x] Shared collections push run and piece updates, with the piece's last C4 photo and the
      official part picture, while sorting.
- [x] Turning sharing on for a collection sends everything already sorted into it.
- [x] Plan changes, review corrections and cleared unknowns are synced to the site.
- [x] Sorting carries on at full speed with the site down, and catches up after.
- [x] Sorter shows sharing status (connected, queued count, last error).

## Plan

Sends to the site's push API as documented in `services/sortviewer/README.md` (Push API).

**The outbox** (the heart of it). A table `share_outbox (kind, ref, queued_at, attempts,
last_error)`, unique on `(kind, ref)`: kinds `collection`, `run`, `piece`, `photo`,
`clear_unknowns`, with `ref` the id. Something changing just marks "send this again"; the
sender reads the *current* record from the database when it sends, so ten changes to a piece
become one send, and a backfill is just marking everything. Kept in the database, so a restart
or a long outage loses nothing.

**Sender** (`logic/sorter/sharing.go`, its own goroutine, never on the control loop): takes the
oldest marks in order (collections and runs before their pieces, a piece before its photo),
sends each, deletes it on success. On failure it notes the error and backs off (2 s doubling to
1 min); a 4xx other than 404/408/429 (bad data) is dropped after 5 tries so one bad record can't
block the rest. Sorting never waits on it.

**What marks what** (hooks, all after the record is saved):
- Collection shared on: marks the collection (name + plan), all its runs, pieces and photos
  (the backfill). Shared off: drops its marks. Rename: the collection.
- Run start/stop (`machine_sort.go`): the run.
- Piece saved (`Records.SavePiece` job) and review saved: the piece; photo saved: the photo.
- Plan changed (a bin gets a category, Bins page edits, profile change, collection picked):
  the collection, via a callback from the sorting service.
- Clear unknowns: `clear_unknowns` for the collection, and its unknown pieces' marks dropped.
Only collections switched to shared are marked; everything else is a no-op.

**What's sent**
- Piece: the fields the API takes; `part_img_url` is the official picture (from the
  Brickognize prediction). A review is applied: a wrong colour with the right one given sends
  the right colour; a wrong part (or "not LEGO"/"no piece") sends it as unknown.
- Photo: the piece's last photo (the newest C4 one), shrunk on the sorter to 800 px JPEG
  before upload (the site keeps 800 px anyway; saves upload bandwidth).
- Plan: `{profile: {name}, bins: [{layer, bin, categories}], bins_started_at}` from the
  collection's plan.

**Data**: `collections.shared BOOLEAN`, `share_outbox`, and a `sharing` settings document
(`url`, `api_key`) in `sorter_settings`. The key is never sent back to the browser (only
whether one is set).

**Routes**: `GET /sorter/sharing` (URL, key set?, status: connected, queued, last error, last
sent), `PUT /sorter/sharing` `{url, api_key}`, `POST /sorter/sharing/test` (the site's
`/api/push/ping`), `PATCH /sorter/collections/:id` gains `shared`.

**UI**: Run › Collections gets a **Sharing** panel (site address, key, Test, and the status
line: "Up to date" / "12 waiting" / the last error), and each collection a **Share to
lego.n8p.xyz** switch with a link to its page there.

**Tests**: models (outbox marks collapse, shared-only marking, ordering), sender against a fake
site (sends, retries after a failure, drops bad data, backfill on share), review applied to
the sent piece, photo shrunk. Then a simulator sort pushing to a locally run sort viewer.

**Questions**: none blocking. Assumed: a reviewed "wrong part" shows as unknown on the site.

**As built** (2026-11-13): `50e2263` (backend), `fc0d164` (UI). Two changes from the plan:
- A record the site refuses as bad (a 4xx) is dropped at once and logged, not after 5 tries:
  retrying can't make it valid, and backing off on it held up everything behind it.
- The hooks wrap the record and sorting stores at wiring (`backend/sharing_wiring.go`)
  instead of editing each call site, so every save that matters marks the outbox.
- Sorter database: `collections.shared` added (backup `private/backups/goklippy-before-sharing-20261113.db`).

**Tested**: Go tests for the outbox and sender (shared-only marking, backfill order, site
down then catching up, refused records dropped, reviews applied, unknowns cleared, unsharing,
photo shrunk to 800 px, settings keep the key); whole backend suite passes. End to end: a
simulated sort sharing to a locally run sort viewer: 6 pieces, plan and bin counts (Tile 3,
Plate 1, Brick 1), 6 photos and the run's end all matched on the site. Nate looked it over
and called it done (2026-11-13). Official part pictures weren't seen end to end (the
simulator has none); real Brickognize identifications carry them.

## Notes
- Depends on [sorter-collections](sorter-collections.md). The API itself is defined in
  [sort-viewer-app](sort-viewer-app.md); plan the two together.
- 2026-11-13, Nate: one photo per piece, the one that looks best, plus the official part
  picture; plan changes and review corrections are synced.
- 2026-11-13, Nate: the photo is the last one taken on C4. Pieces sorted before sharing was
  turned on are backfilled through the same queue.
- To decide when planned: what size to send the photo at.
