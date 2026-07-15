import { Suspense } from "react";
import { useWasm } from "@/hooks/useWasm";
import {
  CheckCircle,
  AlertCircle,
  LogIn,
  Code2,
  ShieldCheck,
} from "lucide-react";
import { useEffect } from "react";
import { useNavigate } from "react-router";
import { useAuth } from "@/context/AuthContext";
import Footer from "@/components/Footer";
import styles from "./home.module.css";

const GITHUB_URL = "https://github.com/j-un/GophDrive";

function HomeContent() {
  const { isReady, error } = useWasm();
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!authLoading && user) {
      navigate("/drive/", { replace: true });
    }
  }, [user, authLoading, navigate]);

  const renderActions = () => {
    if (authLoading) {
      return <div className={styles.spinner}></div>;
    }

    if (user) {
      return <div className={styles.spinner}></div>;
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
          href="/api/auth/login"
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
          <LogIn size={18} strokeWidth={1.8} />
          Login with Google
        </a>
        <a
          href="/api/auth/demo-login"
          className="btn"
          style={{
            width: "100%",
            justifyContent: "center",
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            textDecoration: "none",
            background: "transparent",
            color: "var(--foreground)",
            borderColor: "var(--border)",
          }}
        >
          <LogIn size={18} strokeWidth={1.8} />
          Try Demo Mode
        </a>
      </div>
    );
  };

  return (
    <main className={styles.main}>
      <div className={styles.content}>
        <div className={styles.logoContainer}>
          <img
            src="/icon-512x512.png"
            alt="GophDrive"
            width={160}
            height={160}
            className={styles.logo}
            fetchPriority="high"
          />
        </div>
        <h1 className={styles.title}>GophDrive</h1>
        <p className={styles.subtitle}>
          Serverless Markdown Notes — Synced via Google Drive
        </p>

        <div className={styles.statusContainer}>
          {error ? (
            <div className={styles.statusRow}>
              <AlertCircle size={16} strokeWidth={1.8} />
              <span className={styles.statusText}>
                Wasm: {error.message} (preview disabled)
              </span>
            </div>
          ) : isReady ? (
            <div className={styles.statusRow}>
              <CheckCircle
                size={16}
                strokeWidth={1.8}
                style={{ color: "var(--success)" }}
              />
              <span className={styles.statusText}>Core Module Active</span>
            </div>
          ) : (
            <div className={styles.statusRow}>
              <div className={`${styles.spinner} ${styles.spinnerSmall}`}></div>
              <span className={styles.statusText}>Loading Wasm...</span>
            </div>
          )}

          {renderActions()}
        </div>

        <hr className={styles.divider} />

        <div className={styles.infoSection}>
          <div className={styles.infoItem}>
            <ShieldCheck
              size={16}
              strokeWidth={1.8}
              className={styles.infoIcon}
            />
            <p className={styles.infoText}>
              This service is invite-only. Only Google accounts approved by the
              administrator can log in.
            </p>
          </div>
          <div className={styles.infoItem}>
            <Code2 size={16} strokeWidth={1.8} className={styles.infoIcon} />
            <p className={styles.infoText}>
              This is a self-hosted instance of{" "}
              <a
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                className={styles.infoLink}
              >
                GophDrive
              </a>
              , an open-source serverless note app.
            </p>
          </div>
        </div>
      </div>
      <Footer />
    </main>
  );
}

export default function Home() {
  return (
    <Suspense
      fallback={
        <main className={styles.main}>
          <div className={styles.content}>
            <div className={styles.loadingWrapper}>
              <div className={styles.spinner}></div>
              <p className={styles.loadingText}>Loading...</p>
            </div>
          </div>
          <Footer />
        </main>
      }
    >
      <HomeContent />
    </Suspense>
  );
}
