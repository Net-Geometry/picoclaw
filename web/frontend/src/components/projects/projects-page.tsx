import { IconFile, IconFolder, IconRefresh } from "@tabler/icons-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import {
  type ProjectEntry,
  type ProjectFileResponse,
  type ProjectListResponse,
  fetchProjectEntries,
  fetchProjectFile,
  fetchProjects,
  updateProjectFile,
} from "@/api/projects"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"

export function ProjectsPage() {
  const { t } = useTranslation()

  const [listData, setListData] = useState<ProjectListResponse | null>(null)
  const [loadingProjects, setLoadingProjects] = useState(false)
  const [selectedProject, setSelectedProject] = useState<string>("")
  const [currentPath, setCurrentPath] = useState<string>("")
  const [entries, setEntries] = useState<ProjectEntry[]>([])
  const [loadingEntries, setLoadingEntries] = useState(false)
  const [error, setError] = useState<string>("")
  const [selectedFile, setSelectedFile] = useState<string>("")
  const [fileData, setFileData] = useState<ProjectFileResponse | null>(null)
  const [fileContent, setFileContent] = useState("")
  const [loadingFile, setLoadingFile] = useState(false)
  const [savingFile, setSavingFile] = useState(false)
  const [fileMessage, setFileMessage] = useState("")

  const breadcrumb = useMemo(() => {
    if (!currentPath) return []
    return currentPath.split("/").filter(Boolean)
  }, [currentPath])

  async function loadProjects() {
    setLoadingProjects(true)
    setError("")
    try {
      const data = await fetchProjects()
      setListData(data)
      if (!selectedProject && data.projects.length > 0) {
        setSelectedProject(data.projects[0].name)
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoadingProjects(false)
    }
  }

  async function loadEntries(projectName: string, path = "") {
    setLoadingEntries(true)
    setError("")
    try {
      const data = await fetchProjectEntries(projectName, path)
      setEntries(data.entries)
      setCurrentPath(data.path)
      setSelectedFile("")
      setFileData(null)
      setFileContent("")
      setFileMessage("")
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
      setEntries([])
    } finally {
      setLoadingEntries(false)
    }
  }

  useEffect(() => {
    loadProjects()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!selectedProject) return
    loadEntries(selectedProject, "")
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedProject])

  function openFolder(path: string) {
    if (!selectedProject) return
    loadEntries(selectedProject, path)
  }

  async function openFile(path: string) {
    if (!selectedProject) return
    setLoadingFile(true)
    setFileMessage("")
    try {
      const data = await fetchProjectFile(selectedProject, path)
      setSelectedFile(path)
      setFileData(data)
      setFileContent(data.content)
    } catch (e: unknown) {
      setFileMessage(e instanceof Error ? e.message : String(e))
      setSelectedFile(path)
      setFileData(null)
      setFileContent("")
    } finally {
      setLoadingFile(false)
    }
  }

  async function saveFile() {
    if (!selectedProject || !selectedFile) return
    setSavingFile(true)
    setFileMessage("")
    try {
      const data = await updateProjectFile(
        selectedProject,
        selectedFile,
        fileContent,
      )
      setFileData(data)
      setFileContent(data.content)
      setFileMessage(t("pages.projects.saved"))
      await loadEntries(selectedProject, currentPath)
    } catch (e: unknown) {
      setFileMessage(e instanceof Error ? e.message : String(e))
    } finally {
      setSavingFile(false)
    }
  }

  function openBreadcrumb(index: number) {
    if (!selectedProject) return
    const path = breadcrumb.slice(0, index + 1).join("/")
    loadEntries(selectedProject, path)
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.projects")} />

      <div className="flex flex-1 overflow-hidden">
        <div className="w-72 shrink-0 overflow-y-auto border-r p-3">
          <div className="mb-3 flex items-center justify-between">
            <p className="text-sm font-medium">
              {t("pages.projects.projectList")}
            </p>
            <Button
              size="sm"
              variant="ghost"
              onClick={loadProjects}
              disabled={loadingProjects}
            >
              <IconRefresh className="size-4" />
            </Button>
          </div>

          <div className="space-y-1">
            {listData?.projects.length ? (
              listData.projects.map((p) => (
                <button
                  key={p.name}
                  onClick={() => {
                    setSelectedProject(p.name)
                    setCurrentPath("")
                  }}
                  className={`w-full rounded-md px-3 py-2 text-left text-sm ${selectedProject === p.name ? "bg-muted font-medium" : "hover:bg-muted/60"}`}
                >
                  {p.name}
                </button>
              ))
            ) : (
              <p className="text-muted-foreground text-sm">
                {t("pages.projects.noProjects")}
              </p>
            )}
          </div>
        </div>

        <div className="flex-1 overflow-auto p-4">
          <div className="mb-3 flex items-center gap-2 text-sm">
            <span className="font-medium">{selectedProject || "-"}</span>
            {selectedProject ? (
              <button
                onClick={() => loadEntries(selectedProject, "")}
                className="text-muted-foreground hover:text-foreground"
              >
                /
              </button>
            ) : null}
            {breadcrumb.map((seg, idx) => (
              <button
                key={`${seg}-${idx}`}
                onClick={() => openBreadcrumb(idx)}
                className="text-muted-foreground hover:text-foreground"
              >
                {seg}
                {idx < breadcrumb.length - 1 ? "/" : ""}
              </button>
            ))}
          </div>

          {error ? (
            <p className="text-destructive mb-3 text-sm">{error}</p>
          ) : null}

          {loadingEntries ? (
            <p className="text-muted-foreground text-sm">
              {t("labels.loading")}
            </p>
          ) : (
            <div className="overflow-x-auto rounded-md border">
              <table className="w-full text-sm">
                <thead className="bg-muted/40 border-b">
                  <tr>
                    <th className="px-4 py-2 text-left font-medium">
                      {t("pages.projects.name")}
                    </th>
                    <th className="px-4 py-2 text-left font-medium">
                      {t("pages.projects.type")}
                    </th>
                    <th className="px-4 py-2 text-left font-medium">
                      {t("pages.projects.size")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {entries.length === 0 ? (
                    <tr>
                      <td
                        colSpan={3}
                        className="text-muted-foreground px-4 py-6 text-center"
                      >
                        {t("pages.projects.emptyFolder")}
                      </td>
                    </tr>
                  ) : (
                    entries.map((entry) => (
                      <tr key={entry.path} className="border-b last:border-0">
                        <td className="px-4 py-2">
                          {entry.type === "dir" ? (
                            <button
                              onClick={() => openFolder(entry.path)}
                              className="flex items-center gap-2 text-left hover:underline"
                            >
                              <IconFolder className="size-4 text-amber-500" />
                              {entry.name}
                            </button>
                          ) : (
                            <button
                              onClick={() => openFile(entry.path)}
                              className="flex items-center gap-2 text-left hover:underline"
                            >
                              <IconFile className="text-muted-foreground size-4" />
                              {entry.name}
                            </button>
                          )}
                        </td>
                        <td className="px-4 py-2">
                          {entry.type === "dir"
                            ? t("pages.projects.folder")
                            : t("pages.projects.file")}
                        </td>
                        <td className="px-4 py-2">
                          {entry.type === "dir" ? "-" : (entry.size ?? 0)}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}

          <div className="mt-4 rounded-md border p-3">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium">
                {t("pages.projects.editor")}: {selectedFile || "-"}
              </p>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => selectedFile && openFile(selectedFile)}
                  disabled={!selectedFile || loadingFile}
                >
                  {t("pages.projects.reload")}
                </Button>
                <Button
                  size="sm"
                  onClick={saveFile}
                  disabled={!selectedFile || loadingFile || savingFile}
                >
                  {t("pages.projects.save")}
                </Button>
              </div>
            </div>

            {fileMessage ? (
              <p className="text-muted-foreground mb-2 text-sm">
                {fileMessage}
              </p>
            ) : null}

            {loadingFile ? (
              <p className="text-muted-foreground text-sm">
                {t("labels.loading")}
              </p>
            ) : (
              <textarea
                className="border-input bg-background focus-visible:ring-ring min-h-[320px] w-full resize-y rounded-md border p-3 font-mono text-sm focus-visible:ring-1 focus-visible:outline-none"
                placeholder={t("pages.projects.selectFileToEdit")}
                value={fileContent}
                onChange={(e) => setFileContent(e.target.value)}
                disabled={!selectedFile || savingFile || !fileData?.editable}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
