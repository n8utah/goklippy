---
name: task-done
description: Complete a goklippy work-board item - check its done-when list, record the date, and move it from docs/work/doing to done. Use when the user says "X is done", "finish X", "close out X", or "mark complete".
---

# Finish an item

1. Find the item in `docs/work/doing/` (or `todo/`, if it was done without being started).
2. Go through **Done when**. For each box, check it against the code (and tests, if the box
   says something is tested). Tick the ones that hold.
3. If any box is still open, list them and ask whether to finish anyway, drop those boxes,
   or keep the item in `doing/`. Don't tick a box you couldn't confirm.
4. Add `Finished: <today>` under `Added:`, and a short **Notes** line on what was built and
   the commits involved (`git log --oneline` since the item started).
5. `git mv docs/work/doing/<item>.md docs/work/done/`.

Dropping an item instead (user says "drop", "cancel", "won't do"): ask whether to delete it
or keep it; to keep it, add `Dropped: <today>` and a one-line reason under **Notes**, then
move it to `done/`.

Don't commit unless the user asks.
