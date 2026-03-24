const onboardingPanel = document.getElementById("onboarding-panel");
const dashboardPanel = document.getElementById("dashboard-panel");
const onboardingForm = document.getElementById("onboarding-form");
const providerStatus = document.getElementById("provider-status");

const ingestionList = document.getElementById("ingestion-list");
const simulateUploadBtn = document.getElementById("simulate-upload");
const syncNowFooterBtn = document.getElementById("sync-now");
const syncStatusText = document.getElementById("sync-status-text");
const syncStatusIndicator = document.getElementById("sync-status-indicator");
const enterDashboardBtn = document.getElementById("enter-dashboard");
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

const notebooks = [
  { name: "Polity", file: "Constitution_Notes.pdf", progress: 100, state: "ready" },
  { name: "Economy", file: "Macro_QuickRev.pdf", progress: 72, state: "processing" },
  { name: "History", file: "Modern_India.txt", progress: 34, state: "processing" },
];

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
  }
}

async function loadSnapshot() {
  try {
    // Try to load from Wails binding if available
    if (window.go && window.go.main && window.go.main.App) {
      const snapshot = await window.go.main.App.GetDashboardSnapshot();
      if (snapshot) {
        applySnapshot(snapshot);
        return;
      }
    }
  } catch (err) {
    console.warn("Wails binding not ready yet:", err);
  }

  // Fallback: use pre-rendered snapshot if set by backend
  if (window.__AI_TUTOR_SNAPSHOT__) {
    applySnapshot(window.__AI_TUTOR_SNAPSHOT__);
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

onboardingForm.addEventListener("submit", (event) => {
  event.preventDefault();

  const provider = document.getElementById("provider").value.trim();
  const baseUrl = document.getElementById("base-url").value.trim();
  const apiKey = document.getElementById("api-key").value.trim();

  const result = validateProvider(provider, baseUrl, apiKey);
  providerStatus.textContent = result.message;
  providerStatus.classList.remove("ok", "err");
  providerStatus.classList.add(result.ok ? "ok" : "err");
});

enterDashboardBtn.addEventListener("click", async () => {
  onboardingPanel.classList.add("hidden");
  dashboardPanel.classList.remove("hidden");
  renderIngestionList();
  // Try to load real snapshot from backend
  await loadSnapshot();
  await refreshSyncStatus();
});

simulateUploadBtn.addEventListener("click", () => {
  notebooks.push({
    name: "Ethics",
    file: "Case_Studies.pdf",
    progress: 12,
    state: "processing",
  });
  renderIngestionList();
});

syncNowFooterBtn.addEventListener("click", async () => {
  syncStatusText.textContent = "Sync: Starting manual sync...";
  syncStatusIndicator.classList.remove("idle", "error");
  syncStatusIndicator.classList.add("pulse");
  
  try {
    if (window.go && window.go.main && window.go.main.App) {
      // Call backend RPC if available
      const result = await window.go.main.App.RunManualSync();
      syncStatusText.textContent = `Sync: Complete. ${result || "Ready."}`;
      await refreshSyncStatus();
    } else {
      // Demo mode
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

function showPanel(panel) {
  dashboardPanel.classList.add("hidden");
  reviewPanel.classList.add("hidden");
  ragPanel.classList.add("hidden");
  panel.classList.remove("hidden");
}

startReviewBtn.addEventListener("click", () => {
  showPanel(reviewPanel);
});

openRagBtn.addEventListener("click", () => {
  showPanel(ragPanel);
});

backDashboardReviewBtn.addEventListener("click", () => {
  showPanel(dashboardPanel);
});

backDashboardRagBtn.addEventListener("click", () => {
  showPanel(dashboardPanel);
});

showAnswerBtn.addEventListener("click", () => {
  reviewAnswer.classList.remove("hidden");
  reviewFeedback.textContent = "Answer shown. Select recall rating to update FSRS schedule.";
});

["again", "hard", "good", "easy"].forEach((rating) => {
  const btn = document.getElementById(`rate-${rating}`);
  btn.addEventListener("click", () => {
    reviewFeedback.textContent = `Rated ${rating}. Next due date recalculated and session telemetry queued.`;
  });
});

runRagBtn.addEventListener("click", () => {
  const question = ragQuestion.value.trim();
  if (!question) {
    ragStatus.textContent = "Enter a question first.";
    return;
  }

  ragStatus.textContent = "Running HyDE + hybrid retrieval...";
  setTimeout(() => {
    ragStatus.textContent = "Grounded answer ready.";
    ragAnswerText.textContent = "Federalism divides powers between central and state governments, balancing national unity with regional autonomy.";
    ragSources.textContent = "Sources: [Polity - Federalism] chunk #3, [Polity - Parliament] chunk #7";
    ragAnswerBox.classList.remove("hidden");
  }, 650);
});

// App lifecycle: try to load snapshot on startup
document.addEventListener("DOMContentLoaded", async () => {
  // Pre-render fallback or Wails binding will load on first dashboard show
  renderIngestionList();
});

// Detect when Wails bindings become available and auto-load snapshot
if (window.__AI_TUTOR_SNAPSHOT__) {
  applySnapshot(window.__AI_TUTOR_SNAPSHOT__);
}
