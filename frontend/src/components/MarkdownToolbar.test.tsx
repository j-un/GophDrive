import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { EditorState } from "@codemirror/state";
import type { TransactionSpec } from "@codemirror/state";
import { MarkdownToolbar, BUTTONS, MOBILE_BUTTONS } from "./MarkdownToolbar";
import type { EditorHandle, SelectionInfo } from "./Editor";
import type { EditorView } from "@codemirror/view";

const FAKE_COORDS = { top: 100, bottom: 120, left: 50, right: 60 };

function makeSelection(empty: boolean): SelectionInfo {
  return { from: 2, to: empty ? 2 : 8, empty };
}

/** A minimal `getView()` fake: only `coordsAtPos` (used for positioning) and
 * `state` (used by the Heading cycle test) are exercised by the toolbar. */
function makeView(stateText = "hello world"): EditorView {
  return {
    coordsAtPos: () => ({ ...FAKE_COORDS }),
    state: EditorState.create({ doc: stateText, selection: { anchor: 0 } }),
  } as unknown as EditorView;
}

function makeEditorRef(handle: Partial<EditorHandle> | null = null) {
  return { current: handle as EditorHandle | null };
}

// The component always renders two toolbars simultaneously (a mobile
// persistent bar and, conditionally, a desktop floating bar) — CSS media
// queries decide which one actually paints at a given viewport width, but
// both exist in the DOM regardless. These helpers disambiguate by their
// distinct accessible names rather than relying on CSS visibility (which
// isn't computed in jsdom anyway).
function queryFloatingToolbar() {
  return screen.queryByRole("toolbar", { name: "Markdown formatting" });
}

function getStaticToolbar() {
  return screen.getByRole("toolbar", {
    name: "Markdown formatting (mobile)",
  });
}

describe("MarkdownToolbar — desktop floating bar (selection-triggered)", () => {
  it("renders nothing when there is no selection", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} />);
    expect(queryFloatingToolbar()).toBeNull();
  });

  it("renders nothing when the selection is empty (cursor only)", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={makeSelection(true)} />);
    expect(queryFloatingToolbar()).toBeNull();
  });

  it("renders when there is a non-empty selection", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );
    expect(queryFloatingToolbar()).toBeTruthy();
  });

  it("renders exactly the BUTTONS configuration (7 buttons)", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );
    expect(BUTTONS).toHaveLength(7);
    expect(BUTTONS.map((b) => b.label)).toEqual([
      "Bold",
      "Italic",
      "Strikethrough",
      "Heading",
      "Quote",
      "Code block",
      "Link",
    ]);
    const toolbar = queryFloatingToolbar()!;
    expect(within(toolbar).getAllByRole("button")).toHaveLength(BUTTONS.length);
  });

  it("each button has a unique aria-label", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );
    const toolbar = queryFloatingToolbar()!;
    const labels = within(toolbar)
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label"));
    expect(new Set(labels).size).toBe(BUTTONS.length);
  });

  it("is hidden when readOnly=true even with a non-empty selection", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(
      <MarkdownToolbar
        editorRef={ref}
        selection={makeSelection(false)}
        readOnly
      />,
    );
    expect(queryFloatingToolbar()).toBeNull();
  });

  it("clicking a button calls editorRef.current.runCommand and refocuses the editor", () => {
    const runCommand = vi.fn();
    const focus = vi.fn();
    const ref = makeEditorRef({ runCommand, focus, getView: () => makeView() });
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );

    const toolbar = queryFloatingToolbar()!;
    const boldButton = within(toolbar).getByRole("button", { name: "Bold" });
    fireEvent.click(boldButton);

    expect(runCommand).toHaveBeenCalledTimes(1);
    expect(focus).toHaveBeenCalledTimes(1);
    // the argument must be a function (the toggleBold command)
    expect(typeof runCommand.mock.calls[0][0]).toBe("function");
  });

  it("does not render (and getView() is guarded) when editorRef.current is null", () => {
    const ref = makeEditorRef(null);
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );
    // No view to position against, so the toolbar stays hidden rather than
    // rendering at a default/garbage position.
    expect(queryFloatingToolbar()).toBeNull();
  });

  it("does not render when coordsAtPos() returns null", () => {
    const ref = makeEditorRef({
      getView: () =>
        ({
          coordsAtPos: () => null,
          state: EditorState.create({ doc: "" }),
        }) as unknown as EditorView,
    });
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );
    expect(queryFloatingToolbar()).toBeNull();
  });

  it("pointerdown on a button prevents default so the editor keeps focus", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(
      <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
    );
    const toolbar = queryFloatingToolbar()!;
    const boldButton = within(toolbar).getByRole("button", { name: "Bold" });
    // fireEvent returns false when the event was cancelled (preventDefault called).
    const notCancelled = fireEvent.pointerDown(boldButton);
    expect(notCancelled).toBe(false);
  });

  describe("Heading cycle", () => {
    function runHeadingCommand(docText: string) {
      const runCommand = vi.fn();
      const view = {
        coordsAtPos: () => ({ ...FAKE_COORDS }),
        state: EditorState.create({
          doc: docText,
          selection: { anchor: 0 },
        }),
      } as unknown as EditorView;
      const ref = makeEditorRef({
        runCommand,
        focus: vi.fn(),
        getView: () => view,
      });
      render(
        <MarkdownToolbar editorRef={ref} selection={makeSelection(false)} />,
      );
      const toolbar = queryFloatingToolbar()!;
      fireEvent.click(within(toolbar).getByRole("button", { name: "Heading" }));
      const command = runCommand.mock.calls[0][0] as (
        state: EditorState,
      ) => TransactionSpec;
      const tr = view.state.update(command(view.state));
      return tr.state.doc.toString();
    }

    it("plain paragraph -> H1", () => {
      expect(runHeadingCommand("Title")).toBe("# Title");
    });

    it("H1 -> H2", () => {
      expect(runHeadingCommand("# Title")).toBe("## Title");
    });

    it("H2 -> H3", () => {
      expect(runHeadingCommand("## Title")).toBe("### Title");
    });

    it("H3 -> plain (cycle resets)", () => {
      expect(runHeadingCommand("### Title")).toBe("Title");
    });
  });
});

describe("MarkdownToolbar — mobile persistent bar (always visible)", () => {
  it("renders exactly the MOBILE_BUTTONS configuration (14 buttons), even with no selection", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} />);

    expect(MOBILE_BUTTONS).toHaveLength(14);
    expect(MOBILE_BUTTONS.map((b) => b.label)).toEqual([
      "Heading 1",
      "Heading 2",
      "Heading 3",
      "Bold",
      "Italic",
      "Strikethrough",
      "Bullet list",
      "Numbered list",
      "Checklist",
      "Quote",
      "Code block",
      "Inline code",
      "Link",
      "Divider",
    ]);

    const toolbar = getStaticToolbar();
    expect(within(toolbar).getAllByRole("button")).toHaveLength(
      MOBILE_BUTTONS.length,
    );
  });

  it("stays rendered when there is no selection (unlike the floating bar)", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} />);
    expect(getStaticToolbar()).toBeTruthy();
  });

  it("each button has a unique aria-label", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} />);
    const toolbar = getStaticToolbar();
    const labels = within(toolbar)
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label"));
    expect(new Set(labels).size).toBe(MOBILE_BUTTONS.length);
  });

  it("stays rendered but disables its buttons when readOnly=true", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} readOnly />);
    const toolbar = getStaticToolbar();
    within(toolbar)
      .getAllByRole("button")
      .forEach((b) => {
        expect((b as HTMLButtonElement).disabled).toBe(true);
      });
  });

  it("enables its buttons when readOnly=false", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(
      <MarkdownToolbar editorRef={ref} selection={null} readOnly={false} />,
    );
    const toolbar = getStaticToolbar();
    within(toolbar)
      .getAllByRole("button")
      .forEach((b) => {
        expect((b as HTMLButtonElement).disabled).toBe(false);
      });
  });

  it("clicking a button calls editorRef.current.runCommand and refocuses the editor", () => {
    const runCommand = vi.fn();
    const focus = vi.fn();
    const ref = makeEditorRef({ runCommand, focus, getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} />);

    const toolbar = getStaticToolbar();
    const boldButton = within(toolbar).getByRole("button", { name: "Bold" });
    fireEvent.click(boldButton);

    expect(runCommand).toHaveBeenCalledTimes(1);
    expect(focus).toHaveBeenCalledTimes(1);
    expect(typeof runCommand.mock.calls[0][0]).toBe("function");
  });

  it("clicking a button does nothing when editorRef.current is null", () => {
    const ref = makeEditorRef(null);
    render(<MarkdownToolbar editorRef={ref} selection={null} />);
    const toolbar = getStaticToolbar();
    // should not throw
    fireEvent.click(within(toolbar).getByRole("button", { name: "Bold" }));
  });

  it("marks itself as fading via data-zen when zen=true", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} zen />);
    expect(getStaticToolbar().getAttribute("data-zen")).toBe("true");
  });

  it("does not mark itself as fading when zen=false (default)", () => {
    const ref = makeEditorRef({ getView: () => makeView() });
    render(<MarkdownToolbar editorRef={ref} selection={null} />);
    expect(getStaticToolbar().getAttribute("data-zen")).toBe("false");
  });
});
