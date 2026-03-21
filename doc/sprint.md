# Sprint Plan and Progress Tracker

Version: 1.0  
Date: 2026-03-21  
Project: Local-First AI Tutoring System (Track A + Track B)

## Sprint Overview

| Sprint | Focus | Status | Notes |
|---|---|---|---|
| Sprint 0 | Architecture, API contract, schema hardening | Completed | Core docs finalized, critical schema/API issues fixed |
| Sprint 1 | Track A foundation scaffold | Completed | Go module, schema.sql, DB init, models, queries, module skeletons |
| Sprint 2 | Current sprint: ingestion to retrieval pipeline baseline | In Progress | FTS5 guard + vec0 fallback implemented; embedding persistence to vec0 still pending |
| Sprint 3 | FSRS + review workflows + telemetry quality | Planned | FSRS logic and review session metrics |
| Sprint 4 | Classroom sync stabilization + cloud aggregation validation | Planned | Retry, dedup, dashboard consistency |
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

## Current Sprint (Sprint 2): In Progress

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

## Mini Test Cases for Current Sprint (Pending Items)

| ID | Area | Type | Preconditions | Steps | Expected |
|---|---|---|---|---|---|
| S2-TC-001 | App startup migration | Integration | Fresh local DB path | Run app startup | All base tables created; no module errors |
| S2-TC-002 | FTS capability | Integration | SQLite build with FTS5 enabled | Execute schema migration with sqlite_fts5 build tag | chunks_fts created successfully |
| S2-TC-003 | sqlite-vec capability | Integration | sqlite-vec extension available | Execute embeddings table creation | embeddings vec0 table created |
| S2-TC-004 | Document registration | Integration | Notebook exists | Call RegisterDocument | Document stored with status=pending and SHA256 hash |
| S2-TC-005 | Chunking window behavior | Unit | chunker max=400 overlap=50 | Chunk long sample text | Chunks generated with overlap and tagged prefix |
| S2-TC-006 | Sync queue insert | Integration | Valid event payload | Enqueue event | Row added in sync_queue with pending status |
| S2-TC-007 | Embedding request contract | Unit | Mock Ollama endpoint | Call EmbedText | Request uses model nomic-embed-text and decodes embeddings |
| S2-TC-008 | Runtime health guard | Integration | Missing FTS5/sqlite-vec | Run app startup | Fails gracefully with actionable error message |

### Mini Test Case Status Snapshot

- S2-TC-001: Partially satisfied (successful startup path validated with `sqlite_fts5` and vec fallback).
- S2-TC-002: Satisfied (FTS5 verified using `sqlite_fts5` build tag).
- S2-TC-003: Pending (requires runtime with sqlite-vec `vec0` available).
- S2-TC-004: Satisfied (document registration flow implemented and available).
- S2-TC-005: Satisfied (chunking behavior implemented in chunker logic).
- S2-TC-006: Satisfied (sync enqueue flow implemented).
- S2-TC-007: Satisfied (embedding client contract implemented for Ollama `/api/embed`).
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

## Definition of Done for Sprint 2

Sprint 2 can be marked completed when:
- Startup migrations run without module errors.
- FTS5 and sqlite-vec are both operational in local runtime (or ONNX fallback path is fully wired for embeddings while vector storage fallback is defined).
- Document registration + chunking + DB persistence pass all Sprint 2 mini test cases.
- Basic retrieval smoke test can query indexed content successfully.

## Remaining Sprint 2 Work

- Wire embedding persistence end-to-end:
  - If vec0 available: insert vectors into `embeddings` table mapped by chunk rowid.
  - If vec0 unavailable: define temporary non-vec storage behavior clearly (or skip vector retrieval with explicit mode flag).
- Add retrieval smoke integration:
  - ingest sample doc -> query FTS retrieval -> assert expected chunk returned.
- Add optional startup flag/config for forcing strict vec0 requirement in environments where fallback should be disabled.

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
