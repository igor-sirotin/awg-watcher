import type { StatusPayload } from "@/types/api"

const setupToken = new URLSearchParams(window.location.search).get("setup_token") || ""

export function hasSetupToken() {
  return setupToken !== ""
}

export function setupTokenQuery() {
  return setupToken ? `?setup_token=${encodeURIComponent(setupToken)}` : ""
}

export function stripSetupTokenAndReload() {
  window.history.replaceState({}, document.title, window.location.pathname)
  window.location.replace(window.location.pathname)
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (!headers.has("Content-Type") && options.body) headers.set("Content-Type", "application/json")
  if (setupToken) headers.set("X-Setup-Token", setupToken)

  const res = await fetch(path, { ...options, headers })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const error = new Error(data.error || JSON.stringify(data))
    ;(error as Error & { data?: unknown }).data = data
    throw error
  }
  return data
}

export function getStatus() {
  return api<StatusPayload>(`/api/status${setupTokenQuery()}`)
}
