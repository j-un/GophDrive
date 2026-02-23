"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { updateUser } from "@/lib/api";
import FolderSelector from "@/components/FolderSelector";
import { Loader2 } from "lucide-react";

export default function SetupPage() {
  const { user, loading, refreshUser } = useAuth();
  const router = useRouter();
  const [selectedFolderId, setSelectedFolderId] = useState<string>("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!loading && user?.base_folder_id) {
      // Already setup
      router.push("/notes");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, user]);

  const handleSave = async () => {
    if (!selectedFolderId) return;
    setSaving(true);
    try {
      await updateUser({ base_folder_id: selectedFolderId });
      await refreshUser(); // Update context
      router.push("/notes");
    } catch (e) {
      console.error(e);
      alert("Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  if (loading)
    return (
      <div
        style={{
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          height: "100vh",
          background: "var(--background)",
        }}
      >
        <Loader2
          className="animate-spin"
          style={{ color: "var(--text-secondary)" }}
        />
      </div>
    );

  return (
    <div
      style={{
        display: "flex",
        minHeight: "100vh",
        alignItems: "center",
        justifyContent: "center",
        background: "var(--surface)",
        padding: "1rem",
      }}
    >
      <div
        style={{
          maxWidth: "600px",
          width: "100%",
          background: "var(--background)",
          padding: "2rem",
          borderRadius: "1rem",
          border: "1px solid var(--border)",
          boxShadow: "0 4px 6px -1px rgba(0, 0, 0, 0.1)",
        }}
      >
        <h1
          style={{
            fontSize: "1.5rem",
            fontWeight: "bold",
            marginBottom: "0.5rem",
            color: "var(--foreground)",
          }}
        >
          Welcome to GophDrive
        </h1>
        <p
          style={{
            color: "var(--text-secondary)",
            marginBottom: "1.5rem",
            lineHeight: 1.5,
          }}
        >
          Please select a root folder from your Google Drive to store your
          notes. GophDrive will only access files within this folder.
        </p>

        <FolderSelector
          selectedId={selectedFolderId}
          onSelect={setSelectedFolderId}
        />

        <div
          style={{
            marginTop: "2rem",
            display: "flex",
            justifyContent: "flex-end",
          }}
        >
          <button
            onClick={handleSave}
            disabled={!selectedFolderId || saving}
            className="btn btn-primary"
            style={{
              opacity: !selectedFolderId || saving ? 0.5 : 1,
              cursor: !selectedFolderId || saving ? "not-allowed" : "pointer",
              padding: "0.75rem 2rem",
            }}
          >
            {saving ? (
              <Loader2 className="animate-spin" size={18} />
            ) : (
              "Start Using GophDrive"
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
