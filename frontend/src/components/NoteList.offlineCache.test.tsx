import "fake-indexeddb/auto";
import { describe, it, expect, vi, afterEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { saveNoteLocal } from "@/lib/idb";
import NoteList from "./NoteList";

vi.mock("react-router", () => import("@/__mocks__/react-router"));

vi.mock("@/context/AuthContext", () => ({
  useAuth: () => ({
    user: { id: "user-b", base_folder_id: "root-b" },
    loading: false,
    refreshUser: vi.fn(),
  }),
}));

vi.mock("@/hooks/useOffline", () => ({
  useOffline: () => true,
}));

afterEach(() => {
  cleanup();
});

describe("NoteList offline cache", () => {
  it("lists only the current subject's notes", async () => {
    await saveNoteLocal({
      id: "note-a",
      subject: "user-a",
      name: "Alice secret",
      content: "alice body",
      modifiedTime: "2026-01-01T00:00:00.000Z",
    });
    await saveNoteLocal({
      id: "note-b",
      subject: "user-b",
      name: "Bob note",
      content: "bob body",
      modifiedTime: "2026-01-02T00:00:00.000Z",
    });

    render(<NoteList />);

    expect(await screen.findByText("Bob note")).toBeTruthy();
    expect(screen.queryByText("Alice secret")).toBeNull();
  });
});
