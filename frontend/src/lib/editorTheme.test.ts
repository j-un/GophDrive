import { describe, expect, it } from "vitest";
import { EditorState } from "@codemirror/state";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";

import { TAG_RE, WIKILINK_RE, isInCodeNode } from "./editorTheme";

/**
 * Executes a global regex against `input` and returns every match's captured
 * groups. Resets `lastIndex` between calls so a shared regex instance is safe.
 */
function matchAll(re: RegExp, input: string): RegExpExecArray[] {
  re.lastIndex = 0;
  const out: RegExpExecArray[] = [];
  let m: RegExpExecArray | null;
  while ((m = re.exec(input)) !== null) {
    out.push(m);
    if (m[0].length === 0) re.lastIndex++; // zero-length safety
  }
  return out;
}

describe("WIKILINK_RE", () => {
  it("matches a simple [[Title]]", () => {
    const matches = matchAll(WIKILINK_RE, "See [[Auth Design]] for details.");
    expect(matches).toHaveLength(1);
    expect(matches[0][0]).toBe("[[Auth Design]]");
    expect(matches[0][1]).toBe("Auth Design");
  });

  it("matches multiple wikilinks on one line", () => {
    const matches = matchAll(WIKILINK_RE, "[[A]] and [[B]] and [[C]]");
    expect(matches.map((m) => m[1])).toEqual(["A", "B", "C"]);
  });

  it("matches a Japanese title", () => {
    const matches = matchAll(WIKILINK_RE, "参考: [[認証設計]]");
    expect(matches).toHaveLength(1);
    expect(matches[0][1]).toBe("認証設計");
  });

  it("does not match [] (single brackets)", () => {
    expect(matchAll(WIKILINK_RE, "See [not a wikilink]")).toHaveLength(0);
  });

  it("does not match [[]] with empty title", () => {
    expect(matchAll(WIKILINK_RE, "empty [[]] here")).toHaveLength(0);
  });

  it("does not match [[title|alias]] (pipe excluded)", () => {
    expect(matchAll(WIKILINK_RE, "[[Auth|Login]]")).toHaveLength(0);
  });

  it("does not match [[title#anchor]] (hash excluded)", () => {
    expect(matchAll(WIKILINK_RE, "[[Auth#section]]")).toHaveLength(0);
  });

  it("returns nothing for plain text", () => {
    expect(matchAll(WIKILINK_RE, "just prose with no links")).toHaveLength(0);
  });
});

describe("TAG_RE", () => {
  it("matches a basic inline tag after whitespace", () => {
    const matches = matchAll(TAG_RE, "hello #dev world");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("dev");
  });

  it("matches a tag at the start of the string", () => {
    const matches = matchAll(TAG_RE, "#feature is ready");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("feature");
  });

  it("matches hierarchical tag with slash", () => {
    const matches = matchAll(TAG_RE, "See #work/q3 for info");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("work/q3");
  });

  it("matches hyphenated tag", () => {
    const matches = matchAll(TAG_RE, "status: #in-progress");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("in-progress");
  });

  it("matches a CJK tag body", () => {
    const matches = matchAll(TAG_RE, "今日 #開発 完了");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("開発");
  });

  it("matches a tag after Japanese punctuation", () => {
    const matches = matchAll(TAG_RE, "タグ。#開発する");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("開発する");
  });

  it("matches a tag right after a `>` (blockquote/HTML close) boundary", () => {
    const matches = matchAll(TAG_RE, ">#tag");
    expect(matches).toHaveLength(1);
    expect(matches[0][1]).toBe(">");
    expect(matches[0][2]).toBe("tag");
  });

  it("matches a quoted-line tag (`> #tag`, boundary is the space after `>`)", () => {
    const matches = matchAll(TAG_RE, "> #tag");
    expect(matches).toHaveLength(1);
    expect(matches[0][2]).toBe("tag");
  });

  it("does not match digit-only sequences", () => {
    // #1 has a digit start — first tag-body char must be a letter or _.
    expect(matchAll(TAG_RE, "issue #1 closed")).toHaveLength(0);
  });

  it("does not match mid-word hash", () => {
    expect(matchAll(TAG_RE, "foo#bar is not a tag")).toHaveLength(0);
  });

  it("matches multiple tags in one line", () => {
    const matches = matchAll(TAG_RE, "#alpha and #beta and #gamma");
    expect(matches.map((m) => m[2])).toEqual(["alpha", "beta", "gamma"]);
  });

  it("returns nothing for plain text", () => {
    expect(matchAll(TAG_RE, "no tags here at all")).toHaveLength(0);
  });
});

describe("isInCodeNode", () => {
  function makeState(doc: string): EditorState {
    return EditorState.create({
      doc,
      extensions: [markdown({ base: markdownLanguage })],
    });
  }

  it("is false for a position in plain paragraph text", () => {
    const doc = "hello #tag world";
    const state = makeState(doc);
    const pos = doc.indexOf("#tag");
    expect(isInCodeNode(state, pos)).toBe(false);
  });

  it("is true for a position inside inline code (`#tag`)", () => {
    const doc = "before `#tag` after";
    const state = makeState(doc);
    const pos = doc.indexOf("#tag");
    expect(isInCodeNode(state, pos)).toBe(true);
  });

  it("is true for a position inside a fenced code block", () => {
    const doc = "```\n#tag\n```\n";
    const state = makeState(doc);
    const pos = doc.indexOf("#tag");
    expect(isInCodeNode(state, pos)).toBe(true);
  });

  it("is true for a position inside an indented code block", () => {
    const doc = "para\n\n    #tag\n";
    const state = makeState(doc);
    const pos = doc.indexOf("#tag");
    expect(isInCodeNode(state, pos)).toBe(true);
  });

  it("is false right outside a fenced code block (in the following paragraph)", () => {
    const doc = "```\ncode\n```\n#tag after\n";
    const state = makeState(doc);
    const pos = doc.lastIndexOf("#tag");
    expect(isInCodeNode(state, pos)).toBe(false);
  });

  it("is true for a wikilink-shaped token inside inline code", () => {
    const doc = "see `[[Not A Link]]` here";
    const state = makeState(doc);
    const pos = doc.indexOf("[[Not A Link]]");
    expect(isInCodeNode(state, pos)).toBe(true);
  });
});
