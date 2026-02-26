import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { AnalyticsDashboard } from "./AnalyticsDashboard";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function makeDashboardData(overrides: Record<string, unknown> = {}) {
  return {
    summary: {
      totalViews: 350,
      uniqueViews: 210,
      avgDailyViews: 50,
      totalVideos: 12,
      totalWatchTimeSeconds: 7200,
      ...(overrides.summary as Record<string, unknown> ?? {}),
    },
    daily: overrides.daily ?? [
      { date: "2026-02-20", views: 45, uniqueViews: 30 },
      { date: "2026-02-21", views: 55, uniqueViews: 35 },
      { date: "2026-02-22", views: 60, uniqueViews: 40 },
    ],
    topVideos: overrides.topVideos ?? [
      { id: "v1", title: "Intro Video", views: 120, uniqueViews: 80, thumbnailUrl: "/api/watch/abc/thumbnail" },
      { id: "v2", title: "Demo Recording", views: 90, uniqueViews: 60, thumbnailUrl: "" },
    ],
  };
}

function renderDashboard() {
  return render(
    <MemoryRouter initialEntries={["/analytics"]}>
      <AnalyticsDashboard />
    </MemoryRouter>
  );
}

describe("AnalyticsDashboard", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders summary stat cards with correct values", async () => {
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("350")).toBeInTheDocument();
    });
    expect(screen.getByText("210")).toBeInTheDocument();
    expect(screen.getByText("50")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("2h")).toBeInTheDocument();
  });

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    renderDashboard();

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("fetches dashboard with default 7d range", async () => {
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/analytics/dashboard?range=7d");
    });
  });

  it("switches range when clicking 30d button", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("350")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    await user.click(screen.getByRole("button", { name: "30d" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/analytics/dashboard?range=30d");
    });
  });

  it("switches range when clicking 90d button", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("350")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    await user.click(screen.getByRole("button", { name: "90d" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/analytics/dashboard?range=90d");
    });
  });

  it("shows error state when fetch fails", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("Network error"));
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Failed to load analytics.")).toBeInTheDocument();
    });
  });

  it("shows empty state when no views", async () => {
    mockApiFetch.mockResolvedValueOnce(
      makeDashboardData({
        summary: { totalViews: 0, uniqueViews: 0, avgDailyViews: 0, totalVideos: 5, totalWatchTimeSeconds: 0 },
        daily: [],
        topVideos: [],
      })
    );
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText(/No analytics yet/)).toBeInTheDocument();
    });
  });

  it("renders top videos with titles and view counts", async () => {
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Top Videos")).toBeInTheDocument();
    });
    expect(screen.getByText("Intro Video")).toBeInTheDocument();
    expect(screen.getByText("Demo Recording")).toBeInTheDocument();
    expect(screen.getByText("120 views · 80 unique")).toBeInTheDocument();
    expect(screen.getByText("90 views · 60 unique")).toBeInTheDocument();
  });

  it("links top videos to per-video analytics", async () => {
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("Intro Video")).toBeInTheDocument();
    });

    const links = screen.getAllByRole("link");
    const introLink = links.find((l) => l.getAttribute("href") === "/videos/v1/analytics");
    const demoLink = links.find((l) => l.getAttribute("href") === "/videos/v2/analytics");
    expect(introLink).toBeInTheDocument();
    expect(demoLink).toBeInTheDocument();
  });

  it("formats watch time in minutes", async () => {
    mockApiFetch.mockResolvedValueOnce(
      makeDashboardData({ summary: { totalWatchTimeSeconds: 300 } })
    );
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("5m")).toBeInTheDocument();
    });
  });

  it("formats watch time in hours and minutes", async () => {
    mockApiFetch.mockResolvedValueOnce(
      makeDashboardData({ summary: { totalWatchTimeSeconds: 5430 } })
    );
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("1h 30m")).toBeInTheDocument();
    });
  });

  it("formats watch time in seconds for short durations", async () => {
    mockApiFetch.mockResolvedValueOnce(
      makeDashboardData({ summary: { totalWatchTimeSeconds: 45 } })
    );
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("45s")).toBeInTheDocument();
    });
  });

  it("renders all four range pills", async () => {
    mockApiFetch.mockResolvedValueOnce(makeDashboardData());
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("350")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: "7d" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "30d" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "90d" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
  });

  it("hides top videos section when no top videos", async () => {
    mockApiFetch.mockResolvedValueOnce(
      makeDashboardData({ topVideos: [] })
    );
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText("350")).toBeInTheDocument();
    });
    expect(screen.queryByText("Top Videos")).not.toBeInTheDocument();
  });
});
