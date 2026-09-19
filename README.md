# The Flock — Twitter Clone

A full-stack Twitter/X clone built for The Flock's technical challenge: custom authentication, tweets, a followed-users timeline, likes, follows, search, reply threads, and a mobile-first responsive UI. Go API + SQLite on the back, React + Vite on the front.

## Purpose & Rights Notice

> [!IMPORTANT]
> **This repository exists for a single purpose: it is the author's submission for The Flock's "AI Verified" technical challenge** — a demonstration that agentic AI tooling can be directed to build a working Twitter-style application from a written brief. It is not an open-source project, a template, or a product.
>
> - **All rights reserved.** The author ([@jdavidrt](https://github.com/jdavidrt)) is the sole rights holder. No license is granted: the source code, documentation, and commit history may not be copied, redistributed, or incorporated into any commercial or business project without the author's explicit written permission.
> - **No AI training.** This repository, in whole or in part, may not be used as training, fine-tuning, evaluation, or benchmarking data for any machine-learning or AI model.
> - **Evaluation use is welcome.** Cloning, running, and reviewing the project locally as part of The Flock's assessment is the intended and expected use.

> **Project status:** Step 12 complete (2026-09-18) — the last step before delivery. All 12 implementation steps are done: custom auth, profiles/follows, tweets/timeline/likes/search, reply threads (bonus), the mobile-first responsive UI, and the **SQLite-by-default** persistence layer (`internal/store/sqlite`, seeded from `server/data/sample.json` via `npm run seed` or automatically on a fresh clone's first boot). The full `internal/store/storetest` conformance suite and the entire `internal/httpapi` integration suite pass against both stores. This step added the required Playwright E2E auth-flow spec (`npm run test:e2e`, `/e2e/auth.spec.ts`), a responsive QA pass at 375/768/1280px that found and fixed two real bugs (a nav item losing its accessible name once its label is visually hidden at the tablet breakpoint, and an undersized 36px follow-button touch target), and this documentation pass. Backend coverage **91.5 % total statements** (D-32 floor is 85 %); client suite **8/8** passing; E2E spec passing against the real stack. The Docker Compose bonus was added after Step 12 (D-70): `docker compose up --build` brings up the whole stack on a machine with only Docker installed — see [Runbook Option B](#option-b--docker-compose-one-command). This README describes the current, final setup — see commit history for how it got here.

---

## Table of Contents

- [Purpose & Rights Notice](#purpose--rights-notice)
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
- Docker Compose — the whole stack (`docker compose up --build`) with nothing installed but Docker (see [Runbook Option B](#option-b--docker-compose-one-command))

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
  /cmd/seed             Truncate-then-insert `server/data/sample.json` into the SQLite store (`npm run seed`)
  /internal/config      Typed configuration from env + a small .env loader
  /internal/httpapi     Handlers, middlewares, JSON responses (the HTTP layer)
  /internal/domain      Entity structs (User, Tweet, Follow, Like)
  /internal/validation  Pure validation rules (username, email, password, display name)
  /internal/service     Business rules, no HTTP imports — auth (Step 3), social/follows (Step 4), tweets/timeline/likes/search (Steps 5-6)
  /internal/store       Store interface + sample-data loader + conformance suite; store/memory (dev-only) and store/sqlite (default, D-66) both pass it
  /data/sample.json     Hand-authored sample dataset (12 users, tweets, follows, likes, replies)
  /data/twitter.db      SQLite database file — gitignored, created and seeded on first boot
/client                 React + Vite + TypeScript SPA
  /src/pages            Route-level views (Home, Login, Register, Profile, FollowList, Search, NotFound)
  /src/components       Reusable UI (AppShell, AuthLayout, Avatar, ComposeBox, TweetCard, TweetList, Protected/PublicOnlyRoute, icons)
  /src/context          AuthContext — resolves the session once from GET /api/auth/me
  /src/lib              Typed API client (api.ts), validation mirror (validation.ts, D-54), relative-time formatting (time.ts, D-30), infinite-scroll/tweet-feed hooks (useInfiniteScrollSentinel.ts, useTweetFeed.ts)
  /tests                Vitest + React Testing Library + MSW integration tests
/e2e                    Playwright E2E specs (auth.spec.ts, D-36) + playwright.config.ts
/.github/workflows      CI: Go (gofmt, vet, race tests, httpapi suite against sqlite too, coverage ≥ 85 %) + client (lint, build, tests) + docker (build both images, smoke-test the stack)
docker-compose.yml      Full-stack bonus (D-70): server + client services, named volume, healthcheck
server/Dockerfile       Two-stage build → static Go binary (+ seed tool) on alpine
client/Dockerfile       Two-stage build → Vite bundle served by nginx (client/nginx.conf proxies /api)
.dockerignore           Keeps host deps, build outputs, the local DB and secrets out of the build context
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

> The commands below reflect what exists **today** (Steps 1–11: scaffold, health endpoint, domain model + sample data, custom authentication, user profiles + the follow graph, tweets/timeline/likes, user search, the frontend app shell + auth UI, timeline/compose/like/delete, profile/follow/search pages, the bonus reply-threads feature, and the SQLite store that's now the default persistence backend). Later steps add to this section in the same commit that adds the feature.

There are two ways to run the app. **Option A** (local) is the primary path and needs Go + Node. **Option B** (Docker Compose) needs only Docker and is the one-command bonus (D-70).

### Prerequisites

**Option A — local:**

- **Go 1.23+** — https://go.dev/dl/
- **Node.js 20+** and npm 10+ (the frontend and the workspace scripts; CI runs on 20)
- Git

No database server: SQLite is a file created by the app.

**Option B — Docker Compose:**

- **Docker Engine 25+ / Docker Desktop 4.27+**, which includes Compose v2 (the stack uses `env_file: required: false`, Compose ≥ 2.24). Nothing else — no Go, Node, or database install.

### Option A — local (Go + Node)

```bash
git clone https://github.com/jdavidrt/theflock-twitter-clone.git
cd theflock-twitter-clone
cp .env.example .env          # defaults work as-is
npm install                   # installs the client workspace + root tooling
npm run dev                   # API on http://localhost:3000, client on http://localhost:5173
```

`npm run dev` runs `go run ./cmd/api` in `server/` and the Vite dev server in `client/` side by side (Go modules are downloaded on first run). The client calls the API through a relative `/api` path that Vite proxies to port 3000, so there is no CORS setup and no client-side env var.

### Option B — Docker Compose (one command)

```bash
git clone https://github.com/jdavidrt/theflock-twitter-clone.git
cd theflock-twitter-clone
docker compose up --build     # API on http://localhost:3000, client on http://localhost:5173
```

That is the whole setup — **no `.env`, no `npm install`, no seed step.** Compose builds two images (a static Go binary on `alpine`; the Vite build served by `nginx`, proxying `/api` to the API) and starts them; the client waits until the API's `/api/health` check passes. On the first boot the API creates and seeds its SQLite database, logging `seeded empty sqlite store … users=12 …`. Open http://localhost:5173 and log in with the [sample credentials](#sample-credentials); the `curl` examples below and `npm run test:e2e` work unchanged against the same URLs.

- **Data persists** on the named volume `sqlite-data`: tweets/follows/likes survive `docker compose down` and `up` (a later boot logs `opened sqlite store` and does not re-seed).
- **Reset to the sample dataset:** `docker compose exec server /app/seed`, or `docker compose down -v` (drops the volume) then `up` again.
- **Configuration:** a root `.env`, if present, is honored for `JWT_SECRET`, `APP_ENV`, and `COOKIE_SECURE` (e.g. set `APP_ENV=production` with a real `JWT_SECRET` for JSON logs and the production secret guard); with no `.env`, the stack boots under `APP_ENV=development` with the placeholder secret. The SQLite and sample-data paths are fixed to their in-container locations and ignore any `.env` values. Ports are fixed at `3000`/`5173`; stop `npm run dev` first to avoid a collision.
- **Stop:** `docker compose down` (keep data) or `docker compose down -v` (also delete the database volume).

Smoke check: `curl http://localhost:3000/api/health` → `{"status":"ok"}`. Open `http://localhost:5173` in a browser: an unauthenticated visit redirects to `/login`; register or log in (try the [sample credentials](#sample-credentials) below) and you land on `/` inside the app shell (bottom tab bar under 640px, an icon rail from 640px, a labeled sidebar with a centered content column from 1024px — D-28), with a logout action always reachable. `/login` and `/register` redirect an already-authenticated visitor back to `/`.

On a fresh clone, the API opens (creating if needed) `server/data/twitter.db` and, finding it empty, seeds it from `server/data/sample.json`, logging the counts (`seeded empty sqlite store store=sqlite path=./data/twitter.db users=12 tweets=169 follows=63 likes=499`). **This is the default, delivered persistence (`STORE=sqlite`, D-66): data created through the API — new tweets, follows, likes, profile edits — survives a restart.** A later boot against the same file logs `opened sqlite store` instead and does not re-seed. To reset to the sample dataset, run `npm run seed` (truncates then reinserts; refuses under `APP_ENV=production` unless `SEED_FORCE=true`), or delete `server/data/twitter.db*` and restart. Setting `STORE=memory` in `.env` switches to the earlier in-memory store instead (loads `SAMPLE_DATA_PATH` fresh at every boot; **restarting then resets all data back to the sample dataset**) — useful for quick local experimentation, not what ships. Auth endpoints are live: `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`. Profile and follow endpoints: `GET /api/users/{username}` (profile with counts + `isFollowedByMe`), `PATCH /api/users/me` (edit display name/bio), `POST`/`DELETE /api/users/{username}/follow` (idempotent follow/unfollow), `GET /api/users/{username}/followers` and `/following` (cursor-paginated). Tweet, timeline and like endpoints: `POST /api/tweets` (create), `GET`/`DELETE /api/tweets/{id}` (read as a D-48 thread page — `{ ancestors, tweet, replies }` — / author-only soft delete), `GET /api/users/{username}/tweets` (a user's tweets), `GET /api/timeline` (followed-users + own tweets, cursor-paginated), `POST`/`DELETE /api/tweets/{id}/like` (idempotent like/unlike). Reply threads (bonus, D-47…D-50): `POST /api/tweets/{id}/replies` (create a reply, 404 if the parent is missing or deleted); a deleted tweet inside a thread's ancestor chain still returns its data with `isDeleted: true` so the client can render the "this tweet was deleted" placeholder rather than breaking the chain. Search: `GET /api/search/users?q=` (case-insensitive substring on username or display name, capped at 20 results) — see [Sample credentials](#sample-credentials) to try all of the above against the seeded data.

### Running tests

```bash
npm test                   # Go suite (server) + client suite
npm run test:coverage      # Go coverage over server/internal — 91.5 % total statements as of Step 11 (floor: 85 %, D-32)
npm run lint               # ESLint (client) + go vet (server)
npm run format:check       # Prettier check (client + config files); CI also checks gofmt
npm run format             # Prettier --write + gofmt -w
npm run build              # go build → server/bin/, tsc + vite build → client/dist/
```

Server-only shortcuts from `server/`: `go test ./...`, `go test ./internal/... -coverpkg=./internal/... -coverprofile=coverage.out && go tool cover -func=coverage.out` (the `-coverpkg` flag is needed once cross-package test helpers exist, e.g. `internal/store/storetest` — see DECISIONS.md D-32). `go test ./...` alone only exercises `internal/httpapi` against the default in-memory store; run `HTTPAPI_TEST_STORE=sqlite go test ./internal/httpapi/...` to run the same integration suite against a real SQLite file instead (CI runs both — D-66).

Client-only shortcut from `client/`: `npx vitest` (watch mode). The suite renders the real `App` against a mocked API (MSW) and covers all three required frontend flows — `client/tests/integration/login.test.tsx` (login end to end — valid credentials land on `/` with the username visible in the nav, invalid credentials show the server's error message inline), `client/tests/integration/compose-tweet.test.tsx` (the live counter updates while typing, submitting posts a tweet and it appears at the top of the timeline, plus the 280-code-point submit-disable boundary), and `client/tests/integration/follow-flow.test.tsx` (clicking Follow on another user's profile flips the button to "Following" and increments the follower count; the reverse for Unfollow) — plus the bonus reply-threads flow in `client/tests/integration/thread.test.tsx` (the thread page renders the ancestor chain, focused tweet and replies, posting a reply appends it, and a deleted ancestor renders the D-49 placeholder).

**End-to-end (Playwright, D-36):** `npm run test:e2e` runs `/e2e/auth.spec.ts` against an **already-running** `npm run dev` at `http://localhost:5173` — it does not start the app itself, so start the dev server first. The spec registers a unique `e2e_<timestamp>` user (rerunnable without resetting the database) and drives register → land on the timeline with the username visible in the nav → log out → redirected back to `/login` → log back in with the same credentials. Not run in CI (D-37) since it needs the full stack up.

### Environment variables

Copy [`.env.example`](.env.example) to `.env` at the repository root. The API looks for `.env` in its working directory and then in the parent, so it is found whether you run from the root or from `server/`. Variables already set in the environment always win over the file.

| Variable           | Default / example                       | Description                                                                                                                                                                          |
| ------------------ | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `PORT`             | `3000`                                  | API listen port                                                                                                                                                                      |
| `APP_ENV`          | `development`                           | `development` / `test` / `production`. `test` lowers bcrypt cost, disables rate limiting and silences logs                                                                           |
| `JWT_SECRET`       | `change-me-to-a-random-32+-char-string` | Secret for signing session tokens (≥ 32 chars). The placeholder is refused when `APP_ENV=production`                                                                                 |
| `COOKIE_SECURE`    | `false`                                 | Set `true` only when serving over HTTPS                                                                                                                                              |
| `STORE`            | `sqlite`                                | Persistence backend. `sqlite` (default) persists to `SQLITE_PATH`, seeding from `SAMPLE_DATA_PATH` only on first boot; `memory` loads `SAMPLE_DATA_PATH` fresh at every boot instead |
| `SAMPLE_DATA_PATH` | `./data/sample.json`                    | Sample dataset the memory store (always) and the sqlite store (first boot only) load, relative to `server/`                                                                          |
| `SQLITE_PATH`      | `./data/twitter.db`                     | SQLite database file, relative to `server/`; gitignored; created on first boot                                                                                                       |
| `SEED_FORCE`       | `false`                                 | Lets `npm run seed` (`cmd/seed`) run under `APP_ENV=production`                                                                                                                      |

The frontend needs no environment variables.

### Sample credentials

`server/data/sample.json` contains 12 users sharing the password `Password123!`, led by the fixed sample account `alice@example.com` (username `alice`). Confirmed against a **freshly seeded** API — a fresh clone's first `npm run dev`, or after `npm run seed` (see the restart note above; on a database that already has data, restarting no longer resets it):

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
- **In-memory store kept as a dev-only option:** `STORE=memory` still works (loads `server/data/sample.json` fresh at every boot, writes live only in process memory, a restart resets to the sample) — useful for quick local experimentation without touching the SQLite file, but `STORE=sqlite` is the default and the delivered behavior (D-66).
- **Single-writer SQLite:** the store caps its connection pool at one (`SetMaxOpenConns(1)`), matching D-64's "one writer at a time is fine at this scale" — avoids `SQLITE_BUSY` without a retry loop; a multi-instance deployment would need a server database.
- **Client-side validation is a UX mirror, not the source of truth (D-54):** `client/src/lib/validation.ts` pins the same length bounds, username regex and code-point counting as the server so bad input is caught before a round trip, but business rules like the reserved-username list are checked server-side only — the client just displays whatever `details` the server returns.
- **Docker Compose behind nginx shares one auth-rate-limit bucket (D-70):** the auth limiter keys on the request's remote address, which in Compose is the nginx container, so all browsers share one 20-request / 15-min login+register budget. Fine for a single evaluator; honoring `X-Forwarded-For` would be a trust-boundary change beyond a packaging bonus. Related: the SQLite store is single-writer on one named volume, so there is one `server` container per stack (no `--scale`), and Docker is for running the delivered app — dev still uses `npm run dev` on the host, not hot-reload containers.
- **No image uploads, real-time updates, or notifications** in this delivery — deferred deliberately to keep the required features solid within 72 hours. See [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md) §8 for the full out-of-scope list and rationale.
- **E2E scope is one spec, by design (D-36):** `e2e/auth.spec.ts` covers the required real-browser auth flow only; compose/like/delete/follow/search/threads are exercised by the Vitest+MSW integration suite instead (faster, no flakiness from a real backend), not duplicated as browser E2E.
- **React Query default retry policy:** the client only retries a failed request when the error isn't a 4xx (client errors — not-found, validation, forbidden — are never transient). Found during Step 12's responsive QA: visiting an unknown username left the profile page on "Loading profile…" for ~7 seconds (three retries with backoff) before showing "User not found" under the library's out-of-the-box default of retrying every error including 404s.
- This section will grow as real implementation trade-offs are made.

---

## AI Tooling Usage

This project was built with Claude Code throughout — planning, scaffolding, feature implementation, and test writing. Every prompt used and the work it produced is logged verbatim in [my-docs/AGENT-COLLABORATION-HISTORY.MD](my-docs/AGENT-COLLABORATION-HISTORY.MD), and the requirement analysis that preceded implementation is in [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md). Project-specific operating instructions for AI sessions live in [CLAUDE.md](CLAUDE.md), including a working rule that the agent edits plain-text and config files directly rather than via generated scripts — kept there because it's an agent-operating rule, not part of the delivered app.
