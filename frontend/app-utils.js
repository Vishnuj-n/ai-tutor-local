export function validateProvider(provider, baseUrl, apiKey) {
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

export function deriveNotebookName(filePath) {
  const fallback = "General Notebook";
  const base = (filePath.split(/[\\/]/).pop() || "").trim();
  if (!base) {
    return fallback;
  }

  const withoutExt = base.replace(/\.[^.]+$/, "").trim();
  return withoutExt || fallback;
}