import { Button } from "@ui-sdk/react/button"
import { Dialog, DialogBackdrop, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/react/dialog"
import { Radio, RadioGroup, RadioIndicator } from "@ui-sdk/react/radio"
import * as React from "react"
import type { Meta } from "../api/types"
import { useI18n } from "../i18n"
import { fold } from "../lib/text"
import { useStore, type Build } from "../store"

// Stuffs listed at once; beyond that the player narrows the search.
const SHOWN = 30

interface PickOptions {
  title: string
  hint: string
  confirmLabel: string
}

interface Request extends PickOptions {
  resolve(build: Build | null): void
}

let current: Request | null = null
const listeners = new Set<() => void>()

function settle(build: Build | null) {
  const request = current
  current = null
  request?.resolve(build)
  for (const listener of listeners) listener()
}

// Asks the player which stuff an action is for. Resolves with the stuff, or
// null when the dialog is dismissed. BuildPickerHost shows the dialog.
export function pickBuild(options: PickOptions): Promise<Build | null> {
  return new Promise((resolve) => {
    settle(null)
    current = { ...options, resolve }
    for (const listener of listeners) listener()
  })
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// The dialog behind pickBuild; rendered once, near the root of the app.
export function BuildPickerHost({ meta }: { meta: Meta }) {
  const { t } = useI18n()
  const request = React.useSyncExternalStore(subscribe, () => current)
  const builds = useStore((s) => s.builds)
  const [chosen, setChosen] = React.useState<string | null>(null)
  const [query, setQuery] = React.useState("")
  // The texts stay while the dialog closes, after the request is settled.
  const [shownRequest, setShownRequest] = React.useState<Request | null>(null)

  const activeBuildId = useStore((s) => s.activeBuildId)

  // Each question starts on the stuff the player works on.
  if (request && request !== shownRequest) {
    setShownRequest(request)
    setChosen(activeBuildId)
    setQuery("")
  }

  const breedName = (id: number | null) => meta.breeds.find((b) => b.id === id)?.name
  const needle = fold(query.trim())
  // The chosen stuff stays in view, then the most recently changed ones.
  const matches = builds
    .filter(
      (b) =>
        b.id === chosen || !needle || fold(b.name).includes(needle) || fold(breedName(b.breedId) ?? "").includes(needle)
    )
    .sort((a, b) => Number(b.id === chosen) - Number(a.id === chosen) || b.updatedAt - a.updatedAt)
  const shown = matches.slice(0, SHOWN)
  const target = builds.find((b) => b.id === chosen)
  const texts = request ?? shownRequest

  return (
    <Dialog open={request !== null} onOpenChange={(next) => !next && settle(null)}>
      <DialogBackdrop />
      <DialogContent className="apply-dialog">
        <form
          className="grid gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (target) settle(target)
          }}
        >
          <div className="grid gap-1">
            <DialogTitle>{texts?.title}</DialogTitle>
            <DialogDescription>{texts?.hint}</DialogDescription>
          </div>
          {builds.length > 6 && (
            <input
              type="search"
              className="text-input"
              aria-label={t("build.search")}
              placeholder={t("build.search")}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          )}
          <RadioGroup
            aria-label={texts?.title}
            value={chosen}
            onValueChange={(id) => typeof id === "string" && setChosen(id)}
            className="apply-list"
          >
            {shown.map((b) => (
              <label key={b.id} className="apply-option">
                <Radio value={b.id}>
                  <RadioIndicator />
                </Radio>
                <span className="grid min-w-0">
                  <span className="truncate font-semibold">{b.name || t("build.unnamed")}</span>
                  <span className="truncate text-[0.86rem] text-ink-3">
                    {[
                      breedName(b.breedId),
                      t("item.level", { level: b.level }),
                      t("solver.applyItems", { count: Object.keys(b.items).length }),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </span>
                </span>
              </label>
            ))}
          </RadioGroup>
          {matches.length > shown.length && (
            <p className="m-0 text-[0.86rem] text-ink-2">{t("solver.applyMore", { count: matches.length - shown.length })}</p>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => settle(null)}>
              {t("build.cancel")}
            </Button>
            <Button type="submit" data-variant="solid" disabled={!target}>
              {texts?.confirmLabel}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
