import * as React from "react"
import { useI18n } from "../i18n"
import { usePlatform, type AppUpdate } from "../platform"
import { toaster } from "../ui/toaster"

export function useAppUpdates() {
  const { updates } = usePlatform()
  const { t } = useI18n()
  const shown = React.useRef(new Set<string>())

  React.useEffect(() => {
    if (!updates) return
    const show = (update: AppUpdate | null) => {
      if (!update) return
      const key = `${update.state}:${update.version}`
      if (shown.current.has(key)) return
      shown.current.add(key)
      const ready = update.state === "ready"
      toaster(t(ready ? "update.ready" : "update.available", { version: update.version }), {
        description: t(ready ? "update.readyHint" : "update.availableHint"),
        duration: Infinity,
        action: { label: t(ready ? "update.restart" : "update.download"), onAction: () => updates.install() },
      })
    }
    void updates.last().then(show)
    return updates.subscribe(show)
  }, [updates, t])
}
