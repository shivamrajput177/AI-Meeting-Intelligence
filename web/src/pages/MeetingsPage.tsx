import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import * as meetingsApi from "../api/meetings";
import { StatusBadge } from "../components/StatusBadge";
import { ApiError } from "../api/client";

export function MeetingsPage() {
  const [meetings, setMeetings] = useState<meetingsApi.Meeting[]>([]);
  const [loading, setLoading] = useState(true);
  const [title, setTitle] = useState("");
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  async function refresh() {
    setLoading(true);
    try {
      const list = await meetingsApi.listMeetings();
      setMeetings(list.data ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load meetings");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  // The three-step upload flow this drives: create an upload intent (gets
  // a meeting id + a presigned MinIO URL) -> PUT the file bytes directly
  // to that URL -> confirm the upload so Meeting Service can verify the
  // object actually landed. See docs/architecture/microservices.md §5.
  async function onUpload(e: React.FormEvent) {
    e.preventDefault();
    const file = fileInput.current?.files?.[0];
    if (!file || !title.trim()) return;

    setError(null);
    setUploading(true);
    try {
      const intent = await meetingsApi.createUploadIntent(title.trim());
      await meetingsApi.uploadToPresignedUrl(intent.uploadUrl, file);
      await meetingsApi.confirmUpload(intent.meetingId);
      setTitle("");
      if (fileInput.current) fileInput.current.value = "";
      await refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Upload failed");
    } finally {
      setUploading(false);
    }
  }

  return (
    <div>
      <h1 style={{ fontSize: 22 }}>Upload a meeting</h1>
      <form onSubmit={onUpload} style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
        <input placeholder="Meeting title" value={title} onChange={(e) => setTitle(e.target.value)} required style={{ flex: 1, minWidth: 200 }} />
        <input type="file" accept="audio/*,video/*" ref={fileInput} required />
        <button type="submit" disabled={uploading}>
          {uploading ? "Uploading…" : "Upload"}
        </button>
      </form>
      {error && <p style={{ color: "#b91c1c", fontSize: 14, marginTop: 8 }}>{error}</p>}

      <h2 style={{ fontSize: 18, marginTop: 32 }}>Your meetings</h2>
      {loading ? (
        <p>Loading…</p>
      ) : meetings.length === 0 ? (
        <p style={{ color: "#6b7280" }}>No meetings yet — upload one above.</p>
      ) : (
        <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 12 }}>
          <thead>
            <tr style={{ textAlign: "left", borderBottom: "1px solid #e5e7eb" }}>
              <th style={{ padding: "8px 4px" }}>Title</th>
              <th style={{ padding: "8px 4px" }}>Status</th>
              <th style={{ padding: "8px 4px" }}>Uploaded</th>
            </tr>
          </thead>
          <tbody>
            {meetings.map((m) => (
              <tr key={m.id} style={{ borderBottom: "1px solid #f3f4f6" }}>
                <td style={{ padding: "8px 4px" }}>
                  <Link to={`/meetings/${m.id}`}>{m.title}</Link>
                </td>
                <td style={{ padding: "8px 4px" }}>
                  <StatusBadge status={m.status} />
                </td>
                <td style={{ padding: "8px 4px", color: "#6b7280" }}>{new Date(m.createdAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
