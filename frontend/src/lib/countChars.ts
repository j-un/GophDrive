/**
 * Counts visible characters in note content for the NotePage zen-mode
 * indicator, excluding whitespace (space, tab, newline, and the U+3000
 * full-width space, which `\s` matches). A word count doesn't mean much
 * for Japanese/CJK text — those
 * scripts don't separate words with spaces — so NotePage shows a character
 * count instead.
 *
 * Uses `Intl.Segmenter` with grapheme-cluster granularity so a multi-code-
 * point sequence (an emoji with a skin-tone/ZWJ modifier, a combining
 * character) counts as the one visible character a user would count by eye,
 * rather than as 2+ characters. Where `Intl.Segmenter` isn't available, this
 * falls back to `Array.from`, which is still surrogate-pair safe (a
 * single-code-point emoji isn't split into 2), but doesn't merge grapheme
 * clusters — a modifier-sequence emoji counts as more than one character in
 * that fallback path.
 */
export function countChars(text: string): number {
  const stripped = text.replace(/\s/g, "");
  if (!stripped) return 0;
  if (typeof Intl !== "undefined" && typeof Intl.Segmenter === "function") {
    const segmenter = new Intl.Segmenter(undefined, {
      granularity: "grapheme",
    });
    let count = 0;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    for (const _ of segmenter.segment(stripped)) count++;
    return count;
  }
  return Array.from(stripped).length;
}
