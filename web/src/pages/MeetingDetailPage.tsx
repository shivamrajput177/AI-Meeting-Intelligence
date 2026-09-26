import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import * as meetingsApi from "../api/meetings";
import * as transcriptApi from "../api/transcript";
import * as summaryApi from "../api/summary";
import { StatusBadge } from "../components/StatusBadge";
import { ApiError } from "../api/client";

function formatTimestamp(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

export function MeetingDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [meeting, setMeeting] = useState<meetingsApi.Meeting | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [transcript, setTranscript] = useState<transcriptApi.Transcript | null>(null);
  const [transcriptLoading, setTranscriptLoading] = useState(true);
  const [transcriptReady, setTranscriptReady] = useState(true);
  const [transcriptError, setTranscriptError] = useState<string | null>(null);

  const [summary, setSummary] = useState<summaryApi.Summary | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(true);
  const [summaryReady, setSummaryReady] = useState(true);
  const [summaryError, setSummaryError] = useState<string | null>(null);
  const [regenerating, setRegenerating] = useState(false);

  const loadTranscript = useCallback(async () => {
    if (!id) return;
    setTranscriptLoading(true);
    setTranscriptError(null);
    try {
      setTranscript(await transcriptApi.getTranscript(id));
      setTranscriptReady(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setTranscriptReady(false);
      } else {
        setTranscriptError(err instanceof ApiError ? err.message : "Failed to load transcript");
      }
    } finally {
      setTranscriptLoading(false);
    }
  }, [id]);

  const loadSummary = useCallback(async () => {
    if (!id) return;
    setSummaryLoading(true);
    setSummaryError(null);
    try {
      setSummary(await summaryApi.getSummary(id));
      setSummaryReady(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setSummaryReady(false);
      } else {
        setSummaryError(err instanceof ApiError ? err.message : "Failed to load summary");
      }
    } finally {
      setSummaryLoading(false);
    }
  }, [id]);

  useEffect(() => {
    if (!id) return;
    meetingsApi
      .getMeeting(id)
      .then(setMeeting)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load meeting"));
  }, [id]);

  useEffect(() => {
    void loadTranscript();
  }, [loadTranscript]);

  useEffect(() => {
    void loadSummary();
  }, [loadSummary]);

  async function onGenerateSummary() {
    if (!id) return;
    setRegenerating(true);
    setSummaryError(null);
    try {
      setSummary(await summaryApi.regenerateSummary(id));
      setSummaryReady(true);
    } catch (err) {
      setSummaryError(err instanceof ApiError ? err.message : "Failed to generate summary");
    } finally {
      setRegenerating(false);
    }
  }

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

      <div style={{ marginTop: 32, display: "flex", alignItems: "center", gap: 12 }}>
        <h2 style={{ fontSize: 18 }}>Transcript</h2>
        <button onClick={() => void loadTranscript()} disabled={transcriptLoading}>
          Refresh
        </button>
      </div>
      {transcriptLoading ? (
        <p style={{ color: "#6b7280", fontSize: 14 }}>Loading…</p>
      ) : transcriptError ? (
        <p style={{ color: "#b91c1c", fontSize: 14 }}>{transcriptError}</p>
      ) : !transcriptReady ? (
        <p style={{ color: "#6b7280", fontSize: 14 }}>Not ready yet — still processing.</p>
      ) : transcript && transcript.segments.length > 0 ? (
        <ol style={{ marginTop: 12, padding: 0, listStyle: "none" }}>
          {transcript.segments.map((s, i) => (
            <li key={i} style={{ display: "flex", gap: 12, padding: "6px 0", borderBottom: "1px solid #f3f4f6" }}>
              <span style={{ color: "#6b7280", fontSize: 12, minWidth: 48 }}>{formatTimestamp(s.startMs)}</span>
              <span style={{ fontWeight: 600, fontSize: 13, minWidth: 90 }}>{s.speakerLabel}</span>
              <span style={{ fontSize: 14 }}>{s.text}</span>
            </li>
          ))}
        </ol>
      ) : (
        <p style={{ marginTop: 12, fontSize: 14, whiteSpace: "pre-wrap" }}>{transcript?.rawText}</p>
      )}

      <div style={{ marginTop: 32, display: "flex", alignItems: "center", gap: 12 }}>
        <h2 style={{ fontSize: 18 }}>Summary</h2>
        <button onClick={() => void loadSummary()} disabled={summaryLoading}>
          Refresh
        </button>
        {summaryReady && summary && (
          <button onClick={onGenerateSummary} disabled={regenerating}>
            {regenerating ? "Regenerating…" : "Regenerate"}
          </button>
        )}
      </div>
      {summaryLoading ? (
        <p style={{ color: "#6b7280", fontSize: 14 }}>Loading…</p>
      ) : summaryError ? (
        <p style={{ color: "#b91c1c", fontSize: 14 }}>{summaryError}</p>
      ) : !summaryReady ? (
        <div>
          <p style={{ color: "#6b7280", fontSize: 14 }}>
            {transcriptReady ? "Not generated yet." : "Not ready yet — the transcript has to finish first."}
          </p>
          {transcriptReady && (
            <button onClick={onGenerateSummary} disabled={regenerating} style={{ marginTop: 8 }}>
              {regenerating ? "Generating…" : "Generate summary"}
            </button>
          )}
        </div>
      ) : (
        summary && (
          <div style={{ marginTop: 12 }}>
            <p style={{ fontSize: 14 }}>{summary.summaryText}</p>
            {summary.keyDecisions.length > 0 && (
              <>
                <h3 style={{ fontSize: 14, marginTop: 16 }}>Key decisions</h3>
                <ul style={{ fontSize: 14 }}>
                  {summary.keyDecisions.map((d, i) => (
                    <li key={i}>{d}</li>
                  ))}
                </ul>
              </>
            )}
            {summary.risks.length > 0 && (
              <>
                <h3 style={{ fontSize: 14, marginTop: 16 }}>Risks</h3>
                <ul style={{ fontSize: 14 }}>
                  {summary.risks.map((r, i) => (
                    <li key={i}>{r}</li>
                  ))}
                </ul>
              </>
            )}
            {summary.blockers.length > 0 && (
              <>
                <h3 style={{ fontSize: 14, marginTop: 16 }}>Blockers</h3>
                <ul style={{ fontSize: 14 }}>
                  {summary.blockers.map((b, i) => (
                    <li key={i}>{b}</li>
                  ))}
                </ul>
              </>
            )}
          </div>
        )
      )}
    </div>
  );
}
