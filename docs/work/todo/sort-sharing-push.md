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
- [ ] URL and API key can be set and tested from the sorter's settings.
- [ ] Shared collections push run and piece updates, with the piece's last C4 photo and the
      official part picture, while sorting.
- [ ] Turning sharing on for a collection sends everything already sorted into it.
- [ ] Plan changes, review corrections and cleared unknowns are synced to the site.
- [ ] Sorting carries on at full speed with the site down, and catches up after.
- [ ] Sorter shows sharing status (connected, queued count, last error).

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Depends on [sorter-collections](sorter-collections.md). The API itself is defined in
  [sort-viewer-app](sort-viewer-app.md); plan the two together.
- 2026-11-13, Nate: one photo per piece, the one that looks best, plus the official part
  picture; plan changes and review corrections are synced.
- 2026-11-13, Nate: the photo is the last one taken on C4. Pieces sorted before sharing was
  turned on are backfilled through the same queue.
- To decide when planned: what size to send the photo at.
