import { IconArrowUp, IconFileText, IconPhotoPlus, IconX } from "@tabler/icons-react"
import type { KeyboardEvent } from "react"
import { useTranslation } from "react-i18next"
import TextareaAutosize from "react-textarea-autosize"

import { ContextUsageRing } from "@/components/chat/context-usage-ring"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import type { ChatAttachment, ContextUsage } from "@/store/chat"

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

export type ChatChannelMode = "auto" | "pico" | "manus"

interface ChatComposerProps {
  input: string
  attachments: ChatAttachment[]
  attachmentUploadDir?: string
  onInputChange: (value: string) => void
  onAddImages: () => void
  onRemoveAttachment: (index: number) => void
  onSend: () => void
  onContextDetail?: () => void
  inputDisabledReason: ChatInputDisabledReason | null
  canSend: boolean
  contextUsage?: ContextUsage
  channelMode: ChatChannelMode
  onChannelModeChange: (mode: ChatChannelMode) => void
  focusedProject: string
  projectOptions: string[]
  onProjectFocusChange: (project: string) => void
  channelHint?: string
  canAttachImages?: boolean
}

export function ChatComposer({
  input,
  attachments,
  attachmentUploadDir = "projects/incoming/uploads",
  onInputChange,
  onAddImages,
  onRemoveAttachment,
  onSend,
  onContextDetail,
  inputDisabledReason,
  canSend,
  contextUsage,
  channelMode,
  onChannelModeChange,
  focusedProject,
  projectOptions,
  onProjectFocusChange,
  channelHint,
  canAttachImages = true,
}: ChatComposerProps) {
  const { t } = useTranslation()
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
    <div className="before:bg-background pointer-events-none relative z-10 -mt-[24px] shrink-0 overflow-y-auto px-4 pb-[calc(1rem+env(safe-area-inset-bottom))] [scrollbar-gutter:stable] before:pointer-events-none before:absolute before:inset-x-0 before:top-[24px] before:bottom-0 before:content-[''] md:px-8 md:pb-8 lg:px-24 xl:px-48">
      <div className="bg-card border-border/60 pointer-events-auto relative mx-auto flex max-w-[1000px] flex-col rounded-2xl border p-3 shadow-sm">
        {attachments.length > 0 && (
          <div className="mb-3 px-2">
            <div className="flex flex-wrap gap-2">
              {attachments.map((attachment, index) => (
                <div
                  key={`${attachment.url}-${index}`}
                  className="bg-background relative flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border"
                >
                  {attachment.type === "image" ? (
                    <img
                      src={attachment.url}
                      alt={attachment.filename || t("chat.uploadedImage")}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="text-muted-foreground flex flex-col items-center gap-1 px-2 text-center">
                      <IconFileText className="h-5 w-5" />
                      <span className="line-clamp-2 text-[10px] leading-tight">
                        {attachment.filename || "File"}
                      </span>
                    </div>
                  )}
                  <button
                    type="button"
                    onClick={() => onRemoveAttachment(index)}
                    className="bg-background/85 text-foreground absolute top-1 right-1 inline-flex h-6 w-6 items-center justify-center rounded-full border shadow-sm transition hover:bg-white"
                    aria-label={t("chat.removeImage")}
                    title={t("chat.removeImage")}
                  >
                    <IconX className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
            </div>
            <div className="text-muted-foreground mt-2 space-y-1 text-[11px]">
              {attachments.map((attachment, index) => {
                const filename = attachment.filename || `upload-${index + 1}`
                return (
                  <div key={`workspace-path-${attachment.url}-${index}`}>
                    {t("chat.uploadWorkspacePath", {
                      filename,
                      path: `${attachmentUploadDir}/${filename}`,
                    })}
                  </div>
                )
              })}
            </div>
          </div>
        )}

        <TextareaAutosize
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={!canInput}
          title={disabledMessage || undefined}
          className={cn(
            "placeholder:text-muted-foreground/50 max-h-[200px] min-h-[64px] resize-none border-0 bg-transparent px-2 py-1 text-[15px] shadow-none transition-colors focus-visible:ring-0 focus-visible:outline-none dark:bg-transparent",
            !canInput && "cursor-not-allowed",
          )}
          minRows={1}
          maxRows={8}
        />

        <div className="mt-2 flex items-center justify-between px-1">
          <div className="flex items-center gap-1">
            <select
              value={channelMode}
              onChange={(e) =>
                onChannelModeChange(e.target.value as ChatChannelMode)
              }
              className="bg-background text-foreground border-border h-8 rounded-md border px-2 text-xs"
              title={t("chat.channel.title")}
              disabled={!canInput}
            >
              <option value="auto">{t("chat.channel.auto")}</option>
              <option value="pico">{t("chat.channel.pico")}</option>
              <option value="manus">{t("chat.channel.manus")}</option>
            </select>
            <select
              value={focusedProject}
              onChange={(e) => onProjectFocusChange(e.target.value)}
              className="bg-background text-foreground border-border h-8 max-w-44 rounded-md border px-2 text-xs"
              title={t("chat.projectFocus.title")}
              disabled={!canInput || channelMode === "manus"}
            >
              <option value="">{t("chat.projectFocus.none")}</option>
              {projectOptions.map((project) => (
                <option key={project} value={project}>
                  {project}
                </option>
              ))}
            </select>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="text-muted-foreground hover:text-foreground h-8 w-8 rounded-full"
              onClick={onAddImages}
              disabled={!canInput || !canAttachImages}
              aria-label={t("chat.attachImage")}
              title={
                canAttachImages
                  ? t("chat.attachImage")
                  : t("chat.channel.manusNoImage")
              }
            >
              <IconPhotoPlus className="size-4" />
            </Button>
          </div>

          <div className="flex items-center gap-1.5">
            {contextUsage && (
              <ContextUsageRing
                usage={contextUsage}
                onDetailClick={onContextDetail}
              />
            )}
            {canInput ? (
              <Tooltip delayDuration={700}>
                <TooltipTrigger asChild>
                  <span tabIndex={!canSend ? 0 : undefined}>
                    <Button
                      type="button"
                      size="icon"
                      className="size-8 rounded-full bg-violet-500 text-white transition-transform hover:bg-violet-600 active:scale-95"
                      onClick={onSend}
                      disabled={!canSend}
                      aria-label={t("chat.sendMessage")}
                    >
                      <IconArrowUp className="size-4" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent
                  className="border-border/70 bg-muted text-foreground border text-center whitespace-pre-line shadow-lg shadow-black/10 dark:shadow-black/30"
                  arrowClassName="bg-muted fill-muted"
                >
                  {t("chat.sendHint")}
                </TooltipContent>
              </Tooltip>
            ) : null}
          </div>
        </div>
        {channelHint ? (
          <div className="text-muted-foreground mt-2 px-2 text-xs">
            {channelHint}
          </div>
        ) : null}
      </div>
    </div>
  )
}
