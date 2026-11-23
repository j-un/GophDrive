import "fake-indexeddb/auto";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { apiFetch } from "@/lib/api";
import { saveNoteLocal } from "@/lib/idb";
import { useOffline } from "@/hooks/useOffline";
import NotePage from "./NotePage";

const NOTE_ID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";

const auth = vi.hoisted(() => ({
  user: { id: "user-b", base_folder_id: "root-b" } as {
    id: string;
    base_folder_id: string;
  } | null,
}));

const query = vi.hoisted(() => ({ id: "" }));

vi.mock("react-router", async () => {
  const mod = await import("@/__mocks__/react-router");
  return {
    ...mod,
    useSearchParams: () => [
      new URLSearchParams(query.id ? `id=${query.id}` : ""),
      vi.fn(),
    ],
  };
});

vi.mock("@/context/AuthContext", () => ({
  useAuth: () => ({
    user: auth.user,
    loading: false,
    refreshUser: vi.fn(),
  }),
}));

vi.mock("@/hooks/useOffline", () => ({
  useOffline: vi.fn(() => false),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    apiFetch: vi.fn(),
    getBreadcrumbs: vi.fn().mockResolvedValue([{ id: "", name: "Home" }]),
  };
});

vi.mock("@/components/Editor", () => ({
  Editor: ({ value }: { value: string }) => <div>{value}</div>,
}));

vi.mock("@/components/Preview", () => ({
  Preview: () => null,
}));

vi.mock("@/components/Sidebar", () => ({
  Sidebar: () => null,
}));

function setOnline(online: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    get: () => online,
  });
}

beforeEach(() => {
  auth.user = { id: "user-b", base_folder_id: "root-b" };
  query.id = NOTE_ID;
  setOnline(true);
  vi.mocked(useOffline).mockReturnValue(false);
  vi.mocked(apiFetch).mockReset();
});

afterEach(() => {
  cleanup();
});

describe("NotePage cached notes", () => {
  it("does not render another subject's cached note when the server returns 404", async () => {
    await saveNoteLocal({
      id: NOTE_ID,
      subject: "user-a",
      name: "Alice secret",
      content: "alice body",
      modifiedTime: "2026-01-01T00:00:00.000Z",
    });
    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path.includes("/lock")) return new Response(null, { status: 200 });
      return new Response("missing", { status: 404 });
    });

    render(<NotePage />);

    expect(await screen.findByText("Note not found")).toBeTruthy();
    expect(document.body.textContent).not.toContain("Alice secret");
    expect(document.body.textContent).not.toContain("alice body");
  });

  it("renders the same subject's cached note while offline", async () => {
    await saveNoteLocal({
      id: NOTE_ID,
      subject: "user-b",
      name: "Bob note",
      content: "bob body",
      modifiedTime: "2026-01-02T00:00:00.000Z",
    });
    setOnline(false);
    vi.mocked(useOffline).mockReturnValue(true);

    render(<NotePage />);

    expect(await screen.findByText("Bob note")).toBeTruthy();
    expect(document.body.textContent).toContain("bob body");
  });

  it("renders the same subject's cached note when the note request fails", async () => {
    await saveNoteLocal({
      id: NOTE_ID,
      subject: "user-b",
      name: "Bob note",
      content: "bob body",
      modifiedTime: "2026-01-02T00:00:00.000Z",
    });
    vi.mocked(apiFetch).mockRejectedValue(new TypeError("Failed to fetch"));

    render(<NotePage />);

    expect(await screen.findByText("Bob note")).toBeTruthy();
    expect(document.body.textContent).toContain("bob body");
  });
});
