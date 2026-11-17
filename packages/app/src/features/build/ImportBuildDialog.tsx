import { Button } from "@ui-sdk/react/button"
import { Dialog, DialogBackdrop, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/react/dialog"
import * as React from "react"
import type { Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { readShared } from "../../lib/share"
import { useStore } from "../../store"
import { toaster } from "../../ui/toaster"

interface Props {
  meta: Meta
  open: boolean
  onOpenChange(open: boolean): void
}

// A stuff shared by another player arrives as a code (desktop) or a link (web);
// either one pasted here becomes a new stuff in its own tab.
export function ImportBuildDialog({ meta, open, onOpenChange }: Props) {
  const { t } = useI18n()
  const addBuild = useStore((s) => s.addBuild)
  const [text, setText] = React.useState("")
  const [wasOpen, setWasOpen] = React.useState(false)
  const fieldId = React.useId()
  const statusId = React.useId()

  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setText("")
  }

  const build = React.useMemo(() => (text.trim() ? readShared(text) : null), [text])
  const invalid = text.trim() !== "" && build === null
  const breed = build ? meta.breeds.find((b) => b.id === build.breedId)?.name : undefined

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogBackdrop />
      <DialogContent className="import-build" initialFocus={`#${CSS.escape(fieldId)}`}>
        <form
          className="grid gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!build) return
            addBuild(build)
            toaster(t("build.importedCode", { name: build.name }))
            onOpenChange(false)
          }}
        >
          <div className="grid gap-1">
            <DialogTitle>{t("build.importTitle")}</DialogTitle>
            <DialogDescription>{t("build.importHint")}</DialogDescription>
          </div>
          <div className="grid gap-1.5">
            <label htmlFor={fieldId} className="font-semibold">
              {t("build.importLabel")}
            </label>
            <textarea
              id={fieldId}
              className="text-input import-build-field"
              rows={4}
              spellCheck={false}
              autoComplete="off"
              value={text}
              aria-invalid={invalid || undefined}
              aria-describedby={statusId}
              onChange={(event) => setText(event.target.value)}
            />
            <p id={statusId} className="m-0 min-h-[1.3em] text-[0.86rem]" aria-live="polite">
              {invalid && <span className="text-bad">{t("build.importInvalid")}</span>}
              {build && (
                <span className="text-ink-2">
                  {t("build.importPreview", {
                    name: build.name,
                    breed: breed ?? t("build.noBreed"),
                    level: build.level,
                    count: Object.keys(build.items).length,
                  })}
                </span>
              )}
            </p>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => onOpenChange(false)}>
              {t("build.cancel")}
            </Button>
            <Button type="submit" data-variant="solid" disabled={!build}>
              {t("build.importConfirm")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
