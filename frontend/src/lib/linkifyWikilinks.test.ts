import { describe, it, expect } from "vitest";
import { linkifyWikilinks, WikiLinkRef } from "./linkifyWikilinks";

const resolved = (
  title: string,
  targetId: string,
  currentTitle?: string,
): WikiLinkRef => ({
  title,
  targetId,
  currentTitle,
  resolved: true,
});

const unresolved = (title: string): WikiLinkRef => ({
  title,
  resolved: false,
});

describe("linkifyWikilinks", () => {
  it("returns html unchanged when links array is empty", () => {
    const html = "<p>See [[Auth Design]] for details.</p>";
    expect(linkifyWikilinks(html, [])).toBe(html);
  });

  it("renders a resolved link with targetId", () => {
    const links = [resolved("Auth Design", "uuid-1")];
    const result = linkifyWikilinks("<p>[[Auth Design]]</p>", links);
    expect(result).toContain('data-note-id="uuid-1"');
    expect(result).toContain('href="/note?id=uuid-1"');
    expect(result).toContain('class="wikilink"');
    expect(result).toContain(">Auth Design</a>");
  });

  it("uses currentTitle as link label for resolved links", () => {
    const links = [resolved("Auth Design", "uuid-1", "Authentication Design")];
    const result = linkifyWikilinks("<p>[[Auth Design]]</p>", links);
    expect(result).toContain(">Authentication Design</a>");
    expect(result).not.toContain(">Auth Design</a>");
  });

  it("renders an unresolved link with wikilink-unresolved class", () => {
    const links = [unresolved("Missing Note")];
    const result = linkifyWikilinks("<p>[[Missing Note]]</p>", links);
    expect(result).toContain('class="wikilink wikilink-unresolved"');
    expect(result).toContain('data-note-title="Missing Note"');
    expect(result).toContain('href="#"');
    expect(result).toContain(">Missing Note</a>");
  });

  it("treats unknown title (not in links array) as unresolved", () => {
    const links = [resolved("Other Note", "uuid-2")];
    const result = linkifyWikilinks("<p>[[Unknown]]</p>", links);
    expect(result).toContain("wikilink-unresolved");
  });

  it("skips [[...]] inside <code> blocks", () => {
    const links = [resolved("Auth Design", "uuid-1")];
    const html = "<p><code>[[Auth Design]]</code></p>";
    const result = linkifyWikilinks(html, links);
    expect(result).not.toContain("data-note-id");
    expect(result).toContain("[[Auth Design]]");
  });

  it("skips [[...]] inside <pre> blocks", () => {
    const links = [resolved("Auth Design", "uuid-1")];
    const html = "<pre>[[Auth Design]]</pre>";
    const result = linkifyWikilinks(html, links);
    expect(result).not.toContain("data-note-id");
    expect(result).toContain("[[Auth Design]]");
  });

  it("resolves outside pre/code but skips inside", () => {
    const links = [resolved("Auth Design", "uuid-1")];
    const html = "<pre>[[Auth Design]]</pre><p>[[Auth Design]]</p>";
    const result = linkifyWikilinks(html, links);
    expect(result).toContain("[[Auth Design]]");
    expect(result).toContain('data-note-id="uuid-1"');
  });

  it("escapes special HTML characters in title and id", () => {
    const links = [resolved("<b>Evil</b>", "uuid-&1")];
    const result = linkifyWikilinks("<p>[[<b>Evil</b>]]</p>", links);
    expect(result).toContain("&lt;b&gt;Evil&lt;/b&gt;");
    expect(result).not.toContain("<b>Evil</b>");
  });

  it("is case-insensitive when matching titles", () => {
    const links = [resolved("auth design", "uuid-1")];
    const result = linkifyWikilinks("<p>[[Auth Design]]</p>", links);
    expect(result).toContain('data-note-id="uuid-1"');
  });

  it("treats a nested <pre><code> as one skipped unit (backreference)", () => {
    const links = [resolved("Auth Design", "uuid-1")];
    const html =
      "<pre><code>[[Auth Design]]</code>[[Auth Design]]</pre><p>[[Auth Design]]</p>";
    const result = linkifyWikilinks(html, links);

    // Exactly one link, and only from the <p> outside the pre/code block.
    expect(result.match(/data-note-id="uuid-1"/g)).toHaveLength(1);
    // Both occurrences inside the <pre> (including the one between </code>
    // and </pre>) stay literal — no leakage past the inner </code>.
    expect(result).toContain(
      "<pre><code>[[Auth Design]]</code>[[Auth Design]]</pre>",
    );
  });
});
