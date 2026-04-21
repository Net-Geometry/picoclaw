import { IconCheck, IconLoader2, IconUserCog } from "@tabler/icons-react"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  type Agent,
  type AgentListResponse,
  type AgentModel,
  createAgent,
  fetchAgents,
  updateAgent,
} from "@/api/agents"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// ─── Component ────────────────────────────────────────────────────────────────

export function AgentConfigurePage() {
  const { t } = useTranslation()

  const [data, setData] = useState<AgentListResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  // Editable state for selected agent
  const [editMode, setEditMode] = useState<string>("")
  const [editPrimary, setEditPrimary] = useState<string>("")
  const [editFallbacks, setEditFallbacks] = useState<string>("")
  const [editSkills, setEditSkills] = useState<Set<string>>(new Set())
  const [editAllowAgents, setEditAllowAgents] = useState<Set<string>>(new Set())
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [newId, setNewId] = useState("")
  const [newName, setNewName] = useState("")
  const [newMode, setNewMode] = useState("active")
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  useEffect(() => {
    fetchAgents()
      .then((d) => {
        setData(d)
        if (d.agents.length > 0) {
          selectAgent(d.agents[0])
        }
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  function selectAgent(agent: Agent) {
    setSelectedId(agent.id)
    setEditMode(agent.mode ?? "active")
    setEditPrimary(agent.model?.primary ?? "")
    setEditFallbacks((agent.model?.fallbacks ?? []).join(", "))
    setEditSkills(new Set(agent.skills ?? []))
    setEditAllowAgents(new Set(agent.allow_agents ?? []))
    setSaved(false)
    setSaveError(null)
  }

  function toggleSet(
    set: Set<string>,
    item: string,
    setter: (s: Set<string>) => void,
  ) {
    const next = new Set(set)
    if (next.has(item)) next.delete(item)
    else next.add(item)
    setter(next)
  }

  async function handleSave() {
    if (!selectedId) return
    setSaving(true)
    setSaveError(null)
    setSaved(false)
    try {
      const fallbackList = editFallbacks
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean)
      const model: AgentModel = editPrimary
        ? { primary: editPrimary, fallbacks: fallbackList }
        : { primary: "" }
      const updated = await updateAgent(selectedId, {
        mode: editMode,
        model,
        skills: Array.from(editSkills),
        allow_agents: Array.from(editAllowAgents),
      })
      setData((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          agents: prev.agents.map((a) => (a.id === updated.id ? updated : a)),
        }
      })
      setSaved(true)
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : "Save failed")
    } finally {
      setSaving(false)
    }
  }

  async function handleCreateAgent() {
    const id = newId.trim()
    if (!id) {
      setCreateError(t("pages.agentConfigure.agentIdRequired"))
      return
    }
    setCreating(true)
    setCreateError(null)
    try {
      const created = await createAgent({
        id,
        name: newName.trim() || id,
        mode: newMode,
      })
      setData((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          agents: [...prev.agents, created],
        }
      })
      selectAgent(created)
      setNewId("")
      setNewName("")
      setNewMode("active")
    } catch (e: unknown) {
      setCreateError(e instanceof Error ? e.message : "Create failed")
    } finally {
      setCreating(false)
    }
  }

  const selectedAgent = data?.agents.find((a) => a.id === selectedId)

  if (loading) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader title={t("navigation.configure")} />
        <div className="flex flex-1 items-center justify-center">
          <IconLoader2 className="text-muted-foreground size-6 animate-spin" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader title={t("navigation.configure")} />
        <div className="text-destructive p-6">{error}</div>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.configure")} />
      <div className="flex flex-1 overflow-hidden">
        {/* ── Agent list sidebar ── */}
        <div className="bg-muted/30 w-56 shrink-0 overflow-y-auto border-r p-2">
          <div className="bg-background mb-3 space-y-2 rounded-md border p-2">
            <p className="text-muted-foreground text-xs font-medium">
              {t("pages.agentConfigure.addAgent")}
            </p>
            <input
              className="border-input placeholder:text-muted-foreground focus-visible:ring-ring flex h-8 w-full rounded-md border bg-transparent px-2 text-xs shadow-sm focus-visible:ring-1 focus-visible:outline-none"
              placeholder={t("pages.agentConfigure.agentId")}
              value={newId}
              onChange={(e) => setNewId(e.target.value)}
            />
            <input
              className="border-input placeholder:text-muted-foreground focus-visible:ring-ring flex h-8 w-full rounded-md border bg-transparent px-2 text-xs shadow-sm focus-visible:ring-1 focus-visible:outline-none"
              placeholder={t("pages.agentConfigure.agentName")}
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
            />
            <Select value={newMode} onValueChange={setNewMode}>
              <SelectTrigger className="h-8 w-full text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="active">active</SelectItem>
                <SelectItem value="passive">passive</SelectItem>
              </SelectContent>
            </Select>
            <Button
              size="sm"
              className="w-full"
              onClick={handleCreateAgent}
              disabled={creating}
            >
              {creating ? (
                <IconLoader2 className="mr-2 size-3 animate-spin" />
              ) : null}
              {t("pages.agentConfigure.create")}
            </Button>
            {createError && (
              <p className="text-destructive text-xs">{createError}</p>
            )}
          </div>

          {data?.agents.map((agent) => (
            <button
              key={agent.id}
              onClick={() => selectAgent(agent)}
              className={[
                "hover:bg-muted w-full rounded-md px-3 py-2 text-left text-sm transition-colors",
                selectedId === agent.id ? "bg-muted font-medium" : "",
              ].join(" ")}
            >
              <div className="truncate">{agent.name || agent.id}</div>
              {agent.is_default && (
                <Badge variant="secondary" className="mt-0.5 text-[10px]">
                  default
                </Badge>
              )}
            </button>
          ))}
        </div>

        {/* ── Config panel ── */}
        {selectedAgent ? (
          <div className="flex-1 overflow-y-auto p-6">
            <div className="max-w-xl space-y-6">
              <div>
                <h2 className="text-base font-semibold">
                  {selectedAgent.name || selectedAgent.id}
                </h2>
                <p className="text-muted-foreground text-sm">
                  {selectedAgent.id}
                </p>
              </div>

              {/* Mode */}
              <section className="space-y-2">
                <label className="text-sm font-medium">
                  {t("pages.agentConfigure.mode")}
                </label>
                <Select value={editMode} onValueChange={setEditMode}>
                  <SelectTrigger className="w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">active</SelectItem>
                    <SelectItem value="passive">passive</SelectItem>
                  </SelectContent>
                </Select>
              </section>

              {/* Model */}
              <section className="space-y-2">
                <label className="text-sm font-medium">
                  {t("pages.agentConfigure.primaryModel")}
                </label>
                <p className="text-muted-foreground text-xs">
                  {t("pages.agentConfigure.primaryModelHint", {
                    defaultValue: data?.default_model_name,
                  })}
                </p>
                <Select
                  value={editPrimary || "__default__"}
                  onValueChange={(v) =>
                    setEditPrimary(v === "__default__" ? "" : v)
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__default__">
                      (use default: {data?.default_model_name})
                    </SelectItem>
                    {data?.available_models.map((m) => (
                      <SelectItem key={m} value={m}>
                        {m}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <label className="text-sm font-medium">
                  {t("pages.agentConfigure.fallbackModels")}
                </label>
                <input
                  className="border-input placeholder:text-muted-foreground focus-visible:ring-ring flex h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:ring-1 focus-visible:outline-none"
                  placeholder="model-a, model-b"
                  value={editFallbacks}
                  onChange={(e) => setEditFallbacks(e.target.value)}
                />
              </section>

              {/* Skills */}
              <section className="space-y-2">
                <label className="text-sm font-medium">
                  {t("pages.agentConfigure.skills")}
                </label>
                <div className="max-h-48 overflow-y-auto rounded-md border p-2">
                  {data?.available_skills.length === 0 && (
                    <p className="text-muted-foreground text-xs">
                      {t("pages.agentConfigure.noSkills")}
                    </p>
                  )}
                  {data?.available_skills.map((skill) => (
                    <label
                      key={skill}
                      className="hover:bg-muted flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm"
                    >
                      <input
                        type="checkbox"
                        checked={editSkills.has(skill)}
                        onChange={() =>
                          toggleSet(editSkills, skill, setEditSkills)
                        }
                        className="accent-primary"
                      />
                      {skill}
                    </label>
                  ))}
                </div>
              </section>

              {/* Allowed subagents */}
              <section className="space-y-2">
                <label className="text-sm font-medium">
                  {t("pages.agentConfigure.allowedSubagents")}
                </label>
                <div className="max-h-48 overflow-y-auto rounded-md border p-2">
                  {data?.agents
                    .filter((a) => a.id !== selectedId)
                    .map((a) => (
                      <label
                        key={a.id}
                        className="hover:bg-muted flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm"
                      >
                        <input
                          type="checkbox"
                          checked={editAllowAgents.has(a.id)}
                          onChange={() =>
                            toggleSet(editAllowAgents, a.id, setEditAllowAgents)
                          }
                          className="accent-primary"
                        />
                        {a.name || a.id}
                      </label>
                    ))}
                </div>
              </section>

              {/* Save */}
              <div className="flex items-center gap-3">
                <Button onClick={handleSave} disabled={saving}>
                  {saving ? (
                    <IconLoader2 className="mr-2 size-4 animate-spin" />
                  ) : null}
                  {t("pages.agentConfigure.save")}
                </Button>
                {saved && (
                  <span className="flex items-center gap-1 text-sm text-green-600">
                    <IconCheck className="size-4" />
                    {t("pages.agentConfigure.saved")}
                  </span>
                )}
                {saveError && (
                  <span className="text-destructive text-sm">{saveError}</span>
                )}
              </div>
            </div>
          </div>
        ) : (
          <div className="text-muted-foreground flex flex-1 items-center justify-center">
            <div className="text-center">
              <IconUserCog className="mx-auto mb-2 size-10 opacity-30" />
              <p className="text-sm">{t("pages.agentConfigure.selectAgent")}</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
