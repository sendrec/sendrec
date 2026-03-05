import { getCurrentOrgId } from "./orgContext";

class ApiError extends Error {
  public readonly data: Record<string, unknown>;
  constructor(
    public readonly status: number,
    message: string,
    data: Record<string, unknown> = {}
  ) {
    super(message);
    this.name = "ApiError";
    this.data = data;
  }
}

let accessToken: string | null = null;
let refreshPromise: Promise<string> | null = null;

function setAccessToken(token: string | null): void {
  accessToken = token;
}

function getAccessToken(): string | null {
  return accessToken;
}

async function refreshToken(): Promise<string> {
  const response = await fetch("/api/auth/refresh", {
    method: "POST",
    credentials: "include",
  });

  if (!response.ok) {
    throw new ApiError(response.status, "Token refresh failed");
  }

  const data = (await response.json()) as { accessToken: string };
  return data.accessToken;
}

async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T | undefined> {
  const headers = new Headers(options.headers);
  if (options.body) {
    headers.set("Content-Type", "application/json");
  }

  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  const orgId = getCurrentOrgId();
  if (orgId) {
    headers.set("X-Organization-Id", orgId);
  }

  let response = await fetch(path, { ...options, headers });

  if (response.status === 401 && accessToken) {
    try {
      if (!refreshPromise) {
        refreshPromise = refreshToken().finally(() => {
          refreshPromise = null;
        });
      }
      const newToken = await refreshPromise;
      setAccessToken(newToken);
      headers.set("Authorization", `Bearer ${newToken}`);
      response = await fetch(path, { ...options, headers });
    } catch {
      setAccessToken(null);
      window.location.href = "/login";
      return undefined;
    }
  }

  if (!response.ok) {
    let message = response.statusText;
    let data: Record<string, unknown> = {};
    try {
      const body = await response.json();
      if (body.error) {
        message = body.error;
      }
      data = body;
    } catch {
      // response body wasn't JSON, keep statusText
    }
    throw new ApiError(response.status, message, data);
  }

  if (response.status === 204 || response.status === 202) {
    return undefined;
  }

  return (await response.json()) as T;
}

async function tryRefreshToken(): Promise<boolean> {
  try {
    if (!refreshPromise) {
      refreshPromise = refreshToken().finally(() => {
        refreshPromise = null;
      });
    }
    const token = await refreshPromise;
    setAccessToken(token);
    return true;
  } catch {
    return false;
  }
}

export { ApiError, setAccessToken, getAccessToken, apiFetch, tryRefreshToken };
