"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { updateUser, apiFetch } from "@/lib/api";
import FolderSelector from "@/components/FolderSelector";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Loader2, ArrowLeft, Save } from "lucide-react";

export default function SettingsPage() {
  const { user, loading, refreshUser } = useAuth();
  const router = useRouter();
  const [selectedFolderId, setSelectedFolderId] = useState<string>("");
  const [saving, setSaving] = useState(false);

  const handleLogout = async () => {
    if (confirm("Are you sure you want to logout?")) {
      try {
        await apiFetch("/auth/logout", { method: "POST" });
      } catch (e) {
        console.error("Logout failed", e);
      }
      localStorage.removeItem("session_token");
      await refreshUser();
      router.push("/");
    }
  };

  const handleSave = async () => {
    if (!selectedFolderId || selectedFolderId === user?.base_folder_id) return;
    setSaving(true);
    try {
      await updateUser({ base_folder_id: selectedFolderId });
      await refreshUser();
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
          style={{ color: "var(--muted-foreground)" }}
        />
      </div>
    );

  return (
    <div
      style={{
        padding: "2rem",
        maxWidth: "800px",
        margin: "0 auto",
        color: "var(--foreground)",
      }}
    >
      <div
        style={{
          marginBottom: "2rem",
          display: "flex",
          alignItems: "center",
          gap: "1rem",
        }}
      >
        <button
          onClick={() => router.back()}
          style={{
            background: "transparent",
            border: "1px solid var(--border)",
            cursor: "pointer",
            padding: "0.5rem",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            borderRadius: "50%",
          }}
        >
          <ArrowLeft size={20} style={{ color: "var(--foreground)" }} />
        </button>
        <h1 style={{ fontSize: "1.5rem", fontWeight: "bold" }}>Settings</h1>
      </div>

      <div
        style={{
          background: "var(--card)",
          padding: "1.5rem",
          borderRadius: "0.5rem",
          border: "1px solid var(--border)",
          marginBottom: "2rem",
        }}
      >
        <h2
          style={{
            fontSize: "1.125rem",
            fontWeight: 600,
            marginBottom: "1rem",
          }}
        >
          Appearance
        </h2>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <span style={{ color: "var(--muted-foreground)" }}>Theme</span>
          <ThemeToggle />
        </div>
      </div>

      <div
        style={{
          background: "var(--card)",
          padding: "1.5rem",
          borderRadius: "0.5rem",
          border: "1px solid var(--border)",
          marginBottom: "2rem",
        }}
      >
        <h2
          style={{
            fontSize: "1.125rem",
            fontWeight: 600,
            marginBottom: "1rem",
          }}
        >
          Account
        </h2>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(100px, auto) 1fr",
            gap: "1rem",
            alignItems: "center",
          }}
        >
          <span style={{ color: "var(--muted-foreground)" }}>User ID</span>
          <code
            style={{
              background: "var(--muted)",
              color: "var(--foreground)",
              padding: "0.25rem 0.5rem",
              borderRadius: "0.25rem",
              fontFamily: "monospace",
            }}
          >
            {user?.id}
          </code>

          <span style={{ color: "var(--muted-foreground)" }}>
            Current Base Folder ID
          </span>
          <code
            style={{
              background: "var(--muted)",
              color: "var(--foreground)",
              padding: "0.25rem 0.5rem",
              borderRadius: "0.25rem",
              fontFamily: "monospace",
            }}
          >
            {user?.base_folder_id}
          </code>
        </div>

        <div
          style={{
            marginTop: "1.5rem",
            borderTop: "1px solid var(--border)",
            paddingTop: "1.5rem",
          }}
        >
          <button
            onClick={handleLogout}
            className="btn"
            style={{
              color: "var(--destructive)",
              borderColor: "var(--destructive)",
              background: "transparent",
            }}
          >
            Logout
          </button>
        </div>
      </div>

      <div
        style={{
          background: "var(--card)",
          padding: "1.5rem",
          borderRadius: "0.5rem",
          border: "1px solid var(--border)",
        }}
      >
        <h2
          style={{
            fontSize: "1.125rem",
            fontWeight: 600,
            marginBottom: "1rem",
          }}
        >
          Storage Settings
        </h2>
        <p style={{ marginBottom: "1rem", color: "var(--muted-foreground)" }}>
          Change the root folder used by GophDrive.
        </p>

        <FolderSelector
          selectedId={selectedFolderId || user?.base_folder_id}
          onSelect={setSelectedFolderId}
        />

        <div
          style={{
            marginTop: "1.5rem",
            display: "flex",
            justifyContent: "flex-end",
          }}
        >
          <button
            onClick={handleSave}
            disabled={
              !selectedFolderId ||
              selectedFolderId === user?.base_folder_id ||
              saving
            }
            className="btn btn-primary"
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              opacity:
                !selectedFolderId ||
                selectedFolderId === user?.base_folder_id ||
                saving
                  ? 0.5
                  : 1,
              cursor:
                !selectedFolderId ||
                selectedFolderId === user?.base_folder_id ||
                saving
                  ? "not-allowed"
                  : "pointer",
            }}
          >
            {saving ? (
              <Loader2 className="animate-spin" size={18} />
            ) : (
              <Save size={18} />
            )}
            Save Changes
          </button>
        </div>
      </div>
    </div>
  );
}
