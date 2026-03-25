# AI Tutor Cloud Handoff (Track B)

Version: 1.0  
Date: 2026-03-25  
Owner: Friend (Cloud)

## 1. Goal

Build the cloud side (`ai-tutor-cloud`) so the local app can sync analytics and teachers can view class + student performance.

This document is implementation-first and should be used together with:
- `doc/DATA_API.md` (API contract)
- `doc/SCHEMA.md` (PostgreSQL schema definitions)
- `doc/PLAN_SCOPE.md` (scope boundaries)

## 2. What Local App Already Does

From `ai-tutor-local` current behavior:
- Review workflow writes FSRS logs and session summaries locally.
- Telemetry events are enqueued into local `sync_queue`.
- Manual sync performs local validation and status transitions.
- Cloud HTTP delivery is not fully wired yet.

Implication: you can build and test cloud endpoints now with mocked requests first, then integrate when local HTTP sync is enabled.

## 3. Must-Build API Endpoints (Priority Order)

### P0: Student Sync Path

1. `POST /api/v1/classes/join`
- Validate class code
- Upsert/create student
- Return class metadata

2. `POST /api/v1/sync`
- Accept batched events
- Deduplicate by `event_id`
- Persist raw event log (`analytics_events`)
- Update aggregate tables/materialized views
- Return accepted/rejected counts

3. `POST /api/v1/sync/notebook-deleted`
- Preserve historical analytics
- Mark notebook inactive/deleted for that student

### P1: Teacher Read APIs

4. `POST /api/v1/teacher/classes`
- Create class with unique class code

5. `GET /api/v1/teacher/classes/:class_id/overview`
- Class-level KPIs and weak topics

6. `GET /api/v1/teacher/students/:student_id`
- Student-level drilldown metrics

## 4. Request/Response Contract Notes

Use `doc/DATA_API.md` as binding source. Important points:
- `event_id` must be unique and idempotent-safe.
- Duplicate `event_id` should not fail the whole batch.
- Return success even if some events are duplicates.
- Keep optional numeric fields nullable (`quiz_score`, `quiz_total`, `accuracy_pct`).
- Never require raw study content fields; cloud only receives analytics metadata.

## 5. Current Compatibility Watchlist

There is one naming mismatch to handle safely during integration:
- Local currently emits `event_type = flashcard_session_completed`.
- API contract examples mention `flashcard_session`.

Recommended cloud behavior:
- Accept both values now.
- Normalize to one internal canonical value (for example `flashcard_session`).
- Do not reject event due to this difference.

## 6. Suggested Cloud Repository Structure

```text
ai-tutor-cloud/
  src/
    app.ts
    server.ts
    config/
    db/
      pool.ts
      migrations/
    middleware/
      auth.ts
      rateLimit.ts
      validate.ts
    routes/
      student.routes.ts
      teacher.routes.ts
    controllers/
    services/
      sync.service.ts
      class.service.ts
      analytics.service.ts
    repositories/
    types/
  dashboard/
    (React app)
```

## 7. Database Strategy (PostgreSQL)

Minimum tables to ship P0/P1:
- `teachers`
- `classes`
- `students`
- `analytics_events` (append-only raw events, unique on `event_id`)
- `aggregated_stats` (or equivalent summary table)

Recommended constraints:
- Unique class code
- Unique event ID
- Indexes:
  - `analytics_events(student_id, occurred_at)`
  - `analytics_events(class_id, occurred_at)`
  - `analytics_events(notebook_id)`

## 8. Processing Model for /sync

On each `/sync` request:
1. Validate body shape and required fields.
2. For each event:
- Skip/mark duplicate if `event_id` exists.
- Insert into `analytics_events` if new.
3. Update aggregates in same transaction or async worker.
4. Return `{ success, events_accepted, events_rejected }`.

If a per-event issue occurs:
- Reject only that event, not whole batch.
- Include count in `events_rejected`.

## 9. Security and Reliability Requirements

- JWT for teacher routes.
- Student routes can use `X-Student-Token` plus class code validation.
- Rate limit `/sync` by student identity.
- Keep `/sync` idempotent.
- Log request IDs and failed payload reasons.

## 10. Friend Build Checklist

### Week 1
- Bootstrap Node + PostgreSQL project
- Add migrations and core tables
- Implement `POST /api/v1/classes/join`
- Implement `POST /api/v1/sync` with dedup

### Week 2
- Implement teacher auth + class creation
- Implement overview and student drilldown APIs
- Add dashboard skeleton pages (overview and student detail)

### Week 3
- Add test fixtures and end-to-end API tests
- Add deployment config and environment docs
- Coordinate with local app for real sync smoke test

## 11. Integration Smoke Test (When Local HTTP Sync Is Added)

1. Start cloud API locally (for example `http://localhost:8080`).
2. Seed teacher + class.
3. Join class from local app.
4. Run review session in local app to generate telemetry.
5. Trigger manual sync.
6. Verify:
- rows inserted in `analytics_events`
- class overview metrics updated
- student drilldown shows new session data

## 12. Definition of Done for Track B MVP

Track B MVP is done when:
- All endpoints in Section 3 are implemented and documented.
- `/sync` is idempotent and handles duplicate events correctly.
- Teacher can create class, view class overview, and inspect one student timeline.
- Local + cloud integration smoke test passes with real data.
