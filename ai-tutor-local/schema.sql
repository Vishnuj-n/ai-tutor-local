-- SQLite Schema for ai-tutor-local
-- Source of truth for all local database tables
-- Version: 1.0 (March 2025)

-- ============================================================================
-- NOTEBOOKS
-- ============================================================================
CREATE TABLE IF NOT EXISTS notebooks (
  id          TEXT PRIMARY KEY,   -- UUID
  name        TEXT NOT NULL,      -- e.g., 'Polity'
  description TEXT,
  created_at  TEXT NOT NULL,      -- ISO 8601
  updated_at  TEXT NOT NULL
);

-- ============================================================================
-- DOCUMENTS
-- ============================================================================
CREATE TABLE IF NOT EXISTS documents (
  id          TEXT PRIMARY KEY,   -- UUID
  notebook_id TEXT NOT NULL REFERENCES notebooks(id) ON DELETE CASCADE,
  filename    TEXT NOT NULL,
  file_path   TEXT NOT NULL,      -- Absolute local path
  file_hash   TEXT NOT NULL,      -- SHA256 for deduplication
  page_count  INTEGER,
  status      TEXT DEFAULT 'pending', -- pending | processing | ready | error
  error_msg   TEXT,
  created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_notebook ON documents(notebook_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);

-- ============================================================================
-- CHUNKS
-- ============================================================================
CREATE TABLE IF NOT EXISTS chunks (
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

CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_chunks_notebook ON chunks(notebook_id);

-- ============================================================================
-- FULL-TEXT SEARCH (FTS5)
-- ============================================================================
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
  content,
  content='chunks',
  content_rowid='rowid'
);

-- Trigger to keep FTS5 index in sync when chunks are inserted
CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
  INSERT INTO chunks_fts(rowid, content) VALUES (new.rowid, new.tagged_content);
END;

-- Trigger to keep FTS5 index in sync when chunks are updated
CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
  INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES('delete', old.rowid, old.tagged_content);
  INSERT INTO chunks_fts(rowid, content) VALUES (new.rowid, new.tagged_content);
END;

-- Trigger to keep FTS5 index in sync when chunks are deleted
CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
  INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES('delete', old.rowid, old.tagged_content);
END;

-- ============================================================================
-- VECTOR EMBEDDINGS (sqlite-vec)
-- ============================================================================
-- Note: Requires sqlite-vec extension loaded before table creation
CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0(
  chunk_rowid INTEGER PRIMARY KEY, -- Maps to chunks.rowid (implicit rowid)
  embedding   float[768]           -- nomic-embed-text output dimension
);

-- ============================================================================
-- FLASHCARDS
-- ============================================================================
CREATE TABLE IF NOT EXISTS flashcards (
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

CREATE INDEX IF NOT EXISTS idx_flashcards_notebook ON flashcards(notebook_id);
CREATE INDEX IF NOT EXISTS idx_flashcards_due_date ON flashcards(due_date);
CREATE INDEX IF NOT EXISTS idx_flashcards_state ON flashcards(state);

-- ============================================================================
-- REVIEW LOGS
-- ============================================================================
CREATE TABLE IF NOT EXISTS review_logs (
  id            TEXT PRIMARY KEY,
  flashcard_id  TEXT NOT NULL REFERENCES flashcards(id),
  rating        INTEGER NOT NULL,     -- 1=Again  2=Hard  3=Good  4=Easy
  reviewed_at   TEXT NOT NULL,
  time_taken_ms INTEGER              -- milliseconds taken to answer
);

CREATE INDEX IF NOT EXISTS idx_review_logs_flashcard ON review_logs(flashcard_id);
CREATE INDEX IF NOT EXISTS idx_review_logs_reviewed_at ON review_logs(reviewed_at DESC);

-- ============================================================================
-- QUIZ SESSIONS
-- ============================================================================
CREATE TABLE IF NOT EXISTS quiz_sessions (
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

CREATE INDEX IF NOT EXISTS idx_quiz_sessions_notebook ON quiz_sessions(notebook_id);
CREATE INDEX IF NOT EXISTS idx_quiz_sessions_synced ON quiz_sessions(synced);

-- ============================================================================
-- STUDY SESSIONS
-- ============================================================================
CREATE TABLE IF NOT EXISTS study_sessions (
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

CREATE INDEX IF NOT EXISTS idx_study_sessions_notebook ON study_sessions(notebook_id);
CREATE INDEX IF NOT EXISTS idx_study_sessions_synced ON study_sessions(synced);

-- ============================================================================
-- SYNC QUEUE (offline-first analytics)
-- ============================================================================
CREATE TABLE IF NOT EXISTS sync_queue (
  id           TEXT PRIMARY KEY,   -- Matches event_id in API payload (UUID)
  payload      TEXT NOT NULL,      -- Full JSON string of the event
  created_at   TEXT NOT NULL,
  attempts     INTEGER DEFAULT 0,
  last_attempt TEXT,
  status       TEXT DEFAULT 'pending'  -- pending | sent | failed
);

CREATE INDEX IF NOT EXISTS idx_sync_queue_status ON sync_queue(status);
CREATE INDEX IF NOT EXISTS idx_sync_queue_created ON sync_queue(created_at DESC);

-- ============================================================================
-- STUDENT CONFIGURATION
-- ============================================================================
CREATE TABLE IF NOT EXISTS student_config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Insert default initial config values
INSERT OR IGNORE INTO student_config (key, value) VALUES ('embedding_mode', 'ollama');
INSERT OR IGNORE INTO student_config (key, value) VALUES ('local_ollama_url', 'http://localhost:11434');
