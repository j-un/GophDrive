/* eslint-disable react-refresh/only-export-components */
// TODO(aria-pressed): Derive active-marker state from CodeMirror ViewPlugin to reflect
// current selection formatting in button aria-pressed attributes (future enhancement).

import {
  Fragment,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import type { RefObject } from "react";
import {
  Bold,
  Code,
  Code2,
  Heading,
  Heading1,
  Heading2,
  Heading3,
  Italic,
  Link,
  List,
  ListChecks,
  ListOrdered,
  Minus,
  Quote,
  Strikethrough,
} from "lucide-react";
import type { EditorHandle, SelectionInfo } from "./Editor";
import type { EditorState } from "@codemirror/state";
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
} from "@/lib/markdownCommands";
import styles from "./MarkdownToolbar.module.css";

interface Props {
  editorRef: RefObject<EditorHandle | null>;
  selection: SelectionInfo | null;
  readOnly?: boolean;
  /**
   * Whether NotePage's zen mode is currently fading chrome out. Only affects
   * the mobile persistent bar below — the desktop floating bar already
   * disappears on its own once typing collapses the selection.
   */
  zen?: boolean;
}

type ButtonIcon = React.ComponentType<{
  size?: number;
  strokeWidth?: number;
  "aria-hidden"?: boolean | "true" | "false";
}>;

type ButtonSpec = {
  label: string;
  icon: ButtonIcon;
  command: (state: EditorState) => TransactionSpec;
};

/**
 * Cycles the heading level of the current line: none → H1 → H2 → H3 → none.
 * Reads the `#` count straight off the line the selection starts on and
 * delegates to `toggleHeading`, whose own same-level-removes/different-level-
 * replaces semantics does the actual strip/replace work.
 */
function cycleHeadingCommand(state: EditorState): TransactionSpec {
  const line = state.doc.lineAt(state.selection.main.from);
  const match = line.text.match(/^(#{1,6}) /);
  const currentLevel = match ? match[1].length : 0;
  const nextLevel = currentLevel === 0 ? 1 : currentLevel === 1 ? 2 : 3;
  return toggleHeading(nextLevel as 1 | 2 | 3)(state);
}

/** Desktop (≥769px) floating toolbar: shown only while there's a selection. */
export const BUTTONS: ReadonlyArray<ButtonSpec> = [
  { label: "Bold", icon: Bold, command: toggleBold },
  { label: "Italic", icon: Italic, command: toggleItalic },
  { label: "Strikethrough", icon: Strikethrough, command: toggleStrike },
  { label: "Heading", icon: Heading, command: cycleHeadingCommand },
  { label: "Quote", icon: Quote, command: toggleQuote },
  { label: "Code block", icon: Code, command: toggleCode },
  { label: "Link", icon: Link, command: insertLink },
];

// Index (into BUTTONS) after which the vertical separator is rendered.
const SEPARATOR_AFTER_INDEX = 2;

/**
 * Mobile (≤768px) persistent toolbar: the full pre-redesign button set,
 * always visible while editing. Selecting text to trigger a floating
 * toolbar is fiddly on touch (and competes with the OS's own selection
 * menu), so mobile keeps a static row instead of `BUTTONS` above.
 */
export const MOBILE_BUTTONS: ReadonlyArray<ButtonSpec> = [
  { label: "Heading 1", icon: Heading1, command: toggleHeading(1) },
  { label: "Heading 2", icon: Heading2, command: toggleHeading(2) },
  { label: "Heading 3", icon: Heading3, command: toggleHeading(3) },
  { label: "Bold", icon: Bold, command: toggleBold },
  { label: "Italic", icon: Italic, command: toggleItalic },
  { label: "Strikethrough", icon: Strikethrough, command: toggleStrike },
  { label: "Bullet list", icon: List, command: toggleBulletList },
  { label: "Numbered list", icon: ListOrdered, command: toggleOrderedList },
  { label: "Checklist", icon: ListChecks, command: toggleCheckList },
  { label: "Quote", icon: Quote, command: toggleQuote },
  { label: "Code block", icon: Code, command: toggleCode },
  { label: "Inline code", icon: Code2, command: toggleInlineCode },
  { label: "Link", icon: Link, command: insertLink },
  { label: "Divider", icon: Minus, command: insertDivider },
];

// Fallback size used for the very first position calculation, before the
// toolbar has mounted and `offsetWidth`/`offsetHeight` are available. Close
// to the real rendered size (7 buttons + 1 separator), so the toolbar never
// visibly jumps once the accurate measurement lands on the next recalc.
const FALLBACK_WIDTH = 246;
const FALLBACK_HEIGHT = 40;
const VIEWPORT_MARGIN = 8;
const GAP_ABOVE_SELECTION = 8;

export function MarkdownToolbar({
  editorRef,
  selection,
  readOnly = false,
  zen = false,
}: Props) {
  const toolbarRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);

  const visible = !readOnly && !!selection && !selection.empty;

  const recalc = useCallback(() => {
    const view = editorRef.current?.getView();
    if (!view || !selection) {
      setPos(null);
      return;
    }

    const fromCoords = view.coordsAtPos(selection.from);
    const toCoords = view.coordsAtPos(selection.to);
    if (!fromCoords || !toCoords) {
      setPos(null);
      return;
    }

    const top = Math.min(fromCoords.top, toCoords.top);
    const bottom = Math.max(fromCoords.bottom, toCoords.bottom);
    const left = Math.min(fromCoords.left, toCoords.left);
    const right = Math.max(fromCoords.right, toCoords.right);
    const centerX = (left + right) / 2;

    const width = toolbarRef.current?.offsetWidth ?? FALLBACK_WIDTH;
    const height = toolbarRef.current?.offsetHeight ?? FALLBACK_HEIGHT;

    let toolbarTop = top - GAP_ABOVE_SELECTION - height;
    if (toolbarTop < VIEWPORT_MARGIN) {
      // Not enough room above the selection — show below it instead.
      toolbarTop = bottom + GAP_ABOVE_SELECTION;
    }

    const maxLeft = Math.max(
      window.innerWidth - width - VIEWPORT_MARGIN,
      VIEWPORT_MARGIN,
    );
    const toolbarLeft = Math.min(
      Math.max(centerX - width / 2, VIEWPORT_MARGIN),
      maxLeft,
    );

    setPos({ top: toolbarTop, left: toolbarLeft });
  }, [editorRef, selection]);

  useLayoutEffect(() => {
    if (!visible) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setPos(null);
      return;
    }
    recalc();
    // The first recalc for a newly-shown selection runs before the toolbar
    // itself has mounted, so it centers against FALLBACK_WIDTH/HEIGHT rather
    // than the real box. Once the node exists, remeasure on the next frame
    // so the toolbar snaps to its accurate centered position.
    const raf = requestAnimationFrame(() => recalc());
    return () => cancelAnimationFrame(raf);
  }, [visible, recalc]);

  useEffect(() => {
    if (!visible) return;
    const handle = () => recalc();
    window.addEventListener("scroll", handle, true);
    window.addEventListener("resize", handle);
    return () => {
      window.removeEventListener("scroll", handle, true);
      window.removeEventListener("resize", handle);
    };
  }, [visible, recalc]);

  const dispatch = useCallback(
    (command: (state: EditorState) => TransactionSpec) => {
      editorRef.current?.runCommand(command);
      editorRef.current?.focus();
    },
    [editorRef],
  );

  return (
    <>
      {/* Mobile-only (CSS media query); readOnly disables buttons rather than
          hiding the bar, matching the pre-redesign toolbar's behavior. */}
      <div
        className={`${styles.toolbarStatic} ${zen ? styles.toolbarStaticZen : ""}`}
        role="toolbar"
        aria-label="Markdown formatting (mobile)"
        data-zen={zen}
      >
        {MOBILE_BUTTONS.map(({ label, icon: Icon, command }) => (
          <button
            key={label}
            type="button"
            className={styles.staticButton}
            aria-label={label}
            disabled={readOnly}
            // Prevents the button from stealing focus so the editor retains
            // cursor position and the virtual keyboard stays up.
            onPointerDown={(e) => e.preventDefault()}
            onClick={() => dispatch(command)}
          >
            <Icon size={16} strokeWidth={1.8} aria-hidden />
          </button>
        ))}
      </div>

      {/* Desktop-only (CSS media query); shown only while there's a
          non-empty selection, positioned above/below it. */}
      {visible && pos && (
        <div
          ref={toolbarRef}
          className={styles.toolbarFloating}
          role="toolbar"
          aria-label="Markdown formatting"
          style={{ top: pos.top, left: pos.left }}
        >
          {BUTTONS.map(({ label, icon: Icon, command }, i) => (
            <Fragment key={label}>
              {i === SEPARATOR_AFTER_INDEX + 1 && (
                <span className={styles.separator} aria-hidden="true" />
              )}
              <button
                type="button"
                className={styles.button}
                aria-label={label}
                disabled={readOnly}
                onPointerDown={(e) => e.preventDefault()}
                onClick={() => dispatch(command)}
              >
                <Icon size={15} strokeWidth={1.8} aria-hidden />
              </button>
            </Fragment>
          ))}
        </div>
      )}
    </>
  );
}
