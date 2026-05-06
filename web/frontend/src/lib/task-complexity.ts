/**
 * Task Complexity Analyzer
 * Analyzes user input to determine task complexity similar to backend classifier
 * Returns a score in [0, 1] where:
 * - 0.0 - 0.35: Simple/general tasks (light model suitable)
 * - 0.35 - 0.65: Medium complexity tasks
 * - 0.65 - 1.0: Complex tasks (requires capable model)
 */

export interface TaskComplexityFeatures {
  tokenEstimate: number
  codeBlockCount: number
  hasImages: boolean
  hasMathEquations: boolean
  hasJSONContent: boolean
  messageLength: number
}

export function extractTaskFeatures(
  message: string,
  hasAttachments: boolean
): TaskComplexityFeatures {
  // Token estimate (rough: ~4 chars per token)
  const tokenEstimate = Math.ceil(message.length / 4)

  // Count code blocks (fenced with ```)
  const codeBlockCount = (message.match(/```/g) || []).length / 2

  // Check for math content (LaTeX-like patterns)
  const hasMathEquations = /\$[^$]+\$|\\\(|\\\[|\\frac|\\sqrt/.test(message)

  // Check for JSON patterns
  const hasJSONContent = /\{[^}]*["'].*["'][^}]*\}/.test(message)

  // Message length
  const messageLength = message.length

  return {
    tokenEstimate,
    codeBlockCount: Math.floor(codeBlockCount),
    hasImages: hasAttachments,
    hasMathEquations,
    hasJSONContent,
    messageLength,
  }
}

export function scoreTaskComplexity(features: TaskComplexityFeatures): number {
  let score = 0

  // Images/attachments: hard gate to complex
  if (features.hasImages) {
    return 1.0
  }

  // Token estimate — primary verbosity signal
  if (features.tokenEstimate > 200) {
    score += 0.35
  } else if (features.tokenEstimate > 50) {
    score += 0.15
  }

  // Code blocks — strongest indicator of coding/technical task
  if (features.codeBlockCount > 0) {
    score += 0.40
  }

  // JSON content — structured data processing
  if (features.hasJSONContent) {
    score += 0.15
  }

  // Math equations — scientific/technical content
  if (features.hasMathEquations) {
    score += 0.20
  }

  // Message length alone (complementary to token estimate)
  if (features.messageLength > 500) {
    score += 0.10
  }

  // Cap at 1.0
  return Math.min(score, 1.0)
}

export function classifyComplexity(
  score: number
): "simple" | "medium" | "complex" {
  if (score < 0.35) {
    return "simple"
  }
  if (score < 0.65) {
    return "medium"
  }
  return "complex"
}
