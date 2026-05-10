"use client";

import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { apiFetch } from "@/lib/api";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Loader2, ArrowLeft } from "lucide-react";

export default function SettingsPage() {
  const { user, loading, refreshUser } = useAuth();
  const router = useRouter();

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
    </div>
  );
}
