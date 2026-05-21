import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MobileMarkdownToolbar, BUTTONS } from "./MobileMarkdownToolbar";
import type { EditorHandle } from "./Editor";

function makeEditorRef(handle: EditorHandle | null = null) {
  return { current: handle };
}

describe("MobileMarkdownToolbar", () => {
  it("renders all buttons", () => {
    const ref = makeEditorRef();
    render(<MobileMarkdownToolbar editorRef={ref} />);
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(BUTTONS.length);
  });

  it("has role=toolbar and accessible label", () => {
    const ref = makeEditorRef();
    const { container } = render(<MobileMarkdownToolbar editorRef={ref} />);
    const toolbar = container.querySelector('[role="toolbar"]');
    expect(toolbar).toBeTruthy();
    expect(toolbar?.getAttribute("aria-label")).toBe("Markdown formatting");
  });

  it("each button has a unique aria-label", () => {
    const ref = makeEditorRef();
    render(<MobileMarkdownToolbar editorRef={ref} />);
    const labels = screen
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label"));
    const unique = new Set(labels);
    expect(unique.size).toBe(BUTTONS.length);
  });

  it("all buttons are disabled when readOnly=true", () => {
    const ref = makeEditorRef();
    render(<MobileMarkdownToolbar editorRef={ref} readOnly />);
    const buttons = screen.getAllByRole("button");
    buttons.forEach((b) => {
      expect((b as HTMLButtonElement).disabled).toBe(true);
    });
  });

  it("all buttons are enabled when readOnly=false", () => {
    const ref = makeEditorRef();
    render(<MobileMarkdownToolbar editorRef={ref} readOnly={false} />);
    const buttons = screen.getAllByRole("button");
    buttons.forEach((b) => {
      expect((b as HTMLButtonElement).disabled).toBe(false);
    });
  });

  it("clicking a button calls editorRef.current.runCommand with the command", () => {
    const runCommand = vi.fn();
    const focus = vi.fn();
    const ref = makeEditorRef({ runCommand, focus });
    render(<MobileMarkdownToolbar editorRef={ref} />);

    const boldButton = screen.getByRole("button", { name: "Bold" });
    fireEvent.click(boldButton);

    expect(runCommand).toHaveBeenCalledTimes(1);
    expect(focus).toHaveBeenCalledTimes(1);
    // the argument must be a function (the toggleBold command)
    expect(typeof runCommand.mock.calls[0][0]).toBe("function");
  });

  it("clicking a button does nothing when editorRef.current is null", () => {
    const ref = makeEditorRef(null);
    render(<MobileMarkdownToolbar editorRef={ref} />);
    // should not throw
    fireEvent.click(screen.getByRole("button", { name: "Bold" }));
  });

  it("root element carries the mobile-md-toolbar class (CSS controls visibility)", () => {
    const ref = makeEditorRef();
    const { container } = render(<MobileMarkdownToolbar editorRef={ref} />);
    const toolbar = container.firstChild as HTMLElement;
    expect(toolbar.classList.contains("mobile-md-toolbar")).toBe(true);
  });
});
