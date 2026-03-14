import fs from "fs";
import path from "path";
import { notFound } from "next/navigation";
import Link from "next/link";
import { Preview } from "@/components/Preview";
import styles from "../page.module.css";

export default function TermsOfServicePage() {
  if (!process.env.NEXT_PUBLIC_TERMS_OF_SERVICE_URL) {
    notFound();
  }

  const templatePath = path.join(
    process.cwd(),
    "..",
    "docs",
    "TERMS_OF_SERVICE_TEMPLATE.md",
  );

  let markdown = "";
  try {
    markdown = fs.readFileSync(templatePath, "utf8");
  } catch (error) {
    console.error("Failed to read terms of service template:", error);
    notFound();
  }

  const appName = process.env.NEXT_PUBLIC_APP_NAME || "GophDrive";
  const contactInfo =
    process.env.NEXT_PUBLIC_CONTACT_EMAIL || "the administrator";
  const jurisdiction =
    process.env.NEXT_PUBLIC_JURISDICTION || "Your Jurisdiction";
  const currentDate = new Date().toISOString().split("T")[0];

  markdown = markdown
    .replace(
      /> \*\*Note for hosters\*\*[\s\S]*?(?=\n\n|\n*$)/i,
      "<!-- Host note removed -->",
    )
    .replace(/\[Your Service Name\]/g, appName)
    .replace(/\[Your contact information\]/g, contactInfo)
    .replace(/\[Your Jurisdiction\]/g, jurisdiction)
    .replace(/\[Date\]/g, currentDate);

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
          href="/"
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
