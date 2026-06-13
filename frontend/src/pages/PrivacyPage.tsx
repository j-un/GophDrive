import { useMemo } from "react";
import { Link, Navigate } from "react-router";
import { Preview } from "@/components/Preview";
import privacyTemplate from "@docs/PRIVACY_POLICY_TEMPLATE.md?raw";
import styles from "./home.module.css";

export default function PrivacyPage() {
  const privacyPolicyUrl = import.meta.env.VITE_PRIVACY_POLICY_URL || "";
  const appName = import.meta.env.VITE_APP_NAME || "GophDrive";
  const contactInfo = import.meta.env.VITE_CONTACT_EMAIL || "the administrator";
  const encryptionMethod =
    import.meta.env.VITE_ENCRYPTION_METHOD ||
    "AWS KMS / your encryption method";
  const currentDate = new Date().toISOString().split("T")[0];

  const markdown = useMemo(() => {
    return privacyTemplate
      .replace(
        /> \*\*Note for hosters\*\*[\s\S]*?(?=\n\n|\n*$)/i,
        "<!-- Host note removed -->",
      )
      .replace(/\[Your Service Name\]/g, appName)
      .replace(/\[Your contact information\]/g, contactInfo)
      .replace(/\[AWS KMS \/ your encryption method\]/g, encryptionMethod)
      .replace(/\[Date\]/g, currentDate);
  }, [appName, contactInfo, encryptionMethod, currentDate]);

  if (!privacyPolicyUrl) {
    return <Navigate to="/" replace />;
  }

  return (
    <main className={styles.main}>
      <div
        className={`${styles.card} glass`}
        style={{
          maxWidth: "800px",
          width: "100%",
          padding: "2rem",
          textAlign: "left",
        }}
      >
        <Link
          to="/"
          className="btn"
          style={{
            marginBottom: "2rem",
            display: "inline-block",
            textDecoration: "none",
          }}
        >
          ← Back to Home
        </Link>

        <div style={{ marginTop: "1rem" }}>
          <Preview markdown={markdown} className="w-full" />
        </div>
      </div>
    </main>
  );
}
