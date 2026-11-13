---
name: task-start
description: Take a goklippy work-board item - move it from docs/work/todo to doing and write its implementation plan. Use when the user says "start X", "take the next task", "let's work on X", or "pick something up".
---

# Start an item

1. Find the item in `docs/work/todo/` by file name or title. If the user said "next" or
   didn't name one, propose the oldest high-priority item (then medium, then low) and confirm.
   If `docs/work/doing/` already holds three items, say so and ask whether to start anyway.
2. `git mv docs/work/todo/<item>.md docs/work/doing/`.
3. Fill in **Plan**: read the code and docs the item touches first, then write the approach,
   the files/areas that change (respecting the routes/logic/models split in `AGENTS.md`),
   how it will be tested, and open questions. Tighten **Done when** if the boxes are vague.
4. Show the user the plan and stop. Don't start coding until they approve it; the plan in
   the file is the thing they're approving, so update the file if they change it.

Don't commit unless the user asks.
