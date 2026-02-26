import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api/client";
import { useTheme } from "../hooks/useTheme";

interface DashboardSummary {
  totalViews: number;
  uniqueViews: number;
  avgDailyViews: number;
  totalVideos: number;
  totalWatchTimeSeconds: number;
}

interface DashboardDaily {
  date: string;
  views: number;
  uniqueViews: number;
}

interface DashboardTopVideo {
  id: string;
  title: string;
  views: number;
  uniqueViews: number;
  thumbnailUrl: string;
}

interface DashboardData {
  summary: DashboardSummary;
  daily: DashboardDaily[];
  topVideos: DashboardTopVideo[];
}

type Range = "7d" | "30d" | "90d" | "all";

function formatWatchTime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (remainingMinutes === 0) return `${hours}h`;
  return `${hours}h ${remainingMinutes}m`;
}

function formatChartDate(isoDate: string): string {
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

export function AnalyticsDashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [range, setRange] = useState<Range>("7d");
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<unknown>(null);
  const { resolvedTheme } = useTheme();

  const fetchDashboard = useCallback(async (selectedRange: Range) => {
    setLoading(true);
    setError(false);
    try {
      const result = await apiFetch<DashboardData>(`/api/analytics/dashboard?range=${selectedRange}`);
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
  }, []);

  useEffect(() => {
    fetchDashboard(range);
  }, [range, fetchDashboard]);

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
            legend: { display: false },
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
              ticks: { stepSize: 1, color: chartLabelColor },
              grid: { color: chartGridColor },
            },
            x: {
              ticks: { color: chartLabelColor },
              grid: { display: false },
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

  const ranges: Range[] = ["7d", "30d", "90d", "all"];
  const hasViews = data.summary.totalViews > 0;

  return (
    <div className="page-container">
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
          <p className="stat-card-value">{data.summary.avgDailyViews}</p>
        </div>
        <div className="stat-card">
          <p className="stat-card-label">Total Videos</p>
          <p className="stat-card-value">{data.summary.totalVideos}</p>
        </div>
        <div className="stat-card">
          <p className="stat-card-label">Watch Time</p>
          <p className="stat-card-value">{formatWatchTime(data.summary.totalWatchTimeSeconds)}</p>
        </div>
      </div>

      {hasViews && data.daily.length > 0 && (
        <div className="card" style={{ marginTop: 16, height: 300 }}>
          <canvas ref={canvasRef} />
        </div>
      )}

      {hasViews && data.topVideos.length > 0 && (
        <div className="card" style={{ marginTop: 16 }}>
          <h3 className="card-title">Top Videos</h3>
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {data.topVideos.map((video) => (
              <Link
                key={video.id}
                to={`/videos/${video.id}/analytics`}
                style={{ display: "flex", alignItems: "center", gap: 12, textDecoration: "none", color: "inherit" }}
              >
                <div
                  style={{
                    width: 80,
                    height: 45,
                    borderRadius: "var(--radius-sm)",
                    overflow: "hidden",
                    background: "var(--color-surface-raised)",
                    flexShrink: 0,
                  }}
                >
                  {video.thumbnailUrl && (
                    <img
                      src={video.thumbnailUrl}
                      alt=""
                      style={{ width: "100%", height: "100%", objectFit: "cover" }}
                    />
                  )}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <p style={{
                    color: "var(--color-text)",
                    fontSize: 14,
                    fontWeight: 500,
                    margin: 0,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}>
                    {video.title}
                  </p>
                  <p style={{ color: "var(--color-text-secondary)", fontSize: 12, margin: "2px 0 0" }}>
                    {video.views} views · {video.uniqueViews} unique
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      {!hasViews && (
        <p style={{ color: "var(--color-text-secondary)", fontSize: 14, textAlign: "center", marginTop: 32 }}>
          No analytics yet. Views will appear here once your videos are watched.
        </p>
      )}
    </div>
  );
}
