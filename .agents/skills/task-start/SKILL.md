---
name: task-start
description: Claim a goklippy work-board item - move it from docs/work/todo to doing under the user's name, push the claim, and write its implementation plan. Use when the user says "start X", "take the next task", "let's work on X", "pick something up", or "take over X".
---

# Claim and start an item

Follow the sharing rules in `docs/work/README.md`.

1. `git pull --rebase origin lego-sorter`.
2. Find the item in `docs/work/todo/` by file name or title. If the user said "next" or didn't
   name one, propose the oldest high-priority item (then medium, then low) and confirm.
   If it's already in `doing/` under someone else, say who and since when, and don't take it
   unless the user says to take it over.
3. Claim it, as its own commit, before doing any other work:
   - `git mv docs/work/todo/<item>.md docs/work/doing/`
   - add `Owner: <git user.name> + <your agent name, e.g. Claude, Codex>` and `Started: <today>`; add `Branch:` if the code
     will live on a branch other than `lego-sorter`.
   - commit only that file as `board: start <item>` and push.
   - If the push is rejected, `git pull --rebase`. If the rebase shows someone else claimed the
     same item first, abort (`git rebase --abort`), reset your claim commit
     (`git reset --hard origin/lego-sorter` is fine only if you have no other local commits;
     otherwise drop just that commit), tell the user who has it, and offer another item.
4. Write the **Plan**: read the code and docs the item touches first, then write the approach,
   the files/areas that change (respecting the routes/logic/models split in `AGENTS.md`), how
   it will be tested, and open questions. Tighten **Done when** if the boxes are vague.
   Commit and push the plan as `board: plan <item>`.
5. Show the user the plan and stop. Don't start coding until they approve it; the plan in the
   file is the thing they're approving, so update the file (and push) if they change it.

Taking over someone else's item: only with the user's go-ahead. Change `Owner:`, add a note
under **Notes** ("Taken over from Ashton, 2026-11-14"), commit as `board: take over <item>`, push.
