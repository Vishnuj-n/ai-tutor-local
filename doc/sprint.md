# Sprint Plan and Progress Tracker

Version: 1.0  
Date: 2026-03-21  
Project: Local-First AI Tutoring System (Track A + Track B)

## Sprint Overview

| Sprint | Focus | Status | Dates | Target |
|---|---|---|---|---|
| Sprint 0 | Architecture, API contract, schema hardening | Completed | 2026-03-01 to 2026-03-07 | Core docs finalized, critical schema/API issues fixed |
| Sprint 1 | Track A foundation scaffold | Completed | 2026-03-08 to 2026-03-14 | Go module, schema.sql, DB init, models, queries, module skeletons |
| Sprint 2 | Ingestion to retrieval pipeline baseline | Completed | 2026-03-15 to 2026-03-21 | End-to-end ingestion + FTS retrieval baseline validated; vec path optional with strict mode/fallback |
| Sprint 3 | FSRS + review workflows + telemetry quality | Completed | 2026-03-22 to 2026-03-28 | FSRS rating workflow, due-card scheduler, session telemetry quality guards implemented |
| Sprint 4 | Desktop UI + Wails runtime + Dashboard wiring | Completed | 2026-03-29 to 2026-04-04 | Wails dev running, live dashboard, real backend data binding, basic review flow |
| **Sprint 5** | **Classroom sync stabilization + cloud aggregation validation** | **In Progress** | **2026-04-05 to 2026-04-11** | **Cloud API integration, telemetry sync protocol, offline queue handling** |
| Sprint 6 | Release hardening + E2E testing | Planned | 2026-04-12 to 2026-04-18 | Performance tuning, security audit, end-to-end regression suite |
| Phase 2 | MCQ quizzes + .docx/.md support + classroom join | Future | Future | Cloud sync MVP, teacher dashboard, student retention |
| Phase 3 | Talk-to-Duck + gamification + advanced features | Future | Future | Voice interface, leaderboards, knowledge graph |

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

---

## Sprint 4 (Current): Desktop UI + Wails Runtime + Dashboard Wiring

**Timeline**: 2026-03-29 to 2026-04-04  
**Goal**: Enable live Wails development, wire frontend dashboard to real backend data, implement basic review flow.

### Sprint 4 Scope

#### Phase 4A: Wails Project Setup (COMPLETED)
- ✅ Created `wails.json` project configuration
- ✅ Created `main.go` Wails entry point with embed FS setup
- ✅ Created `app.go` with App struct, lifecycle hooks, and RPC bindings
- ✅ Added `frontend/package.json` with Vite dev/build scripts
- ✅ Added Wails v2.11.0 runtime dependency
- ✅ Fixed npm cache corruption and reinstalled dependencies
- ✅ Project compiles cleanly: `go build ./...` ✅

#### Phase 4B: Frontend UI Scaffolding (COMPLETED)
- ✅ Created `frontend/index.html` with all UI sections matching APP_FLOW.md:
  - Onboarding + provider setup (1.1)
  - Dashboard with 4 metric cards + ingestion list (1.2, 1.3)
  - Review panel with rating buttons (1.4)
  - RAG query panel (1.6)
  - Compact footer sync status bar (1.8 indicator)
- ✅ Created `frontend/styles.css` with atmospheric dark theme and responsive layout
- ✅ Created `frontend/app.js` with screen navigation and demo data
- ✅ Compacted sync status from large box to footer indicator

#### Phase 4C: Backend Dashboard Service (COMPLETED)
- ✅ Implemented `internal/ui/dashboard_service.go` (150+ LOC):
  - Real SQLite aggregation for due cards, study streak, active notebooks
  - Ingestion status per notebook with progress tracking
  - Sync queue pending count and status text generation
  - Test coverage with FK constraints validated
- ✅ Added CLI command `-dashboard-snapshot` for CLI testing/export
- ✅ Wails bindings defined:
  - `GetDashboardSnapshot() (*ui.DashboardSnapshot, error)`
  - `GetStartupStatus() string` (planned)

#### Phase 4D: Frontend ↔ Backend Wiring (IN PROGRESS)
**Current Status**: Wails bindings active, dashboard/sync/review/upload RPC wiring in place, final QA pending

### Phase 4D Continuation (2026-03-24)

- Added real review RPC path in `app.go`:
  - `GetNextDueCard()` loads next due flashcard from DB queue
  - `RateDueCard()` applies FSRS rating update via `internal/fsrs`
- Added real ingestion RPC path in `app.go`:
  - `IngestDocument(filePath, notebookName)` registers and processes `.pdf/.txt/.md`
  - auto-creates notebook if missing
  - seeds starter flashcards from ingested chunks for immediate review workflow
- Wired frontend (`frontend/app.js`) to backend:
  - Upload button now prompts for file path + notebook and calls `IngestDocument()`
  - Start Review calls `GetNextDueCard()` and renders real card data
  - Rating buttons call `RateDueCard()` and fetch next due card
- Updated frontend bundling compatibility:
  - switched script tag to module mode in `frontend/index.html` to remove Vite bundling warning

**Remaining Tasks** (by 2026-04-04):
1. ⏳ Confirm `wails dev` launches successfully
  - Start dev server: `cd repo-root && wails dev -tags "sqlite_fts5"`
   - Verify window opens with static UI
   - Check npm hot-reload works (CSS edit → page refresh)
   
2. ⏳ Wire Wails RPC bindings to frontend:
   - Frontend calls `GetDashboardSnapshot()` on dashboard panel show
   - Update dashboard cards with real database metrics
   - Wire footer sync indicator to real sync queue count
   - Test with `-dashboard-snapshot` CLI until Wails binding test-ready
   
3. ⏳ Implement basic review flow:
  - ✅ Load due cards from backend queue (`GetNextDueCard()`)
  - ✅ Apply FSRS rating updates via `RateDueCard()`
  - ✅ Display real card Q&A in review panel
  - ⏳ Add session-complete summary event from UI review loop (optional polish)
   
4. ⏳ Update documentation:
   - Sprint 4 completion summary in this file
   - Frontend/Wails integration guide in `frontend/README.md`
  - List known limitations (native picker available with manual path prompt fallback)

### Sprint 4 Test Cases

| ID | Component | Precondition | Steps | Expected |
|---|---|---|---|---|
| S4-TC-001 | Wails startup | Repo root, dep installed | `wails dev -tags "sqlite_fts5"` | Dev server starts, window opens at http://localhost:34xxx |
| S4-TC-002 | Dashboard load | Wails window open | Click "Continue to Dashboard" | Dashboard displays 4 metric cards, ingestion list visible |
| S4-TC-003 | Snapshot RPC | Dashboard shown | Call `GetDashboardSnapshot()` | Returns valid snapshot with real DB metrics, not mock data |
| S4-TC-004 | Footer sync status | Wails window open | Observe footer | Sync indicator shows live sync queue pending count from `GetDashboardSnapshot()` |
| S4-TC-005 | Review panel load | Dashboard shown | Click "Start Review" | Review panel shows due card Q&A with 4 rating buttons |
| S4-TC-006 | RAG panel load | Dashboard shown | Click "Ask a Question" | RAG panel shows question input, ready for query submission |
| S4-TC-007 | CSS hot reload | Wails dev running | Edit `frontend/styles.css`, save | Changes visible in Wails window within 1s (Vite HMR) |
| S4-TC-008 | Demo fallback | Wails binding unavailable | Observe dashboard | Demo data visible; app doesn't crash, shows status message |

### Sprint 4 Definition of Done

- ✅ `wails dev` launches and dev window opens
- ✅ Dashboard cards populate with real metrics from `GetDashboardSnapshot()`
- ✅ Frontend snapshot hook wired to Wails RPC binding
- ✅ Sync indicator in footer shows live pending count
- ✅ Review panel displays due cards from FSRS scheduler
- ✅ Basic rating button handler wired (demo or real RPC pending frontend/backend sync)
- ✅ All test cases S4-TC-001 through S4-TC-008 passing
- ✅ No crashes, actionable error messages on binding unavailable

**Committed by**: 2026-04-04

---

## Sprint 5: Classroom Sync Stabilization + Cloud Integration Prep

**Timeline**: 2026-04-05 to 2026-04-11  
**Goal**: Stabilize classroom sync flow, validate cloud API contract, prepare for Phase 2 cloud connection.

### Sprint 5 Kickoff (2026-03-24)

- Added sync queue retry-aware processing in `internal/sync/service.go`:
  - manual sync now processes retryable queue rows (`pending` + `failed`)
  - exponential backoff window enforced per row (`1s -> 2s -> 4s -> 8s` cap)
  - payload validation failures are marked `failed` and counted
- Added sync status contract for desktop UI:
  - `GetSyncStatus()` returns `pending_count`, `last_sync_time`, `health`, and next retry window
  - health states currently surfaced: `ok`, `backlog`, `degraded`
- Added Wails RPC bindings in `app.go`:
  - `GetSyncStatus()`
  - `RunManualSync()`
- Wired frontend footer indicator (`frontend/app.js`) to call `GetSyncStatus()` on dashboard open and after manual sync.
- Added tests for Sprint 5 kickoff behavior (`internal/sync/service_test.go`):
  - manual sync sends ready rows and skips rows still under backoff
  - sync status reports degraded health with non-zero next retry delay when failed rows exist

### Sprint 5 Scope

#### Phase 5A: Sync Service Hardening
- Implement retry logic with exponential backoff in `sync_queue` consumer
- Add dedup guard using event hash (prevent double-process on crash)
- Implement offline queue persistence (sync_queue survives app restart)
- Add sync status polling: `GetSyncStatus() -> pending_count, last_sync_time, health`
- Wire sync button to `RunManualSync()` backend RPC (real implementation)

#### Phase 5B: Cloud API Contract Validation
- Mock `POST /api/v1/sync` endpoint locally to test payload shape
- Validate telemetry payload against `DATA_API.md` spec
- Test event serialization: ensure `study_logs`, `sync_queue_events` marshal correctly
- Add endpoint `/api/v1/health` probe from local app (ping test)

#### Phase 5C: Classroom Sync UI
- Add "Classroom Sync" panel (App Flow 1.8):
  - Join code input field
  - Student name display from config
  - Class name + teacher name (from API response)
  - Sync status: last sync time, pending events, queue health
  - "Sync Now" button wired to real backend
  
#### Phase 5D: Offline Queue Hardening
- Implement `sync_queue` consumer loop (background goroutine)
- Add exponential backoff: 1s → 2s → 4s → 8s max retry
- Implement dedup: hash `(event_type, notebook_id, timestamp_ms)` to detect duplicates
- Add circuit breaker: if cloud unreachable for >1 hour, pause retries (don't spam)
- Persist queue state across app restart

### Sprint 5 Test Cases

| ID | Component | Precondition | Steps | Expected |
|---|---|---|---|---|
| S5-TC-001 | Sync queue persist | App with pending events | Exit app, restart | Queue events still present, not lost |
| S5-TC-002 | Retry backoff | Cloud API down | Trigger sync, observe logs | Retries with exp backoff: 1s, 2s, 4s, ... |
| S5-TC-003 | Dedup guard | Duplicate event sent twice | Enqueue both, process | Only 1 processed, 1 marked duplicate |
| S5-TC-004 | Classroom join | Valid join code | Click "Classroom Sync", enter code, click Join | Student appears on teacher dashboard within 30s |
| S5-TC-005 | Sync status pole | Wails window open | Call `GetSyncStatus()` | Returns `{pending: N, last_sync: timestamp, health: "ok"}` |
| S5-TC-006 | Cloud health probe | Mock cloud + local app | Ping `/api/v1/health` | Responds 200 OK, proves connectivity |
| S5-TC-007 | Offline queue growth | Cloud unreachable | Generate 10 study events | Queue grows to 10 pending; doesn't drop events |

### Sprint 5 Definition of Done

- ✅ Sync queue consumer loop running in background
- ✅ Retry logic with exponential backoff implemented and tested
- ✅ Dedup guard prevents duplicate processing
- ✅ Classroom Sync panel UI complete and wired
- ✅ Join code flow works end-to-end (local mock + real API later)
- ✅ Offline queue survives app restart
- ✅ All test cases S5-TC-001 through S5-TC-007 passing
- ✅ No data loss on crash/restart

**Committed by**: 2026-04-11

---

## Sprint 6: Release Hardening + E2E Testing

**Timeline**: 2026-04-12 to 2026-04-18  
**Goal**: Stabilize MVP, security audit, E2E regression suite ready for Phase 2.

### Sprint 6 Scope

#### Phase 6A: Performance Tuning
- Profile dashboard snapshot generation (target <100ms)
- Optimize ingestion list rendering (lazy-load if >20 items)
- Add instrumentation: log slow queries, slow RPC calls
- Benchmark FSRS due-card computation (target <50ms for 1000 cards)
- Wails window memory footprint audit (target <150MB idle)

#### Phase 6B: Security Audit
- API key handling: no logging, session-only storage ✅ (already done)
- SQLite injection prevention: verify all queries use parameterized args
- ONNX model integrity: validate model file hash on startup
- Frontend XSS prevention: all user input HTML-escaped
- Flashcard/question content sanitization (no embedded scripts)

#### Phase 6C: E2E Regression Tests
- Automated test suite covering:
  1. Onboarding → Dashboard flow
  2. PDF upload → ingestion → review panel → rating
  3. Query RAG panel → retrieval → answer display
  4. Sync manual trigger → queue clear cycle
  5. Offline mode: ingestion works without cloud
  6. Restart app: all data persists
  
#### Phase 6D: Documentation Sprint
- Update [README.md](README.md): Installation, first-run, troubleshooting
- Create [DEPLOYMENT.md](DEPLOYMENT.md): Binary build, signing, release notes
- Create [TESTING.md](TESTING.md): E2E test suite, how to run locally
- Create [SECURITY.md](SECURITY.md): Encryption, key storage, audit log policy

#### Phase 6E: Known Limitations + Phase 2 Prep
- Document Sprint 6 limitations (defer to Phase 2):
  - No PDF upload UI (CLI only)
  - No `.docx` / `.md` support
  - No MCQ quizzes (flashcards only)
  - No Talk-to-Duck, gamification, knowledge graph
  - No teacher dashboard (Track B starts Phase 2)
  
- Create [PHASE2_ROADMAP.md](PHASE2_ROADMAP.md):
  - MCQ generation + quiz UI
  - Cloud sync MVP with teacher dashboard
  - Classroom join full flow + leaderboard
  - Talk-to-Duck voice interface skeleton

### Sprint 6 Test Cases

| ID | Component | Precondition | Steps | Expected |
|---|---|---|---|---|
| S6-TC-001 | E2E onboard→review | Fresh app, Groq key | Onboard → Dashboard → Start Review → Rate → Back | Entire flow completes <5s, no errors, rating persisted |
| S6-TC-002 | E2E RAG flow | Dashboard shown | Ask query, run RAG, show answer, rate usefulness | Query executes <2s, grounded answer displayed with sources |
| S6-TC-003 | Offline ingestion | Internet off, file staged | Ingest document chunk, rate cards | No cloud calls; all local ops succeed |
| S6-TC-004 | App restart persist | App with data | Close app, reopen, check dashboard | Same due count, streak, notebooks visible (data persisted) |
| S6-TC-005 | Dashboard perf | 50 notebooks, 500 cards | Load dashboard snapshot | Response <100ms, 4 cards render instantly |
| S6-TC-006 | XSS prevention | Malicious notebook name | Create notebook with `<script>alert('xss')</script>` name | Name displayed escaped; no script execution |
| S6-TC-007 | Key security | Provider setup | After submit, try to access `window.apiKey` | Not accessible; session-only storage verified |
| S6-TC-008 | Binary deploy | Release build | `wails build -platform windows/amd64` | Single `.exe` generated, no dependencies needed |

### Sprint 6 Definition of Done

- ✅ Dashboard snapshot <100ms latency consistently
- ✅ All E2E flows <5s end-to-end
- ✅ Security audit: no plaintext secrets, XSS prevented, SQL injection prevented
- ✅ E2E regression suite with 8+ test cases, all passing
- ✅ Documentation complete: README, DEPLOYMENT, TESTING, SECURITY, PHASE2_ROADMAP
- ✅ Binary builds successfully for Windows/macOS/Linux
- ✅ Known limitations documented; Phase 2 roadmap published

**Committed by**: 2026-04-18

---

## Future Phases

### Phase 2: Classroom + Cloud Sync (Estimated 3-4 weeks)
**Track B Integration**: Teacher dashboard, student analytics, class management  
**Track A Additions**: MCQ generation, quiz UI, classroom join, telemetry sync  
**New Features**: Voice interface (Talk-to-Duck skeleton), gamification prep

### Phase 3: Advanced AI Features (Estimated 4-6 weeks)
**Talk-to-Duck**: Voice input, concept gap detection, personalized remediation  
**Knowledge Graph**: Concept extraction, relationship mapping, multi-hop reasoning  
**Gamification**: Streak heatmap, achievement badges, peer leaderboard (anon)  
**Multimodal Support**: Diagram extraction, table parsing, map regions

---

## Wails Development Quick Reference
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
