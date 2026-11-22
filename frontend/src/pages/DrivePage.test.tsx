import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

const mockListFiles = vi.hoisted(() => vi.fn().mockResolvedValue([]));

vi.mock("@/components/Sidebar", () => ({
  Sidebar: () => null,
}));

vi.mock("react-router", () => import("@/__mocks__/react-router"));

vi.mock("@/lib/api", () => ({
  listFiles: mockListFiles,
  getBreadcrumbs: vi.fn(),
  isValidNoteId: vi.fn(),
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

describe("DrivePage new note shortcut", () => {
  beforeEach(() => {
    mockListFiles.mockReset();
    mockListFiles.mockResolvedValue([]);
  });

  it("shows ⌘⌥N on the New note button", async () => {
    await renderDrive();
    expect(
      screen.getByRole("button", { name: /New note/ }).textContent,
    ).toContain("⌘⌥N");
  });

  it("opens the name field for ⌘⌥N when Option turns the key into ˜", async () => {
    await renderDrive();
    pressN({ key: "˜", altKey: true, metaKey: true });
    expect(await screen.findByPlaceholderText("Note name...")).toBeTruthy();
  });

  it("opens the name field for Ctrl+Alt+N", async () => {
    await renderDrive();
    pressN({ key: "n", altKey: true, ctrlKey: true });
    expect(await screen.findByPlaceholderText("Note name...")).toBeTruthy();
  });

  it("does not open the name field for ⌘N", async () => {
    await renderDrive();
    pressN({ key: "n", metaKey: true });
    expect(screen.queryByPlaceholderText("Note name...")).toBeNull();
  });

  it("does not open the name field for ⌘⇧⌥N", async () => {
    await renderDrive();
    pressN({ key: "N", altKey: true, metaKey: true, shiftKey: true });
    expect(screen.queryByPlaceholderText("Note name...")).toBeNull();
  });

  it("leaves a typed name in place when the chord hits the name field", async () => {
    await renderDrive();
    pressN({ key: "˜", altKey: true, metaKey: true });
    const input = await screen.findByPlaceholderText("Note name...");
    fireEvent.change(input, { target: { value: "Keep" } });
    fireEvent.keyDown(input, {
      key: "˜",
      code: "KeyN",
      altKey: true,
      metaKey: true,
    });
    expect((input as HTMLInputElement).value).toBe("Keep");
  });
});
