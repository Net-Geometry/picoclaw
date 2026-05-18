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

export interface ProjectFileUploadResponse {
  project: string
  files: Array<{
    project: string
    path: string
    name: string
    size: number
  }>
}

export interface ContainerRunRequest {
  project_name: string
  project_path?: string
  image: string
  command: string[]
  timeout?: number
  env?: string[]
  work_dir?: string
}

export interface ContainerRunResponse {
  container_id?: string
  exit_code: number
  stdout: string
  stderr: string
  duration_ms: number
  status: string
}

export interface TestRunRequest {
  project_name: string
  project_path?: string
  test_path?: string
  verbose?: boolean
  timeout?: number
}

export interface TestRunResponse {
  test_name: string
  passed: number
  failed: number
  skipped: number
  duration_ms: number
  stdout: string
  stderr: string
  status: string
  exit_code: number
}

export interface ScriptRunRequest {
  project_name: string
  project_path?: string
  script_path: string
  args?: string[]
  env?: string[]
  timeout?: number
}

export interface ScriptRunResponse {
  exit_code: number
  stdout: string
  stderr: string
  duration_ms: number
  status: string
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

export async function uploadProjectFile(
  projectName: string,
  files: File[],
  targetPath = "",
): Promise<ProjectFileUploadResponse> {
  const formData = new FormData()
  for (const file of files) {
    formData.append("file", file)
  }

  const query = targetPath ? `?path=${encodeURIComponent(targetPath)}` : ""
  const res = await launcherFetch(
    `/api/projects/${encodeURIComponent(projectName)}/upload${query}`,
    {
      method: "POST",
      body: formData,
    },
  )
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function downloadProjectFile(
  projectName: string,
  filePath: string,
): Promise<Blob> {
  const query = `?path=${encodeURIComponent(filePath)}`
  const res = await launcherFetch(
    `/api/projects/${encodeURIComponent(projectName)}/download${query}`,
  )
  if (!res.ok) throw new Error(await res.text())
  return res.blob()
}

export async function runContainer(
  req: ContainerRunRequest,
): Promise<ContainerRunResponse> {
  const res = await launcherFetch("/api/containers/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function runTest(req: TestRunRequest): Promise<TestRunResponse> {
  const res = await launcherFetch("/api/tests/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function runScript(
  req: ScriptRunRequest,
): Promise<ScriptRunResponse> {
  const res = await launcherFetch("/api/scripts/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
