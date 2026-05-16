import { describe, it, expect } from "vitest";
import { linkifyTags } from "./linkifyTags";

describe("linkifyTags", () => {
  it("linkifies a basic inline tag", () => {
    const result = linkifyTags("<p>hello #dev world</p>");
    expect(result).toContain(
      '<a href="/drive?tag=dev" class="tag-link">#dev</a>',
    );
  });

  it("linkifies a tag at the start of an HTML element content", () => {
    const result = linkifyTags("<p>#feature is ready</p>");
    expect(result).toContain(
      '<a href="/drive?tag=feature" class="tag-link">#feature</a>',
    );
  });

  it("escapes hierarchical tags in the href", () => {
    const result = linkifyTags("<p>See #work/q3 for info</p>");
    expect(result).toContain('href="/drive?tag=work%2Fq3"');
    expect(result).toContain(">#work/q3</a>");
  });

  it("linkifies hyphenated tags", () => {
    const result = linkifyTags("<p>Status: #in-progress</p>");
    expect(result).toContain('href="/drive?tag=in-progress"');
  });

  it("linkifies CJK tags", () => {
    const result = linkifyTags("<p>今日の作業 #開発 完了</p>");
    expect(result).toContain('href="/drive?tag=%E9%96%8B%E7%99%BA"');
  });

  it("skips content inside <code> blocks", () => {
    const result = linkifyTags(
      "<p>Use <code>#notag</code> but <span> #realtag</span></p>",
    );
    expect(result).not.toContain('href="/drive?tag=notag"');
    expect(result).toContain('href="/drive?tag=realtag"');
  });

  it("skips content inside <pre> blocks", () => {
    const result = linkifyTags(
      "<pre>#inside code block</pre>\n<p> #outside</p>",
    );
    expect(result).not.toContain('href="/drive?tag=inside"');
    expect(result).toContain('href="/drive?tag=outside"');
  });

  it("does not linkify digit-only sequences", () => {
    const result = linkifyTags("<p>Issue #1 is closed</p>");
    expect(result).not.toContain("tag-link");
  });

  it("does not linkify mid-word hash", () => {
    const result = linkifyTags("<p>foo#bar is not a tag</p>");
    expect(result).not.toContain("tag-link");
  });

  it("trims trailing slashes from tag name", () => {
    const result = linkifyTags("<p>See #work/ for info</p>");
    expect(result).toContain('href="/drive?tag=work"');
    expect(result).not.toContain("tag=work%2F");
  });

  it("preserves URL hash fragments in href attributes", () => {
    const result = linkifyTags(
      '<a href="https://example.com/page#section">link</a>',
    );
    expect(result).not.toContain("tag-link");
  });

  it("tag after Japanese punctuation", () => {
    const result = linkifyTags("<p>タグ。#開発する</p>");
    expect(result).toContain(
      'href="/drive?tag=%E9%96%8B%E7%99%BA%E3%81%99%E3%82%8B"',
    );
  });

  it("returns unchanged html when no tags present", () => {
    const html = "<p>Just plain text without tags</p>";
    expect(linkifyTags(html)).toBe(html);
  });
});
