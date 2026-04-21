import { IconTrash } from "@tabler/icons-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { LogLevelSelect } from "@/components/logs/log-level-select"
import { LogsPanel } from "@/components/logs/logs-panel"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { useGatewayLogs } from "@/hooks/use-gateway-logs"
import { useLogWrapColumns } from "@/hooks/use-log-wrap-columns"

type ActivityFilter = "all" | "agent" | "subagents"

function isAgentActivityLine(line: string) {
  const lower = line.toLowerCase()
  return (
    lower.includes("agent event:") ||
    lower.includes('"event_kind":') ||
    lower.includes('"agent_id":')
  )
}

function isSubagentActivityLine(line: string) {
  const lower = line.toLowerCase()
  return (
    lower.includes("subturn_") ||
    lower.includes("child_agent_id") ||
    (lower.includes("spawn") && lower.includes("subagent"))
  )
}

export function LogsPage() {
  const { t } = useTranslation()
  const { clearLogs, clearing, logs } = useGatewayLogs()
  const { contentRef, measureRef, wrapColumns } = useLogWrapColumns()
  const [filter, setFilter] = useState<ActivityFilter>("all")

  const visibleLogs = useMemo(() => {
    switch (filter) {
      case "agent":
        return logs.filter((line) => isAgentActivityLine(line))
      case "subagents":
        return logs.filter((line) => isSubagentActivityLine(line))
      default:
        return logs
    }
  }, [filter, logs])

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("navigation.logs")}
        children={
          <>
            <LogLevelSelect />

            <div className="flex items-center gap-1 rounded-md border border-zinc-700 bg-zinc-900/60 p-1">
              <Button
                variant={filter === "all" ? "secondary" : "ghost"}
                size="sm"
                onClick={() => setFilter("all")}
              >
                {t("pages.logs.filter.all")}
              </Button>
              <Button
                variant={filter === "agent" ? "secondary" : "ghost"}
                size="sm"
                onClick={() => setFilter("agent")}
              >
                {t("pages.logs.filter.agent")}
              </Button>
              <Button
                variant={filter === "subagents" ? "secondary" : "ghost"}
                size="sm"
                onClick={() => setFilter("subagents")}
              >
                {t("pages.logs.filter.subagents")}
              </Button>
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={clearLogs}
              disabled={visibleLogs.length === 0 || clearing}
            >
              <IconTrash className="size-4" />
              {t("pages.logs.clear")}
            </Button>
          </>
        }
      />

      <div className="flex flex-1 flex-col gap-4 overflow-hidden p-4 sm:p-8">
        <LogsPanel
          logs={visibleLogs}
          wrapColumns={wrapColumns}
          contentRef={contentRef}
          measureRef={measureRef}
        />
      </div>
    </div>
  )
}
