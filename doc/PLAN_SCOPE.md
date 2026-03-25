# PLAN_SCOPE.md
## Project Scope, Phases & Task Boundaries
> Version 2.0 | March 2026

---

## 1. Project Division

| Track | Owner | Deliverable | Tech Stack |
|---|---|---|---|
| Track A — Local App | Vishnu | Go/Wails desktop app with Guided Task Board, RAG, FSRS, and sync | Go, Wails, SQLite, sqlite-vec, ONNX Runtime, Azure OpenAI (via provider abstraction) |
| Track B — Cloud | Friend | REST API + React teacher dashboard | Node.js, PostgreSQL, React, Tailwind |

Both tracks can proceed in parallel once `DATA_API.md` and `SCHEMA.md` are finalized. The API contract is the only hard dependency between the two tracks.

---

## 2. Phase 1 — MVP (Local App, Track A)

**Goal:** A fully functional local-first desktop app that runs daily study loops with clear guidance and grounded Q&A.

### In Scope

- PDF upload, text extraction, semantic chunking, embedding (local ONNX model: `onnx/model_int8.onnx`)
- SQLite + `sqlite-vec` for vector storage + FTS5 for keyword search
- Hybrid retrieval (vector + BM25/FTS5) with Reciprocal Rank Fusion (RRF)
- LLM orchestration via provider abstraction, with Azure OpenAI as default production provider
- Automatic flashcard generation from document chunks (no manual creation)
- FSRS scheduling: stability, difficulty, retrievability tracking per card
- Guided Task Board in local UI (daily ordered tasks: read -> review -> quiz)
- Basic Wails UI: Notebook view, document reader, flashcard review, quiz, and Q&A interface
- Analytics event logging in SQLite `sync_queue` (data is stored even if sync not yet active)
- Student config storage: name, USN, LLM mode preference

### Out of Scope (Phase 1)

- Cloud sync transport and teacher dashboard visibility -> Phase 2
- Classroom join via code → Phase 2
- Advanced adaptive tutoring loops beyond fixed daily tasks -> Phase 3
- Talk-to-Duck (self-explanation module) → Phase 2
- `.docx` and `.md` file parsers → Phase 2
- Knowledge Graph → Phase 3 / Future
- Multimodal support (tables, maps, diagrams) → Phase 3 / Future

---

## 3. Phase 2 — Classroom Extension (Both Tracks)

**Goal:** Connect the local app to the cloud. Enable teacher oversight. Add quiz system.

### Track A Additions

- Classroom join via 6-character class code
- Classroom Sync settings (dashboard URL, connectivity probe, sync health)
- Telemetry sync module: batch `POST /api/v1/sync` every 15 min + event-triggered
- Offline queue handling (`sync_queue` in SQLite with retry logic)
- Guided Task Board backed by local planning state (`topics`, `daily_tasks`)
- MCQ quiz generation + quiz UI flow
- `.docx` and `.md` file support

### Track B Deliverables

- Cloud REST API: all endpoints defined in `DATA_API.md`
- PostgreSQL database with full analytics schema from `SCHEMA.md`
- React teacher dashboard: class overview, student drilldown, weak topic highlights
- Classroom and student roster management endpoints
- Class code generation and student registration flow

### Track B Handoff Package (Friend)

Before implementation starts, hand over:

1. `doc/AI_TUTOR_CLOUD_HANDOFF.md` (execution checklist)
2. `doc/DATA_API.md` (binding API contract)
3. `doc/SCHEMA.md` (cloud table contract)
4. `doc/APP_FLOW.md` (runtime flow + planned flow)
5. `doc/sprint.md` (timeline + status)

---

## 4. Phase 3 — Advanced Features (Future)

- **Talk-to-Duck:** Student explains a concept aloud; AI evaluates and identifies gaps
- **Time Tracker:** Tracks session duration and activity type automatically
- **Gamification:** Streak heatmap, achievement badges, leaderboard
- **Agentic Retrieval:** AI autonomously searches web when local KB has gaps
- **Knowledge Graph:** Extract concept relationships for multi-hop reasoning
- **Multimodal:** Support for diagrams, tables, and maps in documents
- **Teacher task assignment:** Push study tasks from dashboard to student local app

---

## 4.5 Phase 2.5 - Quality Hardening

- Structured output validation and retry for generation JSON (quiz/flashcard/task payloads)
- Token budget guardrails for retrieval context packing
- Queue and sync observability (clear pending/failed state and diagnostics)
- End-to-end smoke tests for join, sync, guided task progression

---

## 5. Explicit Design Decisions

| Decision | Choice | Reason |
|---|---|---|
| Knowledge Graph | SKIP (Phase 3) | Too slow on local LLMs; scope risk for final year |
| Local backend language | Go only | Single binary, zero deploy overhead |
| Cloud backend language | Node.js | Friend's preference; good ecosystem for REST APIs |
| Embedding model | `onnx/model_int8.onnx` (local ONNX Runtime) | Fixed local model, privacy-safe, predictable dimensions |
| Vector storage | `sqlite-vec` | No extra dependency; same SQLite file |
| Keyword search | SQLite FTS5 (BM25) | Built-in, zero-config |
| Reranker | Optional cross-encoder (local) | Phase 1 may skip; Phase 2 adds it |
| Retrieval fusion | RRF (vector + BM25) | Stable relevance without tight score calibration |
| Sync type | Periodic (15 min) + event-based | Balance freshness vs. battery/bandwidth |
| Student auth (cloud) | UUID + class code | No account creation required for students |
| Phase 1 formats | PDF only | Scope control |
| Manual flashcard editing | Allowed locally | Schema tracks `source` flag: `ai` vs `user` |
| Generation default | Azure OpenAI | Managed quality + predictable latency |
| Python | No | Deployment nightmare for a native desktop app |

---

## 6. Success Criteria

### Phase 1 MVP is complete when:

1. A student can upload a PDF and have flashcards auto-generated within 5 minutes.
2. The FSRS scheduler correctly surfaces due cards daily.
3. A student can ask a natural-language question and receive a grounded answer with source references.
4. The app works offline for ingestion, retrieval, review, and task progression.

### Phase 2 is complete when:

1. A student can join a class via code and have their activity visible on the teacher dashboard within 30 minutes.
2. A teacher can see class-wide accuracy per topic.
3. Offline sync correctly queues and delivers data on reconnection.
4. Guided Task Board state is visible locally and remains consistent across app restarts.

---

## 7. Team Communication Protocol

- `DATA_API.md` and `SCHEMA.md` are **frozen** once Phase 2 starts. Any changes require discussion from both devs.
- New optional fields in the sync payload must be `nullable` in PostgreSQL to avoid breaking existing clients.
- Schema changes use numbered migration files — no direct edits to the live database.
- Both devs run a mock server/client against `DATA_API.md` before integration to verify the contract.
