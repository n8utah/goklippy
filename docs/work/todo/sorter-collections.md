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
- A collection has a sorting plan (the profile's bins and which categories go in each), so we
  can say what groups it will come back in. The plan can change partway through; the
  collection always shows its current plan, and a change is synced to the viewer site.
- Existing pieces and runs are test data and get cleared out, so nothing exists without a
  collection.

## Done when
- [ ] Collections can be created, renamed and archived.
- [ ] Starting a sort requires (or defaults to) a chosen collection, shown while sorting.
- [ ] Each piece records its collection; existing test pieces, runs and photos are cleared.
- [ ] Pieces page filters by collection; counts and unknowns show per collection.
- [ ] A collection shows its current sorting plan (bins and their categories), and the plan
      can be changed partway through.

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Needed before [sort-sharing-push](sort-sharing-push.md) and [sort-viewer-app](sort-viewer-app.md).
- Today: `sort_runs` → `pieces`; the plan is `sorting_profiles` + `bin_categories`. A collection
  likely hangs off `sort_runs` (a run belongs to one collection), with pieces reached through
  their run. To decide when planned.
- 2026-11-13, Nate: the plan can change during a collection's sort; the latest plan is the
  one shown and synced. Existing pieces/runs were all test data and can be cleared.
- Open: is the collection set per run, or can it change mid-run? (Assumed per run.)
