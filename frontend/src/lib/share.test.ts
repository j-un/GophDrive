import { describe, expect, it } from "vitest";
import { buildShareLink } from "./share";

const URL = "https://example.com/note?id=abc123";

describe("buildShareLink", () => {
  it("returns Markdown link when title is present", () => {
    expect(buildShareLink("My Note", URL)).toBe(`[My Note](${URL})`);
  });

  it("returns URL only when title is empty", () => {
    expect(buildShareLink("", URL)).toBe(URL);
  });

  it("returns URL only when title is whitespace-only", () => {
    expect(buildShareLink("   ", URL)).toBe(URL);
  });

  it("escapes ] in title", () => {
    expect(buildShareLink("Note [draft]", URL)).toBe(
      `[Note \\[draft\\]](${URL})`,
    );
  });

  it("escapes [ in title", () => {
    expect(buildShareLink("[WIP] Note", URL)).toBe(`[\\[WIP\\] Note](${URL})`);
  });

  it("escapes backslash in title", () => {
    expect(buildShareLink("path\\to\\file", URL)).toBe(
      `[path\\\\to\\\\file](${URL})`,
    );
  });
});
