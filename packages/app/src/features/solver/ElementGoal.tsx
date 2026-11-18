import { Button } from "@ui-sdk/react/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import { PlusIcon, XIcon } from "lucide-react"
import type { DamageMeasure, ElementEntry, ElementMix, WeaponElement } from "../../api/types"
import { useI18n } from "../../i18n"
import { addElement, elementsOf, MAX_ELEMENTS, weaponElements } from "../../lib/maximize"
import { NumberField } from "../../ui/NumberField"

const measures: DamageMeasure[] = ["mean", "crit", "normal"]
const mixes: ElementMix[] = ["floor", "sum"]

function ElementSelect({
  label,
  value,
  options,
  auto,
  onChange,
}: {
  label: string
  value: WeaponElement | "auto"
  options: WeaponElement[]
  auto: boolean
  onChange(value: WeaponElement | "auto"): void
}) {
  const { t } = useI18n()
  return (
    <Select value={value} onValueChange={(next) => onChange(next && next !== "auto" ? (next as WeaponElement) : "auto")}>
      <SelectTrigger aria-label={label} className="min-w-0 flex-1 justify-between">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {auto && (
          <SelectItem value="auto" label={t("solver.finalDamage.auto")}>
            {t("solver.finalDamage.auto")}
          </SelectItem>
        )}
        {options.map((element) => (
          <SelectItem key={element} value={element} label={t(`element.${element}`)}>
            <span className="pip" data-tone={element} />
            {t(`element.${element}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

// The elements of a "final damage" goal: one, left to the search or named, or
// several with what each counts for.
export function ElementGoalOptions({ entry, onChange }: { entry: ElementEntry; onChange(entry: ElementEntry): void }) {
  const { t, colon } = useI18n()
  const others = entry.others ?? []
  const several = others.length > 0
  const mix = entry.mix ?? "floor"
  const named = elementsOf(entry)
  // An element is named once: each list offers its own and the free ones.
  const free = (own: WeaponElement | "auto") => weaponElements.filter((element) => element === own || !named.includes(element))

  const measure = (
    <Select
      value={entry.measure}
      onValueChange={(next) => onChange({ ...entry, measure: next === "crit" || next === "normal" ? next : "mean" })}
    >
      <SelectTrigger aria-label={t("solver.damage.hit")} className="min-w-28 flex-1 justify-between">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {measures.map((value) => (
          <SelectItem key={value} value={value} label={t(`solver.damage.measure.${value}`)}>
            {t(`solver.damage.measure.${value}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  return (
    <div className="grid min-w-0 gap-1.5 pr-1.5 pb-1">
      <div className="flex flex-wrap items-center gap-2">
        <ElementSelect
          label={several ? t("solver.elements.main") : t("solver.finalDamage.element")}
          value={entry.element}
          options={free(entry.element)}
          auto={!several}
          onChange={(element) => onChange({ ...entry, element })}
        />
        {!several && measure}
        {several && mix === "floor" && <span className="text-ink-3">{t("solver.elements.main")}</span>}
        {several && mix === "sum" && entry.element !== "auto" && (
          <span className="flex items-center gap-2">
            <span className="text-ink-3">{t("solver.elements.weight")}</span>
            <NumberField
              label={`${t("solver.elements.weight")}${colon}${t(`element.${entry.element}`)}`}
              value={entry.share ?? 100}
              min={1}
              max={100}
              onChange={(share) => onChange({ ...entry, share })}
            />
          </span>
        )}
      </div>
      {others.map((other, index) => {
        const name = t(`element.${other.element}`)
        const replace = (next: typeof other) => onChange({ ...entry, others: others.map((o, i) => (i === index ? next : o)) })
        return (
          <div key={other.element} className="flex items-center gap-2">
            <ElementSelect
              label={t("solver.finalDamage.element")}
              value={other.element}
              options={free(other.element)}
              auto={false}
              onChange={(element) => element !== "auto" && replace({ ...other, element })}
            />
            <span className="flex items-center gap-2">
              <span className="text-ink-3">{mix === "floor" ? "≥" : t("solver.elements.weight")}</span>
              <NumberField
                label={`${t(mix === "floor" ? "solver.elements.floor" : "solver.elements.weight")}${colon}${name}`}
                value={other.share}
                min={1}
                max={100}
                onChange={(share) => replace({ ...other, share })}
              />
              {mix === "floor" && <span className="text-ink-3">%</span>}
            </span>
            <button
              type="button"
              className="icon-btn size-6!"
              aria-label={`${t("common.remove")}${colon}${name}`}
              onClick={() => onChange({ ...entry, others: others.filter((_, i) => i !== index) })}
            >
              <XIcon aria-hidden="true" />
            </button>
          </div>
        )
      })}
      {several && (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <Select value={mix} onValueChange={(next) => onChange({ ...entry, mix: next === "sum" ? "sum" : "floor" })}>
              <SelectTrigger aria-label={t("solver.elements.mix")} className="min-w-28 flex-1 justify-between">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {mixes.map((value) => (
                  <SelectItem key={value} value={value} label={t(`solver.elements.mix.${value}`)}>
                    {t(`solver.elements.mix.${value}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {measure}
          </div>
          <p className="m-0 text-[0.86rem] text-ink-3">{t(`solver.elements.hint.${mix}`)}</p>
        </>
      )}
      {named.length < MAX_ELEMENTS && (
        <div>
          <Button onClick={() => onChange(addElement(entry))}>
            <PlusIcon aria-hidden="true" className="size-4" />
            {t("solver.elements.add")}
          </Button>
        </div>
      )}
    </div>
  )
}
