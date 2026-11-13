---
name: task-release
description: Give up a claimed goklippy work-board item - move it from docs/work/doing back to todo with a note on where it got to, so someone else can pick it up. Use when the user says "release X", "unclaim X", "stop working on X", "put X back", or "pause X".
---

# Release an item

Follow the sharing rules in `docs/work/README.md`.

1. `git pull --rebase origin lego-sorter`.
2. Find the item in `docs/work/doing/`. If its `Owner:` isn't the user, say so and only
   continue if they confirm (e.g. clearing out an abandoned claim).
3. Add a dated line under **Notes** on where the work got to: what's done, what's left, the
   branch and last commit holding any unfinished code. Keep the **Plan** and any ticked boxes.
4. Remove the `Owner:` and `Started:` lines (keep `Branch:` if unfinished code lives there),
   then `git mv docs/work/doing/<item>.md docs/work/todo/`.
5. Commit only that file as `board: release <item>` and push. Rejected push: pull with rebase,
   push again.
