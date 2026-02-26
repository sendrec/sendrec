import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { apiFetch } from "../api/client";
import { useTheme } from "../hooks/useTheme";

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

interface SegmentData {
  segment: number;
  watchCount: number;
  intensity: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  daily: DailyViews[];
  milestones: Milestones;
  viewers: Viewer[];
  heatmap: SegmentData[];
}

type Range = "7d" | "30d" | "all";

function formatPeakDate(isoDate: string): string {
  if (!isoDate) return "";
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

function formatChartDate(isoDate: string): string {
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

export function Analytics() {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [range, setRange] = useState<Range>("7d");
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<unknown>(null);
  const { resolvedTheme } = useTheme();

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

      const styles = getComputedStyle(document.documentElement);
      const accentColor = styles.getPropertyValue("--color-accent").trim();
      const chartLabelColor = styles.getPropertyValue("--color-chart-label").trim();
      const chartGridColor = styles.getPropertyValue("--color-chart-grid").trim();

      chartRef.current = new Chart(canvasRef.current, {
        type: "bar",
        data: {
          labels: data!.daily.map((d) => formatChartDate(d.date)),
          datasets: [
            {
              label: "Views",
              data: data!.daily.map((d) => d.views),
              backgroundColor: accentColor,
              borderRadius: 3,
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: {
              display: false,
            },
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
                color: chartLabelColor,
              },
              grid: {
                color: chartGridColor,
              },
            },
            x: {
              ticks: {
                color: chartLabelColor,
              },
              grid: {
                display: false,
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
  }, [data, resolvedTheme]);

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
      <Link to={`/videos/${id}`} className="back-link">
        &larr; Back
      </Link>

      <div className="analytics-header">
        <h1 style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}>
          Analytics
        </h1>
        <div className="range-pills">
          {ranges.map((r) => (
            <button
              key={r}
              className={`range-pill${range === r ? " range-pill--active" : ""}`}
              onClick={() => handleRangeChange(r)}
            >
              {r === "all" ? "All" : r}
            </button>
          ))}
        </div>
      </div>

      <div className="analytics-stats">
        <div className="stat-card">
          <p className="stat-card-label">Total Views</p>
          <p className="stat-card-value">{data.summary.totalViews}</p>
        </div>
        <div className="stat-card">
          <p className="stat-card-label">Unique Views</p>
          <p className="stat-card-value">{data.summary.uniqueViews}</p>
        </div>
        <div className="stat-card">
          <p className="stat-card-label">Avg / Day</p>
          <p className="stat-card-value">{data.summary.averageDailyViews}</p>
        </div>
        <div className="stat-card">
          <p className="stat-card-label">Peak Day</p>
          <p className="stat-card-value">{data.summary.peakDayViews}</p>
          {data.summary.peakDay && (
            <p className="stat-card-detail">{formatPeakDate(data.summary.peakDay)}</p>
          )}
        </div>
        <div className="stat-card">
          <p className="stat-card-label">CTA Clicks</p>
          <p className="stat-card-value">{data.summary.totalCtaClicks}</p>
          {data.summary.totalViews > 0 && (
            <p className="stat-card-detail">
              {(data.summary.ctaClickRate * 100).toFixed(1)}% click rate
            </p>
          )}
        </div>
      </div>

      {data.summary.totalViews > 0 && data.heatmap && data.heatmap.length > 0 && (
        <div className="card" style={{ marginTop: 16 }}>
          <h3 className="card-title">Engagement</h3>
          <div style={{ display: "flex", gap: 1, height: 40, borderRadius: 4, overflow: "hidden" }}>
            {Array.from({ length: 50 }, (_, i) => {
              const seg = data.heatmap.find((s) => s.segment === i);
              const intensity = seg ? seg.intensity : 0;
              return (
                <div
                  key={i}
                  title={`${i * 2}%-${(i + 1) * 2}%: ${seg ? seg.watchCount : 0} views`}
                  style={{
                    flex: 1,
                    background: "var(--color-accent)",
                    opacity: Math.max(intensity, 0.08),
                  }}
                />
              );
            })}
          </div>
          <div style={{ display: "flex", justifyContent: "space-between", marginTop: 4 }}>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 11 }}>0%</span>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 11 }}>50%</span>
            <span style={{ color: "var(--color-text-secondary)", fontSize: 11 }}>100%</span>
          </div>
        </div>
      )}

      {data.summary.totalViews > 0 && (
        <div className="card" style={{ marginTop: 16 }}>
          <h3 className="card-title">Completion Funnel</h3>
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
        <div className="card" style={{ marginTop: 16 }}>
          <h3 className="card-title">Viewers</h3>
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
                  <td style={{ color: "var(--color-text-secondary)", fontSize: 13, padding: "4px 8px 4px 0" }}>{new Date(v.firstViewedAt).toLocaleDateString("en-GB")}</td>
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
        <div className="card" style={{ marginTop: 16, height: 300 }}>
          <canvas ref={canvasRef} />
        </div>
      )}
    </div>
  );
}
