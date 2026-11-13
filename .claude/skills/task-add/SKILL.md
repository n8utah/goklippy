---
name: task-add
description: Add a new item (feature, fix, idea) to the goklippy work board in docs/work/todo. Use when the user says "add a task", "put X on the list", "we should do X sometime", or describes a feature they want planned.
---

# Add an item to the board

1. Pick a short kebab-case file name from the feature (`servo-sweep-routine.md`). Check
   `docs/work/{todo,doing,done}` for an existing item covering the same thing; if there is one,
   tell the user and offer to update it instead.
2. Create `docs/work/todo/<name>.md` from the template in `docs/work/README.md`:
   - `Added:` today's date. Leave out the `Finished:` line.
   - `Priority:` what the user said; otherwise `medium`, and mention it.
   - **Why**, **What** and **Done when** from what the user told you, in their terms. Use generic
     hardware names (stepper_x, servo_board ch 0), not sorter-specific ones.
   - Leave **Plan** with its placeholder line; it's filled in when the item starts.
   - Don't invent requirements. Where something important is unknown, write it as an open
     question under **Notes** rather than guessing.
3. Several features at once: one file each.
4. Reply with the file(s) created and a one-line summary of each, plus any open questions.

Don't write code or move other items. Don't commit unless the user asks.
