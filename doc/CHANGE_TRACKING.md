# Change Tracking Guide

Use this file as the single source of truth for what changed and why.

## 1. Quick Commands

Run from project root:

```powershell
git status
git diff --name-only
git diff
```

For staged changes:

```powershell
git diff --cached --name-only
git diff --cached
```

For history with changed files:

```powershell
git log --name-status --oneline -n 20
```

## 2. Recommended Daily Workflow

1. Start day:
- `git status`
- create branch (`git checkout -b feature/<topic>`)

2. After each small milestone:
- `git diff --name-only`
- update this file section "Worklog"
- commit with clear message

3. Before handoff:
- `git log --name-status --oneline -n 10`
- share the latest commit hashes and docs list with teammate

## 3. Worklog Template

Copy this block for each work session:

```markdown
### YYYY-MM-DD HH:MM
- Goal:
- Files changed:
  - path/to/file1
  - path/to/file2
- Behavior change:
- Tests run:
- Notes for teammate:
```

## 4. Current Handoff Docs for Cloud Teammate

- `doc/DATA_API.md`
- `doc/APP_FLOW.md`
- `doc/AI_TUTOR_CLOUD_HANDOFF.md`
- `doc/PLAN_SCOPE.md`
- `doc/sprint.md`
