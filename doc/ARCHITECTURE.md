# ARCHITECTURE.md
## System Architecture & Technical Design
> Version 1.0 | March 2025

---

## 1. High-Level Architecture

```
Student Machine                          Cloud
─────────────────────────────            ─────────────────────────
  Wails UI (WebView frontend)
         │
  Go Backend (orchestrator)
         │
  ┌──────┴──────────────┐
  │  SQLite             │             Node.js REST API
  │  + sqlite-vec       │  ──sync──▶  │
  │  + FTS5             │             PostgreSQL
  └──────┬──────────────┘                  │
         │                            React Dashboard
  Ollama / API LLM
  (local process)
```

The project lives in two independent repositories that communicate through the versioned REST API defined in `DATA_API.md`. Both tracks can be developed in parallel once the API contract is locked.

---

## 2. Repository Map

| Repo | Owner | Tech Stack | Purpose |
|---|---|---|---|
| `ai-tutor-local` | Developer A | Go, Wails, SQLite, sqlite-vec, ONNX Runtime, Ollama | Local desktop app |
| `ai-tutor-cloud` | Developer B | Node.js, PostgreSQL, React, Tailwind | Cloud API + teacher dashboard |

---

## 3. System A — Local App Architecture

### 3.1 Module Map

| Module | Responsibility |
|---|---|
| `ingestion` | PDF parsing (pdfcpu), chapter detection, semantic chunking, overlap |
| `embedding` | Runs local ONNX embedding model (`onnx/model_int8.onnx`); stores float32 vectors in sqlite-vec |
| `retrieval` | Hybrid search (sqlite-vec ANN + FTS5 BM25), HyDE query expansion, optional reranker |
| `generation` | LLM orchestration — routes to Ollama (Local) or OpenAI/Gemini/Anthropic (API); flashcard + quiz gen |
| `fsrs` | Pure Go FSRS-4.5 algorithm; manages stability, difficulty, retrievability per card |
| `scheduler` | Timetable logic — accepts exam config, outputs weighted day-by-day schedule |
| `sync` | Analytics event extraction; manages `sync_queue`; periodic + event-triggered cloud POST |
| `db` | SQLite migrations, query layer, connection pooling |

### 3.2 Concurrency Model

Go routines keep the UI responsive at all times:

- **Ingestion goroutine** — PDF parsing, chunking, embedding run fully async
- **Sync goroutine** — ticker-based (15 min), independent of UI
- **Generation goroutine** — LLM calls are non-blocking; results streamed back via channels

### 3.3 LLM Mode Toggle

| Mode | Inference Target | Internet | Trigger |
|---|---|---|---|
| Local Mode | Ollama (`localhost:11434`) | No | User selects in Settings; Ollama must be running |
| API Mode | OpenAI / Gemini / Anthropic endpoint | Yes | User selects + provides API key |

All LLM calls are routed through a single `LLMClient` interface in Go — adding a new provider requires only a new implementation of that interface, no downstream code changes.

### 3.3.1 CRITICAL: Embedding Model Strategy (Decoupled from Generation)

**Architecture Decision: Option A (Mandatory)**

Embedding and text-generation models are **decoupled**. This system uses **local embeddings only** (ONNX `onnx/model_int8.onnx`, 768 dimensions) regardless of the generation mode.

**Why Decoupling is Non-Negotiable:**

- `onnx/model_int8.onnx` (768-dim) is hardcoded in the `sqlite-vec` schema
- OpenAI's `text-embedding-3-small` outputs 1536 dimensions, which will cause a hard crash when inserted into the 768-dim vector column
- Switching embedding models per-provider would require dynamic schema changes (impossible at runtime)
- Privacy: Local embeddings ensure sensitive study content never leaves the student's machine

**Rule:**
- ✅ **ALWAYS use local ONNX embedding runtime with `onnx/model_int8.onnx`**, even in API mode
- ❌ **NEVER attempt to use cloud embedding APIs** (OpenAI embed, Gemini embed, etc.)
- Text generation CAN use either Local (Ollama) or API mode (OpenAI/Gemini/Anthropic)

**Configuration in `student_config`:**
```
'llm_mode'         → 'local' | 'api'       (applies ONLY to generation)
'embedding_mode'   → always 'onnx'         (immutable; never changes)
'onnx_model_path'  → 'onnx/model_int8.onnx'
```

This separation prevents the dimensional mismatch that would corrupt the vector store.

### 3.4 Data Flow — Ingestion

```
User drops PDF
    │
IngestionService.Start(filePath, notebookID)
    │
pdfcpu → raw text per page
    │
ChunkingService → semantic chunks (300–500 tokens, 50-token overlap)
    │
Tag each chunk: "[NotebookName - HeadingName] chunk text..."
    │
EmbeddingService → ONNX embed (`onnx/model_int8.onnx`) → float32[] stored in sqlite-vec
    │
FTS5 virtual table indexed with chunk plain text
    │
GenerationService → auto flashcard + quiz gen per chapter
```

### 3.5 Data Flow — RAG Query

```
User question
    │
HyDE: LLM generates hypothetical 2-sentence answer
    │
Embed hypothetical answer → sqlite-vec ANN (top 20)
    │                    ╲
    │               FTS5 BM25 keyword search (top 20)
    │                    ╱
Reciprocal Rank Fusion (RRF) → merged top results
    │
Optional: cross-encoder reranker → top 5
    │
Assemble context (chunks + doc summaries)
    │
LLM generates grounded answer
    │
Answer + source links shown to user
```

---

## 4. System B — Cloud Architecture

### 4.1 API Server

- **Framework:** Node.js with Express or Fastify
- **Auth:** JWT tokens for teacher sessions. Students authenticate via `UUID + class_code` only (no account creation required).
- **Rate Limiting:** Analytics ingestion endpoint rate-limited per `student_id`.

### 4.2 Key Database Tables (see `SCHEMA.md` for full definitions)

| Table | Purpose |
|---|---|
| `teachers` | Teacher auth records |
| `classes` | Class metadata + join codes |
| `students` | UUID, name, USN, class membership |
| `analytics_events` | Append-only log of all incoming telemetry |
| `aggregated_stats` | Pre-computed per-student, per-notebook summaries |

### 4.3 Teacher Dashboard (React)

- React + Tailwind CSS, served as a SPA
- Charts: Recharts or Chart.js for heatmaps, accuracy bars, score timelines
- Polling every 30 seconds for updated activity (WebSocket optional in Phase 3)

---

## 5. Repository Folder Structure

### `ai-tutor-local` (Go/Wails)

```
ai-tutor-local/
  cmd/                  # Main entry point (main.go)
  internal/
    ingestion/          # PDF parsing, chunking, chapter detection
    embedding/          # ONNX embedding runtime integration
    retrieval/          # Hybrid search, HyDE, reranker
    generation/         # LLM orchestration, flashcard/quiz gen
    fsrs/               # FSRS-4.5 algorithm implementation
    scheduler/          # Timetable logic
    sync/               # Analytics extractor + cloud sync goroutine
    db/                 # SQLite migrations, query helpers
  frontend/             # Wails WebView UI (HTML/JS/CSS)
  schema.sql            # SQLite schema (source of truth)
```

### `ai-tutor-cloud` (Node.js)

```
ai-tutor-cloud/
  src/
    routes/             # API route handlers
    services/           # Business logic layer
    db/                 # PostgreSQL connection + migrations
    middleware/         # Auth, rate limiting, validation
  dashboard/            # React frontend (built separately)
  migrations/           # Numbered SQL migration files
  schema.sql            # PostgreSQL schema (source of truth)
```

---

## 6. Key Architectural Decisions

| Decision | Choice | Reason |
|---|---|---|
| No Knowledge Graph | Skipped (Phase 3) | Local LLMs too inconsistent for entity extraction at scale |
| Python vs Go | Go only (local) | Single binary, zero deploy overhead, zero idle RAM |
| Vector DB | sqlite-vec | No extra dependency; same SQLite file as all other data |
| Keyword search | FTS5 (BM25) | Built into SQLite, zero-config, no extra process |
| Query expansion | HyDE | Improves chunk retrieval for dense study text without complexity |
| Chunk tagging | `[Notebook - Chapter]` prefix | Gives LLM heading context without needing a graph |
