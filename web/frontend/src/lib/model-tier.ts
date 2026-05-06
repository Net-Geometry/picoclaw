/**
 * Model Tier Classification
 * Groups models by capability tier for autonomous selection
 */

import type { ModelInfo } from "@/api/models"

export type ModelTier = "light" | "balanced" | "heavy"

export interface TieredModels {
  light: ModelInfo[] // Fast, cost-effective (good for simple tasks)
  balanced: ModelInfo[] // Medium capability (general purpose)
  heavy: ModelInfo[] // Most capable (complex reasoning, coding)
}

// Model capability patterns (provider/model combinations)
const LIGHT_MODEL_PATTERNS = [
  /gemini.*flash/i,
  /gpt-4o-mini/i,
  /claude-3\.5-haiku/i,
  /qwen.*0\.5b|qwen.*1b|qwen.*2b|qwen-lite/i,
  /llama-2-7b/i,
  /mistral-7b/i,
  /phi-/i,
  /deepseek-lite/i,
  /glm-4-flash/i,
]

const HEAVY_MODEL_PATTERNS = [
  /gpt-5\.[0-9]+/i, // GPT-5.x family
  /gpt-4-turbo/i,
  /claude-(opus|sonnet-4)/i,
  /claude-sonnet-4\.[0-9]/i,
  /claude-3-opus/i,
  /gemini-2\.0-pro/i,
  /qwen.*72b|qwen.*110b|qwen-max/i,
  /llama-3\.(1-70b|8-70b|405b)/i,
  /mistral-large/i,
  /deepseek-chat|deepseek-v[34]/i,
]

function getTierForModel(model: ModelInfo): ModelTier {
  if (!model.available) {
    return "balanced" // Treat unavailable as lowest selectable
  }

  const modelStr = `${model.provider}/${model.model}`

  if (HEAVY_MODEL_PATTERNS.some((pattern) => pattern.test(modelStr))) {
    return "heavy"
  }

  if (LIGHT_MODEL_PATTERNS.some((pattern) => pattern.test(modelStr))) {
    return "light"
  }

  // Default to balanced for unknown models
  return "balanced"
}

export function tierModels(models: ModelInfo[]): TieredModels {
  const tiered: TieredModels = {
    light: [],
    balanced: [],
    heavy: [],
  }

  for (const model of models) {
    if (model.available) {
      const tier = getTierForModel(model)
      tiered[tier].push(model)
    }
  }

  return tiered
}

export function selectModelForComplexity(
  complexity: "simple" | "medium" | "complex",
  tieredModels: TieredModels
): ModelInfo | null {
  switch (complexity) {
    case "simple":
      // Prefer light, fall back to balanced, then heavy
      return (
        tieredModels.light[0] ??
        tieredModels.balanced[0] ??
        tieredModels.heavy[0] ??
        null
      )

    case "medium":
      // Prefer balanced, fall back to heavy, then light
      return (
        tieredModels.balanced[0] ??
        tieredModels.heavy[0] ??
        tieredModels.light[0] ??
        null
      )

    case "complex":
      // Prefer heavy, fall back to balanced, then light
      return (
        tieredModels.heavy[0] ??
        tieredModels.balanced[0] ??
        tieredModels.light[0] ??
        null
      )

    default:
      return null
  }
}
