# PLAN_SCOPE.md
## Project Scope, Phases & Task Boundaries
> Version 1.0 | March 2025

---

## 1. Project Division

| Track | Owner | Deliverable | Tech Stack |
|---|---|---|---|
| Track A — Local App | Vishnu | Go/Wails desktop app with RAG, FSRS, and sync | Go, Wails, SQLite, sqlite-vec, ONNX Runtime, Ollama |
| Track B — Cloud | Friend | REST API + React teacher dashboard | Node.js, PostgreSQL, React, Tailwind |

Both tracks can proceed in parallel once `DATA_API.md` and `SCHEMA.md` are finalized. The API contract is the only hard dependency between the two tracks.

---

## 2. Phase 1 — MVP (Local App, Track A)

**Goal:** A fully functional standalone desktop app a student can use daily with zero internet.

### In Scope

- PDF upload, text extraction, semantic chunking, embedding (local ONNX model: `onnx/model_int8.onnx`)
- SQLite + `sqlite-vec` for vector storage + FTS5 for keyword search
- Hybrid retrieval (vector + BM25) with HyDE query expansion
- LLM orchestration: Local Mode (Ollama) and API Mode (OpenAI/Gemini/Anthropic)
- Automatic flashcard generation from document chunks (no manual creation)
- FSRS scheduling: stability, difficulty, retrievability tracking per card
- Basic Wails UI: Notebook view, document reader, flashcard review, Q&A interface
- Analytics event logging in SQLite `sync_queue` (data is stored even if sync not yet active)
- Student config storage: name, USN, LLM mode preference

### Out of Scope (Phase 1)

- Cloud sync (no data leaves the machine yet) → Phase 2
- Classroom join via code → Phase 2
- MCQ quiz generation (flashcards only in Phase 1) → Phase 2
- Talk-to-Duck (self-explanation module) → Phase 2
- `.docx` and `.md` file parsers → Phase 2
- Knowledge Graph → Phase 3 / Future
- Multimodal support (tables, maps, diagrams) → Phase 3 / Future

---

## 3. Phase 2 — Classroom Extension (Both Tracks)

**Goal:** Connect the local app to the cloud. Enable teacher oversight. Add quiz system.

### Track A Additions

- Classroom join via 6-character class code
- Telemetry sync module: batch `POST /api/v1/sync` every 15 min + event-triggered
- Offline queue handling (`sync_queue` in SQLite with retry logic)
- MCQ quiz generation + quiz UI flow
- `.docx` and `.md` file support

### Track B Deliverables

- Cloud REST API: all endpoints defined in `DATA_API.md`
- PostgreSQL database with full analytics schema from `SCHEMA.md`
- React teacher dashboard: class overview, student drilldown, weak topic highlights
- Class code generation and student registration flow

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
| Sync type | Periodic (15 min) + event-based | Balance freshness vs. battery/bandwidth |
| Student auth (cloud) | UUID + class code | No account creation required for students |
| Phase 1 formats | PDF only | Scope control |
| Manual flashcard editing | Allowed locally | Schema tracks `source` flag: `ai` vs `user` |
| Python | No | Deployment nightmare for a native desktop app |

---

## 6. Success Criteria

### Phase 1 MVP is complete when:

1. A student can upload a PDF and have flashcards auto-generated within 5 minutes.
2. The FSRS scheduler correctly surfaces due cards daily.
3. A student can ask a natural-language question and receive a grounded answer with source references.
4. The app works 100% offline.

### Phase 2 is complete when:

1. A student can join a class via code and have their activity visible on the teacher dashboard within 30 minutes.
2. A teacher can see class-wide accuracy per topic.
3. Offline sync correctly queues and delivers data on reconnection.

---

## 7. Team Communication Protocol

- `DATA_API.md` and `SCHEMA.md` are **frozen** once Phase 2 starts. Any changes require discussion from both devs.
- New optional fields in the sync payload must be `nullable` in PostgreSQL to avoid breaking existing clients.
- Schema changes use numbered migration files — no direct edits to the live database.
- Both devs run a mock server/client against `DATA_API.md` before integration to verify the contract.
