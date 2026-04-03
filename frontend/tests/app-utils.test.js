import { describe, expect, it } from "vitest";

import { deriveNotebookName, validateProvider } from "../app-utils.js";

describe("validateProvider", () => {
  it("rejects missing input", () => {
    const result = validateProvider("", "", "");
    expect(result.ok).toBe(false);
  });

  it("accepts valid provider details", () => {
    const result = validateProvider("openai", "https://api.example.com", "123456789012");
    expect(result.ok).toBe(true);
    expect(result.message).toContain("OPENAI");
  });
});

describe("deriveNotebookName", () => {
  it("derives notebook name from filename", () => {
    expect(deriveNotebookName("C:/notes/Constitution_Notes.pdf")).toBe("Constitution_Notes");
  });

  it("falls back when input is empty", () => {
    expect(deriveNotebookName("   ")).toBe("General Notebook");
  });
});