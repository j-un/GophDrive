import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { mockNavigate } from "@/__mocks__/react-router";
import { createFolder, createNote, deleteFile } from "@/lib/api";
import { getAllNotesLocal } from "@/lib/idb";
import { useOffline } from "@/hooks/useOffline";

const mockListFiles = vi.hoisted(() =>
  vi.fn().mockResolvedValue([
    {
      id: "note-1",
      name: "Test Note",
      mimeType: "application/vnd.google-apps.document",
      parents: [],
      modifiedTime: "2026-05-14T00:00:00Z",
      starred: false,
    },
  ]),
);

vi.mock("react-router", () => import("@/__mocks__/react-router"));

vi.mock("@/lib/api", () => ({
  listFiles: mockListFiles,
  deleteFile: vi.fn().mockResolvedValue(undefined),
  duplicateNote: vi.fn().mockResolvedValue({ id: "note-2" }),
  renameNote: vi.fn().mockResolvedValue(undefined),
  starFile: vi.fn().mockResolvedValue(undefined),
  searchFiles: vi.fn().mockResolvedValue([]),
  createNote: vi.fn().mockResolvedValue({ id: "note-new" }),
  createFolder: vi.fn().mockResolvedValue({ id: "folder-new" }),
}));

vi.mock("@/lib/idb", () => ({
  deleteNoteLocal: vi.fn().mockResolvedValue(undefined),
  getAllNotesLocal: vi.fn().mockResolvedValue([]),
}));

vi.mock("@/hooks/useOffline", () => ({
  useOffline: vi.fn().mockReturnValue(false),
}));

import NoteList from "./NoteList";

const openDeleteDialog = async () => {
  await screen.findByText("Test Note");
  fireEvent.click(screen.getByTitle("Options"));
  fireEvent.click(screen.getByText("Delete"));
  await screen.findByRole("button", { name: "Delete" });
};

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe("NoteList row list", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useOffline).mockReturnValue(false);
  });

  it("renders folder rows and note rows for the current folder", async () => {
    mockListFiles.mockResolvedValueOnce([
      {
        id: "folder-1",
        name: "My Folder",
        mimeType: "application/vnd.google-apps.folder",
        parents: [],
        modifiedTime: "2026-05-14T00:00:00Z",
        starred: false,
      },
      {
        id: "note-1",
        name: "Test Note",
        mimeType: "application/vnd.google-apps.document",
        parents: [],
        modifiedTime: "2026-05-14T00:00:00Z",
        starred: false,
      },
    ]);
    render(<NoteList />);

    expect(await screen.findByText("My Folder")).toBeTruthy();
    expect(screen.getByText("Test Note")).toBeTruthy();
  });

  it("renders the folder row as a link to /drive/?folderId=...", async () => {
    mockListFiles.mockResolvedValueOnce([
      {
        id: "folder-1",
        name: "My Folder",
        mimeType: "application/vnd.google-apps.folder",
        parents: [],
        modifiedTime: "2026-05-14T00:00:00Z",
        starred: false,
      },
    ]);
    render(<NoteList />);

    const link = await screen.findByRole("link", { name: /My Folder/ });
    expect(link.getAttribute("href")).toBe("/drive/?folderId=folder-1");
  });

  it("renders the note row as a link to /note/?id=...", async () => {
    render(<NoteList />);

    const link = await screen.findByRole("link", { name: /Test Note/ });
    expect(link.getAttribute("href")).toBe("/note/?id=note-1");
  });

  it("does not render a Refresh button", async () => {
    render(<NoteList />);
    await screen.findByText("Test Note");
    expect(screen.queryByText("Refresh")).toBeNull();
    expect(screen.queryByTitle("Refresh")).toBeNull();
  });

  it("renders a note's tags as separate links to /drive/?tag=... (not the note link)", async () => {
    mockListFiles.mockResolvedValueOnce([
      {
        id: "note-1",
        name: "Tagged Note",
        mimeType: "application/vnd.google-apps.document",
        parents: [],
        modifiedTime: "2026-05-14T00:00:00Z",
        starred: false,
        tags: ["work"],
      },
    ]);
    render(<NoteList />);

    const tagLink = await screen.findByRole("link", { name: "#work" });
    expect(tagLink.getAttribute("href")).toBe("/drive/?tag=work");

    const noteLink = screen.getByRole("link", { name: /Tagged Note/ });
    expect(noteLink.getAttribute("href")).toBe("/note/?id=note-1");
  });

  it("opens the note menu without navigating to the note", async () => {
    render(<NoteList />);

    await screen.findByText("Test Note");
    fireEvent.click(screen.getByTitle("Options"));

    expect(await screen.findByText("Delete")).toBeTruthy();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("shows a filled star for starred notes", async () => {
    mockListFiles.mockResolvedValueOnce([
      {
        id: "note-1",
        name: "Starred Note",
        mimeType: "application/vnd.google-apps.document",
        parents: [],
        modifiedTime: "2026-05-14T00:00:00Z",
        starred: true,
      },
    ]);
    const { container } = render(<NoteList />);

    await screen.findByText("Starred Note");
    const star = container.querySelector("svg.lucide-star");
    expect(star).toBeTruthy();
    expect(star?.getAttribute("fill")).toBe("var(--star)");
  });
});

describe("NoteList — new note flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useOffline).mockReturnValue(false);
  });

  it("opens the inline row and creates a note when createNoteSignal is bumped (DrivePage's + New note / ⌘N)", async () => {
    mockListFiles.mockResolvedValueOnce([]);
    const { rerender } = render(<NoteList createNoteSignal={0} />);
    await waitFor(() => expect(mockListFiles).toHaveBeenCalled());

    // Simulates DrivePage incrementing the signal on button click or ⌘N.
    rerender(<NoteList createNoteSignal={1} />);

    const input = await screen.findByPlaceholderText("Note name...");
    fireEvent.change(input, { target: { value: "My New Note" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() =>
      expect(createNote).toHaveBeenCalledWith(
        "My New Note",
        "# My New Note",
        undefined,
      ),
    );
    expect(mockNavigate).toHaveBeenCalledWith("/note/?id=note-new");
  });

  it("does not open the inline row on initial mount", () => {
    render(<NoteList createNoteSignal={0} />);
    expect(screen.queryByPlaceholderText("Note name...")).toBeNull();
  });
});

describe("NoteList — new folder flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useOffline).mockReturnValue(false);
  });

  it("creates a folder from the inline '+ New folder' row", async () => {
    mockListFiles.mockResolvedValueOnce([]);
    render(<NoteList />);

    fireEvent.click(await screen.findByText("+ New folder"));
    const input = screen.getByPlaceholderText("Folder name...");
    fireEvent.change(input, { target: { value: "My Folder" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() =>
      expect(createFolder).toHaveBeenCalledWith("My Folder", undefined),
    );
  });
});

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
