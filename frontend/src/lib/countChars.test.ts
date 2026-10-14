import { describe, it, expect } from "vitest";
import { countChars } from "./countChars";

describe("countChars", () => {
  it("counts English letters, excluding whitespace", () => {
    expect(countChars("hello world")).toBe(10);
  });

  it("counts Japanese characters (no whitespace to exclude)", () => {
    expect(countChars("今日は良い天気です")).toBe(9);
  });

  it("counts a modifier-sequence emoji as one character", () => {
    expect(countChars("👍🏽")).toBe(1);
  });

  it("returns 0 for whitespace-only content (space, tab, newline, full-width space)", () => {
    expect(countChars("  \n\t　　")).toBe(0);
  });

  it("returns 0 for empty content", () => {
    expect(countChars("")).toBe(0);
  });

  it("falls back to code-point counting when Intl.Segmenter is unavailable", () => {
    const intl = Intl as { Segmenter?: typeof Intl.Segmenter };
    const original = intl.Segmenter;
    delete intl.Segmenter;
    try {
      expect(countChars("hello world")).toBe(10);
      expect(countChars("👍🏽")).toBe(2);
    } finally {
      intl.Segmenter = original;
    }
  });
});
