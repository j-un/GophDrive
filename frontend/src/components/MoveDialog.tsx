import React, { useState, useEffect } from "react";
import { X, Folder, ChevronRight, Home } from "lucide-react";
import { listFiles, getBreadcrumbs, BreadcrumbItem, FileItem } from "@/lib/api";
import { partitionFilesByKind } from "@/lib/partitionFiles";

interface MoveDialogProps {
  isOpen: boolean;
  itemId: string;
  itemName: string;
  itemIsFolder: boolean;
  currentParentId: string | undefined;
  onMove: (destFolderId: string) => void;
  onCancel: () => void;
}

export const MoveDialog: React.FC<MoveDialogProps> = ({
  isOpen,
  itemId,
  itemName,
  itemIsFolder,
  currentParentId,
  onMove,
  onCancel,
}) => {
  const [pickerFolderId, setPickerFolderId] = useState<string>("");
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([]);
  const [subfolders, setSubfolders] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPickerFolderId("");
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true);

    const fetchContents = async () => {
      try {
        const [items, crumbs] = await Promise.all([
          listFiles(pickerFolderId || undefined),
          pickerFolderId ? getBreadcrumbs(pickerFolderId) : Promise.resolve([]),
        ]);
        if (cancelled) return;
        const { folders } = partitionFilesByKind(items);
        // Exclude the item itself from the list. Moves into own descendants are
        // rejected server-side (ErrInvalidMove); the UI doesn't pre-traverse
        // the subtree to hide them all upfront.
        const visible = folders.filter((f) => f.id !== itemId);
        setBreadcrumbs(crumbs);
        setSubfolders(visible);
      } catch {
        if (!cancelled) {
          setSubfolders([]);
          setBreadcrumbs([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchContents();
    return () => {
      cancelled = true;
    };
  }, [isOpen, pickerFolderId, itemId]);

  if (!isOpen) return null;

  const isCurrentLocation = pickerFolderId === (currentParentId ?? "");

  const handleBreadcrumbClick = (id: string) => {
    setPickerFolderId(id);
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
          width: "360px",
          maxWidth: "90vw",
          boxShadow: "0 4px 6px -1px rgba(0, 0, 0, 0.1)",
          display: "flex",
          flexDirection: "column",
          gap: "1rem",
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <h3 style={{ margin: 0, fontWeight: "bold" }}>
            Move &ldquo;{itemName}&rdquo;
          </h3>
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

        {/* Breadcrumb */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.25rem",
            flexWrap: "wrap",
            fontSize: "0.875rem",
            color: "var(--muted-foreground)",
          }}
        >
          <button
            onClick={() => handleBreadcrumbClick("")}
            style={crumbBtnStyle(pickerFolderId === "")}
          >
            <Home size={14} /> Home
          </button>
          {breadcrumbs.map((crumb) => (
            <React.Fragment key={crumb.id}>
              <ChevronRight size={14} />
              <button
                onClick={() => handleBreadcrumbClick(crumb.id)}
                style={crumbBtnStyle(crumb.id === pickerFolderId)}
              >
                {crumb.name}
              </button>
            </React.Fragment>
          ))}
        </div>

        {/* Folder list */}
        <div
          style={{
            border: "1px solid var(--border)",
            borderRadius: "0.375rem",
            overflow: "hidden",
            minHeight: "120px",
            maxHeight: "240px",
            overflowY: "auto",
          }}
        >
          {loading ? (
            <div style={emptyStyle}>Loading…</div>
          ) : subfolders.length === 0 ? (
            <div style={emptyStyle}>No subfolders</div>
          ) : (
            subfolders.map((folder) => {
              const isMovingIntoSelf = itemIsFolder && folder.id === itemId;
              return (
                <button
                  key={folder.id}
                  disabled={isMovingIntoSelf}
                  onClick={() => setPickerFolderId(folder.id)}
                  style={folderRowStyle(isMovingIntoSelf)}
                  onMouseEnter={(e) => {
                    if (!isMovingIntoSelf)
                      (e.currentTarget as HTMLElement).style.backgroundColor =
                        "var(--muted)";
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLElement).style.backgroundColor =
                      "transparent";
                  }}
                >
                  <Folder size={16} style={{ flexShrink: 0 }} />
                  <span
                    style={{
                      flex: 1,
                      textAlign: "left",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {folder.name}
                  </span>
                  <ChevronRight
                    size={14}
                    style={{ flexShrink: 0, opacity: 0.5 }}
                  />
                </button>
              );
            })
          )}
        </div>

        {/* Actions */}
        <div
          style={{ display: "flex", justifyContent: "flex-end", gap: "0.5rem" }}
        >
          <button type="button" className="btn" onClick={onCancel}>
            Cancel
          </button>
          <button
            type="button"
            className="btn btn-primary"
            disabled={isCurrentLocation}
            onClick={() => onMove(pickerFolderId)}
          >
            Move here
          </button>
        </div>
      </div>
    </div>
  );
};

const crumbBtnStyle = (active: boolean): React.CSSProperties => ({
  background: "transparent",
  border: "none",
  cursor: "pointer",
  padding: "0.125rem 0.25rem",
  borderRadius: "0.25rem",
  display: "flex",
  alignItems: "center",
  gap: "0.25rem",
  color: active ? "var(--foreground)" : "inherit",
  fontWeight: active ? "600" : "normal",
  fontSize: "inherit",
});

const emptyStyle: React.CSSProperties = {
  padding: "2rem",
  textAlign: "center",
  color: "var(--muted-foreground)",
  fontSize: "0.875rem",
};

const folderRowStyle = (disabled: boolean): React.CSSProperties => ({
  display: "flex",
  alignItems: "center",
  gap: "0.75rem",
  width: "100%",
  padding: "0.625rem 1rem",
  background: "transparent",
  border: "none",
  borderBottom: "1px solid var(--border)",
  cursor: disabled ? "not-allowed" : "pointer",
  color: disabled ? "var(--muted-foreground)" : "inherit",
  opacity: disabled ? 0.5 : 1,
  fontSize: "0.875rem",
  transition: "background-color 0.1s",
});
