import { Button } from "@ui-sdk/react/button"
import { Popover, PopoverContent, PopoverDescription, PopoverTitle, PopoverTrigger } from "@ui-sdk/react/popover"
import { Progress, ProgressIndicator, ProgressLabel, ProgressTrack, ProgressValue } from "@ui-sdk/react/progress"
import { DatabaseIcon } from "lucide-react"
import { useCheckForUpdate, useStartUpdate } from "../api/queries"
import type { DataStatus } from "../api/types"
import { useI18n, type Translate } from "../i18n"

function stepLabel(t: Translate, status: DataStatus): string {
  const step = status.progress?.step
  return step ? t(`data.step.${step}`) : t("data.step.start")
}

export function UpdateProgress({ status }: { status: DataStatus }) {
  const { t } = useI18n()
  const percent = status.progress?.percent
  const showValue = status.progress?.step === "download" || status.progress?.step === "images"
  return (
    <Progress value={showValue ? (percent ?? null) : null} className="grid w-full grid-cols-[1fr_auto] gap-x-3 gap-y-1.5">
      <ProgressLabel className="font-semibold">{stepLabel(t, status)}</ProgressLabel>
      {showValue ? <ProgressValue className="text-ink-2" /> : <span />}
      <ProgressTrack className="col-span-2">
        <ProgressIndicator />
      </ProgressTrack>
    </Progress>
  )
}

export function DataStatusButton({ status }: { status: DataStatus }) {
  const { t, date } = useI18n()
  const check = useCheckForUpdate()
  const update = useStartUpdate()
  const busy = status.state !== "idle"

  return (
    <Popover>
      <PopoverTrigger className="btn-ghost gap-2">
        <DatabaseIcon aria-hidden="true" className="size-4" />
        <span className="max-md:sr-only">{t("data.version", { version: status.installedVersion })}</span>
        {status.updateAvailable && <span className="pip" data-tone="notice" aria-hidden="true" />}
      </PopoverTrigger>
      <PopoverContent align="end" className="grid w-80 gap-3">
        <PopoverTitle className="section-title">
          {status.updateAvailable
            ? t("data.updateAvailable", { latest: status.latestVersion, installed: status.installedVersion })
            : t("data.upToDate")}
        </PopoverTitle>
        <dl className="m-0 grid grid-cols-[1fr_auto] gap-x-4 gap-y-1">
          <dt className="text-ink-2">{t("data.installed")}</dt>
          <dd className="m-0 font-semibold">{status.installedVersion}</dd>
          <dt className="text-ink-2">{t("data.latest")}</dt>
          <dd className="m-0 font-semibold">{status.latestVersion || "–"}</dd>
        </dl>
        <PopoverDescription className="m-0 text-ink-2">
          {t("data.imported", { date: date(status.importedAt) })}
          <br />
          {status.lastCheckedAt && t("data.checked", { date: date(status.lastCheckedAt) })}
          {status.autoUpdate && (
            <>
              <br />
              {t("data.autoUpdate")}
            </>
          )}
        </PopoverDescription>
        {check.isError && (
          <p className="m-0 text-bad" role="alert">
            {t("data.checkFailed")}
          </p>
        )}
        <div className="flex gap-2">
          <Button disabled={busy} onClick={() => check.mutateAsync().catch(() => undefined)}>
            {status.state === "checking" ? t("data.checking") : t("data.check")}
          </Button>
          {status.updateAvailable && (
            <Button data-variant="solid" disabled={busy} onClick={() => update.mutateAsync()}>
              {t("data.update")}
            </Button>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}

export function DataBanner({ status }: { status: DataStatus }) {
  const { t } = useI18n()
  const update = useStartUpdate()

  if (status.state === "updating") {
    return (
      <div className="banner" role="status">
        <strong>{t("data.updating", { version: status.progress?.target || status.latestVersion })}</strong>
        <div className="min-w-60 flex-1">
          <UpdateProgress status={status} />
        </div>
      </div>
    )
  }
  if (status.error) {
    return (
      <div className="banner" data-kind="error" role="alert">
        <span className="flex-1">{t("data.failed", { error: status.error })}</span>
        <Button onClick={() => update.mutateAsync()}>{t("data.retry")}</Button>
      </div>
    )
  }
  if (status.updateAvailable) {
    return (
      <div className="banner" role="status">
        <span className="flex-1">
          <strong>
            {t("data.updateAvailable", { latest: status.latestVersion, installed: status.installedVersion })}
          </strong>{" "}
          <span className="text-ink-2">{t("data.updateHint")}</span>
        </span>
        <Button data-variant="solid" onClick={() => update.mutateAsync()}>
          {t("data.update")}
        </Button>
      </div>
    )
  }
  return null
}
