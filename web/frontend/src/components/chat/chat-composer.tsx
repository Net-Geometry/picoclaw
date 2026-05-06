import {
  IconArrowUp,
  IconFilePlus,
  IconMicrophone,
  IconPlayerStop,
  IconX,
} from "@tabler/icons-react"
import type { KeyboardEvent } from "react"
import { useTranslation } from "react-i18next"
import TextareaAutosize from "react-textarea-autosize"

import type { ModelInfo } from "@/api/models"
import { ModelSelector } from "@/components/chat/model-selector"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { ChatAttachment } from "@/store/chat"

function formatFileSize(bytes: number): string {
  if (bytes === 0) return "0 B"
  const k = 1024
  const sizes = ["B", "KB", "MB", "GB"]
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export type ChatInputDisabledReason =
  | "gatewayUnknown"
  | "gatewayStarting"
  | "gatewayRestarting"
  | "gatewayStopping"
  | "gatewayStopped"
  | "gatewayError"
  | "websocketConnecting"
  | "websocketDisconnected"
  | "websocketError"
  | "noDefaultModel"

export type ChatChannel = "pico" | "manus"

interface ChatComposerProps {
  input: string
  attachments: ChatAttachment[]
  selectedChannel: ChatChannel
  onChannelChange: (channel: ChatChannel) => void
  defaultModelName: string
  apiKeyModels: ModelInfo[]
  oauthModels: ModelInfo[]
  localModels: ModelInfo[]
  hasAvailableModels: boolean
  onModelChange: (modelName: string) => void
  onInputChange: (value: string) => void
  onAddFiles: () => void
  onToggleRecording: () => void
  onRemoveAttachment: (index: number) => void
  onSend: () => void
  inputDisabledReason: ChatInputDisabledReason | null
  canSend: boolean
  isRecording: boolean
  recordingTranscript?: string
}

export function ChatComposer({
  input,
  attachments,
  selectedChannel,
  onChannelChange,
  defaultModelName,
  apiKeyModels,
  oauthModels,
  localModels,
  hasAvailableModels,
  onModelChange,
  onInputChange,
  onAddFiles,
  onToggleRecording,
  onRemoveAttachment,
  onSend,
  inputDisabledReason,
  canSend,
  isRecording,
  recordingTranscript,
}: ChatComposerProps) {
  const { t } = useTranslation()
  const showModelSelector = selectedChannel === "pico" && hasAvailableModels
  const canInput = inputDisabledReason === null
  const disabledMessage =
    inputDisabledReason === null
      ? null
      : t(`chat.disabledPlaceholder.${inputDisabledReason}`)
  const placeholder = disabledMessage ?? t("chat.placeholder")

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.nativeEvent.isComposing) return
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      onSend()
    }
  }

  return (
    <div className="bg-background shrink-0 px-4 pt-4 pb-[calc(1rem+env(safe-area-inset-bottom))] md:px-8 md:pb-8 lg:px-24 xl:px-48">
      <div className="bg-card border-border/80 mx-auto flex max-w-[1000px] flex-col rounded-2xl border p-3 shadow-md">
        {attachments.length > 0 && (
          <div className="mb-3 flex flex-wrap gap-2 px-2">
            {attachments.map((attachment, index) => (
              <div
                key={`${attachment.url}-${index}`}
                className="bg-background relative flex min-h-20 min-w-20 flex-col gap-1 overflow-hidden rounded-xl border px-3 py-2"
              >
                {attachment.type === "image" ? (
                  <img
                    src={attachment.url}
                    alt={attachment.filename || t("chat.uploadedImage")}
                    className="h-12 w-12 rounded object-cover"
                  />
                ) : (
                  <div className="bg-muted text-muted-foreground flex h-12 w-12 items-center justify-center rounded text-xs font-semibold uppercase">
                    {attachment.type === "audio" ? "AUD" : "DOC"}
                  </div>
                )}
                <div className="flex max-w-40 flex-col pr-5 text-xs">
                  <div className="truncate font-medium">
                    {attachment.filename || t("chat.uploadedFile")}
                  </div>
                  {attachment.size && (
                    <div className="text-muted-foreground text-xs">
                      {formatFileSize(attachment.size)}
                    </div>
                  )}
                  {attachment.type === "audio" && attachment.transcript && (
                    <div className="text-muted-foreground mt-1 line-clamp-2 text-xs italic">
                      "{attachment.transcript}"
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => onRemoveAttachment(index)}
                  className="bg-background/85 text-foreground absolute top-1 right-1 inline-flex h-6 w-6 items-center justify-center rounded-full border shadow-sm transition hover:bg-white"
                  aria-label={t("chat.removeAttachment")}
                  title={t("chat.removeAttachment")}
                >
                  <IconX className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="border-border/70 bg-muted/30 mb-3 rounded-xl border p-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-muted-foreground px-1 text-[11px] font-semibold tracking-wide uppercase">
              Chat Target
            </span>

            <div className="bg-background border-border/80 inline-flex rounded-lg border p-0.5">
              <Button
                type="button"
                size="sm"
                variant={selectedChannel === "pico" ? "default" : "ghost"}
                className="h-7 rounded-md px-3 text-xs"
                onClick={() => onChannelChange("pico")}
                aria-pressed={selectedChannel === "pico"}
              >
                Pico
              </Button>
              <Button
                type="button"
                size="sm"
                variant={selectedChannel === "manus" ? "default" : "ghost"}
                className="h-7 rounded-md px-3 text-xs"
                onClick={() => onChannelChange("manus")}
                aria-pressed={selectedChannel === "manus"}
              >
                Manus
              </Button>
            </div>

            {showModelSelector && (
              <div className="min-w-0 flex-1 sm:flex-none">
                <ModelSelector
                  defaultModelName={defaultModelName}
                  apiKeyModels={apiKeyModels}
                  oauthModels={oauthModels}
                  localModels={localModels}
                  onValueChange={onModelChange}
                />
              </div>
            )}

            {selectedChannel === "manus" && (
              <span className="text-muted-foreground border-border/70 rounded-md border px-2 py-1 text-xs">
                Direct Manus task mode
              </span>
            )}
          </div>
        </div>

        <TextareaAutosize
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={!canInput}
          title={disabledMessage || undefined}
          className={cn(
            "placeholder:text-muted-foreground/50 max-h-[200px] min-h-[60px] resize-none border-0 bg-transparent px-2 py-1 text-[15px] shadow-none transition-colors focus-visible:ring-0 focus-visible:outline-none dark:bg-transparent",
            !canInput && "cursor-not-allowed",
          )}
          minRows={1}
          maxRows={8}
        />

        {isRecording && recordingTranscript && (
          <div className="bg-muted/30 border-muted mx-2 rounded-lg border px-3 py-2 text-xs">
            <div className="text-muted-foreground mb-1 text-xs font-medium">
              {t("chat.transcriptPreview")}
            </div>
            <div className="text-foreground line-clamp-3">
              {recordingTranscript}
            </div>
          </div>
        )}

        {!canInput && disabledMessage && (
          <div className="text-muted-foreground px-3 py-1 text-xs">
            {disabledMessage}
          </div>
        )}

        <div className="mt-2 flex items-center justify-between px-1">
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="text-muted-foreground hover:text-foreground h-8 w-8 rounded-full"
              onClick={onAddFiles}
              disabled={!canInput}
              aria-label={t("chat.attachFile")}
              title={t("chat.attachFile")}
            >
              <IconFilePlus className="size-4" />
            </Button>
            <Button
              type="button"
              variant={isRecording ? "default" : "ghost"}
              size="icon"
              className="text-muted-foreground hover:text-foreground h-8 w-8 rounded-full"
              onClick={onToggleRecording}
              disabled={!canInput}
              aria-label={
                isRecording
                  ? t("chat.stopVoiceRecording")
                  : t("chat.recordVoice")
              }
              title={
                isRecording
                  ? t("chat.stopVoiceRecording")
                  : t("chat.recordVoice")
              }
            >
              {isRecording ? (
                <IconPlayerStop className="size-4" />
              ) : (
                <IconMicrophone className="size-4" />
              )}
            </Button>
          </div>

          {canInput ? (
            <Button
              type="button"
              size="icon"
              className="size-8 rounded-full bg-violet-500 text-white transition-transform hover:bg-violet-600 active:scale-95"
              onClick={onSend}
              disabled={!canSend}
            >
              <IconArrowUp className="size-4" />
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  )
}
