import React, { useState, useEffect, useRef } from "react";
import { X } from "lucide-react";

interface RenameDialogProps {
  isOpen: boolean;
  initialName: string;
  onRename: (newName: string) => void;
  onCancel: () => void;
  title?: string;
}

export const RenameDialog: React.FC<RenameDialogProps> = ({
  isOpen,
  initialName,
  onRename,
  onCancel,
  title = "Rename",
}) => {
  const [name, setName] = useState(initialName);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setName(initialName);
      setTimeout(() => inputRef.current?.select(), 50);
    }
  }, [isOpen, initialName]);

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onRename(name);
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "rgba(0,0,0,0.5)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 100,
      }}
    >
      <div
        style={{
          background: "var(--card)",
          padding: "1.5rem",
          borderRadius: "0.5rem",
          minWidth: "300px",
          boxShadow: "0 4px 6px -1px rgba(0, 0, 0, 0.1)",
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "1rem",
          }}
        >
          <h3 style={{ margin: 0, fontWeight: "bold" }}>{title}</h3>
          <button
            onClick={onCancel}
            style={{
              background: "transparent",
              border: "none",
              cursor: "pointer",
            }}
          >
            <X size={20} />
          </button>
        </div>
        <form onSubmit={handleSubmit}>
          <input
            ref={inputRef}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            style={{
              width: "100%",
              padding: "0.5rem",
              marginBottom: "1rem",
              border: "1px solid var(--border)",
              borderRadius: "0.25rem",
              fontSize: "1rem",
              background: "var(--background)",
              color: "var(--foreground)",
            }}
          />
          <div
            style={{
              display: "flex",
              justifyContent: "flex-end",
              gap: "0.5rem",
            }}
          >
            <button type="button" className="btn" onClick={onCancel}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary">
              Rename
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
