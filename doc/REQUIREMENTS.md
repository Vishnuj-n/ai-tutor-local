# REQUIREMENTS.md
## Local-First AI Tutoring System with Behavioral Learning & Classroom Analytics
> Version 1.0 | March 2025

---

## 1. System Overview

The **AI Tutoring & Classroom Analytics System** is a hybrid educational platform for high-stakes exam preparation (e.g., UPSC). It has two distinct layers:

1. **Local Learning Engine (System A)** — A privacy-first, offline-capable desktop app using RAG + FSRS to auto-generate and schedule study materials from uploaded documents. Runs entirely on the student's machine (Go + Wails).
2. **Cloud Analytics Layer (System B)** — A centralized web platform that receives lightweight, anonymized telemetry from local apps, letting teachers monitor cohort progress without ever accessing private study content.

---

## 2. Stakeholders

| Stakeholder | Role | Primary Concern |
|---|---|---|
| Student (Primary User) | Uses local desktop app daily | Privacy, offline access, retention |
| Teacher / HOD | Monitors progress via cloud dashboard | Class analytics, weak topic identification |
| Developer A (Vishnu) | Builds Go/Wails local app | Local schema, RAG pipeline, FSRS |
| Developer B (Friend) | Builds Node.js/PostgreSQL cloud dashboard | API contract, analytics schema |

---

## 3. Functional Requirements

### 3.1 Document Ingestion & AI Generation (Local)

- **REQ-1.1** The system shall allow users to upload PDF, TXT, and Markdown files into subject-specific Notebooks.
- **REQ-1.2** The system shall parse, semantically chunk (preserving heading boundaries), and embed uploaded documents using a local ONNX embedding model (`onnx/model_int8.onnx`).
- **REQ-1.3** The system shall automatically generate flashcards, MCQ quizzes, and descriptive prompts from document chunks using the selected LLM. No manual creation required.
- **REQ-1.4** All generated artifacts shall maintain a bidirectional reference to the originating document chunk, enabling click-through navigation back to the source paragraph.

### 3.2 Retrieval Pipeline (Local)

- **REQ-2.1** The system shall perform hybrid search combining vector similarity (`sqlite-vec`) and full-text search (SQLite FTS5 / BM25).
- **REQ-2.2** The system shall optionally apply a local cross-encoder reranker to improve relevance ranking of top-k retrieved chunks.
- **REQ-2.3** The system shall use HyDE (Hypothetical Document Embeddings) to expand user queries before searching, improving retrieval over dense study material.
- **REQ-2.4** Context sent to the LLM shall include retrieved chunks prepended with their document title and heading (e.g., `[Polity - Fundamental Rights] ...`).

### 3.3 Spaced Repetition & Study Engine (Local)

- **REQ-3.1** The system shall implement the FSRS algorithm, tracking stability, retrievability, and difficulty per flashcard.
- **REQ-3.2** The scheduler shall dynamically re-prioritize overdue cards if a student misses study sessions.
- **REQ-3.3** A Timetable System shall accept total exam duration, daily study hours, and per-notebook weightage, and output a global study schedule.

### 3.4 Classroom Sync & Telemetry (Bridge)

- **REQ-4.1** The local app shall allow a student to enter a teacher-generated Class Code to join a cohort.
- **REQ-4.2** The local app shall generate and sync an analytics payload to the cloud API.
  - **PERMITTED:** `student_id`, `name`, `USN`, `class_id`, `notebook_name`, `activity_type`, `time_spent_seconds`, `flashcards_completed`, `quiz_score`, `accuracy_pct`, `current_streak`, `synced_at`
  - **PROHIBITED:** raw document text, embeddings, generated notes, full flashcard content
- **REQ-4.3** Sync shall be event-based (immediately after quiz) AND periodic (every 15 minutes). Offline payloads shall queue in SQLite and send in batch when connectivity is restored.

### 3.5 Teacher Dashboard (Cloud)

- **REQ-5.1** Teachers shall create classes and generate alphanumeric join codes.
- **REQ-5.2** The dashboard shall display class-level aggregated performance: average accuracy per topic, activity heatmaps, streak distributions.
- **REQ-5.3** Teachers shall drill down into individual student performance (accuracy by notebook, session time, quiz scores over time).
- **REQ-5.4** The dashboard shall support teacher queries, e.g., *"Which students have not studied for 3 days?"* or *"Lowest accuracy topic in Polity?"*

---

## 4. Non-Functional Requirements

### 4.1 Privacy & Security
- All unstructured content stays exclusively on student hardware — no exceptions.
- Student identification uses UUID + name + USN. No unnecessary PII beyond cohort management.
- The cloud system must never have an endpoint that can pull document content from the local app.

### 4.2 Performance
- Vector retrieval must complete in under 500ms to maintain uninterrupted study flow.
- Background telemetry sync must be non-blocking and fail gracefully when offline.
- PDF ingestion and embedding must run as a goroutine, never blocking the UI.

### 4.3 Usability
- The local app requires zero manual configuration — SQLite initializes automatically on first launch.
- The app must work fully offline; cloud sync is optional and additive.
- Phase 1 supported formats: **PDF only**. Phase 2 adds `.docx` and `.md`.

---

## 5. System Constraints

| Constraint | Value | Rationale |
|---|---|---|
| Max PDF size (Phase 1) | ~500 pages / ~50MB | Prevent memory overload on low-spec laptops |
| LLM Mode | API-first via OpenAI-compatible protocol; Local Ollama mode deferred | Supports OpenAI/Groq/Gemini/OpenRouter using provider base URL + API key; local mode to be re-enabled in later sprint |
| Embedding Model | `onnx/model_int8.onnx` via local ONNX Runtime | Stable local inference, no network dependency |
| Local DB | SQLite + sqlite-vec + FTS5 | Single file, zero-config, portable |
| Cloud DB | PostgreSQL | Relational, queryable analytics store |
| Sync Frequency | Event-based + every 15 min | Balance freshness vs. bandwidth |
| Knowledge Graph | NOT in Phase 1 or 2 | Too slow for local LLMs; deferred to Phase 3 |

---

## 6. Phase Plan

| Phase | Scope | Owner |
|---|---|---|
| Phase 1 — MVP | Local app: PDF ingestion, RAG, auto flashcards, FSRS scheduler, basic UI | Developer A |
| Phase 2 — Classroom | Cloud API, telemetry sync, Teacher Dashboard (React) | Both |
| Phase 3 — Advanced | Talk-to-Duck, knowledge graph, multimodal, agentic retrieval | Both |
