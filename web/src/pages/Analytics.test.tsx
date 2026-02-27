import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { Analytics } from "./Analytics";
import { expectNoA11yViolations } from "../test-utils/a11y";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
  getAccessToken: () => "test-token",
}));

function makeVideoAnalyticsData(overrides: Record<string, unknown> = {}) {
  return {
    summary: {
      totalViews: 142,
      uniqueViews: 98,
      viewsToday: 12,
      averageDailyViews: 4.7,
      peakDay: "2026-02-10",
      peakDayViews: 23,
      totalCtaClicks: 5,
      ctaClickRate: 0.25,
      ...(overrides.summary as Record<string, unknown> ?? {}),
    },
    daily: overrides.daily ?? [
      { date: "2026-02-08", views: 15, uniqueViews: 10 },
      { date: "2026-02-09", views: 20, uniqueViews: 14 },
      { date: "2026-02-10", views: 23, uniqueViews: 18 },
    ],
    milestones: {
      reached25: 80,
      reached50: 60,
      reached75: 40,
      reached100: 25,
      ...(overrides.milestones as Record<string, unknown> ?? {}),
    },
    viewers: overrides.viewers ?? [
      { email: "alice@example.com", firstViewedAt: "2026-02-10T14:30:00Z", viewCount: 3, completion: 82, watchTimeSeconds: 180, country: "DE", city: "Berlin" },
      { email: "bob@example.com", firstViewedAt: "2026-02-12T09:15:00Z", viewCount: 1, completion: 41, watchTimeSeconds: 90, country: "US", city: "New York" },
    ],
    heatmap: "heatmap" in overrides ? (overrides.heatmap as unknown[] | null) : [
      { segment: 0, watchCount: 10, intensity: 1.0 },
      { segment: 1, watchCount: 8, intensity: 0.8 },
      { segment: 24, watchCount: 5, intensity: 0.5 },
      { segment: 49, watchCount: 2, intensity: 0.2 },
    ],
    trends: overrides.trends !== undefined ? overrides.trends : {
      views: 15.5,
      uniqueViews: 12.0,
      avgWatchTime: -5.2,
      completionRate: 8.3,
    },
    referrers: overrides.referrers ?? [
      { source: "Direct", count: 50, percentage: 45.5 },
      { source: "Email", count: 30, percentage: 27.3 },
      { source: "Slack", count: 20, percentage: 18.2 },
    ],
    browsers: overrides.browsers ?? [
      { name: "Chrome", percentage: 65 },
      { name: "Safari", percentage: 25 },
      { name: "Firefox", percentage: 10 },
    ],
    devices: overrides.devices ?? [
      { name: "Desktop", percentage: 80 },
      { name: "Mobile", percentage: 15 },
      { name: "Tablet", percentage: 5 },
    ],
  };
}

function makeDashboardData(overrides: Record<string, unknown> = {}) {
  return {
    summary: {
      totalViews: 350,
      uniqueViews: 210,
      avgDailyViews: 50,
      totalVideos: 12,
      totalWatchTimeSeconds: 7200,
      avgCompletion: 62,
      ...(overrides.summary as Record<string, unknown> ?? {}),
    },
    daily: overrides.daily ?? [
      { date: "2026-02-20", views: 45, uniqueViews: 30 },
      { date: "2026-02-21", views: 55, uniqueViews: 35 },
      { date: "2026-02-22", views: 60, uniqueViews: 40 },
    ],
    topVideos: overrides.topVideos ?? [
      { id: "v1", title: "Intro Video", views: 120, uniqueViews: 80, thumbnailUrl: "/api/watch/abc/thumbnail", completion: 75 },
      { id: "v2", title: "Demo Recording", views: 90, uniqueViews: 60, thumbnailUrl: "", completion: 55 },
    ],
  };
}

function renderVideoAnalytics(videoId = "v1") {
  return render(
    <MemoryRouter initialEntries={[`/videos/${videoId}/analytics`]}>
      <Routes>
        <Route path="/videos/:id/analytics" element={<Analytics />} />
        <Route path="/analytics" element={<Analytics />} />
      </Routes>
    </MemoryRouter>
  );
}

function renderDashboard() {
  return render(
    <MemoryRouter initialEntries={["/analytics"]}>
      <Routes>
        <Route path="/analytics" element={<Analytics />} />
        <Route path="/videos/:id/analytics" element={<Analytics />} />
      </Routes>
    </MemoryRouter>
  );
}

describe("Analytics", () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("has no accessibility violations", async () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    const { container } = renderVideoAnalytics();
    await expectNoA11yViolations(container);
  });

  describe("Video analytics view", () => {
    it("renders summary stat cards with correct values", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      const { container } = renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });
      expect(screen.getByText("98")).toBeInTheDocument();
      expect(screen.getByText("4.7")).toBeInTheDocument();

      const statValues = container.querySelectorAll(".stat-card-value");
      const peakDayCard = Array.from(statValues).find((el) => el.textContent === "23");
      expect(peakDayCard).not.toBeNull();
    });

    it("shows skeleton loading state initially", () => {
      mockApiFetch.mockReturnValue(new Promise(() => {}));
      const { container } = renderVideoAnalytics();

      const skeletonCards = container.querySelectorAll(".skeleton-stat");
      expect(skeletonCards.length).toBeGreaterThan(0);
    });

    it("fetches video analytics with default 7d range", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/analytics?range=7d");
      });
    });

    it("switches range when clicking 30d button", async () => {
      const user = userEvent.setup();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      await user.click(screen.getByRole("button", { name: "30d" }));

      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/analytics?range=30d");
      });
    });

    it("switches range when clicking 90d button", async () => {
      const user = userEvent.setup();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      await user.click(screen.getByRole("button", { name: "90d" }));

      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/analytics?range=90d");
      });
    });

    it("shows empty state when no views", async () => {
      mockApiFetch.mockResolvedValueOnce(
        makeVideoAnalyticsData({
          summary: { totalViews: 0, uniqueViews: 0, viewsToday: 0, averageDailyViews: 0, peakDay: "", peakDayViews: 0, totalCtaClicks: 0, ctaClickRate: 0 },
          daily: [],
          heatmap: [],
          viewers: [],
          referrers: [],
          browsers: [],
          devices: [],
        })
      );
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("No views in this period")).toBeInTheDocument();
      });
    });

    it("shows error state when fetch fails", async () => {
      mockApiFetch.mockRejectedValueOnce(new Error("Network error"));
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Failed to load analytics.")).toBeInTheDocument();
      });
    });

    it("has back link to video detail", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      const backLink = screen.getByRole("link", { name: /Back/ });
      expect(backLink).toHaveAttribute("href", "/videos/v1");
    });

    it("displays completion funnel with milestone data", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({
        summary: { totalViews: 100 },
        milestones: { reached25: 80, reached50: 60, reached75: 40, reached100: 25 },
      }));
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Completion Funnel")).toBeInTheDocument();
      });
      expect(screen.getByText("80")).toBeInTheDocument();
      expect(screen.getByText("60")).toBeInTheDocument();
      expect(screen.getByText("40")).toBeInTheDocument();
      expect(screen.getByText("25")).toBeInTheDocument();
    });

    it("hides funnel when no views", async () => {
      mockApiFetch.mockResolvedValueOnce(
        makeVideoAnalyticsData({
          summary: { totalViews: 0, uniqueViews: 0, viewsToday: 0, averageDailyViews: 0, peakDay: "", peakDayViews: 0, totalCtaClicks: 0, ctaClickRate: 0 },
          daily: [],
          heatmap: [],
          viewers: [],
          referrers: [],
          browsers: [],
          devices: [],
        })
      );
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("No views in this period")).toBeInTheDocument();
      });
      expect(screen.queryByText("Completion Funnel")).not.toBeInTheDocument();
    });

    it("displays viewers table", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Viewer Activity")).toBeInTheDocument();
      });
      expect(screen.getByText("alice@example.com")).toBeInTheDocument();
      expect(screen.getByText("bob@example.com")).toBeInTheDocument();
    });

    it("hides viewers when empty", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({
        viewers: [],
      }));
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });
      expect(screen.queryByText("Viewer Activity")).not.toBeInTheDocument();
    });

    it("renders heatmap", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      const { container } = renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Viewer Retention")).toBeInTheDocument();
      });

      const segments = container.querySelectorAll(".heatmap-segment");
      expect(segments.length).toBe(50);
    });

    it("does not crash when heatmap is null", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({
        heatmap: null,
      }));
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });
      expect(screen.queryByText("Viewer Retention")).not.toBeInTheDocument();
    });

    it("displays referrers section", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Top Referrers")).toBeInTheDocument();
      });
      expect(screen.getByText("Direct")).toBeInTheDocument();
      expect(screen.getByText("Email")).toBeInTheDocument();
    });

    it("displays browsers breakdown", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Chrome")).toBeInTheDocument();
      });
      expect(screen.getByText("Safari")).toBeInTheDocument();
    });

    it("displays devices breakdown", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Desktop")).toBeInTheDocument();
      });
      expect(screen.getByText("Mobile")).toBeInTheDocument();
    });

    it("shows trend indicators", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      const { container } = renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      const trendElements = container.querySelectorAll(".stat-card-trend");
      expect(trendElements.length).toBeGreaterThan(0);

      const trendUp = container.querySelector(".stat-card-trend--up");
      expect(trendUp).not.toBeNull();
    });

    it("hides trends for all range", async () => {
      const user = userEvent.setup();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({ trends: null }));
      await user.click(screen.getByRole("button", { name: "All" }));

      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/analytics?range=all");
      });

      const { container } = renderVideoAnalytics();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({ trends: null }));

      await waitFor(() => {
        expect(screen.queryAllByText("142").length).toBeGreaterThan(0);
      });

      const trendElements = container.querySelectorAll(".stat-card-trend");
      expect(trendElements.length).toBe(0);
    });

    it("sorts viewer table by column click", async () => {
      const viewers = Array.from({ length: 10 }, (_, i) => ({
        email: `viewer${String(i).padStart(2, "0")}@example.com`,
        firstViewedAt: `2026-02-${String(10 + i).padStart(2, "0")}T10:00:00Z`,
        viewCount: 1,
        completion: (i + 1) * 10,
        watchTimeSeconds: (i + 1) * 60,
        country: "DE",
        city: "Berlin",
      }));

      const user = userEvent.setup();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({ viewers }));
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Viewer Activity")).toBeInTheDocument();
      });

      const rows = screen.getAllByRole("row");
      const firstDataCellBefore = rows[1].querySelector("td");
      expect(firstDataCellBefore?.textContent).toBe("viewer09@example.com");

      const completionHeader = screen.getByRole("columnheader", { name: /Completion/ });
      await user.click(completionHeader);

      const rowsAfter = screen.getAllByRole("row");
      const firstDataCellAfter = rowsAfter[1].querySelector("td");
      expect(firstDataCellAfter?.textContent).toBe("viewer09@example.com");

      await user.click(completionHeader);

      const rowsAsc = screen.getAllByRole("row");
      const firstDataCellAsc = rowsAsc[1].querySelector("td");
      expect(firstDataCellAsc?.textContent).toBe("viewer00@example.com");
    });

    it("shows 'Show N more viewers' button", async () => {
      const viewers = Array.from({ length: 10 }, (_, i) => ({
        email: `viewer${String(i).padStart(2, "0")}@example.com`,
        firstViewedAt: `2026-02-${String(10 + i).padStart(2, "0")}T10:00:00Z`,
        viewCount: 1,
        completion: 50,
        watchTimeSeconds: 120,
        country: "DE",
        city: "Berlin",
      }));

      const user = userEvent.setup();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData({ viewers }));
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Viewer Activity")).toBeInTheDocument();
      });

      const showMoreBtn = screen.getByText("Show 3 more viewers");
      expect(showMoreBtn).toBeInTheDocument();

      await user.click(showMoreBtn);

      expect(screen.queryByText(/Show .* more viewer/)).not.toBeInTheDocument();
      expect(screen.getByText("viewer09@example.com")).toBeInTheDocument();
    });

    it("renders CSS bar chart not canvas", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      const { container } = renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("Views Over Time")).toBeInTheDocument();
      });

      expect(container.querySelector("canvas")).toBeNull();
      const bars = container.querySelectorAll(".analytics-chart-bar");
      expect(bars.length).toBeGreaterThan(0);
    });

    it("displays CTA clicks card", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("CTA Clicks")).toBeInTheDocument();
      });
      expect(screen.getByText("5")).toBeInTheDocument();
      expect(screen.getByText("25.0% click rate")).toBeInTheDocument();
    });

    it("renders all four range pills", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      expect(screen.getByRole("button", { name: "7d" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "30d" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "90d" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
    });

    it("has export CSV button", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      expect(screen.getByText("Export CSV")).toBeInTheDocument();
    });
  });

  describe("Dashboard view", () => {
    it("renders dashboard stat cards", async () => {
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
          summary: { totalViews: 0, uniqueViews: 0, avgDailyViews: 0, totalVideos: 5, totalWatchTimeSeconds: 0, avgCompletion: 0 },
          daily: [],
          topVideos: [],
        })
      );
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("No analytics yet")).toBeInTheDocument();
      });
    });

    it("renders top videos with titles", async () => {
      mockApiFetch.mockResolvedValueOnce(makeDashboardData());
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("Top Videos")).toBeInTheDocument();
      });
      expect(screen.getByText("Intro Video")).toBeInTheDocument();
      expect(screen.getByText("Demo Recording")).toBeInTheDocument();
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
      expect(introLink).toBeDefined();
      expect(demoLink).toBeDefined();
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

    it("hides top videos when empty", async () => {
      mockApiFetch.mockResolvedValueOnce(
        makeDashboardData({ topVideos: [] })
      );
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("350")).toBeInTheDocument();
      });
      expect(screen.queryByText("Top Videos")).not.toBeInTheDocument();
    });

    it("shows avg completion", async () => {
      mockApiFetch.mockResolvedValueOnce(makeDashboardData());
      renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("62%")).toBeInTheDocument();
      });
    });
  });

  describe("View toggle", () => {
    it("shows Video toggle active on video analytics route", async () => {
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      const { container } = renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      const activeButtons = container.querySelectorAll(".analytics-toggle-btn--active");
      expect(activeButtons.length).toBe(1);
      expect(activeButtons[0].textContent).toBe("Video");
    });

    it("shows Dashboard toggle active on dashboard route", async () => {
      mockApiFetch.mockResolvedValueOnce(makeDashboardData());
      const { container } = renderDashboard();

      await waitFor(() => {
        expect(screen.getByText("350")).toBeInTheDocument();
      });

      const activeButtons = container.querySelectorAll(".analytics-toggle-btn--active");
      expect(activeButtons.length).toBe(1);
      expect(activeButtons[0].textContent).toBe("Dashboard");
    });

    it("switches to dashboard when toggle clicked", async () => {
      const user = userEvent.setup();
      mockApiFetch.mockResolvedValueOnce(makeVideoAnalyticsData());
      renderVideoAnalytics();

      await waitFor(() => {
        expect(screen.getByText("142")).toBeInTheDocument();
      });

      mockApiFetch.mockResolvedValueOnce(makeDashboardData());
      await user.click(screen.getByText("Dashboard"));

      await waitFor(() => {
        expect(mockApiFetch).toHaveBeenCalledWith("/api/analytics/dashboard?range=7d");
      });
    });
  });
});
