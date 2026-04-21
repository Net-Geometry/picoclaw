import { IconPlus } from "@tabler/icons-react"
import { type ChangeEvent, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { AssistantMessage } from "@/components/chat/assistant-message"
import {
  ChatComposer,
  type ChatInputDisabledReason,
} from "@/components/chat/chat-composer"
import { ChatEmptyState } from "@/components/chat/chat-empty-state"
import { ModelSelector } from "@/components/chat/model-selector"
import { SessionHistoryMenu } from "@/components/chat/session-history-menu"
import { TypingIndicator } from "@/components/chat/typing-indicator"
import { UserMessage } from "@/components/chat/user-message"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { useChatModels } from "@/hooks/use-chat-models"
import { useGateway } from "@/hooks/use-gateway"
import { usePicoChat } from "@/hooks/use-pico-chat"
import { useSessionHistory } from "@/hooks/use-session-history"
import type { ConnectionState } from "@/store/chat"
import type { ChatAttachment } from "@/store/chat"
import type { GatewayState } from "@/store/gateway"

const MAX_ATTACHMENT_SIZE_BYTES = 20 * 1024 * 1024
const MAX_ATTACHMENT_SIZE_LABEL = "20 MB"
const ALLOWED_IMAGE_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
])
const ALLOWED_DOCUMENT_TYPES = new Set([
  "application/pdf",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "text/plain",
  "text/markdown",
  "text/x-markdown",
  "application/rtf",
  "text/rtf",
])
const ALLOWED_AUDIO_TYPES = new Set([
  "audio/mpeg",
  "audio/mp3",
  "audio/wav",
  "audio/x-wav",
  "audio/webm",
  "audio/ogg",
  "audio/ogg;codecs=opus",
  "audio/mp4",
  "audio/x-m4a",
])
const EXTENSION_MIME_TYPE: Record<string, string> = {
  pdf: "application/pdf",
  docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  pptx: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  txt: "text/plain",
  md: "text/markdown",
  markdown: "text/markdown",
  rtf: "application/rtf",
  mp3: "audio/mpeg",
  wav: "audio/wav",
  webm: "audio/webm",
  ogg: "audio/ogg",
  m4a: "audio/mp4",
}
const CHAT_FILE_ACCEPT = [
  ".pdf",
  ".docx",
  ".pptx",
  ".xlsx",
  ".txt",
  ".md",
  ".rtf",
  ".mp3",
  ".wav",
  ".webm",
  ".ogg",
  ".m4a",
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
].join(",")

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === "string") {
        resolve(reader.result)
        return
      }
      reject(new Error("Failed to read file"))
    }
    reader.onerror = () =>
      reject(reader.error || new Error("Failed to read file"))
    reader.readAsDataURL(file)
  })
}

interface SpeechRecognitionEvent {
  results: SpeechRecognitionResultList
}

interface SpeechRecognitionResultList {
  [index: number]: SpeechRecognitionResult
  length: number
}

interface SpeechRecognitionResult {
  [index: number]: SpeechRecognitionAlternative
  isFinal: boolean
  length: number
}

interface SpeechRecognitionAlternative {
  transcript: string
  confidence: number
}

interface SpeechRecognition extends EventTarget {
  continuous: boolean
  interimResults: boolean
  language: string
  start(): void
  stop(): void
  abort(): void
  onerror: ((event: Event) => void) | null
  onstart: ((event: Event) => void) | null
  onend: ((event: Event) => void) | null
  onresult: ((event: SpeechRecognitionEvent) => void) | null
}

declare global {
  interface Window {
    SpeechRecognition?: new () => SpeechRecognition
    webkitSpeechRecognition?: new () => SpeechRecognition
  }
}

function resolveChatInputDisabledReason({
  hasDefaultModel,
  connectionState,
  gatewayState,
}: {
  hasDefaultModel: boolean
  connectionState: ConnectionState
  gatewayState: GatewayState
}): ChatInputDisabledReason | null {
  if (gatewayState === "unknown") {
    return "gatewayUnknown"
  }

  if (gatewayState === "starting") {
    return "gatewayStarting"
  }

  if (gatewayState === "restarting") {
    return "gatewayRestarting"
  }

  if (gatewayState === "stopping") {
    return "gatewayStopping"
  }

  if (gatewayState === "stopped") {
    return "gatewayStopped"
  }

  if (gatewayState === "error") {
    return "gatewayError"
  }

  if (connectionState === "connecting") {
    return "websocketConnecting"
  }

  if (connectionState === "error") {
    return "websocketError"
  }

  if (connectionState === "disconnected") {
    return "websocketDisconnected"
  }

  if (!hasDefaultModel) {
    return "noDefaultModel"
  }

  return null
}

export function ChatPage() {
  const { t } = useTranslation()
  const scrollRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const recordingChunksRef = useRef<BlobPart[]>([])
  const speechRecognitionRef = useRef<SpeechRecognition | null>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)
  const [hasScrolled, setHasScrolled] = useState(false)
  const [input, setInput] = useState("")
  const [attachments, setAttachments] = useState<ChatAttachment[]>([])
  const [isRecording, setIsRecording] = useState(false)
  const [isDraggingFiles, setIsDraggingFiles] = useState(false)
  const [recordingTranscript, setRecordingTranscript] = useState("")

  const {
    messages,
    connectionState,
    isTyping,
    activeSessionId,
    sendMessage,
    switchSession,
    newChat,
  } = usePicoChat()

  const { state: gwState } = useGateway()
  const isGatewayRunning = gwState === "running"

  const {
    defaultModelName,
    hasAvailableModels,
    apiKeyModels,
    oauthModels,
    localModels,
    handleSetDefault,
  } = useChatModels({ isConnected: isGatewayRunning })
  const hasDefaultModel = Boolean(defaultModelName)
  const inputDisabledReason = resolveChatInputDisabledReason({
    hasDefaultModel,
    connectionState,
    gatewayState: gwState,
  })
  const canInput = inputDisabledReason === null

  const {
    sessions,
    hasMore,
    loadError,
    loadErrorMessage,
    observerRef,
    loadSessions,
    handleDeleteSession,
  } = useSessionHistory({
    activeSessionId,
    onDeletedActiveSession: newChat,
  })

  const syncScrollState = (element: HTMLDivElement) => {
    const { scrollTop, scrollHeight, clientHeight } = element
    setHasScrolled(scrollTop > 0)
    setIsAtBottom(scrollHeight - scrollTop <= clientHeight + 10)
  }

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    syncScrollState(e.currentTarget)
  }

  useEffect(() => {
    if (scrollRef.current) {
      if (isAtBottom) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight
      }
      syncScrollState(scrollRef.current)
    }
  }, [messages, isTyping, isAtBottom])

  const handleSend = () => {
    if ((!input.trim() && attachments.length === 0) || !canInput) return
    if (
      sendMessage({
        content: input,
        attachments,
      })
    ) {
      setInput("")
      setAttachments([])
    }
  }

  const handleAddFiles = () => {
    if (!canInput) return
    fileInputRef.current?.click()
  }

  const handleRemoveAttachment = (index: number) => {
    setAttachments((prev) => prev.filter((_, itemIndex) => itemIndex !== index))
  }

  const resolveMimeType = (file: File): string => {
    if (file.type) {
      return file.type.toLowerCase()
    }

    const extension = file.name.split(".").pop()?.toLowerCase() || ""
    return EXTENSION_MIME_TYPE[extension] || ""
  }

  const classifyAttachment = (
    mimeType: string,
  ): ChatAttachment["type"] | null => {
    const normalized = mimeType.toLowerCase()
    if (ALLOWED_IMAGE_TYPES.has(normalized)) {
      return "image"
    }
    if (
      normalized.startsWith("audio/") ||
      ALLOWED_AUDIO_TYPES.has(normalized)
    ) {
      return "audio"
    }
    if (ALLOWED_DOCUMENT_TYPES.has(normalized)) {
      return "file"
    }
    return null
  }

  const appendAttachments = (next: ChatAttachment[]) => {
    if (next.length === 0) {
      return
    }

    setAttachments((prev) => {
      const merged = [...prev]
      for (const item of next) {
        if (!merged.some((existing) => existing.url === item.url)) {
          merged.push(item)
        }
      }
      return merged
    })
  }

  const handleFileSelection = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ""

    if (files.length === 0) {
      return
    }

    const nextAttachments: ChatAttachment[] = []
    for (const file of files) {
      const mimeType = resolveMimeType(file)
      const attachmentType = classifyAttachment(mimeType)

      if (!attachmentType) {
        toast.error(
          t("chat.invalidAttachment", {
            name: file.name,
          }),
        )
        continue
      }

      if (file.size > MAX_ATTACHMENT_SIZE_BYTES) {
        toast.error(
          t("chat.attachmentTooLarge", {
            name: file.name,
            size: MAX_ATTACHMENT_SIZE_LABEL,
          }),
        )
        continue
      }

      try {
        nextAttachments.push({
          type: attachmentType,
          filename: file.name,
          mimeType,
          size: file.size,
          url: await readFileAsDataUrl(file),
        })
      } catch {
        toast.error(
          t("chat.attachmentReadFailed", {
            name: file.name,
          }),
        )
      }
    }

    appendAttachments(nextAttachments)
  }

  const handleToggleRecording = async () => {
    if (!canInput) {
      return
    }

    const activeRecorder = mediaRecorderRef.current
    if (activeRecorder && activeRecorder.state !== "inactive") {
      activeRecorder.stop()
      if (speechRecognitionRef.current) {
        speechRecognitionRef.current.stop()
      }
      return
    }

    if (
      typeof navigator === "undefined" ||
      !navigator.mediaDevices?.getUserMedia
    ) {
      toast.error(t("chat.voiceUnsupported"))
      return
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mimeTypeCandidates = [
        "audio/webm;codecs=opus",
        "audio/webm",
        "audio/ogg;codecs=opus",
      ]
      const selectedMimeType = mimeTypeCandidates.find((candidate) =>
        MediaRecorder.isTypeSupported(candidate),
      )

      const recorder = selectedMimeType
        ? new MediaRecorder(stream, { mimeType: selectedMimeType })
        : new MediaRecorder(stream)

      mediaRecorderRef.current = recorder
      recordingChunksRef.current = []
      setRecordingTranscript("")

      // Initialize speech recognition
      const SpeechRecognitionClass =
        window.SpeechRecognition || window.webkitSpeechRecognition
      if (SpeechRecognitionClass) {
        const recognition = new SpeechRecognitionClass()
        recognition.continuous = true
        recognition.interimResults = true
        recognition.language = "en-US"

        let finalTranscript = ""
        recognition.onresult = (event: SpeechRecognitionEvent) => {
          let interimTranscript = ""
          for (let i = event.results.length - 1; i >= 0; i--) {
            const transcript = event.results[i][0].transcript
            if (event.results[i].isFinal) {
              finalTranscript += transcript + " "
            } else {
              interimTranscript += transcript
            }
          }
          setRecordingTranscript((finalTranscript + interimTranscript).trim())
        }

        recognition.onerror = () => {
          // Silently handle speech recognition errors - they don't break recording
        }

        speechRecognitionRef.current = recognition
        try {
          recognition.start()
        } catch {
          // Silently handle if recognition is already started
        }
      }

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          recordingChunksRef.current.push(event.data)
        }
      }

      recorder.onstop = async () => {
        const chunks = [...recordingChunksRef.current]
        recordingChunksRef.current = []
        setIsRecording(false)
        setRecordingTranscript("")

        if (speechRecognitionRef.current) {
          speechRecognitionRef.current.stop()
          speechRecognitionRef.current = null
        }

        for (const track of stream.getTracks()) {
          track.stop()
        }

        if (chunks.length === 0) {
          return
        }

        const blobMimeType = recorder.mimeType || "audio/webm"
        const blob = new Blob(chunks, { type: blobMimeType })
        const blobSize = blob.size

        if (blob.size > MAX_ATTACHMENT_SIZE_BYTES) {
          toast.error(
            t("chat.attachmentTooLarge", {
              name: t("chat.voiceMessage"),
              size: MAX_ATTACHMENT_SIZE_LABEL,
            }),
          )
          return
        }

        const extension = blobMimeType.includes("ogg")
          ? "ogg"
          : blobMimeType.includes("wav")
            ? "wav"
            : blobMimeType.includes("mpeg") || blobMimeType.includes("mp3")
              ? "mp3"
              : "webm"
        const fileName = `voice-${Date.now()}.${extension}`

        try {
          const dataUrl = await readFileAsDataUrl(
            new File([blob], fileName, { type: blobMimeType }),
          )
          appendAttachments([
            {
              type: "audio",
              filename: fileName,
              mimeType: blobMimeType,
              size: blobSize,
              url: dataUrl,
              transcript: recordingTranscript || undefined,
            },
          ])
        } catch {
          toast.error(
            t("chat.attachmentReadFailed", {
              name: fileName,
            }),
          )
        }
      }

      recorder.onerror = () => {
        setIsRecording(false)
        toast.error(t("chat.voiceRecordFailed"))
      }

      recorder.start()
      setIsRecording(true)
    } catch {
      setIsRecording(false)
      toast.error(t("chat.voicePermissionDenied"))
    }
  }

  useEffect(() => {
    return () => {
      if (
        mediaRecorderRef.current &&
        mediaRecorderRef.current.state !== "inactive"
      ) {
        mediaRecorderRef.current.stop()
      }
      if (speechRecognitionRef.current) {
        speechRecognitionRef.current.abort()
      }
    }
  }, [])

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDraggingFiles(true)
  }

  const handleDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.target === scrollRef.current) {
      setIsDraggingFiles(false)
    }
  }

  const handleDrop = async (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDraggingFiles(false)

    const files = Array.from(e.dataTransfer.files ?? [])
    if (files.length === 0) {
      return
    }

    const nextAttachments: ChatAttachment[] = []
    for (const file of files) {
      const mimeType = resolveMimeType(file)
      const attachmentType = classifyAttachment(mimeType)

      if (!attachmentType) {
        toast.error(
          t("chat.invalidAttachment", {
            name: file.name,
          }),
        )
        continue
      }

      if (file.size > MAX_ATTACHMENT_SIZE_BYTES) {
        toast.error(
          t("chat.attachmentTooLarge", {
            name: file.name,
            size: MAX_ATTACHMENT_SIZE_LABEL,
          }),
        )
        continue
      }

      try {
        nextAttachments.push({
          type: attachmentType,
          filename: file.name,
          mimeType,
          size: file.size,
          url: await readFileAsDataUrl(file),
        })
      } catch {
        toast.error(
          t("chat.attachmentReadFailed", {
            name: file.name,
          }),
        )
      }
    }

    appendAttachments(nextAttachments)
  }

  const canSubmit =
    canInput && (Boolean(input.trim()) || attachments.length > 0)

  return (
    <div className="bg-background/95 flex h-full flex-col">
      <PageHeader
        title={t("navigation.chat")}
        className={`transition-shadow ${
          hasScrolled ? "shadow-xs" : "shadow-none"
        }`}
        titleExtra={
          hasAvailableModels && (
            <ModelSelector
              defaultModelName={defaultModelName}
              apiKeyModels={apiKeyModels}
              oauthModels={oauthModels}
              localModels={localModels}
              onValueChange={handleSetDefault}
            />
          )
        }
      >
        <Button
          variant="secondary"
          size="sm"
          onClick={newChat}
          className="h-9 gap-2"
        >
          <IconPlus className="size-4" />
          <span className="hidden sm:inline">{t("chat.newChat")}</span>
        </Button>

        <SessionHistoryMenu
          sessions={sessions}
          activeSessionId={activeSessionId}
          hasMore={hasMore}
          loadError={loadError}
          loadErrorMessage={loadErrorMessage}
          observerRef={observerRef}
          onOpenChange={(open) => {
            if (open) {
              void loadSessions(true)
            }
          }}
          onSwitchSession={switchSession}
          onDeleteSession={handleDeleteSession}
        />
      </PageHeader>

      <div
        ref={scrollRef}
        onScroll={handleScroll}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        className={`min-h-0 flex-1 overflow-y-auto px-4 py-6 transition-colors md:px-8 lg:px-24 xl:px-48 ${
          isDraggingFiles ? "bg-blue-50 dark:bg-blue-950/20" : "bg-transparent"
        }`}
      >
        <div className="mx-auto flex w-full max-w-250 flex-col gap-8 pb-8">
          {isDraggingFiles && (
            <div className="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-black/20">
              <div className="bg-card border-border rounded-2xl border px-8 py-6 text-center shadow-lg">
                <p className="text-foreground text-lg font-medium">
                  {t("chat.dropFilesHere")}
                </p>
              </div>
            </div>
          )}
          {messages.length === 0 && !isTyping && (
            <ChatEmptyState
              hasAvailableModels={hasAvailableModels}
              defaultModelName={defaultModelName}
              isConnected={isGatewayRunning}
            />
          )}

          {messages.map((msg) => (
            <div key={msg.id} className="flex w-full">
              {msg.role === "assistant" ? (
                <AssistantMessage
                  content={msg.content}
                  isThought={msg.kind === "thought"}
                  timestamp={msg.timestamp}
                />
              ) : (
                <UserMessage
                  content={msg.content}
                  attachments={msg.attachments}
                />
              )}
            </div>
          ))}

          {isTyping && <TypingIndicator />}
        </div>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept={CHAT_FILE_ACCEPT}
        multiple
        className="hidden"
        onChange={handleFileSelection}
      />

      <ChatComposer
        input={input}
        attachments={attachments}
        onInputChange={setInput}
        onAddFiles={handleAddFiles}
        onToggleRecording={handleToggleRecording}
        onRemoveAttachment={handleRemoveAttachment}
        onSend={handleSend}
        inputDisabledReason={inputDisabledReason}
        canSend={canSubmit}
        isRecording={isRecording}
        recordingTranscript={recordingTranscript}
      />
    </div>
  )
}
