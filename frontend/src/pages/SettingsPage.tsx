import { useState, type CSSProperties } from "react";
import { useNavigate } from "react-router";
import { useAuth } from "@/context/AuthContext";
import { apiFetch, exportNotes } from "@/lib/api";
import { ThemeToggle } from "@/components/ThemeToggle";
import { APIKeysSection } from "@/components/APIKeysSection";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { Loader2, ArrowLeft, Download } from "lucide-react";

const sectionHeadingStyle: CSSProperties = {
  fontSize: "13px",
  fontWeight: 600,
  color: "var(--text-secondary)",
  textTransform: "uppercase",
  letterSpacing: "0.08em",
  marginBottom: "1rem",
};

const sectionStyle: CSSProperties = {
  padding: "1.75rem 0",
  borderBottom: "1px solid var(--border)",
};

const lastSectionStyle: CSSProperties = {
  padding: "1.75rem 0 0",
};

const rowLabelStyle: CSSProperties = {
  color: "var(--text-secondary)",
  fontSize: "0.9rem",
};

export default function SettingsPage() {
  const { user, loading, refreshUser } = useAuth();
  const navigate = useNavigate();
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);

  const handleLogout = async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } catch (e) {
      console.error("Logout failed", e);
    }
    await refreshUser();
    navigate("/");
  };

  const handleExport = async () => {
    setExporting(true);
    setExportError(null);
    try {
      const { blob, filename } = await exportNotes();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (e) {
      console.error("Export failed", e);
      setExportError(e instanceof Error ? e.message : "Export failed");
    } finally {
      setExporting(false);
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
          strokeWidth={1.8}
          style={{ color: "var(--text-muted)" }}
        />
      </div>
    );

  return (
    <div
      style={{
        width: "720px",
        maxWidth: "100%",
        margin: "0 auto",
        padding: "3rem 1.5rem 4rem",
        color: "var(--foreground)",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "0.75rem",
          paddingBottom: "1.5rem",
          borderBottom: "1px solid var(--border)",
        }}
      >
        <button
          onClick={() => navigate(-1)}
          aria-label="Back"
          style={{
            background: "transparent",
            border: "none",
            cursor: "pointer",
            padding: "0.375rem",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--text-muted)",
          }}
        >
          <ArrowLeft size={20} strokeWidth={1.8} />
        </button>
        <h1
          style={{
            fontSize: "26px",
            fontWeight: 700,
            letterSpacing: "-0.02em",
            margin: 0,
          }}
        >
          Settings
        </h1>
      </div>

      <section style={sectionStyle}>
        <h2 style={sectionHeadingStyle}>Appearance</h2>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <span style={rowLabelStyle}>Theme</span>
          <ThemeToggle />
        </div>
      </section>

      <section style={sectionStyle}>
        <h2 style={sectionHeadingStyle}>Export</h2>
        <p
          style={{
            color: "var(--text-secondary)",
            fontSize: "0.875rem",
            marginBottom: "1rem",
          }}
        >
          Download every note as a ZIP archive. Folder hierarchy is preserved.
        </p>
        <button
          onClick={handleExport}
          disabled={exporting}
          className="btn"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "0.5rem",
            background: "transparent",
            color: "var(--foreground)",
            borderColor: "var(--border)",
            opacity: exporting ? 0.6 : 1,
            cursor: exporting ? "not-allowed" : "pointer",
          }}
        >
          {exporting ? (
            <Loader2 size={16} strokeWidth={1.8} className="animate-spin" />
          ) : (
            <Download size={16} strokeWidth={1.8} />
          )}
          {exporting ? "Exporting..." : "Export all notes (.zip)"}
        </button>
        {exportError && (
          <p
            style={{
              color: "var(--destructive)",
              fontSize: "0.875rem",
              marginTop: "0.75rem",
            }}
          >
            {exportError}
          </p>
        )}
      </section>

      <APIKeysSection />

      <section style={lastSectionStyle}>
        <h2 style={sectionHeadingStyle}>Account</h2>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(100px, auto) 1fr",
            gap: "1rem",
            alignItems: "center",
          }}
        >
          <span style={rowLabelStyle}>User ID</span>
          <code
            style={{
              background: "var(--muted)",
              color: "var(--foreground)",
              padding: "0.25rem 0.5rem",
              borderRadius: "0.25rem",
              fontFamily: "var(--font-mono)",
              fontSize: "0.875rem",
            }}
          >
            {user?.id}
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
            onClick={() => setShowLogoutConfirm(true)}
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
      </section>

      <ConfirmDialog
        isOpen={showLogoutConfirm}
        title="Logout"
        message="Are you sure you want to logout?"
        confirmLabel="Logout"
        onConfirm={() => {
          setShowLogoutConfirm(false);
          handleLogout();
        }}
        onCancel={() => setShowLogoutConfirm(false)}
      />
    </div>
  );
}
