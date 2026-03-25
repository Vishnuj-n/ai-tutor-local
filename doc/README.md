# AI Tutoring & Classroom Analytics System

> A modular, local-first AI tutoring system that combines semantic retrieval (RAG) and behavioral learning science (FSRS) to automate studying, optimize memory retention, and provide optional classroom analytics.

**One-line pitch:** Transform passive reading into active, AI-driven learning — entirely on your laptop, with optional teacher oversight via a cloud analytics dashboard.

> Doc status: Updated for Sprint 5 handoff (March 2026)

---

## Repositories

| Repo | Description | Owner |
|---|---|---|
| `ai-tutor-local` | Go/Wails desktop app — RAG, FSRS, flashcards, PDF ingestion | Vishnu |
| `ai-tutor-cloud` | Node.js API + React Teacher Dashboard — analytics, class management | Friend |

---

## What It Does

### For Students (Local App)
- Upload any PDF study material. The system reads, chunks, and understands it automatically.
- AI generates flashcards and quiz questions — no manual effort required.
- FSRS spaced repetition schedules exactly when to review each card for maximum retention.
- Ask questions in natural language; get grounded answers with references to the source text.
- Works **100% offline**. No private data ever leaves your machine.

### For Teachers (Cloud Dashboard)
- Create a class and share a join code with students.
- Monitor quiz accuracy, study streaks, and activity by subject across the cohort.
- Identify weak topics and at-risk students — without ever seeing private study content.

---

## Tech Stack

| Component | Technology |
|---|---|
| Desktop Framework | Go + Wails (native WebView) |
| AI Inference (Local) | ONNX embeddings (`onnx/model_int8.onnx`) |
| AI Inference (Generation) | OpenAI-compatible Chat Completions client (OpenAI, Groq, Gemini, OpenRouter via `base_url`) |
| AI Inference (Generation, Planned Later) | Optional local Ollama mode (deferred) |
| Local Database | SQLite + sqlite-vec + FTS5 |
| Cloud API | Node.js (Express / Fastify) |
| Cloud Database | PostgreSQL |
| Teacher Dashboard | React + Tailwind CSS |
| Spaced Repetition | FSRS 4.5 (Go implementation) |
| PDF Parsing | pdfcpu (Go library) |

---

## Project Documents

| Document | Purpose |
|---|---|
| `REQUIREMENTS.md` | Full functional and non-functional requirements |
| `APP_FLOW.md` | Screen-by-screen user journey and error flows |
| `ARCHITECTURE.md` | Technical design, module map, data flows |
| `DATA_API.md` | API contract between local app and cloud — **binding for both devs** |
| `PLAN_SCOPE.md` | Phase plan, team boundaries, in/out of scope decisions |
| `AI_TUTOR_CLOUD_HANDOFF.md` | Step-by-step cloud implementation checklist for Track B owner |
| `README.md` | This file |
| `SCHEMA.md` | SQLite (local) and PostgreSQL (cloud) table schemas |

### Cloud Handoff Read Order

1. `AI_TUTOR_CLOUD_HANDOFF.md`
2. `DATA_API.md`
3. `SCHEMA.md`
4. `APP_FLOW.md`
5. `sprint.md`

---

## Getting Started

### System A — Local App (Vishnu)

```bash
# Prerequisites
# Install Go 1.22+, Wails CLI
# Configure generation provider using OpenAI-compatible env vars (examples)
# GROQ_API_KEY=...
# GROQ_BASE_URL=https://api.groq.com/openai/v1
# Optional later: local Ollama mode will be re-enabled in a future sprint

# Embedding model is local ONNX file committed in repo
# onnx/model_int8.onnx

# Clone and run
git clone https://github.com/your-org/ai-tutor-local
cd ai-tutor-local
wails dev -tags "sqlite_fts5"

# Production build
wails build -tags "sqlite_fts5"
```

### System B — Cloud (Friend)

```bash
# Prerequisites: Node.js 20+, PostgreSQL 15+

git clone https://github.com/your-org/ai-tutor-cloud
cd ai-tutor-cloud
npm install
cp .env.example .env   # fill in DB credentials, JWT secret
npm run db:migrate
npm run dev
```

---

## Privacy Guarantee

| Data | Stays Local? | Sent to Cloud? |
|---|---|---|
| Uploaded PDFs & documents | ✅ Always | ❌ Never |
| Embedding vectors | ✅ Always | ❌ Never |
| Generated flashcard content | ✅ Always | ❌ Never |
| Personal notes & annotations | ✅ Always | ❌ Never |
| Study session duration | ✅ Yes | ✅ Yes (anonymized) |
| Quiz scores & accuracy | ✅ Yes | ✅ Yes (aggregate only) |
| Study streak | ✅ Yes | ✅ Yes |
| Student name & USN | ✅ Yes | ✅ Yes (class management only) |

---

## Key Design Decisions

**No Knowledge Graph in Phase 1/2** — Local LLMs are not reliable enough for consistent entity extraction on dense study material. Hybrid search (vector + BM25) + HyDE delivers strong retrieval without the complexity or latency.

**Go over Python for local app** — Go compiles to a single binary. Python packaging (PyInstaller) adds gigabytes and startup latency. ONNX Runtime handles local embeddings, and generation currently uses a single OpenAI-compatible API client while optional Ollama mode remains planned.

**API key handling** — Raw API keys are not persisted in `student_config`. Use process environment variables or OS keychain/vault references only.

**SQLite over a dedicated vector database** — `sqlite-vec` brings vector search into the same file as all other data. Zero external dependency, works offline, perfectly portable.

**Periodic + event-based sync** — Sending data every 15 min plus on quiz completion keeps the app battery and bandwidth friendly while keeping teacher data reasonably fresh.

---

## Authors

- **Vishnu** — Local App (System A)  
- **[Friend's Name]** — Cloud Dashboard (System B)

*Final Year Project — VTU, 2025*
