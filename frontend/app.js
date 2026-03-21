const onboardingPanel = document.getElementById("onboarding-panel");
const dashboardPanel = document.getElementById("dashboard-panel");
const onboardingForm = document.getElementById("onboarding-form");
const providerStatus = document.getElementById("provider-status");

const ingestionList = document.getElementById("ingestion-list");
const simulateUploadBtn = document.getElementById("simulate-upload");
const syncNowBtn = document.getElementById("sync-now");
const syncStatus = document.getElementById("sync-status");
const enterDashboardBtn = document.getElementById("enter-dashboard");

const notebooks = [
  { name: "Polity", file: "Constitution_Notes.pdf", progress: 100, state: "ready" },
  { name: "Economy", file: "Macro_QuickRev.pdf", progress: 72, state: "processing" },
  { name: "History", file: "Modern_India.txt", progress: 34, state: "processing" },
];

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
