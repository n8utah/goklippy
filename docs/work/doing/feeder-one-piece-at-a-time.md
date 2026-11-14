# Feeder: one piece at a time into C4

Priority: high
Added: 2026-11-14 by Nate
Owner: Nate + Claude
Started: 2026-11-14

## Why
On a short run (2026-11-14, run 1) about half of C4's drops were several pieces at once
(C4 tracks starting within a fraction of a second: 2, ~7 and 4 together out of 7 drops).
Three causes in the feeder:
- Touching or near-touching pieces are one blob to it: perception joins detections within
  6 px and records that (`Observation.Merged`), but the feeder never looks, so a clump on C2
  is carried and pushed off like one piece. Nothing tries to part it.
- C3 doesn't keep the next piece back (`exitClearDeg.c3` was 0), so whatever rides close
  behind C3's lead goes with it onto C4.
- Moves are fixed sizes rather than what the picture says: carried in 30° or 90° chunks,
  then 90% of the way to the exit, then 2° nudges in the exit until the piece falls,
  whether or not anything is close behind.

## What
- **A.** C3 keeps its next piece back (`exitClearDeg.c3` 3°), on the machine and as the default.
- **B.** Moves come from the picture. A piece short of the exit is carried to the precise
  zone in one move. From there one exit move takes it to where pieces fall off that channel,
  learned from its falls (starting at the middle of the exit zone). Small nudges are only
  for when another piece is close behind (the keep-back rule shortens the push) or the
  piece didn't fall where expected.
- **C.** A blob that's several pieces (joined from several detections, or pile-sized) is
  shaken apart at the precise zone (sudden back-and-forth jogs, so pieces slide on the
  rotor), a few times; if it's still one blob it's nudged off carefully, so the front piece
  goes on its own.
- D (turning C3 while C2 drops, to spread out pieces that fall together) waits until
  A–C have been tried on the machine.

## Done when
- [ ] `exitClearDeg.c3` is 3° on the machine and by default.
- [ ] Carrying goes to the precise zone in one move; the drop-zone pulse and exit approach
      settings are gone.
- [ ] An exit move goes to the channel's learned fall point, limited by the next piece;
      the fall point adapts from falls (fell sooner → lower; didn't fall → higher).
- [ ] Blobs of several pieces are shaken at the precise zone (Settings: how many shakes,
      how far), then nudged off with the careful exit pulse.
- [ ] Feeder tests cover the above; tried on the machine.

## Notes
- 2026-11-14, Nate: do A, B and C now; test on the machine, then decide on D.
