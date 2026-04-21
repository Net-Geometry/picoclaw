import {
  IconCalendar,
  IconCircleCheck,
  IconCircleX,
  IconPencil,
  IconPlus,
  IconRefresh,
  IconTrash,
} from "@tabler/icons-react"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"

// ---- types ----------------------------------------------------------------

interface CronSchedule {
  kind: "cron" | "every" | "at"
  expr?: string
  everyMs?: number
  atMs?: number
  tz?: string
}

interface CronPayload {
  kind: string
  message: string
  channel?: string
  to?: string
}

interface CronJobState {
  nextRunAtMs?: number
  lastRunAtMs?: number
  lastStatus?: string
  lastError?: string
}

interface CronJob {
  id: string
  name: string
  enabled: boolean
  schedule: CronSchedule
  payload: CronPayload
  state: CronJobState
  createdAtMs: number
  updatedAtMs: number
  deleteAfterRun: boolean
}

// ---- helpers ---------------------------------------------------------------

function formatMs(ms?: number): string {
  if (!ms) return "—"
  return new Date(ms).toLocaleString()
}

function scheduleLabel(s: CronSchedule): string {
  if (s.kind === "cron") return `cron: ${s.expr ?? ""}`
  if (s.kind === "every") {
    const mins = Math.round((s.everyMs ?? 0) / 60000)
    if (mins < 60) return `every ${mins}m`
    return `every ${Math.round(mins / 60)}h`
  }
  if (s.kind === "at") return `once: ${formatMs(s.atMs)}`
  return s.kind
}

// ---- blank form ------------------------------------------------------------

interface JobForm {
  name: string
  scheduleKind: "cron" | "every" | "at"
  cronExpr: string
  everyMinutes: string
  atDatetime: string
  message: string
  channel: string
  to: string
}

function blankForm(): JobForm {
  return {
    name: "",
    scheduleKind: "cron",
    cronExpr: "0 9 * * 1-5",
    everyMinutes: "60",
    atDatetime: "",
    message: "",
    channel: "",
    to: "",
  }
}

function jobToForm(job: CronJob): JobForm {
  const f = blankForm()
  f.name = job.name
  f.scheduleKind = job.schedule.kind
  f.cronExpr = job.schedule.expr ?? ""
  f.everyMinutes = job.schedule.everyMs
    ? String(Math.round(job.schedule.everyMs / 60000))
    : "60"
  if (job.schedule.atMs) {
    const d = new Date(job.schedule.atMs)
    f.atDatetime = d.toISOString().slice(0, 16)
  }
  f.message = job.payload.message
  f.channel = job.payload.channel ?? ""
  f.to = job.payload.to ?? ""
  return f
}

function formToRequest(f: JobForm) {
  let schedule: CronSchedule = { kind: f.scheduleKind }
  if (f.scheduleKind === "cron") {
    schedule.expr = f.cronExpr
  } else if (f.scheduleKind === "every") {
    schedule.everyMs = Number(f.everyMinutes) * 60000
  } else if (f.scheduleKind === "at") {
    schedule.atMs = new Date(f.atDatetime).getTime()
  }
  return {
    name: f.name,
    schedule,
    message: f.message,
    channel: f.channel,
    to: f.to,
  }
}

// ---- component ------------------------------------------------------------

export function SchedulerPage() {
  const { t } = useTranslation()
  const [jobs, setJobs] = useState<CronJob[]>([])
  const [loading, setLoading] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<CronJob | null>(null)
  const [form, setForm] = useState<JobForm>(blankForm())
  const [saving, setSaving] = useState(false)

  async function fetchJobs() {
    setLoading(true)
    try {
      const res = await fetch("/api/cron/jobs")
      if (res.ok) setJobs(await res.json())
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchJobs()
  }, [])

  function openCreate() {
    setEditing(null)
    setForm(blankForm())
    setDialogOpen(true)
  }

  function openEdit(job: CronJob) {
    setEditing(job)
    setForm(jobToForm(job))
    setDialogOpen(true)
  }

  async function handleSave() {
    setSaving(true)
    try {
      const body = formToRequest(form)
      let res: Response
      if (editing) {
        res = await fetch(`/api/cron/jobs/${editing.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ ...editing, ...body }),
        })
      } else {
        res = await fetch("/api/cron/jobs", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        })
      }
      if (res.ok) {
        setDialogOpen(false)
        await fetchJobs()
      }
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string) {
    await fetch(`/api/cron/jobs/${id}`, { method: "DELETE" })
    await fetchJobs()
  }

  async function handleToggle(job: CronJob) {
    const action = job.enabled ? "disable" : "enable"
    await fetch(`/api/cron/jobs/${job.id}/${action}`, { method: "POST" })
    await fetchJobs()
  }

  function setField<K extends keyof JobForm>(k: K, v: JobForm[K]) {
    setForm((f) => ({ ...f, [k]: v }))
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("pages.scheduler.title")} />

      <div className="flex-1 overflow-auto p-4">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-muted-foreground text-sm font-semibold">
            {jobs.length} {t("pages.scheduler.jobs")}
          </h2>
          <div className="flex gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={fetchJobs}
              disabled={loading}
            >
              <IconRefresh className="size-4" />
            </Button>
            <Button size="sm" onClick={openCreate}>
              <IconPlus className="mr-1 size-4" />
              {t("pages.scheduler.newJob")}
            </Button>
          </div>
        </div>

        <div className="overflow-x-auto rounded-md border">
          <table className="w-full text-sm">
            <thead className="bg-muted/40 border-b">
              <tr>
                <th className="px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.name")}
                </th>
                <th className="px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.schedule")}
                </th>
                <th className="px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.message")}
                </th>
                <th className="px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.lastRun")}
                </th>
                <th className="px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.nextRun")}
                </th>
                <th className="px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.status")}
                </th>
                <th className="w-[100px] px-4 py-2 text-left font-medium">
                  {t("pages.scheduler.col.actions")}
                </th>
              </tr>
            </thead>
            <tbody>
              {jobs.length === 0 && (
                <tr>
                  <td
                    colSpan={7}
                    className="text-muted-foreground py-8 text-center"
                  >
                    {loading
                      ? t("pages.scheduler.loading")
                      : t("pages.scheduler.noJobs")}
                  </td>
                </tr>
              )}
              {jobs.map((job) => (
                <tr
                  key={job.id}
                  className="hover:bg-muted/20 border-b last:border-0"
                >
                  <td className="px-4 py-2 font-medium">{job.name}</td>
                  <td className="px-4 py-2 font-mono text-xs">
                    {scheduleLabel(job.schedule)}
                  </td>
                  <td className="text-muted-foreground max-w-[180px] truncate px-4 py-2">
                    {job.payload.message}
                  </td>
                  <td className="text-muted-foreground px-4 py-2 text-xs">
                    {formatMs(job.state.lastRunAtMs)}
                  </td>
                  <td className="text-muted-foreground px-4 py-2 text-xs">
                    {formatMs(job.state.nextRunAtMs)}
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={job.enabled}
                        onCheckedChange={() => handleToggle(job)}
                        className="scale-75"
                      />
                      {job.state.lastStatus === "ok" && (
                        <IconCircleCheck className="size-4 text-green-500" />
                      )}
                      {job.state.lastStatus === "error" && (
                        <IconCircleX
                          className="size-4 text-red-500"
                          title={job.state.lastError}
                        />
                      )}
                      {!job.enabled && (
                        <Badge variant="secondary" className="text-xs">
                          {t("pages.scheduler.disabled")}
                        </Badge>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => openEdit(job)}
                      >
                        <IconPencil className="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-red-500 hover:text-red-600"
                        onClick={() => handleDelete(job.id)}
                      >
                        <IconTrash className="size-4" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Create / Edit dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              <IconCalendar className="mr-2 inline size-4" />
              {editing
                ? t("pages.scheduler.editJob")
                : t("pages.scheduler.createJob")}
            </DialogTitle>
          </DialogHeader>

          <div className="grid gap-4 py-2">
            <div className="grid gap-1.5">
              <Label>{t("pages.scheduler.field.name")}</Label>
              <Input
                value={form.name}
                onChange={(e) => setField("name", e.target.value)}
                placeholder="Daily standup report"
              />
            </div>

            <div className="grid gap-1.5">
              <Label>{t("pages.scheduler.field.scheduleKind")}</Label>
              <Select
                value={form.scheduleKind}
                onValueChange={(v) =>
                  setField("scheduleKind", v as JobForm["scheduleKind"])
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="cron">
                    {t("pages.scheduler.scheduleKind.cron")}
                  </SelectItem>
                  <SelectItem value="every">
                    {t("pages.scheduler.scheduleKind.every")}
                  </SelectItem>
                  <SelectItem value="at">
                    {t("pages.scheduler.scheduleKind.at")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            {form.scheduleKind === "cron" && (
              <div className="grid gap-1.5">
                <Label>{t("pages.scheduler.field.cronExpr")}</Label>
                <Input
                  value={form.cronExpr}
                  onChange={(e) => setField("cronExpr", e.target.value)}
                  placeholder="0 9 * * 1-5"
                  className="font-mono"
                />
                <p className="text-muted-foreground text-xs">
                  {t("pages.scheduler.field.cronExprHint")}
                </p>
              </div>
            )}

            {form.scheduleKind === "every" && (
              <div className="grid gap-1.5">
                <Label>{t("pages.scheduler.field.everyMinutes")}</Label>
                <Input
                  type="number"
                  min={1}
                  value={form.everyMinutes}
                  onChange={(e) => setField("everyMinutes", e.target.value)}
                />
              </div>
            )}

            {form.scheduleKind === "at" && (
              <div className="grid gap-1.5">
                <Label>{t("pages.scheduler.field.atDatetime")}</Label>
                <Input
                  type="datetime-local"
                  value={form.atDatetime}
                  onChange={(e) => setField("atDatetime", e.target.value)}
                />
              </div>
            )}

            <div className="grid gap-1.5">
              <Label>{t("pages.scheduler.field.message")}</Label>
              <Textarea
                value={form.message}
                onChange={(e) => setField("message", e.target.value)}
                placeholder={t("pages.scheduler.field.messagePlaceholder")}
                rows={3}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>{t("pages.scheduler.field.channel")}</Label>
                <Input
                  value={form.channel}
                  onChange={(e) => setField("channel", e.target.value)}
                  placeholder="default"
                />
              </div>
              <div className="grid gap-1.5">
                <Label>{t("pages.scheduler.field.to")}</Label>
                <Input
                  value={form.to}
                  onChange={(e) => setField("to", e.target.value)}
                  placeholder="pm"
                />
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving || !form.name.trim() || !form.message.trim()}
            >
              {saving ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
