import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { apiFetch } from "../api/client";

interface DailyViews {
  date: string;
  views: number;
  uniqueViews: number;
}

interface AnalyticsSummary {
  totalViews: number;
  uniqueViews: number;
  viewsToday: number;
  averageDailyViews: number;
  peakDay: string;
  peakDayViews: number;
  totalCtaClicks: number;
  ctaClickRate: number;
}

interface Milestones {
  reached25: number;
  reached50: number;
  reached75: number;
  reached100: number;
}

interface Viewer {
  email: string;
  firstViewedAt: string;
  viewCount: number;
  completion: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  daily: DailyViews[];
  milestones: Milestones;
  viewers: Viewer[];
}

type Range = "7d" | "30d" | "all";

function formatPeakDate(isoDate: string): string {
  if (!isoDate) return "";
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatChartDate(isoDate: string): string {
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

export function Analytics() {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [range, setRange] = useState<Range>("7d");
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<unknown>(null);

  const fetchAnalytics = useCallback(async (selectedRange: Range) => {
    setLoading(true);
    setError(false);
    try {
      const result = await apiFetch<AnalyticsData>(`/api/videos/${id}/analytics?range=${selectedRange}`);
      if (result) {
        setData(result);
      } else {
        setError(true);
      }
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchAnalytics(range);
  }, [range, fetchAnalytics]);

  useEffect(() => {
    if (!data || data.summary.totalViews === 0 || !canvasRef.current) return;

    let destroyed = false;

    async function renderChart() {
      const chartModule = await import("chart.js/auto");
      const Chart = chartModule.default;

      if (destroyed || !canvasRef.current) return;

      if (chartRef.current) {
        (chartRef.current as { destroy: () => void }).destroy();
      }

      chartRef.current = new Chart(canvasRef.current, {
        type: "bar",
        data: {
          labels: data!.daily.map((d) => formatChartDate(d.date)),
          datasets: [
            {
              label: "Views",
              data: data!.daily.map((d) => d.views),
              backgroundColor: "var(--color-accent)",
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            tooltip: {
              callbacks: {
                afterLabel: (context) => {
                  const dayData = data!.daily[context.dataIndex];
                  return `Unique: ${dayData.uniqueViews}`;
                },
              },
            },
          },
          scales: {
            y: {
              beginAtZero: true,
              ticks: {
                stepSize: 1,
              },
            },
          },
        },
      });
    }

    renderChart();

    return () => {
      destroyed = true;
      if (chartRef.current) {
        (chartRef.current as { destroy: () => void }).destroy();
        chartRef.current = null;
      }
    };
  }, [data]);

  function handleRangeChange(newRange: Range) {
    setRange(newRange);
  }

  if (loading) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Loading...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-error)", fontSize: 16 }}>Failed to load analytics.</p>
      </div>
    );
  }

  const ranges: Range[] = ["7d", "30d", "all"];

  return (
    <div className="page-container">
      <Link
        to="/library"
        style={{
          color: "var(--color-text-secondary)",
          textDecoration: "none",
          fontSize: 14,
          marginBottom: 16,
          display: "inline-block",
        }}
      >
        &larr; Library
      </Link>

      <div className="analytics-header">
        <h1 style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}>
          Analytics
        </h1>
        <div style={{ display: "flex", gap: 4 }}>
          {ranges.map((r) => (
            <button
              key={r}
              onClick={() => handleRangeChange(r)}
              style={{
                background: range === r ? "var(--color-accent)" : "transparent",
                color: range === r ? "#fff" : "var(--color-text-secondary)",
                border: `1px solid ${range === r ? "var(--color-accent)" : "var(--color-border)"}`,
                borderRadius: 4,
                padding: "6px 12px",
                fontSize: 13,
                fontWeight: 600,
                cursor: "pointer",
              }}
            >
              {r === "all" ? "All" : r}
            </button>
          ))}
        </div>
      </div>

      <div className="analytics-stats">
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Total Views</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.totalViews}
          </p>
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Unique Views</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.uniqueViews}
          </p>
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Avg / Day</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.averageDailyViews}
          </p>
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Peak Day</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.peakDayViews}
          </p>
          {data.summary.peakDay && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 12, margin: "4px 0 0" }}>
              {formatPeakDate(data.summary.peakDay)}
            </p>
          )}
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>CTA Clicks</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.totalCtaClicks}
          </p>
          {data.summary.totalViews > 0 && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 12, margin: "4px 0 0" }}>
              {(data.summary.ctaClickRate * 100).toFixed(1)}% click rate
            </p>
          )}
        </div>
      </div>

      {data.summary.totalViews > 0 && (
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            marginTop: 16,
          }}
        >
          <h3 style={{ color: "var(--color-text)", fontSize: 16, fontWeight: 600, margin: "0 0 12px" }}>
            Completion Funnel
          </h3>
          {[
            { label: "25%", value: data.milestones.reached25 },
            { label: "50%", value: data.milestones.reached50 },
            { label: "75%", value: data.milestones.reached75 },
            { label: "100%", value: data.milestones.reached100 },
          ].map((m) => (
            <div key={m.label} style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 13, width: 40, textAlign: "right" }}>
                {m.label}
              </span>
              <div style={{ flex: 1, background: "var(--color-border)", borderRadius: 4, height: 24, overflow: "hidden" }}>
                <div
                  style={{
                    width: `${(m.value / data.summary.totalViews) * 100}%`,
                    background: "var(--color-accent)",
                    height: "100%",
                    borderRadius: 4,
                    minWidth: m.value > 0 ? 2 : 0,
                  }}
                />
              </div>
              <span style={{ color: "var(--color-text)", fontSize: 13, fontWeight: 600, width: 50 }}>
                {m.value}
              </span>
              <span style={{ color: "var(--color-text-secondary)", fontSize: 12, width: 45 }}>
                {((m.value / data.summary.totalViews) * 100).toFixed(0)}%
              </span>
            </div>
          ))}
        </div>
      )}

      {data.viewers && data.viewers.length > 0 && (
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            marginTop: 16,
          }}
        >
          <h3 style={{ color: "var(--color-text)", fontSize: 16, fontWeight: 600, margin: "0 0 12px" }}>
            Viewers
          </h3>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <thead>
              <tr>
                <th style={{ textAlign: "left", color: "var(--color-text-secondary)", fontSize: 13, padding: "4px 8px 8px 0" }}>Email</th>
                <th style={{ textAlign: "left", color: "var(--color-text-secondary)", fontSize: 13, padding: "4px 8px 8px 0" }}>First viewed</th>
                <th style={{ textAlign: "right", color: "var(--color-text-secondary)", fontSize: 13, padding: "4px 0 8px 8px" }}>Views</th>
                <th style={{ textAlign: "right", color: "var(--color-text-secondary)", fontSize: 13, padding: "4px 0 8px 8px" }}>Completion</th>
              </tr>
            </thead>
            <tbody>
              {data.viewers.map((v) => (
                <tr key={v.email}>
                  <td style={{ color: "var(--color-text)", fontSize: 13, padding: "4px 8px 4px 0" }}>{v.email}</td>
                  <td style={{ color: "var(--color-text-secondary)", fontSize: 13, padding: "4px 8px 4px 0" }}>{new Date(v.firstViewedAt).toLocaleDateString()}</td>
                  <td style={{ textAlign: "right", color: "var(--color-text)", fontSize: 13, padding: "4px 0 4px 8px" }}>{v.viewCount}</td>
                  <td style={{ textAlign: "right", color: "var(--color-text)", fontSize: 13, padding: "4px 0 4px 8px" }}>{v.completion}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {data.summary.totalViews === 0 ? (
        <p style={{ color: "var(--color-text-secondary)", fontSize: 14, textAlign: "center", marginTop: 32 }}>
          No views in this period.
        </p>
      ) : (
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            height: 300,
          }}
        >
          <canvas ref={canvasRef} />
        </div>
      )}
    </div>
  );
}
