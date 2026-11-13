# Sort viewer site

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-13
Finished: 2026-11-13

## Why
When sorting a friend's LEGO, they should be able to watch it live and see what they'll get
back: what's been found, how many, pictures, and how it's being grouped.

## What
A separate web app on our own server (not part of the bench sorter):
- Lives in this repo under `services/` (e.g. `services/sortviewer/`) with its own `go.mod`
  and a mage deploy script that pushes it to Harbor Keeper, like our other projects.
- Called `sortviewer` (`services/sortviewer/`), live at `lego.n8p.xyz`.
- Go backend, React frontend, SQLite.
- Two kinds of sign-in:
  - **Viewers** (the collection's owner, friends) sign in with a username and password that
    the admin creates for them on the site. No Harbor Keeper account needed.
  - **Admin** (Nate) signs in through Harbor Keeper's identity (Gangway OIDC).
- Receives updates from the sorter over an API authenticated with an API key. Keys are made
  and revoked in the site's admin.
- For each shared collection, the people allowed to see it get:
  - **Live view:** the sort in progress, pieces as they're found, with photos, updating
    without a reload.
  - **Counts:** pieces found, by part, colour and category. Unknowns are listed separately,
    each with its C4 photo and an "unknown" tag; they leave the list when the sorter clears
    them to re-run them.
  - **Pieces:** browse and search what's been found, with pictures.
  - **Sorting plan:** the groups their collection is being sorted into (each bin and its
    categories) and what's landed in each so far, i.e. what they'll be handed back.
- Viewers can only look; there's nothing for them to change.
- Admin (Nate): collections, viewer accounts (create, reset password, remove) and which
  collections each can see, API keys, and clearing out old photos by hand.

## Done when
- [ ] Deployed on Harbor Keeper at `lego.n8p.xyz`; admin signs in through Gangway.
      (Deployed and live; the Gangway sign-in itself is unconfirmed, see Notes.)
- [x] Admin can create viewer accounts; viewers sign in with email and password.
- [x] Sorter push API accepts runs, pieces, photos and plans with a valid key, rejects without.
- [x] Viewers see only collections they've been given.
- [ ] Live view updates while the sorter runs. (Events stream as pieces are pushed; the page
      updating hasn't been watched in a browser.)
- [ ] Counts, unknowns, pieces with photos and the sorting plan are viewable per collection.
      (The API serves them all; the pages haven't been looked at in a browser.)
- [x] Admin can clear a collection's photos (or the whole collection) by hand.

## Plan

Everything new lives in `services/sortviewer/`; nothing outside it changes (another agent is
working in the sorter at the same time).

**Layout** (the goklippy conventions, own module `goklippy/services/sortviewer`)
- `backend/`: Go + Gin, `routes/` · `logic/` · `models/` split, `log/slog`, testify tests.
  One binary that embeds the built frontend and serves UI and API on one origin.
- `frontend/`: Vite + React, Tailwind v4 + shadcn, the same look as goklippy (dark, orange
  accent), `PageName.js` / `.logic.js` pages. Phone-first: friends will mostly watch on phones.
- `database/schema.sql`: SQLite (`modernc.org/sqlite` + `sqlx`), `IF NOT EXISTS`, applied on
  start. Database and photos live in `DATA_DIR` (Dry Dock app data).
- `mage.go` (own magefile, run from `services/sortviewer/`): `Dev`, `Test`, `Build` (frontend,
  embed, linux/amd64 binary), `Deploy` (`keeper push SortViewer production -file … -activate`,
  then checks `https://lego.n8p.xyz/api/health` reports the deployed commit), like
  SimpleBudget and Webcutter.

**Data**: `collections` (keyed by the sorter's collection id, name, plan JSON, current run,
updated), `runs`, `pieces` (keyed by the sorter's piece id: part, colour, category, bin,
virtual, unknown, scores, official image URL, photo path, seen/sorted times), `viewers`
(username, bcrypt hash), `viewer_collections`, `api_keys` (name, SHA-256 of the key, last
used, revoked), `sessions`.

**Sorter push API** (`/api/push/...`, `Authorization: Bearer <key>`). Every call is an
idempotent upsert keyed by the sorter's own ids, so the sorter's queue can retry and backfill
safely; sort-sharing-push builds against this.
- `PUT /api/push/collections/:id` `{name, plan}`: create/rename, replace its plan.
- `PUT /api/push/runs/:id` `{collection_id, started_at, ended_at}`.
- `PUT /api/push/pieces/:id` the piece record (also how review corrections arrive).
- `PUT /api/push/pieces/:id/photo` JPEG body (the last C4 photo); shrunk on arrival.
- `POST /api/push/collections/:id/clear-unknowns`: drops its unknown pieces and photos.
- `GET /api/push/ping`: checks the URL and key (the sorter's "Test" button).

**Sign-in**
- Viewers: username + password made by the admin; an HttpOnly, Secure, SameSite=Lax session
  cookie. Login attempts are rate-limited.
- Admin: Gangway OIDC (code flow + PKCE, client secret, `openid email profile`), at
  `/auth/login` → `/auth/callback`. Who may be admin = Gangway users with access to the app,
  narrowed by an `ADMIN_EMAILS` setting. Admin also sees every collection.
- Settings (keeper vars): `APP_URL`, `OIDC_ISSUER` (`https://login.n8p.xyz`), `OIDC_CLIENT_ID`,
  `OIDC_CLIENT_SECRET`, `ADMIN_EMAILS`, `DATA_DIR`.

**Pages**
- Viewer: their collections → a collection with tabs **Live** (now sorting?, counts ticking,
  newest pieces with photo + official picture, updating over server-sent events), **Plan**
  (each bin and what it takes, pieces in it so far), **Parts** (by part and colour, with counts;
  filter by colour/category), **Unknowns**.
- Admin: collections (who can see each; clear photos; delete), viewers (create, reset
  password, remove, pick collections), API keys (make one, shown once; revoke).

**Tests**: push API (key checks, idempotent upserts, clear unknowns), access (a viewer can't
read another's collection; push keys can't read; viewers can't push), sessions, OIDC callback
with a fake provider. Then run it locally and push sample data with curl.

**Needs Nate (Harbor Keeper, can't be done from here)**
- Make the Dry Dock app `SortViewer` (environment `production`) and the Lighthouse route for
  `lego.n8p.xyz`; or say so and `mage keeperSetup` does it with the keeper CLI.
- Register the Gangway OIDC client (redirect `https://lego.n8p.xyz/auth/callback`) and give
  me the client id; set the secret with `keeper vars set ... -secret`.

**Decided** (Nate, 2026-11-13): viewers sign in with their email and a password; the app stays
running (no idle stop). Claude sets up the Dry Dock app, the route and the Gangway OIDC client
with the keeper CLI (`mage keeperSetup`).

## Notes
- Works with [sort-sharing-push](sort-sharing-push.md); needs [sorter-collections](sorter-collections.md).
- Harbor Keeper deploy and Gangway sign-in: see how `charmingcharlees` does it (its `AGENTS.md`,
  Deploy section), and the `percy-hosted-app` / `fav-newproj` setups.
- 2026-11-13, Nate: code lives in this repo under `services/`, with its own mage deploy. Go backend.
- 2026-11-13, Nate: viewers only look. Photos stay until cleared by hand from the admin.
- 2026-11-13, progress: built in `4276417` (backend) and `0fef899` (pages, mage, README with
  the push API). `mage keeperSetup` made the Dry Dock app `SortViewer` (always on) at
  `lego.n8p.xyz`, its Gangway OIDC client and its settings; `mage deploy` shipped
  `sortviewer-1`, live and answering. Tested: Go tests for the push API, access rules and
  sign-in; a local run with pushed data and live events. Not tested: the real Gangway sign-in
  (needs Nate to sign in) and the pages in a browser. Gangway sign-in is only covered by the
  redirect check, not a test against a fake provider.
- 2026-11-13, closed at Nate's say-so with three boxes unconfirmed: the real Gangway admin
  sign-in, and the live view and collection pages in a browser. They get checked when Nate
  signs in and when [sort-sharing-push](../todo/sort-sharing-push.md) sends real data.
  Commits: `4276417`, `0fef899`, plus the admin clear/delete test.
- First steps on the live site: Admin sign-in, make a sorter key (shown once), add viewers.
- 2026-11-13, Nate: name `sortviewer`, at `lego.n8p.xyz`. Viewers get site-local
  username/password accounts; only the admin uses Harbor Keeper OIDC. Unknowns show as
  "unknown" for now.
