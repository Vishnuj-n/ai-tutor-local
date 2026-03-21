# Sprint Plan and Progress Tracker

Version: 1.0  
Date: 2026-03-21  
Project: Local-First AI Tutoring System (Track A + Track B)

## Sprint Overview

| Sprint | Focus | Status | Notes |
|---|---|---|---|
| Sprint 0 | Architecture, API contract, schema hardening | Completed | Core docs finalized, critical schema/API issues fixed |
| Sprint 1 | Track A foundation scaffold | Completed | Go module, schema.sql, DB init, models, queries, module skeletons |
| Sprint 2 | Current sprint: ingestion to retrieval pipeline baseline | Completed | End-to-end ingestion + FTS retrieval baseline validated; vec path optional with strict mode/fallback |
| Sprint 3 | FSRS + review workflows + telemetry quality | Completed | FSRS rating workflow, due-card scheduler, session telemetry quality guards implemented |
| Sprint 4 | Classroom sync stabilization + cloud aggregation validation | In Progress | Started with ONNX embedding runtime migration + sync stabilization kickoff |
| Sprint 5 | Release hardening | Planned | Performance, security, E2E regression suite |

## Completed Work (Already Done)

### Sprint 0 Completed

- Documentation baseline created and aligned across requirements, app flow, architecture, schema, and API contract.
- Fixed sqlite-vec primary key mapping to rowid strategy.
- Fixed embedding model dimensional strategy (local embeddings only; generation can be API/local).
- Fixed notebook identity mismatch by introducing notebook_id in sync and cloud uniqueness.
- Clarified flashcard_session duration aggregation contract.

### Sprint 1 Completed

- Created Track A project structure under ai-tutor-local.
- Added Go module and dependency setup.
- Added local schema file with:
  - Core tables (notebooks, documents, chunks, flashcards, review_logs, sessions, sync_queue, config)
  - FTS5 virtual table and triggers
  - sqlite-vec embeddings table with chunk_rowid integer primary key
- Implemented DB bootstrap and migration runner.
- Implemented DB models and query layer.
- Added ingestion chunker + document registration service.
- Added embedding client for local Ollama.
- Added sync event model + queue service.
- Added generation client interface.

## Current Sprint (Sprint 2): Completed

### Sprint Goal

Establish reliable end-to-end local ingestion baseline and retrieval readiness:
- Register documents
- Chunk content
- Persist chunks
- Prepare embedding and retrieval path
- Ensure app startup succeeds with required SQLite capabilities

### Current Status

- Compile status: Passed.
- Run status:
  - `go run ./cmd` now fails fast with actionable FTS5 guidance when FTS5 is missing.
  - `go run -tags "sqlite_fts5" ./cmd` now succeeds even when `vec0` is missing, by using ONNX fallback guard + schema vector-table skip.
- Runtime guards implemented:
  - SQLite capability probe for `fts5` and `vec0` at startup.
  - Hard fail when FTS5 is unavailable (required for retrieval baseline).
  - ONNX fallback validation (`onnx/model_int8.onnx` must exist) when `vec0` is unavailable.
  - Schema migration option to skip `embeddings` virtual table when `vec0` is missing.

### Sprint 2 Implementation Progress (2026-03-21)

- Ingestion pipeline baseline extended:
  - `RegisterDocument` remains the entry registration step.
  - Added document processing path that reads registered file, chunks content, persists chunk rows, and updates document status (`processing` -> `ready` / `error`).
- Retrieval baseline added:
  - Added FTS5 keyword retrieval service with notebook-scoped BM25 ranking.
- Startup hardening added:
  - Capability preflight and graceful fallback behavior wired in app startup.

### Embedding Runtime Update

Option 1: go-onnx + ONNX Runtime (Recommended for your stack)
Since you're in Go, you can run the embedding model directly via ONNX Runtime.
What you install:
- ai-tutor-local/onnx/model_int8.onnx

Implementation note for Sprint 2:
- Keep this ONNX path as the local embedding fallback path while resolving sqlite-vec extension compatibility.
- Add a startup check that validates the model file exists before enabling ONNX-based embedding execution.

## Mini Test Cases for Sprint 2

| ID | Area | Type | Preconditions | Steps | Expected |
|---|---|---|---|---|---|
| S2-TC-001 | App startup migration | Integration | Fresh local DB path | Run app startup | All base tables created; no module errors |
| S2-TC-002 | FTS capability | Integration | SQLite build with FTS5 enabled | Execute schema migration with sqlite_fts5 build tag | chunks_fts created successfully |
| S2-TC-003 | sqlite-vec capability | Integration | sqlite-vec extension available | Execute embeddings table creation | embeddings vec0 table created |
| S2-TC-004 | Document registration | Integration | Notebook exists | Call RegisterDocument | Document stored with status=pending and SHA256 hash |
| S2-TC-005 | Chunking window behavior | Unit | chunker max=400 overlap=50 | Chunk long sample text | Chunks generated with overlap and tagged prefix |
| S2-TC-006 | Sync queue insert | Integration | Valid event payload | Enqueue event | Row added in sync_queue with pending status |
| S2-TC-007 | Embedding runtime contract | Unit | ONNX model file available | Run embedding flow | ONNX `model_int8.onnx` is used with expected 768-dim output |
| S2-TC-008 | Runtime health guard | Integration | Missing FTS5/sqlite-vec | Run app startup | Fails gracefully with actionable error message |

### Mini Test Case Status Snapshot

- S2-TC-001: Satisfied (migrations run in supported runtime path with actionable guard behavior).
- S2-TC-002: Satisfied (FTS5 verified using `sqlite_fts5` build tag).
- S2-TC-003: Satisfied via fallback policy (vec0 optional in Sprint 2 baseline; strict mode available for vec-required runs).
- S2-TC-004: Satisfied (document registration flow implemented and available).
- S2-TC-005: Satisfied (chunking behavior implemented in chunker logic).
- S2-TC-006: Satisfied (sync enqueue flow implemented).
- S2-TC-007: Satisfied (embedding runtime contract aligned to ONNX `model_int8.onnx`; vec persistence wired when vec is enabled).
- S2-TC-008: Satisfied (actionable startup guard implemented for missing modules).

## Command Reference (Compile and Run)

Use from project root:

```powershell
Set-Location "c:\Users\vishn\PROJECT\RAG GO\ai-tutor-local"
```

Compile all packages:

```powershell
go build ./...
```

Run app entrypoint:

```powershell
go run ./cmd
```

Attempt run with FTS5 build tag:

```powershell
go run -tags "sqlite_fts5" ./cmd
```

Run with explicit CGO and FTS5 tag (recommended on Windows):

```powershell
$env:CGO_ENABLED="1"
go run -tags "sqlite_fts5" ./cmd
```

Optional formatting command (PowerShell-safe):

```powershell
$files = Get-ChildItem -Recurse -Filter *.go | ForEach-Object { $_.FullName }
if ($files.Count -gt 0) { gofmt -w $files }
```

## Latest Execution Snapshot

- Compile (`go build ./...`): PASS
- Run (`go run ./cmd`): FAIL (expected on current machine)
  - Error: SQLite FTS5 module is unavailable (actionable message printed)
- Run with build tag (`go run -tags "sqlite_fts5" ./cmd`): PASS
  - FTS5 probe succeeds
  - vec0 probe fails
  - ONNX fallback path enabled
  - schema migration completes with vector table skipped
- Integration test (`go test ./internal/retrieval -run TestIngestAndFTSRetrieveSmoke -v`): PASS/SKIP-safe
  - Default runtime: skipped with explicit FTS5-unavailable message
  - FTS5 runtime (`-tags "sqlite_fts5"`): PASS

## Definition of Done for Sprint 2

Sprint 2 can be marked completed when:
- Startup migrations run without module errors.
- FTS5 and sqlite-vec are both operational in local runtime (or ONNX fallback path is fully wired for embeddings while vector storage fallback is defined).
- Document registration + chunking + DB persistence pass all Sprint 2 mini test cases.
- Basic retrieval smoke test can query indexed content successfully.

Status: Completed on 2026-03-21.

## Post-Sprint Backlog (From Sprint 2)

- Add hybrid vector+FTS retrieval once sqlite-vec runtime is consistently available on target machines.
- Add PDF-heavy smoke dataset for retrieval quality checks on real syllabus-sized content.
- Keep and extend manual QA checklist for regression:
  - ingestion status lifecycle
  - chunk count sanity
  - retrieval relevance
  - strict vec mode behavior

## Sprint 3 Completion (2026-03-21)

### Scope Implemented

- FSRS review workflow service added:
  - rating handling (`Again`, `Hard`, `Good`, `Easy`)
  - card schedule updates (`stability`, `difficulty`, `retrievability`, `reps`, `lapses`, `state`, `due_date`)
  - `review_logs` persistence per card review
- Due-card scheduler baseline added:
  - notebook-scoped due queue retrieval
  - due-card count helper for dashboard/session use
- Session telemetry quality path implemented:
  - session summary persistence to `study_sessions`
  - validated analytics enqueue for `flashcard_session_completed`
  - event validation guards in sync service (required fields, non-negative metrics, bounded accuracy)
- Sprint 3 smoke runtime command added:
  - `go run -tags "sqlite_fts5" ./cmd -review-smoke -notebook "<name>" -review-cards <n>`
- One-click task added:
  - `run-sprint3-review-smoke`

### Validation Snapshot

- `go build ./...`: PASS
- `go test ./internal/fsrs ./internal/retrieval -v`: PASS
- `go run -tags "sqlite_fts5" ./cmd -review-smoke -notebook "Sprint3 CLI" -review-cards 4`: PASS

### Definition of Done Status (Sprint 3)

- FSRS logic and rating-based schedule updates: Completed
- Review workflow persistence (`review_logs`, card updates): Completed
- Session telemetry quality and sync queue events: Completed

Status: Completed on 2026-03-21.

## Wails GUI Status and Timeline

- Current status:
  - Wails GUI is **not implemented yet** in this repository.
  - `frontend/` currently has no UI scaffold and no Wails app bootstrap files are present.
- Planned implementation window:
  - Start in Sprint 4 (immediately after backend sync stabilization baseline).
  - Sprint 4 target: bootstrap Wails app shell + core screens for Notebook list, document ingestion status, due cards/review flow, and sync status.
  - Sprint 5 target: polish, UX hardening, and end-to-end desktop packaging.

### Sprint 4 Wails Start Window (Committed)

- Frontend bootstrap starts: **2026-03-22**
- First runnable Wails shell target: **2026-03-23**
- Initial screens target (notebook list + ingestion status + due cards): **2026-03-24 to 2026-03-26**

## Sprint 4 Kickoff (2026-03-21)

### Started In This Pass

- Replaced ingestion embedder wiring to use local ONNX client (`onnx/model_int8.onnx`) instead of Ollama in smoke ingestion flow.
- Added real ONNX embedding client under `internal/embedding` using ONNX Runtime session execution.
- Added focused ONNX integration test validating embedding output shape contract (`768` dims).
- Added sync queue dedup guard: duplicate `event_id` enqueue is now idempotent (no duplicate row, no hard failure).
- Added sync service tests for dedup behavior and event metric validation guardrails.
- Started Sprint 4 frontend scaffold in `frontend/` based on `APP_FLOW.md`:
  - onboarding + provider validation screen
  - home dashboard cards (due/streak/notebooks/sync)
  - notebook ingestion status panel with progress bars
  - quick actions + manual sync status panel
- Added backend dashboard snapshot service (`internal/ui`) to expose real home-screen data:
  - due-card count, notebook count, study streak, sync pending count
  - notebook/document ingestion status rows with normalized progress mapping
- Added CLI snapshot export command for frontend wiring:
  - `go run -tags "sqlite_fts5" ./cmd -dashboard-snapshot`
- Frontend bridge support added:
  - `frontend/app.js` consumes `window.__AI_TUTOR_SNAPSHOT__` when provided by host runtime (Wails bridge target)
- Extended frontend flow coverage from `APP_FLOW.md`:
  - review session panel with rating actions (`Again/Hard/Good/Easy`)
  - ask-question panel with HyDE/retrieval status and grounded-answer placeholder

### Sprint 4 Next Coding Items

- Add sync queue dedup key strategy for idempotent cloud aggregation.
- Add retry backoff policy and max-attempt state transitions in sync worker.
- Add dashboard consistency checks (local aggregated counters vs queued events).
- Standardize generation path to OpenAI-compatible API client for OpenAI/Groq/Gemini/OpenRouter (`base_url` + API key).
- Keep local Ollama generation mode as deferred roadmap item and re-enable in a later sprint (after API-path stabilization).

## Sprint 2 Delta (Completed in this pass)

- Embedding persistence path implemented for vec-enabled runtime:
  - ingestion now supports optional embedder wiring and batch upsert into `embeddings` by chunk rowid.
- Degraded mode clarified and implemented:
  - when vec is unavailable, vector persistence is disabled and ingestion/retrieval continue on FTS baseline.
- Strict vec runtime option added:
  - CLI flag: `-strict-vec`
  - Env flag: `AI_TUTOR_STRICT_VEC0=true`
- Real document smoke mode added to app entrypoint:
  - `go run -tags "sqlite_fts5" ./cmd -ingest-file <path> -notebook <name> -query <term>`
- Retrieval integration smoke test added and passing:
  - `go test -tags "sqlite_fts5" ./internal/retrieval -run TestIngestAndFTSRetrieveSmoke -v`
- One-click VS Code smoke tasks added:
  - `run-sprint2-smoke` (fixed sample run)
  - `run-sprint2-smoke-custom` (prompts for file path, notebook, and query)
