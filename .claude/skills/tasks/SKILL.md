---
name: tasks
description: Show the goklippy work board (docs/work) - what's in progress, what's queued by priority, and what was finished recently. Use when the user asks "what are we working on", "list tasks", "what's next", "show the board", or asks for an item's status.
---

# List the work board

The board is `docs/work/`; each item is one Markdown file and its folder is its status
(`todo/`, `doing/`, `done/`). `docs/work/README.md` describes the format.

1. Read the title (`# ...`), `Priority:` and `Added:`/`Finished:` lines of every item file
   (skip `.gitkeep`). `grep -H -m1 "^# \|^Priority:\|^Added:\|^Finished:"` over the folders is enough;
   don't read whole files unless the user asks about one item.
2. Report in this order:
   - **Doing**: every item, with the first unticked `Done when` box so it's clear where it stands.
   - **To do**: grouped high, medium, low; oldest `Added` first within a group.
   - **Done**: the five most recently finished, unless the user asks for all.
3. Show each item as its title plus file name (e.g. `Servo sweep routine (servo-sweep-routine.md)`).
   Say plainly when a column is empty.

If the user names one item ("status of servo sweep"), find it by file name or title and
summarise that file instead: status, why, done-when progress, and the plan if it has one.

Don't change any files in this skill.
