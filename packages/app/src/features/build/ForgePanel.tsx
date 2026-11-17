import { Button } from "@ui-sdk/react/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import { RotateCcwIcon, XIcon } from "lucide-react"
import * as React from "react"
import type {
  Draco,
  DracoKey,
  DracoRune,
  Element,
  Evaluation,
  ForgedLine,
  Item,
  ItemMods,
  Meta,
  Stat,
  WeaponElement,
} from "../../api/types"
import { useI18n } from "../../i18n"
import { statLabel, toneOf } from "../../lib/stats"
import { isEmptyMods } from "../../store"
import { ForgedLines } from "../../ui/ForgedLines"
import { NumberField } from "../../ui/NumberField"
import { SearchSelect } from "../../ui/SearchSelect"
import { StatSelect } from "../../ui/StatSelect"

const weaponElements: WeaponElement[] = ["earth", "fire", "water", "air"]

interface NaturalLine {
  stat: Stat
  min: number
  max: number
  cap: number
}

// The item's own stat lines, summed per stat, with how high each can be forged.
function naturalLines(item: Item, stats: Map<number, Stat>): NaturalLine[] {
  const lines = new Map<number, NaturalLine>()
  for (const effect of item.effects) {
    if (effect.kind !== "stat" || effect.statId === null) continue
    const stat = stats.get(effect.statId)
    if (!stat) continue
    const line = lines.get(stat.id) ?? { stat, min: 0, max: 0, cap: 0 }
    line.min += effect.min
    line.max += effect.max
    lines.set(stat.id, line)
  }
  for (const line of lines.values()) line.cap = Math.max(line.max, line.stat.forgeMax)
  return [...lines.values()]
}

// The elements a dracorune can convert on this item, and to what.
function dracoChoices(rune: DracoRune, meta: Meta, natural: Set<number>): { from: Element[]; to: Element[] } {
  const ids = meta.forge.elementIds
  const has = (id: number) => id >= 0 && natural.has(id)
  if (rune.kind === "element" && rune.target) {
    const target = ids[rune.target]
    if (has(target.statId) || has(target.damageId)) return { from: [], to: [] }
    const from = meta.forge.elements.filter(
      (el) => el !== rune.target && el !== "neutral" && (has(ids[el].statId) || has(ids[el].damageId))
    )
    return { from, to: [rune.target] }
  }
  if (rune.kind === "resistance") {
    const from = meta.forge.elements.filter((el) => has(ids[el].resFixedId) || has(ids[el].resPercentId))
    const to = meta.forge.elements.filter((el) => !has(ids[el].resFixedId) && !has(ids[el].resPercentId))
    return { from, to }
  }
  return { from: [], to: [] }
}

function dracoUsable(rune: DracoRune, meta: Meta, item: Item, natural: Set<number>): boolean {
  if (rune.kind === "power") {
    const ids = meta.forge.elementIds
    const positive = new Set(
      item.effects.filter((e) => e.kind === "stat" && e.max > 0 && e.statId !== null).map((e) => e.statId as number)
    )
    return meta.forge.elements.some((el) => positive.has(ids[el].statId) || positive.has(ids[el].damageId))
  }
  const choices = dracoChoices(rune, meta, natural)
  return choices.from.length > 0 && choices.to.length > 0
}

interface Props {
  meta: Meta
  item: Item
  position: string
  mods: ItemMods | undefined
  evaluation: Evaluation | undefined
  onChange(mods: ItemMods | null): void
}

export function ForgePanel({ meta, item, position, mods, evaluation, onChange }: Props) {
  const { t, colon } = useI18n()
  const stats = React.useMemo(() => new Map(meta.stats.map((s) => [s.id, s])), [meta])
  const lines = React.useMemo(() => naturalLines(item, stats), [item, stats])
  const natural = React.useMemo(() => new Set(lines.map((l) => l.stat.id)), [lines])

  const current: ItemMods = mods ?? {}
  const exos = current.exos ?? []
  const patch = (next: Partial<ItemMods>) => onChange({ ...current, ...next })

  const forged: ForgedLine[] = evaluation?.forge[position] ?? []
  const issues = evaluation?.issues.filter((issue) => issue.position === position && issue.code.startsWith("forge_")) ?? []

  const limited = meta.forge.limitedExos
  const hasLimitedExo = exos.some((exo) => limited.includes(exo.statId))
  const exoIds = exos.map((exo) => exo.statId)

  // Only the runes the item can take are offered, plus the one it holds.
  const fitting = evaluation?.runes[position]
  const runeGroups = React.useMemo(
    () => [
      {
        label: "",
        options: meta.forge.runes
          .filter((rune) => !fitting || fitting.includes(rune.id) || rune.id === current.transcendence)
          .map((rune) => ({
            value: String(rune.id),
            label: `${rune.text} · ${rune.name} · ${t("forge.density", { density: rune.density })}`,
            tone: toneOf(stats.get(rune.statId)?.key),
          })),
      },
    ],
    [meta, stats, fitting, current.transcendence, t]
  )

  // A weapon whose attack has a Neutral line can be forged to an element.
  const neutralWeapon = item.weapon !== null && item.effects.some((e) => e.kind === "weapon" && e.element === 0)

  const dracoRune = meta.forge.dracoRunes.find((rune) => rune.key === current.draco?.rune)
  const choices = dracoRune ? dracoChoices(dracoRune, meta, natural) : { from: [], to: [] }

  function setLine(line: NaturalLine, value: number) {
    const next = { ...current.lines }
    if (value === line.max) delete next[String(line.stat.id)]
    else next[String(line.stat.id)] = value
    patch({ lines: next })
  }

  function chooseDraco(key: string) {
    const rune = meta.forge.dracoRunes.find((r) => r.key === key)
    if (!rune) return patch({ draco: undefined })
    const options = dracoChoices(rune, meta, natural)
    const draco: Draco = { rune: rune.key }
    if (rune.kind !== "power") {
      draco.from = options.from[0]
      draco.to = rune.kind === "resistance" ? options.to.find((el) => el !== draco.from) : rune.target
    }
    patch({ draco })
  }

  return (
    <div className="grid content-start gap-5 p-4">
      <div className="flex items-start justify-between gap-3">
        <p className="m-0 text-ink-2">{t("forge.hint")}</p>
        {!isEmptyMods(mods) && (
          <Button className="flex-none" onClick={() => onChange(null)}>
            <RotateCcwIcon aria-hidden="true" className="size-4" />
            {t("forge.reset")}
          </Button>
        )}
      </div>

      {issues.length > 0 && (
        <ul className="m-0 grid list-none gap-1 rounded-lg border border-bad bg-bad-soft p-3" role="alert">
          {issues.map((issue) => (
            <li key={issue.code}>{t(`build.issue.${issue.code}`, { item: item.name })}</li>
          ))}
        </ul>
      )}

      <section className="grid gap-2">
        <h3 className="section-title">{t("forge.lines")}</h3>
        <ul className="m-0 grid list-none gap-1.5 p-0">
          {lines.map((line) => {
            const value = current.lines?.[String(line.stat.id)] ?? line.max
            const over = value > line.max
            return (
              <li key={line.stat.id} className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2">
                <span className="flex min-w-0 items-center gap-2">
                  <span className="pip" data-tone={toneOf(line.stat.key)} />
                  <span className="truncate">{statLabel(line.stat)}</span>
                  {over && <span className="tag" data-origin="over">{t("forge.origin.over")}</span>}
                </span>
                <span className="text-[0.86rem] whitespace-nowrap text-ink-3">
                  {line.min === line.max ? line.max : `${line.min} – ${line.max}`}
                  {line.cap > line.max ? ` · ${t("forge.cap", { cap: line.cap })}` : ""}
                </span>
                <NumberField
                  label={`${statLabel(line.stat)}, ${t("filter.value")}`}
                  value={value}
                  min={line.min}
                  max={line.cap}
                  onChange={(next) => setLine(line, next)}
                />
              </li>
            )
          })}
        </ul>
      </section>

      <section className="grid gap-2">
        <h3 className="section-title">{t("forge.exos")}</h3>
        {exos.length > 0 && (
          <ul className="m-0 grid list-none gap-1.5 p-0">
            {exos.map((exo, index) => {
              const stat = stats.get(exo.statId)
              return (
                <li key={exo.statId} className="grid grid-cols-[minmax(0,1fr)_auto_2rem] items-center gap-2">
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="pip" data-tone={toneOf(stat?.key)} />
                    <span className="truncate">{statLabel(stat)}</span>
                    <span className="tag" data-origin="exo">{t("forge.origin.exo")}</span>
                  </span>
                  <NumberField
                    label={`${statLabel(stat)}, ${t("filter.value")}`}
                    value={exo.value}
                    min={1}
                    max={stat?.forgeMax ?? 1}
                    onChange={(value) => patch({ exos: exos.map((e, i) => (i === index ? { ...e, value } : e)) })}
                  />
                  <button
                    type="button"
                    className="icon-btn"
                    aria-label={`${t("common.remove")}${colon}${statLabel(stat)}`}
                    onClick={() => patch({ exos: exos.filter((_, i) => i !== index) })}
                  >
                    <XIcon aria-hidden="true" />
                  </button>
                </li>
              )
            })}
          </ul>
        )}
        <div className="flex flex-wrap gap-1.5">
          {limited.map((statId) => {
            const stat = stats.get(statId)
            if (!stat) return null
            return (
              <button
                key={statId}
                type="button"
                className="chip"
                disabled={natural.has(statId) || hasLimitedExo}
                onClick={() => patch({ exos: [...exos, { statId, value: 1 }] })}
              >
                <span className="pip" data-tone={toneOf(stat.key)} />
                {t("forge.addExo", { stat: stat.label })}
              </button>
            )
          })}
        </div>
        <StatSelect
          meta={meta}
          label={t("forge.otherExo")}
          placeholder={t("forge.otherExo")}
          value={null}
          forgeable
          exclude={[...natural, ...exoIds, ...(hasLimitedExo ? limited : [])]}
          onChange={(statId) => patch({ exos: [...exos, { statId, value: 1 }] })}
        />
      </section>

      <section className="grid gap-2">
        <h3 className="section-title">{t("forge.transcendence")}</h3>
        <div className="flex items-center gap-1.5">
          <SearchSelect
            label={t("forge.transcendence")}
            groups={runeGroups}
            value={current.transcendence ? String(current.transcendence) : null}
            placeholder={t("forge.pickRune")}
            emptyText={t("forge.noRune")}
            showLabel={t("forge.showRunes")}
            onChange={(id) => patch({ transcendence: Number(id) })}
          />
          {current.transcendence ? (
            <button
              type="button"
              className="icon-btn"
              aria-label={`${t("common.remove")}${colon}${t("forge.transcendence")}`}
              onClick={() => patch({ transcendence: undefined })}
            >
              <XIcon aria-hidden="true" />
            </button>
          ) : null}
        </div>
        <p className="m-0 text-[0.86rem] text-ink-2">{t("forge.transcendenceHint")}</p>
      </section>

      <section className="grid gap-2">
        <h3 className="section-title flex items-center gap-2">
          {t("forge.draco")}
          <span className="tag">{t("forge.dracoVersion")}</span>
        </h3>
        <div className="flex flex-wrap gap-1.5">
          {meta.forge.dracoRunes.map((rune) => {
            const pressed = current.draco?.rune === rune.key
            return (
              <button
                key={rune.key}
                type="button"
                className="chip"
                aria-pressed={pressed}
                disabled={!pressed && !dracoUsable(rune, meta, item, natural)}
                title={t(`draco.${rune.key}.effect`)}
                onClick={() => chooseDraco(pressed ? "" : rune.key)}
              >
                {t(`draco.${rune.key}`)}
              </button>
            )
          })}
        </div>
        {dracoRune && dracoRune.kind !== "power" && current.draco && (
          <div className="flex flex-wrap items-center gap-2">
            <ElementSelect
              label={t("forge.dracoFrom")}
              value={current.draco.from}
              options={choices.from}
              onChange={(from) => patch({ draco: { ...current.draco!, from } })}
            />
            <span className="text-ink-3">→</span>
            {dracoRune.kind === "resistance" ? (
              <ElementSelect
                label={t("forge.dracoTo")}
                value={current.draco.to}
                options={choices.to.filter((el) => el !== current.draco?.from)}
                onChange={(to) => patch({ draco: { ...current.draco!, to } })}
              />
            ) : (
              <strong>{t(`element.${dracoRune.target!}`)}</strong>
            )}
          </div>
        )}
        <p className="m-0 text-[0.86rem] text-ink-2">
          {dracoRune ? t(`draco.${dracoRune.key as DracoKey}.effect`) : t("forge.dracoHint")}
        </p>
      </section>

      {neutralWeapon && (
        <section className="grid gap-2">
          <h3 className="section-title">{t("forge.weaponElement")}</h3>
          <div className="flex flex-wrap gap-1.5">
            {weaponElements.map((element) => {
              const pressed = current.element === element
              return (
                <button
                  key={element}
                  type="button"
                  className="chip"
                  aria-pressed={pressed}
                  onClick={() => patch({ element: pressed ? undefined : element })}
                >
                  <span className="pip" data-tone={element} />
                  {t(`element.${element}`)}
                </button>
              )
            })}
          </div>
          <p className="m-0 text-[0.86rem] text-ink-2">{t("forge.weaponElementHint")}</p>
        </section>
      )}

      <section className="grid gap-2">
        <h3 className="section-title">{t("forge.result")}</h3>
        <ForgedLines lines={forged} stats={stats} />
      </section>
    </div>
  )
}

function ElementSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: Element | undefined
  options: Element[]
  onChange(element: Element): void
}) {
  const { t } = useI18n()
  return (
    <Select value={value ?? null} placeholder={label} onValueChange={(next) => next && onChange(next as Element)}>
      <SelectTrigger aria-label={label} className="w-auto! min-w-28! justify-between">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
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
