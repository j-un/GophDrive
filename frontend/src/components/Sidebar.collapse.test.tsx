import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
  within,
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

vi.mock("react-router", () => import("@/__mocks__/react-router"));

import {
  listRecent,
  listStarred,
  listTags,
  starFile,
  type FileItem,
} from "@/lib/api";
import { mockNavigate } from "@/__mocks__/react-router";
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

type SidebarTestProps = {
  refreshTrigger?: number;
  isOpen?: boolean;
  onClose?: () => void;
};

const renderSidebar = (props: SidebarTestProps = {}) =>
  render(<Sidebar onNavigate={() => {}} {...props} />);

// Sidebar's mount effect runs loadFolders(), which awaits listStarred ->
// listRecent -> listTags in sequence and only then sets state. A test that
// renders and asserts synchronously returns before those resolutions land, so
// the state updates arrive outside the test's act() scope and React warns
// "An update to Sidebar inside a test was not wrapped in act(...)". Whichever
// synchronous test happens to be running when the microtasks flush gets blamed,
// which makes the warning look intermittent and unrelated to the real cause.
//
// Use this instead of renderSidebar() in any test that does not otherwise await
// the loaded data. Waiting on listTags (the last call in the chain) rather than
// flushing blindly guarantees the whole chain has settled; the act() flush then
// applies the renders it produced.
const renderSidebarSettled = async (props: SidebarTestProps = {}) => {
  const result = renderSidebar(props);
  await waitFor(() => expect(listTags).toHaveBeenCalled());
  await act(async () => {});
  return result;
};

const getRail = () => screen.getByTestId("sidebar-rail");
const getPanel = () => screen.getByTestId("sidebar-panel");
const getRoot = (container: HTMLElement) => container.firstChild as HTMLElement;

// The mobile backdrop has no data-testid; it's the only element whose CSS
// Module class name contains "overlay" (base class is always applied, so
// this matches whether or not `.overlayOpen` is also present).
const getOverlay = () =>
  document.querySelector('[class*="overlay"]') as HTMLElement;

const getRecentButton = () =>
  within(getPanel()).getByRole("button", { name: /recent/i });

const getRecentList = () => {
  const list = document.getElementById("sidebar-recent-list");
  if (!list) throw new Error("recent list region not found");
  return list as HTMLElement;
};

// Isolate the new `sidebar:collapsed` key (and any other sidebar state)
// between tests; individual describes below still call `.clear()` or
// `.setItem()` themselves where the ordering matters for readability.
beforeEach(() => {
  window.localStorage.clear();
  // renderSidebarSettled() waits for listTags to have been called by *this*
  // render; without clearing, a previous test's call would satisfy it instantly
  // and the wait would be a no-op.
  vi.mocked(listTags).mockClear();
});

describe("Sidebar Recent section collapsible", () => {
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
    mockNavigate.mockClear();
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
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("calls router.push and onClose when a starred note is clicked", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([NOTE_ITEM]);
    const onClose = vi.fn();
    render(<Sidebar onNavigate={() => {}} onClose={onClose} />);
    await screen.findByText("StarredNote");
    fireEvent.click(screen.getByText("StarredNote"));
    expect(mockNavigate).toHaveBeenCalledWith("/note/?id=note-1");
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
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("keeps the sidebar expanded after navigating via a starred note (no longer auto-collapses)", async () => {
    vi.mocked(listStarred).mockResolvedValueOnce([NOTE_ITEM]);
    const { container } = renderSidebar();
    const root = getRoot(container);
    await screen.findByText("StarredNote");

    fireEvent.click(screen.getByText("StarredNote"));

    expect(mockNavigate).toHaveBeenCalledWith("/note/?id=note-1");
    expect(root.className).not.toMatch(/rootCollapsed/);
  });
});

describe("Sidebar Recent section keyboard access", () => {
  beforeEach(() => {
    mockNavigate.mockClear();
  });

  it("activates a recent row via Enter, matching Starred row semantics", async () => {
    renderSidebar();

    const row = await within(getPanel()).findByRole("button", {
      name: "MySecretNote",
    });
    expect(row.getAttribute("tabIndex")).toBe("0");

    fireEvent.keyDown(row, { key: "Enter" });
    expect(mockNavigate).toHaveBeenCalledWith("/note/?id=n1");
  });

  it("activates a recent row via Space", async () => {
    renderSidebar();

    const row = await within(getPanel()).findByRole("button", {
      name: "MySecretNote",
    });
    fireEvent.keyDown(row, { key: " " });
    expect(mockNavigate).toHaveBeenCalledWith("/note/?id=n1");
  });
});

describe("Sidebar branding link", () => {
  it("navigates to /drive/ when the logo is clicked", async () => {
    await renderSidebarSettled();
    expect(
      screen.getByRole("link", { name: "GophDrive" }).getAttribute("href"),
    ).toBe("/drive/");
  });
});

describe("Sidebar icon rail (collapsed)", () => {
  beforeEach(() => {
    window.localStorage.setItem("sidebar:collapsed", "true");
  });

  it("shows Expand sidebar, Notes, and Settings on the rail", async () => {
    await renderSidebarSettled();
    const rail = getRail();
    expect(
      within(rail).getByRole("button", { name: "Expand sidebar" }),
    ).toBeTruthy();
    expect(within(rail).getByRole("button", { name: "Notes" })).toBeTruthy();
    expect(within(rail).getByRole("button", { name: "Settings" })).toBeTruthy();
  });

  it("no longer has separate Search/Starred/Recent/Tags rail buttons", async () => {
    await renderSidebarSettled();
    const rail = getRail();
    expect(within(rail).queryByRole("button", { name: "Search" })).toBeNull();
    expect(within(rail).queryByRole("button", { name: "Starred" })).toBeNull();
    expect(within(rail).queryByRole("button", { name: "Recent" })).toBeNull();
    expect(within(rail).queryByRole("button", { name: "Tags" })).toBeNull();
  });
});

describe("Sidebar collapse/expand toggle", () => {
  it("collapses on 'Collapse sidebar' click, persists to localStorage, and expands again on 'Expand sidebar'", async () => {
    const { container } = await renderSidebarSettled();
    const root = getRoot(container);

    expect(root.className).not.toMatch(/rootCollapsed/);

    fireEvent.click(
      within(getPanel()).getByRole("button", { name: "Collapse sidebar" }),
    );

    expect(root.className).toMatch(/rootCollapsed/);
    expect(window.localStorage.getItem("sidebar:collapsed")).toBe("true");

    fireEvent.click(
      within(getRail()).getByRole("button", { name: "Expand sidebar" }),
    );

    expect(root.className).not.toMatch(/rootCollapsed/);
    expect(window.localStorage.getItem("sidebar:collapsed")).toBe("false");
  });

  it("starts collapsed when sidebar:collapsed=true is set before mount", async () => {
    window.localStorage.setItem("sidebar:collapsed", "true");

    const { container } = await renderSidebarSettled();
    const root = getRoot(container);

    expect(root.className).toMatch(/rootCollapsed/);
  });

  it("defaults to expanded when no sidebar:collapsed key is present", async () => {
    const { container } = await renderSidebarSettled();
    const root = getRoot(container);

    expect(root.className).not.toMatch(/rootCollapsed/);
    expect(window.localStorage.getItem("sidebar:collapsed")).toBe("false");
  });
});

describe("Sidebar Cmd+K shortcut", () => {
  it("focuses the search input", async () => {
    renderSidebar();

    fireEvent.keyDown(window, { key: "k", metaKey: true });

    const input = screen.getByPlaceholderText("Search...");
    await waitFor(() => expect(document.activeElement).toBe(input));
  });

  it("also responds to Ctrl+K", async () => {
    renderSidebar();

    fireEvent.keyDown(window, { key: "k", ctrlKey: true });

    const input = screen.getByPlaceholderText("Search...");
    await waitFor(() => expect(document.activeElement).toBe(input));
  });

  it("expands from a collapsed state and focuses the search input", async () => {
    window.localStorage.setItem("sidebar:collapsed", "true");
    const { container } = renderSidebar();
    const root = getRoot(container);
    expect(root.className).toMatch(/rootCollapsed/);

    fireEvent.keyDown(window, { key: "k", metaKey: true });

    await waitFor(() => expect(root.className).not.toMatch(/rootCollapsed/));
    const input = screen.getByPlaceholderText("Search...");
    await waitFor(() => expect(document.activeElement).toBe(input));
  });
});

describe("Sidebar persistence (no dismiss-on-away behavior)", () => {
  it("does not collapse on Escape", async () => {
    const { container } = await renderSidebarSettled();
    const root = getRoot(container);
    expect(root.className).not.toMatch(/rootCollapsed/);

    fireEvent.keyDown(window, { key: "Escape" });

    expect(root.className).not.toMatch(/rootCollapsed/);
  });

  it("does not collapse on a mousedown outside the panel", async () => {
    const { container } = await renderSidebarSettled();
    const root = getRoot(container);
    expect(root.className).not.toMatch(/rootCollapsed/);

    fireEvent.mouseDown(document.body);

    expect(root.className).not.toMatch(/rootCollapsed/);
  });
});

describe("Sidebar mobile drawer independence from desktop collapse", () => {
  it("shows the drawer (panelMobileOpen) and calls onClose on overlay click, even while desktop-collapsed", async () => {
    window.localStorage.setItem("sidebar:collapsed", "true");
    const onClose = vi.fn();
    await renderSidebarSettled({ isOpen: true, onClose });

    expect(getPanel().className).toMatch(/panelMobileOpen/);

    fireEvent.click(getOverlay());

    expect(onClose).toHaveBeenCalled();
  });
});
