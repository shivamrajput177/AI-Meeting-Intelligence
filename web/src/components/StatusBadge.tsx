const COLORS: Record<string, string> = {
  uploaded: "#6b7280",
  transcribing: "#b45309",
  transcribed: "#b45309",
  summarizing: "#b45309",
  summarized: "#1d4ed8",
  completed: "#15803d",
  failed: "#b91c1c",
};

export function StatusBadge({ status }: { status: string }) {
  const color = COLORS[status] ?? "#6b7280";
  return (
    <span
      style={{
        display: "inline-block",
        padding: "2px 10px",
        borderRadius: 999,
        fontSize: 12,
        fontWeight: 600,
        color: "#fff",
        background: color,
        textTransform: "capitalize",
      }}
    >
      {status}
    </span>
  );
}
