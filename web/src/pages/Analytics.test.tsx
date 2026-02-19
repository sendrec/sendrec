import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { Analytics } from "./Analytics";

const mockApiFetch = vi.fn();

vi.mock("../api/client", () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}));

function makeAnalyticsData(overrides: Record<string, unknown> = {}) {
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
      reached25: 0,
      reached50: 0,
      reached75: 0,
      reached100: 0,
      ...(overrides.milestones as Record<string, unknown> ?? {}),
    },
  };
}

function renderAnalytics(videoId = "v1") {
  return render(
    <MemoryRouter initialEntries={[`/videos/${videoId}/analytics`]}>
      <Routes>
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

  it("renders summary cards with correct values", async () => {
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData());
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("142")).toBeInTheDocument();
    });
    expect(screen.getByText("98")).toBeInTheDocument();
    expect(screen.getByText("4.7")).toBeInTheDocument();
    expect(screen.getByText("23")).toBeInTheDocument();
  });

  it("shows loading state initially", () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}));
    renderAnalytics();

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("fetches analytics with default 7d range", async () => {
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData());
    renderAnalytics();

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/analytics?range=7d");
    });
  });

  it("switches range when clicking 30d button", async () => {
    const user = userEvent.setup();
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData());
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("142")).toBeInTheDocument();
    });

    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData());
    await user.click(screen.getByRole("button", { name: "30d" }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith("/api/videos/v1/analytics?range=30d");
    });
  });

  it("shows empty state when no views", async () => {
    mockApiFetch.mockResolvedValueOnce(
      makeAnalyticsData({
        summary: { totalViews: 0, uniqueViews: 0, viewsToday: 0, averageDailyViews: 0, peakDay: "", peakDayViews: 0 },
        daily: [],
      })
    );
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("No views in this period.")).toBeInTheDocument();
    });
  });

  it("shows error state when fetch fails", async () => {
    mockApiFetch.mockRejectedValueOnce(new Error("Network error"));
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("Failed to load analytics.")).toBeInTheDocument();
    });
  });

  it("has link back to library", async () => {
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData());
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("142")).toBeInTheDocument();
    });

    const libraryLink = screen.getByRole("link", { name: /Library/ });
    expect(libraryLink).toHaveAttribute("href", "/library");
  });

  it("displays CTA clicks card", async () => {
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData({
      summary: { totalCtaClicks: 5, ctaClickRate: 0.25 },
    }));
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("CTA Clicks")).toBeInTheDocument();
    });
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("25.0% click rate")).toBeInTheDocument();
  });

  it("displays completion funnel with milestone data", async () => {
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData({
      summary: { totalViews: 100 },
      milestones: { reached25: 80, reached50: 60, reached75: 40, reached100: 25 },
    }));
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("Completion Funnel")).toBeInTheDocument();
    });
    expect(screen.getByText("80")).toBeInTheDocument();
    expect(screen.getByText("60")).toBeInTheDocument();
    expect(screen.getByText("40")).toBeInTheDocument();
    expect(screen.getByText("80%")).toBeInTheDocument();
    expect(screen.getByText("60%")).toBeInTheDocument();
    expect(screen.getByText("40%")).toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
  });

  it("hides completion funnel when no views", async () => {
    mockApiFetch.mockResolvedValueOnce(makeAnalyticsData({
      summary: { totalViews: 0, uniqueViews: 0, viewsToday: 0, averageDailyViews: 0, peakDay: "", peakDayViews: 0 },
      daily: [],
    }));
    renderAnalytics();

    await waitFor(() => {
      expect(screen.getByText("No views in this period.")).toBeInTheDocument();
    });
    expect(screen.queryByText("Completion Funnel")).not.toBeInTheDocument();
  });
});
