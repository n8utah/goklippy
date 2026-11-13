# Sorter collections

Priority: medium
Added: 2026-11-13 by Nate

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
- A collection keeps the sorting plan it was sorted with (the profile's bins and which
  categories go in each), so we can say what groups it will come back in.

## Done when
- [ ] Collections can be created, renamed and archived.
- [ ] Starting a sort requires (or defaults to) a chosen collection, shown while sorting.
- [ ] Each piece records its collection; runs from earlier are left without one or put in a default.
- [ ] Pieces page filters by collection; counts and unknowns show per collection.
- [ ] A collection shows the sorting plan (bins and their categories) used for it.

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Needed before [sort-sharing-push](sort-sharing-push.md) and [sort-viewer-app](sort-viewer-app.md).
- Today: `sort_runs` → `pieces`; the plan is `sorting_profiles` + `bin_categories`. A collection
  likely hangs off `sort_runs` (a run belongs to one collection), with pieces reached through
  their run. To decide when planned.
- Open: is the collection set per run, or can it change mid-run? Can a collection be sorted
  with more than one profile over its life, and if so which plan do we show?
- Open: what happens to existing pieces/runs with no collection.
