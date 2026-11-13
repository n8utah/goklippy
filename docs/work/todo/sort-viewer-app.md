# Sort viewer site

Priority: medium
Added: 2026-11-13 by Nate

## Why
When sorting a friend's LEGO, they should be able to watch it live and see what they'll get
back: what's been found, how many, pictures, and how it's being grouped.

## What
A separate web app on our own server (not part of the bench sorter):
- Go backend, React frontend, SQLite, deployed with Harbor Keeper; people sign in through
  Harbor Keeper's identity (Gangway OIDC).
- Receives updates from the sorter over an API authenticated with an API key. Keys are made
  and revoked in the site's admin.
- For each shared collection, the people allowed to see it get:
  - **Live view:** the sort in progress, pieces as they're found, with photos, updating
    without a reload.
  - **Counts:** pieces found, by part, colour and category; unknowns listed separately.
  - **Pieces:** browse and search what's been found, with pictures.
  - **Sorting plan:** the groups their collection is being sorted into (each bin and its
    categories) and what's landed in each so far, i.e. what they'll be handed back.
- Admin (Nate): collections, who can see which, API keys.

## Done when
- [ ] Deployed on Harbor Keeper; sign-in through Gangway works.
- [ ] Sorter push API accepts runs, pieces, photos and plans with a valid key, rejects without.
- [ ] Viewers see only collections they've been given.
- [ ] Live view updates while the sorter runs.
- [ ] Counts, unknowns, pieces with photos and the sorting plan are viewable per collection.

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Works with [sort-sharing-push](sort-sharing-push.md); needs [sorter-collections](sorter-collections.md).
- Harbor Keeper deploy and Gangway sign-in: see how `charmingcharlees` does it (its `AGENTS.md`,
  Deploy section), and the `percy-hosted-app` / `fav-newproj` setups.
- Open: does the code live in this repo (e.g. a `viewer/` folder) or its own repo? It's a
  separate app, so its own repo is likely, with this item tracking it.
- Open: can viewers do anything beyond watching (flag a piece, ask for a part)? Assumed no.
- Open: how long are photos kept on the server?
