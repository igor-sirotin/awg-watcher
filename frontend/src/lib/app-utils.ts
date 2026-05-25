export function titleStatus(status?: string) {
  return String(status || "unknown").replace(/_/g, " ")
}

export function statusBadgeClass(status?: string) {
  switch (status) {
    case "ok":
      return "border-green-200 bg-green-50 text-green-700 dark:border-green-900/50 dark:bg-green-950 dark:text-green-300"
    case "changed":
    case "missing":
    case "api_error":
      return "border-red-200 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-950 dark:text-red-300"
    case "baseline":
      return "border-yellow-200 bg-yellow-50 text-yellow-800 dark:border-yellow-900/50 dark:bg-yellow-950 dark:text-yellow-300"
    default:
      return "border-yellow-200 bg-yellow-50 text-yellow-800 dark:border-yellow-900/50 dark:bg-yellow-950 dark:text-yellow-300"
  }
}

export function formatDateTime(value?: string | null) {
  if (!value || value.startsWith("0001-")) return "not yet"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date)
}

export function formatDate(value?: string | null) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(date)
}

export function countryFlag(code?: string) {
  const cc = String(code || "").toUpperCase()
  if (!/^[A-Z]{2}$/.test(cc)) return ""
  return cc.replace(/./g, (ch) => String.fromCodePoint(127397 + ch.charCodeAt(0)))
}

export function countryLabel(code?: string) {
  const cc = String(code || "").toUpperCase()
  return `${countryFlag(cc)} ${cc}`.trim()
}

export function jsonBlock(value: unknown) {
  return typeof value === "string" ? value : JSON.stringify(value, null, 2)
}
