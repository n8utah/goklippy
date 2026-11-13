---
name: task-done
description: Complete a goklippy work-board item - check its done-when list, record the date, and move it from docs/work/doing to done. Use when the user says "X is done", "finish X", "close out X", "mark complete", or wants to drop/cancel an item.
---

# Finish an item

Follow the sharing rules in `docs/work/README.md`.

1. `git pull --rebase origin lego-sorter`.
2. Find the item in `docs/work/doing/` (or `todo/`, if it was done without being claimed).
   If its `Owner:` isn't the user, say who owns it and only continue if they confirm.
3. Go through **Done when**. For each box, check it against the code (and tests, if the box
   says something is tested), on the item's `Branch:` if it has one. Tick the ones that hold.
   If the code is on a branch that isn't merged into `lego-sorter` yet, say so: it's only done
   if the user says that's fine.
4. If any box is still open, list them and ask whether to finish anyway, drop those boxes,
   or keep the item in `doing/`. Don't tick a box you couldn't confirm.
5. Add `Finished: <today>` and a short **Notes** line on what was built and the commits
   involved (`git log --oneline` since `Started:`).
6. `git mv docs/work/doing/<item>.md docs/work/done/`, commit only that file as
   `board: done <item>`, and push. Rejected push: pull with rebase, push again.

Dropping an item instead (user says "drop", "cancel", "won't do"): ask whether to delete it
or keep it; to keep it, add `Dropped: <today>` and a one-line reason under **Notes**, move it
to `done/`, and commit as `board: drop <item>`.
