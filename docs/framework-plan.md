# goklippy Framework Plan

## Goal
Create a clean, buildable baseline for a Go backend and Vite React frontend with clear architecture boundaries and JWT authentication scaffolding.

## Architecture Targets
- Backend (`backend/`):
  - `app.go` as the main entrypoint.
  - `routes/` contains HTTP endpoint handlers and route registration.
  - `logic/` contains business logic and orchestration.
  - `models/` contains all database access and SQL queries.
- Frontend (`frontend/`):
  - Vite + React application.
  - API calls only to backend REST endpoints.
  - Page structure with `.js` + `.logic.js` + `.module.css` where logic grows.
- Database (`database/`):
  - Atlas schema and migration source of truth.
  - Add migration workflow and docs as next step.
- Docs (`docs/`):
  - PRDs, implementation plans, mockups, and technical notes.
- Private (`private/`):
  - Non-committed configs, keys, and local logs.
- Mage (`mage.go`):
  - Standardized build and test commands for backend and frontend.

## Milestones
1. Baseline Build
- Add backend Go module and compile-safe app skeleton.
- Add JWT auth scaffolding (login, refresh, logout endpoints).
- Add frontend Vite React app that builds.
- Add basic mage tasks for build/test workflows.

2. Data Layer Foundation
- Define initial Atlas schema files and migration naming convention.
- Add users and refresh token tables for auth.
- Add model methods for auth and user lookup.

3. API & Auth Hardening
- Add input validation and structured error responses.
- Add auth middleware for protected endpoints.
- Add token revocation and session expiration policies.

4. Frontend Feature Baseline
- Add auth screens and state management.
- Add API helper abstraction with environment-based backend URL.
- Add route guards for authenticated pages.

5. Delivery Readiness
- Add integration tests for auth routes.
- Add frontend smoke tests.
- Update deployment and environment docs.

## Definition of Done for Foundation
- `go build ./...` succeeds in `backend/`.
- `npm run build` succeeds in `frontend/`.
- Auth routes are reachable and return expected HTTP responses.
- Copilot instructions align with this repository structure and conventions.
