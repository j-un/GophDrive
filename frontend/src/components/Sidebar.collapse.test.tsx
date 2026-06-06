import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";

vi.mock("@/lib/api", () => ({
  listFiles: vi.fn().mockResolvedValue([]),
  listStarred: vi.fn().mockResolvedValue([]),
  listRecent: vi
    .fn()
    .mockResolvedValue([
      { id: "n1", name: "MySecretNote", mimeType: "text/markdown" },
    ]),
  listTags: vi.fn().mockResolvedValue([]),
  createFolder: vi.fn(),
  deleteFile: vi.fn(),
  renameNote: vi.fn(),
  starFile: vi.fn().mockResolvedValue({}),
}));

const mockPush = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => new URLSearchParams(),
}));

import { listRecent, listStarred, starFile, type FileItem } from "@/lib/api";
import { Sidebar } from "./Sidebar";

const FOLDER_ITEM: FileItem = {
  id: "folder-1",
  name: "StarredFolder",
  mimeType: "application/vnd.google-apps.folder",
  starred: true,
  parents: [],
  modifiedTime: "2026-05-14T00:00:00Z",
  size: 0,
};
const NOTE_ITEM: FileItem = {
  id: "note-1",
  name: "StarredNote",
  mimeType: "application/vnd.google-apps.document",
  starred: true,
  parents: [],
  modifiedTime: "2026-05-14T00:00:00Z",
  size: 0,
};

const renderSidebar = (props: { refreshTrigger?: number } = {}) =>
  render(<Sidebar onNavigate={() => {}} {...props} />);

const getRecentButton = () => screen.getByRole("button", { name: /recent/i });

const getRecentList = () => {
  const list = document.getElementById("sidebar-recent-list");
  if (!list) throw new Error("recent list region not found");
  return list as HTMLElement;
};

describe("Sidebar Recent section collapsible", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("shows recent note names by default and hides them after toggling", async () => {
    renderSidebar();

    expect(await screen.findByText("MySecretNote")).toBeTruthy();
    expect(getRecentButton().getAttribute("aria-expanded")).toBe("true");
    expect(getRecentList().hidden).toBe(false);

    await act(async () => {
      fireEvent.click(getRecentButton());
    });

    expect(getRecentButton().getAttribute("aria-expanded")).toBe("false");
    expect(getRecentList().hidden).toBe(true);
    expect(window.localStorage.getItem("sidebar:recent:collapsed")).toBe(
      "true",
    );
  });

  it("restores collapsed state from localStorage on mount", async () => {
    window.localStorage.setItem("sidebar:recent:collapsed", "true");

    renderSidebar();

    await waitFor(() => {
      expect(getRecentButton().getAttribute("aria-expanded")).toBe("false");
    });
    expect(getRecentList().hidden).toBe(true);
  });

  it("keeps aria-controls target element rendered while collapsed", async () => {
    window.localStorage.setItem("sidebar:recent:collapsed", "true");

    renderSidebar();

    await waitFor(() => {
      expect(getRecentButton().getAttribute("aria-controls")).toBe(
        "sidebar-recent-list",
      );
    });

    expect(document.getElementById("sidebar-recent-list")).not.toBeNull();
  });
});

describe("Sidebar refreshTrigger", () => {
  beforeEach(() => {
    vi.mocked(listRecent).mockClear();
  });

  it("re-fetches listRecent when refreshTrigger increments", async () => {
    const { rerender } = renderSidebar({ refreshTrigger: 0 });
    await waitFor(() => expect(listRecent).toHaveBeenCalledTimes(1));

    rerender(<Sidebar onNavigate={() => {}} refreshTrigger={1} />);

    await waitFor(() => expect(listRecent).toHaveBeenCalledTimes(2));
  });

  it("does not re-fetch when refreshTrigger is unchanged", async () => {
    const { rerender } = renderSidebar({ refreshTrigger: 0 });
    await waitFor(() => expect(listRecent).toHaveBeenCalledTimes(1));

    rerender(<Sidebar onNavigate={() => {}} refreshTrigger={0} />);

    // Allow a render cycle; count must not increase
    await new Promise((r) => setTimeout(r, 50));
    expect(listRecent).toHaveBeenCalledTimes(1);
  });
});

describe("Sidebar Starred section", () => {
  beforeEach(() => {
    mockPush.mockClear();
    vi.mocked(listStarred).mockClear();
    vi.mocked(starFile).mockClear();
  });

  it("shows 'Nothing starred' when list is empty", async () => {
    renderSidebar();
    expect(await screen.findByText("Nothing starred")).toBeTruthy();
  });

  it("renders both folder and note items from listStarred", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([FOLDER_ITEM, NOTE_ITEM]);
    renderSidebar();
    expect(await screen.findByText("StarredFolder")).toBeTruthy();
    expect(screen.getByText("StarredNote")).toBeTruthy();
  });

  it("calls onNavigate when a starred folder is clicked", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([FOLDER_ITEM]);
    const onNavigate = vi.fn();
    render(<Sidebar onNavigate={onNavigate} />);
    await screen.findByText("StarredFolder");
    fireEvent.click(screen.getByText("StarredFolder"));
    expect(onNavigate).toHaveBeenCalledWith("folder-1");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("calls router.push and onClose when a starred note is clicked", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([NOTE_ITEM]);
    const onClose = vi.fn();
    render(<Sidebar onNavigate={() => {}} onClose={onClose} />);
    await screen.findByText("StarredNote");
    fireEvent.click(screen.getByText("StarredNote"));
    expect(mockPush).toHaveBeenCalledWith("/note/?id=note-1");
    expect(onClose).toHaveBeenCalled();
  });

  it("calls starFile(id, false) when the Unstar button is clicked", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([NOTE_ITEM]);
    renderSidebar();
    await screen.findByText("StarredNote");
    fireEvent.click(screen.getByRole("button", { name: "Unstar StarredNote" }));
    await waitFor(() => expect(starFile).toHaveBeenCalledWith("note-1", false));
  });

  it("does not navigate when Unstar button is clicked (stopPropagation)", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([NOTE_ITEM]);
    renderSidebar();
    await screen.findByText("StarredNote");
    fireEvent.click(screen.getByRole("button", { name: "Unstar StarredNote" }));
    await waitFor(() => expect(starFile).toHaveBeenCalled());
    expect(mockPush).not.toHaveBeenCalled();
  });
});
