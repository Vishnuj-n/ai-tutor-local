# APP_FLOW.md
## User Journey & Screen Flow Specification
> Version 1.0 | March 2025

---

## 1. Local App Flow (Student — System A)

### 1.1 First Launch / Onboarding

1. App launches → SQLite DB initializes automatically (zero setup required).
2. Onboarding screen: Enter **Student Name** + **USN** (stored locally; used in analytics payload).
3. **LLM Mode selection:** Choose Local Mode (requires Ollama running locally) or API Mode (enter API key for current session).
   - **Important:** Embeddings are ALWAYS generated locally via ONNX (`onnx/model_int8.onnx`), regardless of this choice.
   - This choice only affects text generation (flashcard & quiz generation, Q&A).
   - API keys are not stored in `student_config`; only in-memory/session use or secure OS keychain reference is allowed.
4. If Local Mode selected and Ollama is not detected → Show warning + prompt to switch to API Mode.
5. User lands on **Home Dashboard**.

---

### 1.2 Home Dashboard

Displays at a glance:
- **Today's Due Cards** — count of flashcards due per FSRS schedule
- **Study Streak** — consecutive study days
- **Active Notebooks** — list of subject notebooks
- **Quick Actions** — `[Start Review]` `[Upload PDF]` `[Ask a Question]`

---

### 1.3 Notebook Creation & Document Upload

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

### 1.4 Study / Flashcard Review Flow

1. User clicks `[Start Review]` → FSRS scheduler selects due cards.
2. Card displayed (question side).
3. User clicks `[Show Answer]`.
4. User rates recall: `[Again]` `[Hard]` `[Good]` `[Easy]`.
5. FSRS updates stability, retrievability, difficulty → schedules next review date.
6. User can click `[View Source]` → app jumps to the exact source paragraph in the original PDF.
7. On session end → session stats pushed to analytics sync queue.

---

### 1.5 Quiz Flow

1. User opens a Notebook → clicks `[Take Quiz]`.
2. System retrieves relevant chunks, generates 10 questions via LLM (MCQ + descriptive mix).
3. Questions are grounded in retrieved chunks (fact-verified before display).
4. User completes quiz → sees score + correct answers with source references.
5. Quiz result is **immediately** pushed to the telemetry sync queue.

---

### 1.6 Ask a Question (RAG Query)

1. User types a natural-language question in the Search bar.
2. **HyDE:** LLM generates a hypothetical 2-sentence answer to the question.
3. Hypothetical answer is embedded → `sqlite-vec` ANN search finds top-k chunks.
4. FTS5 keyword search runs in parallel on the same query.
5. Results are merged via Reciprocal Rank Fusion (RRF) → optional cross-encoder reranks top 10.
6. LLM generates a grounded answer using retrieved context.
7. Answer displayed with source chunk references (click to view in PDF).

---

### 1.7 Timetable Generation

1. User opens `[Timetable]` → enters: Exam date, daily study hours, per-notebook priority weight.
2. System generates a day-by-day study schedule.
3. Schedule shown as a calendar view and is exportable.

---

### 1.8 Classroom Join Flow

1. User goes to `[Settings]` → `[Classroom Sync]`.
2. User enters Class Code provided by teacher.
3. App validates code with Cloud API (`POST /api/v1/classes/join`).
4. On success: student is linked to `class_id`. Sync is enabled.
5. Periodic sync runs every 15 minutes in background. Manual sync button also available.
6. If offline: payloads queue in SQLite `sync_queue` table → sent on next successful connection.

---

## 2. Cloud Dashboard Flow (Teacher — System B)

### 2.1 Teacher Registration & Class Creation

1. Teacher registers on the web dashboard.
2. Teacher creates a Class → system generates a unique 6-character alphanumeric Class Code.
3. Teacher shares the code with students (WhatsApp, email, etc.).

### 2.2 Class Overview Dashboard

- Active student count and last-seen timestamps
- Average quiz accuracy per topic (bar chart)
- Class-wide activity heatmap (days × hours)
- Students inactive for 3+ days highlighted as "at risk"

### 2.3 Individual Student Drilldown

1. Teacher clicks on a student name.
2. Sees: subject-wise accuracy, session time history, flashcard completion rate, quiz scores over time.
3. "Danger zones" highlighted: topics below 50% accuracy or 3+ days inactive.

### 2.4 Teacher Queries (Supported)

- *Which students have not studied for 3+ days?*
- *What is the class average quiz score for Polity?*
- *Who is weakest in Economy?*
- *Show students with a streak greater than 7 days.*

---

## 3. Error & Edge Case Flows

| Scenario | System Behavior |
|---|---|
| PDF upload fails (corrupt file) | Show error toast. Allow re-upload. App does not crash. |
| Ollama not running (Local Mode) | Dialog: "Ollama not detected. Switch to API Mode?" |
| API key invalid (API Mode) | Inline error on key entry. Do not proceed. |
| Sync fails (offline) | Queue payload in `sync_queue`. Retry on next connection. No user disruption. |
| LLM generates hallucinated flashcard | Fact-check against source chunks. Flag/discard if key facts absent from context. |
| Student deletes a notebook locally | Cloud retains historical data. A `notebook_deleted` event is synced. |
| Student leaves a class | Sync stops. Historical data archived on cloud (not deleted). |
