import * as React from "react"
import { useI18n, type Translate } from "../i18n"
import { usePlatform, type AppUpdate, type UpdateBridge } from "../platform"
import { toaster } from "../ui/toaster"

// The notice that a version waits. It keeps one id, so asking again brings the
// same toast back instead of stacking a second one.
export function announceUpdate(t: Translate, updates: UpdateBridge, update: AppUpdate) {
  if (update.state === "downloading") {
    toaster(t("update.downloading", { version: update.version }), { id: "app-update-progress" })
    return
  }
  const ready = update.state === "ready"
  toaster(t(ready ? "update.ready" : "update.available", { version: update.version }), {
    id: "app-update",
    description: t(ready ? "update.readyHint" : "update.availableHint"),
    duration: Infinity,
    action: { label: t(ready ? "update.restart" : "update.download"), onAction: () => updates.install() },
  })
}

// The update the desktop shell has found, kept current while the app runs, and
// announced once when there is something to do about it.
export function useAppUpdates(): AppUpdate | null {
  const { updates } = usePlatform()
  const { t } = useI18n()
  const [update, setUpdate] = React.useState<AppUpdate | null>(null)
  const shown = React.useRef(new Set<string>())

  React.useEffect(() => {
    if (!updates) return
    // What the shell says while the first answer is on its way is newer than that answer.
    let told = false
    void updates.last().then((first) => {
      if (!told) setUpdate(first)
    })
    const unsubscribe = updates.subscribe((next) => {
      told = true
      setUpdate(next)
    })
    return () => {
      told = true
      unsubscribe()
    }
  }, [updates])

  React.useEffect(() => {
    if (!updates || !update || update.state === "downloading") return
    const key = `${update.state}:${update.version}`
    if (shown.current.has(key)) return
    shown.current.add(key)
    announceUpdate(t, updates, update)
  }, [updates, update, t])

  return updates ? update : null
}
