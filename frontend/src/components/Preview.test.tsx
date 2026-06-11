import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor, cleanup } from "@testing-library/react";
import type { WikiLinkRef } from "@/lib/linkifyWikilinks";

const mockPush = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("@/hooks/useWasm", () => ({
  useWasm: () => ({ isReady: true, error: null }),
}));

import { Preview } from "./Preview";

describe("Preview XSS sanitization", () => {
  beforeEach(() => {
    // Identity stub: renderMarkdown returns its input unchanged so the full
    // linkifyTags → linkifyWikilinks → sanitizeRenderedMarkdown pipeline is exercised.
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
    // Exercises the real linkifyWikilinks → sanitizeRenderedMarkdown integration.
    // The identity renderMarkdown stub returns "[[Test Note]]" unchanged, then
    // linkifyWikilinks transforms it to <a class="wikilink" data-note-id="abc123">,
    // and sanitizeRenderedMarkdown must preserve the data-note-id attribute.
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
