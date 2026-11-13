# Work board

The plan for goklippy lives here, in the repo, next to the code it describes.
Several people and agents (Claude, Codex, Copilot, ...) can use it at the same time;
the rules below are how they avoid stepping on each other.

Each piece of work is one Markdown file, and the folder it sits in is its status:

| Folder   | Means                                                 |
| -------- | ----------------------------------------------------- |
| `todo/`  | Things we want to do. Ideas and planned features.     |
| `doing/` | Claimed by someone and being worked on now.           |
| `done/`  | Finished or dropped. Kept as a record of what was built and why. |

Bigger reference docs (PRDs, specs, hardware notes) stay in `docs/` and `docs/prds/`;
an item links to them rather than copying them.

## Rules for working together

The board's shared copy is the `lego-sorter` branch on `origin`. What's in your local
checkout is only as fresh as your last pull, and a claim nobody else can see isn't a claim.

1. **Pull before you look.** `git pull --rebase origin lego-sorter` before listing, adding,
   claiming or finishing anything.
2. **A board change is its own commit, pushed straight away.** Commit only the item file(s),
   with a message starting `board:` (`board: start servo-sweep-routine`), then push. Don't bundle
   board changes into code commits, and don't sit on them.
3. **Claiming is moving the file to `doing/` and setting `Owner:`.** If the push is rejected,
   pull and look again: if someone else's claim of the same item landed first, theirs stands;
   undo yours and pick something else.
4. **Only the owner changes a `doing/` item** (plan, status, finishing it). To take over someone
   else's item, ask them, or the user, first, then change `Owner:` in a `board:` commit.
5. **One file per item, no shared index.** Nothing lists all items in one file, so two people
   adding or moving different items never conflict.
6. **Code can live elsewhere.** If an item's code is on its own branch or worktree, record it in
   `Branch:`; the item file itself is still changed on `lego-sorter`.
7. **Let go of what you've stopped.** If you stop work on an item before it's done, move it back to
   `todo/`, clear `Owner:`/`Branch:`/`Started:`, keep the plan, and add a note on where it got to.

`Owner:` names the person and, if an agent is doing the work, the agent: `Nate`, `Nate + Claude`,
`Ashton + Codex`. Use the person's `git config user.name`.

## Skills

The board skills live in `.agents/skills/` (the shared Agent Skills format, one `SKILL.md` per skill) so any agent can use them; `.claude/skills/` only holds stubs that point Claude Code at them. Type them as commands where your agent supports that, or just ask in plain words.
Agents that do not load skills follow the same rules from this file.

| Skill           | Does                                                                          |
| --------------- | ----------------------------------------------------------------------------- |
| `/tasks`        | Lists doing (with owners), to do (by priority) and recently done; or one item. |
| `/task-add`     | Writes a new item into `todo/` and pushes it.                                  |
| `/task-start`   | Claims an item (moves it to `doing/`, sets the owner, pushes), then writes its plan and waits for approval before coding. |
| `/task-release` | Gives up a claimed item: back to `todo/` with a note on where it got to.       |
| `/task-done`    | Checks the done-when boxes, dates the item and moves it to `done/`; also drops items. |

Each one pulls first and pushes its `board:` commit at the end.

## Working with items by hand

- **Add:** copy the template below into `todo/<short-name>.md` (kebab-case, e.g. `todo/servo-sweep-routine.md`).
- **Start:** `git mv docs/work/todo/<item>.md docs/work/doing/`, set `Owner:` and `Started:`, commit, push.
- **Finish:** `git mv docs/work/doing/<item>.md docs/work/done/`, tick the done-when boxes, set `Finished:`, commit, push.
- **Drop:** delete the file, or set `Dropped:` with a one-line reason under Notes and move it to `done/`.

Order within `todo/` is set by the `Priority` line, not by file name.
To see the board at a glance:

```sh
git pull --rebase origin lego-sorter
grep -H "^Owner:" docs/work/doing/*.md
grep -H "^Priority:" docs/work/todo/*.md
```

## Item template

Leave a field out until it applies (`Owner:`, `Branch:` and `Started:` arrive with the claim).

```markdown
# <Feature name>

Priority: high | medium | low
Added: YYYY-MM-DD by <name>
Owner: <name> [+ <agent>]
Branch: <branch, if the code isn't on lego-sorter>
Started: YYYY-MM-DD
Finished: YYYY-MM-DD

## Why
What problem this solves, and for whom.

## What
What the feature does, from the user's side. Use generic hardware names
(stepper_x, servo_board ch 0), not sorter-specific ones.

## Done when
- [ ] ...

## Plan
Filled in when the item is claimed: approach, files touched, open questions.

## Notes
Decisions, progress, links to related docs and commits.
```
