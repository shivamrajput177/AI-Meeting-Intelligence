import { request } from "./client";

export interface TranscriptSegment {
  speakerLabel: string;
  startMs: number;
  endMs: number;
  text: string;
  confidence?: number;
}

export interface Transcript {
  id: string;
  meetingId: string;
  language: string;
  engine: string;
  modelName: string;
  status: string;
  rawText: string;
  wordCount: number;
  segments: TranscriptSegment[];
  createdAt: string;
}

export function getTranscript(meetingId: string) {
  return request<Transcript>("GET", `/meetings/${meetingId}/transcript`);
}
