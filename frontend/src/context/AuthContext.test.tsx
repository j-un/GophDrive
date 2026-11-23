import "fake-indexeddb/auto";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { getUser } from "@/lib/api";
import { getNoteLocal, saveNoteLocal } from "@/lib/idb";
import { AuthProvider, useAuth } from "./AuthContext";

vi.mock("react-router", () => import("@/__mocks__/react-router"));

vi.mock("@/lib/api", () => ({
  getUser: vi.fn(),
}));

let session: { id: string; base_folder_id: string } | { error: Error };

function SessionProbe() {
  const { user, loading, refreshUser } = useAuth();
  if (loading) return <div>loading</div>;
  return (
    <button type="button" onClick={() => void refreshUser()}>
      user:{user?.id ?? "none"}
    </button>
  );
}

async function settle() {
  render(
    <AuthProvider>
      <SessionProbe />
    </AuthProvider>,
  );
  const button = await screen.findByRole("button");
  return button;
}

beforeEach(() => {
  session = { error: new Error("unset") };
  vi.mocked(getUser).mockImplementation(async () => {
    if ("error" in session) throw session.error;
    return session;
  });
});

afterEach(() => {
  cleanup();
});

describe("AuthProvider local note cache", () => {
  it("drops another subject's note when the session subject changes", async () => {
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
    session = { id: "user-a", base_folder_id: "root-a" };

    const button = await settle();
    expect(button.textContent).toBe("user:user-a");
    expect((await getNoteLocal("note-a", "user-a"))?.content).toBe(
      "alice body",
    );

    session = { id: "user-b", base_folder_id: "root-b" };
    fireEvent.click(button);

    expect(
      await screen.findByRole("button", { name: "user:user-b" }),
    ).toBeTruthy();
    expect(await getNoteLocal("note-a", "user-a")).toBeUndefined();
    expect((await getNoteLocal("note-b", "user-b"))?.content).toBe("bob body");
  });

  it("drops cached notes when the session ends", async () => {
    await saveNoteLocal({
      id: "note-logout",
      subject: "user-a",
      name: "Alice secret",
      content: "alice body",
      modifiedTime: "2026-01-01T00:00:00.000Z",
    });
    session = { error: new Error("Failed to get user profile: Unauthorized") };

    const button = await settle();
    expect(button.textContent).toBe("user:none");
    expect(await getNoteLocal("note-logout", "user-a")).toBeUndefined();
  });

  it("keeps the current subject's note when the profile request fails on the network", async () => {
    await saveNoteLocal({
      id: "note-offline",
      subject: "user-a",
      name: "Alice secret",
      content: "alice body",
      modifiedTime: "2026-01-01T00:00:00.000Z",
    });
    session = { error: new TypeError("Failed to fetch") };

    const button = await settle();
    expect(button.textContent).toBe("user:none");
    expect((await getNoteLocal("note-offline", "user-a"))?.content).toBe(
      "alice body",
    );
  });
});
