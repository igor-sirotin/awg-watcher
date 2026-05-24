import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Activity,
  AlertTriangle,
  Bell,
  Check,
  CircleDashed,
  Download,
  Eye,
  FileKey2,
  KeyRound,
  LayoutDashboard,
  Loader2,
  Plus,
  RefreshCw,
  Settings,
  ShieldAlert,
  SlidersHorizontal,
  Trash2,
  Wrench,
} from "lucide-react"

import { api, getStatus, hasSetupToken, stripSetupTokenAndReload, setupTokenQuery } from "@/lib/api"
import { cn, countryFlag, countryLabel, formatDate, formatDateTime, jsonBlock, statusTone, titleStatus } from "@/lib/utils"
import type { Config, KeyConfig, KeyState, SettingsPatch, SettingsResponse, StatusPayload } from "@/types/api"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button, buttonVariants } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

const emptyConfig: Config = { keys: [], poll_interval_hours: 6 }
type View = "dashboard" | "keys" | "awg" | "tools"

export default function App() {
  const [model, setModel] = useState<StatusPayload | null>(null)
  const [fatalError, setFatalError] = useState("")
  const [output, setOutput] = useState("")
  const [isChecking, setIsChecking] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [editingKey, setEditingKey] = useState<KeyConfig | null>(null)
  const [keyEditorOpen, setKeyEditorOpen] = useState(false)
  const [detailsKeyID, setDetailsKeyID] = useState("")
  const [view, setView] = useState<View>("dashboard")

  const refresh = useCallback(async ({ quiet = false } = {}) => {
    try {
      const next = await getStatus()
      setModel(next)
      setFatalError("")
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setFatalError(message)
      if (!quiet) setOutput(message)
    }
  }, [])

  useEffect(() => {
    void refresh()
    const id = window.setInterval(() => void refresh({ quiet: true }), 10_000)
    return () => window.clearInterval(id)
  }, [refresh])

  useEffect(() => {
    const req = model?.setup_requirements
    if (!req) return
    const missingSettings = !req.admin_password || !req.gateway_public_keys
    if (model?.setup_mode && missingSettings) setSettingsOpen(true)
  }, [model])

  const saveConfig = useCallback(
    async (patch: SettingsPatch) => {
      const cfg = model?.config || emptyConfig
      const body: SettingsPatch = {
        poll_interval_hours: patch.poll_interval_hours ?? cfg.poll_interval_hours ?? 6,
        telegram: patch.telegram || {},
        amnezia: patch.amnezia || {},
        web_password: patch.web_password || "",
        gateway_public_keys: patch.gateway_public_keys || "",
        auto_select_issued_countries: patch.auto_select_issued_countries || false,
      }
      if (Object.prototype.hasOwnProperty.call(patch, "keys")) body.keys = patch.keys
      return api<SettingsResponse>("/api/settings", { method: "POST", body: JSON.stringify(body) })
    },
    [model],
  )

  const runCheck = useCallback(async () => {
    setIsChecking(true)
    try {
      const result = await api<unknown>("/api/check", { method: "POST", body: "{}" })
      setOutput(jsonBlock(result))
      await refresh()
    } catch (err) {
      const data = (err as Error & { data?: unknown }).data
      setOutput(jsonBlock(data || (err instanceof Error ? err.message : String(err))))
      await refresh({ quiet: true })
    } finally {
      setIsChecking(false)
    }
  }, [refresh])

  const saveKey = useCallback(
    async (nextKey: KeyConfig, autoSelectIssued: boolean) => {
      const keys = mergeKeyForPatch(model?.config?.keys || [], nextKey)
      await saveConfig({ keys, auto_select_issued_countries: autoSelectIssued })
      await api<unknown>("/api/check", { method: "POST", body: "{}" }).catch(() => null)
      await refresh()
    },
    [model, refresh, saveConfig],
  )

  const deleteKey = useCallback(
    async (id: string) => {
      const keys = (model?.config?.keys || [])
        .filter((key) => key.id !== id)
        .map((key) => ({ id: key.id, name: key.name, countries: key.countries || [] }))
      await saveConfig({ keys })
      setDetailsKeyID("")
      await refresh()
    },
    [model, refresh, saveConfig],
  )

  const stateError = model?.state?.status === "api_error" ? model.state.last_error || "Gateway API error" : ""
  const displayedFatal = fatalError || stateError
  const config = model?.config || emptyConfig
  const detailsKey = (config.keys || []).find((key) => key.id === detailsKeyID) || null
  const viewMeta = {
    dashboard: {
      title: "Dashboard",
      description: "Overall status, schedule, and recent key activity.",
    },
    keys: {
      title: "AmneziaVPN keys",
      description: "Keys are checked independently and countries can be tracked per key.",
    },
    awg: {
      title: "AWG Manager",
      description: "Reserved for future router tunnel integration.",
    },
    tools: {
      title: "Tools",
      description: "Diagnostics, test notifications, and redacted support snapshots.",
    },
  }[view]

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <Sidebar view={view} onView={setView} fixture={Boolean(model?.fixture)} onSettings={() => setSettingsOpen(true)} />
      <main className="flex min-h-screen min-w-0 flex-1 flex-col">
        <header className="flex flex-col gap-4 px-4 py-4 md:flex-row md:items-center md:justify-between lg:px-6">
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">{viewMeta.title}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{viewMeta.description}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            {view === "dashboard" ? (
              <Button onClick={runCheck} disabled={isChecking || !model}>
                {isChecking ? <Loader2 className="animate-spin" /> : <RefreshCw />}
                Check now
              </Button>
            ) : null}
            {view === "keys" ? (
              <Button onClick={() => openNewKey(setEditingKey, setKeyEditorOpen)}>
                <Plus />
                Add key
              </Button>
            ) : null}
          </div>
        </header>

        <div className="flex-1 space-y-4 overflow-auto p-4 lg:p-6">
          {displayedFatal ? (
            <Alert variant="destructive">
              <ShieldAlert className="absolute left-4 top-4 size-4" />
              <AlertTitle className="pl-6">Fatal status</AlertTitle>
              <AlertDescription className="pl-6">{displayedFatal}</AlertDescription>
            </Alert>
          ) : null}

          {model ? <SetupAlert model={model} onSettings={() => setSettingsOpen(true)} onAddKey={() => openNewKey(setEditingKey, setKeyEditorOpen)} /> : null}

          {view === "dashboard" ? (
            <DashboardPage
              model={model}
              onKeys={() => setView("keys")}
              onTools={() => setView("tools")}
              onAddKey={() => openNewKey(setEditingKey, setKeyEditorOpen)}
              onDetails={(id) => setDetailsKeyID(id)}
              onEdit={(key) => {
                setEditingKey(key)
                setKeyEditorOpen(true)
              }}
            />
          ) : null}
          {view === "keys" ? (
            <KeysPage
              model={model}
              onDetails={(id) => setDetailsKeyID(id)}
              onEdit={(key) => {
                setEditingKey(key)
                setKeyEditorOpen(true)
              }}
            />
          ) : null}
          {view === "awg" ? <AwgPage /> : null}
          {view === "tools" ? <ToolsPage output={output} setOutput={setOutput} /> : null}
        </div>
      </main>

      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} model={model} saveConfig={saveConfig} refresh={refresh} />
      <KeyEditorDialog
        open={keyEditorOpen}
        onOpenChange={setKeyEditorOpen}
        keyConfig={editingKey}
        model={model}
        onSave={saveKey}
        onDelete={deleteKey}
      />
      <KeyDetailsDialog
        keyConfig={detailsKey}
        state={detailsKey ? model?.state?.keys?.[detailsKey.id] : undefined}
        nextCheck={model?.next_check}
        onOpenChange={(open) => {
          if (!open) setDetailsKeyID("")
        }}
      />
    </div>
  )
}

function Sidebar({
  view,
  onView,
  fixture,
  onSettings,
}: {
  view: View
  onView: (view: View) => void
  fixture: boolean
  onSettings: () => void
}) {
  const items = [
    { id: "dashboard" as const, label: "Dashboard", icon: LayoutDashboard },
    { id: "keys" as const, label: "Keys", icon: KeyRound },
    { id: "awg" as const, label: "AWG Manager", icon: Wrench },
    { id: "tools" as const, label: "Tools", icon: SlidersHorizontal },
  ]

  return (
    <aside className="flex w-16 shrink-0 flex-col bg-background md:w-64">
      <div className="flex h-16 items-center justify-center gap-2 px-3 md:justify-start md:px-4">
        <div className="flex size-9 items-center justify-center rounded-md bg-muted">
          <FileKey2 className="size-4" />
        </div>
        <div className="hidden min-w-0 md:block">
          <div className="truncate text-sm font-semibold">Config Watcher</div>
          <div className="truncate text-xs text-muted-foreground">Amnezia Premium</div>
        </div>
      </div>
      <nav className="grid gap-1 p-2">
        {items.map((item) => {
          const Icon = item.icon
          return (
            <Button
              key={item.id}
              type="button"
              variant="ghost"
              className={cn("justify-center px-0 md:justify-start md:px-4", view === item.id && "bg-accent text-accent-foreground")}
              onClick={() => onView(item.id)}
              title={item.label}
            >
              <Icon />
              <span className="hidden md:inline">{item.label}</span>
            </Button>
          )
        })}
      </nav>
      <div className="mt-auto grid gap-2 p-2 md:p-3">
        <Button
          type="button"
          variant="ghost"
          className="justify-center px-0 md:justify-start md:px-4"
          onClick={onSettings}
          title="Settings"
        >
          <Settings />
          <span className="hidden md:inline">Settings</span>
        </Button>
        <div className="hidden md:block">
          <Badge variant={fixture ? "secondary" : "default"}>{fixture ? "fixture mode" : "live mode"}</Badge>
        </div>
      </div>
    </aside>
  )
}

function DashboardPage({
  model,
  onKeys,
  onTools,
  onAddKey,
  onDetails,
  onEdit,
}: {
  model: StatusPayload | null
  onKeys: () => void
  onTools: () => void
  onAddKey: () => void
  onDetails: (id: string) => void
  onEdit: (key: KeyConfig) => void
}) {
  const keys = model?.config?.keys || []
  const keyStates = model?.state?.keys || {}
  const changedKeys = Object.values(keyStates).filter((key) => key.status === "changed" || key.status === "api_error")
  const watched = keys.reduce((sum, key) => sum + (key.countries || []).length, 0)

  return (
    <div className="space-y-4">
      <StatusOverview model={model} />
      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Keys</CardTitle>
            <CardDescription>{keys.length} configured, {watched} watched countries</CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button onClick={onKeys} variant="secondary">
              <KeyRound />
              Open keys
            </Button>
            <Button onClick={onAddKey} variant="outline">
              <Plus />
              Add key
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Events</CardTitle>
            <CardDescription>{changedKeys.length ? `${changedKeys.length} key needs attention` : "No key needs attention"}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={onTools} variant="secondary">
              <SlidersHorizontal />
              Open tools
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Schedule</CardTitle>
            <CardDescription>Next check {formatDateTime(model?.next_check)}</CardDescription>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            Poll interval {model?.config?.poll_interval_hours || 6} hours.
          </CardContent>
        </Card>
      </div>
      <section className="space-y-3">
        <div>
          <h2 className="text-lg font-semibold tracking-normal">Recent key status</h2>
          <p className="text-sm text-muted-foreground">The dashboard shows the same live status data as the keys page.</p>
        </div>
        <KeyList model={model} onDetails={onDetails} onEdit={onEdit} />
      </section>
    </div>
  )
}

function KeysPage({
  model,
  onDetails,
  onEdit,
}: {
  model: StatusPayload | null
  onDetails: (id: string) => void
  onEdit: (key: KeyConfig) => void
}) {
  return (
    <section>
      <KeyList model={model} onDetails={onDetails} onEdit={onEdit} />
    </section>
  )
}

function AwgPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Not configured yet</CardTitle>
        <CardDescription>This release only detects Amnezia metadata changes and sends notifications.</CardDescription>
      </CardHeader>
    </Card>
  )
}

function ToolsPage({ output, setOutput }: { output: string; setOutput: (output: string) => void }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Diagnostics and tests</CardTitle>
        <CardDescription>Validate delivery or collect a redacted snapshot.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            onClick={async () => {
              try {
                setOutput(jsonBlock(await api<unknown>("/api/telegram/test", { method: "POST", body: "{}" })))
              } catch (err) {
                setOutput(err instanceof Error ? err.message : String(err))
              }
            }}
          >
            <Bell />
            Telegram test
          </Button>
          <a className={buttonVariants({ variant: "outline" })} href={`/api/diagnostics${setupTokenQuery()}`}>
            <Download />
            Download diagnostics
          </a>
        </div>
        <pre className="min-h-48 max-h-[32rem] overflow-auto rounded-lg bg-muted p-3 text-xs text-muted-foreground">{output}</pre>
      </CardContent>
    </Card>
  )
}

function StatusOverview({ model }: { model: StatusPayload | null }) {
  const cfg = model?.config || emptyConfig
  const st = model?.state || {}
  const keys = cfg.keys || []
  const keyStates = st.keys || {}
  const watched = keys.reduce((sum, key) => sum + (key.countries || []).length, 0)
  const changed = Object.values(keyStates).reduce(
    (sum, keyState) => sum + Object.values(keyState.countries || {}).filter((country) => country.status === "changed" || country.status === "missing").length,
    0,
  )
  const apiErrors = Object.values(keyStates).filter((keyState) => keyState.status === "api_error").length
  const status = st.status || "unknown"

  return (
    <Card>
      <CardContent className="grid gap-4 p-4 md:grid-cols-[minmax(220px,0.7fr)_1fr]">
        <div className="flex items-center gap-4">
          <StatusIcon status={status} />
          <div>
            <div className="text-xs font-medium uppercase text-muted-foreground">Overall status</div>
            <div className="text-2xl font-semibold capitalize leading-tight">{titleStatus(status)}</div>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2 md:grid-cols-6">
          <Metric label="Keys" value={model ? keys.length : "loading"} tone="accent" />
          <Metric label="Watched" value={watched} tone="accent" />
          <Metric label="Changed" value={changed} tone={changed > 0 ? "destructive" : undefined} />
          <Metric label="API errors" value={apiErrors} tone={apiErrors > 0 ? "destructive" : undefined} />
          <Metric label="Last check" value={formatDateTime(st.last_check)} />
          <Metric label="Next check" value={formatDateTime(model?.next_check)} />
        </div>
      </CardContent>
    </Card>
  )
}

function StatusIcon({ status }: { status?: string }) {
  const Icon = status === "ok" ? Check : status === "api_error" || status === "changed" ? AlertTriangle : CircleDashed
  return (
    <div
      className={cn(
        "flex size-12 items-center justify-center rounded-lg",
        status === "ok" && "bg-primary text-primary-foreground",
        (status === "api_error" || status === "changed") && "bg-destructive text-destructive-foreground",
        status !== "ok" && status !== "api_error" && status !== "changed" && "bg-secondary text-secondary-foreground",
      )}
    >
      <Icon className="size-5" />
    </div>
  )
}

function SetupAlert({ model, onSettings, onAddKey }: { model: StatusPayload; onSettings: () => void; onAddKey: () => void }) {
  const req = model.setup_requirements || {}
  const missing = [
    !req.admin_password ? "admin password" : "",
    !req.gateway_public_keys ? "gateway public keys" : "",
    !req.amnezia_keys ? "AmneziaVPN key" : "",
  ].filter(Boolean)

  if (!missing.length) return null

  return (
    <Alert>
      <AlertTitle>Initial setup needs {missing.join(", ")}</AlertTitle>
      <AlertDescription className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span>Complete the required fields before relying on scheduled checks.</span>
        <span className="flex gap-2">
          <Button size="sm" variant="outline" onClick={onSettings}>
            <Settings />
            Settings
          </Button>
          {!req.amnezia_keys ? (
            <Button size="sm" onClick={onAddKey}>
              <Plus />
              Add key
            </Button>
          ) : null}
        </span>
      </AlertDescription>
    </Alert>
  )
}

function KeyList({
  model,
  onDetails,
  onEdit,
}: {
  model: StatusPayload | null
  onDetails: (id: string) => void
  onEdit: (key: KeyConfig) => void
}) {
  const keys = model?.config?.keys || []
  const states = model?.state?.keys || {}

  if (!model) {
    return <Card><CardContent className="p-5 text-sm text-muted-foreground">Loading keys...</CardContent></Card>
  }
  if (!keys.length) {
    return (
      <Card>
        <CardContent className="flex min-h-44 items-center justify-center p-5 text-center text-sm text-muted-foreground">
          Add an AmneziaVPN key. The app will fetch account info and start watching countries already in use.
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="grid gap-3">
      {keys.map((key) => (
        <KeyCard key={key.id} keyConfig={key} state={states[key.id]} nextCheck={model.next_check} onDetails={onDetails} onEdit={onEdit} />
      ))}
    </div>
  )
}

function KeyCard({
  keyConfig,
  state,
  nextCheck,
  onDetails,
  onEdit,
}: {
  keyConfig: KeyConfig
  state?: KeyState
  nextCheck?: string | null
  onDetails: (id: string) => void
  onEdit: (key: KeyConfig) => void
}) {
  const account = state?.last_account || {}
  const countries = Object.values(state?.countries || {}).sort((a, b) => a.code.localeCompare(b.code))
  const changedCount = countries.filter((country) => country.status === "changed" || country.status === "missing").length

  return (
    <Card className="overflow-hidden">
      <CardContent className="grid gap-4 p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="truncate text-base font-semibold tracking-normal">{keyConfig.name || "Key"}</h3>
              <Badge variant={statusTone(state?.status) as never}>{titleStatus(state?.status)}</Badge>
              {changedCount > 0 ? <Badge variant="destructive">{changedCount} changed</Badge> : null}
            </div>
            {state?.last_error ? <p className="mt-2 text-sm text-destructive">{state.last_error}</p> : null}
          </div>
          <div className="flex shrink-0 gap-2">
            <Button variant="outline" size="sm" onClick={() => onDetails(keyConfig.id)}>
              <Eye />
              Details
            </Button>
            <Button variant="secondary" size="sm" onClick={() => onEdit(keyConfig)}>
              <Settings />
              Edit
            </Button>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-7">
          <Metric label="Last check" value={formatDateTime(state?.last_check)} />
          <Metric label="Next check" value={formatDateTime(nextCheck)} />
          <Metric label="Devices" value={account.max_device_count ? `${account.active_device_count || 0}/${account.max_device_count}` : "-"} />
          <Metric label="Subscription" value={formatDate(account.subscription_end_date)} />
          <Metric label="Available" value={account.available_countries?.length ?? "-"} tone="accent" />
          <Metric label="Issued" value={account.issued_country_configs?.length ?? "-"} tone="accent" />
          <Metric label="Changed" value={changedCount} tone={changedCount > 0 ? "destructive" : undefined} />
        </div>
        <CountryRows countries={countries} />
      </CardContent>
    </Card>
  )
}

function CountryRows({ countries }: { countries: Array<{ code: string; status?: string; worker_last_updated?: string; last_downloaded?: string }> }) {
  if (!countries.length) return <p className="mt-3 text-sm text-muted-foreground">Run a check to create a baseline.</p>
  return (
    <div className="mt-3 grid gap-1 rounded-lg bg-muted p-2 text-xs sm:grid-cols-[90px_90px_1fr_1fr]">
      {countries.map((country) => (
        <div key={country.code} className="contents">
          <span>{countryLabel(country.code)}</span>
          <Badge variant={statusTone(country.status) as never}>{titleStatus(country.status)}</Badge>
          <span className="text-muted-foreground">worker {formatDateTime(country.worker_last_updated)}</span>
          <span className="text-muted-foreground">downloaded {formatDateTime(country.last_downloaded)}</span>
        </div>
      ))}
    </div>
  )
}

function Metric({ label, value, tone }: { label: string; value: unknown; tone?: "accent" | "destructive" }) {
  return (
    <div
      className={cn(
        "min-w-0 rounded-md px-3 py-2",
        !tone && "bg-muted",
        tone === "accent" && "bg-accent text-accent-foreground",
        tone === "destructive" && "bg-destructive/10 text-destructive",
      )}
    >
      <div className="truncate text-[11px] font-medium uppercase opacity-70">{label}</div>
      <div className="mt-0.5 truncate text-sm font-medium">{String(value ?? "-")}</div>
    </div>
  )
}

function SettingsDialog({
  open,
  onOpenChange,
  model,
  saveConfig,
  refresh,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  model: StatusPayload | null
  saveConfig: (patch: SettingsPatch) => Promise<SettingsResponse>
  refresh: () => Promise<void>
}) {
  const cfg = model?.config || emptyConfig
  const [password, setPassword] = useState("")
  const [pollInterval, setPollInterval] = useState(6)
  const [gatewayEndpoint, setGatewayEndpoint] = useState("")
  const [gatewayPath, setGatewayPath] = useState("")
  const [gatewayKeys, setGatewayKeys] = useState("")
  const [botToken, setBotToken] = useState("")
  const [chatID, setChatID] = useState("")
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setPassword("")
    setPollInterval(cfg.poll_interval_hours || 6)
    setGatewayEndpoint(cfg.amnezia?.gateway_endpoint || "")
    setGatewayPath(cfg.amnezia?.gateway_public_key_filepath || "")
    setGatewayKeys("")
    setBotToken("")
    setChatID("")
  }, [cfg.amnezia?.gateway_endpoint, cfg.amnezia?.gateway_public_key_filepath, cfg.poll_interval_hours, open])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    try {
      const data = await saveConfig({
        web_password: password,
        poll_interval_hours: Number(pollInterval || 6),
        gateway_public_keys: gatewayKeys,
        telegram: { bot_token: botToken, chat_id: chatID },
        amnezia: {
          gateway_endpoint: gatewayEndpoint,
          gateway_public_key_filepath: gatewayPath,
        },
      })
      onOpenChange(false)
      if (hasSetupToken() && (data.setup_complete || data.password_changed)) {
        stripSetupTokenAndReload()
        return
      }
      await refresh()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription>Admin access, gateway public keys, polling, and Telegram notifications.</DialogDescription>
        </DialogHeader>
        <form className="grid gap-4" onSubmit={submit}>
          <SetupChecklist model={model} />
          <div className="grid gap-2">
            <Label htmlFor="web_password">Admin password</Label>
            <Input id="web_password" type="password" autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="Set or leave unchanged" />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="poll_interval">Poll interval, hours</Label>
              <Input id="poll_interval" type="number" min={1} step={1} value={pollInterval} onChange={(event) => setPollInterval(Number(event.target.value))} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="gateway_endpoint">Gateway endpoint</Label>
              <Input id="gateway_endpoint" value={gatewayEndpoint} onChange={(event) => setGatewayEndpoint(event.target.value)} />
            </div>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="gateway_path">Gateway public key file</Label>
            <Input id="gateway_path" value={gatewayPath} onChange={(event) => setGatewayPath(event.target.value)} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="gateway_keys">Gateway public keys</Label>
            <Textarea
              id="gateway_keys"
              className="min-h-40 font-mono text-xs"
              spellCheck={false}
              value={gatewayKeys}
              onChange={(event) => setGatewayKeys(event.target.value)}
              placeholder="-----BEGIN PUBLIC KEY-----"
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="bot_token">Telegram bot token</Label>
              <Input id="bot_token" type="password" autoComplete="off" value={botToken} onChange={(event) => setBotToken(event.target.value)} placeholder="Leave unchanged" />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="chat_id">Telegram chat ID</Label>
              <Input id="chat_id" value={chatID} onChange={(event) => setChatID(event.target.value)} placeholder="Leave unchanged" />
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={saving}>
              {saving ? <Loader2 className="animate-spin" /> : <Check />}
              Save settings
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function SetupChecklist({ model }: { model: StatusPayload | null }) {
  const req = model?.setup_requirements || {}
  const rows = [
    ["Admin password", req.admin_password],
    ["Gateway public keys", req.gateway_public_keys],
    ["AmneziaVPN key", req.amnezia_keys],
  ] as const
  return (
    <div className="grid gap-2 rounded-lg bg-muted p-3">
      <div className="text-sm font-medium">Initial setup</div>
      <div className="grid gap-1 text-sm">
        {rows.map(([label, ok]) => (
          <div key={label} className="flex items-center justify-between gap-3">
            <span className="text-muted-foreground">{label}</span>
            <Badge variant={ok ? "default" : "secondary"}>{ok ? "ready" : "needed"}</Badge>
          </div>
        ))}
      </div>
    </div>
  )
}

function KeyEditorDialog({
  open,
  onOpenChange,
  keyConfig,
  model,
  onSave,
  onDelete,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  keyConfig: KeyConfig | null
  model: StatusPayload | null
  onSave: (key: KeyConfig, autoSelectIssued: boolean) => Promise<void>
  onDelete: (id: string) => Promise<void>
}) {
  const [name, setName] = useState("")
  const [vpnKey, setVpnKey] = useState("")
  const [countries, setCountries] = useState<Set<string>>(new Set())
  const [saving, setSaving] = useState(false)
  const keyState = keyConfig ? model?.state?.keys?.[keyConfig.id] : undefined
  const available = keyState?.last_account?.available_countries || []
  const issued = useMemo(() => new Set((keyState?.last_account?.issued_country_configs || []).map((country) => country.code)), [keyState])

  useEffect(() => {
    if (!open) return
    setName(keyConfig?.name || "")
    setVpnKey("")
    setCountries(new Set(keyConfig?.countries || []))
  }, [keyConfig, open])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    try {
      const selectedCountries = [...countries].sort()
      const nextKey: KeyConfig = {
        id: keyConfig?.id || "",
        name,
        countries: selectedCountries,
      }
      if (vpnKey.trim()) nextKey.vpn_key = vpnKey.trim()
      await onSave(nextKey, selectedCountries.length === 0)
      onOpenChange(false)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{keyConfig ? "Edit key" : "Add key"}</DialogTitle>
          <DialogDescription>
            Save a new key once and the app will fetch account info, detect countries already in use, and start tracking them.
          </DialogDescription>
        </DialogHeader>
        <form className="grid gap-4" onSubmit={submit}>
          <div className="grid gap-2">
            <Label htmlFor="key_name">Name</Label>
            <Input id="key_name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Personal Premium" />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="vpn_key">vpn:// key</Label>
            <Textarea
              id="vpn_key"
              className="min-h-28 font-mono text-xs"
              spellCheck={false}
              value={vpnKey}
              onChange={(event) => setVpnKey(event.target.value)}
              placeholder={keyConfig ? "Leave empty to keep existing key" : "Paste vpn:// key"}
            />
          </div>
          <CountryPicker available={available} issued={issued} selected={countries} onSelected={setCountries} />
          <DialogFooter>
            {keyConfig ? (
              <Button
                type="button"
                variant="destructive"
                disabled={saving}
                onClick={async () => {
                  setSaving(true)
                  try {
                    await onDelete(keyConfig.id)
                    onOpenChange(false)
                  } finally {
                    setSaving(false)
                  }
                }}
              >
                <Trash2 />
                Delete
              </Button>
            ) : null}
            <Button type="submit" disabled={saving}>
              {saving ? <Loader2 className="animate-spin" /> : <Check />}
              Save key
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function CountryPicker({
  available,
  issued,
  selected,
  onSelected,
}: {
  available: Array<{ code: string; name?: string }>
  issued: Set<string>
  selected: Set<string>
  onSelected: (selected: Set<string>) => void
}) {
  if (!available.length) {
    return (
      <Alert>
        <Activity className="absolute left-4 top-4 size-4" />
        <AlertTitle className="pl-6">Countries will load after the first check</AlertTitle>
        <AlertDescription className="pl-6">When no country is selected, issued configs are selected automatically.</AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="grid gap-2">
      <Label>Countries</Label>
      <div className="grid max-h-80 gap-2 overflow-auto rounded-lg bg-muted p-2">
        {available.map((country) => {
          const checked = selected.has(country.code)
          return (
            <label key={country.code} className="grid cursor-pointer grid-cols-[1rem_3.5rem_1fr_auto] items-center gap-3 rounded-md px-2 py-2 text-sm hover:bg-muted">
              <input
                type="checkbox"
                className="size-4"
                checked={checked}
                onChange={(event) => {
                  const next = new Set(selected)
                  if (event.target.checked) next.add(country.code)
                  else next.delete(country.code)
                  onSelected(next)
                }}
              />
              <span>{countryFlag(country.code)} {country.code}</span>
              <span className="truncate text-muted-foreground">{country.name || ""}</span>
              <Badge variant={issued.has(country.code) ? "default" : "secondary"}>{issued.has(country.code) ? "in use" : "available"}</Badge>
            </label>
          )
        })}
      </div>
    </div>
  )
}

function KeyDetailsDialog({
  keyConfig,
  state,
  nextCheck,
  onOpenChange,
}: {
  keyConfig: KeyConfig | null
  state?: KeyState
  nextCheck?: string | null
  onOpenChange: (open: boolean) => void
}) {
  const account = state?.last_account || {}
  const issued = account.issued_country_configs || []
  const countries = Object.values(state?.countries || {}).sort((a, b) => a.code.localeCompare(b.code))

  return (
    <Dialog open={Boolean(keyConfig)} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-5xl">
        <DialogHeader>
          <DialogTitle>{keyConfig?.name || "Key status"}</DialogTitle>
          <DialogDescription>Account metadata, issued configs, and watched-country status for this key.</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
            <Metric label="Status" value={titleStatus(state?.status)} />
            <Metric label="Last check" value={formatDateTime(state?.last_check)} />
            <Metric label="Next check" value={formatDateTime(nextCheck)} />
            <Metric label="Errors" value={state?.error_count || 0} tone={state?.error_count ? "destructive" : undefined} />
            <Metric label="Devices" value={account.max_device_count ? `${account.active_device_count || 0}/${account.max_device_count}` : "-"} />
            <Metric label="Subscription ends" value={formatDate(account.subscription_end_date)} />
            <Metric label="Available countries" value={account.available_countries?.length ?? "-"} />
            <Metric label="Issued configs" value={issued.length} />
          </div>
          {state?.last_error ? <Alert variant="destructive"><AlertTitle>Key error</AlertTitle><AlertDescription>{state.last_error}</AlertDescription></Alert> : null}
          <DataTable
            title="Watched countries"
            empty="No watched country state yet."
            columns={["Country", "Status", "Worker updated", "Last downloaded", "UUID"]}
            rows={countries.map((country) => [
              `${countryLabel(country.code)} ${country.name || ""}`,
              titleStatus(country.status),
              formatDateTime(country.worker_last_updated),
              formatDateTime(country.last_downloaded),
              country.installation_uuid || "-",
            ])}
          />
          <DataTable
            title="Issued country configs"
            empty="No issued country configs returned yet."
            columns={["Country", "Worker updated", "Last downloaded", "UUID"]}
            rows={issued.map((country) => [
              `${countryLabel(country.code)} ${country.name || ""}`,
              formatDateTime(country.worker_last_updated),
              formatDateTime(country.last_downloaded),
              country.installation_uuid || "-",
            ])}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}

function DataTable({ title, empty, columns, rows }: { title: string; empty: string; columns: string[]; rows: string[][] }) {
  return (
    <section className="grid gap-2">
      <h3 className="text-sm font-semibold tracking-normal">{title}</h3>
      {!rows.length ? (
        <p className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">{empty}</p>
      ) : (
        <div className="overflow-auto rounded-lg bg-muted">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead className="bg-muted text-muted-foreground">
              <tr>
                {columns.map((column) => (
                  <th key={column} className="px-3 py-2 text-left font-medium">{column}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, rowIndex) => (
                <tr key={`${row[0]}-${rowIndex}`}>
                  {row.map((cell, cellIndex) => (
                    <td key={`${cell}-${cellIndex}`} className={cn("px-3 py-2", cellIndex === row.length - 1 && "mono text-xs text-muted-foreground")}>
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

function mergeKeyForPatch(existing: KeyConfig[], nextKey: KeyConfig) {
  const keys: KeyConfig[] = existing.map((key) => ({ id: key.id, name: key.name, countries: key.countries || [] }))
  const normalizedNext = { ...nextKey, countries: nextKey.countries || [] }
  const idx = nextKey.id ? keys.findIndex((key) => key.id === nextKey.id) : -1
  if (idx >= 0) keys[idx] = normalizedNext
  else keys.push(normalizedNext)
  return keys
}

function openNewKey(setEditingKey: (key: KeyConfig | null) => void, setOpen: (open: boolean) => void) {
  setEditingKey(null)
  setOpen(true)
}
