import { createFileRoute } from "@tanstack/react-router"

import { SchedulerPage } from "@/components/scheduler/scheduler-page"

export const Route = createFileRoute("/scheduler")({
  component: SchedulerPage,
})
