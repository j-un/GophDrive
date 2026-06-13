import styles from "./Footer.module.css";

const privacyPolicyUrl = import.meta.env.VITE_PRIVACY_POLICY_URL || "";
const termsOfServiceUrl = import.meta.env.VITE_TERMS_OF_SERVICE_URL || "";

function isExternal(url: string): boolean {
  return url.startsWith("http://") || url.startsWith("https://");
}

export default function Footer() {
  if (!privacyPolicyUrl && !termsOfServiceUrl) {
    return null;
  }

  return (
    <footer className={styles.footer}>
      {privacyPolicyUrl && (
        <a
          href={privacyPolicyUrl}
          className={styles.link}
          {...(isExternal(privacyPolicyUrl) && {
            target: "_blank",
            rel: "noopener noreferrer",
          })}
        >
          Privacy Policy
        </a>
      )}
      {privacyPolicyUrl && termsOfServiceUrl && (
        <span className={styles.separator}>|</span>
      )}
      {termsOfServiceUrl && (
        <a
          href={termsOfServiceUrl}
          className={styles.link}
          {...(isExternal(termsOfServiceUrl) && {
            target: "_blank",
            rel: "noopener noreferrer",
          })}
        >
          Terms of Service
        </a>
      )}
    </footer>
  );
}
