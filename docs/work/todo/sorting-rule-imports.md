# Sorting rule imports

Priority: medium
Added: 2026-11-15 by Nate

## Why
Split out of [sorting-rule-editor](../doing/sorting-rule-editor.md) so the editor ships first.
Lists that already exist (a BrickLink wanted list, a MOC's parts, what we found last time)
should become rules without typing part numbers.

## What
Imports that turn an outside list into a rule (or a set of kit rules) in the rule editor:
- a BrickLink wanted list / set inventory XML, or a BrickStore `.bsx` file (already BrickLink
  part and colour ids, which is what Brickognize gives us);
- a Rebrickable part list or MOC parts CSV (Rebrickable ids, translated to BrickLink ones;
  parts that can't be mapped are reported);
- the parts already seen in one of our collections (e.g. "pull out everything we found last time").

Profiles made in the editor can be exported as JSON and imported on another sorter.

## Done when
- [ ] BrickLink XML / BrickStore and Rebrickable CSV part lists import as rules, with
      unmapped parts reported.
- [ ] A rule can be built from the parts seen in a collection.
- [ ] Profiles export and import as JSON; Hive uploads still work unchanged.

## Plan
Filled in when the item is claimed.

## Notes
- Set-by-number import stays in the rule editor item (it's one of the rule kinds there).
- Id systems: see the notes in sorting-rule-editor. BrickLink's API needs OAuth keys
  registered to an IP, so BrickLink lists come in as files, not via the API.
