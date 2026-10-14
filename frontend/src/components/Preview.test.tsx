import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor, cleanup } from "@testing-library/react";
import type { WikiLinkRef } from "@/lib/linkifyWikilinks";

vi.mock("react-router", () => import("@/__mocks__/react-router"));

vi.mock("@/hooks/useWasm", () => ({
  useWasm: () => ({ isReady: true, error: null }),
}));

import { Preview } from "./Preview";

describe("Preview XSS sanitization", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "renderMarkdown",
      vi.fn((s: string) => s),
    );
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("strips onerror handler from img tag", async () => {
    const { container } = render(
      <Preview markdown='<img src="x" onerror="alert(1)">' />,
    );
    await waitFor(() => {
      expect(
        container.querySelector("img")?.getAttribute("onerror"),
      ).toBeNull();
    });
  });

  it("strips javascript: href from anchor tag", async () => {
    const { container } = render(
      <Preview markdown='<a href="javascript:alert(1)">click</a>' />,
    );
    await waitFor(() => {
      const href = container.querySelector("a")?.getAttribute("href") ?? "";
      expect(href).not.toContain("javascript:");
    });
  });

  it("strips script tag entirely", async () => {
    const { container } = render(
      <Preview markdown="<script>alert(1)</script><p>safe</p>" />,
    );
    await waitFor(() => {
      expect(container.querySelector("script")).toBeNull();
    });
  });

  it("adds rel=noopener noreferrer to target=_blank links", async () => {
    const { container } = render(
      <Preview markdown='<a href="https://example.com" target="_blank">External</a>' />,
    );
    await waitFor(() => {
      const rel = container.querySelector("a")?.getAttribute("rel") ?? "";
      expect(rel).toContain("noopener");
      expect(rel).toContain("noreferrer");
    });
  });

  it("preserves data-note-id on wikilinks through full pipeline", async () => {
    const links: WikiLinkRef[] = [
      {
        title: "Test Note",
        targetId: "abc123",
        currentTitle: "Test Note",
        resolved: true,
      },
    ];
    const { container } = render(
      <Preview markdown="[[Test Note]]" links={links} />,
    );
    await waitFor(() => {
      const a = container.querySelector("a.wikilink");
      expect(a?.getAttribute("data-note-id")).toBe("abc123");
    });
  });
});

describe("Preview active prop", () => {
  let renderMarkdownMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    renderMarkdownMock = vi.fn((s: string) => s);
    vi.stubGlobal("renderMarkdown", renderMarkdownMock);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("renders by default when active is unspecified", async () => {
    const { container } = render(<Preview markdown="hello" />);
    await waitFor(() => {
      expect(container.textContent).toContain("hello");
    });
    expect(renderMarkdownMock).toHaveBeenCalledWith("hello");
  });

  it("does not render markdown-derived content while inactive, even as markdown changes", () => {
    const { container, rerender } = render(
      <Preview markdown="one" active={false} />,
    );
    rerender(<Preview markdown="two" active={false} />);
    rerender(<Preview markdown="three" active={false} />);

    expect(container.textContent).not.toContain("one");
    expect(container.textContent).not.toContain("two");
    expect(container.textContent).not.toContain("three");

    expect(renderMarkdownMock).not.toHaveBeenCalled();
  });

  it("shows the latest markdown once reactivated, including edits made while inactive", async () => {
    const { container, rerender } = render(
      <Preview markdown="one" active={true} />,
    );
    await waitFor(() => {
      expect(container.textContent).toContain("one");
    });

    rerender(<Preview markdown="two" active={false} />);
    rerender(<Preview markdown="three" active={false} />);
    expect(renderMarkdownMock).not.toHaveBeenCalledWith("two");
    expect(renderMarkdownMock).not.toHaveBeenCalledWith("three");

    rerender(<Preview markdown="three" active={true} />);
    await waitFor(() => {
      expect(container.textContent).toContain("three");
    });
  });
});
