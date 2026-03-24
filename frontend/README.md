# Frontend Sprint 4 Scaffold

This folder contains the initial Wails-target frontend scaffold aligned to `doc/APP_FLOW.md`.

## Current Screens

- Onboarding with provider setup validation
- Home dashboard cards
- Notebook ingestion status list
- Sync status + manual sync action
- Review panel wired to real due-card + FSRS rating RPC flow
- Upload action wired to backend ingestion (`IngestDocument`) using native file picker with manual path prompt fallback

## Backend Snapshot Bridge

The UI can consume a preloaded global object from the host runtime:

```javascript
window.__AI_TUTOR_SNAPSHOT__ = {
  due_today: 2,
  study_streak_days: 1,
  active_notebooks: 3,
  pending_sync: 1,
  sync_status_text: "1 event pending in sync queue. Retry worker active.",
  ingestion: [
    {
      notebook_name: "Polity",
      filename: "notes.pdf",
      status: "processing",
      progress_pct: 55
    }
  ]
}
```

`frontend/app.js` auto-applies this snapshot if present.

## Snapshot Generation (CLI fallback)

Use from project root:

```powershell
go run -tags "sqlite_fts5" ./cmd -dashboard-snapshot
```

Wails runtime bindings are now used by default in dashboard/review/sync/upload flows, with CLI snapshot remaining useful for quick diagnostics.
