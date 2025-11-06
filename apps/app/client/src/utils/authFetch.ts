import { API_URL } from "../config";

/**
 * authFetch
 * - Sends Authorization: Bearer <access_token> if present.
 * - Uses credentials from caller if provided; defaults to 'omit' (no cookies) to avoid unnecessary CORS constraints.
 * - On 401, attempts a refresh using cookies (credentials: 'include') and retries once with the same credentials mode as the original call.
 */
export async function authFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
    const token = localStorage.getItem("token");
    const headers = new Headers(init.headers || {});
    if (token) headers.set("Authorization", `Bearer ${token}`);

    // Important: don't force cookies unless caller asked for them.
    // This keeps /join (directory) simple: credentials: 'omit' avoids extra CORS requirements.
    const credentials = init.credentials ?? "omit";

    let res = await fetch(input, { ...init, headers, credentials });
    if (res.status !== 401) return res;

    // Try refresh (cookie-based). This always uses credentials: 'include'.
    const refresh = await fetch(`${API_URL}/refresh`, { method: "POST", credentials: "include" });
    if (!refresh.ok) return res;

    const data = await refresh.json().catch(() => ({} as any));
    if (data?.token) {
        localStorage.setItem("token", data.token);
        headers.set("Authorization", `Bearer ${data.token}`);
        // Retry original request with the SAME credentials mode as the first attempt.
        return fetch(input, { ...init, headers, credentials });
    }
    return res;
}

