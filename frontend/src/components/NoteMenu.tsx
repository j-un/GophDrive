import React, { useEffect, useRef } from "react";
import { Trash2, Copy, MoreVertical, PenLine, Star } from "lucide-react";

interface NoteMenuProps {
  isOpen: boolean;
  onToggle: (e: React.MouseEvent) => void;
  onClose: () => void;
  onDelete: (e: React.MouseEvent) => void;
  onDuplicate?: (e: React.MouseEvent) => void;
  onRename?: (e: React.MouseEvent) => void;
  onStar?: (e: React.MouseEvent) => void;
  isStarred?: boolean;
  align?: "right" | "left";
}

export const NoteMenu: React.FC<NoteMenuProps> = ({
  isOpen,
  onToggle,
  onClose,
  onDelete,
  onDuplicate,
  onRename,
  onStar,
  isStarred,
  align = "right",
}) => {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isOpen, onClose]);

  return (
    <div
      style={{ position: "relative", display: "inline-block" }}
      ref={menuRef}
    >
      <button
        onClick={onToggle}
        className="btn icon-btn"
        style={{
          padding: "0.25rem",
          background: "transparent",
          border: "none",
          borderRadius: "50%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          cursor: "pointer",
        }}
        onMouseEnter={(e) =>
          (e.currentTarget.style.backgroundColor = "var(--muted)")
        }
        onMouseLeave={(e) =>
          (e.currentTarget.style.backgroundColor = "transparent")
        }
        title="Options"
      >
        <MoreVertical size={18} style={{ color: "var(--muted-foreground)" }} />
      </button>

      {isOpen && (
        <div
          style={{
            position: "absolute",
            top: "100%",
            [align]: 0,
            zIndex: 50,
            background: "var(--card)",
            border: "1px solid var(--border)",
            borderRadius: "0.5rem",
            boxShadow: "0 4px 6px -1px rgba(0, 0, 0, 0.1)",
            minWidth: "140px",
            overflow: "hidden",
            marginTop: "0.25rem",
            display: "flex",
            flexDirection: "column",
          }}
        >
          {onStar && (
            <button
              onClick={onStar}
              style={menuItemStyle}
              onMouseEnter={(e) =>
                (e.currentTarget.style.backgroundColor = "var(--muted)")
              }
              onMouseLeave={(e) =>
                (e.currentTarget.style.backgroundColor = "transparent")
              }
            >
              <Star
                size={16}
                fill={isStarred ? "currentColor" : "none"}
                style={{ color: isStarred ? "var(--yellow)" : "inherit" }}
              />
              {isStarred ? "Unstar" : "Star"}
            </button>
          )}
          {onDuplicate && (
            <button
              onClick={onDuplicate}
              style={menuItemStyle}
              onMouseEnter={(e) =>
                (e.currentTarget.style.backgroundColor = "var(--muted)")
              }
              onMouseLeave={(e) =>
                (e.currentTarget.style.backgroundColor = "transparent")
              }
            >
              <Copy size={16} /> Duplicate
            </button>
          )}
          {onRename && (
            <button
              onClick={onRename}
              style={menuItemStyle}
              onMouseEnter={(e) =>
                (e.currentTarget.style.backgroundColor = "var(--muted)")
              }
              onMouseLeave={(e) =>
                (e.currentTarget.style.backgroundColor = "transparent")
              }
            >
              <PenLine size={16} /> Rename
            </button>
          )}
          <div
            style={{
              borderTop: "1px solid var(--border)",
              margin: "0.25rem 0",
            }}
          ></div>
          <button
            onClick={onDelete}
            style={{ ...menuItemStyle, color: "var(--destructive)" }}
            onMouseEnter={(e) =>
              (e.currentTarget.style.backgroundColor = "var(--muted)")
            }
            onMouseLeave={(e) =>
              (e.currentTarget.style.backgroundColor = "transparent")
            }
          >
            <Trash2 size={16} /> Delete
          </button>
        </div>
      )}
    </div>
  );
};

const menuItemStyle: React.CSSProperties = {
  textAlign: "left",
  padding: "0.75rem 1rem",
  background: "transparent",
  border: "none",
  cursor: "pointer",
  display: "flex",
  alignItems: "center",
  gap: "0.75rem",
  color: "inherit",
  fontSize: "0.875rem",
  width: "100%",
  transition: "background-color 0.1s",
};
