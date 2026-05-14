import { describe, it, expect } from "vitest";
import { parseSearchQuery, buildSearchRequest } from "./searchQuery";

describe("parseSearchQuery", () => {
  it("returns empty text and no tags for empty string", () => {
    expect(parseSearchQuery("")).toEqual({ text: "", tags: [] });
  });

  it("returns plain text unchanged", () => {
    expect(parseSearchQuery("hello world")).toEqual({
      text: "hello world",
      tags: [],
    });
  });

  it("extracts a single tag", () => {
    expect(parseSearchQuery("tag:test")).toEqual({ text: "", tags: ["test"] });
  });

  it("extracts tag and keeps remaining text", () => {
    expect(parseSearchQuery("tag:test frontend")).toEqual({
      text: "frontend",
      tags: ["test"],
    });
  });

  it("extracts multiple tags", () => {
    expect(parseSearchQuery("tag:alpha tag:beta")).toEqual({
      text: "",
      tags: ["alpha", "beta"],
    });
  });

  it("handles tag in the middle of text", () => {
    expect(parseSearchQuery("hello tag:dev world")).toEqual({
      text: "hello world",
      tags: ["dev"],
    });
  });

  it("preserves tag case (backend uses EqualFold)", () => {
    expect(parseSearchQuery("tag:MyTag")).toEqual({
      text: "",
      tags: ["MyTag"],
    });
  });

  it("supports hierarchical tags with slash", () => {
    expect(parseSearchQuery("tag:work/q3")).toEqual({
      text: "",
      tags: ["work/q3"],
    });
  });

  it("silently drops bare 'tag:' with no value (not added to text or tags)", () => {
    expect(parseSearchQuery("tag:")).toEqual({ text: "", tags: [] });
  });

  it("handles extra whitespace between tokens", () => {
    expect(parseSearchQuery("  tag:dev  notes  ")).toEqual({
      text: "notes",
      tags: ["dev"],
    });
  });

  it("Tag:/TAG: is case-sensitive — treated as plain text", () => {
    expect(parseSearchQuery("Tag:foo")).toEqual({ text: "Tag:foo", tags: [] });
    expect(parseSearchQuery("TAG:foo")).toEqual({ text: "TAG:foo", tags: [] });
  });

  it("duplicate tags are preserved (dedup is buildSearchRequest's job)", () => {
    expect(parseSearchQuery("tag:a tag:a")).toEqual({
      text: "",
      tags: ["a", "a"],
    });
  });

  it("splits on tabs and newlines as well as spaces", () => {
    expect(parseSearchQuery("tag:a\thello\nworld")).toEqual({
      text: "hello world",
      tags: ["a"],
    });
  });

  it("captures everything after the first colon as the tag value", () => {
    expect(parseSearchQuery("tag:work:q3")).toEqual({
      text: "",
      tags: ["work:q3"],
    });
  });
});

describe("buildSearchRequest", () => {
  it("merges URL tags and query tags", () => {
    expect(buildSearchRequest("tag:a foo", ["b"])).toEqual({
      text: "foo",
      tags: ["b", "a"],
    });
  });

  it("URL tags come first in the merged list", () => {
    const { tags } = buildSearchRequest("tag:query", ["url"]);
    expect(tags).toEqual(["url", "query"]);
  });

  it("deduplicates tags from both sources", () => {
    expect(buildSearchRequest("tag:a", ["a"])).toEqual({
      text: "",
      tags: ["a"],
    });
  });

  it("returns undefined tags when no tags from either source", () => {
    expect(buildSearchRequest("hello world", [])).toEqual({
      text: "hello world",
      tags: undefined,
    });
  });

  it("returns undefined tags when both inputs are empty", () => {
    expect(buildSearchRequest(undefined, undefined)).toEqual({
      text: "",
      tags: undefined,
    });
  });

  it("text is empty when query is only tag tokens", () => {
    expect(buildSearchRequest("tag:dev tag:backend", undefined)).toEqual({
      text: "",
      tags: ["dev", "backend"],
    });
  });

  it("URL tags only, no query", () => {
    expect(buildSearchRequest(undefined, ["develop"])).toEqual({
      text: "",
      tags: ["develop"],
    });
  });
});
