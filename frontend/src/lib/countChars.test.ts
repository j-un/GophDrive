import { describe, it, expect } from "vitest";
import { countChars } from "./countChars";

describe("countChars", () => {
  it("counts English letters, excluding whitespace", () => {
    expect(countChars("hello world")).toBe(10);
  });

  it("counts Japanese characters (no whitespace to exclude)", () => {
    // 今 日 は 良 い 天 気 で す — 9 characters, no spaces between words.
    expect(countChars("今日は良い天気です")).toBe(9);
  });

  it("counts a modifier-sequence emoji as one character", () => {
    // Thumbs-up + Fitzpatrick medium skin tone modifier: 2 code points / 4
    // UTF-16 code units, but a single visible grapheme cluster.
    expect(countChars("👍🏽")).toBe(1);
  });

  it("returns 0 for whitespace-only content (space, tab, newline, full-width space)", () => {
    expect(countChars("  \n\t　　")).toBe(0);
  });

  it("returns 0 for empty content", () => {
    expect(countChars("")).toBe(0);
  });

  it("falls back to code-point counting when Intl.Segmenter is unavailable", () => {
    const original = Intl.Segmenter;
    // @ts-expect-error -- simulating an environment without Intl.Segmenter
    delete Intl.Segmenter;
    try {
      expect(countChars("hello world")).toBe(10);
      // Documented fallback limitation: without grapheme segmentation, a
      // modifier-sequence emoji counts as 2 code points instead of 1.
      expect(countChars("👍🏽")).toBe(2);
    } finally {
      // @ts-expect-error -- restoring after simulating an unavailable Intl.Segmenter
      Intl.Segmenter = original;
    }
  });
});
