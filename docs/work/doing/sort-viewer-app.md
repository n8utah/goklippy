# Sort viewer site

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-13

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
- [ ] Admin can create viewer accounts; viewers sign in with username and password.
- [ ] Sorter push API accepts runs, pieces, photos and plans with a valid key, rejects without.
- [ ] Viewers see only collections they've been given.
- [ ] Live view updates while the sorter runs.
- [ ] Counts, unknowns, pieces with photos and the sorting plan are viewable per collection.
- [ ] Admin can clear a collection's photos (or the whole collection) by hand.

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Works with [sort-sharing-push](sort-sharing-push.md); needs [sorter-collections](sorter-collections.md).
- Harbor Keeper deploy and Gangway sign-in: see how `charmingcharlees` does it (its `AGENTS.md`,
  Deploy section), and the `percy-hosted-app` / `fav-newproj` setups.
- 2026-11-13, Nate: code lives in this repo under `services/`, with its own mage deploy. Go backend.
- 2026-11-13, Nate: viewers only look. Photos stay until cleared by hand from the admin.
- 2026-11-13, Nate: name `sortviewer`, at `lego.n8p.xyz`. Viewers get site-local
  username/password accounts; only the admin uses Harbor Keeper OIDC. Unknowns show as
  "unknown" for now.
