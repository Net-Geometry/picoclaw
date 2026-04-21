import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import type { OAuthProviderStatus } from "@/api/oauth"

import { ProviderStatusLine } from "./provider-status-line"

interface CredentialCardProps {
  title: ReactNode
  description: string
  status: OAuthProviderStatus["status"]
  authMethod?: string
  credentialCount?: number
  details?: ReactNode
  actions: ReactNode
  footer?: ReactNode
}

export function CredentialCard({
  title,
  description,
  status,
  authMethod,
  credentialCount,
  details,
  actions,
  footer,
}: CredentialCardProps) {
  const { t } = useTranslation()
  return (
    <section className="bg-card flex h-full flex-col rounded-xl border p-4">
      <div className="min-h-16">
        <h3 className="text-base font-semibold">{title}</h3>
        <p className="text-muted-foreground mt-1 text-xs">{description}</p>
      </div>

      <ProviderStatusLine status={status} authMethod={authMethod} />
      {credentialCount && credentialCount > 1 ? (
        <p className="text-muted-foreground mt-2 text-[11px] leading-5">
          {t("credentials.labels.credentialCount", { count: credentialCount })}
        </p>
      ) : null}
      <div className="text-muted-foreground mt-3 min-h-11 text-xs leading-5">
        {details}
      </div>

      <div className="mt-auto flex flex-col gap-4 pt-4">
        <div className="min-h-[112px]">{actions}</div>
        <div className="min-h-8">{footer}</div>
      </div>
    </section>
  )
}
