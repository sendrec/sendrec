class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

let accessToken: string | null = null;

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

  const data = (await response.json()) as { access_token: string };
  return data.access_token;
}

async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T | undefined> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");

  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  let response = await fetch(path, { ...options, headers });

  if (response.status === 401 && accessToken) {
    try {
      const newToken = await refreshToken();
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
    throw new ApiError(response.status, response.statusText);
  }

  if (response.status === 204) {
    return undefined;
  }

  return (await response.json()) as T;
}

export { ApiError, setAccessToken, getAccessToken, apiFetch };
