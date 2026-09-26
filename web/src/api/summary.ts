import { request } from "./client";

export interface Summary {
  id: string;
  meetingId: string;
  summaryText: string;
  keyDecisions: string[];
  risks: string[];
  blockers: string[];
  modelUsed: string;
  promptVersion: string;
  createdAt: string;
}

export function getSummary(meetingId: string) {
  return request<Summary>("GET", `/meetings/${meetingId}/summary`);
}

export function regenerateSummary(meetingId: string) {
  return request<Summary>("POST", `/meetings/${meetingId}/summary/regenerate`);
}
