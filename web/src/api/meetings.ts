import { request } from "./client";

export interface Meeting {
  id: string;
  orgId: string;
  title: string;
  createdBy: string;
  status: string;
  sourceType: string;
  durationSeconds?: number;
  startedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateMeetingResponse {
  meetingId: string;
  uploadUrl: string;
}

export function createUploadIntent(title: string) {
  return request<CreateMeetingResponse>("POST", "/meetings", { title });
}

// Uploads directly to the presigned MinIO URL — never through the
// gateway/Meeting Service, per docs/architecture/microservices.md §5.
export async function uploadToPresignedUrl(uploadUrl: string, file: File): Promise<void> {
  const res = await fetch(uploadUrl, { method: "PUT", body: file });
  if (!res.ok) {
    throw new Error(`Upload failed: ${res.status}`);
  }
}

export function confirmUpload(meetingId: string) {
  return request<Meeting>("POST", `/meetings/${meetingId}/complete-upload`);
}

export function getMeeting(id: string) {
  return request<Meeting>("GET", `/meetings/${id}`);
}

export interface MeetingList {
  data: Meeting[];
  page: number;
  pageSize: number;
  total: number;
}

export function listMeetings(page = 1, pageSize = 20) {
  return request<MeetingList>("GET", `/meetings?page=${page}&pageSize=${pageSize}`);
}
