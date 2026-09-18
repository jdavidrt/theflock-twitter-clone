# The Flock — Twitter Clone

A full-stack Twitter/X clone built for The Flock's technical challenge: custom authentication, tweets, a followed-users timeline, likes, follows, search, reply threads, and a mobile-first responsive UI. Go API + SQLite on the back, React + Vite on the front.

> **Project status:** Step 9 complete (2026-09-18) — profile pages (`/:username`), follow/unfollow with optimistic updates, an inline bio editor, paginated followers/following lists (`/:username/followers`, `/:username/following`), and debounced user search (`/search`) are all live and wired into the nav. Every required frontend flow in the brief now has a passing integration test (login, create tweet, follow). Verified in the browser against the real Go API and alice's real sample follow graph, plus a passing client integration test suite (Vitest + React Testing Library + MSW, 6/6). Step 8's home timeline and Step 7's app shell/auth loop remain in place. Steps 2–6's domain model, sample dataset, in-memory store, custom authentication, profile/follow graph, tweets/timeline/likes, and user search remain in place on the backend (backend coverage **93.4 % total statements**, D-32 floor is 85 %). SQLite persistence lands as its own step before delivery. This README describes the current setup and is kept accurate as implementation lands (see commit history). Sections not yet implemented are marked **Planned**.

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
  /internal/validation  Pure validation rules (username, email, password, display name)
  /internal/service     Business rules, no HTTP imports — auth (Step 3), social/follows (Step 4), tweets/timeline/likes/search (Steps 5-6)
  /internal/store       Store interface + memory store + sample-data loader + conformance suite; sqlite — Step 11
  /data/sample.json     Hand-authored sample dataset (12 users, tweets, follows, likes, replies)
/client                 React + Vite + TypeScript SPA
  /src/pages            Route-level views (Home, Login, Register, Profile, FollowList, Search, NotFound)
  /src/components       Reusable UI (AppShell, AuthLayout, Avatar, ComposeBox, TweetCard, TweetList, Protected/PublicOnlyRoute, icons)
  /src/context          AuthContext — resolves the session once from GET /api/auth/me
  /src/lib              Typed API client (api.ts), validation mirror (validation.ts, D-54), relative-time formatting (time.ts, D-30), infinite-scroll/tweet-feed hooks (useInfiniteScrollSentinel.ts, useTweetFeed.ts)
  /tests                Vitest + React Testing Library + MSW integration tests
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

> The commands below reflect what exists **today** (Steps 1–7: scaffold, health endpoint, domain model + sample data + in-memory store, custom authentication, user profiles + the follow graph, tweets/timeline/likes, user search, and the frontend app shell + auth UI). Later steps add to this section in the same commit that adds the feature.

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

Smoke check: `curl http://localhost:3000/api/health` → `{"status":"ok"}`. Open `http://localhost:5173` in a browser: an unauthenticated visit redirects to `/login`; register or log in (try the [sample credentials](#sample-credentials) below) and you land on `/` inside the app shell (bottom tab bar under 640px, an icon rail from 640px, a labeled sidebar with a centered content column from 1024px — D-28), with a logout action always reachable. `/login` and `/register` redirect an already-authenticated visitor back to `/`.

On start, the API loads `server/data/sample.json` into memory and logs the counts (`loaded sample data store=memory users=12 tweets=169 follows=63 likes=499`); while the in-memory store is the default (Phase 1, D-66), **restarting the API resets all data — including anything created through the API — back to the sample dataset.** This is a development convenience, not the delivered behavior: Step 11 switches the default to SQLite, which persists across restarts. Auth endpoints are live: `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`. Profile and follow endpoints: `GET /api/users/{username}` (profile with counts + `isFollowedByMe`), `PATCH /api/users/me` (edit display name/bio), `POST`/`DELETE /api/users/{username}/follow` (idempotent follow/unfollow), `GET /api/users/{username}/followers` and `/following` (cursor-paginated). Tweet, timeline and like endpoints: `POST /api/tweets` (create), `GET`/`DELETE /api/tweets/{id}` (read / author-only soft delete), `GET /api/users/{username}/tweets` (a user's tweets), `GET /api/timeline` (followed-users + own tweets, cursor-paginated), `POST`/`DELETE /api/tweets/{id}/like` (idempotent like/unlike). Search: `GET /api/search/users?q=` (case-insensitive substring on username or display name, capped at 20 results) — see [Sample credentials](#sample-credentials) to try all of the above against the seeded data. **Planned (Step 11):** the default switches to SQLite (`server/data/twitter.db`, created and seeded automatically on first start; `npm run seed` resets it).

### Running tests

```bash
npm test                   # Go suite (server) + client suite
npm run test:coverage      # Go coverage over server/internal — 93.4 % total statements as of Step 6 (floor: 85 %, D-32)
npm run lint               # ESLint (client) + go vet (server)
npm run format:check       # Prettier check (client + config files); CI also checks gofmt
npm run format             # Prettier --write + gofmt -w
npm run build              # go build → server/bin/, tsc + vite build → client/dist/
```

Server-only shortcuts from `server/`: `go test ./...`, `go test ./internal/... -coverpkg=./internal/... -coverprofile=coverage.out && go tool cover -func=coverage.out` (the `-coverpkg` flag is needed once cross-package test helpers exist, e.g. `internal/store/storetest` — see DECISIONS.md D-32).

Client-only shortcut from `client/`: `npx vitest` (watch mode). The suite renders the real `App` against a mocked API (MSW) and now covers all three required frontend flows: `client/tests/integration/login.test.tsx` (login end to end — valid credentials land on `/` with the username visible in the nav, invalid credentials show the server's error message inline), `client/tests/integration/compose-tweet.test.tsx` (the live counter updates while typing, submitting posts a tweet and it appears at the top of the timeline, plus the 280-code-point submit-disable boundary), and `client/tests/integration/follow-flow.test.tsx` (clicking Follow on another user's profile flips the button to "Following" and increments the follower count; the reverse for Unfollow).

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

`server/data/sample.json` contains 12 users sharing the password `Password123!`, led by the fixed sample account `alice@example.com` (username `alice`). Confirmed against a **freshly started** API (Phase 1 loads the sample dataset fresh on every boot — see the restart note above):

```bash
curl -i -c cookies.txt -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"Password123!"}'

curl -i -b cookies.txt http://localhost:3000/api/auth/me

curl -i -b cookies.txt http://localhost:3000/api/users/alice

curl -i -b cookies.txt "http://localhost:3000/api/timeline?limit=5"

curl -i -b cookies.txt "http://localhost:3000/api/search/users?q=aR"   # case-insensitive substring, matches carol & oscar
```

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
- **Client-side validation is a UX mirror, not the source of truth (D-54):** `client/src/lib/validation.ts` pins the same length bounds, username regex and code-point counting as the server so bad input is caught before a round trip, but business rules like the reserved-username list are checked server-side only — the client just displays whatever `details` the server returns.
- **No Docker Compose yet:** deferred to the post-MVP backlog so the core work never depends on it; the local Runbook needs only Go and Node.
- **No image uploads, real-time updates, or notifications** in this delivery — deferred deliberately to keep the required features solid within 72 hours. See [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md) §8 for the full out-of-scope list and rationale.
- This section will grow as real implementation trade-offs are made.

---

## AI Tooling Usage

This project was built with Claude Code throughout — planning, scaffolding, feature implementation, and test writing. Every prompt used and the work it produced is logged verbatim in [my-docs/AGENT-COLLABORATION-HISTORY.MD](my-docs/AGENT-COLLABORATION-HISTORY.MD), and the requirement analysis that preceded implementation is in [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md). Project-specific operating instructions for AI sessions live in [CLAUDE.md](CLAUDE.md), including a working rule that the agent edits plain-text and config files directly rather than via generated scripts — kept there because it's an agent-operating rule, not part of the delivered app.
