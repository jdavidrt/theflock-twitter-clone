# Implementation Plan — 12 Steps / 12 Prompts

**Status:** v2.0 — 2026-09-16 (**re-planned for the Go + SQLite pivot** — DECISIONS D-65/D-66/D-67. v1.x history: v1.2 moved Docker Compose to the backlog (D-63) and put the MVP on SQLite via Prisma (D-64); the Node/Express work done under v1.x was reverted in commit `c9336b3` and the plan restarts at Step 1.)
**Deadline:** 2026-09-18
**Spec:** [VALIDATION-OF-REQUIREMENTS.md](./VALIDATION-OF-REQUIREMENTS.md) · **Detailed specs:** [DECISIONS.md](./DECISIONS.md) · **Agent rules:** [CLAUDE.md](../CLAUDE.md)

Every prompt below implicitly starts with: _"Follow `my-docs/DECISIONS.md` for all behavioral details (D-xx references); where this prompt and DECISIONS.md differ, DECISIONS.md wins."_

Each step is one Claude Code session/prompt and should end in one or more small, coherent commits (made by the user in VS Code — Claude proposes the boundaries, D-61). The order is deliberate: it produces the commit story the brief says evaluators look for (§7.3 — scaffolding first, features one by one with tests alongside, bonus, then polish/docs) and, per the stakeholder's instruction, gets the **data model and sample data in place first** so every later step has real data to work with.

Every step's prompt ends with the same two standing instructions, so they're not repeated in each box:

- _"Append an entry to `my-docs/AGENT-COLLABORATION-HISTORY.MD` for this prompt."_
- _"Call out commit boundaries as you go; don't leave everything for one commit at the end."_

Rough budget: ~24 working hours of the remaining window. Steps 1–6 (Go backend on the in-memory store) by end of day 1 (2026-09-16); 7–10 (frontend + bonus) by end of day 2; 11 (SQLite) and 12 (E2E + polish) on day 3 with margin. **Step 11 is not optional** — the brief requires a relational database (D-66).

## The two phases (D-66)

|                             | Phase 1 — Steps 2–10                                                                                  | Phase 2 — Step 11 onward                                                                                                      |
| --------------------------- | ----------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Store                       | `store/memory`, loaded from `server/data/sample.json` at boot; writes are in-process, restart = reset | `store/sqlite` (`database/sql` + `modernc.org/sqlite`), schema from embedded `schema.sql`, seeded from the same `sample.json` |
| `STORE` default             | `memory`                                                                                              | `sqlite`                                                                                                                      |
| Tests                       | Go integration tests on the memory store                                                              | The same tests on both stores + one conformance suite                                                                         |
| What changes between phases | Nothing in handlers, services, validation, or the client                                              | —                                                                                                                             |

## Progress

| Step                                                                | Status                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Log                                                 |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------- |
| 0 — Pivot: docs + re-scaffold                                       | ✅ Done 2026-09-16                                                                | [History Entry 9](./AGENT-COLLABORATION-HISTORY.MD) |
| 1 — Scaffolding (Go + React)                                        | ✅ Done 2026-09-16                                                                | Entry 9                                             |
| 2 — Domain model + sample data + memory store                       | ✅ Done 2026-09-16 — coverage 93 %+                                               | This entry                                          |
| 3 — Custom authentication                                           | ✅ Done 2026-09-17 — coverage 93.5 %+                                             | This entry                                          |
| 4 — User profiles + follow graph                                    | ✅ Done 2026-09-17 — coverage 93.6 %+                                             | This entry                                          |
| 5 — Tweets, timeline, likes                                         | ✅ Done 2026-09-18 — coverage 93.1 %+                                             | This entry                                          |
| 6 — User search + coverage checkpoint                               | ✅ Done 2026-09-18 — coverage 93.4 %, recorded in README                          | This entry                                          |
| 7 — Frontend foundation + auth UI + first frontend integration test | ✅ Done 2026-09-18 — register→login→logout verified at 375/768/1280px            | This entry                                          |
| 8 — Timeline, compose tweet, delete, like (frontend + test)         | ✅ Done 2026-09-18 — client suite 4/4                                             | This entry                                          |
| 9 — Profile pages, follow/unfollow, search (frontend + test)        | ✅ Done 2026-09-18 — client suite 6/6                                             | This entry                                          |
| 10 — Bonus: reply threads (backend + frontend + tests)              | ✅ Done 2026-09-18 — coverage 93.2 %, client suite 8/8                            | This entry                                          |
| 11 — SQLite store, seed command, switch the default                 | ✅ Done 2026-09-18 — coverage 91.5 %, `storetest` + `httpapi` suites green on both stores | This entry                                          |
| 12 — E2E, responsive QA, coverage/docs                              | ✅ Done 2026-09-18 — Playwright auth spec passing, 2 responsive bugs fixed, coverage 91.5 % | This entry                                          |

All 12 steps are committed on `main` across five commits (`42bcbce` … `28372df`).

Pre-pivot history (for the record): under v1.x, Step 1 (Node scaffold) was committed and Step 2 (Prisma on SQLite + Vitest infra) was completed and verified locally but never committed; both were reverted by the user in `c9336b3` when the pivot was decided. Entries 4–8 in the history log document that work.

---

## Step 1 — Scaffolding: Go API + React client + tooling + CI

**Goal:** the "first commit: project scaffolding" the brief expects, with both apps bootable, lint/format wired, and CI green from the first push.

**Prompt:**

> Scaffold the project per the layout in CLAUDE.md. `server/`: a Go module (`github.com/jdavidrt/theflock-twitter-clone/server`, `go 1.23`) with `cmd/api/main.go` (loads the root `.env` via `internal/config`, builds the handler, `http.Server` with sane timeouts and graceful shutdown on SIGINT/SIGTERM), `internal/config` (typed `Config` from env — `PORT`, `APP_ENV`, `JWT_SECRET` ≥ 32 chars, `COOKIE_SECURE` — plus the small dotenv loader; D-43), and `internal/httpapi` (`NewHandler` on the Go 1.22 `http.ServeMux`, `GET /api/health` → `200 {"status":"ok"}`, a JSON 404 for unknown `/api/*` routes in the D-52 envelope, and middlewares: recover → 500, request log via `log/slog`, security headers, 16 kB body limit — D-55/D-57). Tests with `testing` + `httptest` for config parsing, the dotenv loader, health, 404 envelope, recover and body limit. `client/`: Vite + React 18 + TypeScript + plain hand-written CSS (mobile-first, D-68) + React Router + TanStack Query with a placeholder home page and a Vite dev proxy `/api` → `http://localhost:3000` (D-42). Root: `package.json` with the `client` workspace and scripts `dev` (concurrently: `cd server && go run ./cmd/api` + Vite), `build`, `lint` (ESLint + `go vet`), `format`/`format:check` (Prettier + `gofmt`), `test`, `test:coverage` (D-32/D-45); `.nvmrc`, `.editorconfig`, `.gitattributes` (`* text=auto eol=lf`), ESLint + Prettier config, `.env.example` with exactly the Step 1 variables from D-43. `.github/workflows/ci.yml` per D-37 (server: setup-go, gofmt check, vet, `go test -race` + coverage ≥ 85 %; client: `npm ci`, lint, format check, build, test). README Runbook updated to the real commands.

**Done when:** `cp .env.example .env && npm install && npm run dev` serves the client at 5173 and `curl localhost:3000/api/health` returns 200; `npm test` and `npm run lint` are green; CI passes. Two-plus commits on `main` (tooling / server / client / CI).

**Rubric:** Development process, Code quality.

_As executed (Entry 9, 2026-09-16):_ all files written during the pivot session. Client verified (`npm install`, lint, build). Go side wasn't compiled yet (Go not installed on the dev machine at the time).

---

## Step 2 — Domain model, sample data, `Store` interface, in-memory store

**Goal:** the whole data model and a realistic dataset exist before any feature — the stakeholder's "very first step" — so every later endpoint is built and tested against real data.

**Prompt:**

> In `server/`, add `internal/domain` with the entity structs from VALIDATION-OF-REQUIREMENTS.md §7 (`User` with `DisplayName`, `Bio`, `PasswordHash`, no avatar — D-12; `Follow` keyed by `(FollowerID, FolloweeID)`; `Tweet` with `Content`, nullable `ParentTweetID` and `DeletedAt` — D-16/D-47; `Like` keyed by `(UserID, TweetID)`), UUID string ids (D-51), `time.Time` UTC timestamps, explicit camelCase `json` tags. Define `internal/store.Store` (D-66): the interface every service will need — users (create, get by id/username/email, update profile, search), follows (create/delete idempotently, list followers/following with cursor, counts, `IsFollowing`), tweets (create, get, soft-delete, list by author, timeline for a viewer, replies of a tweet, ancestors), likes (create/delete idempotently, count, `LikedBy`), and the cursor type from D-19 (`createdAt|id`, base64url). Sentinel errors (`ErrNotFound`, `ErrConflict`, `ErrSelfFollow`). Write `server/data/sample.json` per D-67 (12 users starting with alice, plain `password` fields, fixed UUIDs and RFC 3339 timestamps, 8–20 top-level tweets each, follows 3–8 each with alice ≥ 6, cross-likes, ~30 replies some nested) and `internal/store/sample` — a loader that parses + validates the file (uniqueness, references, no self-follow, D-13 length) and hashes each distinct password once with bcrypt at the configured cost. Implement `internal/store/memory` (maps + `sync.RWMutex`, every list ordered per D-17/D-19/D-24/D-48, soft-deleted rows excluded everywhere). Add `internal/store/storetest`: a conformance suite (`func Run(t *testing.T, newStore func(t *testing.T) store.Store)`) covering every interface method including pagination stability and soft-delete exclusion, and run it from `memory`. Unit-test the loader against the real `sample.json`. Wire `STORE` (default `memory`) and `SAMPLE_DATA_PATH` (default `./data/sample.json`) into `internal/config`, `.env.example`, D-43's table and the README; `cmd/api` builds the store at boot and logs the counts loaded. Add a temporary read-only `GET /api/health` detail or a log line showing the loaded counts — no feature endpoints yet.

**Done when:** `npm run dev` logs "loaded 12 users, N tweets, N follows, N likes"; `go test ./...` runs the conformance suite green on the memory store; coverage stays ≥ 85 %. Commits: domain + store interface → sample data + loader → memory store + conformance suite.

**Rubric:** Code quality, Testing, Seed requirement (§5.2).

_As executed (2026-09-16):_ Added `internal/domain` (User/Follow/Tweet/Like structs), `internal/store` (the `Store` interface, sentinel errors, `Cursor`/`Page[T]`), `internal/store/sample` (loader: parses + validates `sample.json`, hashes each distinct password once, topologically orders replies after parents), `internal/store/memory` (maps + `sync.RWMutex`), and `internal/store/storetest` (the conformance suite). `server/data/sample.json` generated via a throwaway script (12 users, 169 tweets, 63 follows, 499 likes). One deliberate interface decision: `Ancestors` does not exclude soft-deleted tweets like every other list method does — needed later so a deleted tweet still renders as a placeholder in a reply thread instead of breaking the chain (D-49); `storetest` covers both halves. `STORE`/`SAMPLE_DATA_PATH` wired into config/`.env.example`; `cmd/api` loads the store and logs counts at boot (no HTTP endpoints yet). Bug found: `storetest` has no `_test.go` of its own, so coverage tooling attributed it 0 % and total coverage read 44 % instead of 93 % until `-coverpkg=./internal/...` was added (documented as a D-32 amendment). Verified: `go build`/`go vet`/`gofmt` clean, full suite green, coverage 93.2 %.

---

## Step 3 — Custom authentication (backend + tests)

**Goal:** the hard requirement the brief singles out — fully custom auth, no third-party providers.

**Prompt:**

> Implement custom authentication in `server/`: `POST /api/auth/register` (email, username, password, optional displayName — rules per D-01…D-05 including lowercase normalization and the reserved-username list; bcrypt cost 12, cost 4 when `APP_ENV=test` — D-06), `POST /api/auth/login` (email only, generic 401 on failure — D-07/D-08), `POST /api/auth/logout`, `GET /api/auth/me`. Session = HS256 JWT (`golang-jwt/jwt/v5`) in an httpOnly `token` cookie, SameSite=Lax, 7-day expiry, `Secure` from `COOKIE_SECURE` (D-09). Add `internal/validation` (pure functions, per-field errors → D-52 `details`; D-54), `internal/service/auth`, the `requireAuth` middleware that puts the user id in the request context, a central `writeError` producing the D-52 envelope for every error path, a per-IP `x/time/rate` limiter on login/register (D-55, off when `APP_ENV=test`), and JSON-only body enforcement on state-changing routes (D-56). Layering per D-53. Tests: unit tests for the validation rules (username regex/bounds, reserved list, email normalization, password bounds), the code-point counter with a multi-byte emoji case (D-13), JWT sign/verify round-trip and expiry; `httptest` integration tests on the memory store covering successful register/login/logout/me, duplicate email/username → 409, reserved username → 400, wrong password → 401, protected route without cookie → 401, malformed JSON / wrong content-type → 400, cookie attributes.

**Done when:** all auth tests pass; `go test ./internal/... -cover` is visible and ≥ 85 %.

**Rubric:** Functionality (auth), Testing, Code quality.

_As executed (2026-09-17):_ Added `internal/validation` (Username/Email/Password/DisplayName rules, D-54) and `internal/service/auth` (Register/Login/IssueToken/VerifyToken; bcrypt cost + JWT secret from config; `VerifyToken` explicitly rejects `alg=none`). `internal/httpapi` gained `auth.go` (register/login/logout/me handlers), `cookie.go` (D-09 cookie shape), `context.go` (`requireAuth` middleware), `decode.go`, `ratelimit.go` (per-IP limiter on login/register, off in test env), and a `jsonOnly` middleware (D-56). Dependency note: pinned `golang.org/x/time@v0.9.0` and kept `go 1.23.0` in `go.mod` — a newer version would have forced a toolchain bump past the D-65 floor. Tests cover validation rules, auth service (register/login/JWT edge cases), and the full HTTP flow (register→me→logout→me, duplicate email/username → 409, reserved username → 400, wrong password → generic 401, missing cookie → 401, malformed JSON/content-type → 400, rate-limit exhaustion). Verified: `go build`/`go vet`/`gofmt` clean, full suite green, coverage 93.5 %.

---

## Step 4 — User profiles + follow graph (backend + tests)

**Prompt:**

> Add user and follow endpoints: `GET /api/users/{username}` (profile with tweet/follower/following counts computed on read — D-23 — and `isFollowedByMe`), `PATCH /api/users/me` (displayName 1–50, bio ≤ 160 — D-12), `POST`/`DELETE /api/users/{username}/follow` (idempotent, self-follow → 400 — D-21), `GET /api/users/{username}/followers` and `/following` (cursor-paginated per D-19, newest first — D-24). No avatar field (D-12). Integration tests for each endpoint including: follow twice is idempotent, unfollow when not following is a no-op, self-follow rejected, pagination cursor returns no duplicates/gaps across the sample data, unknown username → 404.

**Done when:** tests green; the follow graph can be built and queried entirely via the API on top of the sample data.

**Rubric:** Functionality (profile, follows, followers lists), Testing.

_As executed (2026-09-17):_ Added `internal/service/social` (GetProfile/UpdateProfile/Follow/Unfollow/ListFollowers/ListFollowing; counts computed on read per D-23; `ErrSelfFollow` checked before the store per D-21) and `validation.Bio`. `internal/httpapi` gained `users.go` (profile/follow/follow-list handlers, `profileResponse` deliberately omits email) and `pagination.go` (shared `parseCursorAndLimit`, reused by every cursor-paginated endpoint from here on). Routes behind `requireAuth`: `GET /api/users/{username}`, `PATCH /api/users/me`, `POST`/`DELETE /api/users/{username}/follow`, followers/following lists. Tests cover profile counts/follow-state, follow idempotency, self-follow rejection, unfollow no-op, unknown-username → 404, and a pagination test walking alice's real 11 sample followers a page at a time to confirm no duplicates/gaps. Mid-session: `IMPLEMENTATION-PLAN.md` was moved from `my-docs/` to the repo root (outside the gitignore) so the build plan ships publicly; the other `my-docs/` files stay gitignored. Verified: `gofmt`/`go build`/`go vet` clean, full suite green across 5 repeated runs, coverage 93.6 %.

---

## Step 5 — Tweets, timeline, likes (backend + tests)

**Prompt:**

> Add tweet endpoints: `POST /api/tweets` (content 1–280 code points, trimmed, newlines preserved — D-13/D-14), `DELETE /api/tweets/{id}` (author only → 403 otherwise; **soft delete** via `deletedAt`, excluded by every store list method — D-16), `GET /api/tweets/{id}` (404 if deleted), `GET /api/users/{username}/tweets` (top-level, not deleted, cursor-paginated). Timeline: `GET /api/timeline` returns top-level tweets from followed users plus the viewer's own (D-17), `createdAt DESC, id DESC`, cursor-paginated per D-19, each tweet including an author summary, `likeCount`, `replyCount`, and `likedByMe`. Likes: `POST`/`DELETE /api/tweets/{id}/like` (idempotent, own tweets allowed, deleted tweet → 404 — D-22). Counts on read (D-23); note that choice in README trade-offs. Integration tests: 281-code-point rejected (multi-byte emoji case), empty rejected, delete by non-author → 403, deleted tweet disappears from timeline/profile and returns 404 by id, timeline excludes non-followed users' tweets and excludes replies, timeline ordering, pagination has no duplicates/gaps, like twice counts once, unlike removes. Use alice's sample timeline as a fixture where it makes assertions concrete.

**Done when:** tests green; a timeline can be produced end to end via the API.

**Rubric:** Functionality (tweets, timeline, pagination, likes, counter), Testing.

_As executed (2026-09-18):_ Store-level tweet/like methods already existed from Step 2, so this step was the service and HTTP layers. Added `internal/service/tweet` (Create/Get/Delete/ListByUsername/Timeline/Like/Unlike; `ErrForbidden` on non-author delete) and `validation.TweetContent` (1–280 code points, trimmed, newlines preserved — D-13/D-14). `internal/httpapi/tweets.go` gained the seven tweet/timeline/like routes, all behind `requireAuth`. Tests cover content validation (including a multi-byte emoji boundary case), soft-delete exclusion, non-author delete → 403, timeline include/exclude by follow status, like-twice-counts-once. One pagination test loops `GET /api/timeline` at `limit=50` and `limit=3` to exhaustion and asserts the id sequences match against alice's ~99-tweet real timeline. Verified: `gofmt`/`go build`/`go vet` clean, full suite green, coverage 93.1 % (down slightly from 93.6 %, expected — Step 6 is the dedicated coverage checkpoint).

---

## Step 6 — User search + backend coverage checkpoint

**Prompt:**

> Add `GET /api/search/users?q=` per D-25 — case-insensitive substring on username or displayName, `q` trimmed and 1–50 chars (empty → 400), limit 20, ordered by username — with tests (including a mixed-case query against the sample data). Then run `npm run test:coverage`, report the total, and add tests for any uncovered service/handler/store branches until `server/internal` meets the D-32 threshold. Confirm the README Runbook's sample credentials (`alice@example.com` / `Password123!`) work against the running API and note there that Phase-1 data resets on restart (D-66).

**Done when:** API returns a populated timeline for alice on a fresh start; coverage ≥ 85 % and recorded in the README.

**Rubric:** Functionality (search), Seed requirement, Testing (85 %+), Documentation.

_As executed (2026-09-18):_ Added `validation.SearchQuery` (1–50 code points, D-25) and `social.Service.SearchUsers` (case-insensitive substring on username/display name, capped at 20, no pagination). `handleSearchUsers` wired at `GET /api/search/users?q=` behind `requireAuth`. Tests: query validation, substring match, the mixed-case `q=aR` case against real sample data (matches carol/oscar), empty query → 400. Coverage checkpoint: closed two partial-coverage gaps from Step 5 (`writeTweetError`, `handleTimeline`) with a store-error test and an invalid-cursor test. Final: 93.4 % total statements. Confirmed sample credentials and restart-resets-to-sample behavior against a freshly started API. Updated README's status banner, Runbook, and sample-credentials section.

**Outcome:** Files created: none (search extends the existing `social` service/`users.go` handler rather than a new package). Files changed: `server/internal/validation/{validation.go,validation_test.go}` (added `SearchQuery`), `server/internal/service/social/{social.go,social_test.go}` (added `SearchUsers`, `SearchResultLimit`), `server/internal/httpapi/{users.go,users_test.go,handler.go}` (search handler/route), `server/internal/httpapi/tweets_test.go` (two coverage-closing tests), `README.md`, `IMPLEMENTATION-PLAN.md`, `my-docs/AGENT-COLLABORATION-HISTORY.MD`. No commits made (user commits manually); suggested boundaries: (1) `feat(server): add user search endpoint`, (2) `test: close Step 5 coverage gaps (tweet service error path, timeline validation)`, (3) `docs: record Step 6 completion, coverage 93.4%`.

---

## Step 7 — Frontend foundation + auth UI + first frontend integration test

**Goal:** mobile-first app shell and the full auth loop in the browser.

**Prompt:**

> In `client/`, build the mobile-first app shell per D-28: bottom tab bar on `<640px`, icon rail at `sm:`, labeled sidebar + `max-w-[600px]` content column at `lg:`. Set up a typed API client (relative `/api`, `credentials: 'include'`, unwraps the D-52 error envelope), TanStack Query provider, an `Avatar` component rendering initials on a username-hashed color (D-12), and an auth context backed by `GET /api/auth/me`. Pages: `/register` (with client-only confirm-password), `/login`, a `ProtectedRoute` wrapper that redirects to `/login`, and redirect-away-if-authenticated on the two public pages (D-11). Forms with client-side validation mirroring the server rules via `client/src/lib/validation.ts` (D-54) and server error display. Logout button in the nav. Configure Vitest + React Testing Library + MSW for `client/`, and write the login-flow integration test: renders form, submits valid credentials, mocked API returns user, app navigates to `/` and shows the username in the nav; plus an invalid-credentials error case.

**Done when:** register → login → logout works against the real Go API in the browser at all three breakpoints; client test suite runs green.

**Rubric:** Functionality (auth), Responsive design, Testing (frontend).

_As executed (2026-09-18):_ Added `client/src/lib/api.ts` (typed fetch wrapper, unwraps the D-52 error envelope) and `validation.ts` (D-54 mirror of server-side rules; the reserved-username list stays server-only). `AuthContext` resolves the session once from `GET /api/auth/me`. Components: `ProtectedRoute`/`PublicOnlyRoute` (D-11), `AppShell` (D-28 responsive nav — bottom tab bar → icon rail → labeled sidebar), `Avatar` (initials on a hashed color, no image dependency), `AuthLayout`, `icons.tsx` (inline SVGs instead of an icon library). Pages: `Login`, `Register` (client-only confirm-password), `NotFound`. Test infra: Vitest + RTL + MSW (`client/tests/`), `integration/login.test.tsx` covering valid and invalid credentials. Two real bugs caught only by driving a real browser: (1) `POST /api/auth/logout` omitted `Content-Type: application/json`, which the server's `jsonOnly` middleware rejects on every state-changing request regardless of body; (2) a CSS `:invalid:not(:placeholder-shown)` rule painted a red border on the untouched login form because no `placeholder` was set — removed, field-level error text covers it. Verified: `tsc`/`vite build`/`eslint`/`prettier` clean, `go vet` clean, 2/2 Vitest passing, and the full register→login→logout loop driven in a real headless browser at 375/768/1280px against the real API.

---

## Step 8 — Timeline, compose tweet, delete, like (frontend + test)

**Prompt:**

> Build the home timeline at `/`: `useInfiniteQuery` against `/api/timeline` with an intersection-observer sentinel plus a visible "Load more" fallback button (D-20), loading/error states, and the D-18 empty state linking to `/search`. Compose box at the top with a live code-point counter (D-13) that disables submit when over limit or empty; optimistic insert on success. `TweetCard` component: initials avatar, display name, `@username`, relative time with absolute `title` (D-30), content rendered `whitespace-pre-wrap` with `@mentions` linkified (D-14), like button with count (optimistic toggle, D-31), reply icon with count, delete action visible only for the author's tweets with a confirm. Write the create-tweet integration test: type content, counter updates, submit, mocked POST called with the content, tweet appears at the top; plus a test that the submit button is disabled at 281 chars.

**Done when:** alice's sample timeline renders and scrolls; compose/like/delete all work against the real API.

**Rubric:** Functionality (tweets, timeline, infinite scroll, likes), Testing (frontend), Responsive design.

_As executed (2026-09-18):_ Extended `api.ts` with tweet types and `getTimeline`/`createTweet`/`deleteTweet`/`likeTweet`/`unlikeTweet`. Added `time.ts` (relative/absolute formatting, D-30). `ComposeBox` (live code-point counter, disabled over 280/empty). `TweetCard` (avatar, `@mentions` linkified via regex — D-14, no backend mention model — like/reply/author-only-delete via `window.confirm`). `Home.tsx` rewritten around `useInfiniteQuery` with an IntersectionObserver sentinel + "Load more" fallback (D-20), and optimistic-update-with-rollback mutations for create/like/delete (D-31). Test: `compose-tweet.test.tsx` (counter updates, submit posts and prepends, 281-char boundary disables submit). Found and fixed a real test-infra bug: `tests/setup.ts` never called RTL's `cleanup()` between tests (no `test.globals: true` in `vite.config.ts`, so auto-cleanup's `afterEach` never registered) — Step 7's test only avoided the symptom because its two cases happened to navigate away between them. Verified: `tsc`/`vite build`/`eslint`/`prettier` clean, 4/4 Vitest passing, and compose/like/delete driven against the real API and alice's real sample timeline in a real browser at 375/768/1280px.

---

## Step 9 — Profile pages, follow/unfollow, search (frontend + test)

**Prompt:**

> Add `/:username` profile page: header with placeholder avatar, display name, `@username`, bio, follower/following counts linking to `/:username/followers` and `/:username/following` list pages (paginated), a Follow/Unfollow button (hidden on own profile; optimistic toggle updates counts), an "Edit bio" inline form on own profile, and the user's tweets below with infinite scroll. Add `/search` with a debounced input hitting `/api/search/users` and a result list linking to profiles; put a search entry in the nav. Write the follow-flow integration test: render another user's profile, click Follow, mocked POST called, button flips to Following and follower count increments; and the reverse for Unfollow.

**Done when:** all three required frontend flows (login, create tweet, follow) have passing integration tests; every required feature in the brief §4 is usable end to end.

**Rubric:** Functionality (profile, follows, lists, search), Testing (frontend).

_As executed (2026-09-18):_ Extended `api.ts` with profile/follow/search types and endpoints — including the gotcha that `PATCH /api/users/me` isn't a true partial update, so a bio-only edit must still resend the current `displayName`. Added `validateBio`. Refactored Home's infinite-scroll/like/delete logic into reusable `useInfiniteScrollSentinel.ts` and `useTweetFeed.ts` hooks plus a shared `TweetList.tsx`, since Profile needed the same behavior. Built `Profile.tsx` (bio/follow-toggle/tweet list, optimistic follow with server-truth reconciliation), `FollowList.tsx` (followers/following, D-24), and `Search.tsx` (300ms-debounced, D-26). Wired all four routes into `App.tsx`. Test: `follow-flow.test.tsx` (Follow → button flips + count increments; Unfollow → reverse). Verified: `tsc`/`vite build`/`eslint`/`prettier` clean, 6/6 Vitest passing, and follow/unfollow/bio-edit/followers-list/search driven against the real API and alice's real sample follow graph in a real browser at 375/1280px — including a debugging false alarm where a scratch Playwright script's loose role-name matcher (not the app) appeared to show a follower-count regression.

---

## Step 10 — Bonus: reply threads (backend + frontend + tests)

**Prompt:**

> Implement reply threads per D-47…D-50. Backend: `POST /api/tweets/{id}/replies` (same content rules, sets `parentTweetId`, 404 if parent missing or deleted), extend `GET /api/tweets/{id}` to return `{ ancestors, tweet, replies: { items, nextCursor } }` (ancestors walked to the root capped at 50, direct replies only, `createdAt ASC`), and make sure `replyCount` excludes deleted replies. Frontend: tweet detail page at `/tweet/:id` showing the ancestor chain (deleted ancestors as a "This tweet was deleted" placeholder — D-49), the focused tweet, an inline reply composer, and the paginated replies list; reply cards show a "Replying to @user" line. Tests: backend integration tests for reply creation, 404 on missing/deleted parent, thread retrieval order, deleted-ancestor placeholder data, and timeline exclusion (the sample data already contains nested replies to assert against); one frontend test that the detail page renders the thread and posting a reply appends it.

**Done when:** a multi-level thread can be created and navigated in the browser; tests green; backend coverage still ≥ 85 %.

**Rubric:** Bonus, Testing.

_As executed (2026-09-18):_ Store layer for replies/ancestors already existed from Step 2, so this step was service/HTTP/frontend. Added `Service.CreateReply` and `Service.GetThread` (returns `Thread{ Ancestors, Tweet, Replies }`; ancestors deliberately include deleted rows per the Step 2 exception, so a deleted ancestor's data still reaches the client with `isDeleted: true` — D-49; `MaxAncestorHops = 50`, D-48). `GET /api/tweets/{id}` now returns the thread shape instead of a bare tweet (replaces the old handler); added `POST /api/tweets/{id}/replies`. Tests cover reply creation/validation, missing/deleted parent → 404, thread ordering (ancestors root-first, replies `createdAt ASC`), and the deleted-ancestor placeholder data reaching the service layer. Frontend: `useThread.ts` hook (mirrors `useTweetFeed`, adds `appendReply`), `TweetDetail.tsx` at `/tweet/:id` (ancestor chain, focused tweet + replies reusing `TweetCard`, "Replying to @user" line per D-50, inline composer). Deleting the currently-viewed tweet navigates home rather than trying to keep the thread view in sync. Test: `thread.test.tsx` (renders ancestors/tweet/replies, posting a reply appends it, deleted-ancestor placeholder renders without leaking the live tweet's content). One real CSS bug caught only in a real browser: `.tweet-detail__replies` (a `<ul>`) was missing the `list-style: none` reset every other list in the app has, so a bullet rendered next to "Replying to" — fixed. Verified: `go build`/`go vet`/`gofmt` clean, coverage 93.2 %; `tsc`/`vite build`/`eslint`/`prettier` clean, 8/8 Vitest passing; a multi-level thread (root → reply → nested reply → delete root → confirm D-49 placeholder) driven in a real browser at 375/1280px.

---

## Step 11 — SQLite store, seed command, switch the default (Phase 2)

**Goal:** satisfy the brief's relational-database requirement with the same API and tests — a pure addition behind the `Store` interface (D-66).

**Prompt:**

> Add `internal/store/sqlite`: `database/sql` + `modernc.org/sqlite` (pure Go), an embedded `schema.sql` with the D-64 table shapes (snake_case tables, composite PKs on `follows`/`likes`, indexes `(author_id, created_at DESC, id DESC)`, `(followee_id, created_at DESC)`, `(follower_id, created_at DESC)`, `(parent_tweet_id, created_at)`, `(tweet_id)`, the self-follow `CHECK` inside `CREATE TABLE follows`, `ON DELETE NO ACTION` on reply → parent, `PRAGMA foreign_keys = ON` on every connection, WAL mode) applied at open via `CREATE TABLE IF NOT EXISTS` + a `schema_version` row, and every `Store` method as parameterized SQL (search per D-25 amendment, counts via `COUNT(*)`, cursor pagination via `(created_at, id) < (?, ?)`). Run the `storetest` conformance suite and the `httpapi` integration tests against it on a `t.TempDir()` file. Add `cmd/seed` (D-39/D-67: truncate then insert from `sample.json`; refuses under `APP_ENV=production` unless `SEED_FORCE=true`) and the root script `npm run seed`. Wire `SQLITE_PATH` (default `./data/twitter.db`, gitignored) and `SEED_FORCE`; flip the `STORE` default to `sqlite`; `cmd/api` seeds automatically when the `users` table is empty so `npm run dev` on a fresh clone is populated. Update `.env.example`, D-43, the README (Runbook now: `cp .env.example .env`, `npm install`, `npm run dev`; `npm run seed` to reset the data; the restart-resets note becomes the memory-store-only note), and the trade-offs section.

**Done when:** the whole suite is green on both stores; a fresh clone + Runbook yields a seeded SQLite app; `git status` shows no `.db` files.

**Rubric:** Functionality, Testing, Documentation — and the brief's §2 database requirement.

_As executed (2026-09-18):_ Added `internal/store/sqlite` on `database/sql` + `modernc.org/sqlite` (pinned `v1.38.2` to keep `go.mod`'s minimum at `go 1.23.0`, D-65). `schema.sql` (embedded via `go:embed`) carries the D-64 shapes: snake_case tables, the self-follow `CHECK`, `ON DELETE NO ACTION` reply→parent, and the required indexes. `foreign_keys`/`WAL`/`busy_timeout` set via DSN pragmas; connection pool capped at `SetMaxOpenConns(1)` (D-64 licenses "one writer at a time," sidesteps `SQLITE_BUSY` without a retry loop). Timestamps stored as fixed-width zero-padded RFC 3339 text so lexicographic ordering matches cursor comparisons. `Follow`/`Like` use `INSERT ... ON CONFLICT DO NOTHING` for idempotency; `Ancestors` walks the parent chain in Go, capped at `maxHops`, deliberately not filtering `deleted_at` (D-49). Bug caught by the conformance suite: the first `CreateUser` draft string-matched SQLite's constraint-error text to report which field conflicted, and got it wrong under `HTTPAPI_TEST_STORE=sqlite` — fixed by checking username/email existence with explicit `SELECT`s before inserting instead. Added `cmd/seed` (truncate + reload from `sample.json`, refuses under `APP_ENV=production` without `SEED_FORCE=true`) and flipped `STORE`'s default to `sqlite`; `cmd/api` seeds automatically when the store is empty. `internal/httpapi`'s scattered `memory.New()` test call sites were consolidated into one `newStore(t)` helper that returns SQLite when `HTTPAPI_TEST_STORE=sqlite` is set, so the whole integration suite runs against either backend (now a second CI step). Verified: `gofmt`/`go build`/`go vet` clean, full suite green on both stores, coverage 91.5 % (down from 93.2 %, expected — a whole new store implementation landed). Ran the real binary: fresh boot seeds and logs counts; a tweet posted via `curl` survives a process restart (proving persistence, where the memory store would reset); `git status` shows no `.db`/`.db-wal`/`.db-shm` files tracked.

---

## Step 12 — E2E auth test (Playwright), responsive QA, coverage proof, final documentation

**Prompt:**

> (1) Add Playwright at the repo root (`/e2e`) with one spec covering the auth flow against an already-running stack at `http://localhost:5173` (D-36): visit `/`, get redirected to `/login`, register a unique `e2e_<timestamp>` user, land on the timeline with the username visible, log out, verify `/` redirects to `/login` again, log back in; add `npm run test:e2e` and document that the app must be running first. (2) Responsive QA: walk every page at 375px, 768px and 1280px; fix overflow, cramped touch targets, desktop-first leftovers. (3) Run the full suite — server coverage on both stores, client tests, E2E — and fix anything red; put the final server coverage number in README. (4) Runbook verification: follow README's Runbook from a fresh clone literally (Go + Node only), fix any step that needs undocumented intervention, remove remaining "Planned" markers. (5) Fill README's Known Trade-offs with the real decisions (COUNT-on-read, single JWT cookie, replies hidden from timeline, phased persistence and the memory store, Docker deferred, anything discovered). (6) `gofmt`/`go vet`/lint clean, remove dead code and debug logs, make sure `.env.example` lists every variable the code reads. Commit as a few small polish/docs commits, not one blob.

**Done when:** README is fully accurate, all suites green, coverage ≥ 85 % recorded, working tree clean, `main` pushed.

**Rubric:** Testing (E2E requirement), Documentation, Code quality, Development process — and protects the Functionality score by guaranteeing the Runbook works.

_As executed (2026-09-18):_
1. Added `e2e/playwright.config.ts` (no `webServer` entry per D-36 — the spec doesn't start the app itself) and `e2e/auth.spec.ts`: registers a unique `e2e_<timestamp>` user, register → land on `/` with username visible → logout → redirected to `/login` → login again. `npm run test:e2e` added. Passed against a live `npm run dev`.
2. Responsive QA at 375/768/1280px across every route, checking `scrollWidth` vs `clientWidth` for overflow. Found and fixed two real bugs: (a) nav items in `AppShell.tsx` lost their accessible name at the tablet breakpoint (label `<span>` hidden by CSS, icon is `aria-hidden`) — fixed with an explicit `aria-label`; (b) the follow-list Follow/Following button was 36px tall, under the 44px touch-target minimum every other control uses — fixed. No horizontal overflow found anywhere.
3. Full suite green: Go suite on both stores, coverage 91.5 %; `eslint`/`prettier`/`tsc`/`vite build` clean; 8/8 Vitest; 1/1 E2E. Also caught (via a slow "unknown username" screenshot) a UX bug unrelated to layout: TanStack Query's default retry policy retried a definite 404 three times with backoff, so a nonexistent-username profile sat on "Loading…" for ~7.6s. Fixed by not retrying 4xx errors at the `QueryClient` level — down to ~0.16s.
4. Runbook verification: reproduced a fresh-clone state locally (stopped the server, deleted `twitter.db*`, confirmed `.env` matches `.env.example`) and re-ran every command and curl example in the README's Runbook and Sample Credentials sections against a freshly seeded API — all matched. A literal `git clone` re-check on a second machine is still worth doing once commits land.
5. Filled in the remaining Known Trade-offs entries (why E2E is one spec, the retry-policy fix).
6. `gofmt`/`go vet` clean; grepped for stray `console.log`/`debugger`/`TODO`/`FIXME` — none found; `.env.example` already lists all eight variables the code reads.

Files changed: `e2e/*`, `package.json`, `client/src/main.tsx`, `client/src/components/AppShell.tsx`, `client/src/pages/FollowList.css`, `README.md`, `IMPLEMENTATION-PLAN.md`, `my-docs/AGENT-COLLABORATION-HISTORY.MD`.

---

## Before delivery — final checklist

- [ ] Repo is public (or evaluators invited) — open item #1 in VALIDATION-OF-REQUIREMENTS.md
- [x] All 12 steps committed on `main` (`42bcbce` … `28372df`) — a true fresh `git clone` re-check is still recommended before delivery
- [x] README Runbook works with only Go 1.23+, Node 20+ and Git installed — verified against a reset local database in Step 12
- [x] `STORE` default is `sqlite` and the SQLite store passes the full suite (open item #10) — done in Step 11
- [x] Sample credentials in README are valid against `sample.json` — re-verified in Step 12 against a freshly seeded database
- [x] Server coverage ≥ 85 % and stated in README — 91.5 % as of Step 11, unchanged through Step 12
- [x] Three frontend integration tests (login, create tweet, follow) + one E2E auth spec present and passing — 8/8 Vitest (including the bonus thread test) + 1/1 Playwright
- [ ] Decide whether `my-docs/` and `CLAUDE.md` ship in the repo (currently gitignored) — see [CLAUDE.md](../CLAUDE.md)
- [x] AGENT-COLLABORATION-HISTORY.MD has an entry for every prompt used

---

## Post-MVP backlog (only after every step above is done)

### B-1 — Docker Compose full stack (bonus; D-63, D-40/D-41/D-46 as amended)

> Complete the Docker Compose bonus: services `server` (two-stage `golang:1.23-alpine` → static `CGO_ENABLED=0` binary in a minimal runtime image, `sample.json` included, a named volume for the SQLite file, entrypoint seeds only if `users` is empty, `GET /api/health` healthcheck) and `client` (Vite build served by nginx on host 5173, proxying `/api` to `server:3000`), with healthchecks, `depends_on: condition: service_healthy`, and `.dockerignore`. `docker compose up --build` from a clean clone must produce a seeded, working app with no other steps. Add a README Runbook "Option A — Docker Compose" that matches exactly what you built.

Starting material: `my-docs/archive/docker/` (a postgres-era compose file, now only useful for the healthcheck/volume shape).

**Done when:** `git clone` → `cp .env.example .env` → `docker compose up --build` → log in with the sample credentials works on a machine with only Docker installed.

### Dropped (2026-09-16, D-65)

B-2 (switch to PostgreSQL) is no longer a backlog item: SQLite is the final database. `my-docs/archive/postgres/` is kept only as a DDL reference for `schema.sql`.
