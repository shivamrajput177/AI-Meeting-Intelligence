import { request } from "./client";

export interface TokenPair {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export function signup(orgName: string, email: string, name: string, password: string) {
  return request<TokenPair>("POST", "/auth/signup", { orgName, email, name, password }, { auth: false });
}

export function login(email: string, password: string) {
  return request<TokenPair>("POST", "/auth/login", { email, password }, { auth: false });
}

export function refresh(orgId: string, refreshToken: string, role: string) {
  return request<TokenPair>("POST", "/auth/refresh", { orgId, refreshToken, role }, { auth: false });
}

export function logout(orgId: string, refreshToken: string) {
  return request<void>("POST", "/auth/logout", { orgId, refreshToken }, { auth: false });
}

export function requestPasswordReset(email: string) {
  return request<{ devToken?: string }>("POST", "/auth/password/reset-request", { email }, { auth: false });
}

export function confirmPasswordReset(token: string, newPassword: string) {
  return request<void>("POST", "/auth/password/reset-confirm", { token, newPassword }, { auth: false });
}
