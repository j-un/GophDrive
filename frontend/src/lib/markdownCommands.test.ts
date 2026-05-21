import { describe, expect, it } from "vitest";
import { EditorState, EditorSelection } from "@codemirror/state";
import type { TransactionSpec } from "@codemirror/state";
import {
  insertDivider,
  insertLink,
  toggleBold,
  toggleBulletList,
  toggleCheckList,
  toggleCode,
  toggleHeading,
  toggleInlineCode,
  toggleItalic,
  toggleOrderedList,
  toggleQuote,
  toggleStrike,
} from "./markdownCommands";

function apply(
  doc: string,
  from: number,
  to: number,
  fn: (state: EditorState) => TransactionSpec,
): { doc: string; from: number; to: number } {
  const state = EditorState.create({
    doc,
    selection: EditorSelection.single(from, to),
  });
  const newState = state.update(fn(state)).state;
  const sel = newState.selection.main;
  return { doc: newState.doc.toString(), from: sel.from, to: sel.to };
}

describe("toggleBold", () => {
  it("no selection: inserts markers and places cursor between them", () => {
    const r = apply("abc", 1, 1, toggleBold);
    expect(r.doc).toBe("a****bc");
    expect(r.from).toBe(3);
    expect(r.to).toBe(3);
  });

  it("with selection: wraps text in markers", () => {
    const r = apply("hello", 0, 5, toggleBold);
    expect(r.doc).toBe("**hello**");
    expect(r.from).toBe(2);
    expect(r.to).toBe(7);
  });

  it("already wrapped: removes markers", () => {
    const r = apply("**hello**", 2, 7, toggleBold);
    expect(r.doc).toBe("hello");
    expect(r.from).toBe(0);
    expect(r.to).toBe(5);
  });

  it("selection includes markers: strips from inside", () => {
    const r = apply("**hello**", 0, 9, toggleBold);
    expect(r.doc).toBe("hello");
  });

  it("selection is the content of a bold span: strips the surrounding markers", () => {
    // "a**b**c" with "b" selected at (3,4) — the ** do wrap this selection, so bold is removed
    const r = apply("a**b**c", 3, 4, toggleBold);
    expect(r.doc).toBe("abc");
  });

  it("full bold span selected: strips markers via inner-content priority", () => {
    // selecting "**hello**" (0,9) — selected text itself starts/ends with marker
    const r = apply("**hello**", 0, 9, toggleBold);
    expect(r.doc).toBe("hello");
  });

  it("empty doc: inserts markers at start", () => {
    const r = apply("", 0, 0, toggleBold);
    expect(r.doc).toBe("****");
    expect(r.from).toBe(2);
  });

  it("reversed selection (head < anchor): wraps correctly", () => {
    const state = EditorState.create({
      doc: "hello",
      selection: EditorSelection.single(5, 0),
    });
    const newState = state.update(toggleBold(state)).state;
    expect(newState.doc.toString()).toBe("**hello**");
  });
});

describe("toggleItalic", () => {
  it("no selection: inserts markers and places cursor between them", () => {
    const r = apply("abc", 0, 0, toggleItalic);
    expect(r.doc).toBe("**abc");
    expect(r.from).toBe(1);
    expect(r.to).toBe(1);
  });

  it("with selection: wraps text in markers", () => {
    const r = apply("hi", 0, 2, toggleItalic);
    expect(r.doc).toBe("*hi*");
  });

  it("already wrapped: removes markers", () => {
    const r = apply("*hi*", 1, 3, toggleItalic);
    expect(r.doc).toBe("hi");
  });
});

describe("toggleStrike", () => {
  it("with selection: wraps text in markers", () => {
    const r = apply("text", 0, 4, toggleStrike);
    expect(r.doc).toBe("~~text~~");
  });

  it("already wrapped: removes markers", () => {
    const r = apply("~~text~~", 2, 6, toggleStrike);
    expect(r.doc).toBe("text");
  });
});

describe("toggleHeading", () => {
  it("H1: adds prefix and moves cursor past it", () => {
    const r = apply("hello", 0, 0, toggleHeading(1));
    expect(r.doc).toBe("# hello");
    expect(r.from).toBe(2);
    expect(r.to).toBe(2);
  });

  it("H1: removes prefix when same level applied again", () => {
    const r = apply("# hello", 0, 0, toggleHeading(1));
    expect(r.doc).toBe("hello");
  });

  it("H2: replaces H1 prefix with H2", () => {
    const r = apply("# hello", 0, 0, toggleHeading(2));
    expect(r.doc).toBe("## hello");
  });

  it("H3: replaces H2 prefix with H3", () => {
    const r = apply("## hello", 0, 0, toggleHeading(3));
    expect(r.doc).toBe("### hello");
  });

  it("H2: replaces H3 prefix with H2", () => {
    const r = apply("### hello", 0, 0, toggleHeading(2));
    expect(r.doc).toBe("## hello");
  });

  it("H1: applies to all selected lines", () => {
    const r = apply("a\nb", 0, 3, toggleHeading(1));
    expect(r.doc).toBe("# a\n# b");
  });

  it("empty doc: inserts prefix", () => {
    const r = apply("", 0, 0, toggleHeading(1));
    expect(r.doc).toBe("# ");
  });
});

describe("toggleBulletList", () => {
  it("single line: adds prefix and moves cursor past it", () => {
    const r = apply("hello", 0, 0, toggleBulletList);
    expect(r.doc).toBe("- hello");
    expect(r.from).toBe(2);
    expect(r.to).toBe(2);
  });

  it("single line: removes prefix on toggle", () => {
    const r = apply("- hello", 0, 0, toggleBulletList);
    expect(r.doc).toBe("hello");
  });

  it("multiple lines without prefix: adds prefix to all", () => {
    const r = apply("a\nb", 0, 3, toggleBulletList);
    expect(r.doc).toBe("- a\n- b");
  });

  it("multiple lines all prefixed: removes prefix from all", () => {
    const r = apply("- a\n- b", 0, 7, toggleBulletList);
    expect(r.doc).toBe("a\nb");
  });

  it("multiple lines mixed: adds prefix to lines missing it", () => {
    const r = apply("- a\nb", 0, 5, toggleBulletList);
    expect(r.doc).toBe("- a\n- b");
  });

  it("mixed with trailing unprefixed line: adds prefix to all (all-or-nothing)", () => {
    // "- a\n- b\nc" — not all have prefix → adds to the missing one
    const r = apply("- a\n- b\nc", 0, 9, toggleBulletList);
    expect(r.doc).toBe("- a\n- b\n- c");
  });
});

describe("toggleCheckList", () => {
  it("adds checklist prefix", () => {
    const r = apply("task", 0, 0, toggleCheckList);
    expect(r.doc).toBe("- [ ] task");
  });

  it("removes checklist prefix", () => {
    const r = apply("- [ ] task", 0, 0, toggleCheckList);
    expect(r.doc).toBe("task");
  });
});

describe("toggleQuote", () => {
  it("adds quote prefix", () => {
    const r = apply("text", 0, 0, toggleQuote);
    expect(r.doc).toBe("> text");
  });

  it("removes quote prefix", () => {
    const r = apply("> text", 0, 0, toggleQuote);
    expect(r.doc).toBe("text");
  });

  it("multiple lines without prefix: adds prefix to all", () => {
    const r = apply("a\nb", 0, 3, toggleQuote);
    expect(r.doc).toBe("> a\n> b");
  });

  it("quote+bullet nested line: strips only the quote prefix, keeps bullet", () => {
    const r = apply("> - foo", 0, 0, toggleQuote);
    expect(r.doc).toBe("- foo");
  });
});

describe("toggleOrderedList", () => {
  it("single line: adds numbered prefix", () => {
    const r = apply("hello", 0, 0, toggleOrderedList);
    expect(r.doc).toBe("1. hello");
  });

  it("single line: removes numbered prefix", () => {
    const r = apply("1. hello", 0, 0, toggleOrderedList);
    expect(r.doc).toBe("hello");
  });

  it("multiple lines: adds sequential numbers", () => {
    const r = apply("a\nb\nc", 0, 5, toggleOrderedList);
    expect(r.doc).toBe("1. a\n2. b\n3. c");
  });

  it("multiple lines all numbered: removes all prefixes", () => {
    const r = apply("1. a\n2. b", 0, 9, toggleOrderedList);
    expect(r.doc).toBe("a\nb");
  });

  it("multiple lines all numbered: removes even when numbers are out of order", () => {
    const r = apply("3. a\n1. b", 0, 9, toggleOrderedList);
    expect(r.doc).toBe("a\nb");
  });

  it("round-trip: add then remove restores original", () => {
    const doc = "a\nb\nc";
    const after = apply(doc, 0, doc.length, toggleOrderedList);
    expect(after.doc).toBe("1. a\n2. b\n3. c");
    const restored = apply(after.doc, 0, after.doc.length, toggleOrderedList);
    expect(restored.doc).toBe(doc);
  });
});

describe("toggleCode", () => {
  it("no selection: inserts empty fence block and places cursor inside", () => {
    const r = apply("abc", 1, 1, toggleCode);
    expect(r.doc).toBe("a```\n\n```bc");
    expect(r.from).toBe(5);
    expect(r.to).toBe(5);
  });

  it("single-line selection at line start: wraps in fence block", () => {
    const r = apply("hello", 0, 5, toggleCode);
    expect(r.doc).toBe("```\nhello\n```");
  });

  it("multi-line selection: wraps in fence block", () => {
    const r = apply("a\nb", 0, 3, toggleCode);
    expect(r.doc).toBe("```\na\nb\n```");
  });

  it("selection not at line start: prepends newline so fence is at column 0", () => {
    // selecting "b" in "abc" at pos 1..2 — opening fence must be on its own line
    const r = apply("abc", 1, 2, toggleCode);
    expect(r.doc).toBe("a\n```\nb\n```\nc");
  });
});

describe("toggleInlineCode", () => {
  it("no selection: inserts backtick pair and places cursor between them", () => {
    const r = apply("abc", 1, 1, toggleInlineCode);
    expect(r.doc).toBe("a``bc");
    expect(r.from).toBe(2);
    expect(r.to).toBe(2);
  });

  it("with selection: wraps text in backticks", () => {
    const r = apply("hello", 0, 5, toggleInlineCode);
    expect(r.doc).toBe("`hello`");
  });

  it("already wrapped: removes backticks", () => {
    const r = apply("`hello`", 1, 6, toggleInlineCode);
    expect(r.doc).toBe("hello");
  });
});

describe("insertLink", () => {
  it("no selection: inserts [](url) with url selected", () => {
    const r = apply("abc", 1, 1, insertLink);
    expect(r.doc).toBe("a[](url)bc");
    expect(r.from).toBe(4);
    expect(r.to).toBe(7);
  });

  it("with selection: converts to [text](url) with url selected", () => {
    const r = apply("abc", 0, 3, insertLink);
    expect(r.doc).toBe("[abc](url)");
    expect(r.from).toBe(6);
    expect(r.to).toBe(9);
  });
});

describe("insertDivider", () => {
  it("inserts --- after the current line", () => {
    const r = apply("abc", 1, 1, insertDivider);
    expect(r.doc).toBe("abc\n\n---\n");
  });

  it("in a multi-line doc, the original newline becomes a blank line after ---", () => {
    const r = apply("abc\ndef", 1, 1, insertDivider);
    expect(r.doc).toBe("abc\n\n---\n\ndef");
  });

  it("at doc end: appends divider", () => {
    const r = apply("abc", 3, 3, insertDivider);
    expect(r.doc).toBe("abc\n\n---\n");
  });

  it("cursor mid-line: anchors to end of that line, not cursor column", () => {
    const r = apply("a\nbc\nd", 3, 3, insertDivider);
    expect(r.doc).toBe("a\nbc\n\n---\n\nd");
  });
});
