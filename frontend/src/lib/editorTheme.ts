import {
  HighlightStyle,
  syntaxHighlighting,
  syntaxTree,
} from "@codemirror/language";
import { RangeSet } from "@codemirror/state";
import type {
  EditorState,
  Extension,
  TransactionSpec,
} from "@codemirror/state";
import { tags as t } from "@lezer/highlight";
import type { SyntaxNode } from "@lezer/common";
import {
  Decoration,
  EditorView,
  MatchDecorator,
  ViewPlugin,
  keymap,
} from "@codemirror/view";
import type { DecorationSet, ViewUpdate } from "@codemirror/view";

import { toggleBold, toggleItalic, toggleStrike } from "./markdownCommands";

/**
 * Base editor theme. Depends only on CSS variables — swaps light/dark via
 * `data-theme` on <html> without extension re-mounting.
 */
export const quietEditorTheme = EditorView.theme({
  "&": {
    backgroundColor: "var(--background)",
    color: "var(--foreground)",
    height: "100%",
  },
  ".cm-scroller": {
    fontFamily: "var(--font-mono)",
    backgroundColor: "var(--background)",
    lineHeight: "1.9",
  },
  ".cm-content": {
    maxWidth: "660px",
    marginLeft: "auto",
    marginRight: "auto",
    paddingTop: "48px",
    paddingBottom: "48px",
    fontFamily: "var(--font-mono)",
    fontSize: "16px",
    lineHeight: "1.9",
    caretColor: "var(--primary)",
    color: "var(--foreground)",
  },
  // drawSelection (enabled via basicSetup) replaces the native caret with a DOM
  // element — style the DOM caret via `.cm-cursor`, not `caret-color`.
  ".cm-cursor, .cm-dropCursor": {
    borderLeft: "2px solid var(--primary)",
    borderRight: "none",
    marginLeft: "-1px",
  },
  ".cm-selectionBackground, ::selection": {
    background: "var(--selection)",
  },
  "&.cm-focused .cm-selectionBackground, .cm-content ::selection": {
    background: "var(--selection)",
  },
  ".cm-activeLine": {
    backgroundColor: "transparent",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "transparent",
  },
  ".cm-gutters": {
    backgroundColor: "var(--background)",
    color: "var(--text-muted)",
    border: "none",
  },
  "&.cm-focused": {
    outline: "none",
  },
});

export const quietHighlightStyle = HighlightStyle.define([
  {
    tag: [
      t.heading,
      t.heading1,
      t.heading2,
      t.heading3,
      t.heading4,
      t.heading5,
      t.heading6,
    ],
    fontWeight: "700",
    color: "var(--foreground)",
  },
  { tag: t.strong, fontWeight: "700" },
  { tag: t.emphasis, fontStyle: "italic" },
  { tag: t.strikethrough, textDecoration: "line-through" },
  {
    tag: [t.processingInstruction, t.punctuation, t.meta, t.contentSeparator],
    color: "var(--text-faint)",
  },
  { tag: t.quote, color: "var(--text-secondary)" },
  { tag: [t.link, t.url], color: "var(--primary)" },
  {
    tag: t.monospace,
    color: "var(--text-secondary)",
  },
]);

/**
 * Matches `[[Title]]` — same shape as `lib/linkifyWikilinks.ts`. Title
 * excludes `[]`, newlines, `|`, and `#`. Global flag is required by
 * `MatchDecorator`.
 */
export const WIKILINK_RE = /\[\[([^[\]\n|#]+?)\]\]/g;

/**
 * Matches `#tag` inline. Same shape as `lib/linkifyTags.ts`:
 *   - a boundary char (whitespace / punctuation / line start / `>`) precedes
 *     `#`. `>` is included so a tag right after an HTML/blockquote close
 *     (or, in the source doc, after a `> ` quote marker) still matches — kept
 *     in sync with the boundary class in `lib/linkifyTags.ts`. The `\s`
 *     escape already matches the U+3000 ideographic space, so no literal
 *     full-width space is listed here (avoids eslint no-irregular-whitespace).
 *   - the tag body starts with a Unicode letter or `_`, followed by
 *     letters/digits/`_`/`-`/`/`
 * The boundary is captured (group 1) so callers know where the `#` starts.
 */
export const TAG_RE =
  /(^|[\s>、。，．！？「」『』【】（）[\]])#([\p{L}_][\p{L}\p{N}_\-/]*)/gmu;

const CODE_NODE_NAMES = new Set([
  "FencedCode",
  "CodeBlock",
  "InlineCode",
  "CodeText",
  "CodeMark",
]);

/** True when `pos` sits inside a fenced/indented/inline code Lezer node. */
export function isInCodeNode(state: EditorState, pos: number): boolean {
  let node: SyntaxNode | null = syntaxTree(state).resolveInner(pos, 1);
  while (node) {
    if (CODE_NODE_NAMES.has(node.name)) return true;
    node = node.parent;
  }
  return false;
}

const accentMark = Decoration.mark({ class: "cm-quiet-accent" });

const wikilinkDecorator = new MatchDecorator({
  regexp: WIKILINK_RE,
  decoration(_match, view, pos) {
    return isInCodeNode(view.state, pos) ? null : accentMark;
  },
});

const tagDecorator = new MatchDecorator({
  regexp: TAG_RE,
  decorate(add, from, _to, match, view) {
    const boundaryLen = match[1].length;
    const start = from + boundaryLen;
    const end = start + 1 + match[2].length;
    if (isInCodeNode(view.state, start)) return;
    add(start, end, accentMark);
  },
});

/**
 * ViewPlugin that colors `[[...]]` and `#tag` occurrences with `--primary`,
 * skipping matches inside fenced/inline/indented code (see `isInCodeNode`).
 * Holds two independent `DecorationSet`s (one per MatchDecorator) so each can
 * be updated incrementally via `updateDeco`.
 */
export const primaryAccentPlugin = ViewPlugin.fromClass(
  class {
    wiki: DecorationSet;
    tag: DecorationSet;
    decorations: DecorationSet;

    constructor(view: EditorView) {
      this.wiki = wikilinkDecorator.createDeco(view);
      this.tag = tagDecorator.createDeco(view);
      this.decorations = RangeSet.join([this.wiki, this.tag]);
    }

    update(update: ViewUpdate) {
      this.wiki = wikilinkDecorator.updateDeco(update, this.wiki);
      this.tag = tagDecorator.updateDeco(update, this.tag);
      this.decorations = RangeSet.join([this.wiki, this.tag]);
    }
  },
  { decorations: (v) => v.decorations },
);

const accentTheme = EditorView.theme({
  ".cm-quiet-accent": { color: "var(--primary)" },
});

type MarkdownCmd = (state: EditorState) => TransactionSpec;

function runMarkdownCmd(build: MarkdownCmd): (view: EditorView) => boolean {
  return (view) => {
    view.dispatch(build(view.state));
    return true;
  };
}

export const quietMarkdownKeymap = keymap.of([
  { key: "Mod-b", run: runMarkdownCmd(toggleBold), preventDefault: true },
  { key: "Mod-i", run: runMarkdownCmd(toggleItalic), preventDefault: true },
  {
    key: "Mod-Shift-x",
    run: runMarkdownCmd(toggleStrike),
    preventDefault: true,
  },
]);

export const quietEditorExtensions: Extension[] = [
  quietEditorTheme,
  accentTheme,
  syntaxHighlighting(quietHighlightStyle),
  primaryAccentPlugin,
  quietMarkdownKeymap,
];
