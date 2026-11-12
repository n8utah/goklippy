# UX spec

The full UX, put together from the decisions in `docs/ui-ux-plan.md` (all ten decided
2026-11-11). The mockups are on the "goklippy UI directions" canvas: direction C (boards C, C ·
Run › Sort, C · flyout), the journey modal, search, the quick setting editor, the status bar in
light and dark, and two phone screens. Where the canvas and this spec differ, this spec wins.

## Principles

1. **Pages are named for jobs.** You find a page by what you want to do, never by how it's built.
2. **Every setting lives once** and can be reached from where it matters (its page, Settings,
   search).
3. **Stop is always one click away**, on every page and screen size.
4. **Status first**: what the machine is doing and what's wrong is the first thing on screen.
5. **The same thing looks the same everywhere**: one button, one input, one card, one piece card,
   one journey modal.
6. **Works on any screen**, comfortable to tap at the machine.

## The frame

### Laptop and desktop (≥ 1024 px)

- **Rail** (84 px, left): the app mark, then Home, Run, Rules, Machine, Diagnose, System, each
  an icon with its label. The current group is highlighted.
  - Clicking or hovering a group icon opens its **flyout**: the group's pages, each with a line on
    what it's for. In Machine, each page also shows its setup tick or warning. Clicking a page goes
    there; Esc or clicking away closes it. Clicking the icon itself (not a flyout item) goes to the
    group's first page.
- **Top bar** (64 px):
  - **Search box** on the left (Ctrl+K anywhere).
  - **Status pill**: state and run ("Sorting · run 14", "Ready", "Paused", "Error"), colored by
    state, with a dot. A running task adds a small progress ring.
  - **Stop**: red, one click, always enabled when there's anything to stop.
- **Status panel** (opens from the pill): the state and run summary (started, pieces, profile);
  board; motor power; light (with switch); chute (homed, aimed at); the running task with
  progress and Cancel; problems (an incident, low power, a lost board) with what to do; and
  Home / Start / Pause / Resume / Stop buttons, enabled as the state allows.
- **Page area**: the page template, below.

### Tablet (640–1023 px)

Same as desktop. Cards go to fewer columns, the camera tiles to two across, and the status panel
opens as a sheet from the top.

### Phone (< 640 px)

- **Top bar**: search icon (opens search full screen), the status pill (opens the status panel as
  a full-height sheet), Stop.
- **Bottom bar**: Home, Run, Rules, Machine, More. More opens a sheet with Diagnose and System and
  their pages. Tapping a group goes to its first page; its pages are the tabs at the top.
- **Tabs** scroll sideways. Part tabs (C1–C4) become pills.
- **Cards** stack in one column. Tables become lists. Side panels open as sheets.
- **Controls**: at least 44 px tall.

## Pages

URLs are `/<group>/<page>/<tab>`. The browser's back button and links work, and a page can be
bookmarked. The first tab is the default.

| Group | Page | URL | Tabs | What's on it | From today's |
|---|---|---|---|---|---|
| — | Home | `/` | — | Status hero (state, profile, Start/Resume or what's blocking), task cards (Sort, Sorting rules, Look into a piece), setup progress (from Machine › Overview), recent problems | new; Dashboard's status |
| Run | Sort | `/run/sort` | — | Cameras (C2, C3, C4 live with overlays), live numbers (pieces, per minute, identified, bins full), stations (C4 slots aboard), the incident card when there is one, **Recently sorted** (rolling strip, newest first; a card opens the journey modal), channel turn/shake controls (folded away while sorting) | Dashboard |
| Run | Records | `/run/records/{pieces,runs,incidents}` | Pieces, Runs, Incidents | Filters, the tables, piece detail with review; a piece opens the journey modal | Records |
| Rules | Profiles | `/rules/profiles` | — | Import, list, activate, view rules | Profiles |
| Rules | Bins | `/rules/bins/{layout,lookup}` | Layout, Where would it go | Layout: layers as tabs, bins grid with counts, full, emptied, kits; per-layer limits; share bins. Where would it go: part/color/size lookup and identify-from-photos | Bins |
| Machine | Overview | `/machine` | — | Setup checklist (board, motors, chute, cameras, light, motor power, profile) with live ticks and links | new |
| Machine | Board | `/machine/board/{connection,wiring,config,picture}` | Connection, Wiring, Config file, Board picture | Connection: ports, connect/disconnect, reset. Wiring: board diagram, slot panel (mark connected, safety timeout). Config file: raw editor, files, save. Board picture: the overlay editor | Devices, Board Setup, Overlay Editor |
| Machine | Channels | `/machine/channels/{c1,c2,c3,c4,routines}` | C1, C2, C3, C4, Routines | Per channel: live rpm and graph, motor, gearing (with calibrate), sections, direction, speed, jog/shake/run; C4 adds indexing (per move, speed, accel, dwell, auto index, set slot 1). Routines: the routine editor | LEGO Sorter (top half), Board Setup's routines |
| Machine | Chute | `/machine/chute/{calibrate,layers,test}` | Calibrate, Layers, Test bins | Calibrate: home, set 0°, the bin wheel, capture first/last bin, work it out, chute settings. Layers: layer servos (open/closed angles), bins per layer, motor and gearing. Test bins: layer wheel, test every bin, send to bin | Chute, LEGO Sorter (distributor) |
| Machine | Cameras | `/machine/cameras/{setup,c2,c3,c4}` | Setup, C2, C3, C4 | Setup: cameras found, previews, watches, find which camera is where, channel list. Per channel: setup checklist, live picture with zones, measure ring, zones, empty picture / across a section, camera settings | Cameras |
| Diagnose | Piece journeys | `/diagnose/journeys` | — | The full list, further back than Sort's strip; a piece opens the same journey modal | Piece journeys |
| Diagnose | Motor bench | `/diagnose/bench` | — | All steppers (jog, speed, groups), all servos (angles, groups, PCA9685), individual controls | Board Setup's side panels |
| System | Settings | `/system/settings/<section>` | one per section | Every registry setting by section (Feeder, Platter, Pictures, Cameras, Chute, Light, Motor power, Appearance) | Tuning, and settings from every page |
| System | Account | `/system/account` | — | Sign in, users, dev tools | Auth / Dev |

## Page template

- **Header**:
  - the group title (Space Grotesk 28);
  - the group's pages as tabs (an underline in the accent color);
  - a line on what the page is for, where it isn't obvious;
  - the page's main actions on the right.
- **Part tabs**: below the header, for pages split into parts (Channels › C1…; Board ›
  Connection…), as pills or a second row of tabs.
- **Content**: cards on a 12-column grid (gap 16). Cards have a title, optional actions in their
  top right, and content.
- **Inline settings** on a page use the shared `SettingField` (label, help, value with unit,
  default and reset), editing the registry value.
- **Empty and blocked states**: say what's missing and link to where it's fixed ("No camera on
  C2: choose one under Cameras › Setup").
- **Machine-moving controls**: disabled while sorting or busy, with a tooltip saying why.

## Design tokens

### Color (light / dark)

| Token | Light | Dark | Use |
|---|---|---|---|
| `background` | #f6f4f0 | #171412 | Page ground |
| `surface` | #ffffff | #1f1b18 | Cards, rail, bars, dialogs |
| `surface-muted` | #fafaf9 | #262220 | Inputs, hover |
| `border` | #e7e5e4 | #2e2925 | Card and input borders |
| `border-strong` | #d6d3d1 | #44403c | Secondary buttons |
| `text` | #1c1917 | #f5f5f4 | Body text |
| `text-muted` | #57534e | #a8a29e | Secondary text (passes 4.5:1 on surfaces) |
| `accent` | #c2410c | #c2410c | Primary buttons, selected tabs, links (button text white) |
| `accent-text` | #c2410c | #fb923c | Accent used as text or lines on the ground |
| `media` | #1c1917 | #000000 | Behind camera pictures and piece crops |
| `running` | #16a34a / bg #f0fdf4 | #4ade80 / bg #052e16 | Sorting, OK, kept, ticks |
| `warning` | #b45309 / bg #fffbeb | #fbbf24 / bg #451a03 | Needs attention, unidentified |
| `danger` | #dc2626 | #dc2626 | Stop, errors |
| `idle` | #78716c | #a8a29e | Ready, standby |

Status colors differ in lightness as well as hue, and always come with a word or icon.

### Type

- **Space Grotesk** (600/700): page titles 28, section titles 18–20, hero 34.
- **Inter** (400/500/600/700): body 15, secondary 13, small labels 12 (never smaller), buttons 14–15.
- **JetBrains Mono** (500): live numbers (rpm, volts, fps, angles, counts in tables).

### Spacing, shape, size

- **Spacing**: 4 px scale; cards pad 16–20; grid gap 16; page padding 28 (desktop), 14 (phone).
- **Radius**: 10 (inputs, buttons), 14 (cards), 18 (dialogs), full (pills).
- **Controls** (comfortable density): 40 px buttons and inputs on desktop, 44 on touch; rows 48–56.
- **Elevation**: cards flat with a border; popovers and dialogs with one soft shadow.
- **Breakpoints**: 640, 1024, 1440.

## Components

**shadcn/ui** (copied into `frontend/src/components/ui/`, restyled with the tokens): Button
(primary, secondary, ghost, danger), Input, Select, Switch, Checkbox, Slider, Tabs, Dialog, Sheet
(phone panels), Popover (status panel, flyout), Command (search), Tooltip, Badge, Card, Table,
Progress, Toast (Sonner), Separator, Skeleton.

**App components** (shared, in `frontend/src/components/`):
- **Frame**: AppFrame, Rail, RailFlyout, BottomBar, TopBar, StatusPill, StatusPanel, StopButton.
- **Page structure**: PageHeader (title, tabs, actions), PartTabs.
- **Settings and setup**: SettingField and SettingQuickEditor (from the registry), SetupChecklist.
- **Pieces and cameras**: CameraView (exists), CameraTile, PieceCard, RecentStrip, JourneyModal,
  RingDiagram.
- **Search**: SearchPalette.

AGENTS.md changes with the switch: Tailwind for styling with the shadcn components, CSS modules
only where a page needs something special. The feature-folder page structure stays.

## Search (Ctrl+K)

- **Opens** on Ctrl+K or "/" anywhere, or the search box (the icon on phone, full screen).
- **Empty**: recent pages and settings (last 5), then up to 3 suggested actions for the
  machine's state.
- **Results** are grouped Settings, Pages, Actions, ranked by match. Each shows its place
  ("Settings › Feeder", "Machine › Cameras › C3"). A setting shows its current value. An action
  that moves the machine says so ("Moves C3 · only while paused or stopped").
- **Keys**: ↑↓ choose, Enter open (a page goes there, a setting opens its quick editor, an action
  runs or asks for its one input), Esc close. ">" limits to actions.
- **Sources**: the page list (with tabs and keywords), the settings registry, and an action list
  (each with label, keywords, when it's allowed, and what it calls).

## Settings registry

One frontend module (mirrored by the backend's settings defaults). Each entry has:
- **What it is**: key (`feeder.exitPulseDeg`), label, help, unit, kind (number, enum, boolean,
  text, a section name), and min, max and step for numbers.
- **Its value's rules**: default, and whether it applies while sorting.
- **Where it lives**: its section on System › Settings, and the pages and tabs that show it inline.
- **Search**: keywords.

The registry drives the Settings page, the inline SettingFields, the quick editor, search, and
reset to default. Values still save in the `machine` settings document.

## Journey modal

Opened from Run › Sort's strip, Records and Diagnose › Piece journeys:
- **Header**: piece (thumbnail, part, color, score, bin, run, time, id), link to its record.
- **Its way through the machine**: one card per stage (C2, C3, C4, chute). Each shows its kept
  pictures, what was dropped and why, and the handoff ("fell to C3 cleanly"). Clicking a picture
  shows its frame details (the current Piece journeys detail: ring diagram, decision, sharpness).
- **What was sent to Brickognize**: the pictures sent, with their channel, and the answer.

Until linking across channels (phase 9 step 2) exists, a piece shows the stages it has.

## Accessibility

- **Keyboard**: everything reachable; visible focus rings in the accent color; the rail, tabs and
  search are keyboard-navigable; dialogs trap focus and return it.
- **Contrast**: text 4.5:1 in both themes; status never by color alone.
- **Touch**: targets of at least 44 px on touch screens.
- **Motion**: live updates don't animate layout; reduced motion respected.

## Build order

1. **Foundation**: Tailwind, shadcn, tokens (light and dark), fonts, theme switch. No page
   changes.
2. **Frame and routes**: React Router with the URLs above; AppFrame with rail, flyout, top bar,
   status pill and panel, Stop, bottom bar on phones. Today's pages render inside it under their
   new URLs; the old tab row goes.
3. **Search and settings**: the settings registry, System › Settings (replacing Tuning),
   SettingField, the quick editor, the search palette with pages, settings and actions.
4. **Pages onto the template**, one at a time:
   - Run › Sort (with the recent strip and journey modal) and Home;
   - Machine › Board (merging three pages);
   - Channels and Chute (splitting LEGO Sorter);
   - Cameras, Bins, Records, Profiles, Piece journeys, Motor bench, Account.
5. **Machine › Overview** checklist, and Home's setup progress.

Each step keeps the app working and is its own commit.

### Progress

- **Step 1** (2026-11-11): done. `src/styles/globals.css`, `src/components/ui/`, the UI kit
  at System › UI kit.
- **Step 2** (2026-11-11): done. `src/components/AppFrame/`, the page list in
  `src/helpers/navigation.js`, routes in `src/App.js`, and one shared sorter WebSocket
  (`SorterStateProvider`). For now:
  - `/` opens Run › Sort and `/machine` opens Board, until Home and Overview are built; neither
    is in the rail yet.
  - Motor bench isn't listed: its controls are still on Board › Wiring.
  - Search finds pages and their parts; settings and actions come with step 3.
  - Board's Config file tab is still the Config file switch inside Wiring.
  - Old pages render inside `.legacy-page`, which stays light in dark mode.
  - Stop stops the run (or else cancels the running task) and any motor turned by hand.
- **Step 3** (2026-11-11): done. The registry is `src/helpers/settingsRegistry.js` (eight
  sections, 50 settings), read and written through `src/helpers/useSetting.js`; one shared copy
  of the machine document (`MachineSettingsProvider`). `SettingField`, `SettingQuickEditor`
  (opened with `useQuickEditor().open(key)`), System › Settings at
  `/system/settings/<section>` (Tuning is gone), and search over settings, pages and actions,
  with recent and suggested when empty and ">" for actions only. For now:
  - Wiring (which stepper drives what), calibrated geometry and camera picture size stay on their
    pages, where they're measured; the registry holds what a person tunes.
  - `stopped: true` settings (platter sectors) are marked "Change while stopped", not locked.
  - No action asks for an input yet (such as turning a channel by an amount).
  - Pages show inline SettingFields as they're rebuilt in step 4.
- **Step 4, part 1** (2026-11-11): Home (`/`, `pages/HomePage`: status hero with what's
  blocking a sort and where to fix it, task cards, recent problems) and Run › Sort
  (`pages/SortPage`: state and controls, the run's numbers, incident card, camera tiles, on C4
  now, the Recently sorted strip, turn/shake folded away while sorting). Shared `PieceCard` and
  `JourneyModal` (opened with `useJourneyModal().open(pieceId)`, mounted in the frame). The old
  Auth/Dev page is `pages/AccountPage`; the Dashboard is gone. For now:
  - Pieces aren't linked to their C2/C3 camera journeys yet (phase 9 step 2), so the modal shows
    C4 and the chute from the piece's record and points to Diagnose › Piece journeys.
  - Home's setup progress comes with Machine › Overview (step 5).
- **Step 4, part 2** (2026-11-11): Machine › Board (`pages/BoardPage`, one route
  `/machine/board/:part?` so tabs keep edits) replaces Devices, Board Setup and the Overlay
  Editor: a connection strip and unsaved-wiring bar on every tab; Connection (ports, check for
  boards, connect, a dialog instead of the browser's confirm for unsaved wiring); Wiring;
  Config file; Board picture. Diagnose › Motor bench (`pages/BenchPage`) takes the steppers,
  servos, groups, routines and PCA9685 panels. The config workspace is shared through
  `helpers/useKlipperWorkspace.js`. For now:
  - The board diagram, the pin panel (`BoardPage/components/SlotPanel`), the picture editor
    (`pages/BoardEditorPage`) and the bench panels are the earlier components in light panels;
    restyling them is still to do.
- **Step 4, part 3** (2026-11-11): Machine › Channels (`pages/ChannelsPage`, a tab per
  channel and Routines, speeds as SettingFields) and Machine › Chute (Calibrate, Layers, Test
  bins), splitting the LEGO Sorter page; its logic is `helpers/useMachineMotors.js`. Shared
  `GearCalibration`, `NumberField`, `ObjectSelect`.
- **Step 4, part 4** (2026-11-11): Cameras (Setup, C2–C4), Bins (Layout with a bin sheet,
  Where would it go), Records (Pieces, Runs, Incidents; a piece opens the journey modal, which
  also holds the review), Profiles (delete asks first), Piece journeys and Account rebuilt.
- **Step 5** (2026-11-11): done. Machine › Overview (`/machine`, `pages/OverviewPage`) is the
  setup checklist from `helpers/useSetupChecklist.js` (board, motors, chute, cameras, light,
  motor power (optional), profile; live ticks, each linking to where it's done), and Home shows
  its progress and next step until it's complete.

- **Restyle** (2026-11-11): the board diagram, pin panel, picture editor
  (`BoardPage/components/PictureEditor`, no longer a page) and the bench (Steppers and Servos
  tabs at `/diagnose/bench/<part>`) are rebuilt on the design system; `app.module.css` and the
  `.legacy-page` island are gone. Only `CameraView.module.css` (the camera overlay) remains.

Still to do: link pieces to their C2/C3 journeys (backend phase 9 step 2) so the journey modal
shows every stage.
