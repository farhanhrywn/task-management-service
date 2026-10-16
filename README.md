# Task Management Service

A multi-user task management REST API. Users manage their own tasks; a single
**super admin** manages team membership, can access every task, and is the only
one who can assign a task to another user **who shares a team with the task owner**.

**Stack:** Go 1.25 · Gin · GORM · PostgreSQL 16 · `log/slog` · Docker Compose

---

## Table of Contents

1. [Getting Started](#1-getting-started)
2. [Running Tests](#2-running-tests)
3. [API Endpoints](#3-api-endpoints)
4. [Architecture](#4-architecture)
5. [Product Decisions](#5-product-decisions)
6. [Trade-offs & Notes](#6-trade-offs--notes)

---

## 1. Getting Started

### Prerequisites

- Docker with Docker Compose v2 (Docker Desktop, OrbStack, …)
- Go 1.25+ (only to run tests or the API outside Docker)

### Run with Docker Compose

```bash
cp .env.example .env          # then set a strong JWT_SECRET
docker compose up --build
```

The API listens on `http://localhost:8080`. On startup it connects to
PostgreSQL, runs AutoMigrate, creates the partial/trigram indexes and seeds the
default data (idempotent, safe on every restart).

PostgreSQL is published on host port **5433** (`DB_HOST_PORT`) to avoid
clashing with a local PostgreSQL on 5432.

### Seeded data

| Data | Value |
|---|---|
| Teams | `Team Alpha`, `Team Beta` |
| Super admin | email `SUPERADMIN_EMAIL`, password `SUPERADMIN_PASSWORD` from `.env` (defaults in `.env.example`: `superadmin@mail.com` / `password123`) |

The super admin is not a member of any team by default.

### Run the API on the host

```bash
docker compose up -d postgres
go run ./cmd/api              # reads .env (DB_HOST=localhost, DB_PORT=5433)
```

### Configuration

All configuration, credentials and secrets come from environment variables
loaded from `.env`, which is git-ignored. `.env.example` documents every
variable.

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` / `APP_PORT` | `development` / `8080` | |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | `localhost`, `5433`, `postgres`, –, `task_management`, `disable` | Compose overrides host/port to `postgres:5432` |
| `DB_HOST_PORT` | `5433` | Host port for the postgres container |
| `JWT_SECRET` | – (required) | HS256 signing secret |
| `JWT_EXPIRES_IN` | `24h` | |
| `SUPERADMIN_NAME`, `SUPERADMIN_EMAIL`, `SUPERADMIN_PASSWORD` | `Super Admin`, – , – | Email and password are required |
| `IDEMPOTENCY_TTL` | `24h` | Key window |
| `IDEMPOTENCY_WAIT_TIMEOUT` | `10s` | Max wait for an in-flight duplicate |
| `IDEMPOTENCY_POLL_INTERVAL` | `100ms` | |
| `PAGINATION_DEFAULT_LIMIT` / `PAGINATION_MAX_LIMIT` | `10` / `50` | |

### Manual test scenario

1. Register users A, B and C (`POST /auth/register`).
2. Log in as super admin, add A and B to Team Alpha and C to Team Beta.
3. Log in as A, `POST /tasks` with an `Idempotency-Key`, send it again with the
   same key → the same response, still one task.
4. Super admin assigns A's task to B → 200.
5. Super admin assigns A's task to C → 422 `ASSIGNEE_NOT_IN_TEAM`.
6. Log in as B, `GET /tasks/:id` on A's task → 403.

---

## 2. Running Tests

Unit tests run **without PostgreSQL**: every repository is replaced by a
thread-safe in-memory fake in [`test/mocks`](test/mocks).

```bash
go test ./... -race -count=20
```

| ID | Scenario | Where |
|---|---|---|
| UT-01 | Sequential duplicate key → identical response, 1 task | `internal/service/idempotency_service_test.go` |
| UT-02 | 100 concurrent goroutines, same key → 1 task, identical responses, race-free | `internal/service/idempotency_service_test.go` |

---

## 3. API Endpoints

Base URL `http://localhost:8080`. Bodies are JSON. Successful responses are
wrapped in `{ "data": ... }` (plus `meta` for lists); deletes return
`{ "message": ... }`. All timestamps are UTC with second precision:
`YYYY-MM-DDTHH:mm:ssZ`.

| Method | Path | Access | Success |
|---|---|---|---|
| POST | `/auth/register` | Public | 201 |
| POST | `/auth/login` | Public | 200 |
| POST | `/tasks` | JWT + `Idempotency-Key` | 201 |
| GET | `/tasks` | JWT (own tasks; super admin: all) | 200 |
| GET | `/tasks/:id` | JWT (owner / super admin) | 200 |
| PUT | `/tasks/:id` | JWT (owner / super admin) | 200 |
| DELETE | `/tasks/:id` | JWT (owner / super admin) | 200 |
| POST | `/tasks/:id/assign` | JWT (super admin) | 200 |
| GET | `/teams` | JWT | 200 |
| GET | `/teams/:id/members` | JWT | 200 |
| POST | `/teams/:id/members` | JWT (super admin) | 201 |
| DELETE | `/teams/:id/members/:userId` | JWT (super admin) | 200 |

Protected endpoints require `Authorization: Bearer <access_token>`.

### Auth

**`POST /auth/register`** — name 1–100 chars, valid unique email, password 8–72 chars.

```bash
curl -X POST localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Farhan","email":"farhan@mail.com","password":"secret123"}'
```

```json
{ "data": { "id": "uuid", "name": "Farhan", "email": "farhan@mail.com", "role": "user",
            "created_at": "2026-10-15T08:00:00Z", "updated_at": "2026-10-15T08:00:00Z" } }
```

Errors: 400 `VALIDATION_ERROR`, 409 `EMAIL_ALREADY_EXISTS`.

**`POST /auth/login`**

```bash
curl -X POST localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"farhan@mail.com","password":"secret123"}'
```

```json
{ "data": { "access_token": "eyJ...", "token_type": "Bearer", "expires_in": 86400 } }
```

Errors: 400 `VALIDATION_ERROR`, 401 `INVALID_CREDENTIALS` (same for unknown email and wrong password).

### Tasks

Task object:

```json
{
  "id": "uuid",
  "user_id": "uuid",
  "title": "Setup CI pipeline",
  "description": "GitHub Actions",
  "status": "todo",
  "assignee_id": null,
  "created_at": "2026-10-15T08:00:00Z",
  "updated_at": "2026-10-15T08:00:00Z"
}
```

`user_id`, `assignee_id` and timestamps are server-controlled; any value sent by the client is ignored.

**`POST /tasks`** — `title` required (1–255), `description` optional (≤5000), `status` optional (`todo` default, `in_progress`, `done`).

```bash
curl -X POST localhost:8080/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Idempotency-Key: 0f8fad5b-d9cb-469f-a165-70867728950e' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Setup CI pipeline","description":"GitHub Actions"}'
```

`201 { "data": <task> }`. A replayed response carries the header `Idempotent-Replayed: true`.

Errors: 400 `IDEMPOTENCY_KEY_MISSING` / `IDEMPOTENCY_KEY_INVALID` / `VALIDATION_ERROR`, 409 `IDEMPOTENCY_IN_PROGRESS`.

**`GET /tasks?status=todo&search=ci&page=1&limit=10`**

| Param | Rule | Default |
|---|---|---|
| `status` | `todo` / `in_progress` / `done` | – |
| `search` | case-insensitive partial match on `title` | – |
| `page` | integer ≥ 1 | 1 |
| `limit` | integer 1–50 (> 50 → 400) | 10 |

```json
{
  "data": [ <task>, ... ],
  "meta": { "page": 1, "limit": 10, "total_items": 42, "total_pages": 5 }
}
```

Ordered by `created_at DESC`. A page past the end returns `data: []` with correct `meta`.

**`GET /tasks/:id`** → `200 { "data": <task> }`

**`PUT /tasks/:id`** — `title` and `status` required, `description` optional (empty/omitted clears it). `assignee_id` cannot be changed here.

```bash
curl -X PUT localhost:8080/tasks/$ID -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Setup CI pipeline v2","description":"GitHub Actions + lint","status":"in_progress"}'
```

**`DELETE /tasks/:id`** — soft delete → `200 { "message": "task deleted" }`

Access errors for `GET/PUT/DELETE /tasks/:id`, checked in this order:
400 `INVALID_ID` → 404 `TASK_NOT_FOUND` (missing or deleted) → 403 `FORBIDDEN` (not owner, not super admin).

**`POST /tasks/:id/assign`** — super admin only.

```bash
curl -X POST localhost:8080/tasks/$ID/assign -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"assignee_id":"uuid"}'
```

`200 { "data": <task with assignee_id> }`. Errors: 400 `VALIDATION_ERROR` / `INVALID_ID`,
403 `FORBIDDEN`, 404 `TASK_NOT_FOUND` / `USER_NOT_FOUND`, 409 `ALREADY_ASSIGNED`,
422 `ASSIGNEE_NOT_IN_TEAM`.

### Teams

**`GET /teams`**

```json
{ "data": [ { "id": "uuid", "name": "Team Alpha", "created_at": "...", "updated_at": "..." } ] }
```

**`GET /teams/:id/members`**

```json
{ "data": [ { "user_id": "uuid", "name": "Farhan", "email": "farhan@mail.com", "joined_at": "2026-10-15T08:00:00Z" } ] }
```

**`POST /teams/:id/members`** — super admin only. Body `{ "user_id": "uuid" }`.

```json
{ "data": { "team_id": "uuid", "user_id": "uuid", "joined_at": "2026-10-15T08:00:00Z" } }
```

Errors: 403 `FORBIDDEN`, 404 `TEAM_NOT_FOUND` / `USER_NOT_FOUND`, 409 `MEMBER_ALREADY_EXISTS`.

**`DELETE /teams/:id/members/:userId`** — super admin only → `200 { "message": "member removed" }`.
Errors: 403, 404 `TEAM_NOT_FOUND` / `MEMBER_NOT_FOUND`. Tasks assigned to the user are left unchanged.

### Error format

Every 4xx/5xx response uses the same envelope:

```json
{
  "status": 403,
  "code": "FORBIDDEN",
  "message": "You do not have access to this resource",
  "timestamp": "2026-10-15T08:12:33Z",
  "request_id": "6f1c2c8e-1b5a-4c0e-9a3d-2d7b6c1e0f11",
  "details": null
}
```

`details` lists per-field problems for `VALIDATION_ERROR`, e.g.
`[{"field":"email","message":"must be a valid email address"}]`.

| HTTP | Code | When |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Invalid body or query parameter |
| 400 | `INVALID_ID` | Path parameter is not a UUID |
| 400 | `IDEMPOTENCY_KEY_MISSING` | `Idempotency-Key` header missing |
| 400 | `IDEMPOTENCY_KEY_INVALID` | Header is not a canonical UUID |
| 401 | `UNAUTHORIZED` | Token missing, invalid or expired |
| 401 | `INVALID_CREDENTIALS` | Login failed |
| 403 | `FORBIDDEN` | Not the task owner, or super admin only endpoint |
| 404 | `TASK_NOT_FOUND` | Task missing or deleted |
| 404 | `USER_NOT_FOUND` | User missing |
| 404 | `TEAM_NOT_FOUND` | Team missing |
| 404 | `MEMBER_NOT_FOUND` | User is not a team member |
| 404 | `ROUTE_NOT_FOUND` | Unknown endpoint |
| 405 | `METHOD_NOT_ALLOWED` | Unsupported method |
| 409 | `EMAIL_ALREADY_EXISTS` | Email already registered |
| 409 | `MEMBER_ALREADY_EXISTS` | Already a team member |
| 409 | `ALREADY_ASSIGNED` | Same assignee as now |
| 409 | `IDEMPOTENCY_IN_PROGRESS` | Same key still processing after 10s |
| 422 | `ASSIGNEE_NOT_IN_TEAM` | Assignee shares no team with the owner |
| 500 | `INTERNAL_ERROR` | Unexpected error or panic |

---

## 4. Architecture

### Layers

```
HTTP ─▶ router + middleware ─▶ handler ─▶ service ─▶ repository ─▶ PostgreSQL
                                             ├────▶ notifier (mock: slog)
                                             └────▶ TxManager
```

- **handler** binds and validates DTOs, reads the caller from the context and
  maps results to HTTP. It never touches the database.
- **service** holds the use cases and business rules and returns `*AppError`
  for known failures. It depends only on interfaces (repositories,
  `TxManager`, `Notifier`, `Clock`), so it is unit tested with in-memory fakes.
- **repository** implements persistence with GORM and maps driver errors
  (`23505` unique violation, `55P03` lock timeout, not found) to sentinel errors.
- **TxManager** stores the GORM transaction in `context.Context`; every
  repository resolves its connection through it, so the same repository works
  inside and outside a transaction.

```
cmd/api/              entrypoint: config, DB, migrate, seed, wiring, graceful shutdown
internal/config/      env → Config
internal/database/    connection, AutoMigrate + custom indexes, seed
internal/domain/      GORM entities, enums, Actor (access rules)
internal/dto/         request/response payloads + validation tags
internal/handler/     auth, task, team handlers
internal/middleware/  request id, access log, recovery, error handler, auth, role
internal/notifier/    Notifier interface + LogNotifier
internal/repository/  interfaces + GORM implementations + TxManager
internal/router/      route registration
internal/service/     auth, task, idempotency, assign, team
pkg/                  apperror, clock, ctxkey, jwtutil, logger, response
test/mocks/           thread-safe in-memory fakes for unit tests
```

### Request lifecycle

`RequestID` → `Logger` → `Recovery` → `ErrorHandler` → (`Auth` → `RequireRole`) → handler

1. **RequestID** generates a UUID v4, sets `X-Request-ID`, and puts a
   request-scoped logger (with `request_id`) into the context.
2. **Logger** writes exactly one JSON access log after the request:
   `request_id, method, path, status, latency_ms, client_ip, user_id`.
   Level: INFO (<400), WARN (4xx), ERROR (5xx, plus `error` and `stack`).
   Bodies, passwords and tokens are never logged.
3. **Recovery** turns a panic into 500 `INTERNAL_ERROR`; the stack goes to the log only.
4. **ErrorHandler** renders the last `c.Error(...)`: `*AppError` as is,
   anything else as a generic 500. Internal causes never reach the client.
5. **Auth** validates the Bearer JWT (HS256, `sub`, `role`, `iat`, `exp`);
   **RequireRole** returns 403 for non super admins.

### Idempotency (`POST /tasks`)

PostgreSQL alone guarantees exactly-once creation. No Redis is used.

```sql
BEGIN;
SET LOCAL lock_timeout = '10000ms';

INSERT INTO idempotency_keys (id, user_id, idem_key, created_at, expires_at)
VALUES (...)
ON CONFLICT (user_id, idem_key) DO UPDATE
   SET created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at,
       task_id = NULL, response_status = NULL, response_body = NULL
 WHERE idempotency_keys.expires_at <= $now        -- reclaim only expired keys
RETURNING id;
-- 1 row  → we own the key: INSERT task, UPDATE response, COMMIT → 201
-- 0 rows → a valid key exists: SELECT stored response → replay

COMMIT;
```

- **Exactly once:** `UNIQUE (user_id, idem_key)` is the final guard. The claim,
  the task insert and the stored response commit together.
- **Concurrent duplicates:** a second transaction inserting the same key waits
  on the unique index until the first commits, then gets 0 rows and replays the
  committed response. If the wait exceeds `IDEMPOTENCY_WAIT_TIMEOUT` (10s),
  `lock_timeout` raises `55P03` → 409 `IDEMPOTENCY_IN_PROGRESS`.
- **Failure:** if anything fails, the whole transaction, including the key,
  rolls back, so a retry is processed normally.
- **Byte-identical replay:** `response_body` is a `json` (not `jsonb`) column,
  which keeps the exact bytes; the first response is also written from those
  same bytes.
- **Scope & window:** keys are per user and valid for 24h; after that the key
  is reclaimed and creates a new task. A different body with the same key
  replays the first response.
- The service also has a generic wait-and-poll loop (every 100ms, up to 10s)
  for a key reported as in-flight. The in-memory test fake uses it; with
  PostgreSQL the wait happens inside the database.

Verified against PostgreSQL: 50 parallel `curl` requests with one key → 1
task and 50 byte-identical `201` responses. A key held in another
transaction for 13s → 409 after 10s, and the retry succeeds after rollback.

### Assign (single DB transaction)

```
BEGIN
  SELECT task FOR UPDATE                    → 404 TASK_NOT_FOUND
  SELECT assignee                           → 404 USER_NOT_FOUND
  same assignee?                            → 409 ALREADY_ASSIGNED
  owner & assignee share a team? (EXISTS)   → 422 ASSIGNEE_NOT_IN_TEAM
  UPDATE tasks SET assignee_id, updated_at
  INSERT task_logs (ASSIGN, old, new, actor)
  notifier.TaskAssigned (mock: structured log)
COMMIT            -- any error or panic → ROLLBACK of every step
```

`FOR UPDATE` serializes concurrent assignments of the same task.

### Data model

All primary keys are UUIDs; all timestamps are `TIMESTAMPTZ(0)` in UTC (GORM
`NowFunc` truncated to seconds, DB session `TimeZone=UTC`).

| Table | Key constraints |
|---|---|
| `users` | `email` UNIQUE (stored lowercase), `role IN ('user','super_admin')` |
| `teams` | `name` UNIQUE |
| `team_members` | PK `(team_id, user_id)`, FKs `ON DELETE CASCADE`, index `(user_id)` |
| `tasks` | FKs to users (owner, assignee), `status IN ('todo','in_progress','done')`, soft delete via `deleted_at` |
| `task_logs` | FKs to task and actor, index `(task_id, created_at)` |
| `idempotency_keys` | UNIQUE `(user_id, idem_key)`, `response_body json` |

Custom indexes, created after AutoMigrate:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_tasks_user_created ON tasks (user_id, created_at DESC)         WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_user_status  ON tasks (user_id, status, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_created      ON tasks (created_at DESC)                  WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_title_trgm   ON tasks USING GIN (title gin_trgm_ops);
```

Lists run two queries (COUNT + SELECT) that share one filter scope. There is
no N+1: team members are fetched with a single JOIN.

### Security

- Passwords hashed with bcrypt; never stored or returned in plain text. Login
  compares against a dummy hash for unknown emails to avoid timing-based user
  enumeration.
- JWT HS256 with the secret from env; algorithm pinned, `exp` required.
- Every query uses parameter binding; `%`, `_` and `\` in search are escaped.
- Duplicates (email, membership, idempotency key) are rejected by database
  constraints, not check-then-insert, so they are race-safe.
- Containers run a distroless, non-root image.

---

## 5. Product Decisions

| # | Decision |
|---|---|
| K1 | Register takes `name`, `email`, `password`; login takes `email` + `password`; new users get role `user`. |
| K2 | JWT is valid for 24 hours. |
| K3 | One super admin, seeded from `.env` (`SUPERADMIN_EMAIL` / `SUPERADMIN_PASSWORD`). |
| K4 | Teams come from seed only: Team Alpha and Team Beta. No team owner. The super admin is in no team by default. |
| K5 | A user can belong to many teams; membership is managed by the super admin. |
| K6 | Removing a user from a team leaves tasks assigned to them unchanged. |
| K7 | Task fields: `id`, `user_id`, `title`, `description`, `status`, `assignee_id`, `created_at`, `updated_at`, `deleted_at`. |
| K8 | Status `todo` / `in_progress` / `done`, default `todo`. |
| K9 | Delete is a soft delete and returns 200 with a message. |
| K10 | Another user's task → 403; missing or deleted task → 404. The assignee has no access to a task assigned to them. |
| K11 | Timestamps are UTC with second precision, `YYYY-MM-DDTHH:mm:ssZ`. |
| K12 | Validation: name 1–100, unique valid email, password ≥ 8 (≤ 72, the bcrypt limit); title 1–255, description ≤ 5000, status enum. |
| K13 | List: page default 1; limit default 10, max 50; `ILIKE` partial search; `created_at DESC`. |
| K14 | Only the super admin can assign. |
| K15 | The assignee must share at least one team with the task owner. |
| K16 | Re-assigning to another eligible user is allowed; assigning the current assignee again → 409. |
| K17 | Assigning the owner is allowed if the owner is in at least one team. |
| K18 | `Idempotency-Key` is required on `POST /tasks`, canonical UUID format. |
| K19 | Keys are scoped per user with a 24h window. |
| K20 | Same key with a different body → the first response is replayed. |
| K21 | A duplicate arriving while the first is in progress waits and replays, up to 10s, then 409. |
| K22 | Storage is PostgreSQL only: unique constraint + one transaction. |

---

## 6. Trade-offs & Notes

| Topic | Note |
|---|---|
| Notification inside the transaction | The brief asks for the notification to be part of the transaction (mocked). A real notification is an external side effect that cannot be rolled back; in production this would use the transactional outbox pattern. |
| Same key, different body | The body is ignored (K20), so a client that wrongly reuses a key gets no warning. |
| Soft delete & replay | Deleted tasks stay in the database, so `task_logs` and `idempotency_keys` remain valid. Replaying a key for a deleted task still returns the original 201. |
| Role in JWT | The role is read from the token; a role change applies after the next login. This is acceptable because the super admin only comes from seed. |
| Removed team member | Because of K6, a task can keep an assignee who no longer shares a team with the owner. The next re-assign is still validated. |
| bcrypt | Only the first 72 bytes are used, so passwords are capped at 72 characters. |
| Ordering ties | `created_at` has second precision, so `id DESC` breaks ties to keep pagination stable. |
