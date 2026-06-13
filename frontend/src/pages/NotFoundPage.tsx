import { Link } from "react-router";

export default function NotFoundPage() {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        height: "100vh",
        gap: "1rem",
        color: "var(--foreground)",
        background: "var(--background)",
      }}
    >
      <h1 style={{ fontSize: "3rem", fontWeight: "bold", opacity: 0.3 }}>
        404
      </h1>
      <p style={{ opacity: 0.6 }}>Page not found</p>
      <Link to="/" className="btn btn-primary">
        Go Home
      </Link>
    </div>
  );
}
