# Sort viewer

A small site where people watch their LEGO get sorted: pieces as they're found (our photo
next to the official picture), counts by part and colour, unknowns, the sorting plan (the
groups they'll get back), and which LEGO sets they could build from it. The sorter
(goklippy) pushes everything here; the site never talks to the sorter.

Live at https://lego.n8p.xyz, on Harbor Keeper's Dry Dock as the app `SortViewer`.

- **Viewers** sign in with an email and password the admin made for them, and see only the
  collections they were given.
- **The admin** signs in through Harbor Keeper's Gangway (OIDC). Admins are the Gangway users
  listed in `ADMIN_EMAILS`.

## Layout

| Path | |
|---|---|
| `backend/` | Go + Gin server: `routes/` (HTTP) · `logic/` (rules) · `models/` (SQL). `app.go` is main. |
| `backend/webui/` | The built frontend, embedded in the binary (`mage build` copies it in). |
| `frontend/` | Vite + React, Tailwind + shadcn, goklippy's look. `pages/<Page>/<Page>.js` + `.logic.js`. |
| `database/schema.sql` | SQLite schema, applied on every start (`IF NOT EXISTS`). |
| `database/catalog.sql` | Schema of the set data (`sets.db`, see below). |
| `mage.go` | `build`, `test`, `deploy`, `keeperSetup`. Run mage from this folder. |

## Develop

```sh
# The server on :8090 with a throwaway data folder; SORTVIEWER_DEV_ADMIN signs the admin
# in without the Gangway (only when OIDC isn't set up).
DATA_DIR=./private PORT=8090 SORTVIEWER_DEV_ADMIN=you@example.com go run ./backend

# The pages on :4180, proxying /api and /auth to :8090.
cd frontend && npm install && npm run dev
```

Open http://localhost:4180, use **Admin sign-in**, make a viewer and a sorter key, and push
some data with curl (below).

`mage test` runs the Go tests (push API, access rules, sign-in) and lints the frontend.

## Settings (environment)

| Variable | |
|---|---|
| `PORT`, `HOST` | Where to listen (Dry Dock sets both). Default `:8090`. |
| `DATA_DIR` | Database and photos. Default `KEEPER_DATA_DIR` (Dry Dock), else `../private`. |
| `APP_URL` | The site's address, for cookies and sign-in redirects. |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` | The Gangway client (`mage keeperSetup` makes it). |
| `ADMIN_EMAILS` | Comma-separated emails that may sign in as admin. |
| `SORTVIEWER_DEV_ADMIN` | Local development only: sign in as this admin with no Gangway. |
| `SORTVIEWER_SETS_DIR` | Build the set data from downloaded Rebrickable `.csv.gz` files in this folder instead of fetching them. |

## Set data

Each collection's **Sets** tab ranks LEGO sets by how much of each the collection has, in
the right colours and in any colour, and opens a set's parts with what's missing. The sets
come from [Rebrickable's downloads](https://rebrickable.com/downloads/) (public, no key),
built into `sets.db` in the data folder: on the first start, monthly after that, and from
Admin → Set data. A build takes a minute or two and about 55 MB; the site keeps using the
old file until the new one is swapped in.

- A set's parts are its latest inventory, without spare parts, stickers or minifigures.
  Mold variants and alternates (Rebrickable's `M` and `A` relationships) count as one part;
  prints don't.
- The sorter's part numbers and colours are BrickLink's (from Brickognize). Colours are
  matched by name, which BrickLink and Rebrickable share. Part numbers mostly match; the
  Sets tab lists the ones that don't (`4073` is `6141` on Rebrickable, for example), and
  those pieces aren't counted.
- Scoring every set for a big collection takes a second or two, so it's kept per
  collection until its counts change. The Sets tab doesn't refresh itself while sorting;
  its Refresh button does.

## Deploy

```sh
mage keeperSetup   # once: the Dry Dock app (always on), lego.n8p.xyz, the Gangway client, settings
mage deploy        # build, keeper push SortViewer production -activate, check /api/health
```

Both use the `keeper` CLI's saved login (`keeper login -url https://n8p.xyz`).
`keeperSetup` also calls Harbor Keeper's admin API with that login for the Gangway parts,
which the CLI doesn't cover. Roll back with `keeper versions SortViewer production` and
`keeper activate SortViewer production sortviewer-N`.

## Push API

For the sorter. Every call needs `Authorization: Bearer sv_...` (a key made under Admin →
Sorter keys; only its hash is kept). Ids are the sorter's own, and every call is an upsert,
so retrying or re-sending (a backfill) is always safe. Times are RFC 3339.

| Call | Body | |
|---|---|---|
| `GET /api/push/ping` | | Checks the address and key. |
| `PUT /api/push/collections/:id` | `{name, plan}` | Creates or renames it; a `plan` replaces its plan. |
| `PUT /api/push/runs/:id` | `{collection_id, profile_name, started_at, ended_at}` | `ended_at` null while sorting. |
| `PUT /api/push/pieces/:id` | the piece (below) | Also how review corrections arrive. |
| `PUT /api/push/pieces/:id/photo` | JPEG or PNG bytes | The piece's last C4 photo; shrunk to 800 px. Push the piece first. |
| `POST /api/push/collections/:id/clear-unknowns` | | Deletes its unknown pieces and their photos. |

A piece: `{collection_id, run_id, status, seen_at, sorted_at, part_id, part_name,
part_category, part_score, part_img_url, color_id, color_name, category, category_name,
bin_layer, bin_number, virtual_bin}`. `part_id` "" is an unknown. `bin_layer` 0 with a
`sorted_at` is the bucket. `part_img_url` is the official picture (Brickognize's).

A plan: `{profile: {name}, bins: [{layer, bin, categories: [{id, name}]}], bins_started_at}`.
Each bin's piece count is worked out here from the pieces sorted since `bins_started_at`.

Errors: 400 for a bad body, 401 for a missing or revoked key, 404 for a photo of a piece not
pushed yet.

```sh
curl -X PUT https://lego.n8p.xyz/api/push/collections/1 -H "Authorization: Bearer $KEY" \
  -H 'Content-Type: application/json' -d '{"name":"Sam’s LEGO"}'
```
