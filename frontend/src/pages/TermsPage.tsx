import { useMemo } from "react";
import { Link, Navigate } from "react-router";
import { Preview } from "@/components/Preview";
import termsTemplate from "@docs/TERMS_OF_SERVICE_TEMPLATE.md?raw";
import styles from "./home.module.css";

export default function TermsPage() {
  const termsUrl = import.meta.env.VITE_TERMS_OF_SERVICE_URL || "";
  const appName = import.meta.env.VITE_APP_NAME || "GophDrive";
  const contactInfo = import.meta.env.VITE_CONTACT_EMAIL || "the administrator";
  const jurisdiction = import.meta.env.VITE_JURISDICTION || "Your Jurisdiction";
  const currentDate = new Date().toISOString().split("T")[0];

  const markdown = useMemo(() => {
    return termsTemplate
      .replace(
        /> \*\*Note for hosters\*\*[\s\S]*?(?=\n\n|\n*$)/i,
        "<!-- Host note removed -->",
      )
      .replace(/\[Your Service Name\]/g, appName)
      .replace(/\[Your contact information\]/g, contactInfo)
      .replace(/\[Your Jurisdiction\]/g, jurisdiction)
      .replace(/\[Date\]/g, currentDate);
  }, [appName, contactInfo, jurisdiction, currentDate]);

  if (!termsUrl) {
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
