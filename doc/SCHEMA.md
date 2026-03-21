# SCHEMA.md
## Database Schema — SQLite (Local) & PostgreSQL (Cloud)
> Version 1.0 | March 2025

**This document is the shared data contract. Both developers must treat it as authoritative. Schema changes must be discussed and this file updated before implementation.**

---

# PART 1 — SQLite Schema (Local App — Track A)

---

## `notebooks`

```sql
CREATE TABLE notebooks (
  id          TEXT PRIMARY KEY,   -- UUID
  name        TEXT NOT NULL,      -- e.g., 'Polity'
  description TEXT,
  created_at  TEXT NOT NULL,      -- ISO 8601
  updated_at  TEXT NOT NULL
);
```

---

## `documents`

```sql
CREATE TABLE documents (
  id          TEXT PRIMARY KEY,   -- UUID
  notebook_id TEXT NOT NULL REFERENCES notebooks(id) ON DELETE CASCADE,
  filename    TEXT NOT NULL,
  file_path   TEXT NOT NULL,      -- Absolute local path
  file_hash   TEXT NOT NULL,      -- SHA256 for deduplication
  page_count  INTEGER,
  status      TEXT DEFAULT 'pending', -- pending | processing | ready | error
  created_at  TEXT NOT NULL
);
```

---

## `chunks`

```sql
CREATE TABLE chunks (
  id              TEXT PRIMARY KEY,  -- UUID
  document_id     TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  notebook_id     TEXT NOT NULL,
  chapter_name    TEXT,              -- Heading this chunk belongs to
  chunk_index     INTEGER NOT NULL,  -- Order within document
  content         TEXT NOT NULL,     -- Raw chunk text
  tagged_content  TEXT NOT NULL,     -- '[Notebook - Chapter] content...' prefixed version
  token_count     INTEGER,
  created_at      TEXT NOT NULL
);
```

---

## `chunks_fts` (FTS5 Virtual Table)

```sql
CREATE VIRTUAL TABLE chunks_fts USING fts5(
  content,
  content='chunks',
  content_rowid='rowid'
);
```

---

## `embeddings` (sqlite-vec Virtual Table)

```sql
CREATE VIRTUAL TABLE embeddings USING vec0(
  chunk_rowid INTEGER PRIMARY KEY, -- Maps to chunks.rowid (implicit rowid)
  embedding   float[768]           -- ONNX model_int8.onnx output dimension
);
```

**Important:** `sqlite-vec` virtual tables require an `INTEGER PRIMARY KEY` (mapped to the implicit `rowid` of the target table). Do NOT use TEXT UUIDs as the key column. When inserting vectors, reference the `rowid` of the corresponding chunk in the `chunks` table.

---

## `flashcards`

```sql
CREATE TABLE flashcards (
  id              TEXT PRIMARY KEY,   -- UUID
  chunk_id        TEXT NOT NULL REFERENCES chunks(id),
  notebook_id     TEXT NOT NULL,
  question        TEXT NOT NULL,
  answer          TEXT NOT NULL,
  source          TEXT DEFAULT 'ai',  -- 'ai' | 'user' (user-edited cards)

  -- FSRS algorithm fields
  stability       REAL DEFAULT 0.0,
  difficulty      REAL DEFAULT 0.0,
  retrievability  REAL DEFAULT 1.0,
  due_date        TEXT,               -- ISO 8601; NULL = new card not yet reviewed
  reps            INTEGER DEFAULT 0,
  lapses          INTEGER DEFAULT 0,
  state           TEXT DEFAULT 'new', -- new | learning | review | relearning
  last_review     TEXT,

  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
```

---

## `review_logs`

```sql
CREATE TABLE review_logs (
  id            TEXT PRIMARY KEY,
  flashcard_id  TEXT NOT NULL REFERENCES flashcards(id),
  rating        INTEGER NOT NULL,     -- 1=Again  2=Hard  3=Good  4=Easy
  reviewed_at   TEXT NOT NULL,
  time_taken_ms INTEGER              -- milliseconds taken to answer
);
```

---

## `quiz_sessions`

```sql
CREATE TABLE quiz_sessions (
  id            TEXT PRIMARY KEY,
  notebook_id   TEXT NOT NULL,
  topic_name    TEXT,
  score         INTEGER NOT NULL,
  total         INTEGER NOT NULL,
  accuracy_pct  REAL NOT NULL,
  started_at    TEXT NOT NULL,
  completed_at  TEXT NOT NULL,
  synced        INTEGER DEFAULT 0    -- 0 = pending sync, 1 = synced
);
```

---

## `study_sessions`

```sql
CREATE TABLE study_sessions (
  id                   TEXT PRIMARY KEY,
  notebook_id          TEXT,
  activity_type        TEXT NOT NULL,  -- 'flashcard' | 'quiz' | 'reading' | 'search'
  time_spent_seconds   INTEGER NOT NULL,
  flashcards_completed INTEGER DEFAULT 0,
  accuracy_pct         REAL,
  started_at           TEXT NOT NULL,
  ended_at             TEXT NOT NULL,
  synced               INTEGER DEFAULT 0
);
```

---

## `sync_queue`

```sql
CREATE TABLE sync_queue (
  id           TEXT PRIMARY KEY,   -- Matches event_id in API payload (UUID)
  payload      TEXT NOT NULL,      -- Full JSON string of the event
  created_at   TEXT NOT NULL,
  attempts     INTEGER DEFAULT 0,
  last_attempt TEXT,
  status       TEXT DEFAULT 'pending'  -- pending | sent | failed
);
```

---

## `student_config`

```sql
CREATE TABLE student_config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Keys used:
-- 'student_id'   → UUID generated on first launch
-- 'name'         → student display name
-- 'usn'          → university seat number
-- 'class_id'     → cloud class UUID (set after joining)
-- 'class_code'   → last joined class code
-- 'llm_mode'     → 'local' | 'api'
-- 'api_key_ref'  → optional secure keychain reference (never raw API key)
-- 'api_provider' → 'openai' | 'gemini' | 'anthropic'
-- 'embedding_mode'  → 'onnx'
-- 'onnx_model_path' → 'onnx/model_int8.onnx'
```

---

# PART 2 — PostgreSQL Schema (Cloud — Track B)

---

## `teachers`

```sql
CREATE TABLE teachers (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email         TEXT UNIQUE NOT NULL,
  name          TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ DEFAULT NOW()
);
```

---

## `classes`

```sql
CREATE TABLE classes (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  teacher_id  UUID NOT NULL REFERENCES teachers(id),
  name        TEXT NOT NULL,
  class_code  TEXT UNIQUE NOT NULL,   -- 6-char alphanumeric generated on creation
  created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

---

## `students`

```sql
CREATE TABLE students (
  id           UUID PRIMARY KEY,        -- UUID generated by local app, sent on join
  class_id     UUID NOT NULL REFERENCES classes(id),
  name         TEXT NOT NULL,
  usn          TEXT,
  joined_at    TIMESTAMPTZ DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ              -- updated on every successful sync
);
```

---

## `analytics_events` (append-only log)

```sql
CREATE TABLE analytics_events (
  id                   UUID PRIMARY KEY,     -- event_id from local app (deduplication key)
  student_id           UUID NOT NULL REFERENCES students(id),
  class_id             UUID NOT NULL,
  event_type           TEXT NOT NULL,        -- 'quiz_completed' | 'flashcard_session' | 'notebook_deleted'
  notebook_id          UUID,                 -- UUID from local app (stable identifier)
  notebook_name        TEXT,                 -- Denormalized for display; may change if renamed
  topic_name           TEXT,
  activity_type        TEXT,
  time_spent_seconds   INTEGER,
  flashcards_completed INTEGER,
  quiz_score           INTEGER,
  quiz_total           INTEGER,
  accuracy_pct         NUMERIC(5,2),
  current_streak       INTEGER,
  occurred_at          TIMESTAMPTZ NOT NULL,  -- when it happened on the student's machine
  received_at          TIMESTAMPTZ DEFAULT NOW()  -- when the cloud received it
);
```

> This table is **append-only**. Never UPDATE or DELETE rows. It is the source of truth for all analytics.

---

## `aggregated_stats` (pre-computed, updated on each sync)

```sql
CREATE TABLE aggregated_stats (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id            UUID NOT NULL REFERENCES students(id),
  notebook_id           UUID NOT NULL,  -- UUID from local app (not name)
  notebook_name         TEXT,           -- Denormalized for dashboard display; DO NOT use for uniqueness
  total_sessions        INTEGER DEFAULT 0,
  total_time_seconds    INTEGER DEFAULT 0,
  total_flashcards_done INTEGER DEFAULT 0,
  avg_accuracy_pct      NUMERIC(5,2),
  last_active_at        TIMESTAMPTZ,
  current_streak        INTEGER DEFAULT 0,
  updated_at            TIMESTAMPTZ DEFAULT NOW(),

  UNIQUE(student_id, notebook_id)  -- Prevents duplicates if notebook is renamed
);
```

**Important:** Always use `notebook_id` (UUID) as the uniqueness constraint, NOT `notebook_name`. This prevents data corruption when a student renames a notebook locally.

---

## Indexes

```sql
-- analytics_events
CREATE INDEX idx_events_student   ON analytics_events(student_id);
CREATE INDEX idx_events_class     ON analytics_events(class_id);
CREATE INDEX idx_events_occurred  ON analytics_events(occurred_at DESC);
CREATE INDEX idx_events_topic     ON analytics_events(topic_name);
CREATE INDEX idx_events_type      ON analytics_events(event_type);

-- aggregated_stats
CREATE INDEX idx_stats_student    ON aggregated_stats(student_id);
CREATE INDEX idx_stats_accuracy   ON aggregated_stats(avg_accuracy_pct);

-- students
CREATE INDEX idx_students_class   ON students(class_id);
CREATE INDEX idx_students_seen    ON students(last_seen_at DESC);
```

---

## Schema Change Policy

1. All changes are applied via numbered migration files (e.g., `001_initial.sql`, `002_add_streak_col.sql`). Never edit the live database directly.
2. **Track A (SQLite):** Use embedded Go migrations. The migration runner checks version on startup.
3. **Track B (PostgreSQL):** Use a migration tool (e.g., `db-migrate` for Node.js).
4. Breaking changes to `analytics_events` (renaming/dropping columns) require a version bump in the API and a coordinated deploy from both devs.
5. New optional fields added to the sync payload (`DATA_API.md`) must have `DEFAULT NULL` in PostgreSQL so existing older clients don't break.
6. Any schema change must be reflected in this file **before** it is implemented.
