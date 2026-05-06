/**
 * Task-Based Model Routing Hook
 * Autonomously selects the best available model based on task complexity
 */
import { useCallback, useMemo } from "react"

import type { ModelInfo } from "@/api/models"
import { selectModelForComplexity, tierModels } from "@/lib/model-tier"
import {
  classifyComplexity,
  extractTaskFeatures,
  scoreTaskComplexity,
} from "@/lib/task-complexity"

interface UseTaskBasedModelRoutingOptions {
  availableModels: ModelInfo[]
  defaultModelName: string
}

interface RoutingResult {
  selectedModelName: string
  selectedModel: ModelInfo | null
  complexityScore: number
  complexityClass: "simple" | "medium" | "complex"
}

export function useTaskBasedModelRouting({
  availableModels,
  defaultModelName,
}: UseTaskBasedModelRoutingOptions) {
  // Tier available models
  const tieredModels = useMemo(
    () => tierModels(availableModels),
    [availableModels],
  )

  // Route task to best model based on complexity
  const routeTask = useCallback(
    (message: string, hasAttachments: boolean = false): RoutingResult => {
      // Extract task features
      const features = extractTaskFeatures(message, hasAttachments)

      // Score complexity
      const complexityScore = scoreTaskComplexity(features)

      // Classify complexity
      const complexityClass = classifyComplexity(complexityScore)

      // Select model for this complexity tier
      const selectedModel = selectModelForComplexity(
        complexityClass,
        tieredModels,
      )

      const selectedModelName =
        selectedModel?.model_name || defaultModelName || ""

      return {
        selectedModelName,
        selectedModel,
        complexityScore,
        complexityClass,
      }
    },
    [tieredModels, defaultModelName],
  )

  return {
    routeTask,
    tieredModels,
  }
}
