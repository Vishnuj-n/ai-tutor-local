# APP_FLOW.md
## User Journey & Screen Flow Specification
> Version 2.2 | March 2026

---

## 1. Purpose

This document now separates:

- **Implemented Flow (Current)**: What is actually wired in `ai-tutor-local` today.
- **Planned Flow (Roadmap)**: Features designed in docs but not fully implemented yet.

This split avoids confusion during development and testing.

---

## 2. Local App Flow (Student — System A)

### 2.1 Implemented Flow (Current)

### 2.1.1 App Startup

1. Wails app starts.
2. Local SQLite DB initializes (`data/app.db`).
3. Capability checks run:
   - FTS5 is required.
   - `sqlite-vec` (`vec0`) is optional; app can run without vector table.
4. Schema migrations run.
5. Dashboard RPCs become available.

### 2.1.2 Onboarding / Settings (UI)

1. User enters Student Name + USN + provider + base URL + API key in UI.
2. Frontend performs **format-only validation** (non-empty, URL starts with `http`, key length check).
3. Profile values (except API key) are stored in browser localStorage for UI convenience.
4. User enters Dashboard.

**Current limitation:** provider validation does not call a backend/cloud endpoint yet.

### 2.1.3 Dashboard

Dashboard fetches real snapshot data from backend:

- Due cards today
- Study streak
- Active notebooks
- Pending sync count
- Notebook ingestion rows/status

Sync footer also fetches live queue health from backend (`pending`, `health`, `next_retry`).

Current action wiring status:

- `Start Review`: wired to backend FSRS flow.
- `Upload File` (ingestion card): wired to backend ingestion flow.
- `Ask a Question`: opens panel with demo response path.
- `Upload PDF` (quick action button): placeholder UI action (not wired separately).
- `Classroom Sync` (quick action button): placeholder UI action (not wired separately).

### 2.1.4 File Ingestion Flow

1. User chooses file (`.pdf`, `.txt`, `.md`) and notebook name.
2. Backend creates notebook if not present.
3. Backend registers document record (`pending`).
4. Backend processes document:
   - extract text
   - split by heading heuristics
   - chunk text
   - save chunks
   - if vector store is enabled and embedder is configured, save embeddings
5. Document marked `ready` or `error`.
6. Starter flashcards are auto-generated from first chunks for immediate review.

**Current limitation:** this path is currently request/response (foreground) from UI and can feel blocking on larger files.

### 2.1.5 Review (FSRS) Flow

1. User starts review.
2. Backend returns next due card.
3. User reveals answer and rates recall (`Again/Hard/Good/Easy`).
4. Backend updates FSRS fields and due date, and inserts `review_logs` entry.
5. When user exits review (or queue ends), session summary is stored and telemetry event is queued.

### 2.1.6 Sync Queue Flow (Local Baseline)

1. Events are enqueued in local `sync_queue`.
2. Manual sync currently validates payloads and marks queue status transitions (`pending`/`failed`/`sent`) locally.
3. Retry/backoff metadata is maintained.

**Current limitation:** no live HTTP delivery to cloud API endpoint from this repo yet.

Event compatibility note:

- Local currently emits `flashcard_session_completed` for review completion telemetry.
- API examples use `flashcard_session`.
- Cloud side should temporarily accept both and normalize.

### 2.1.7 Ask a Question Panel (UI)

1. User opens RAG panel and submits question.
2. Frontend currently shows simulated HyDE/hybrid-retrieval response text.

**Current limitation:** RAG panel is not yet wired to backend retrieval + generation RPC.

---

### 2.2 Immediate App Flow Fix Plan (Next Sprint)

1. Convert ingestion to async job style from UI perspective:
   - Start ingestion and return job/document id immediately.
   - Poll status and progress from dashboard snapshot until ready/error.

2. Wire RAG panel to backend RPC:
   - add backend query method
   - call retrieval service
   - return grounded answer + chunk references

3. Wire quick action placeholders:
   - connect `Upload PDF` to same file picker/ingest path
   - connect `Classroom Sync` to settings/class-join flow

4. Align telemetry event naming:
   - standardize local event type to `flashcard_session`
   - keep temporary compatibility parser for old value during migration window

5. Add cloud sync transport:
   - configure API base URL from settings/env
   - send retryable queue items to `/api/v1/sync`
   - treat `200` and `409` as sent

---

### 2.3 Planned Flow (Roadmap)

### 2.3.1 First Launch / Onboarding (Planned Enhancements)

1. App launches → SQLite DB initializes automatically (zero setup required).
2. Onboarding screen: Enter **Student Name** + **USN** (stored locally; used in analytics payload).
3. **LLM Provider selection:** Choose API provider (OpenAI, Groq, Gemini, OpenRouter) and set OpenAI-compatible `base_url` + API key for current session.
   - **Important:** Embeddings are ALWAYS generated locally via ONNX (`onnx/model_int8.onnx`), regardless of this choice.
   - This choice only affects text generation (flashcard & quiz generation, Q&A).
   - API keys are not stored in `student_config`; only in-memory/session use or secure OS keychain reference is allowed.
4. Local Ollama mode is planned for a later sprint and is currently disabled by default.
5. If provider validation fails (invalid key/base URL) → show inline error and block generation until corrected.
6. User lands on **Home Dashboard**.

---

### 2.3.2 Home Dashboard (Planned Enhancements)

Displays at a glance:
- **Today's Due Cards** — count of flashcards due per FSRS schedule
- **Study Streak** — consecutive study days
- **Active Notebooks** — list of subject notebooks
- **Quick Actions** — `[Start Review]` `[Upload PDF]` `[Ask a Question]`

---

### 2.3.3 Notebook Creation & Document Upload (Planned Enhancements)

1. User clicks `[+ New Notebook]` → enters subject name (e.g., `Polity`).
2. User opens Notebook → clicks `[Upload Document]`.
3. System begins **background ingestion pipeline** (non-blocking UI):
   - Extract text from PDF via `pdfcpu`
   - Detect chapter/heading boundaries via structure parsing
   - Semantically chunk the document (300–500 tokens, 50-token overlap)
   - Prepend each chunk: `[NotebookName - HeadingName] chunk text...`
   - Generate embeddings via local ONNX model (`onnx/model_int8.onnx`) → store in `sqlite-vec`
   - Index chunk plain text in FTS5 virtual table
4. UI shows a progress indicator (% of pages processed). Student can do other tasks during this.
5. On completion → notification: *"Document ready. Generating study materials..."*
6. System auto-generates flashcards and quiz questions from chunks using LLM.

---

### 2.3.4 Study / Flashcard Review Flow (Planned Enhancements)

1. User clicks `[Start Review]` → FSRS scheduler selects due cards.
2. Card displayed (question side).
3. User clicks `[Show Answer]`.
4. User rates recall: `[Again]` `[Hard]` `[Good]` `[Easy]`.
5. FSRS updates stability, retrievability, difficulty → schedules next review date.
6. User can click `[View Source]` → app jumps to the exact source paragraph in the original PDF.
7. On session end → session stats pushed to analytics sync queue.

---

### 2.3.5 Quiz Flow (Planned)

1. User opens a Notebook → clicks `[Take Quiz]`.
2. System retrieves relevant chunks, generates 10 questions via LLM (MCQ + descriptive mix).
3. Questions are grounded in retrieved chunks (fact-verified before display).
4. User completes quiz → sees score + correct answers with source references.
5. Quiz result is **immediately** pushed to the telemetry sync queue.

---

### 2.3.6 Ask a Question (RAG Query) (Planned Full Backend Wiring)

1. User types a natural-language question in the Search bar.
2. **HyDE:** LLM generates a hypothetical 2-sentence answer to the question.
3. Hypothetical answer is embedded → `sqlite-vec` ANN search finds top-k chunks.
4. FTS5 keyword search runs in parallel on the same query.
5. Results are merged via Reciprocal Rank Fusion (RRF) → optional cross-encoder reranks top 10.
6. LLM generates a grounded answer using retrieved context.
7. Answer displayed with source chunk references (click to view in PDF).

---

### 2.3.7 Timetable Generation (Planned)

1. User opens `[Timetable]` → enters: Exam date, daily study hours, per-notebook priority weight.
2. System generates a day-by-day study schedule.
3. Schedule shown as a calendar view and is exportable.

---

### 2.3.8 Classroom Join Flow (Planned)

1. User goes to `[Settings]` → `[Classroom Sync]`.
2. User enters Class Code provided by teacher.
3. App validates code with Cloud API (`POST /api/v1/classes/join`).
4. On success: student is linked to `class_id`. Sync is enabled.
5. Periodic sync runs every 15 minutes in background. Manual sync button also available.
6. If offline: payloads queue in SQLite `sync_queue` table → sent on next successful connection.

---

## 3. Cloud Dashboard Flow (Teacher — System B)

### 3.1 Teacher Registration & Class Creation

1. Teacher registers on the web dashboard.
2. Teacher creates a Class → system generates a unique 6-character alphanumeric Class Code.
3. Teacher shares the code with students (WhatsApp, email, etc.).

### 3.2 Class Overview Dashboard

- Active student count and last-seen timestamps
- Average quiz accuracy per topic (bar chart)
- Class-wide activity heatmap (days × hours)
- Students inactive for 3+ days highlighted as "at risk"

### 3.3 Individual Student Drilldown

1. Teacher clicks on a student name.
2. Sees: subject-wise accuracy, session time history, flashcard completion rate, quiz scores over time.
3. "Danger zones" highlighted: topics below 50% accuracy or 3+ days inactive.

### 3.4 Teacher Queries (Supported)

- *Which students have not studied for 3+ days?*
- *What is the class average quiz score for Polity?*
- *Who is weakest in Economy?*
- *Show students with a streak greater than 7 days.*

---

## 4. Error & Edge Case Flows

### 4.1 Current Runtime Behavior

| Scenario | Current Behavior |
|---|---|
| Invalid onboarding input (provider/base URL/API key format) | Inline validation message in onboarding form. |
| Upload canceled (no file selected) | Upload flow exits quietly; no DB changes. |
| Unsupported/missing/invalid file path | Ingestion call returns error and UI shows failure message. |
| Corrupt or empty PDF/text extraction failure | Document is marked error in DB; UI shows ingestion failure message. |
| Large file processing exceeds timeout window | Ingestion request can fail due to foreground timeout (currently 2-minute context). |
| No due cards in review | UI shows "No due cards right now" and ends session summary flow if needed. |
| Sync Now with invalid queued payload | Item is marked failed during validation path; sync status reflects degraded health. |
| Sync Now while offline/cloud unavailable | No network delivery path yet in local app, so this case is not triggered in current runtime. |

### 4.2 Planned/Target Behavior

| Scenario | Planned Behavior |
|---|---|
| Provider endpoint unreachable | Inline warning with retry action and provider/base URL troubleshooting hint. |
| API key invalid (API mode live validation) | Inline error on key entry and block generation actions. |
| Ollama not running (when local mode ships) | Dialog to switch mode or retry local runtime. |
| Cloud sync fails (offline/server error) | Keep payload in `sync_queue` and retry with backoff without disrupting user workflow. |
| LLM generates hallucinated flashcard | Fact-check generated content against retrieved source chunks and discard low-grounded output. |
| Student deletes notebook locally | Send `notebook_deleted` sync event while retaining cloud history. |
| Student leaves class | Stop further sync while preserving archived history on cloud. |
