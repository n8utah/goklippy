# Built-in sorting profiles

Priority: medium
Added: 2026-11-13 by Nate

## Why
The only way to get a sorting profile today is to build a `sorting_profile.json` in Hive and
upload it (tens of MB). The sorter has none stored, so there's nothing to sort by without a trip
to Hive first. Most sorts just want "plates here, bricks there" or "by colour", which needs no
part lists at all: Brickognize already returns each part's BrickLink category and we store it on
every piece (`pieces.part_category`).

Separately, the active profile is machine-wide, so a collection just sorts by whatever profile
happened to be active. Now that a collection has a sorting plan, it should say which profile it
uses.

## What
- Two built-in profile types, created from Rules › Profiles with no file:
  - **By part type**: the category is Brickognize's BrickLink category grouped by the part
    before the comma ("Plate", "Plate, Round" and "Plate, Modified" all go to **Plate**; likewise
    Brick, Slope, Tile, Wedge, Technic, Wheel, ...). Today's catalog of ~25 categories comes to
    about a dozen groups.
  - **By colour**: one category per colour.
- Pieces with no part type (or no colour, for the colour profile) go to misc, as they do now.
- Fine-tuning stays on the Bins page: a bin can already take several categories (e.g. Tile +
  Wedge in one bin).
- Uploading Hive profiles keeps working as it does today.
- A collection picks its profile. A run can't start without one: starting a run sorts by the
  collection's profile.

## Done when
- [ ] A "by part type" and a "by colour" profile can be created from the Profiles page without
      uploading anything, and show alongside uploaded ones.
- [ ] Sorting with them routes each piece by its Brickognize part type (grouped) or its colour;
      pieces without one go to misc.
- [ ] Bins can be assigned these categories on the Bins page like any other profile's.
- [ ] A collection has a profile, chosen when it's created and changeable later.
- [ ] Starting a run requires the collection to have a profile, and the run sorts by it.
- [ ] Hive uploads still import, activate and route as before.

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
- Builds on [sorter-collections](../doing/sorter-collections.md) (collections and their
  per-collection bin layouts).
- The plan already listed "local profiles without Hive" as not done yet
  (`docs/lego-sorter-server-plan.md`, phase 6).
- Routing today takes only part and colour (`profile.Route`); by part type also needs the
  piece's Brickognize category.
- Out of scope for now: exact (ungrouped) BrickLink category, part type + colour combined, a
  rule editor, Hive sync.
- Open: once collections choose their profile, is there still a machine-wide "active" profile
  (e.g. for the dry-run "where would this go?"), or does it go away?
- Open: changing a collection's profile partway through: same as today's profile switch (each
  profile keeps its own bins for that collection)?
