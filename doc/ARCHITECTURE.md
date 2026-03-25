# ARCHITECTURE.md
## System Architecture & Technical Design
> Version 1.1 | March 2026

---

## 1. High-Level Architecture

```
Student Machine                          Cloud
─────────────────────────────            ─────────────────────────
  Wails UI (WebView frontend)
         │
  Go Backend (Task Engine + RAG)
         │
  ┌──────┴──────────────┐
  │  SQLite             │             Node.js REST API
  │  + sqlite-vec       │  ──sync──▶  │
  │  + FTS5             │             PostgreSQL
  └──────┬──────────────┘                  │
         │                            React Dashboard
    Cloud LLM API (Azure OpenAI-first)
    + OpenAI-compatible fallback providers
```

The project lives in two independent repositories that communicate through the versioned REST API defined in `DATA_API.md`. Both tracks can be developed in parallel once the API contract is locked.

---

## 2. Repository Map

| Repo | Owner | Tech Stack | Purpose |
|---|---|---|---|
| `ai-tutor-local` | Developer A | Go, Wails, SQLite, sqlite-vec, ONNX Runtime | Local desktop app |
| `ai-tutor-cloud` | Developer B | Node.js, PostgreSQL, React, Tailwind | Cloud API + teacher dashboard |

---

## 3. System A — Local App Architecture

### 3.1 Module Map

| Module | Responsibility |
|---|---|
| `ingestion` | PDF parsing (pdfcpu), chapter detection, semantic chunking, overlap |
| `embedding` | Runs local ONNX embedding model (`onnx/model_int8.onnx`); stores float32 vectors in sqlite-vec |
| `retrieval` | Hybrid search (sqlite-vec ANN + FTS5 BM25), HyDE query expansion, optional reranker |
| `generation` | `LLMProvider` abstraction (Azure OpenAI-first) for answer/quiz/stream generation |
| `taskengine` | Builds daily guided task board from ingested content (READ → REVIEW → QUIZ sequencing) |
| `fsrs` | Pure Go FSRS-4.5 algorithm; manages stability, difficulty, retrievability per card |
| `scheduler` | Timetable logic — accepts exam config, outputs weighted day-by-day schedule |
| `sync` | Analytics event extraction; manages `sync_queue`; periodic + event-triggered cloud POST |
| `db` | SQLite migrations, query layer, connection pooling |

### 3.2 Concurrency Model

Go routines keep the UI responsive at all times:

- **Ingestion worker pool** — cloud-generation enrichment uses bounded concurrency (10-20 workers) for network-bound calls
- **Sync goroutine** — ticker-based (15 min), independent of UI
- **Generation goroutine** — LLM calls are non-blocking; results streamed back via channels

### 3.3 LLM Provider Strategy

| Mode | Inference Target | Internet | Trigger |
|---|---|---|---|
| API Mode (Primary) | Azure OpenAI deployments (OpenAI-compatible fallback supported) | Yes | User selects provider and enters API key |
| Local Mode (Planned) | Ollama (`localhost:11434`) | No | Deferred to a later sprint; currently disabled by default |

Current implementation is moving to a provider interface (`LLMProvider`) so generation methods (answer/quiz/stream) are backend-agnostic. Local Ollama mode remains on the roadmap.

### 3.3.1 Agentic Router + Tool Registry

Queries are first routed by a cheaper model (for example GPT-4o-mini on Azure) to select a tool path:

- `VectorSearch`
- `GenerateQuiz`
- `QueryAnalytics`
- `GenerateGroundedAnswer`

This keeps cost low while preserving quality for complex requests.

### 3.3.2 Guided Task Board Model

After ingestion completes, the Task Engine creates a sequenced daily plan:

1. `READ` task (topic/chunk context)
2. `REVIEW_FLASHCARDS` task (starter/generated deck)
3. `TAKE_QUIZ` task (topic-scoped quiz)

The frontend dashboard consumes this task list instead of static utility buttons.

Current status: task engine service and tables exist locally, but full dashboard replacement of utility buttons is still in-progress.

### 3.3.3 Structured Output Validation

All generation outputs (quizzes, summaries, task plans) pass strict JSON unmarshal + schema validation in Go.

- Invalid output triggers automatic retry with corrective prompt.
- Malformed output is never silently accepted into planning tables.

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
- Text generation currently uses OpenAI-compatible Chat Completions API for OpenAI, Groq, Gemini, and OpenRouter.
- Optional local Ollama generation mode is deferred (kept for later implementation).

**Configuration in `student_config`:**
```
'llm_mode'         → 'api' | 'local'       ('local' reserved for planned Ollama mode)
'api_provider'     → 'openai' | 'groq' | 'gemini' | 'openrouter'
'api_base_url'     → provider OpenAI-compatible base URL
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
Topic extraction + objective mapping → `topics`
    │
TaskEngine sequencing → `daily_tasks`
    │
GenerationService → flashcard + quiz generation per topic/task
```

### 3.5 Data Flow — RAG Query

```
User question
    │
Router model chooses tool path
    │
HyDE: LLM generates hypothetical 2-sentence answer (retrieval path)
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
LLM generates grounded answer (Azure OpenAI)
    │
Answer streamed token-by-token + source links shown to user

---

## 3.6 Local Planning and Telemetry Tables

- `topics`: extracted structured syllabus/topic units linked to notebooks
- `daily_tasks`: sequenced guided tasks (`READ`, `REVIEW_FLASHCARDS`, `TAKE_QUIZ`) with status and due dates
- `educational_telemetry`: educational progress telemetry for teacher analytics
- `ai_diagnostic_telemetry`: AI diagnostics telemetry (latency, token usage, retries, provider errors)
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
