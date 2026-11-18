import { Tooltip, TooltipContent, TooltipTrigger } from "@ui-sdk/react/tooltip"
import { CircleArrowUpIcon, DownloadIcon, LoaderCircleIcon } from "lucide-react"
import { useI18n } from "../i18n"
import { usePlatform, type AppUpdate } from "../platform"

const labels = {
  downloading: "update.downloading",
  ready: "update.install",
  available: "update.openPage",
} as const

// Shows only while the desktop shell has a newer version: on its way, ready to
// install on restart, or to fetch by hand when the app can't replace itself.
export function UpdateButton({ update }: { update: AppUpdate | null }) {
  const { t } = useI18n()
  const { updates } = usePlatform()
  if (!updates || !update) return null

  const downloading = update.state === "downloading"
  const label = t(labels[update.state], { version: update.version })
  return (
    <Tooltip type="label">
      <TooltipTrigger
        className="icon-btn update-btn"
        data-state={update.state}
        aria-disabled={downloading || undefined}
        onClick={() => {
          if (!downloading) updates.install()
        }}
      >
        {downloading ? (
          <LoaderCircleIcon aria-hidden="true" className="animate-spin motion-reduce:animate-none" />
        ) : update.state === "ready" ? (
          <CircleArrowUpIcon aria-hidden="true" />
        ) : (
          <DownloadIcon aria-hidden="true" />
        )}
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  )
}
