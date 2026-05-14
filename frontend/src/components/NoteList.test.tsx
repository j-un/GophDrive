import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { deleteFile } from "@/lib/api";
import { getAllNotesLocal } from "@/lib/idb";
import { useOffline } from "@/hooks/useOffline";

vi.mock("@/lib/api", () => ({
  listFiles: vi.fn().mockResolvedValue([
    {
      id: "note-1",
      name: "Test Note",
      mimeType: "application/vnd.google-apps.document",
      parents: [],
      modifiedTime: "2026-05-14T00:00:00Z",
      starred: false,
    },
  ]),
  deleteFile: vi.fn().mockResolvedValue(undefined),
  duplicateNote: vi.fn().mockResolvedValue({ id: "note-2" }),
  renameNote: vi.fn().mockResolvedValue(undefined),
  starFile: vi.fn().mockResolvedValue(undefined),
  searchFiles: vi.fn().mockResolvedValue([]),
  createNote: vi.fn(),
}));

vi.mock("@/lib/idb", () => ({
  deleteNoteLocal: vi.fn().mockResolvedValue(undefined),
  getAllNotesLocal: vi.fn().mockResolvedValue([]),
}));

vi.mock("@/hooks/useOffline", () => ({
  useOffline: vi.fn().mockReturnValue(false),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

import NoteList from "./NoteList";

const openDeleteDialog = async () => {
  await screen.findByText("Test Note");
  fireEvent.click(screen.getByTitle("Options"));
  fireEvent.click(screen.getByText("Delete"));
  await screen.findByRole("button", { name: "Delete" });
};

describe("NoteList onAfterMutation — delete", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useOffline).mockReturnValue(false);
  });

  it("calls onAfterMutation after successful online delete", async () => {
    const onAfterMutation = vi.fn();
    render(<NoteList onAfterMutation={onAfterMutation} />);

    await openDeleteDialog();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(onAfterMutation).toHaveBeenCalledTimes(1));
  });

  it("does not call onAfterMutation when deleting offline", async () => {
    vi.mocked(useOffline).mockReturnValue(true);
    vi.mocked(getAllNotesLocal).mockResolvedValueOnce([
      {
        id: "note-1",
        name: "Test Note",
        content: "",
        modifiedTime: "2026-05-14T00:00:00Z",
      },
    ]);
    const onAfterMutation = vi.fn();
    render(<NoteList onAfterMutation={onAfterMutation} />);

    await openDeleteDialog();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(deleteFile).not.toHaveBeenCalled());
    expect(onAfterMutation).not.toHaveBeenCalled();
  });

  it("does not call onAfterMutation when delete fails", async () => {
    vi.mocked(deleteFile).mockRejectedValueOnce(new Error("Server error"));
    const onAfterMutation = vi.fn();
    render(<NoteList onAfterMutation={onAfterMutation} />);

    await openDeleteDialog();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(deleteFile).toHaveBeenCalled());
    expect(onAfterMutation).not.toHaveBeenCalled();
  });
});
