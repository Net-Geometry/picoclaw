import { createFileRoute } from "@tanstack/react-router"

import { AgentConfigurePage } from "@/components/agent/configure/agent-configure-page"

export const Route = createFileRoute("/agent/configure")({
  component: AgentConfigureRoute,
})

function AgentConfigureRoute() {
  return <AgentConfigurePage />
}
