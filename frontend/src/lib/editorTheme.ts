// CodeMirror 6 theme for GophDrive's "Quiet" redesign.
//
// Uses CSS variables from globals.css so a single theme covers both light and
// dark modes automatically — CodeMirror re-reads the variables on every paint,
// so a `data-theme` flip on <html> is picked up without re-mounting the editor.
//
// Extensions exported from this module (combined by `quietEditorExtensions`):
//   - EditorView.theme  — colors, typography, 660px centered column
//   - HighlightStyle    — markdown token colors (see `quietHighlightStyle`)
//   - ViewPlugin        — wikilink + tag mark decorations (`primaryAccentPlugin`)
//   - keymap            — Mod-B / Mod-I / Mod-Shift-x → toggleBold/Italic/Strike
//
// The regexes for wikilink and tag matching are exported as pure constants so
// they can be unit-tested without spinning up a CodeMirror view.

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

// ---------------------------------------------------------------------------
// Theme (colors + typography)
// ---------------------------------------------------------------------------

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
  // Neutralize any residual active-line / gutter background from defaults.
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

// ---------------------------------------------------------------------------
// Syntax highlight (Lezer markdown tags)
// ---------------------------------------------------------------------------

/**
 * Highlight style for the markdown grammar. Only colors/weights — layout is
 * handled by `quietEditorTheme` above.
 */
export const quietHighlightStyle = HighlightStyle.define([
  // Headings: bold in the body color. Mark chars (`#`) are demoted via
  // `tags.processingInstruction`/`tags.meta` below.
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
  // Markdown control characters: `#`, `**`, `- [ ]`, `>`, list bullets, etc.
  {
    tag: [t.processingInstruction, t.punctuation, t.meta, t.contentSeparator],
    color: "var(--text-faint)",
  },
  { tag: t.quote, color: "var(--text-secondary)" },
  { tag: [t.link, t.url], color: "var(--primary)" },
  // Inline code and fenced code: secondary text color to distinguish from
  // surrounding prose (font is already monospace via the base theme).
  {
    tag: t.monospace,
    color: "var(--text-secondary)",
  },
]);

// ---------------------------------------------------------------------------
// wikilink + tag decorations
// ---------------------------------------------------------------------------

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

/**
 * Lezer node names that mark code content in the markdown grammar (fenced
 * code, indented code blocks, and inline code, plus their inner text/mark
 * nodes). `resolveInner` walks up through these when a match position falls
 * inside one, so wikilink/tag decoration can be skipped there — mirroring
 * the `<pre>`/`<code>` skip that `lib/linkifyWikilinks.ts` and
 * `lib/linkifyTags.ts` apply to rendered HTML.
 */
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
    // Skip the boundary character (match[1]) — decorate only from `#` onward.
    const boundaryLen = match[1].length;
    const start = from + boundaryLen;
    const end = start + 1 /* '#' */ + match[2].length;
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

// ---------------------------------------------------------------------------
// Keymap (⌘B / ⌘I / ⌘⇧X)
// ---------------------------------------------------------------------------

type MarkdownCmd = (state: EditorState) => TransactionSpec;

/**
 * Wraps a pure `markdownCommands` builder into a CodeMirror `Command`:
 * dispatches the resulting TransactionSpec and returns true so key dispatch
 * stops after the shortcut fires.
 */
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

// ---------------------------------------------------------------------------
// Bundled extension
// ---------------------------------------------------------------------------

/**
 * All Quiet-theme extensions in a single array, ready to spread into a
 * CodeMirror `extensions` prop. Order:
 *   1. base theme
 *   2. accent theme (needed for `.cm-quiet-accent`)
 *   3. syntax highlight
 *   4. accent plugin (wikilink + tag)
 *   5. keymap (⌘B / ⌘I / ⌘⇧X)
 */
export const quietEditorExtensions: Extension[] = [
  quietEditorTheme,
  accentTheme,
  syntaxHighlighting(quietHighlightStyle),
  primaryAccentPlugin,
  quietMarkdownKeymap,
];
