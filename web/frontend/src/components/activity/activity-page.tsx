import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { launcherFetch } from "@/api/http"
import { AnsiLogLine } from "@/components/logs/ansi-log-line"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useGatewayLogs } from "@/hooks/use-gateway-logs"

type ActivityEvent = {
  id: string
  kind: string
  agentId: string
  childAgentId: string
  status: string
  raw: string
}

type AgentNode = {
  id: string
  mode: "active" | "passive"
  allowAgents: string[]
}

type ActivityViewMode = "mixed" | "live" | "simulated"

function extractField(line: string, key: string) {
  const jsonMatch = line.match(new RegExp(`"${key}"\\s*:\\s*"([^"]+)"`))
  if (jsonMatch?.[1]) {
    return jsonMatch[1]
  }

  const kvMatch = line.match(new RegExp(`(?:^|[\\s,{])${key}=([^\\s,}]+)`))
  if (kvMatch?.[1]) {
    return kvMatch[1]
  }

  return ""
}

function extractEventKind(line: string) {
  const explicit = line.match(/agent event:\s*([a-z_]+)/i)
  if (explicit?.[1]) {
    return explicit[1].toLowerCase()
  }
  const kind = extractField(line, "event_kind")
  return kind.toLowerCase()
}

function isActivityLine(line: string) {
  const lower = line.toLowerCase()
  return (
    lower.includes("agent event:") ||
    lower.includes('"event_kind":') ||
    lower.includes("event_kind=")
  )
}

function isSubagentKind(kind: string) {
  return kind.startsWith("subturn_")
}

function parseActivityEvents(logs: string[]) {
  const items: ActivityEvent[] = []

  for (let i = 0; i < logs.length; i += 1) {
    const line = logs[i]
    if (!isActivityLine(line)) {
      continue
    }

    const kind = extractEventKind(line)
    const agentId = extractField(line, "agent_id")
    const childAgentId = extractField(line, "child_agent_id")
    const status = extractField(line, "status")

    items.push({
      id: `${i}-${kind || "event"}`,
      kind: kind || "unknown",
      agentId,
      childAgentId,
      status,
      raw: line,
    })
  }

  return items
}

function buildSpawnGraph(events: ActivityEvent[]) {
  const graph = new Map<string, Set<string>>()

  for (const evt of events) {
    if (evt.kind !== "subturn_spawn" || !evt.agentId || !evt.childAgentId) {
      continue
    }
    if (!graph.has(evt.agentId)) {
      graph.set(evt.agentId, new Set())
    }
    graph.get(evt.agentId)?.add(evt.childAgentId)
  }

  return [...graph.entries()].map(([parent, children]) => ({
    parent,
    children: [...children],
  }))
}

function countActiveSubagents(events: ActivityEvent[]) {
  const active = new Map<string, boolean>()
  for (const evt of events) {
    if (!evt.childAgentId) {
      continue
    }
    if (evt.kind === "subturn_spawn") {
      active.set(evt.childAgentId, true)
    }
    if (evt.kind === "subturn_end") {
      active.set(evt.childAgentId, false)
    }
  }
  return [...active.values()].filter(Boolean).length
}

function parseAgentsFromConfig(payload: unknown): AgentNode[] {
  const root = typeof payload === "object" && payload ? payload : {}
  const agents = (root as { agents?: { list?: unknown } }).agents
  const list = Array.isArray(agents?.list) ? agents.list : []

  return list
    .map((item): AgentNode | null => {
      if (typeof item !== "object" || !item) {
        return null
      }
      const obj = item as {
        id?: unknown
        mode?: unknown
        subagents?: { allow_agents?: unknown }
      }
      if (typeof obj.id !== "string" || !obj.id.trim()) {
        return null
      }
      const mode = obj.mode === "passive" ? "passive" : "active"
      const allowAgents = Array.isArray(obj.subagents?.allow_agents)
        ? obj.subagents.allow_agents.filter(
            (value): value is string =>
              typeof value === "string" && value.trim() !== "",
          )
        : []
      return {
        id: obj.id,
        mode,
        allowAgents,
      }
    })
    .filter((item): item is AgentNode => item !== null)
}

function buildSimulationEdges(agents: AgentNode[]) {
  const pm = agents.find((agent) => agent.id === "pm")
  if (!pm) {
    return [] as Array<{ parent: string; child: string }>
  }

  return pm.allowAgents
    .map((child) => ({ parent: pm.id, child }))
    .filter((edge) => agents.some((agent) => agent.id === edge.child))
}

function graphFromEdges(edges: Array<{ parent: string; child: string }>) {
  const graph = new Map<string, Set<string>>()

  for (const edge of edges) {
    if (!graph.has(edge.parent)) {
      graph.set(edge.parent, new Set())
    }
    graph.get(edge.parent)?.add(edge.child)
  }

  return [...graph.entries()].map(([parent, children]) => ({
    parent,
    children: [...children],
  }))
}

export function ActivityPage() {
  const { t } = useTranslation()
  const { logs } = useGatewayLogs()
  const [availableAgents, setAvailableAgents] = useState<AgentNode[]>([])
  const [simulationStep, setSimulationStep] = useState(0)
  const [viewMode, setViewMode] = useState<ActivityViewMode>("mixed")

  useEffect(() => {
    let mounted = true

    const loadConfig = async () => {
      try {
        const res = await launcherFetch("/api/config")
        if (!res.ok) {
          return
        }
        const json = await res.json()
        if (!mounted) {
          return
        }
        setAvailableAgents(parseAgentsFromConfig(json))
      } catch {
        // Ignore config fetch errors to keep page resilient.
      }
    }

    void loadConfig()
    return () => {
      mounted = false
    }
  }, [])

  const liveEvents = useMemo(() => parseActivityEvents(logs), [logs])
  const liveSpawnGraph = useMemo(
    () => buildSpawnGraph(liveEvents),
    [liveEvents],
  )
  const simulationEdges = useMemo(
    () => buildSimulationEdges(availableAgents),
    [availableAgents],
  )
  const simulatedEvents = useMemo(
    () =>
      simulationEdges.map((edge, index) => ({
        id: `sim-${edge.parent}-${edge.child}-${index}`,
        kind: "subturn_spawn",
        agentId: edge.parent,
        childAgentId: edge.child,
        status: "simulated",
        raw: `simulation parent=${edge.parent} child=${edge.child}`,
      })),
    [simulationEdges],
  )
  const simulatedSpawnGraph = useMemo(
    () => graphFromEdges(simulationEdges),
    [simulationEdges],
  )

  useEffect(() => {
    if (simulationEdges.length === 0) {
      return
    }

    const timer = setInterval(() => {
      setSimulationStep((prev) => (prev + 1) % simulationEdges.length)
    }, 1300)

    return () => clearInterval(timer)
  }, [simulationEdges.length])

  const activeSimulationEdge =
    simulationEdges.length > 0
      ? simulationEdges[simulationStep % simulationEdges.length]
      : null

  const displayedEvents = useMemo(() => {
    if (viewMode === "live") {
      return liveEvents
    }
    if (viewMode === "simulated") {
      return simulatedEvents
    }
    if (liveEvents.length > 0) {
      return liveEvents
    }
    return simulatedEvents
  }, [liveEvents, simulatedEvents, viewMode])

  const displayedSpawnGraph = useMemo(() => {
    if (viewMode === "live") {
      return liveSpawnGraph
    }
    if (viewMode === "simulated") {
      return simulatedSpawnGraph
    }
    if (liveSpawnGraph.length > 0) {
      return liveSpawnGraph
    }
    return simulatedSpawnGraph
  }, [liveSpawnGraph, simulatedSpawnGraph, viewMode])

  const displayedEventCount = displayedEvents.length
  const displayedSubagentCount = displayedEvents.filter((evt) =>
    isSubagentKind(evt.kind),
  ).length

  const displayedAgentCount = useMemo(() => {
    const ids = new Set<string>()
    for (const evt of displayedEvents) {
      if (evt.agentId) {
        ids.add(evt.agentId)
      }
      if (evt.childAgentId) {
        ids.add(evt.childAgentId)
      }
    }
    return ids.size
  }, [displayedEvents])

  const displayedActiveSubagentCount = countActiveSubagents(displayedEvents)

  const modeDescription =
    viewMode === "live"
      ? t("pages.activity.liveMode")
      : viewMode === "simulated"
        ? t("pages.activity.simulationMode")
        : t("pages.activity.mixedMode")

  const simulationHint = `${t("pages.activity.liveEvents")}: ${liveEvents.length} • ${t("pages.activity.simulatedEvents")}: ${simulatedEvents.length}`

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.activity")} />

      <div className="flex flex-1 flex-col gap-4 overflow-hidden p-4 sm:p-8">
        <div className="grid gap-4 md:grid-cols-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle>{t("pages.activity.cards.totalEvents")}</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">
              {displayedEventCount}
            </CardContent>
          </Card>
          <Card size="sm">
            <CardHeader>
              <CardTitle>{t("pages.activity.cards.subagentEvents")}</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">
              {displayedSubagentCount}
            </CardContent>
          </Card>
          <Card size="sm">
            <CardHeader>
              <CardTitle>{t("pages.activity.cards.agentsSeen")}</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">
              {displayedAgentCount}
            </CardContent>
          </Card>
          <Card size="sm">
            <CardHeader>
              <CardTitle>{t("pages.activity.cards.activeSubagents")}</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">
              {displayedActiveSubagentCount}
            </CardContent>
          </Card>
        </div>

        <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-3">
          <div className="min-h-0 space-y-4 lg:col-span-1">
            <Card className="min-h-0">
              <CardHeader>
                <CardTitle>{t("pages.activity.spawnGraphTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="min-h-0 flex-1 overflow-hidden">
                <ScrollArea className="h-[18vh] lg:h-[24vh]">
                  {displayedSpawnGraph.length === 0 ? (
                    <div className="text-muted-foreground text-sm">
                      {t("pages.activity.emptySpawnGraph")}
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {displayedSpawnGraph.map((node) => (
                        <div
                          key={node.parent}
                          className="rounded-md border p-3"
                        >
                          <div className="mb-2 text-sm font-medium">
                            {node.parent}
                          </div>
                          <div className="flex flex-wrap gap-1.5">
                            {node.children.map((child) => (
                              <Badge
                                key={`${node.parent}-${child}`}
                                variant="secondary"
                              >
                                {child}
                              </Badge>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </ScrollArea>
              </CardContent>
            </Card>

            <Card className="min-h-0">
              <CardHeader>
                <CardTitle>{t("pages.activity.simulationTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {availableAgents.length === 0 ? (
                  <div className="text-muted-foreground text-sm">
                    {t("pages.activity.simulationNoAgents")}
                  </div>
                ) : (
                  <>
                    <div className="text-muted-foreground text-xs">
                      {modeDescription}
                    </div>
                    <div className="text-muted-foreground text-xs">
                      {simulationHint}
                    </div>

                    <div className="space-y-2">
                      {availableAgents.map((agent) => {
                        const isHot =
                          activeSimulationEdge?.parent === agent.id ||
                          activeSimulationEdge?.child === agent.id
                        return (
                          <div
                            key={agent.id}
                            className={`flex items-center justify-between rounded-md border px-3 py-2 text-xs transition-all ${
                              isHot
                                ? "border-blue-500/50 bg-blue-500/10"
                                : "border-border"
                            }`}
                          >
                            <span className="font-medium">{agent.id}</span>
                            <Badge
                              variant={
                                agent.mode === "active"
                                  ? "default"
                                  : "secondary"
                              }
                            >
                              {agent.mode}
                            </Badge>
                          </div>
                        )
                      })}
                    </div>

                    {activeSimulationEdge && (
                      <div className="bg-muted/40 rounded-md border px-3 py-2 text-xs">
                        <span className="inline-block h-2 w-2 animate-ping rounded-full bg-blue-500 align-middle" />
                        <span className="ml-2 align-middle">
                          {activeSimulationEdge.parent} →{" "}
                          {activeSimulationEdge.child}
                        </span>
                      </div>
                    )}
                  </>
                )}
              </CardContent>
            </Card>
          </div>

          <Card className="min-h-0 lg:col-span-2">
            <CardHeader>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <CardTitle>{t("pages.activity.timelineTitle")}</CardTitle>
                <div className="flex items-center gap-1 rounded-md border p-1">
                  <Button
                    size="sm"
                    variant={viewMode === "live" ? "default" : "ghost"}
                    className="h-7"
                    onClick={() => setViewMode("live")}
                  >
                    {t("pages.activity.mode.live")}
                  </Button>
                  <Button
                    size="sm"
                    variant={viewMode === "simulated" ? "default" : "ghost"}
                    className="h-7"
                    onClick={() => setViewMode("simulated")}
                  >
                    {t("pages.activity.mode.simulated")}
                  </Button>
                  <Button
                    size="sm"
                    variant={viewMode === "mixed" ? "default" : "ghost"}
                    className="h-7"
                    onClick={() => setViewMode("mixed")}
                  >
                    {t("pages.activity.mode.mixed")}
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent className="min-h-0 flex-1 overflow-hidden">
              <ScrollArea className="h-[42vh] lg:h-full">
                {displayedEvents.length === 0 ? (
                  <div className="text-muted-foreground text-sm">
                    {t("pages.activity.emptyTimeline")}
                  </div>
                ) : (
                  <div className="space-y-2">
                    {displayedEvents
                      .slice()
                      .reverse()
                      .map((evt) => (
                        <div key={evt.id} className="rounded-md border p-2.5">
                          <div className="mb-1 flex flex-wrap items-center gap-2">
                            <Badge variant="outline">{evt.kind}</Badge>
                            {evt.agentId && (
                              <Badge variant="secondary">{evt.agentId}</Badge>
                            )}
                            {evt.childAgentId && (
                              <Badge variant="default">
                                {evt.childAgentId}
                              </Badge>
                            )}
                            {evt.status && (
                              <Badge
                                variant={
                                  evt.status === "simulated"
                                    ? "secondary"
                                    : "ghost"
                                }
                              >
                                {evt.status}
                              </Badge>
                            )}
                          </div>
                          <AnsiLogLine line={evt.raw} wrapColumns={140} />
                        </div>
                      ))}
                  </div>
                )}
              </ScrollArea>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
