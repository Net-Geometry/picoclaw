import { launcherFetch } from "@/api/http"

export interface ProjectItem {
  name: string
}

export interface ProjectListResponse {
  workspace: string
  root: string
  projects: ProjectItem[]
}

export interface ProjectEntry {
  name: string
  path: string
  type: "file" | "dir"
  size?: number
  modified_ms: number
}

export interface ProjectEntriesResponse {
  project: string
  path: string
  entries: ProjectEntry[]
}

export interface ProjectFileResponse {
  project: string
  path: string
  content: string
  size: number
  editable: boolean
}

export async function fetchProjects(): Promise<ProjectListResponse> {
  const res = await launcherFetch("/api/projects")
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function fetchProjectEntries(
  projectName: string,
  relPath = "",
): Promise<ProjectEntriesResponse> {
  const query = relPath ? `?path=${encodeURIComponent(relPath)}` : ""
  const res = await launcherFetch(
    `/api/projects/${encodeURIComponent(projectName)}/entries${query}`,
  )
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function fetchProjectFile(
  projectName: string,
  filePath: string,
): Promise<ProjectFileResponse> {
  const query = `?path=${encodeURIComponent(filePath)}`
  const res = await launcherFetch(
    `/api/projects/${encodeURIComponent(projectName)}/file${query}`,
  )
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateProjectFile(
  projectName: string,
  filePath: string,
  content: string,
): Promise<ProjectFileResponse> {
  const query = `?path=${encodeURIComponent(filePath)}`
  const res = await launcherFetch(
    `/api/projects/${encodeURIComponent(projectName)}/file${query}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content }),
    },
  )
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
