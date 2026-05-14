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

import { Sidebar } from "./Sidebar";

const renderSidebar = () =>
  render(<Sidebar onNavigate={() => {}} breadcrumbs={[{ name: "Home" }]} />);

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
