import { vi } from "vitest";

if (!globalThis.window) {
  globalThis.window = {};
}

window.go = {
  main: {
    App: {
      GetDashboardSnapshot: vi.fn(),
      GetSyncSettings: vi.fn(),
      IngestDocument: vi.fn(),
      PickDocumentPath: vi.fn(),
      RunManualSync: vi.fn(),
    },
  },
};