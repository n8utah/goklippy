# Sorting rule editor

Priority: medium
Added: 2026-11-13 by Nate
Owner: Nate + Claude
Started: 2026-11-15

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
    picked from a list, so standard bricks, plates, tiles and so on are a click each,
  - size (2 x 4, 1 x 2, …, from the part name),
  - specific part numbers (printed/assembled variants fall back to the base part, as today),
  - colours,
  - a set/list inventory ("kit"): only the listed parts and colours, optionally only until the
    kit has enough of each.
- A rule can send its pieces to one category or split them by size, part type or colour.
- **Premade defaults**: a library of ready rules (standard bricks by size, plates, tiles,
  slopes, Technic beams, …) and starter profiles to copy and build on.
- Rules can be reordered, disabled, renamed; the category name is what the Bins page shows.
- What's left over goes to misc, or is sorted by part type or by colour (a choice on the profile).
- A "where would this go?" check in the editor: type or pick a part and colour, see which rule
  takes it (reuses the dry run).
- **Set by number** is one kind of rule: type a LEGO set number (`75192-1`) and its inventory
  becomes a kit, without its minifigures or spare parts. A rule can hold one set or several;
  each set still fills separately, into its own bins. Uses a Rebrickable API key entered once
  per sorter in Settings.
- Other imports (BrickLink/BrickStore files, Rebrickable CSVs, from a collection) and JSON
  export/import are in [sorting-rule-imports](../todo/sorting-rule-imports.md).
- The viewer site shows a shared collection's rules as well as its categories: each rule's
  name and description, not its part lists.
- Uploading a Hive profile that was edited here makes a second profile; the edited one is
  left as it is.

## Done when
- [ ] A profile can be created from scratch or copied from any existing one, and its rules
      added, edited, reordered, disabled and deleted on the sorter. A Hive upload can be
      edited in place or copied first.
- [ ] Rules can match on part types (picked from a list, family or exact), sizes, part
      numbers, colours, and set/list inventories (kits), and split by size, part type or colour;
      the leftover behaviour (misc / by part type / by colour) is chosen per profile.
- [ ] Premade rules and starter profiles are offered in the editor and can be added, copied
      and changed.
- [ ] A set can be imported by set number and becomes a kit rule with the right BrickLink part
      and colour ids, minifigures and spare parts left out; one rule can take one set or
      several, and each set fills separately into its own bins.
- [ ] Re-uploading a Hive profile that was edited here makes a second profile and leaves the
      edited one untouched.
- [ ] The Rebrickable API key is set once per sorter in Settings; without one, set import says
      so and points there.
- [ ] The viewer site shows a shared collection's rules (name and description) alongside its
      categories.
- [ ] The editor's "where would this go?" shows the matching rule for a part and colour.
- [ ] Edited profiles route on the machine exactly as the editor says, including kits filling
      up; editing a profile a collection is using follows the same "bins emptied / carry on"
      choice as changing a collection's profile.

## Plan
**Shape.** An editor profile is stored as its own document (`sorting_profiles.source = 'editor'`):
name, description, the leftover choice and an ordered list of rules. Each rule has an id, name,
description, enabled flag, what it matches, and how its pieces are split into bins. On load it
compiles to the router's existing program (ordered rules, first match wins, kits, fallback), so
routing, kits filling up, pre-assigning bins and the Bins page don't change shape. Hive files never
carry the new fields, so they route exactly as today.

**What a rule matches** (all optional, all must hold):
- *Part types*: pick from a list. A family ("Plate": every plate type) or an exact type
  ("Plate, Round"). Plain "Brick"/"Plate"/"Tile" are the standard ones; "Brick, Modified",
  "Brick, Round" and so on are listed separately under the family.
- *Sizes*: read from the part name Brickognize gives ("Brick 2 x 4" → 2 x 4, "Brick 1 x 2 x 3" →
  1 x 2 x 3). Pick sizes from a list (the common ones plus any size the catalog has seen).
- *Part numbers* (base-part fallback as today) and *colours*.
- *Sets*: one or more LEGO set numbers. Each set is fetched from Rebrickable once (BrickLink part and
  colour ids via `external_ids`, minifigures and spares left out; unmapped parts listed on the rule)
  and becomes its own kit category, filling separately until it has enough.

**Split.** A rule can send everything to one category or split it into one bin group per size,
per part type or per colour ("Standard bricks, one per size" → "Bricks 1 x 2", "Bricks 2 x 4", …).
Split categories are made as pieces arrive, like the part-type built-in does today; the common
ones are listed up front so "pre-assign from the rules" can place them.

**Premade defaults.** A preset library in the backend (one Go file, easy to extend):
- *Rules* you add with one click and then change: Standard bricks (by size), Standard plates
  (by size), Tiles, Slopes (all slope types), Wedges, Round bricks & plates, Modified bricks,
  Modified plates, Brackets & hinges, Technic beams, Technic pins/axles/connectors, Technic
  gears, Wheels & tyres, Windows & doors, Minifigure parts, Plants & animals.
- *Starter profiles* to copy: "Bricks and plates by size, the rest by part type"; "Building
  basics, the rest by colour"; plus the two built-ins, which copy into an editor profile as a
  single "everything, split by part type / by colour" rule.

**Hive profiles.** Copying one, or editing one in place, converts its program into editor rules
(part lists, colour lists, kits, fallback). Upload already always adds a new row, so a re-upload
leaves an edited one alone; that gets a test.

**Editing a profile in use.** Saving a profile a current collection sorts by asks the same
"bins emptied / carry on" question as changing the collection's profile (`ChangeProfile`).

**Files**
- `backend/logic/sorter/profile/`: `editor.go` (the document, validation, compile to program),
  `match.go` (part type and size conditions, split), `sizes.go` (size from part name),
  `presets.go`, `hive_convert.go`; `profile.go` gains the new rule fields; `Piece` gains `PartName`
  (passed in from `platter.go`, `identify.go` and the dry-run route).
- `backend/logic/sorter/rebrickable.go`: set lookup client (set parts, id mapping).
- `backend/logic/sorter/sorting.go`: create, copy, save (with the bins choice), convert.
  `settings.go`: Rebrickable API key.
- `backend/models/sorting_store.go`: update a profile's name/summary/data/source. Schema: an
  `updated_at` column on `sorting_profiles` (`mage atlasPush`).
- `backend/routes/sorter_sorting_routes.go`: `GET/PUT /sorter/profiles/:id/rules`,
  `POST /sorter/profiles` (new or `{copy_of}`), `GET /sorter/rule-presets`,
  `GET /sorter/part-types` (types and sizes for the pickers), `POST /sorter/sets/lookup`;
  `/sorter/route` takes `part_name`.
- Frontend: `pages/RuleEditorPage/` (`RuleEditorPage.js`, `.logic.js`, `components/`: RuleList with
  drag reorder, RuleForm, TypePicker, SizePicker, ColorPicker, SetPicker, PresetMenu, WhereCheck);
  ProfilesPage gets New / Copy / Edit; SettingsPage gets the Rebrickable key.
- Viewer: the share push carries each rule's name and description; `services/sortviewer/` shows them.

**Tests.** Unit tests in `profile/` (sizes from names, type/size matching, split, compile, Hive
conversion, set kits filling separately), the Rebrickable client against `httptest`, the
store update in a targeted models test, and API checks with curl against the running backend.
Nate checks the UI.

**Open questions**
1. Sizes: is 2 x 4 the same as 4 x 2? (BrickLink names put the small side first, so it should
   rarely matter; plan is to treat them as the same.) Do 1 x 2 x 3 bricks count as their own size?
   (Plan: yes.)
2. Is the preset list above the right first set? Anything missing (e.g. transparent colours as a
   colour group)?
3. When a set's kit is full, its extra pieces fall through to the next rules (today's kit
   behaviour). OK?

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
- 2026-11-15, Nate: imports (other than set by number) moved to sorting-rule-imports. The
  editor needs lots of sensible premade defaults to build on, part types picked from a list
  (plates, standard bricks, …), sizes, and a rule kind that takes a set number.
- 2026-11-13, Nate (follow-ups):
  - Set imports leave out spare parts as well as minifigures; neither is needed.
  - Several sets in one rule each fill separately, into separate bins (not one shared pile).
  - Re-uploading a Hive profile that was edited in place makes a second profile.
  - The viewer shows each rule's name and description, not the part lists. So a rule has a
    description field.
