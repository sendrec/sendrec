import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { apiFetch, setAccessToken, ApiError } from "./client";

describe("apiFetch", () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    setAccessToken(null);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    setAccessToken(null);
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
