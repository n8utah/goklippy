# AGENTS.md — goklippy

Instructions for any AI coding agent (Claude, Copilot, Codex, Cursor, Gemini, …) working in this repo.
This is the single source of truth; tool-specific files (e.g. `CLAUDE.md`, `.github/copilot-instructions.md`) only point here.

## Project Architecture
- Monorepo layout:
  - `backend/`: Go backend application.
  - `frontend/`: React frontend (Vite).
  - `database/`: Atlas schema (`schema.sql`, SQLite).
  - `docs/`: PRDs, implementation plans, and mockups for development.
  - `private/`: Local-only keys, logs, and configs (never committed).
  - `mage.go`: Build and deployment task runner.

## Backend Rules (Go)
- Main entrypoint is `backend/app.go`.
- Folder responsibilities are strict:
  - `backend/routes/`: HTTP endpoints and route wiring.
  - `backend/logic/`: business logic and orchestration.
  - `backend/models/`: database access and SQL queries only.
- Route/model separation is required:
  - routes handle request/response mapping, validation, and auth checks.
  - models handle database access.
  - keep business flow explicit and testable.
- All database queries must live in `backend/models/`.
- Use `log/slog` for backend logging (structured logs); do not introduce the legacy `log` package for application logging.
- For HTTP logging, prefer structured middleware-based request logs with fields like `request_id`, `method`, `path`, `status`, and latency.
- Use SQLite (`modernc.org/sqlite`, pure Go, no cgo) with `sqlx`. The database file lives in `private/` (default `private/goklippy.db`).
- Use `testify` for Go tests (`assert`/`require`) for cleaner and more readable test code.
- Auth is JWT-based and must support:
  - login
  - refresh token
  - logout

## Frontend Rules (React + Vite)
- Frontend lives in `frontend/` and is built with Vite.
- Frontend calls backend REST APIs only.
- Use ESLint + Prettier for frontend style/lint handling and keep configs committed.
- Use a strict feature-folder page structure:
  - `PageName.js` (UI only)
  - `PageName.logic.js` (state/business logic)
  - `PageName.module.css` (only if needed; see the design system below)
  - local `components/`
- Keep concerns separated using the page structure above.
- Design system (see `docs/ui-ux-spec.md`):
  - Tailwind CSS v4 + shadcn/ui. Theme tokens live in `src/styles/globals.css`; use token classes (`bg-card`, `text-muted-foreground`, `text-primary-text`), never raw hex colors in components, except for marks drawn over photos (the board picture, camera overlays), which must read the same in both themes.
  - shadcn components live in `src/components/ui/` (add with `npx shadcn@latest add <name>`, then fix imports to `@/lib/utils`). Edit them freely; they are our code.
  - Tailwind's reset is scoped: page markup sits inside an element with the `ds` class (each page's root has it; portal content in `components/ui` already does).
  - Style with Tailwind classes; a `PageName.module.css` only for what utilities can't express cleanly (none needs one today).
  - App components shared by 2+ pages live in `src/components/` (AppFrame, SettingField, JourneyModal, PieceCard, NumberField, ObjectSelect, Segmented, …); reuse them before writing another.
  - Theme (light/dark/system) comes from `src/helpers/theme.js` (`useTheme`).
  - Import with the `@/` alias (`@/components/ui/button`).
- Local first, shared later policy:
  - build page-specific components locally first.
  - promote to shared only when used by 2+ pages.
  - avoid turning shared folders into catch-all junk drawers.

## Database Rules (Atlas)
- `database/` is the source of truth for schema.
- Use Atlas tooling for schema management.
- Maintain a single full SQL schema file (`database/schema.sql`) with no migration files. The backend runs it on startup, so every statement must be `IF NOT EXISTS`; changes to existing tables are applied with `mage atlasPush`.
- Coordinate schema updates with matching model/query updates in `backend/models/`.

## Docs and Planning
- Use `docs/` for all PRDs, implementation plans, and mockups.
- Treat docs as development artifacts and keep them in sync with code changes.
- `docs/work/` is the work board, shared by several people and agents at once: one Markdown file per item in `todo/`, `doing/` or `done/`. Before claiming, changing or finishing an item, follow the rules in `docs/work/README.md` (pull first, claim by moving the file to `doing/` with `Owner:` set, push each `board:` commit straight away, don't change items someone else owns). Skills for this live in `.agents/skills/` (`tasks`, `task-add`, `task-start`, `task-release`, `task-done`); edit them there, not the Claude Code stubs in `.claude/skills/`.

## Private Assets
- `private/` is for local secrets, local logs, and private config.
- Never commit anything from `private/`.

## Mage Tasks
- `mage.go` should represent this project's real build/deploy workflow.
- Keep common tasks available for:
  - backend build/test
  - frontend build/test
  - combined build
  - deployment entrypoints

## Quality Bar
- Preserve clear boundaries between routes, logic, and models.
- Do not embed SQL outside `backend/models/`.
- Keep files focused and reasonably sized.
- Prefer explicit, readable code over clever shortcuts.
