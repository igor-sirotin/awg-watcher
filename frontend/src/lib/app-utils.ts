export function titleStatus(status?: string) {
  return String(status || "unknown").replace(/_/g, " ")
}

export function statusBadgeVariant(status?: string) {
  switch (status) {
    case "ok":
      return "default"
    case "changed":
    case "missing":
    case "api_error":
      return "destructive"
    case "baseline":
      return "secondary"
    default:
      return "secondary"
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
