# Sprint Plan and Progress Tracker

Version: 1.0  
Date: 2026-03-21  
Project: Local-First AI Tutoring System (Track A + Track B)

## Sprint Overview

| Sprint | Focus | Status | Notes |
|---|---|---|---|
| Sprint 0 | Architecture, API contract, schema hardening | Completed | Core docs finalized, critical schema/API issues fixed |
| Sprint 1 | Track A foundation scaffold | Completed | Go module, schema.sql, DB init, models, queries, module skeletons |
| Sprint 2 | Current sprint: ingestion to retrieval pipeline baseline | In Progress | Runtime blocked by SQLite extension availability (FTS5/sqlite-vec) |
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
- Run status: Failing at startup migration due to sqlite-vec module availability in runtime environment.
- Observed blocker:
  - no such module: vec0
- FTS5 status:
  - Fixed when running with build tag: sqlite_fts5

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
- Run (`go run ./cmd`): FAIL
  - Error: no such module: fts5
- Run with build tag (`go run -tags "sqlite_fts5" ./cmd`): FTS5 is available, next error is no such module: vec0

## Definition of Done for Sprint 2

Sprint 2 can be marked completed when:
- Startup migrations run without module errors.
- FTS5 and sqlite-vec are both operational in local runtime (or ONNX fallback path is fully wired for embeddings while vector storage fallback is defined).
- Document registration + chunking + DB persistence pass all Sprint 2 mini test cases.
- Basic retrieval smoke test can query indexed content successfully.
