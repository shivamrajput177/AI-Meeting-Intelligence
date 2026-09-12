import { request } from "./client";

export interface User {
  id: string;
  orgId: string;
  email: string;
  name: string;
  role: string;
  status: string;
  avatarUrl: string;
  createdAt: string;
}

export function getMe() {
  return request<User>("GET", "/users/me");
}

export function updateMe(name: string, avatarUrl?: string) {
  return request<User>("PATCH", "/users/me", { name, avatarUrl });
}
