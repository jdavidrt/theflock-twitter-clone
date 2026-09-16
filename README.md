# The Flock — Twitter Clone

A full-stack Twitter/X clone built for The Flock's technical challenge: custom authentication, tweets, a followed-users timeline, likes, follows, search, reply threads, and a mobile-first responsive UI. Go API + SQLite on the back, React + Vite on the front.

> **Project status:** Step 2 complete (2026-09-16) — domain model, the hand-authored sample dataset, and the in-memory store are in place and verified (`go build`/`go test` green). No feature endpoints exist yet; auth lands in Step 3. SQLite persistence lands as its own step before delivery. This README describes the current setup and is kept accurate as implementation lands (see commit history). Sections not yet implemented are marked **Planned**.

---

## Table of Contents

- [Features](#features)
- [Technology Stack & Justification](#technology-stack--justification)
- [Architecture](#architecture)
- [Runbook (Setup & Operations)](#runbook-setup--operations)
- [Technical Decisions](#technical-decisions)
- [Known Trade-offs & Limitations](#known-trade-offs--limitations)
- [AI Tooling Usage](#ai-tooling-usage)

---

## Features

**Core (required):**

- Custom email/password authentication (registration, login, logout, protected routes)
- User profile: unique username, display name, bio, avatar placeholder
- Create / delete tweets (280-char limit, enforced client- and server-side)
- Timeline of followed users, chronological, cursor-paginated (infinite scroll)
- Follow / unfollow, like / unlike, visible like counts, followers/following lists
- Basic user search by name/username
- Fully responsive, mobile-first UI (breakpoints: <640px, 640–1024px, >1024px)

**Bonus (selected for this delivery):**

- Reply threads (self-referential tweets with a thread view)

**Post-MVP backlog:** Docker Compose one-command startup (deferred so nothing in the MVP depends on Docker).

**Explicitly out of scope for this delivery:** real-time updates, image uploads, notifications. See [Known Trade-offs & Limitations](#known-trade-offs--limitations).

---

## Technology Stack & Justification

| Layer          | Choice                                                 | Why                                                                                                                                                                                                                                                            |
| -------------- | ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Backend        | Go 1.23+, standard library `net/http`                  | Routing (Go 1.22 method/path patterns), JSON, testing, coverage and HTTP test harness all ship in the standard library — no framework to justify, one static binary to run, compiled types end to end. A code-quality review reads idiomatic stdlib Go easily. |
| Database       | SQLite via `database/sql` + `modernc.org/sqlite`       | Relational DB required by the brief, which names SQLite as preferred. File-based, zero install, nothing to run alongside the app. Pure-Go driver: no CGO, builds anywhere. Schema is hand-written SQL, applied at boot.                                        |
| Persistence    | Phased: in-memory store first, SQLite last             | The data model and a sample dataset come first; the API serves it from memory while the frontend is built, then the SQLite store implements the same `Store` interface. Handlers, services and tests do not change between phases.                             |
| Frontend       | React 18 + Vite + TypeScript                           | Fast dev server, large ecosystem, and the framework most AI coding tools generate reliably — directly relevant to a challenge that grades how well the tools are directed.                                                                                     |
| Styling        | Plain CSS (hand-written stylesheets, class selectors)  | No utility-class or CSS-in-JS framework — plain, reviewable stylesheets with mobile-first base rules and `min-width` media queries layered on top (D-68).                                                                                                      |
| Data fetching  | TanStack Query                                         | Caching, pagination and refetch semantics needed for an infinite-scroll timeline without hand-rolling loading/error state.                                                                                                                                     |
| Auth           | Custom: `x/crypto/bcrypt` + signed httpOnly JWT cookie | The brief forbids third-party auth providers; this is the minimal correct custom implementation (hashed passwords, signed session token, server-validated on every protected request).                                                                         |
| Backend tests  | Go `testing` + `net/http/httptest`                     | Unit tests beside the code; HTTP-level integration tests against the real handler with a fresh store per test; one conformance suite run against both store implementations.                                                                                   |
| Frontend tests | Vitest + React Testing Library (+ MSW)                 | Integration-style tests for the flows the brief calls out: login, create tweet, follow.                                                                                                                                                                        |
| E2E            | Playwright                                             | Real-browser test of the full auth flow, as required.                                                                                                                                                                                                          |

The stack itself isn't graded — see [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md) for the requirement-by-requirement validation, and [my-docs/DECISIONS.md](my-docs/DECISIONS.md) D-65…D-67 for the pivot from the original Node/Express/PostgreSQL plan.

---

## Architecture

**Repository layout:**

```
/server                 Go module — the API
  /cmd/api              Process bootstrap (env, store, HTTP server, graceful shutdown)
  /internal/config      Typed configuration from env + a small .env loader
  /internal/httpapi     Handlers, middlewares, JSON responses (the HTTP layer)
  /internal/domain      Entity structs (User, Tweet, Follow, Like)
  /internal/validation  Pure validation rules (username, email, 280 chars…)   — Step 3
  /internal/service     Business rules, no HTTP imports                       — Step 3+
  /internal/store       Store interface + memory store + sample-data loader + conformance suite; sqlite — Step 11
  /data/sample.json     Hand-authored sample dataset (12 users, tweets, follows, likes, replies)
/client                 React + Vite + TypeScript SPA
  /src/pages            Route-level views
  /src/components       Reusable UI
  /src/api              TanStack Query hooks per resource
  /tests                Frontend integration tests
/e2e                    Playwright E2E specs                                    — Step 12
/.github/workflows      CI: Go (gofmt, vet, race tests, coverage ≥ 85 %) + client (lint, build, tests)
.env.example            Every environment variable the app reads
my-docs/                Internal planning docs (requirements validation, decisions, plan, AI collaboration log)
```

Layering on the server: `httpapi` (decode, validate, respond) → `service` (rules) → `store` (interface). Handlers never touch the store; services never see `*http.Request`.

### Data model (timeline & follows graph)

- **users**: `id, username (unique, lowercase), email (unique, lowercase), display_name, password_hash, bio, created_at, updated_at` — no avatar column; the avatar placeholder is rendered client-side from the user's initials.
- **follows**: `follower_id, followee_id, created_at` (composite PK, both FKs to `users`, indexed both directions, self-follow blocked by a `CHECK` constraint)
- **tweets**: `id, author_id (FK users), content (≤280 code points), parent_tweet_id, created_at, deleted_at` — `parent_tweet_id` is a nullable self-referential FK (reply threads; `NULL` = top-level). Deletes are soft (`deleted_at`) so threads stay intact when a parent is removed.
- **likes**: `user_id, tweet_id, created_at` (composite PK, both FKs)

**Timeline query:** top-level, non-deleted tweets authored by `me` or by anyone `me` follows, ordered `created_at DESC, id DESC`, cursor-paginated on `(created_at, id)` for stable infinite scroll under concurrent writes. All counts (likes, replies, followers, following) are computed on read.

The same model is served by both store implementations: the in-memory store (maps + slices, loaded from `server/data/sample.json`) during development, and the SQLite store (`schema.sql`, seeded from the same file) for delivery.

---

## Runbook (Setup & Operations)

> The commands below reflect what exists **today** (Step 2: scaffold + health endpoint + domain model + sample data + in-memory store, loaded at boot but not yet exposed by any feature endpoint). Later steps add to this section in the same commit that adds the feature.

### Prerequisites

- **Go 1.23+** — https://go.dev/dl/
- **Node.js 20+** and npm 10+ (the frontend and the workspace scripts; CI runs on 20)
- Git

No database server: SQLite is a file created by the app.

### Setup & run (local)

```bash
git clone https://github.com/jdavidrt/theflock-twitter-clone.git
cd theflock-twitter-clone
cp .env.example .env          # defaults work as-is
npm install                   # installs the client workspace + root tooling
npm run dev                   # API on http://localhost:3000, client on http://localhost:5173
```

`npm run dev` runs `go run ./cmd/api` in `server/` and the Vite dev server in `client/` side by side (Go modules are downloaded on first run). The client calls the API through a relative `/api` path that Vite proxies to port 3000, so there is no CORS setup and no client-side env var.

Smoke check: `curl http://localhost:3000/api/health` → `{"status":"ok"}`.

On start, the API loads `server/data/sample.json` into memory and logs the counts (`loaded sample data store=memory users=12 tweets=169 follows=63 likes=499`); while the in-memory store is the default, **restarting the API resets the data to the sample**. No endpoint serves this data yet — that starts in Step 3. **Planned (Step 11):** the default switches to SQLite (`server/data/twitter.db`, created and seeded automatically on first start; `npm run seed` resets it).

### Running tests

```bash
npm test                   # Go suite (server) + client suite
npm run test:coverage      # Go coverage over server/internal — target ≥ 85 % total statements
npm run lint               # ESLint (client) + go vet (server)
npm run format:check       # Prettier check (client + config files); CI also checks gofmt
npm run format             # Prettier --write + gofmt -w
npm run build              # go build → server/bin/, tsc + vite build → client/dist/
```

Server-only shortcuts from `server/`: `go test ./...`, `go test ./internal/... -coverpkg=./internal/... -coverprofile=coverage.out && go tool cover -func=coverage.out` (the `-coverpkg` flag is needed once cross-package test helpers exist, e.g. `internal/store/storetest` — see DECISIONS.md D-32).

**Planned (Step 12):** `npm run test:e2e` — Playwright auth flow against an already-running `npm run dev`.

### Environment variables

Copy [`.env.example`](.env.example) to `.env` at the repository root. The API looks for `.env` in its working directory and then in the parent, so it is found whether you run from the root or from `server/`. Variables already set in the environment always win over the file.

| Variable           | Default / example                       | Description                                                                                                |
| ------------------ | --------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `PORT`             | `3000`                                  | API listen port                                                                                            |
| `APP_ENV`          | `development`                           | `development` / `test` / `production`. `test` lowers bcrypt cost, disables rate limiting and silences logs |
| `JWT_SECRET`       | `change-me-to-a-random-32+-char-string` | Secret for signing session tokens (≥ 32 chars). The placeholder is refused when `APP_ENV=production`       |
| `COOKIE_SECURE`    | `false`                                 | Set `true` only when serving over HTTPS                                                                    |
| `STORE`            | `memory`                                | Persistence backend. `memory` loads `SAMPLE_DATA_PATH` at boot; `sqlite` arrives in Step 11                |
| `SAMPLE_DATA_PATH` | `./data/sample.json`                    | Sample dataset the memory store loads at boot, relative to `server/`                                       |

**Planned:** `SQLITE_PATH` / `SEED_FORCE` (Step 11) — each is added to `.env.example` in the step that introduces it. The frontend needs no environment variables.

### Sample credentials

`server/data/sample.json` contains 12 users sharing the password `Password123!`, led by the fixed sample account `alice@example.com` (username `alice`). No login endpoint exists yet (Step 3), but the data is loaded and queryable in-process at boot today.

---

## Technical Decisions

- **Why this stack:** see [Technology Stack & Justification](#technology-stack--justification) above. The project started on Node/Express/PostgreSQL and pivoted to Go + SQLite on day 2 (2026-09-16); the reasoning and the consequences are recorded in [my-docs/DECISIONS.md](my-docs/DECISIONS.md) D-65…D-67, and the original scaffold was reverted rather than rewritten in place so the commit history stays honest.
- **Timeline & follows graph modeling:** see [Architecture](#architecture) above.
- **Phased persistence:** the API is written against a `Store` interface. An in-memory implementation loaded from a hand-authored sample dataset lets the whole API and the frontend be built and tested first; the SQLite implementation is added behind the same interface and passes the same test suite. The delivered app runs on SQLite.
- **Authentication:** custom-built per the brief's requirement (no third-party auth). Passwords hashed with bcrypt (cost 12); sessions are a signed httpOnly JWT cookie (HS256, 7 days, `SameSite=Lax`) validated by middleware on every protected route. No refresh-token rotation in this delivery — see trade-offs below.
- **Known trade-offs and limitations:** see next section.
- **AI tools used:** see [AI Tooling Usage](#ai-tooling-usage) below.

---

## Known Trade-offs & Limitations

- **Session strategy:** a single httpOnly JWT cookie rather than an access+refresh token pair. Simpler and sufficient for this scope; means sessions can't be selectively revoked per-device without additional work.
- **Counts on read:** like/reply/follower counts are counted per request instead of stored — correct by construction at this scale, would need denormalizing for a large dataset.
- **Private by default:** every page and endpoint except register/login requires a session; there is no logged-out public browsing of profiles or tweets.
- **Soft deletes:** deleted tweets keep their row (`deleted_at`) so reply threads don't lose their parent; they never appear in feeds or counts.
- **In-memory phase:** until the SQLite store lands (Step 11), data written through the API lives in process memory and a restart resets it to the sample dataset. This is a development convenience, not the delivered behavior.
- **Single-writer SQLite:** fine for this scale; a multi-instance deployment would need a server database.
- **No Docker Compose yet:** deferred to the post-MVP backlog so the core work never depends on it; the local Runbook needs only Go and Node.
- **No image uploads, real-time updates, or notifications** in this delivery — deferred deliberately to keep the required features solid within 72 hours. See [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md) §8 for the full out-of-scope list and rationale.
- This section will grow as real implementation trade-offs are made.

---

## AI Tooling Usage

This project was built with Claude Code throughout — planning, scaffolding, feature implementation, and test writing. Every prompt used and the work it produced is logged verbatim in [my-docs/AGENT-COLLABORATION-HISTORY.MD](my-docs/AGENT-COLLABORATION-HISTORY.MD), and the requirement analysis that preceded implementation is in [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md). Project-specific operating instructions for AI sessions live in [CLAUDE.md](CLAUDE.md), including a working rule that the agent edits plain-text and config files directly rather than via generated scripts — kept there because it's an agent-operating rule, not part of the delivered app.
