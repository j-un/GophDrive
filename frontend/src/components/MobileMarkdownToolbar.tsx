/* eslint-disable react-refresh/only-export-components */
// TODO(aria-pressed): Derive active-marker state from CodeMirror ViewPlugin to reflect
// current selection formatting in button aria-pressed attributes (future enhancement).

import type { RefObject } from "react";
import {
  Bold,
  Code,
  Code2,
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
import type { EditorHandle } from "./Editor";
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

interface Props {
  editorRef: RefObject<EditorHandle | null>;
  readOnly?: boolean;
}

export const BUTTONS: ReadonlyArray<{
  label: string;
  icon: React.ComponentType<{
    size?: number;
    "aria-hidden"?: boolean | "true" | "false";
  }>;
  command: (state: EditorState) => TransactionSpec;
}> = [
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

export function MobileMarkdownToolbar({ editorRef, readOnly = false }: Props) {
  return (
    <div
      className="mobile-md-toolbar"
      role="toolbar"
      aria-label="Markdown formatting"
    >
      {BUTTONS.map(({ label, icon: Icon, command }) => (
        <button
          key={label}
          type="button"
          aria-label={label}
          disabled={readOnly}
          onPointerDown={(e) => e.preventDefault()}
          onClick={() => {
            editorRef.current?.runCommand(command);
            editorRef.current?.focus();
          }}
        >
          <Icon size={18} aria-hidden />
        </button>
      ))}
    </div>
  );
}
