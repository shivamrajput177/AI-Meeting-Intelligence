import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import * as authApi from "../api/auth";

interface JwtPayload {
  sub: string;
  org_id: string;
  role: string;
  exp: number;
}

// Decodes the access token's payload client-side purely to read
// org_id/role/user_id for use in subsequent calls (refresh, logout) — this
// is NOT a verification step (no signature check happens here); the
// server re-verifies the token's signature on every request regardless,
// per docs/architecture/microservices.md's auth design.
function decodeJwt(token: string): JwtPayload | null {
  try {
    const [, payload] = token.split(".");
    return JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
  } catch {
    return null;
  }
}

interface AuthState {
  userId: string;
  orgId: string;
  role: string;
}

interface AuthContextValue {
  auth: AuthState | null;
  signup: (orgName: string, email: string, name: string, password: string) => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function storeTokens(pair: authApi.TokenPair): AuthState | null {
  localStorage.setItem("accessToken", pair.accessToken);
  localStorage.setItem("refreshToken", pair.refreshToken);
  const claims = decodeJwt(pair.accessToken);
  if (!claims) return null;
  const state: AuthState = { userId: claims.sub, orgId: claims.org_id, role: claims.role };
  localStorage.setItem("orgId", state.orgId);
  localStorage.setItem("role", state.role);
  return state;
}

function loadStoredAuth(): AuthState | null {
  const accessToken = localStorage.getItem("accessToken");
  const orgId = localStorage.getItem("orgId");
  const role = localStorage.getItem("role");
  if (!accessToken || !orgId || !role) return null;
  const claims = decodeJwt(accessToken);
  if (!claims) return null;
  return { userId: claims.sub, orgId, role };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState<AuthState | null>(() => loadStoredAuth());

  useEffect(() => {
    // A stale (expired) token found on load isn't silently trusted — a
    // 401 from the first authenticated call will surface it. Phase 1
    // doesn't implement transparent refresh-on-401 (a reasonable
    // near-term addition once there's a second page that needs it).
  }, []);

  async function signup(orgName: string, email: string, name: string, password: string) {
    const pair = await authApi.signup(orgName, email, name, password);
    setAuth(storeTokens(pair));
  }

  async function login(email: string, password: string) {
    const pair = await authApi.login(email, password);
    setAuth(storeTokens(pair));
  }

  async function logout() {
    const refreshToken = localStorage.getItem("refreshToken");
    const orgId = localStorage.getItem("orgId");
    if (refreshToken && orgId) {
      try {
        await authApi.logout(orgId, refreshToken);
      } catch {
        // Logout is best-effort client-side too — clear local state
        // regardless of whether the revoke call succeeded.
      }
    }
    localStorage.removeItem("accessToken");
    localStorage.removeItem("refreshToken");
    localStorage.removeItem("orgId");
    localStorage.removeItem("role");
    setAuth(null);
  }

  return <AuthContext.Provider value={{ auth, signup, login, logout }}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}
