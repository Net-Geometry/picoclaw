import { launcherFetch } from "@/api/http"

export interface AgentModel {
  primary: string
  fallbacks?: string[]
}

export interface Agent {
  id: string
  name: string
  mode: string
  model?: AgentModel
  skills?: string[]
  allow_agents?: string[]
  is_default?: boolean
}

export interface AgentListResponse {
  agents: Agent[]
  available_models: string[]
  available_skills: string[]
  default_model_name: string
}

export interface CreateAgentRequest {
  id: string
  name?: string
  mode?: string
  model?: AgentModel | null
  skills?: string[]
  allow_agents?: string[]
}

export interface UpdateAgentRequest {
  name?: string
  mode?: string
  model?: AgentModel | null
  skills?: string[]
  allow_agents?: string[]
}

export async function fetchAgents(): Promise<AgentListResponse> {
  const res = await launcherFetch("/api/agents")
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function createAgent(payload: CreateAgentRequest): Promise<Agent> {
  const res = await launcherFetch("/api/agents", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateAgent(
  id: string,
  payload: UpdateAgentRequest,
): Promise<Agent> {
  const res = await launcherFetch(`/api/agents/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
