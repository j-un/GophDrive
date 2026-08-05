import { createRef } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, act, cleanup } from "@testing-library/react";
import type { EditorView } from "@codemirror/view";

import { Editor } from "./Editor";
import type { EditorHandle } from "./Editor";

/**
 * Mounts an Editor and hands back the pieces the tests drive it with:
 * `view` (the live CodeMirror view), `doc()` (observable document text),
 * `onChange` (the spy the component emits through) and `setValue()`, which
 * re-renders with a new `value` prop the way a parent's state update would.
 */
function renderEditor(value: string) {
  const onChange = vi.fn();
  const ref = createRef<EditorHandle>();
  const { rerender } = render(
    <Editor ref={ref} value={value} onChange={onChange} />,
  );
  const view = ref.current!.getView()!;
  return {
    view,
    onChange,
    doc: () => view.state.doc.toString(),
    setValue: (next: string) =>
      rerender(<Editor ref={ref} value={next} onChange={onChange} />),
  };
}

/**
 * Simulates a keystroke: CodeMirror updates its own document synchronously and
 * notifies `onChange` afterwards, so the React `value` prop always trails the
 * document by at least one commit.
 */
function typeAtEnd(view: EditorView, text: string) {
  act(() => {
    view.dispatch({ changes: { from: view.state.doc.length, insert: text } });
  });
}

/**
 * Runs past @uiw/react-codemirror's 200ms "is typing" latch, after which it
 * flushes any document update it deferred while the user was typing. The latch
 * is driven by a 1ms interval, so fake time has to actually run.
 */
function settleTypingLatch() {
  act(() => {
    vi.advanceTimersByTime(500);
  });
}

describe("Editor document sync", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("puts the initial value into the document", () => {
    const { doc } = renderEditor("hello");
    expect(doc()).toBe("hello");
  });

  it("keeps characters typed after a lagging value commit", () => {
    // The parent mirrors the document one commit late, so its commit for "abc"
    // can land only after the document has already moved on to "abcd".
    const { view, doc, onChange, setValue } = renderEditor("ab");

    typeAtEnd(view, "c");
    expect(onChange).toHaveBeenLastCalledWith("abc");
    typeAtEnd(view, "d");
    expect(onChange).toHaveBeenLastCalledWith("abcd");

    setValue("abc");
    setValue("abcd");
    settleTypingLatch();

    expect(doc()).toBe("abcd");
  });

  it("keeps the document when only the intermediate value is committed", () => {
    // Same race, except the parent never gets to deliver the catch-up value —
    // the stale "abc" commit is the last thing the component sees.
    const { view, doc, setValue } = renderEditor("ab");

    typeAtEnd(view, "c");
    typeAtEnd(view, "d");

    setValue("abc");
    settleTypingLatch();

    expect(doc()).toBe("abcd");
  });

  it("keeps the document when re-rendered with a value it held earlier", () => {
    const { view, doc, setValue } = renderEditor("ab");

    typeAtEnd(view, "cd");

    setValue("ab");
    settleTypingLatch();

    expect(doc()).toBe("abcd");
  });

  it("applies a value that did not come from the editor", () => {
    // A note load, or the conflict dialog's "Discard Local Changes", pushes
    // text the editor never emitted — that must overwrite the document.
    const { view, doc, setValue } = renderEditor("ab");

    typeAtEnd(view, "cd");
    setValue("remote text");

    expect(doc()).toBe("remote text");
    settleTypingLatch();
    expect(doc()).toBe("remote text");
  });

  it("does not report a value that did not come from the editor", () => {
    // The parent already holds the text it just pushed in, so writing it into
    // the document must not come back out as a fresh edit.
    const { view, doc, onChange, setValue } = renderEditor("ab");

    typeAtEnd(view, "cd");
    expect(onChange).toHaveBeenLastCalledWith("abcd");
    const callsWhileTyping = onChange.mock.calls.length;

    setValue("remote text");

    expect(doc()).toBe("remote text");
    expect(onChange).toHaveBeenCalledTimes(callsWhileTyping);

    settleTypingLatch();
    expect(onChange).toHaveBeenCalledTimes(callsWhileTyping);
  });

  it("emits the whole document text through onChange while typing", () => {
    const { view, onChange } = renderEditor("ab");

    typeAtEnd(view, "c");
    expect(onChange).toHaveBeenLastCalledWith("abc");

    typeAtEnd(view, "d");
    expect(onChange).toHaveBeenLastCalledWith("abcd");
  });
});
