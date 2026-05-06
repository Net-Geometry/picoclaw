import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { type ModelInfo, getModels, setDefaultModel } from "@/api/models"
import { showSaveSuccessOrRestartToast } from "@/lib/restart-required"
import { refreshGatewayState } from "@/store/gateway"

interface UseChatModelsOptions {
  isConnected: boolean
}

function isLocalModel(model: ModelInfo): boolean {
  const isLocalHostBase = Boolean(
    model.api_base?.includes("localhost") ||
    model.api_base?.includes("127.0.0.1"),
  )

  return (
    model.auth_method === "local" || (!model.auth_method && isLocalHostBase)
  )
}

export function useChatModels({ isConnected }: UseChatModelsOptions) {
  const { t } = useTranslation()
  const [modelList, setModelList] = useState<ModelInfo[]>([])
  const [defaultModelName, setDefaultModelName] = useState("")
  const setDefaultRequestIdRef = useRef(0)
  const autoFixingDefaultRef = useRef(false)

  const loadModels = useCallback(async () => {
    try {
      const data = await getModels()
      setModelList(data.models)

      const availableModels = data.models.filter((m) => m.available)
      const hasAvailableDefault = availableModels.some(
        (m) => m.model_name === data.default_model,
      )

      if (hasAvailableDefault) {
        setDefaultModelName(data.default_model)
        autoFixingDefaultRef.current = false
        return
      }

      if (availableModels.length > 0) {
        const fallbackModelName = availableModels[0].model_name
        setDefaultModelName(fallbackModelName)

        // Keep backend default aligned to an actually available model.
        if (!autoFixingDefaultRef.current) {
          autoFixingDefaultRef.current = true
          try {
            await setDefaultModel(fallbackModelName)
            autoFixingDefaultRef.current = false
          } catch {
            autoFixingDefaultRef.current = false
          }
        }
      } else {
        setDefaultModelName("")
      }
    } catch {
      // silently fail
    }
  }, [])

  useEffect(() => {
    const timerId = setTimeout(() => {
      void loadModels()
    }, 0)

    return () => clearTimeout(timerId)
  }, [isConnected, loadModels])

  const handleSetDefault = useCallback(
    async (modelName: string) => {
      if (modelName === defaultModelName) return

      const selectedModel = modelList.find((m) => m.model_name === modelName)
      if (!selectedModel || !selectedModel.available) {
        toast.error(t("models.action.setDefaultDisabled.unavailable"))
        return
      }

      const requestId = ++setDefaultRequestIdRef.current

      try {
        await setDefaultModel(modelName)
        const data = await getModels()
        if (requestId !== setDefaultRequestIdRef.current) {
          return
        }

        setModelList(data.models)
        if (data.models.some((m) => m.model_name === data.default_model)) {
          setDefaultModelName(data.default_model)
        }
        const gateway = await refreshGatewayState({ force: true })
        showSaveSuccessOrRestartToast(
          t,
          t("models.defaultChangeSuccess"),
          modelName,
          gateway?.restartRequired === true,
        )
      } catch (err) {
        console.error("Failed to set default model:", err)
        toast.error(err instanceof Error ? err.message : t("models.loadError"))
      }
    },
    [defaultModelName, modelList, t],
  )

  const hasAvailableModels = useMemo(
    () => modelList.some((m) => m.available),
    [modelList],
  )

  const oauthModels = useMemo(
    () => modelList.filter((m) => m.available && m.auth_method === "oauth"),
    [modelList],
  )

  const localModels = useMemo(
    () => modelList.filter((m) => m.available && isLocalModel(m)),
    [modelList],
  )

  const apiKeyModels = useMemo(
    () =>
      modelList.filter(
        (m) => m.available && m.auth_method !== "oauth" && !isLocalModel(m),
      ),
    [modelList],
  )

  return {
    defaultModelName,
    hasAvailableModels,
    apiKeyModels,
    oauthModels,
    localModels,
    handleSetDefault,
  }
}
