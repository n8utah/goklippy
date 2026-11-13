# Sorting rule editor and imports

Priority: medium
Added: 2026-11-13 by Nate

## Why
The only ways to get a sorting profile are the two built-ins (by part type, by colour) or a
`sorting_profile.json` built in Hive and uploaded. Anything in between ("Technic in these bins,
this set's parts pulled out, the rest by colour") needs a trip to Hive. We want to build and
change sorting rules on the sorter itself, and start them from lists that already exist
(a set's inventory, a wanted list, a MOC's parts) instead of typing part numbers.

## What
- **Rule editor** (Rules › Profiles): create a profile, copy an existing one (built-in or
  Hive upload) and edit the copy, or edit a Hive upload in place. A profile is an ordered list of rules, first match wins (the same
  model as Hive's compiled program, so the router doesn't change shape). Each rule sends
  matching pieces to a named category, and matches on any mix of:
  - part type (Brickognize category, grouped like the built-in or exact: "Plate, Round"),
  - specific part numbers (printed/assembled variants fall back to the base part, as today),
  - colours,
  - a set/list inventory ("kit"): only the listed parts and colours, optionally only until the
    kit has enough of each.
- Rules can be reordered, disabled, renamed; the category name is what the Bins page shows.
- What's left over goes to misc, or is sorted by part type or by colour (a choice on the profile).
- A "where would this go?" check in the editor: type or pick a part and colour, see which rule
  takes it (reuses the dry run).
- **Imports** that turn an outside list into a rule (or a set of kit rules). Set by number is
  built first; the rest follow:
  - a LEGO set by number (`75192-1`): its inventory becomes a kit rule, without its
    minifigures. A rule can hold one set or several. Uses a Rebrickable API key entered once
    per sorter in Settings;
  - a BrickLink wanted list / set inventory XML, or a BrickStore `.bsx` file (already BrickLink
    part and colour ids, which is what Brickognize gives us);
  - a Rebrickable part list or MOC parts CSV;
  - the parts already seen in one of our collections (e.g. "pull out everything we found last
    time");
  - Hive profiles, as today.
- Profiles made here can be exported as JSON and imported on another sorter.
- The viewer site shows a shared collection's rules as well as its categories.

## Done when
- [ ] A profile can be created from scratch or copied from any existing one, and its rules
      added, edited, reordered, disabled and deleted on the sorter. A Hive upload can be
      edited in place or copied first.
- [ ] Rules can match on part type, part numbers, colours, and set/list inventories (kits),
      and the leftover behaviour (misc / by part type / by colour) is chosen per profile.
- [ ] A set can be imported by set number and becomes a kit rule with the right BrickLink part
      and colour ids, minifigures left out; one rule can take one set or several.
- [ ] The Rebrickable API key is set once per sorter in Settings; without one, set import says
      so and points there.
- [ ] The viewer site shows a shared collection's rules alongside its categories.
- [ ] BrickLink XML / BrickStore and Rebrickable CSV part lists import as rules.
- [ ] A rule can be built from the parts seen in a collection.
- [ ] The editor's "where would this go?" shows the matching rule for a part and colour.
- [ ] Edited profiles route on the machine exactly as the editor says, including kits filling
      up; editing a profile a collection is using follows the same "bins emptied / carry on"
      choice as changing a collection's profile.
- [ ] Profiles export and import as JSON; Hive uploads still work unchanged.

## Plan
Filled in when the item is claimed.

## Notes
- Builds on [builtin-sorting-profiles](../done/builtin-sorting-profiles.md), which listed
  "a rule editor" as out of scope. The router (`logic/sorter/profile`) already supports ordered
  rules, part/colour lists, base-part fallback, kits that stop when full (`set_inventories`) and
  fallback by colour; a part-type condition is the main new matching piece.
- **Id systems matter.** Brickognize returns BrickLink part numbers and BrickLink colour ids.
  BrickLink/BrickStore files use the same ids. Rebrickable uses its own part and colour ids;
  its API can return the BrickLink equivalents (`external_ids`), but its free bulk CSV
  downloads (sets, inventories, parts, colours) may not carry the mapping. Whatever imports
  from Rebrickable has to translate, and report parts it couldn't map.
- Candidate source for "set by number": the Rebrickable API (free account + API key; set
  inventories, minifig parts, part/colour id mapping). BrickLink's own API needs OAuth keys
  registered to an IP, so for BrickLink the plan is file upload, not API.
- 2026-11-13, Nate:
  - Set by number is the first import to build.
  - The Rebrickable API key is one per sorter, in Settings.
  - Set imports leave out minifigures. A rule can be one set or several sets.
  - A Hive-uploaded profile can be edited in place or copied and then edited.
  - The viewer site shows the rules as well as the categories.
  - Price- or rarity-based rules ("high value" pieces) are not part of this item.
- The viewer part touches the sharing push and `services/sortviewer/`
  ([sort-sharing-push](../done/sort-sharing-push.md), [sort-viewer-app](../done/sort-viewer-app.md)).
- Open questions:
  - Set import: are a set's spare parts counted in the kit, or left out like the minifigures?
  - Several sets in one rule: one shared pile (need = the sets' parts added together), or does
    each set still fill separately inside the rule?
  - Editing a Hive upload in place: a later re-upload of the same Hive profile would overwrite
    the edits or make a second profile?
  - How much of a rule does the viewer show: its name and what it matches in words, or the full
    part lists?
