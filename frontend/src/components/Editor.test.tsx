import { createRef } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, act, cleanup } from "@testing-library/react";
import type { EditorView } from "@codemirror/view";

import { Editor } from "./Editor";
import type { EditorHandle } from "./Editor";

function renderEditor(value: string, options: { readOnly?: boolean } = {}) {
  const onChange = vi.fn();
  const ref = createRef<EditorHandle>();
  const { rerender } = render(
    <Editor
      ref={ref}
      value={value}
      onChange={onChange}
      readOnly={options.readOnly}
    />,
  );
  const view = ref.current!.getView()!;
  return {
    view,
    onChange,
    doc: () => view.state.doc.toString(),
    setValue: (next: string) =>
      rerender(
        <Editor
          ref={ref}
          value={next}
          onChange={onChange}
          readOnly={options.readOnly}
        />,
      ),
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
    const { view, doc, setValue } = renderEditor("ab");

    typeAtEnd(view, "cd");
    setValue("remote text");

    expect(doc()).toBe("remote text");
    settleTypingLatch();
    expect(doc()).toBe("remote text");
  });

  it("does not report a value that did not come from the editor", () => {
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

describe("Editor initial caret", () => {
  afterEach(() => {
    cleanup();
  });

  it("places the caret after a blank heading", () => {
    const { view } = renderEditor("# ");
    expect(view.state.doc.toString()).toBe("# ");
    expect(view.state.selection.main.anchor).toBe(2);
    expect(view.state.selection.main.head).toBe(2);
    expect(view.hasFocus).toBe(true);
  });

  it("leaves the caret at the start of any other document", () => {
    const { view } = renderEditor("# hello");
    expect(view.state.selection.main.anchor).toBe(0);
    expect(view.state.selection.main.head).toBe(0);
  });

  it("leaves the caret at the start of a read-only blank heading", () => {
    const { view } = renderEditor("# ", { readOnly: true });
    expect(view.state.selection.main.anchor).toBe(0);
    expect(view.state.selection.main.head).toBe(0);
    expect(view.hasFocus).toBe(false);
  });
});
