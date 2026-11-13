# Work board

The plan for goklippy lives here, in the repo, next to the code it describes.
Each piece of work is one Markdown file, and the folder it sits in is its status:

| Folder   | Means                                                 |
| -------- | ----------------------------------------------------- |
| `todo/`  | Things we want to do. Ideas and planned features.     |
| `doing/` | Being worked on now. Keep this short (one to three).  |
| `done/`  | Finished. Kept as a record of what was built and why. |

Bigger reference docs (PRDs, specs, hardware notes) stay in `docs/` and `docs/prds/`;
an item links to them rather than copying them.

## Working with items

- **Add:** copy the template below into `todo/<short-name>.md` (kebab-case, e.g. `todo/servo-sweep-routine.md`).
- **Start:** `git mv docs/work/todo/<item>.md docs/work/doing/`, then fill in the plan.
- **Finish:** `git mv docs/work/doing/<item>.md docs/work/done/`, tick the done-when boxes and add the date.
- **Drop:** delete the file, or move it to `done/` with a line saying why it was dropped.

Order within `todo/` is set by the `Priority` line, not by file name.
To see the board at a glance:

```sh
ls docs/work/todo docs/work/doing
grep -H "^Priority:" docs/work/todo/*.md
```

## Item template

```markdown
# <Feature name>

Priority: high | medium | low
Added: YYYY-MM-DD
Finished: YYYY-MM-DD

## Why
What problem this solves, and for whom.

## What
What the feature does, from the user's side. Use generic hardware names
(stepper_x, servo_board ch 0), not sorter-specific ones.

## Done when
- [ ] ...

## Plan
Filled in when the item moves to `doing/`: approach, files touched, open questions.

## Notes
Decisions, links to related docs and commits.
```
