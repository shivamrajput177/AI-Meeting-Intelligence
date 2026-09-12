import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import * as meetingsApi from "../api/meetings";
import { StatusBadge } from "../components/StatusBadge";
import { ApiError } from "../api/client";

// Phase 1 has no transcript/summary yet (that's Phase 2's Kafka-driven
// pipeline) — this page is deliberately just metadata + status, matching
// what Meeting Service actually has to show at this stage. See
// docs/ROADMAP.md Phase 1's scope.
export function MeetingDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [meeting, setMeeting] = useState<meetingsApi.Meeting | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    meetingsApi
      .getMeeting(id)
      .then(setMeeting)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load meeting"));
  }, [id]);

  if (error) return <p style={{ color: "#b91c1c" }}>{error}</p>;
  if (!meeting) return <p>Loading…</p>;

  return (
    <div>
      <Link to="/meetings" style={{ fontSize: 14 }}>
        ← Back to meetings
      </Link>
      <h1 style={{ fontSize: 22, marginTop: 8 }}>{meeting.title}</h1>
      <div style={{ marginTop: 8 }}>
        <StatusBadge status={meeting.status} />
      </div>
      <dl style={{ marginTop: 20, display: "grid", gridTemplateColumns: "140px 1fr", rowGap: 8 }}>
        <dt style={{ color: "#6b7280" }}>Source</dt>
        <dd>{meeting.sourceType}</dd>
        <dt style={{ color: "#6b7280" }}>Uploaded</dt>
        <dd>{new Date(meeting.createdAt).toLocaleString()}</dd>
        <dt style={{ color: "#6b7280" }}>Last updated</dt>
        <dd>{new Date(meeting.updatedAt).toLocaleString()}</dd>
      </dl>
      <p style={{ marginTop: 24, color: "#6b7280", fontSize: 14 }}>
        Transcript and summary appear here starting Phase 2, once the transcription and summarization pipeline is wired up.
      </p>
    </div>
  );
}
