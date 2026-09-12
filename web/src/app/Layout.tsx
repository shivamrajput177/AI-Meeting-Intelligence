import { Link, Outlet } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";

export function Layout() {
  const { auth, logout } = useAuth();

  return (
    <div style={{ maxWidth: 860, margin: "0 auto", padding: "0 20px" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 0", borderBottom: "1px solid #e5e7eb" }}>
        <Link to="/meetings" style={{ fontWeight: 700, fontSize: 18, textDecoration: "none", color: "#111827" }}>
          AI Meeting Intelligence
        </Link>
        {auth && (
          <button onClick={() => void logout()} style={{ background: "none", border: "1px solid #d1d5db", borderRadius: 6, padding: "6px 12px", cursor: "pointer" }}>
            Log out
          </button>
        )}
      </header>
      <main style={{ padding: "24px 0" }}>
        <Outlet />
      </main>
    </div>
  );
}
