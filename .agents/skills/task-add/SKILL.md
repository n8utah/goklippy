---
name: task-add
description: Add a new item (feature, fix, idea) to the goklippy work board in docs/work/todo. Use when the user says "add a task", "put X on the list", "we should do X sometime", or describes a feature they want planned.
---

# Add an item to the board

Follow the sharing rules in `docs/work/README.md`.

1. `git pull --rebase origin lego-sorter`.
2. Pick a short kebab-case file name from the feature (`servo-sweep-routine.md`). Check
   `docs/work/{todo,doing,done}` for an existing item covering the same thing; if there is one,
   tell the user and offer to update it instead.
3. Create `docs/work/todo/<name>.md` from the template in `docs/work/README.md`:
   - `Added:` today's date and the user's `git config user.name`. No `Owner:`, `Branch:`,
     `Started:` or `Finished:` lines yet.
   - `Priority:` what the user said; otherwise `medium`, and mention it.
   - **Why**, **What** and **Done when** from what the user told you, in their terms. Use generic
     hardware names (stepper_x, servo_board ch 0), not sorter-specific ones.
   - Leave **Plan** with its placeholder line; it's filled in when the item is claimed.
   - Don't invent requirements. Where something important is unknown, write it as an open
     question under **Notes** rather than guessing.
   Several features at once: one file each.
4. Commit only the new file(s) as `board: add <name>` (one commit for a batch is fine) and push.
   If the push is rejected, pull with rebase and push again; if someone added a file with the
   same name meanwhile, rename yours.
5. Reply with the file(s) created and a one-line summary of each, plus any open questions.

Don't write code or touch other items.
