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
  starFile: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

import { listRecent } from "@/lib/api";
import { Sidebar } from "./Sidebar";

const renderSidebar = (props: { refreshTrigger?: number } = {}) =>
  render(
    <Sidebar
      onNavigate={() => {}}
      breadcrumbs={[{ name: "Home" }]}
      {...props}
    />,
  );

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

    rerender(
      <Sidebar
        onNavigate={() => {}}
        breadcrumbs={[{ name: "Home" }]}
        refreshTrigger={1}
      />,
    );

    await waitFor(() => expect(listRecent).toHaveBeenCalledTimes(2));
  });

  it("does not re-fetch when refreshTrigger is unchanged", async () => {
    const { rerender } = renderSidebar({ refreshTrigger: 0 });
    await waitFor(() => expect(listRecent).toHaveBeenCalledTimes(1));

    rerender(
      <Sidebar
        onNavigate={() => {}}
        breadcrumbs={[{ name: "Home" }]}
        refreshTrigger={0}
      />,
    );

    // Allow a render cycle; count must not increase
    await new Promise((r) => setTimeout(r, 50));
    expect(listRecent).toHaveBeenCalledTimes(1);
  });
});
