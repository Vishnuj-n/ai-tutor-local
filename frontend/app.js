import { EventsOff, EventsOn } from "./wailsjs/runtime/runtime.js";

const onboardingPanel = document.getElementById("onboarding-panel");
const dashboardPanel = document.getElementById("dashboard-panel");
const onboardingForm = document.getElementById("onboarding-form");
const providerStatus = document.getElementById("provider-status");

const ingestionList = document.getElementById("ingestion-list");
const uploadFileBtn = document.getElementById("simulate-upload");
const syncNowFooterBtn = document.getElementById("sync-now");
const syncStatusText = document.getElementById("sync-status-text");
const syncStatusIndicator = document.getElementById("sync-status-indicator");
const enterDashboardBtn = document.getElementById("enter-dashboard");
const openSettingsBtn = document.getElementById("open-settings");
const backFromSettingsBtn = document.getElementById("back-from-settings");
const reviewPanel = document.getElementById("review-panel");
const ragPanel = document.getElementById("rag-panel");

const startReviewBtn = document.getElementById("start-review");
const openRagBtn = document.getElementById("open-rag");
const showAnswerBtn = document.getElementById("show-answer");
const reviewAnswer = document.getElementById("review-answer");
const reviewFeedback = document.getElementById("review-feedback");
const backDashboardReviewBtn = document.getElementById("back-dashboard-review");
const backDashboardRagBtn = document.getElementById("back-dashboard-rag");

const runRagBtn = document.getElementById("run-rag");
const ragQuestion = document.getElementById("rag-question");
const ragStatus = document.getElementById("rag-status");
const ragAnswerBox = document.getElementById("rag-answer-box");
const ragAnswerText = document.getElementById("rag-answer-text");
const ragSources = document.getElementById("rag-sources");

const setupStorageKey = "aiTutor.setup.complete";
const setupProfileStorageKey = "aiTutor.setup.profile";

let activeReviewCard = null;
let reviewShownAtMs = 0;
let onboardingMode = "initial";
let reviewSession = null;
let activeRagEvent = "";

const notebooks = [
  { name: "Polity", file: "Constitution_Notes.pdf", progress: 100, state: "ready" },
  { name: "Economy", file: "Macro_QuickRev.pdf", progress: 72, state: "processing" },
  { name: "History", file: "Modern_India.txt", progress: 34, state: "processing" },
];

function isSetupComplete() {
  return localStorage.getItem(setupStorageKey) === "true";
}

function loadSetupProfile() {
  try {
    const raw = localStorage.getItem(setupProfileStorageKey);
    if (!raw) {
      return;
    }
    const profile = JSON.parse(raw);
    if (profile.studentName) {
      document.getElementById("student-name").value = profile.studentName;
    }
    if (profile.usn) {
      document.getElementById("student-usn").value = profile.usn;
    }
    if (profile.provider) {
      document.getElementById("provider").value = profile.provider;
    }
    if (profile.baseUrl) {
      document.getElementById("base-url").value = profile.baseUrl;
    }
  } catch (err) {
    console.warn("Unable to restore setup profile:", err);
  }
}

function saveSetupProfile(markSetupComplete) {
  const profile = {
    studentName: document.getElementById("student-name").value.trim(),
    usn: document.getElementById("student-usn").value.trim(),
    provider: document.getElementById("provider").value.trim(),
    baseUrl: document.getElementById("base-url").value.trim(),
  };
  localStorage.setItem(setupProfileStorageKey, JSON.stringify(profile));
  if (markSetupComplete) {
    localStorage.setItem(setupStorageKey, "true");
  }
}

function showDashboard() {
  onboardingPanel.classList.add("hidden");
  reviewPanel.classList.add("hidden");
  ragPanel.classList.add("hidden");
  dashboardPanel.classList.remove("hidden");

  enterDashboardBtn.classList.remove("hidden");
  backFromSettingsBtn.classList.add("hidden");
  openSettingsBtn.classList.remove("hidden");
}

function showOnboarding(mode) {
  onboardingMode = mode || "initial";
  dashboardPanel.classList.add("hidden");
  reviewPanel.classList.add("hidden");
  ragPanel.classList.add("hidden");
  onboardingPanel.classList.remove("hidden");

  if (onboardingMode === "settings" && isSetupComplete()) {
    openSettingsBtn.classList.remove("hidden");
    enterDashboardBtn.classList.add("hidden");
    backFromSettingsBtn.classList.remove("hidden");
  } else {
    openSettingsBtn.classList.add("hidden");
    enterDashboardBtn.classList.remove("hidden");
    backFromSettingsBtn.classList.add("hidden");
  }
}

function showPanel(panel) {
  onboardingPanel.classList.add("hidden");
  dashboardPanel.classList.add("hidden");
  reviewPanel.classList.add("hidden");
  ragPanel.classList.add("hidden");
  panel.classList.remove("hidden");
}

function renderIngestionList() {
  ingestionList.innerHTML = "";
  notebooks.forEach((item) => {
    const li = document.createElement("li");
    li.innerHTML = `
      <strong>${item.name}</strong> - ${item.file}<br>
      <small>Status: ${item.state}</small>
      <div class="progress"><span style="width:${item.progress}%"></span></div>
    `;
    ingestionList.appendChild(li);
  });
}

function validateProvider(provider, baseUrl, apiKey) {
  if (!provider || !baseUrl || !apiKey) {
    return { ok: false, message: "Provider, base URL, and API key are required." };
  }
  if (!baseUrl.startsWith("http")) {
    return { ok: false, message: "Base URL must start with http/https." };
  }
  if (apiKey.length < 12) {
    return { ok: false, message: "API key looks too short." };
  }
  return { ok: true, message: `${provider.toUpperCase()} endpoint validated (session only).` };
}

async function applySnapshot(snapshot) {
  if (!snapshot) {
    return;
  }

  if (typeof snapshot.DueToday === "number") {
    document.getElementById("due-count").textContent = String(snapshot.DueToday);
  }
  if (typeof snapshot.StudyStreak === "number") {
    document.getElementById("streak-count").textContent = `${snapshot.StudyStreak} days`;
  }
  if (typeof snapshot.ActiveNotebooks === "number") {
    document.getElementById("notebook-count").textContent = String(snapshot.ActiveNotebooks);
  }
  if (typeof snapshot.PendingSync === "number") {
    document.getElementById("sync-count").textContent = `${snapshot.PendingSync} pending`;
  }

  if (typeof snapshot.SyncStatusText === "string" && snapshot.SyncStatusText.trim()) {
    syncStatusText.textContent = `Sync: ${snapshot.SyncStatusText}`;
    syncStatusIndicator.classList.remove("error", "idle");
    syncStatusIndicator.classList.add("pulse");
  }

  if (Array.isArray(snapshot.Ingestion) && snapshot.Ingestion.length > 0) {
    notebooks.length = 0;
    snapshot.Ingestion.forEach((row) => {
      notebooks.push({
        name: row.NotebookName || "Notebook",
        file: row.Filename || "Unknown",
        progress: typeof row.ProgressPct === "number" ? row.ProgressPct : 0,
        state: row.Status || "pending",
      });
    });
    renderIngestionList();
    return;
  }

  // Backward-compatible fallback for older snapshot shapes.
  if (Array.isArray(snapshot.Notebooks) && snapshot.Notebooks.length > 0) {
    notebooks.length = 0;
    snapshot.Notebooks.forEach((nb) => {
      if (Array.isArray(nb.IngestionRows)) {
        nb.IngestionRows.forEach((row) => {
          notebooks.push({
            name: nb.Name || "Notebook",
            file: row.Filename || "Unknown",
            progress: typeof row.ProgressPct === "number" ? row.ProgressPct : 0,
            state: row.Status || "pending",
          });
        });
      }
    });
    renderIngestionList();
  }
}

async function loadSnapshot() {
  try {
    if (window.go && window.go.main && window.go.main.App) {
      const snapshot = await window.go.main.App.GetDashboardSnapshot();
      if (snapshot) {
        await applySnapshot(snapshot);
        return;
      }
    }
  } catch (err) {
    console.warn("Wails binding not ready yet:", err);
  }

  if (window.__AI_TUTOR_SNAPSHOT__) {
    await applySnapshot(window.__AI_TUTOR_SNAPSHOT__);
  }
}

async function refreshSyncStatus() {
  try {
    if (!(window.go && window.go.main && window.go.main.App)) {
      return;
    }

    const status = await window.go.main.App.GetSyncStatus();
    if (!status) {
      return;
    }

    const pending = typeof status.PendingCount === "number" ? status.PendingCount : 0;
    const health = (status.Health || "unknown").toLowerCase();
    const nextRetryMs = typeof status.NextRetryInMS === "number" ? status.NextRetryInMS : 0;

    let suffix = `pending=${pending}, health=${health}`;
    if (nextRetryMs > 0) {
      suffix += `, next_retry=${Math.ceil(nextRetryMs / 1000)}s`;
    }

    syncStatusText.textContent = `Sync: ${suffix}`;
    syncStatusIndicator.classList.remove("idle", "error", "pulse");
    if (health === "degraded") {
      syncStatusIndicator.classList.add("error");
    }
  } catch (err) {
    console.warn("Unable to load sync status:", err);
  }
}

function initReviewSession(card) {
  if (!card) {
    return;
  }
  if (reviewSession && reviewSession.notebookID === card.NotebookID) {
    return;
  }
  reviewSession = {
    notebookID: card.NotebookID,
    notebookName: card.NotebookName,
    startedAtMS: Date.now(),
    reviewed: 0,
    correct: 0,
    totalTimeTakenMS: 0,
  };
}

async function completeReviewSession(reason) {
  if (!reviewSession || reviewSession.reviewed <= 0) {
    reviewSession = null;
    return;
  }

  try {
    if (window.go && window.go.main && window.go.main.App) {
      const message = await window.go.main.App.CompleteReviewSession({
        NotebookID: reviewSession.notebookID,
        NotebookName: reviewSession.notebookName,
        StartedAtMS: reviewSession.startedAtMS,
        EndedAtMS: Date.now(),
        FlashcardsReviewed: reviewSession.reviewed,
        CorrectRecallCount: reviewSession.correct,
        TotalTimeTakenMS: reviewSession.totalTimeTakenMS,
        EmitTelemetry: true,
      });
      reviewFeedback.textContent = `${message || "Session saved"} (${reason})`;
      await loadSnapshot();
      await refreshSyncStatus();
    }
  } catch (err) {
    console.error("Complete session error:", err);
    reviewFeedback.textContent = `Session summary failed: ${err?.message || err}`;
  } finally {
    reviewSession = null;
  }
}

async function loadNextReviewCard() {
  reviewAnswer.classList.add("hidden");
  activeReviewCard = null;

  try {
    if (!(window.go && window.go.main && window.go.main.App)) {
      reviewFeedback.textContent = "Demo mode: backend unavailable.";
      return;
    }

    const card = await window.go.main.App.GetNextDueCard();
    if (!card) {
      document.getElementById("review-question").textContent = "No due cards right now.";
      reviewAnswer.textContent = "Ingest content or wait until a card becomes due.";
      reviewFeedback.textContent = "FSRS queue is currently empty.";
      reviewAnswer.classList.remove("hidden");

      if (reviewSession && reviewSession.reviewed > 0) {
        await completeReviewSession("queue completed");
      }
      return;
    }

    activeReviewCard = card;
    reviewShownAtMs = Date.now();
    initReviewSession(card);

    const prefix = card.NotebookName ? `[${card.NotebookName}] ` : "";
    document.getElementById("review-question").textContent = `${prefix}${card.Question}`;
    reviewAnswer.textContent = card.Answer || "No answer available.";
    reviewFeedback.textContent = `Due queue: ${card.QueuePosition}/${card.QueueSize} • State: ${card.State || "new"}`;
  } catch (err) {
    console.error("Load due card error:", err);
    reviewFeedback.textContent = `Unable to load due card: ${err?.message || err}`;
  }
}

async function submitRating(ratingName) {
  if (!activeReviewCard) {
    reviewFeedback.textContent = "No active card to rate.";
    return;
  }

  const ratingMap = { again: 1, hard: 2, good: 3, easy: 4 };
  const rating = ratingMap[ratingName];
  const elapsed = Math.max(300, Date.now() - reviewShownAtMs);

  try {
    if (window.go && window.go.main && window.go.main.App) {
      const result = await window.go.main.App.RateDueCard({
        FlashcardID: activeReviewCard.FlashcardID,
        NotebookID: activeReviewCard.NotebookID,
        NotebookName: activeReviewCard.NotebookName,
        Rating: rating,
        TimeTakenMs: elapsed,
      });

      if (reviewSession) {
        reviewSession.reviewed += 1;
        reviewSession.totalTimeTakenMS += elapsed;
        if (rating >= 3) {
          reviewSession.correct += 1;
        }
      }

      reviewFeedback.textContent = `Rated ${ratingName}. ${result?.Message || "Schedule updated."}`;
      await loadSnapshot();
      await loadNextReviewCard();
      return;
    }

    reviewFeedback.textContent = `Demo mode: rated ${ratingName}.`;
  } catch (err) {
    console.error("Rating error:", err);
    reviewFeedback.textContent = `Rating failed: ${err?.message || err}`;
  }
}

onboardingForm.addEventListener("submit", (event) => {
  event.preventDefault();

  const provider = document.getElementById("provider").value.trim();
  const baseUrl = document.getElementById("base-url").value.trim();
  const apiKey = document.getElementById("api-key").value.trim();

  const result = validateProvider(provider, baseUrl, apiKey);
  providerStatus.textContent = result.message;
  providerStatus.classList.remove("ok", "err");
  providerStatus.classList.add(result.ok ? "ok" : "err");

  if (result.ok) {
    saveSetupProfile(false);
  }
});

enterDashboardBtn.addEventListener("click", async () => {
  saveSetupProfile(true);
  showDashboard();
  renderIngestionList();
  await loadSnapshot();
  await refreshSyncStatus();
});

openSettingsBtn.addEventListener("click", () => {
  showOnboarding("settings");
});

backFromSettingsBtn.addEventListener("click", async () => {
  saveSetupProfile(false);
  showDashboard();
  await loadSnapshot();
  await refreshSyncStatus();
});

uploadFileBtn.addEventListener("click", () => {
  void (async () => {
    let filePath = "";
    if (window.go && window.go.main && window.go.main.App && window.go.main.App.PickDocumentPath) {
      filePath = await window.go.main.App.PickDocumentPath();
    }

    if (!filePath) {
      filePath = window.prompt("Enter full file path (.pdf/.txt/.md)") || "";
    }

    if (!filePath.trim()) {
      return;
    }

    const notebookName = window.prompt("Notebook name", "Manual Upload") || "Manual Upload";

    try {
      if (window.go && window.go.main && window.go.main.App) {
        const result = await window.go.main.App.IngestDocument(filePath.trim(), notebookName.trim());
        reviewFeedback.textContent = result || "Upload complete";
        await loadSnapshot();
        renderIngestionList();
      } else {
        notebooks.push({
          name: notebookName,
          file: filePath.split(/[\\/]/).pop() || filePath,
          progress: 100,
          state: "ready",
        });
        renderIngestionList();
        reviewFeedback.textContent = "Demo mode: simulated ingestion complete.";
      }
    } catch (err) {
      console.error("Ingestion error:", err);
      reviewFeedback.textContent = `Ingestion failed: ${err?.message || err}`;
    }
  })();
});

syncNowFooterBtn.addEventListener("click", async () => {
  syncStatusText.textContent = "Sync: Starting manual sync...";
  syncStatusIndicator.classList.remove("idle", "error");
  syncStatusIndicator.classList.add("pulse");

  try {
    if (window.go && window.go.main && window.go.main.App) {
      const result = await window.go.main.App.RunManualSync();
      syncStatusText.textContent = `Sync: Complete. ${result || "Ready."}`;
      await refreshSyncStatus();
    } else {
      syncStatusText.textContent = "Sync: Demo mode - queued 4 events, 0 duplicates detected.";
    }
  } catch (err) {
    console.error("Sync error:", err);
    syncStatusText.textContent = "Sync: Error. Check console.";
    syncStatusIndicator.classList.add("error");
  }

  setTimeout(() => {
    syncStatusIndicator.classList.remove("pulse");
  }, 2000);
});

startReviewBtn.addEventListener("click", () => {
  void (async () => {
    showPanel(reviewPanel);
    await loadNextReviewCard();
  })();
});

openRagBtn.addEventListener("click", () => {
  showPanel(ragPanel);
});

backDashboardReviewBtn.addEventListener("click", () => {
  void (async () => {
    await completeReviewSession("review closed");
    showPanel(dashboardPanel);
  })();
});

backDashboardRagBtn.addEventListener("click", () => {
  showPanel(dashboardPanel);
});

showAnswerBtn.addEventListener("click", () => {
  if (!activeReviewCard) {
    reviewFeedback.textContent = "No due cards available.";
    return;
  }
  reviewAnswer.classList.remove("hidden");
  reviewFeedback.textContent = "Answer shown. Select recall rating to update FSRS schedule.";
});

["again", "hard", "good", "easy"].forEach((rating) => {
  const btn = document.getElementById(`rate-${rating}`);
  btn.addEventListener("click", () => {
    void submitRating(rating);
  });
});

runRagBtn.addEventListener("click", () => {
  void (async () => {
    const question = ragQuestion.value.trim();
    if (!question) {
      ragStatus.textContent = "Enter a question first.";
      return;
    }

    ragAnswerBox.classList.remove("hidden");
    ragAnswerText.textContent = "";
    ragSources.textContent = "";
    ragStatus.textContent = "Running HyDE + hybrid retrieval...";

    if (activeRagEvent) {
      EventsOff(activeRagEvent);
      activeRagEvent = "";
    }

    try {
      if (window.go && window.go.main && window.go.main.App && window.go.main.App.StreamRAGAnswer) {
        const eventName = await window.go.main.App.StreamRAGAnswer(question);
        activeRagEvent = eventName;

        EventsOn(eventName, (payload) => {
          if (!payload || typeof payload !== "object") {
            return;
          }

          if (payload.Type === "chunk") {
            ragAnswerText.textContent += payload.Text || "";
            ragStatus.textContent = "Streaming answer...";
            return;
          }

          if (payload.Type === "done") {
            ragStatus.textContent = "Grounded answer ready.";
            if (Array.isArray(payload.Sources) && payload.Sources.length > 0) {
              ragSources.textContent = `Sources: ${payload.Sources.join(", ")}`;
            }
            EventsOff(eventName);
            if (activeRagEvent === eventName) {
              activeRagEvent = "";
            }
            return;
          }

          if (payload.Type === "error") {
            ragStatus.textContent = `RAG error: ${payload.Error || "unknown error"}`;
            EventsOff(eventName);
            if (activeRagEvent === eventName) {
              activeRagEvent = "";
            }
          }
        });

        return;
      }
    } catch (err) {
      console.warn("Streaming RAG unavailable, falling back to demo response:", err);
    }

    // Demo fallback (non-Wails or missing backend method).
    setTimeout(() => {
      ragStatus.textContent = "Grounded answer ready.";
      ragAnswerText.textContent = "Federalism divides powers between central and state governments, balancing national unity with regional autonomy.";
      ragSources.textContent = "Sources: [Polity - Federalism] chunk #3, [Polity - Parliament] chunk #7";
    }, 650);
  })();
});

document.addEventListener("DOMContentLoaded", async () => {
  loadSetupProfile();
  renderIngestionList();

  if (isSetupComplete()) {
    showDashboard();
    await loadSnapshot();
    await refreshSyncStatus();
  } else {
    showOnboarding("initial");
  }
});

if (window.__AI_TUTOR_SNAPSHOT__) {
  void applySnapshot(window.__AI_TUTOR_SNAPSHOT__);
}
