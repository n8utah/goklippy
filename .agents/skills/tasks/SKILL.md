---
name: tasks
description: Show the goklippy work board (docs/work) - what's in progress and who owns it, what's queued by priority, and what was finished recently. Use when the user asks "what are we working on", "list tasks", "what's next", "who's on what", "show the board", or asks for an item's status.
---

# List the work board

The board is `docs/work/`; each item is one Markdown file and its folder is its status
(`todo/`, `doing/`, `done/`). `docs/work/README.md` has the format and the rules for sharing it.

1. `git pull --rebase origin lego-sorter` so the board is current. If the pull fails (no network,
   local conflicts), say so and list what's local, marked as possibly stale.
2. Read the title (`# ...`), `Priority:`, `Owner:`, `Branch:`, `Added:`, `Started:` and `Finished:`
   lines of every item file (skip `.gitkeep`). A `grep -H` over the folders is enough; don't read
   whole files unless the user asks about one item.
3. Report in this order:
   - **Doing**: every item with its owner, start date, branch if any, and the first unticked
     `Done when` box. Flag items started more than a week ago with no commit touching them since
     (`git log -1 --format=%cs -- <file>`), as they may be abandoned.
   - **To do**: grouped high, medium, low; oldest `Added` first within a group.
   - **Done**: the five most recently finished, unless the user asks for all.
4. Show each item as its title plus file name (e.g. `Servo sweep routine (servo-sweep-routine.md)`).
   Say plainly when a column is empty. If the user asks "what's mine", filter by their
   `git config user.name`.

If the user names one item ("status of servo sweep"), find it by file name or title and
summarise that file instead: status, owner, why, done-when progress, and the plan if it has one.

Don't change any files in this skill.
