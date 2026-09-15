# MVP Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable Go service that creates promotion Tracking records through a JD channel boundary and provides the foundation for order, commission, and wallet work.

**Architecture:** Start as a modular monolith with domain packages under `internal/`. HTTP depends on application services; application services depend on narrow repository and channel interfaces. JD-specific protocol code remains behind `channel/jd`, so business logic never imports a provider SDK directly.

**Tech Stack:** Go 1.26, standard-library HTTP and testing, PostgreSQL for later durable storage, Redis only for cache/locking, Docker Compose for local infrastructure.

**Spec:** `docs/产品总设计_V1.1.md`, `docs/01-api-spec.md`, `docs/03-channel-jd.md`

**Progress (2026-09-15):** Tasks 1-3 implemented and verified on `feat/mvp-foundation`; Tasks 4-6 remain.

## Global Constraints

- No real AppKey, AppSecret, PID, access token, cookie, or user credential may enter Git, logs, fixtures, or documentation.
- Money uses integer minor units plus an explicit ISO currency code.
- Every state-changing operation accepts an idempotency key.
- Provider payloads are isolated in adapters and saved only after sensitive-field redaction.
- Production readiness requires evidence from the actually approved JD account and promotion position.

---

### Task 1: Runnable Service Skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/api/main.go`
- Create: `internal/httpapi/router.go`
- Test: `internal/httpapi/router_test.go`

**Interfaces:**
- Produces: `httpapi.NewRouter() http.Handler`
- Produces: `GET /healthz` returning HTTP 200 and `{"status":"ok"}`

- [ ] Write `TestHealthz` using `httptest.NewRecorder` and assert status, JSON content type, and decoded status.
- [ ] Run `go test ./internal/httpapi -run TestHealthz -v`; expect failure because `NewRouter` does not exist.
- [ ] Implement the smallest `http.ServeMux` with the health handler and JSON response.
- [ ] Run `go test ./internal/httpapi -run TestHealthz -v`; expect pass.
- [ ] Run `go test ./...` and commit the runnable skeleton.

### Task 2: Tracking Domain and Idempotent Creation

**Files:**
- Create: `internal/tracking/model.go`
- Create: `internal/tracking/service.go`
- Test: `internal/tracking/service_test.go`

**Interfaces:**
- Consumes: caller-supplied idempotency key, user ID, channel, external product ID, and source.
- Produces: `tracking.Service.Create(ctx context.Context, in CreateInput) (Record, error)`
- Defines: `Repository.FindByIdempotencyKey` and `Repository.Save`

- [ ] Write a test proving the same idempotency key returns the existing record without a second save.
- [ ] Run the focused test and observe the missing-domain failure.
- [ ] Add immutable `Record`, validated `CreateInput`, repository interface, clock/ID dependencies, and minimal service.
- [ ] Run the focused test and then all package tests.
- [ ] Add tests for blank idempotency key, blank user, unsupported channel, and blank product ID; implement only the validation needed to pass.
- [ ] Commit the Tracking domain slice.

### Task 3: JD Promotion Adapter Boundary

**Files:**
- Create: `internal/channel/promotion.go`
- Create: `internal/channel/jd/adapter.go`
- Test: `internal/channel/jd/adapter_test.go`

**Interfaces:**
- Produces: `channel.PromotionLinker.CreatePromotionLink(ctx, Request) (Link, error)`
- JD adapter consumes a narrow `Client.GeneratePromotionLink(ctx, ClientRequest)` interface.
- Internal Tracking ID maps to the JD sub-union field only when `SubUnionEnabled` is true.

- [ ] Test exact mapping of product, position, and Tracking fields into the client request.
- [ ] Run the test and observe failure because the adapter is absent.
- [ ] Implement the adapter without network or credential-loading logic.
- [ ] Run focused and full tests.
- [ ] Test and implement the disabled-sub-union path and provider-error wrapping.
- [ ] Commit the JD adapter boundary.

### Task 4: Promotion Link Application Flow

**Files:**
- Create: `internal/promotion/service.go`
- Test: `internal/promotion/service_test.go`
- Modify: `internal/httpapi/router.go`
- Test: `internal/httpapi/promotion_test.go`

**Interfaces:**
- Consumes: `POST /api/v1/promotions/link` plus `Idempotency-Key` and authenticated user ID.
- Produces: Tracking ID, promotion URL, optional scheme URL, and expiry.
- Depends on: Tracking service and `channel.PromotionLinker`.

- [ ] Prove with a failing application-service test that Tracking is saved before the provider is called.
- [ ] Implement orchestration and stable domain errors.
- [ ] Prove with failing HTTP tests that invalid JSON, missing idempotency, unsupported channel, and provider outage map to stable status/code pairs.
- [ ] Implement the handler and dependency-injected router.
- [ ] Run all tests and commit the end-to-end in-memory flow.

### Task 5: PostgreSQL Tracking Repository

**Files:**
- Create: `migrations/000001_tracking_records.up.sql`
- Create: `migrations/000001_tracking_records.down.sql`
- Create: `internal/tracking/postgres_repository.go`
- Test: `internal/tracking/postgres_repository_test.go`
- Create: `deploy/compose.yaml`

**Interfaces:**
- Implements: Tracking `Repository`.
- Enforces: unique idempotency key and unique Tracking ID in PostgreSQL.

- [ ] Add migration tests that apply the schema to an isolated local database.
- [ ] Implement repository insert and lookup in one transaction boundary.
- [ ] Add a concurrent duplicate-create test and confirm one logical record.
- [ ] Run integration tests with the local Compose database.
- [ ] Commit persistence and local infrastructure.

### Task 6: Real JD Sandbox/Approved-Account Integration

**Files:**
- Create: `internal/channel/jd/http_client.go`
- Test: `internal/channel/jd/http_client_test.go`
- Create: `.env.example`
- Modify: `docs/03-channel-jd.md`

**Interfaces:**
- Consumes environment variable names for JD endpoint, application key reference, secret reference, union ID, and promotion position.
- Produces redacted request/response diagnostics and typed provider errors.

- [ ] Capture the actually approved JD API list, application type, union ID, promotion position, rate limits, signature version, and test environment without recording secret values.
- [ ] Write deterministic signature/request tests from redacted official examples and observe failure.
- [ ] Implement request signing, timeouts, response validation, and redaction.
- [ ] Execute credential validation and promotion-link generation against the approved account.
- [ ] Record request IDs, timestamps, non-sensitive response fields, and negative-path evidence.
- [ ] Commit only after secret scanning and full regression pass.
