# The Flock — Twitter Clone

A full-stack Twitter/X clone built for The Flock's technical challenge: custom authentication, tweets, a followed-users timeline, likes, follows, search, reply threads, and a mobile-first responsive UI — delivered with a Docker Compose one-command startup.

> **Project status:** planning/scaffolding stage. This README describes the target setup; the Runbook below will be kept accurate as implementation lands (see commit history). Sections not yet implemented are marked **Planned**.

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
- User profile: unique username, bio, avatar placeholder
- Create / delete tweets (280-char limit, enforced client- and server-side)
- Timeline of followed users, chronological, paginated (infinite scroll)
- Follow / unfollow, like / unlike, visible like counts, followers/following lists
- Basic user search by name/username
- Fully responsive, mobile-first UI (breakpoints: <640px, 640–1024px, >1024px)

**Bonus (selected for this delivery):**
- Reply threads (self-referential tweets with thread view)
- Docker Compose — full stack up with one command

**Explicitly out of scope for this delivery:** real-time updates, image uploads, notifications. See [Known Trade-offs & Limitations](#known-trade-offs--limitations).

---

## Technology Stack & Justification

| Layer | Choice | Why |
|---|---|---|
| Backend | Node.js 20 (LTS) + TypeScript + Express | Mature, minimal-ceremony framework; TypeScript gives compile-time safety across the whole stack without the overhead of a heavier framework, appropriate for a 72h scope. |
| ORM / DB | Prisma + PostgreSQL 16 | Relational DB required by the brief; Prisma gives fast, reviewable migrations and a type-safe query layer, and its seeding tooling maps directly onto the "must ship realistic seed data" requirement. |
| Frontend | React 18 + Vite + TypeScript | Fast dev server, large ecosystem, and the framework most AI coding tools generate reliably — directly relevant to a challenge that grades how well the tools are directed. |
| Styling | Tailwind CSS | Utility classes make mobile-first breakpoints (the brief's explicit requirement) fast and explicit in markup, rather than fighting a separate stylesheet. |
| Data fetching | TanStack Query | Caching, pagination, and refetch semantics needed for a paginated timeline without hand-rolling loading/error state. |
| Auth | Custom bcrypt + signed httpOnly JWT cookie | The brief explicitly forbids third-party auth providers; this is the minimal correct custom implementation (hashed passwords, signed session token, server-validated on every protected request). |
| Backend tests | Vitest + Supertest | Fast unit tests plus real HTTP-level integration tests against a running Express app and a real Postgres test database. |
| Frontend tests | Vitest + React Testing Library | Integration-style tests for the flows the brief calls out: login, create tweet, follow. |
| E2E | Playwright | Real browser test of the full auth flow, as required. |
| Containerization | Docker + Docker Compose | Bonus feature; also the most reliable way to guarantee the mandatory Runbook works with zero manual intervention. |

The stack itself isn't graded — see [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md) for the full requirement-by-requirement validation this stack was chosen against.

---

## Architecture

**Planned repository layout:**

```
/server              Express + TypeScript API
  /prisma            Schema, migrations, seed script
  /src
    /routes          Express routers per resource (auth, users, tweets, follows, likes, search)
    /services        Business logic, separate from HTTP layer
    /middleware       Auth guard, error handling
  /tests             Unit + integration tests
/client              React + Vite + TypeScript SPA
  /src
    /pages           Route-level views
    /components      Reusable UI
    /api             TanStack Query hooks per resource
  /tests             Frontend integration tests
/e2e                 Playwright E2E specs
docker-compose.yml   postgres + server + client, one command
.env.example
my-docs/             Internal planning docs (requirements validation, AI collaboration log)
```

### Data model (timeline & follows graph)

- **users**: `id, username (unique, lowercase), email (unique, lowercase), display_name, password_hash, bio, created_at, updated_at` — no avatar column; the avatar placeholder is rendered client-side from the user's initials.
- **follows**: `follower_id, followee_id, created_at` (composite PK, both FKs to `users`, indexed both directions, self-follow blocked by a `CHECK` constraint)
- **tweets**: `id, author_id (FK users), content (≤280 code points), parent_tweet_id, created_at, deleted_at` — `parent_tweet_id` is a nullable self-referential FK (reply threads; `NULL` = top-level). Deletes are soft (`deleted_at`) so threads stay intact when a parent is removed.
- **likes**: `user_id, tweet_id, created_at` (composite PK, both FKs)

**Timeline query:** top-level, non-deleted tweets authored by `me` or by anyone `me` follows, ordered `created_at DESC, id DESC`, cursor-paginated on `(created_at, id)` for stable infinite scroll under concurrent writes. All counts (likes, replies, followers, following) are computed on read.

*(Planned — will be reflected in `server/prisma/schema.prisma` once implemented.)*

---

## Runbook (Setup & Operations)

> **Planned.** The commands below are the target Runbook and will be validated end-to-end as implementation lands; this notice will be removed once verified.

### Prerequisites

- Node.js **20.x** (LTS) and npm **10.x**
- PostgreSQL **16.x** (or Docker, see below — no local Postgres install needed if using Docker)
- Docker **25+** and Docker Compose (for the one-command bonus path)
- Git

### Option A — Docker Compose (recommended, single command)

```bash
git clone https://github.com/jdavidrt/theflock-twitter-clone.git
cd theflock-twitter-clone
cp .env.example .env
docker compose up --build
```

This starts PostgreSQL (host port 5433), the API, and the frontend, runs migrations, and seeds the database on first start. App will be available at `http://localhost:5173` (frontend) and `http://localhost:3000` (API).

### Option B — Local (without Docker)

```bash
git clone https://github.com/jdavidrt/theflock-twitter-clone.git
cd theflock-twitter-clone
cp .env.example .env
npm install
npm run db:migrate
npm run seed
npm run dev
```

### Running tests

```bash
npm test              # backend + frontend unit/integration tests
npm run test:coverage # coverage report (target: 85%+ on server/)
npm run test:e2e      # Playwright E2E suite (requires the app running)
```

### Environment variables

Full list will live in [`.env.example`](.env.example) (to be added with the scaffolding):

| Variable | Example | Description |
|---|---|---|
| `DATABASE_URL` | `postgresql://twitter:twitter@localhost:5433/twitter` | PostgreSQL connection string used by the API |
| `JWT_SECRET` | `change-me-to-a-random-32+-char-string` | Secret for signing session tokens (≥ 32 chars) |
| `PORT` | `3000` | API port |
| `NODE_ENV` | `development` | `development` / `test` / `production` |
| `COOKIE_SECURE` | `false` | Set `true` only when serving over HTTPS |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `twitter` | Used by Docker Compose for the database container |

The frontend needs no environment variables — it calls the API via a relative `/api` path that Vite (dev) or nginx (Docker) proxies to the server.

### Sample credentials

**Planned.** The seed will create a fixed sample account — `alice@example.com` / `Password123!` — plus 11 more users sharing the same password. This line will be confirmed once the seed script lands.

---

## Technical Decisions

- **Why this stack:** see [Technology Stack & Justification](#technology-stack--justification) above.
- **Timeline & follows graph modeling:** see [Architecture](#architecture) above.
- **Authentication:** custom-built per the brief's requirement (no third-party auth). Passwords hashed with bcrypt; sessions are a signed httpOnly JWT cookie validated by Express middleware on every protected route. No refresh-token rotation in this delivery — see trade-offs below.
- **Known trade-offs and limitations:** see next section.
- **AI tools used:** see [AI Tooling Usage](#ai-tooling-usage) below.

---

## Known Trade-offs & Limitations

- **Session strategy:** a single httpOnly JWT cookie rather than an access+refresh token pair. Simpler and sufficient for this scope; means sessions can't be selectively revoked per-device without additional work.
- **Counts on read:** like/reply/follower counts are `COUNT()`ed per request instead of stored — correct by construction at this scale, would need denormalizing for a large dataset.
- **Private by default:** every page and endpoint except register/login requires a session; there is no logged-out public browsing of profiles or tweets.
- **Soft deletes:** deleted tweets keep their row (`deleted_at`) so reply threads don't lose their parent; they never appear in feeds or counts.
- **No image uploads, real-time updates, or notifications** in this delivery — deferred deliberately to keep the required features solid within 72 hours. See [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md) §8 for the full out-of-scope list and rationale.
- This section will grow as real implementation trade-offs are made.

---

## AI Tooling Usage

This project was built with Claude Code throughout — planning, scaffolding, feature implementation, and test writing. Every prompt used and the work it produced is logged verbatim in [my-docs/AGENT-COLLABORATION-HISTORY.MD](my-docs/AGENT-COLLABORATION-HISTORY.MD), and the requirement analysis that preceded implementation is in [my-docs/VALIDATION-OF-REQUIREMENTS.md](my-docs/VALIDATION-OF-REQUIREMENTS.md). Project-specific operating instructions for AI sessions live in [CLAUDE.md](CLAUDE.md).
