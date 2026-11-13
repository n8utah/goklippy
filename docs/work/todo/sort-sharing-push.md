# Push sorting updates to the viewer site

Priority: medium
Added: 2026-11-13 by Nate

## Why
The collection's owner wants to watch their LEGO being sorted from home. The sorter runs on
the bench; the [viewer site](sort-viewer-app.md) runs on a server. The sorter has to send it
what's happening.

## What
- In the sorter's settings: the viewer site's URL and an API key (made in the viewer site).
  A test button to check they work.
- Per collection, a switch for whether it's shared to the viewer site.
- While sorting a shared collection, the sorter pushes: run started/stopped, each piece as it's
  identified (part, colour, category, bin, confidence, unknown or not), its photo(s), and the
  collection's sorting plan (bins and their categories) when it changes.
- If the site can't be reached, updates queue up and are sent when it's back; sorting never
  waits on the site.
- Shows on the sorter whether sharing is working and how far behind it is.

## Done when
- [ ] URL and API key can be set and tested from the sorter's settings.
- [ ] Shared collections push run, piece, photo and plan updates while sorting.
- [ ] Sorting carries on at full speed with the site down, and catches up after.
- [ ] Sorter shows sharing status (connected, queued count, last error).

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Depends on [sorter-collections](sorter-collections.md). The API itself is defined in
  [sort-viewer-app](sort-viewer-app.md); plan the two together.
- Open: send every photo, or one per piece, and at what size? Matters for upload bandwidth.
- Open: should a review correction made later on the sorter (wrong part/colour) be pushed too?
- Open: backfill pieces sorted before sharing was turned on?
