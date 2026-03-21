# Test Strategy and Test Cases

Version: 1.0  
Date: 2026-03-21  
Scope: Track A (Local App) + Track B (Cloud API/Dashboard) + Contract Validation

## 1. Test Scope

This document defines the complete test suite for:
- Local app ingestion, retrieval, generation, FSRS, scheduler, and offline sync
- Cloud APIs, analytics persistence, aggregation, and dashboard queries
- Privacy and security constraints
- Performance and reliability targets
- Contract compatibility between local and cloud systems

## 2. Test Levels

- Unit tests: Pure logic and small modules
- Integration tests: Module + DB/API interactions
- End-to-end tests: Real user flows from upload to analytics
- Contract tests: JSON payload and schema compatibility
- Performance tests: Latency, throughput, memory limits
- Security tests: Privacy guarantees and abuse handling
- Resilience tests: Offline, retries, deduplication, failures

## 3. Test Environment Matrix

- OS: Windows 11, Ubuntu 22.04, macOS
- Local DB: SQLite with FTS5 and sqlite-vec enabled
- Cloud DB: PostgreSQL 14+
- Local LLM: Ollama running with nomic-embed-text
- API providers for generation mode: OpenAI, Gemini, Anthropic (generation only)
- Network modes: Online, offline, unstable high-latency

## 4. Unit Test Cases

### 4.1 Ingestion and Chunking

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| UT-ING-001 | Chunking respects max token window | Chunker configured 400/50 | Chunk long text | Each chunk token count <= 400 |
| UT-ING-002 | Chunk overlap preserved | Chunker configured 400/50 | Chunk sample text | Consecutive chunks overlap by ~50 tokens |
| UT-ING-003 | Empty text handling | Input empty string | Chunk text | Returns empty list, no panic |
| UT-ING-004 | Tagged content format | Notebook and heading provided | Chunk text | Prefix format is [Notebook - Heading] |
| UT-ING-005 | File hash stability | Same file bytes twice | Hash both times | Hash values are identical |

### 4.2 Embedding

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| UT-EMB-001 | Embed request uses nomic model | Mock Ollama client | Embed text | Request model == nomic-embed-text |
| UT-EMB-002 | Embed dimension validation | Mock response vector | Validate vector length | Length must be exactly 768 |
| UT-EMB-003 | Non-200 API response handling | Ollama returns 500 | Embed text | Error returned with status info |
| UT-EMB-004 | Timeout handling | Ollama delayed response | Embed text with timeout | Proper timeout error |
| UT-EMB-005 | Empty batch handling | Empty input slice | Embed | Returns validation error or empty result safely |

### 4.3 Retrieval and Ranking

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| UT-RET-001 | RRF merge deterministic ordering | Two result lists | Merge by RRF | Stable deterministic order |
| UT-RET-002 | Duplicate chunk merge | Same chunk in both lists | Merge results | Chunk appears once with combined score |
| UT-RET-003 | Top-k clipping | >k merged hits | Request top-k | Returns exactly k |
| UT-RET-004 | HyDE prompt generation | User query given | Build HyDE prompt | Prompt includes original query |
| UT-RET-005 | Missing vectors fallback | Vector search unavailable | Run retrieval | FTS-only fallback works |

### 4.4 FSRS and Review Logic

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| UT-FSRS-001 | Again rating lowers retrievability | Existing card state | Apply rating 1 | Due date becomes sooner |
| UT-FSRS-002 | Easy rating extends interval | Existing card state | Apply rating 4 | Due date pushed farther |
| UT-FSRS-003 | New card state transition | State new | Review once | State changes to learning/review as designed |
| UT-FSRS-004 | Lapse increment on failure | Reviewed card | Apply Again after review | Lapses increments by 1 |
| UT-FSRS-005 | Due selection boundary | Due exactly now | Fetch due cards | Card included |

### 4.5 Sync Queue Logic

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| UT-SYNC-001 | Event enqueue payload stored | Valid event | Enqueue | Row inserted with pending status |
| UT-SYNC-002 | Retry counter increment | Existing pending item | Mark attempt | attempts + 1 |
| UT-SYNC-003 | Status sent on success | Pending item | Handle 200 | Status becomes sent |
| UT-SYNC-004 | Status sent on dedup conflict | Pending item | Handle 409 | Status becomes sent |
| UT-SYNC-005 | Keep pending on 5xx | Pending item | Handle 500 | Status remains pending |

## 5. Integration Test Cases

### 5.1 SQLite Schema and Constraints

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| IT-DB-001 | Local schema migration runs cleanly | Fresh DB file | Run schema.sql | All tables created |
| IT-DB-002 | FTS triggers update on chunk insert | chunks and chunks_fts created | Insert chunk | chunks_fts has matching rowid |
| IT-DB-003 | FTS triggers update on chunk update | Existing chunk | Update tagged_content | chunks_fts reflects new content |
| IT-DB-004 | FTS triggers cleanup on delete | Existing chunk | Delete chunk | chunks_fts row removed |
| IT-DB-005 | sqlite-vec rowid mapping | Existing chunk rowid | Insert embedding by chunk_rowid | Insert succeeds |
| IT-DB-006 | sqlite-vec rejects wrong dimensions | Existing chunk rowid | Insert non-768 vector | Insert fails with clear error |
| IT-DB-007 | Cascade delete notebook->documents/chunks | Notebook with docs/chunks | Delete notebook | Related rows deleted |

### 5.2 Ingestion Pipeline

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| IT-ING-001 | Upload valid PDF and persist document | Notebook exists | Register + ingest PDF | Document status goes pending->ready |
| IT-ING-002 | Corrupt PDF handling | Invalid PDF | Ingest | status=error, app remains responsive |
| IT-ING-003 | Dedup by file hash | Same PDF twice | Ingest both | Duplicate prevented or linked safely |
| IT-ING-004 | Chunk persistence | Ingestion success | Query chunks by doc | Non-zero chunks stored |
| IT-ING-005 | Embedding persistence | Chunks exist | Generate embeddings | embeddings row count matches chunks |

### 5.3 Retrieval and Q&A

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| IT-RAG-001 | Hybrid search returns results | Indexed notebook exists | Ask question | Non-empty merged top-k |
| IT-RAG-002 | Source attribution in answer | Ask question | Generate answer | Response includes source references |
| IT-RAG-003 | Heading-aware context | Chunk has heading | Build context | Includes [Notebook - Heading] |
| IT-RAG-004 | HyDE path works | HyDE enabled | Ask dense conceptual query | Relevance improves vs baseline |
| IT-RAG-005 | No-hits graceful response | Query out-of-domain | Ask question | Safe fallback message |

### 5.4 Sync Contract and API

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| IT-API-001 | Join class success | Valid class code | POST /classes/join | 200 with class_id |
| IT-API-002 | Join class invalid code | Invalid code | POST /classes/join | 404 with error |
| IT-API-003 | Sync payload includes notebook_id | Prepared events | POST /sync | Server accepts and stores |
| IT-API-004 | notebook-deleted requires notebook_id | Deletion event | POST /sync/notebook-deleted | Accepted with UUID mapping |
| IT-API-005 | Duplicate event_id deduplication | Same event twice | POST /sync twice | Second accepted as duplicate-safe |
| IT-API-006 | flashcard_session duration non-null | Session ended | Build payload | time_spent_seconds populated |
| IT-API-007 | Forbidden fields blocked from sync | Include raw text intentionally | POST /sync | Local validator blocks send |

### 5.5 Cloud Aggregation

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| IT-CLD-001 | analytics append-only behavior | Existing events | Attempt update/delete path | Operation disallowed by service |
| IT-CLD-002 | aggregated_stats uniqueness key | Same notebook_id sync twice | Upsert stats | Single row updated |
| IT-CLD-003 | Rename notebook preserves identity | notebook_name changed, same notebook_id | Sync event | Stats update same row |
| IT-CLD-004 | Different notebooks same name | Two notebook_id with same name | Sync both | Two separate rows exist |
| IT-CLD-005 | Last seen updated on sync | Student syncs | Process payload | students.last_seen_at updated |

## 6. End-to-End Test Cases

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| E2E-001 | First launch onboarding | Fresh install | Launch app, set profile and mode | Config saved and dashboard opens |
| E2E-002 | Full pipeline PDF to flashcards | Notebook exists | Upload PDF and wait | Chunks, embeddings, flashcards generated |
| E2E-003 | Review session updates FSRS | Due cards exist | Start review and rate cards | Scheduling fields updated |
| E2E-004 | Quiz completion telemetry | Quiz generated | Complete quiz | Event queued immediately |
| E2E-005 | Ask question grounded answer | Indexed notebook exists | Ask question | Answer with sources |
| E2E-006 | Timetable generation | Exam inputs provided | Generate timetable | Valid day-by-day plan shown |
| E2E-007 | Classroom join and sync | Cloud available | Join class, trigger sync | Data visible in dashboard |
| E2E-008 | Offline then reconnect sync | Start offline | Generate events, reconnect | Queued events delivered |

## 7. Performance Test Cases

| ID | Test Case | Metric Target | Steps | Expected Result |
|---|---|---|---|---|
| PERF-001 | Vector retrieval latency | <= 500 ms | Query indexed notebook | P95 <= 500 ms |
| PERF-002 | Hybrid retrieval latency | <= 700 ms | Vector + FTS + merge | P95 <= 700 ms |
| PERF-003 | Ingestion responsiveness | Non-blocking UI | Upload large PDF | UI remains interactive |
| PERF-004 | Sync overhead | No UI freeze | Force periodic sync with queue | UI frame drops minimal |
| PERF-005 | Memory bound on large PDF | Stable on 500-page doc | Ingest max-size PDF | No crash/OOM |
| PERF-006 | Batch sync throughput | 1000 events batch | Run sync | Completes within acceptable timeout |

## 8. Security and Privacy Test Cases

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| SEC-001 | Raw text never leaves local | Instrument outgoing payload | Run sync | No raw chunk/document text in payload |
| SEC-002 | Embeddings never leave local | Instrument outgoing payload | Run sync | No vector fields present |
| SEC-003 | API key encryption at rest | API mode configured | Inspect DB storage | Stored value encrypted/obfuscated |
| SEC-004 | SQL injection safety on search | Malicious query input | Run retrieval | No query break, no data leak |
| SEC-005 | Unauthorized student token | Invalid token | Call student endpoint | 401 handled correctly |
| SEC-006 | Teacher endpoint auth required | No bearer token | Call teacher endpoint | 401/403 returned |
| SEC-007 | Sensitive logs redaction | Verbose logs enabled | Trigger failures | Logs do not print keys/content |

## 9. Reliability and Failure Test Cases

| ID | Test Case | Precondition | Steps | Expected Result |
|---|---|---|---|---|
| REL-001 | Network flap during sync | Intermittent network | Sync large queue | Retries continue safely |
| REL-002 | Duplicate retries safety | Re-send same event_id | Sync multiple times | Cloud dedup prevents double count |
| REL-003 | Crash recovery with pending queue | Force app crash mid-run | Restart app | Pending events preserved |
| REL-004 | Ollama unavailable in local mode | Stop Ollama | Start generation | User gets actionable error |
| REL-005 | API generation invalid key | Bad API key | Generate flashcards | Inline error, no crash |
| REL-006 | sqlite-vec extension unavailable | Disable extension | Start app | Startup health check fails gracefully |
| REL-007 | DB locked contention | Parallel writes | Run ingestion + sync | Backoff/retry without corruption |

## 10. Contract Test Cases (Frozen Spec Validation)

| ID | Contract Rule | Validation |
|---|---|---|
| CT-001 | Event payload must include notebook_id | JSON schema validation fails without notebook_id |
| CT-002 | flashcard_session must include time_spent_seconds | Validation rejects null duration |
| CT-003 | embedding vectors are 768-dim only | Local validator rejects non-768 insert |
| CT-004 | notebook identity uses UUID not name | Aggregation key uses student_id + notebook_id |
| CT-005 | Dedup key is event_id | Duplicate event_id treated idempotently |
| CT-006 | Allowed/prohibited data fields | Static allowlist blocks forbidden keys |

## 11. Acceptance Test Cases (Milestone)

### Phase 1 Acceptance

| ID | Requirement | Pass Criteria |
|---|---|---|
| AC-P1-001 | Upload PDF and auto-generate flashcards | Complete in <= 5 minutes on reference machine |
| AC-P1-002 | FSRS due cards available daily | Due list matches algorithm state transitions |
| AC-P1-003 | Grounded Q&A with source links | Answer includes valid references |
| AC-P1-004 | Offline-only operation | Core features usable with no internet |

### Phase 2 Acceptance

| ID | Requirement | Pass Criteria |
|---|---|---|
| AC-P2-001 | Join class and sync analytics | Teacher sees student metrics within 30 min |
| AC-P2-002 | Topic-wise class analytics | Dashboard shows correct aggregates |
| AC-P2-003 | Offline queue replay | All queued events eventually delivered |

## 12. Test Data Sets

- Small PDF: 5 pages, single chapter
- Medium PDF: 100 pages, multiple headings
- Large PDF: 500 pages, mixed formatting
- Corrupt PDF: invalid bytes
- Notebook rename scenarios: same UUID, changed name
- Duplicate notebook names: different UUIDs, same label
- Sync replay data: duplicated event_id, delayed events, out-of-order events

## 13. Exit Criteria

A release candidate is test-complete when:
- 100% critical and high test cases pass
- 0 open critical defects and 0 data corruption defects
- Performance targets met on reference machine
- Privacy/security tests pass with no leakage
- Contract tests pass against frozen API/schema

## 14. Defect Severity Rules

- Critical: crash, data loss, privacy leak, schema corruption
- High: broken major flow (upload/review/sync/query)
- Medium: incorrect metric, degraded ranking, recoverable error
- Low: UI copy/layout issue, minor log formatting

## 15. Suggested Automation Split

- Unit: Go testing package + table-driven tests
- Integration: Go tests with ephemeral SQLite and mock API
- Contract: JSON schema validation in CI
- Performance: scripted benchmark runs with baseline comparison
- E2E: scripted desktop flow harness + API mock server
