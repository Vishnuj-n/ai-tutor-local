# DATA_API.md
## API Contract & Sync Payload Specification
> Version 2.0 | March 2026

**This document is the binding contract between System A (local Go app) and System B (cloud dashboard). Both developers must implement exactly to this specification.**

---

## 1. Base URL & Auth

```
Base URL:     https://api.ai-tutor-cloud.com/api/v1
Content-Type: application/json

Student endpoints:  X-Student-Token: <student_id UUID>
Teacher endpoints:  Authorization: Bearer <JWT>
```

**Deployment Note:** In production, the local app should read this Base URL from Classroom Sync settings (student-provided dashboard URL), not from hardcoded constants.

### `GET /health` (Cloud Connectivity Probe)

Used by local app settings to verify cloud reachability before join/sync.

**Response 200:**
```json
{ "ok": true, "service": "ai-tutor-cloud", "version": "v1" }
```

---

## 1.1 Teacher Shared Content Pack Endpoints (Phase 2 Required)

These endpoints are part of required Phase 2 implementation.

### `GET /api/v1/classes/{class_id}/content-packs`

Returns teacher-published read-only content packs available to the class.

### `POST /api/v1/content-packs/{pack_id}/import`

Triggers a student import operation that creates a new local notebook snapshot from the selected pack.

Versioning rule: each import creates a local snapshot copy; no live two-way editing with teacher source.

### `GET /api/v1/content-packs/{pack_id}/manifest`

Returns metadata and file manifest for the selected content pack version.

### `GET /api/v1/content-packs/{pack_id}/download`

Returns download payload reference (or stream) required by local app to import as a notebook copy.

---

## 2. Student Endpoints (Called by Local App)

### `POST /classes/join`

Student joins a classroom by entering a teacher-generated code.

**Request:**
```json
{
  "student_id": "uuid-v4-string",
  "name": "Vishnu K",
  "usn": "1VE21CS042",
  "class_code": "ABC123"
}
```

**Response 200:**
```json
{
  "success": true,
  "class_id": "class-uuid",
  "class_name": "UPSC 2026 Batch"
}
```

**Response 404:**
```json
{ "error": "Invalid class code" }
```

---

### `POST /sync` ⭐ Primary Endpoint

Sends batched analytics events from the local app to the cloud. This is the most critical endpoint in the entire API.

**Request (full payload):**
```json
{
  "student_id": "uuid-v4-string",
  "class_id":   "class-uuid",
  "synced_at":  "2025-03-15T10:30:00Z",
  "events": [
    {
      "event_id":             "local-uuid-for-dedup",
      "event_type":           "quiz_completed",
      "notebook_id":          "notebook-uuid",
      "notebook_name":        "Polity",
      "topic_name":           "Fundamental Rights",
      "activity_type":        "quiz",
      "time_spent_seconds":   1200,
      "flashcards_completed": 0,
      "quiz_score":           7,
      "quiz_total":           10,
      "accuracy_pct":         70.0,
      "current_streak":       5,
      "occurred_at":          "2025-03-15T10:15:00Z"
    },
    {
      "event_id":             "another-local-uuid",
      "event_type":           "flashcard_session",
      "notebook_id":          "notebook-uuid",
      "notebook_name":        "Economy",
      "topic_name":           "Monetary Policy",
      "activity_type":        "flashcard",
      "time_spent_seconds":   900,
      "flashcards_completed": 25,
      "quiz_score":           null,
      "quiz_total":           null,
      "accuracy_pct":         84.0,
      "current_streak":       5,
      "occurred_at":          "2025-03-15T09:00:00Z"
    }
  ]
}
```

**Response 200:**
```json
{ "success": true, "events_accepted": 2, "events_rejected": 0 }
```

> **Deduplication:** The cloud uses `event_id` (a UUID generated locally) to deduplicate. Resending the same `event_id` is safe — the server ignores duplicates. This makes offline retry completely safe.

---

### `POST /sync/notebook-deleted`

Informs the cloud that a student deleted a notebook locally. Historical data is retained on the cloud.

**Request:**
```json
{
  "student_id":    "uuid",
  "class_id":      "class-uuid",
  "notebook_id":   "notebook-uuid",
  "notebook_name": "History",
  "deleted_at":    "2025-03-14T08:00:00Z"
}
```

---

**Important:** ALWAYS include `notebook_id` in all payloads. Do NOT rely on `notebook_name` alone for sync operations. This prevents data loss when students rename notebooks locally.

---

## 3. Teacher Endpoints (Called by Dashboard)

### `POST /teacher/classes`

Create a new class.

```json
// Request
{ "teacher_id": "...", "class_name": "UPSC 2026 Batch" }

// Response 200
{ "class_id": "...", "class_code": "ABC123", "class_name": "UPSC 2026 Batch" }
```

---

### `GET /teacher/classes/:class_id/overview`

Aggregated class analytics for the dashboard overview.

```json
{
  "total_students": 35,
  "active_today": 28,
  "class_avg_accuracy_pct": 72.4,
  "weakest_topics": [
    { "topic": "Fiscal Policy", "avg_accuracy": 45.2 },
    { "topic": "Constitutional Amendments", "avg_accuracy": 51.0 }
  ]
}
```

---

### `GET /teacher/students/:student_id`

Individual student performance drilldown.

```json
{
  "student_id": "...",
  "name": "Vishnu K",
  "usn": "1VE21CS042",
  "current_streak": 5,
  "notebooks": [
    { "name": "Polity", "total_sessions": 12, "avg_accuracy": 68.0, "time_spent_hours": 4.5 }
  ],
  "quiz_history": [
    { "topic": "Fundamental Rights", "score": 7, "total": 10, "taken_at": "2025-03-15T10:15:00Z" }
  ]
}
```

---

## 4. Allowed vs. Prohibited Sync Data

| Field | Sync Allowed? | Notes |
|---|---|---|
| `student_id` (UUID) | ✅ YES | Pseudonymous identifier |
| `name`, `USN` | ✅ YES | For teacher cohort management |
| `notebook_name`, `topic_name` | ✅ YES | Subject label only |
| `time_spent_seconds` | ✅ YES | Duration only |
| `quiz_score`, `accuracy_pct` | ✅ YES | Aggregate numeric metrics |
| `current_streak` | ✅ YES | Motivational metric |
| `flashcards_completed` | ✅ YES | Count only |
| Raw document text | ❌ NEVER | Never leaves local machine |
| Embedding vectors | ❌ NEVER | Never leaves local machine |
| Full flashcard Q&A content | ❌ NEVER | Private study material |
| Personal notes / annotations | ❌ NEVER | Private study material |

Additional policy constraints:
- Quiz questions/options/explanations should not be synced in telemetry payloads.
- RAG prompts, retrieved chunks, and model outputs are local-only debug artifacts unless an explicit diagnostics API is introduced.

---

## 5. Event Types

| `event_type` | Trigger | Key Fields |
|---|---|---|
| `quiz_completed` | Student finishes a quiz | `quiz_score`, `quiz_total`, `accuracy_pct`, `topic_name` |
| `flashcard_session` | Student finishes a flashcard review session | `flashcards_completed`, `accuracy_pct`, `time_spent_seconds` |
| `study_session` | General reading / search session ends | `time_spent_seconds`, `activity_type` |
| `notebook_deleted` | Student deletes a notebook locally | Sent via separate endpoint |

Compatibility rule during migration:
- Cloud should accept both `flashcard_session` and `flashcard_session_completed`.
- Normalize to a single canonical value in cloud analytics processing.

---

## 5.1 Guided Task Endpoints (Phase 2 Contract)

These endpoints support teacher-assigned or cloud-recommended daily tasks that the local app executes.

### `GET /students/:student_id/tasks/today`

Returns ordered tasks for the student's current day.

```json
{
  "student_id": "uuid-v4-string",
  "date": "2026-03-10",
  "tasks": [
    {
      "task_id": "task-uuid-1",
      "task_type": "READ",
      "notebook_id": "notebook-uuid",
      "topic_name": "Parliament",
      "position": 1,
      "status": "pending"
    },
    {
      "task_id": "task-uuid-2",
      "task_type": "REVIEW_FLASHCARDS",
      "notebook_id": "notebook-uuid",
      "topic_name": "Parliament",
      "position": 2,
      "status": "pending"
    }
  ]
}
```

### `POST /students/:student_id/tasks/:task_id/progress`

Upserts local task completion/progress snapshots to cloud analytics.

```json
{
  "status": "completed",
  "progress_pct": 100,
  "occurred_at": "2026-03-10T09:45:00Z",
  "metadata": {
    "notebook_id": "notebook-uuid",
    "topic_name": "Parliament"
  }
}
```

Response:
```json
{ "success": true }
```

---

## 5.2 Generation Endpoint (Optional, Non-Telemetry)

If cloud-hosted generation is provided to local clients, keep it separate from telemetry APIs.

### `POST /generation/chat`

```json
{
  "student_id": "uuid-v4-string",
  "mode": "rag_answer",
  "messages": [
    { "role": "user", "content": "Explain parliament in simple terms" }
  ],
  "context": {
    "notebook_id": "notebook-uuid",
    "retrieval_chunks": ["..."],
    "max_tokens": 700
  }
}
```

Response:
```json
{
  "provider": "azure-openai",
  "model": "gpt-4o-mini",
  "output_text": "...",
  "finish_reason": "stop"
}
```

This endpoint is optional for Track B in early integration. Track A must keep local fallbacks so study flow remains usable when generation API is unavailable.

---

### Flashcard Session Time Calculation

For `flashcard_session` events, the `time_spent_seconds` field is calculated by the **Go backend** by summing all `time_taken_ms` from the `review_logs` table for cards reviewed in that burst, or by measuring elapsed time from session start (`[Start Review]` click) to session end. 

**Algorithm in Go:**
```
session_start_time = time.Now() (when user clicks [Start Review])
... user reviews cards, each review_log records time_taken_ms ...
session_end_time = time.Now() (when user finishes or clicks exit)
time_spent_seconds = (session_end_time - session_start_time).Seconds()
```

Alternatively, aggregate from review_logs:
```
SELECT SUM(time_taken_ms) / 1000.0 AS time_spent_seconds
FROM review_logs
WHERE flashcard_id IN (cards reviewed in this session)
  AND reviewed_at BETWEEN session_start AND session_end
```

**DO NOT** leave this field null for flashcard sessions. Always measure and populate it.

---

## 6. Error Codes

| HTTP Code | Meaning | Local App Action |
|---|---|---|
| `200 OK` | Success | Process response normally |
| `400 Bad Request` | Malformed payload | Log; do not retry same payload |
| `401 Unauthorized` | Invalid student_id or token | Prompt user to re-enter class code |
| `404 Not Found` | Invalid class code or endpoint | Show user-facing error |
| `409 Conflict` | Duplicate `event_id` already processed | Treat as success; safe to ignore |
| `429 Too Many Requests` | Rate limit hit | Back off exponentially; respect Retry-After header |
| `5xx Server Error` | Cloud server issue | Keep payload in `sync_queue`; retry next cycle |

---

## 7. Offline Handling Contract

1. Local app generates `event_id` (UUID) at event creation time and stores raw JSON in `sync_queue` table.
2. Sync goroutine attempts delivery every 15 minutes (or on event trigger).
3. On `5xx` or network error: increment `attempts` counter, update `last_attempt`, keep `status = pending`.
4. On `200` or `409`: mark `status = sent`.
5. Events older than 30 days with 10+ failed attempts may be marked `status = failed` and skipped.
6. The cloud never rejects a valid payload solely for being delayed.
