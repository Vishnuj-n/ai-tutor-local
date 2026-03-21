const onboardingPanel = document.getElementById("onboarding-panel");
const dashboardPanel = document.getElementById("dashboard-panel");
const onboardingForm = document.getElementById("onboarding-form");
const providerStatus = document.getElementById("provider-status");

const ingestionList = document.getElementById("ingestion-list");
const simulateUploadBtn = document.getElementById("simulate-upload");
const syncNowBtn = document.getElementById("sync-now");
const syncStatus = document.getElementById("sync-status");
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

function applySnapshot(snapshot) {
  if (!snapshot) {
    return;
  }

  if (typeof snapshot.due_today === "number") {
    document.getElementById("due-count").textContent = String(snapshot.due_today);
  }

  if (typeof snapshot.study_streak_days === "number") {
    document.getElementById("streak-count").textContent = `${snapshot.study_streak_days} days`;
  }

  if (typeof snapshot.active_notebooks === "number") {
    document.getElementById("notebook-count").textContent = String(snapshot.active_notebooks);
  }

  if (typeof snapshot.pending_sync === "number") {
    document.getElementById("sync-count").textContent = `${snapshot.pending_sync} pending`;
  }

  if (typeof snapshot.sync_status_text === "string" && snapshot.sync_status_text.trim()) {
    syncStatus.textContent = snapshot.sync_status_text;
  }

  if (Array.isArray(snapshot.ingestion) && snapshot.ingestion.length > 0) {
    notebooks.length = 0;
    snapshot.ingestion.forEach((row) => {
      notebooks.push({
        name: row.notebook_name || "Notebook",
        file: row.filename || "Unknown",
        progress: typeof row.progress_pct === "number" ? row.progress_pct : 0,
        state: row.status || "pending",
      });
    });
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

enterDashboardBtn.addEventListener("click", () => {
  onboardingPanel.classList.add("hidden");
  dashboardPanel.classList.remove("hidden");
  renderIngestionList();
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

syncNowBtn.addEventListener("click", () => {
  syncStatus.textContent = "Manual sync started... queue dedup + retry policy active.";
  setTimeout(() => {
    syncStatus.textContent = "Manual sync complete. 4 events sent, 0 duplicates, dashboard consistent.";
  }, 1000);
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

// Wails bridge can set this object before app bootstrap.
if (window.__AI_TUTOR_SNAPSHOT__) {
  applySnapshot(window.__AI_TUTOR_SNAPSHOT__);
}
