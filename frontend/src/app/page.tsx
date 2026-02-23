"use client";

import { Suspense } from "react";
import { useWasm } from "@/hooks/useWasm";
import { CheckCircle, AlertCircle, LogIn } from "lucide-react";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import styles from "./page.module.css";

import { useAuth } from "@/context/AuthContext";

function HomeContent() {
  const { isReady, error } = useWasm();
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && user) {
      router.replace("/notes");
    }
  }, [user, authLoading, router]);

  // Show login/actions regardless of Wasm status
  const renderActions = () => {
    if (authLoading) {
      return <div className={styles.spinner}></div>;
    }

    if (user) {
      return <div className={styles.spinner}></div>; // Show spinner while redirecting
    }

    return (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "0.75rem",
          width: "100%",
        }}
      >
        <a
          href={`${process.env.NEXT_PUBLIC_API_URL || ""}/auth/login`}
          className="btn btn-primary"
          style={{
            width: "100%",
            justifyContent: "center",
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            textDecoration: "none",
          }}
        >
          <LogIn size={18} />
          Login with Google
        </a>
        <a
          href={`${process.env.NEXT_PUBLIC_API_URL || ""}/auth/demo-login`}
          className="btn"
          style={{
            width: "100%",
            justifyContent: "center",
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            textDecoration: "none",
            background: "var(--muted)",
            opacity: 0.9,
          }}
        >
          <LogIn size={18} />
          Try Demo Mode
        </a>
      </div>
    );
  };

  return (
    <main className={styles.main}>
      <div className={`${styles.card} glass`}>
        <h1 className={styles.title}>GophDrive</h1>
        <p
          style={{ opacity: 0.6, marginBottom: "1.5rem", textAlign: "center" }}
        >
          Serverless Markdown Notes — Synced via Google Drive
        </p>

        <div className={styles.statusContainer}>
          {/* Wasm Status */}
          {error ? (
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                marginBottom: "1rem",
                opacity: 0.6,
              }}
            >
              <AlertCircle size={16} />
              <span style={{ fontSize: "0.75rem" }}>
                Wasm: {error.message} (preview disabled)
              </span>
            </div>
          ) : isReady ? (
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                marginBottom: "1rem",
              }}
            >
              <CheckCircle size={16} style={{ color: "var(--success)" }} />
              <span style={{ fontSize: "0.75rem", opacity: 0.7 }}>
                Core Module Active
              </span>
            </div>
          ) : (
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                marginBottom: "1rem",
              }}
            >
              <div
                className={styles.spinner}
                style={{ width: "16px", height: "16px", borderWidth: "2px" }}
              ></div>
              <span style={{ fontSize: "0.75rem", opacity: 0.6 }}>
                Loading Wasm...
              </span>
            </div>
          )}

          {/* Actions always visible */}
          {renderActions()}
        </div>
      </div>
    </main>
  );
}

export default function Home() {
  return (
    <Suspense
      fallback={
        <main className={styles.main}>
          <div className={`${styles.card} glass`}>
            <div className={styles.loadingWrapper}>
              <div className={styles.spinner}></div>
              <p className="opacity-60">Loading...</p>
            </div>
          </div>
        </main>
      }
    >
      <HomeContent />
    </Suspense>
  );
}
