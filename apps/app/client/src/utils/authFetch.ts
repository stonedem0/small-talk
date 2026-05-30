import { storage, authExpiredCallback, apiUrlRef } from "../context";

export async function authFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
    const token = storage.get("token");
    const headers = new Headers(init.headers || {});
    if (token) headers.set("Authorization", `Bearer ${token}`);

    const credentials = init.credentials ?? "omit";

    let res = await fetch(input, { ...init, headers, credentials });
    if (res.status !== 401) return res;

    const refresh = await fetch(`${apiUrlRef.current}/refresh`, { method: "POST", credentials: "include" });
    if (!refresh.ok) {
        authExpiredCallback.current?.();
        return res;
    }

    const data = await refresh.json().catch(() => ({} as any));
    if (data?.token) {
        storage.set("token", data.token);
        headers.set("Authorization", `Bearer ${data.token}`);
        return fetch(input, { ...init, headers, credentials });
    }
    return res;
}

