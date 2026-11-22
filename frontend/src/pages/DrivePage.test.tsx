import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { mockNavigate } from "@/__mocks__/react-router";
import { createNote, type FileItem } from "@/lib/api";

const mockListFiles = vi.hoisted(() => vi.fn().mockResolvedValue([]));

vi.mock("@/components/Sidebar", () => ({
  Sidebar: () => null,
}));

vi.mock("react-router", () => import("@/__mocks__/react-router"));

vi.mock("@/lib/api", () => ({
  listFiles: mockListFiles,
  getBreadcrumbs: vi.fn(),
  isValidNoteId: vi.fn(),
  createNote: vi.fn().mockResolvedValue({ id: "note-new" }),
}));

import DrivePage from "./DrivePage";

function pressN(chord: {
  key: string;
  altKey?: boolean;
  metaKey?: boolean;
  ctrlKey?: boolean;
  shiftKey?: boolean;
}) {
  fireEvent.keyDown(window, { code: "KeyN", ...chord });
}

async function renderDrive() {
  render(<DrivePage />);
  await screen.findByText("No notes found.");
}

const timestampName = /^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}$/;

async function expectCreatedNote() {
  await waitFor(() => expect(createNote).toHaveBeenCalledTimes(1));
  const name = vi.mocked(createNote).mock.calls[0][0];
  expect(name).toMatch(timestampName);
  expect(createNote).toHaveBeenCalledWith(name, "# ", undefined);
  expect(mockNavigate).toHaveBeenCalledWith("/note/?id=note-new");
}

describe("DrivePage new note shortcut", () => {
  beforeEach(() => {
    mockListFiles.mockReset();
    mockListFiles.mockResolvedValue([]);
    vi.mocked(createNote).mockReset();
    vi.mocked(createNote).mockResolvedValue({ id: "note-new" } as FileItem);
    mockNavigate.mockClear();
  });

  it("shows ⌘⌥N on the New note button", async () => {
    await renderDrive();
    expect(
      screen.getByRole("button", { name: /New note/ }).textContent,
    ).toContain("⌘⌥N");
  });

  it("creates a note for ⌘⌥N when Option turns the key into ˜", async () => {
    await renderDrive();
    pressN({ key: "˜", altKey: true, metaKey: true });
    await expectCreatedNote();
  });

  it("creates a note for Ctrl+Alt+N", async () => {
    await renderDrive();
    pressN({ key: "n", altKey: true, ctrlKey: true });
    await expectCreatedNote();
  });

  it("does not create a note for ⌘N", async () => {
    await renderDrive();
    pressN({ key: "n", metaKey: true });
    expect(createNote).not.toHaveBeenCalled();
  });

  it("does not create a note for ⌘⇧⌥N", async () => {
    await renderDrive();
    pressN({ key: "N", altKey: true, metaKey: true, shiftKey: true });
    expect(createNote).not.toHaveBeenCalled();
  });

  it("does not create a note while an input is focused", async () => {
    await renderDrive();
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.focus();
    try {
      pressN({ key: "˜", altKey: true, metaKey: true });
      expect(createNote).not.toHaveBeenCalled();
    } finally {
      input.remove();
    }
  });

  it("creates a note from the New note button", async () => {
    await renderDrive();
    fireEvent.click(screen.getByRole("button", { name: /New note/ }));
    await expectCreatedNote();
  });
});
