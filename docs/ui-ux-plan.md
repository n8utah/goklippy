# UI/UX plan

The app grew a page at a time and it shows: 13 equal tabs in one row, settings spread across
pages, and names that don't say what a page is for. This plan lists the decisions to make, in
order, then how we turn them into a full UX. Mockups of the options are on the "goklippy UI
directions" canvas.

## What's wrong today (from screenshots of every page, 2026-11-11)

- **No grouping.** Running a sort, setting the machine up, and debugging sit side by side as 13
  tabs. Nothing says which you need now.
- **Names don't match jobs.** "LEGO Sorter" is the motor bench and distributor setup.
  "Devices", "Board Setup" and "Overlay Editor" are three steps of one job (the board). "Tuning"
  is half of the settings; "Auth / Dev" is a developer page in the main menu.
- **Settings are scattered.** Feeder speeds and gearing on LEGO Sorter, chute geometry on Chute,
  layer servos on LEGO Sorter, pulse rules on Tuning, light and power on Tuning, camera size on
  Cameras. Some appear twice. There's no way to find one without knowing where it lives.
- **No persistent status.** Machine state, board, power, light and Stop are on the Dashboard only;
  on any other page you can't see that it's sorting or stop it.
- **Long pages.** LEGO Sorter is ~3,000 px of everything (four motor cards, routines, the whole
  distributor); Bins repeats a full grid per layer.
- **Visual system.** Hand-rolled CSS modules per page: similar-but-different buttons, inputs and
  cards; a serif body font by default; a busy gradient background.

## Decisions, in order

Each decision has options; the recommendation is a starting point to argue with. We go through
them one at a time, generate ideas where needed, decide, and record the decision here.

### 1. Menu structure (information architecture)
What groups and pages exist. Proposal (canvas board 1): **Run** (Sort, Records), **Sorting rules**
(Profiles, Bins), **Machine** (Overview checklist, Board, Channels, Chute, Cameras), **Diagnose**
(Piece journeys, Motor bench), **System** (Settings, Account). Merges: Devices + Board Setup +
Overlay Editor → Board (tabs); LEGO Sorter splits into Channels and the distributor part of Chute;
Tuning → Settings. Open questions: is Piece journeys under Diagnose or Records? Does Motor bench
deserve a page or live inside Board?

### 2. App frame and navigation
How the menu is shown.
- **A. Grouped left sidebar** with a status bar on top. Familiar, scales to more pages, always
  shows where you are. Costs ~250 px of width.
- **B. Run / Set up / Diagnose modes** in the top bar, each mode with its own row of tabs.
  Very clear about "what am I doing now"; two clicks to cross modes; dense, good on a dark
  control-room screen.
- **C. Icon rail + task hub home.** Slim icons, a home page of tasks with the setup checklist
  first. Friendliest for occasional use; icons alone need labels to be learnable.
- Recommendation: **A**, with C's setup checklist as Machine › Overview.

### 3. Always-visible machine status
What's on every page. Recommendation: state (and run), board, motor power, light, the running
job with Cancel, and Home / Start / Pause / Stop, in the frame's top bar. Open question: does Stop
stay one click, or confirm while sorting?

### 4. Design system and components
- **shadcn/ui** (Radix primitives + Tailwind, components copied into the repo): accessible
  dialogs, command palette (cmdk), tabs, popovers, forms; easy to restyle; adds Tailwind and a
  `components/ui/` folder alongside CSS modules (AGENTS.md's page structure stays; pages use the
  shared components).
- **Keep CSS modules + our own tokens and components.** No new tooling; we build the command
  palette, dialogs and focus handling ourselves.
- **A component library** (Mantine, Chakra): fastest to full coverage, heavier, harder to make
  our own.
- Recommendation: **shadcn/ui**. Needs an AGENTS.md update for Tailwind.

### 5. Look: color, theme, type, density
- Light, dark, or both (dark suits a machine room and camera views; light suits setup forms).
- One accent color, plus status colors (running, warning, error, idle) that differ in lightness
  too.
- Type: Inter for UI, a mono for live numbers.
- Density: compact (tables, many controls) vs comfortable (touch at the machine).
- The canvas shows three: neutral light (A), dark with amber (B), warm light with orange (C);
  each has an accent tweak. Recommendation: neutral light and dark both, following the system
  setting, with a toggle.

### 6. Page layout template
One shape for every page so they read alike. Recommendation: header (group › page title, one
line on what it's for, the page's main actions on the right); content in cards on a 12-column
grid; long pages split into tabs (Board: Connection / Wiring / Config file / Board picture;
Channels: per channel; Chute: Calibrate / Layers / Test); a right-hand panel for the selected
thing where it helps (a bin, a piece, a board slot).

### 7. Global search (Ctrl+K)
One palette that finds **pages**, **settings** (opens the quick editor), **actions** (shake a
channel, blink the light, home, capture empty) and **records** (pieces by id, runs). Canvas board 3.
Open questions: actions that move the machine in search, or not? Prefix (">") for actions only?
Recent items when the box is empty?

### 8. Settings model
Recommendation: every setting is defined once in a registry (key, label, help, unit, range,
default, section, "applies while sorting", which page uses it). The Settings page lists them by
section; machine pages show the ones they use inline; search opens any of them in the quick
editor (canvas board 4). The registry also drives search, defaults and reset, so they can't
drift apart.

### 9. Machine setup flow
Machine › Overview: a checklist (board, motors, chute, cameras, light, motor power, profile) with
ticks from live status and a link to the page that fixes each. Open question: also a first-run
wizard, or the checklist alone?

### 10. Screens and devices
Laptop at the machine is the main screen. Recommendation: work down to tablet width (sidebar
collapses to icons); phone gets status + Stop + records, not setup.

## From decisions to a full UX

1. Go through decisions 1–10, one sitting each or several together; record each decision here.
2. For each, generate a few concrete ideas on the canvas where it's visual (frame, look, page
   template, search, settings dialog), pick, and note why.
3. Write the UX spec: the menu, the frame, the page template, the design tokens, and a page-by-
   page layout (every current page's content mapped onto the new pages).
4. Build in steps that keep the app working:
   1. Design system in (tokens, base components, theme), no page changes.
   2. New frame: menu, status bar, routes for the new page names; old pages render inside it.
   3. Settings registry + Settings page + search with pages and settings.
   4. Page by page onto the template, splitting LEGO Sorter and merging the board pages.
   5. Machine › Overview checklist; search actions and records.
