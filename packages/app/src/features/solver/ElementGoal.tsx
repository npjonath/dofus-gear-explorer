import { Button } from "@ui-sdk/react/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import { PlusIcon, XIcon } from "lucide-react"
import type { DamageMeasure, ElementEntry, ElementMix, ElementTarget, Meta, WeaponElement } from "../../api/types"
import { useI18n } from "../../i18n"
import { addElement, elementsOf, MAX_ELEMENTS, spellHitsIn, weaponElements } from "../../lib/maximize"
import { ItemSearch } from "../../ui/ItemSearch"
import { NumberField } from "../../ui/NumberField"
import { SearchSelect } from "../../ui/SearchSelect"
import { GoalName } from "./GoalName"

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

// What an element is read on: every spell of the class until the player
// names one spell, or the weapon.
function TargetPicker({
  meta,
  breedId,
  element,
  target,
  onChange,
}: {
  meta: Meta
  breedId: number
  element: WeaponElement
  target: ElementTarget | undefined
  onChange(target: ElementTarget | undefined): void
}) {
  const { t, colon } = useI18n()
  const name = t(`element.${element}`)
  if (target) {
    return (
      <div className="flex min-h-[1.9rem] items-center gap-2 pl-1">
        <span className="min-w-0 flex-1 truncate">
          <span className="font-semibold">
            <GoalName meta={meta} goal={target} />
          </span>
          <span className="text-ink-3"> · {t(`solver.damage.${target.kind}`)}</span>
        </span>
        <button
          type="button"
          className="icon-btn size-6!"
          aria-label={`${t("solver.elements.allSpells")}${colon}${name}`}
          title={t("solver.elements.allSpells")}
          onClick={() => onChange(undefined)}
        >
          <XIcon aria-hidden="true" />
        </button>
      </div>
    )
  }
  const options = (breed: number) =>
    meta.spells
      .filter((spell) => spell.breedId === breed && spell.damage && spellHitsIn(spell, element))
      .map((spell) => ({ value: String(spell.id), label: spell.name }))
  const breed = meta.breeds.find((b) => b.id === breedId)
  const groups = [
    ...(breed ? [{ label: breed.name, options: options(breed.id) }] : []),
    { label: t("damage.common"), options: options(0) },
  ]
  return (
    <div className="grid grid-cols-2 gap-1.5">
      <SearchSelect
        label={`${t("solver.elements.spell")}${colon}${name}`}
        groups={groups}
        value={null}
        placeholder={t("solver.elements.allSpells")}
        emptyText={t("solver.damage.noSpell")}
        showLabel={`${t("solver.elements.spell")}${colon}${name}`}
        onChange={(id) => onChange({ kind: "spell", id: Number(id) })}
      />
      <ItemSearch
        label={`${t("solver.elements.weapon")}${colon}${name}`}
        placeholder={t("solver.damage.addWeapon")}
        slots={["weapon"]}
        onPick={(item) => onChange({ kind: "weapon", id: item.id })}
      />
    </div>
  )
}

// The elements of a "final damage" goal: one, left to the search or named, or
// several with what each counts for and what each is read on.
export function ElementGoalOptions({
  meta,
  breedId,
  entry,
  onChange,
}: {
  meta: Meta
  breedId: number
  entry: ElementEntry
  onChange(entry: ElementEntry): void
}) {
  const { t, colon } = useI18n()
  // A spell named for an element is dropped when the element changes to one it does not hit in.
  const kept = (target: ElementTarget | undefined, element: WeaponElement) => {
    const spell = target?.kind === "spell" ? meta.spells.find((s) => s.id === target.id) : undefined
    return target?.kind === "spell" && !(spell && spellHitsIn(spell, element)) ? undefined : target
  }
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
          onChange={(element) =>
            onChange({ ...entry, element, target: element === "auto" ? undefined : kept(entry.target, element) })
          }
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
      {several && entry.element !== "auto" && (
        <TargetPicker
          meta={meta}
          breedId={breedId}
          element={entry.element}
          target={entry.target}
          onChange={(target) => onChange({ ...entry, target })}
        />
      )}
      {others.map((other, index) => {
        const name = t(`element.${other.element}`)
        const replace = (next: typeof other) => onChange({ ...entry, others: others.map((o, i) => (i === index ? next : o)) })
        return (
          <div key={other.element} className="grid gap-1.5 border-t border-line pt-1.5">
          <div className="flex items-center gap-2">
            <ElementSelect
              label={t("solver.finalDamage.element")}
              value={other.element}
              options={free(other.element)}
              auto={false}
              onChange={(element) => element !== "auto" && replace({ ...other, element, target: kept(other.target, element) })}
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
            <TargetPicker
              meta={meta}
              breedId={breedId}
              element={other.element}
              target={other.target}
              onChange={(target) => replace({ ...other, target })}
            />
          </div>
        )
      })}
      {several && (
        <>
          <div className="flex flex-wrap items-center gap-2 border-t border-line pt-1.5">
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
