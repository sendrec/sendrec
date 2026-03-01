import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { apiFetch, setAccessToken, ApiError } from "./client";
import { setCurrentOrgId } from "./orgContext";

describe("apiFetch", () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    setAccessToken(null);
    setCurrentOrgId(null);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    setAccessToken(null);
    setCurrentOrgId(null);
  });

  it("returns parsed JSON on success", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ id: "1", name: "test" }),
    });

    const result = await apiFetch("/api/test");
    expect(result).toEqual({ id: "1", name: "test" });
  });

  it("returns undefined on 204 No Content", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const result = await apiFetch("/api/test", { method: "DELETE" });
    expect(result).toBeUndefined();
  });

  it("returns undefined on 202 Accepted", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 202,
    });

    const result = await apiFetch("/api/test", { method: "POST" });
    expect(result).toBeUndefined();
  });

  it("throws ApiError with JSON error message on failure", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: "Bad Request",
      json: () => Promise.resolve({ error: "name is required" }),
    });

    await expect(apiFetch("/api/test")).rejects.toThrow(ApiError);
    try {
      await apiFetch("/api/test");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).message).toBe("name is required");
      expect((err as ApiError).status).toBe(400);
    }
  });

  it("falls back to statusText when response is not JSON", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: () => Promise.reject(new Error("not JSON")),
    });

    await expect(apiFetch("/api/test")).rejects.toThrow("Internal Server Error");
  });

  it("sets Content-Type header when body is provided", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });

    await apiFetch("/api/test", {
      method: "POST",
      body: JSON.stringify({ name: "test" }),
    });

    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const headers = call[1].headers as Headers;
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  it("sets Authorization header when access token exists", async () => {
    setAccessToken("test-token");

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });

    await apiFetch("/api/test");

    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const headers = call[1].headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer test-token");
  });

  it("attempts token refresh on 401 and retries", async () => {
    setAccessToken("expired-token");

    const fetchMock = vi.fn()
      // First call: 401
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ error: "token expired" }),
      })
      // Refresh call: success
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ accessToken: "new-token" }),
      })
      // Retry call: success
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: "success" }),
      });

    globalThis.fetch = fetchMock;

    const result = await apiFetch("/api/test");
    expect(result).toEqual({ data: "success" });
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("deduplicates concurrent 401 refresh calls", async () => {
    setAccessToken("expired-token");

    let refreshCallCount = 0;
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url === "/api/auth/refresh") {
        refreshCallCount++;
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ accessToken: "new-token" }),
        });
      }
      // First two calls: both return 401
      if (fetchMock.mock.calls.filter((c: string[]) => c[0] !== "/api/auth/refresh").length <= 2) {
        return Promise.resolve({
          ok: false,
          status: 401,
          statusText: "Unauthorized",
          json: () => Promise.resolve({ error: "token expired" }),
        });
      }
      // Retry calls: success
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: "ok" }),
      });
    });

    globalThis.fetch = fetchMock;

    const [result1, result2] = await Promise.all([
      apiFetch("/api/test1"),
      apiFetch("/api/test2"),
    ]);

    expect(result1).toEqual({ data: "ok" });
    expect(result2).toEqual({ data: "ok" });
    expect(refreshCallCount).toBe(1);
  });

  it("sets X-Organization-Id header when org is selected", async () => {
    setCurrentOrgId("org-123");

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });

    await apiFetch("/api/test");

    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const headers = call[1].headers as Headers;
    expect(headers.get("X-Organization-Id")).toBe("org-123");
  });

  it("does not set X-Organization-Id header when no org is selected", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });

    await apiFetch("/api/test");

    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const headers = call[1].headers as Headers;
    expect(headers.get("X-Organization-Id")).toBeNull();
  });

  it("redirects to /login when refresh fails", async () => {
    setAccessToken("expired-token");

    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ error: "token expired" }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
      });

    globalThis.fetch = fetchMock;

    // Mock window.location
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, href: "" },
    });

    const result = await apiFetch("/api/test");
    expect(result).toBeUndefined();
    expect(window.location.href).toBe("/login");

    Object.defineProperty(window, "location", {
      writable: true,
      value: originalLocation,
    });
  });
});
