// Pure CodeMirror 6 command functions. Side effects (dispatch, focus) belong to the caller.
import { EditorState, EditorSelection } from "@codemirror/state";
import type { TransactionSpec } from "@codemirror/state";

// Selected text, when inserted, so the user can immediately type or paste the real URL.
const URL_PLACEHOLDER = "url";

/** Line numbers (1-based) covered by the current selection, excluding a bare line-start at the end. */
function getSelectedLineNumbers(state: EditorState): number[] {
  const nums = new Set<number>();
  for (const range of state.selection.ranges) {
    const fromLineNum = state.doc.lineAt(range.from).number;
    const toLine = state.doc.lineAt(range.to);
    // Exclude the next line when range.to sits exactly on its beginning
    const toLineNum =
      range.to > range.from && toLine.from === range.to
        ? toLine.number - 1
        : toLine.number;
    for (let i = fromLineNum; i <= toLineNum; i++) nums.add(i);
  }
  return Array.from(nums).sort((a, b) => a - b);
}

/** Wrap/unwrap the current selection with a symmetric marker (e.g. `**`, `*`, `~~`, `` ` ``). */
function wrapSelection(state: EditorState, marker: string): TransactionSpec {
  const { from, to } = state.selection.main;
  const mLen = marker.length;

  if (from === to) {
    return {
      changes: { from, to, insert: marker + marker },
      selection: EditorSelection.cursor(from + mLen),
    };
  }

  // Priority 1: the selected text itself is wrapped — strip markers from inside
  const selected = state.doc.sliceString(from, to);
  if (
    selected.startsWith(marker) &&
    selected.endsWith(marker) &&
    selected.length >= 2 * mLen
  ) {
    return {
      changes: [
        { from, to: from + mLen, insert: "" },
        { from: to - mLen, to, insert: "" },
      ],
      selection: EditorSelection.single(from, to - 2 * mLen),
    };
  }

  // Priority 2: markers immediately surround the selection from outside
  const beforeStart = Math.max(0, from - mLen);
  const afterEnd = Math.min(state.doc.length, to + mLen);
  const before = state.doc.sliceString(beforeStart, from);
  const after = state.doc.sliceString(to, afterEnd);

  if (before === marker && after === marker) {
    return {
      changes: [
        { from: beforeStart, to: from, insert: "" },
        { from: to, to: afterEnd, insert: "" },
      ],
      selection: EditorSelection.single(beforeStart, to - mLen),
    };
  }

  return {
    changes: [
      { from, to: from, insert: marker },
      { from: to, to, insert: marker },
    ],
    selection: EditorSelection.single(from + mLen, to + mLen),
  };
}

/** Add/remove a line-start prefix on all selected lines (Notion-style: all-have → remove, else add). */
function togglePrefixOnLines(
  state: EditorState,
  prefix: string,
): TransactionSpec {
  const lineNums = getSelectedLineNumbers(state);
  const lines = lineNums.map((n) => state.doc.line(n));
  const allHave = lines.every((l) => l.text.startsWith(prefix));
  const changes = lines.flatMap((line) => {
    if (allHave) {
      return [{ from: line.from, to: line.from + prefix.length, insert: "" }];
    } else if (!line.text.startsWith(prefix)) {
      return [{ from: line.from, to: line.from, insert: prefix }];
    }
    return [];
  });
  const changeSet = state.changes(changes);
  return { changes: changeSet, selection: state.selection.map(changeSet, 1) };
}

/** Toggle `#`/`##`/`###` heading prefix. Same level removes; different level replaces. */
export function toggleHeading(
  level: 1 | 2 | 3,
): (state: EditorState) => TransactionSpec {
  return (state) => {
    const target = "#".repeat(level) + " ";
    const lineNums = getSelectedLineNumbers(state);
    const changes = lineNums.map((n) => {
      const line = state.doc.line(n);
      const m = line.text.match(/^(#{1,6}) /);
      if (m) {
        const existing = m[0];
        if (m[1] === "#".repeat(level)) {
          return {
            from: line.from,
            to: line.from + existing.length,
            insert: "",
          };
        }
        return {
          from: line.from,
          to: line.from + existing.length,
          insert: target,
        };
      }
      return { from: line.from, to: line.from, insert: target };
    });
    const changeSet = state.changes(changes);
    return { changes: changeSet, selection: state.selection.map(changeSet, 1) };
  };
}

/** Toggle `**bold**`. */
export function toggleBold(state: EditorState): TransactionSpec {
  return wrapSelection(state, "**");
}

/** Toggle `*italic*`. */
export function toggleItalic(state: EditorState): TransactionSpec {
  return wrapSelection(state, "*");
}

/** Toggle `~~strikethrough~~`. */
export function toggleStrike(state: EditorState): TransactionSpec {
  return wrapSelection(state, "~~");
}

/** Toggle `- ` bullet list prefix on selected lines. */
export function toggleBulletList(state: EditorState): TransactionSpec {
  return togglePrefixOnLines(state, "- ");
}

/** Toggle `- [ ] ` checklist prefix on selected lines. */
export function toggleCheckList(state: EditorState): TransactionSpec {
  return togglePrefixOnLines(state, "- [ ] ");
}

/** Toggle `> ` blockquote prefix on selected lines. */
export function toggleQuote(state: EditorState): TransactionSpec {
  return togglePrefixOnLines(state, "> ");
}

/** Toggle sequential numbered list. All-numbered → strip; else add 1. 2. 3. */
export function toggleOrderedList(state: EditorState): TransactionSpec {
  const lineNums = getSelectedLineNumbers(state);
  const lines = lineNums.map((n) => state.doc.line(n));
  const allHave = lines.every((l) => /^\d+\.\s/.test(l.text));
  let counter = 1;
  const changes = lines.flatMap((line) => {
    if (allHave) {
      const m = line.text.match(/^(\d+\.\s)/);
      if (!m) return [];
      return [{ from: line.from, to: line.from + m[1].length, insert: "" }];
    }
    const prefix = `${counter++}. `;
    const m = line.text.match(/^(\d+\.\s)/);
    if (m) {
      return [{ from: line.from, to: line.from + m[1].length, insert: prefix }];
    }
    return [{ from: line.from, to: line.from, insert: prefix }];
  });
  const changeSet = state.changes(changes);
  return { changes: changeSet, selection: state.selection.map(changeSet, 1) };
}

/**
 * Wrap selection in a fenced code block.
 * No selection: inserts an empty fence and places cursor on the blank line inside.
 * When the selection does not start at the beginning of a line, a leading newline is prepended
 * so the opening fence stays at column 0 (required for Markdown renderers).
 */
export function toggleCode(state: EditorState): TransactionSpec {
  const { from, to } = state.selection.main;

  if (from === to) {
    const insert = "```\n\n```";
    return {
      changes: { from, to, insert },
      selection: EditorSelection.cursor(from + 4),
    };
  }

  const fromLine = state.doc.lineAt(from);
  const toLine = state.doc.lineAt(to);
  const openFence = from === fromLine.from ? "```\n" : "\n```\n";
  const closeFence = to === toLine.to ? "\n```" : "\n```\n";

  return {
    changes: [
      { from, to: from, insert: openFence },
      { from: to, to, insert: closeFence },
    ],
  };
}

/** Toggle `` `inline code` ``. */
export function toggleInlineCode(state: EditorState): TransactionSpec {
  return wrapSelection(state, "`");
}

/** Insert `[text](url)` with the URL placeholder selected for immediate overwrite. */
export function insertLink(state: EditorState): TransactionSpec {
  const { from, to } = state.selection.main;
  const selectedText = state.doc.sliceString(from, to);

  if (from === to) {
    const urlFrom = from + 3;
    return {
      changes: { from, to, insert: `[](${URL_PLACEHOLDER})` },
      selection: EditorSelection.single(
        urlFrom,
        urlFrom + URL_PLACEHOLDER.length,
      ),
    };
  }

  const urlFrom = from + 1 + selectedText.length + 2;
  return {
    changes: { from, to, insert: `[${selectedText}](${URL_PLACEHOLDER})` },
    selection: EditorSelection.single(
      urlFrom,
      urlFrom + URL_PLACEHOLDER.length,
    ),
  };
}

/**
 * Insert `\n\n---\n` after the current line.
 * The anchor is the end of the current line regardless of cursor column.
 */
export function insertDivider(state: EditorState): TransactionSpec {
  const { head } = state.selection.main;
  const line = state.doc.lineAt(head);
  const insert = "\n\n---\n";
  return {
    changes: { from: line.to, to: line.to, insert },
    selection: EditorSelection.cursor(line.to + insert.length),
  };
}
