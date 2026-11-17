import { Button } from "@ui-sdk/react/button"
import { Dialog, DialogBackdrop, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/react/dialog"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import * as React from "react"
import type { Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { newBuild, useStore } from "../../store"
import { nextBuildName } from "./BuildTabs"

interface Props {
  meta: Meta
  open: boolean
  onOpenChange(open: boolean): void
}

// A new stuff needs a name and a class before it opens in its tab.
export function NewBuildDialog({ meta, open, onOpenChange }: Props) {
  const { t } = useI18n()
  const builds = useStore((s) => s.builds)
  const addBuild = useStore((s) => s.addBuild)
  const [name, setName] = React.useState("")
  const [breedId, setBreedId] = React.useState<number | null>(null)
  const [wasOpen, setWasOpen] = React.useState(false)
  const nameId = React.useId()

  // Each opening starts from a free name and no class.
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      setName(nextBuildName((n) => t("build.numbered", { n }), builds.map((b) => b.name)))
      setBreedId(null)
    }
  }

  const trimmed = name.trim()
  const taken = builds.some((b) => b.name === trimmed)
  const ready = trimmed !== "" && breedId !== null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogBackdrop />
      <DialogContent className="new-build" initialFocus={`#${CSS.escape(nameId)}`}>
        <form
          className="grid gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!ready) return
            const build = newBuild(trimmed)
            build.breedId = breedId
            addBuild(build)
            onOpenChange(false)
          }}
        >
          <div className="grid gap-1">
            <DialogTitle>{t("build.new")}</DialogTitle>
            <DialogDescription>{t("build.newHint")}</DialogDescription>
          </div>
          <div className="grid gap-1.5">
            <label htmlFor={nameId} className="font-semibold">
              {t("build.name")}
            </label>
            <input
              id={nameId}
              className="text-input"
              value={name}
              maxLength={80}
              required
              onChange={(event) => setName(event.target.value)}
            />
            {taken && <p className="m-0 text-[0.86rem] text-ink-2">{t("build.nameTaken")}</p>}
          </div>
          <div className="grid gap-1.5">
            <span className="font-semibold">{t("build.breed")}</span>
            <Select
              value={breedId === null ? null : String(breedId)}
              placeholder={t("build.pickBreed")}
              onValueChange={(id) => setBreedId(id ? Number(id) : null)}
            >
              <SelectTrigger aria-label={t("build.breed")} className="justify-between">
                <SelectValue />
              </SelectTrigger>
              <SelectContent className="max-h-72">
                {meta.breeds.map((breed) => (
                  <SelectItem key={breed.id} value={String(breed.id)} label={breed.name}>
                    {breed.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => onOpenChange(false)}>
              {t("build.cancel")}
            </Button>
            <Button type="submit" data-variant="solid" disabled={!ready}>
              {t("build.create")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
