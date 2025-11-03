import { API_URL } from "../config";

export async function authFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
    const token = localStorage.getItem("token");
    const headers = new Headers(init.headers || {});
    if (token) headers.set("Authorization", `Bearer ${token}`);
    const res = await fetch(input, { ...init, headers, credentials: "include" });
    if (res.status !== 401) return res;
    // try refresh
    const refresh = await fetch(`${API_URL}/refresh`, { method: "POST", credentials: "include" });
    if (!refresh.ok) return res;
    const data = await refresh.json().catch(() => ({} as any));
    if (data?.token) {
        localStorage.setItem("token", data.token);
        headers.set("Authorization", `Bearer ${data.token}`);
        return fetch(input, { ...init, headers, credentials: "include" });
    }
    return res;
}


