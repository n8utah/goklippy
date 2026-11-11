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

All ten are decided (2026-11-11); the full UX is in `docs/ui-ux-spec.md`. Each decision had options; the recommendation was a starting point to argue with. We went through
them one at a time, generate ideas where needed, decide, and record the decision here.

### 1. Menu structure (information architecture)
**Decided (2026-11-11): five groups by job.** **Run** (Sort, Records), **Sorting rules**
(Profiles, Bins), **Machine** (Overview checklist, Board, Channels, Chute, Cameras), **Diagnose**
(Piece journeys, Motor bench), **System** (Settings, Account). Merges: Devices + Board Setup +
Overlay Editor → Board (tabs); LEGO Sorter splits into Channels and the distributor part of Chute;
Tuning → Settings.

- **Piece journeys:** a Diagnose page for looking further back. Run › Sort shows a rolling
  history of the most recently sorted pieces; clicking one opens its journey in a modal. Diagnose
  › Piece journeys (and a piece in Records) opens the same modal.
- **Motor bench:** its own page under Diagnose (raw stepper jogs, servo angles, all-stepper
  controls), so Board stays about wiring and Channels about the sorter.

### 2. App frame and navigation
How the menu is shown.
- **A. Grouped left sidebar** with a status bar on top. Familiar, scales to more pages, always
  shows where you are. Costs ~250 px of width.
- **B. Run / Set up / Diagnose modes** in the top bar, each mode with its own row of tabs.
  Very clear about "what am I doing now"; two clicks to cross modes; dense, good on a dark
  control-room screen.
- **C. Icon rail + task hub home.** Slim icons, a home page of tasks with the setup checklist
  first. Friendliest for occasional use; icons alone need labels to be learnable.
- **Decided (2026-11-11): C, the icon rail with a home hub.** The rail holds Home and the five
  groups. A group's pages are reached both ways: clicking or hovering a rail icon opens a flyout
  of its pages (each with what it's for and, in Machine, a setup tick), and inside a group its
  pages are tabs under the page title.

### 3. Always-visible machine status
**Decided (2026-11-11): a compact top bar that opens to the full status.** Every page's top bar
holds search, one status pill (state, and run while sorting) and Stop. Clicking the pill opens a
panel with the full status and controls: board, motor power, light, the running task with
Cancel, and Home / Start / Pause / Resume. Stop is one click, never confirmed: it's the safety
action, and stopping is safe (pieces aboard are recorded).

### 4. Design system and components
- **shadcn/ui** (Radix primitives + Tailwind, components copied into the repo): accessible
  dialogs, command palette (cmdk), tabs, popovers, forms; easy to restyle; adds Tailwind and a
  `components/ui/` folder alongside CSS modules (AGENTS.md's page structure stays; pages use the
  shared components).
- **Keep CSS modules + our own tokens and components.** No new tooling; we build the command
  palette, dialogs and focus handling ourselves.
- **A component library** (Mantine, Chakra): fastest to full coverage, heavier, harder to make
  our own.
- **Decided (2026-11-11): shadcn/ui.** Needs an AGENTS.md update for Tailwind and the
  `components/ui/` folder.

### 5. Look: color, theme, type, density
- Light, dark, or both (dark suits a machine room and camera views; light suits setup forms).
- One accent color, plus status colors (running, warning, error, idle) that differ in lightness
  too.
- Type: Inter for UI, a mono for live numbers.
- Density: compact (tables, many controls) vs comfortable (touch at the machine).
- The canvas shows three: neutral light (A), dark with amber (B), warm light with orange (C);
  each has an accent tweak.
- **Decided (2026-11-11): light and dark, following the system setting, with a toggle** (in
  System). Direction C's warm palette: warm off-white ground and white cards in light, warm
  charcoal in dark, one orange accent in both.
- **Decided (2026-11-11): comfortable density** (40–44 px controls, room to tap at the machine)
  and **Space Grotesk for titles, Inter for UI text, JetBrains Mono for live numbers** (rpm,
  volts, fps).

### 6. Page layout template
One shape for every page so they read alike. **Decided (2026-11-11): tabs per part.** Header:
the group title, its pages as tabs, one line on what the page is for, its main actions on the
right. Long pages split into tabs per part, one part at a time: Board › Connection / Wiring /
Config file / Picture; Channels › C1 / C2 / C3 / C4 / Routines; Chute › Calibrate / Layers / Test
bins; Cameras › Setup / C2 / C3 / C4. Content in cards on a 12-column grid; a right-hand panel
for the selected thing where it helps (a bin, a board slot).

### 7. Global search (Ctrl+K)
**Decided (2026-11-11): pages, settings and actions; not records.** One palette that finds
**pages** and their tabs ("chute calibrate", "camera C3 zones"), **settings** (Enter opens the
quick editor) and **actions** (home, start/pause, blink the light, shake a channel, capture
empty, find cameras). Actions that move the machine say so and follow the same rules as their
buttons (refused while sorting, and so on). Pieces and runs are found on Records and Diagnose,
not in search. Opened empty, it shows recent pages and settings, then a few actions that fit the
machine's state (Start when ready, Resume when paused, Blink when the light is off). A ">" prefix
limits it to actions.

### 8. Settings model
**Decided (2026-11-11): one registry, three doors.** Every setting is defined once in a registry
(key, label, help, unit, range, default, section, "applies while sorting", which page uses it).
System › Settings lists them by section; machine pages show the ones they use inline; search
opens any of them in the quick editor (canvas board 4). All three edit the same value. The
registry also drives search, defaults and reset, so they can't drift apart.

### 9. Machine setup flow
**Decided (2026-11-11): the checklist only, no wizard.** Machine › Overview lists each step
(board, motors, chute, cameras, light, motor power, profile) with a tick from live status and a
link to the page that does it, in no forced order; Home shows the progress.

### 10. Screens and devices
**Decided (2026-11-11): fully responsive.** Every page works from laptop to phone. On a phone
the rail becomes a bottom bar (Home, Run, Rules, Machine, More, with Diagnose and System under
More), tabs scroll sideways, cards stack, the status pill and Stop stay in the top bar, and
search opens full screen.

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
