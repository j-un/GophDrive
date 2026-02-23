"use client";

import { useState, useEffect } from "react";
import { listDriveFolders, createFolder, FileItem } from "@/lib/api";
import { Loader2, Folder, Check, Plus } from "lucide-react";

interface FolderSelectorProps {
  onSelect: (folderId: string) => void;
  selectedId?: string;
}

export default function FolderSelector({
  onSelect,
  selectedId,
}: FolderSelectorProps) {
  const [folders, setFolders] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");

  const fetchFolders = async () => {
    setLoading(true);
    try {
      const list = await listDriveFolders();
      // Sort alphabetically
      setFolders(list.sort((a, b) => a.name.localeCompare(b.name)));
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchFolders();
  }, []);

  const handleCreate = async () => {
    if (!newFolderName) return;
    setCreating(true);
    try {
      // create folder with no parent = root
      await createFolder(newFolderName);
      await fetchFolders();
      setNewFolderName("");
    } catch (e) {
      console.error(e);
    } finally {
      setCreating(false);
    }
  };

  if (loading)
    return (
      <div
        style={{ display: "flex", justifyContent: "center", padding: "2rem" }}
      >
        <Loader2
          className="animate-spin"
          style={{ color: "var(--muted-foreground)" }}
        />
      </div>
    );

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
        width: "100%",
      }}
    >
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(250px, 1fr))",
          gap: "0.5rem",
          maxHeight: "300px",
          overflowY: "auto",
          paddingRight: "0.25rem",
        }}
      >
        {folders.map((f) => (
          <div
            key={f.id}
            onClick={() => onSelect(f.id)}
            style={{
              padding: "1rem",
              borderRadius: "0.5rem",
              border: "1px solid",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: "0.75rem",
              transition: "background-color 0.2s",
              backgroundColor:
                selectedId === f.id ? "var(--muted)" : "var(--background)",
              borderColor:
                selectedId === f.id ? "var(--primary)" : "var(--border)",
            }}
          >
            <Folder
              size={20}
              style={{
                color:
                  selectedId === f.id
                    ? "var(--primary)"
                    : "var(--muted-foreground)",
              }}
            />
            <span
              style={{
                flex: 1,
                fontWeight: 500,
                color:
                  selectedId === f.id ? "var(--primary)" : "var(--foreground)",
              }}
            >
              {f.name}
            </span>
            {selectedId === f.id && (
              <Check size={18} style={{ color: "var(--primary)" }} />
            )}
          </div>
        ))}

        {folders.length === 0 && (
          <div
            style={{
              textAlign: "center",
              color: "var(--muted-foreground)",
              padding: "2rem",
              border: "1px dashed var(--border)",
              borderRadius: "0.5rem",
              background: "var(--card)",
            }}
          >
            No root folders found. Use the form below to create one.
          </div>
        )}
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "0.5rem",
          marginTop: "0.5rem",
          paddingTop: "1rem",
          borderTop: "1px solid var(--border)",
        }}
      >
        <span
          style={{
            fontSize: "0.875rem",
            fontWeight: 500,
            color: "var(--muted-foreground)",
          }}
        >
          Create New Root Folder
        </span>
        <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
          <input
            type="text"
            placeholder="e.g. My Project Notes"
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            style={{
              flex: 1,
              padding: "0.5rem",
              border: "1px solid var(--border)",
              borderRadius: "0.375rem",
              background: "var(--background)",
              color: "var(--foreground)",
            }}
            onKeyDown={(e) => e.key === "Enter" && handleCreate()}
          />
          <button
            onClick={handleCreate}
            disabled={creating || !newFolderName}
            className="btn"
            style={{
              backgroundColor: "var(--muted)",
              color: "var(--foreground)",
              opacity: creating || !newFolderName ? 0.5 : 1,
              cursor: creating || !newFolderName ? "not-allowed" : "pointer",
            }}
          >
            {creating ? (
              <Loader2 className="animate-spin" size={16} />
            ) : (
              <Plus size={16} />
            )}
            Create
          </button>
        </div>
      </div>
    </div>
  );
}
