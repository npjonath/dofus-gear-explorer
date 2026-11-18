import { Dialog, DialogBackdrop, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/react/dialog"
import { SegmentedControl, SegmentedControlIndicator, SegmentedControlItem } from "@ui-sdk/react/segmented-control"
import { Tabs, TabsList, TabsPanel, TabsTab } from "@ui-sdk/react/tabs"
import { ArrowDownIcon, ArrowLeftRightIcon, ArrowRightIcon, ArrowUpIcon, TriangleAlertIcon, XIcon } from "lucide-react"
import * as React from "react"
import { useDamage, useEvaluation } from "../../api/queries"
import type {
  Attack,
  BuildInput,
  DamageMeasure,
  DamageResult,
  Evaluation,
  Meta,
  Stat,
  WeaponElement,
} from "../../api/types"
import { useI18n } from "../../i18n"
import { AP_STAT, HP_STAT, MP_STAT, RANGE_STAT, statIndex, statLabel, toneOf } from "../../lib/stats"
import { fold } from "../../lib/text"
import { toBuildInput, type Build } from "../../store"
import { ItemCell } from "../../ui/ItemCell"
import { ItemHover } from "../../ui/ItemHover"
import { SearchSelect, type SearchGroup } from "../../ui/SearchSelect"
import { SlotGlyph } from "../../ui/SlotGlyph"
import {
  diffCharacter,
  diffDamage,
  diffItems,
  diffSets,
  diffStats,
  hitValue,
  percentChange,
  statDelta,
  type AttackDelta,
  type SlotDiff,
  type StatDelta,
  type Worn,
} from "./diff"

/** A stuff the dialog can compare: a saved one, or a result of the search. */
export interface CompareOption {
  key: string
  label: string
  group: "saved" | "results"
  breedId: number | null
  input: BuildInput
}

export function savedOption(build: Build, unnamed: string): CompareOption {
  return {
    key: build.id,
    label: build.name || unnamed,
    group: "saved",
    breedId: build.breedId,
    input: toBuildInput(build),
  }
}

interface Props {
  meta: Meta
  open: boolean
  onOpenChange(open: boolean): void
  /** Every stuff either side can be. */
  options: CompareOption[]
  /** The stuffs the dialog opens on, by key; the second one may be left to the player. */
  initial: { from: string; to?: string }
}

const vitals = [HP_STAT, AP_STAT, MP_STAT, RANGE_STAT]

const noInput: BuildInput = {
  level: 1,
  base: { vitality: 0, wisdom: 0, strength: 0, intelligence: 0, chance: 0, agility: 0 },
  scrolls: { vitality: 0, wisdom: 0, strength: 0, intelligence: 0, chance: 0, agility: 0 },
  items: {},
  mods: {},
}

// Sets two stuffs side by side and says what going from the first to the
// second gains and loses. The content only lives while the dialog is open,
// so each opening starts from the stuffs it was asked for.
export function CompareDialog({ meta, open, onOpenChange, options, initial }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogBackdrop />
      <DialogContent className="compare-dialog" initialFocus="content">
        <CompareContent meta={meta} options={options} initial={initial} />
      </DialogContent>
    </Dialog>
  )
}

// A value that went up, went down or stayed: the sign says it, the colour repeats it.
function Delta({ value, unit = "" }: { value: number | null; unit?: string }) {
  const { number } = useI18n()
  if (value === null) return <span className="compare-delta" data-sign="none">–</span>
  if (value === 0) return <span className="compare-delta" data-sign="none">=</span>
  return (
    <span className="compare-delta" data-sign={value > 0 ? "up" : "down"}>
      {value > 0 ? "+" : "−"}
      {number(Math.abs(value))}
      {unit}
    </span>
  )
}

interface DeltaRow {
  key: string
  label: React.ReactNode
  from: string
  to: string
  delta: number | null
  unit?: string
  /** A second reading of the change, such as a percentage. */
  note?: string
}

interface DeltaGroup {
  title?: string
  rows: DeltaRow[]
  /** What to say when the group has no row. */
  empty?: string
}

// The table every part of the comparison uses: what it is, its value in each
// stuff, and the change.
function DeltaTable({
  caption,
  first,
  names,
  showHead,
  groups,
}: {
  caption?: React.ReactNode
  /** The heading of the first column. */
  first: string
  names: { from: string; to: string }
  /** Shows the column headings; without it only screen readers read them. */
  showHead?: boolean
  groups: DeltaGroup[]
}) {
  const { t, colon } = useI18n()
  return (
    <table className="compare-table">
      {caption && <caption>{caption}</caption>}
      <thead className={showHead ? undefined : "sr-only"}>
        <tr>
          <th scope="col">{first}</th>
          <th scope="col">
            {t("compare.from")}
            <span className="sr-only">
              {colon}
              {names.from}
            </span>
          </th>
          <th scope="col">
            {t("compare.to")}
            <span className="sr-only">
              {colon}
              {names.to}
            </span>
          </th>
          <th scope="col">{t("compare.delta")}</th>
        </tr>
      </thead>
      {groups.map((group, index) => (
        <tbody key={group.title ?? index}>
          {group.title && (
            <tr className="compare-group">
              <th colSpan={4} scope="rowgroup">
                {group.title}
              </th>
            </tr>
          )}
          {group.rows.length === 0 && group.empty && (
            <tr>
              <td colSpan={4} className="compare-none">
                {group.empty}
              </td>
            </tr>
          )}
          {group.rows.map((row) => (
            <tr key={row.key} data-same={row.delta === 0 ? "" : undefined}>
              <th scope="row">
                <span className="compare-name">{row.label}</span>
              </th>
              <td className="from">{row.from}</td>
              <td className="to">{row.to}</td>
              <td className="delta">
                <Delta value={row.delta} unit={row.unit} />
                {row.note && <span className="note">{row.note}</span>}
              </td>
            </tr>
          ))}
        </tbody>
      ))}
    </table>
  )
}

function StatName({ stat }: { stat: Stat | undefined }) {
  const { t } = useI18n()
  const label = stat?.id === HP_STAT ? t("stat.hp") : statLabel(stat)
  return (
    <>
      <span className="pip" data-tone={toneOf(stat?.key)} />
      <span className="truncate" title={label}>
        {label}
      </span>
    </>
  )
}

function StatsTab({
  names,
  gains,
  losses,
  unchanged,
}: {
  names: { from: string; to: string }
  gains: StatDelta[]
  losses: StatDelta[]
  unchanged: StatDelta[]
}) {
  const { t, number } = useI18n()
  const rows = (deltas: StatDelta[]): DeltaRow[] =>
    deltas.map((row) => ({
      key: String(row.stat.id),
      label: <StatName stat={row.stat} />,
      from: `${number(row.from)}${row.stat.unit}`,
      to: `${number(row.to)}${row.stat.unit}`,
      delta: row.delta,
      unit: row.stat.unit,
    }))
  return (
    <>
      <div className="compare-columns">
        <div>
          <h3 className="compare-heading" data-sign="up">
            <ArrowUpIcon aria-hidden="true" />
            {t("compare.gains", { count: gains.length })}
          </h3>
          {gains.length > 0 ? (
            <DeltaTable first={t("compare.col.stat")} names={names} groups={[{ rows: rows(gains) }]} />
          ) : (
            <p className="m-0 px-1 text-ink-2">{t("compare.noGain")}</p>
          )}
        </div>
        <div>
          <h3 className="compare-heading" data-sign="down">
            <ArrowDownIcon aria-hidden="true" />
            {t("compare.losses", { count: losses.length })}
          </h3>
          {losses.length > 0 ? (
            <DeltaTable first={t("compare.col.stat")} names={names} groups={[{ rows: rows(losses) }]} />
          ) : (
            <p className="m-0 px-1 text-ink-2">{t("compare.noLoss")}</p>
          )}
        </div>
      </div>
      {unchanged.length > 0 && (
        <details className="compare-unchanged">
          <summary>{t("compare.unchanged", { count: unchanged.length })}</summary>
          <div className="compare-columns">
            {unchanged.map((row) => (
              <div key={row.stat.id} className="sheet-row">
                <StatName stat={row.stat} />
                <span className="value ml-auto" data-negative={row.to < 0 ? "" : undefined}>
                  {number(row.to)}
                  {row.stat.unit}
                </span>
              </div>
            ))}
          </div>
        </details>
      )}
    </>
  )
}

// One side of a place: the item, and what forgemagic put on it.
function WornSide({
  worn,
  stats,
  side,
  withForge = true,
}: {
  worn: Worn | undefined
  stats: Map<number, Stat>
  side: "from" | "to"
  withForge?: boolean
}) {
  const { t } = useI18n()
  if (!worn) {
    return (
      <span className="compare-worn text-ink-3" data-side={side}>
        {t("compare.empty")}
      </span>
    )
  }
  const { item } = worn
  const forged = withForge ? worn.forged.filter((line) => line.value !== 0 && (line.origin !== "base" || line.rune > 0)) : []
  return (
    <ItemHover item={item} stats={stats} forged={worn.forged.length > 0 ? worn.forged : undefined} side="bottom">
      <span className="compare-worn" data-side={side}>
        <ItemCell item={item} size="2.5rem" />
        <span className="grid min-w-0 gap-0.5">
          <span className="truncate font-semibold">{item.name}</span>
          <span className="truncate text-[0.86rem] text-ink-3">
            {t("item.level", { level: item.level })}
            {item.set ? ` · ${item.set.name}` : ""}
          </span>
          {forged.length > 0 && (
            <span className="compare-forged">
              {forged.map((line) => (
                <span key={line.statId}>
                  <span className="tag" data-origin={line.origin}>
                    {t(`forge.origin.${line.rune > 0 ? "transcendence" : line.origin}`)}
                  </span>{" "}
                  {line.text}
                </span>
              ))}
              {worn.mods?.element && (
                <span>{t("forge.weaponElement.to", { element: t(`element.${worn.mods.element}`) })}</span>
              )}
            </span>
          )}
        </span>
      </span>
    </ItemHover>
  )
}

function ItemRow({ row, stats, names }: { row: SlotDiff; stats: Map<number, Stat>; names: { from: string; to: string } }) {
  const { t, number, slotName } = useI18n()
  const element = (value: WeaponElement | undefined) =>
    value ? t("forge.weaponElement.to", { element: t(`element.${value}`) }) : t("forge.weaponElement.none")
  return (
    <li className="compare-item" data-change={row.change}>
      <span className="compare-slot">
        <SlotGlyph slot={row.slot} />
        {slotName(row.slot)}
      </span>
      {row.change === "forge" ? (
        <div className="compare-forge">
          <WornSide worn={row.to} stats={stats} side="to" withForge={false} />
          <DeltaTable
            caption={t("compare.forgeOnly")}
            first={t("compare.col.stat")}
            names={names}
            groups={[
              {
                rows: [
                  ...row.lines.map((line) => {
                    const stat = stats.get(line.statId)
                    return {
                      key: String(line.statId),
                      label: (
                        <>
                          <StatName stat={stat} />
                          {(line.origin !== "base" || line.runeFrom > 0 || line.runeTo > 0) && (
                            <span className="tag" data-origin={line.origin}>
                              {t(`forge.origin.${line.origin === "base" ? "transcendence" : line.origin}`)}
                            </span>
                          )}
                        </>
                      ),
                      from: `${number(line.from)}${stat?.unit ?? ""}`,
                      to: `${number(line.to)}${stat?.unit ?? ""}`,
                      delta: line.to - line.from,
                      unit: stat?.unit,
                    }
                  }),
                  ...(row.element
                    ? [
                        {
                          key: "element",
                          label: <span className="truncate">{t("forge.weaponElement")}</span>,
                          from: element(row.element.from),
                          to: element(row.element.to),
                          delta: null,
                        },
                      ]
                    : []),
                ],
              },
            ]}
          />
        </div>
      ) : (
        <>
          <WornSide worn={row.from} stats={stats} side="from" />
          <span className="compare-arrow">
            <ArrowRightIcon aria-hidden="true" />
            <span className="sr-only">{t("compare.replacedBy")}</span>
          </span>
          <WornSide worn={row.to} stats={stats} side="to" />
        </>
      )}
    </li>
  )
}

function ItemsTab({
  meta,
  names,
  from,
  to,
  rows,
}: {
  meta: Meta
  names: { from: string; to: string }
  from: { option: CompareOption; evaluation: Evaluation }
  to: { option: CompareOption; evaluation: Evaluation }
  rows: SlotDiff[]
}) {
  const { t, number, primaryName } = useI18n()
  const stats = React.useMemo(() => statIndex(meta), [meta])
  const changed = rows.filter((row) => row.change !== "same")
  const same = rows.length - changed.length
  const sets = diffSets(from.evaluation, to.evaluation)
  const character = diffCharacter(meta, from.option.input, to.option.input)
  const points: DeltaRow[] = [
    ...(character.level
      ? [
          {
            key: "level",
            label: <span className="truncate">{t("build.level")}</span>,
            from: number(character.level.from),
            to: number(character.level.to),
            delta: character.level.to - character.level.from,
          },
        ]
      : []),
    ...(["base", "scrolls"] as const).flatMap((field) =>
      character[field].map((row) => ({
        key: `${field}-${row.key}`,
        label: (
          <span className="truncate">
            {primaryName(row.key)}
            <span className="text-ink-3"> · {t(field === "base" ? "build.base" : "build.scroll")}</span>
          </span>
        ),
        from: number(row.from),
        to: number(row.to),
        delta: row.to - row.from,
      }))
    ),
  ]
  return (
    <>
      {changed.length > 0 ? (
        <ul className="compare-items">
          {changed.map((row) => (
            <ItemRow key={(row.to ?? row.from)!.position} row={row} stats={stats} names={names} />
          ))}
        </ul>
      ) : (
        <p className="m-0 text-ink-2">{t("compare.noItemChange")}</p>
      )}
      {changed.length > 0 && same > 0 && <p className="m-0 text-ink-2">{t("compare.itemsSame", { count: same })}</p>}
      {sets.length > 0 && (
        <DeltaTable
          caption={t("build.sets")}
          first={t("build.sets")}
          names={names}
          groups={[
            {
              rows: sets.map((set) => ({
                key: String(set.id),
                label: <span className="truncate">{set.name}</span>,
                from: `${set.from}/${set.size}`,
                to: `${set.to}/${set.size}`,
                delta: set.to - set.from,
              })),
            },
          ]}
        />
      )}
      {points.length > 0 && (
        <DeltaTable caption={t("compare.character")} first={t("compare.character")} names={names} groups={[{ rows: points }]} />
      )}
    </>
  )
}

const measures: DamageMeasure[] = ["mean", "normal", "crit"]

function DamageTab({
  meta,
  names,
  breedId,
  mixedBreeds,
  from,
  to,
  failed,
}: {
  meta: Meta
  names: { from: string; to: string }
  breedId: number | null
  mixedBreeds: boolean
  from: DamageResult | undefined
  to: DamageResult | undefined
  failed: boolean
}) {
  const { t, number } = useI18n()
  const [measure, setMeasure] = React.useState<DamageMeasure>("mean")
  const breed = meta.breeds.find((b) => b.id === breedId)

  const value = (attack: Attack | undefined) => (attack ? hitValue(attack, measure) : null)
  const shown = (hit: number | null) => (hit === null ? "–" : number(Math.round(hit)))
  const toRow = (row: AttackDelta): DeltaRow => {
    const a = value(row.from)
    const b = value(row.to)
    const percent = a !== null && b !== null ? percentChange(a, b) : null
    const rounded = percent === null ? 0 : Math.round(percent)
    const name =
      row.from && row.to && row.from.name !== row.to.name ? `${row.from.name} → ${row.to.name}` : (row.to ?? row.from)!.name
    return {
      key: row.key,
      label: (
        <span className="truncate" title={name}>
          {name}
          {(row.to ?? row.from)!.variantOf > 0 && <span className="text-ink-3"> · {t("damage.variant")}</span>}
        </span>
      ),
      from: shown(a),
      to: shown(b),
      delta: a !== null && b !== null ? Math.round(b) - Math.round(a) : null,
      note:
        rounded === 0
          ? undefined
          : t("compare.percent", { value: `${rounded > 0 ? "+" : "−"}${number(Math.abs(rounded))}` }),
    }
  }

  const diff = from && to ? diffDamage(from, to) : null
  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p className="m-0 max-w-[60ch] flex-1 basis-64 text-ink-2">
          {t("compare.damageHint")}
          {mixedBreeds && breed ? ` ${t("compare.breedNote", { breed: breed.name })}` : ""}
        </p>
        <SegmentedControl
          aria-label={t("compare.measure")}
          value={measure}
          onValueChange={(next) => setMeasure(measures.find((m) => m === next) ?? "mean")}
        >
          <SegmentedControlIndicator />
          <SegmentedControlItem value="mean">{t("compare.measure.mean")}</SegmentedControlItem>
          <SegmentedControlItem value="normal">{t("damage.normal")}</SegmentedControlItem>
          <SegmentedControlItem value="crit">{t("damage.critical")}</SegmentedControlItem>
        </SegmentedControl>
      </div>
      {failed ? (
        <p className="m-0 text-bad" role="alert">
          {t("compare.error")}
        </p>
      ) : !diff ? (
        <div className="skeleton h-40" />
      ) : (
        <DeltaTable
          first={t("compare.col.attack")}
          names={names}
          showHead
          groups={[
            {
              title: t("damage.weapon"),
              rows: diff.weapon ? [toRow(diff.weapon)] : [],
              empty: t("damage.noWeapon"),
            },
            {
              title: breed ? t("damage.spellsOf", { breed: breed.name }) : t("damage.spells"),
              rows: diff.classSpells.map(toRow),
              empty: breed ? t("damage.noSpells") : t("damage.pickClass"),
            },
            ...(diff.commonSpells.length > 0 ? [{ title: t("damage.common"), rows: diff.commonSpells.map(toRow) }] : []),
          ]}
        />
      )}
    </>
  )
}

// What a side says about itself under its field: class, level, items, and a
// warning when the stuff does not hold.
function SideNote({ meta, option, evaluation }: { meta: Meta; option: CompareOption | undefined; evaluation: Evaluation | undefined }) {
  const { t } = useI18n()
  if (!option) return <span className="compare-side-note" />
  const breed = meta.breeds.find((b) => b.id === option.breedId)?.name
  return (
    <span className="compare-side-note">
      <span className="truncate">
        {[
          breed,
          t("item.level", { level: option.input.level }),
          t("solver.applyItems", { count: Object.keys(option.input.items).length }),
        ]
          .filter(Boolean)
          .join(" · ")}
      </span>
      {evaluation && !evaluation.valid && (
        <span className="status" data-state="bad">
          <TriangleAlertIcon aria-hidden="true" />
          {t("build.invalid")}
        </span>
      )}
    </span>
  )
}

type Section = "stats" | "items" | "damage"

function CompareContent({ meta, options, initial }: Pick<Props, "meta" | "options" | "initial">) {
  const { t, number } = useI18n()
  const [fromKey, setFromKey] = React.useState<string | null>(initial.from)
  const [toKey, setToKey] = React.useState<string | null>(initial.to ?? null)
  const [section, setSection] = React.useState<Section>("stats")
  const from = options.find((option) => option.key === fromKey)
  const to = options.find((option) => option.key === toKey)
  const both = from !== undefined && to !== undefined

  // Both stuffs are read with the spells of one class, the first that has one.
  const breedId = from?.breedId ?? to?.breedId ?? null
  const fromEvaluation = useEvaluation(from?.input ?? noInput, from !== undefined)
  const toEvaluation = useEvaluation(to?.input ?? noInput, to !== undefined)
  const fromDamage = useDamage(from?.input ?? noInput, breedId, "", true, both)
  const toDamage = useDamage(to?.input ?? noInput, breedId, "", true, both)
  // While a newly picked stuff loads, the query still holds the one before it.
  const settled = <T,>(query: { data: T | undefined; isPlaceholderData: boolean }, wanted: boolean) =>
    wanted && !query.isPlaceholderData ? query.data : undefined
  const before = settled(fromEvaluation, from !== undefined)
  const after = settled(toEvaluation, to !== undefined)

  const groupsFor = (other: string | null): SearchGroup[] =>
    (["saved", "results"] as const).map((group) => ({
      label: t(group === "saved" ? "build.saved" : "compare.results"),
      options: options
        .filter((option) => option.group === group && option.key !== other)
        .map((option) => ({
          value: option.key,
          label: option.label,
          keywords: [fold(meta.breeds.find((b) => b.id === option.breedId)?.name ?? "")],
        })),
    }))

  const names = { from: from?.label ?? "", to: to?.label ?? "" }
  const byId = React.useMemo(() => statIndex(meta), [meta])
  const stats = before && after ? diffStats(meta, before, after, vitals) : null
  const items =
    before && after && from && to
      ? diffItems(meta, { mods: from.input.mods, evaluation: before }, { mods: to.input.mods, evaluation: after })
      : null
  const vitalRows = before && after ? vitals.flatMap((id) => (byId.has(id) ? [statDelta(byId.get(id)!, before, after)] : [])) : []
  const changedItems = items?.filter((row) => row.change !== "same").length ?? 0
  const character = from && to ? diffCharacter(meta, from.input, to.input) : null
  const identical =
    stats !== null &&
    character !== null &&
    stats.gains.length + stats.losses.length + changedItems + character.base.length + character.scrolls.length === 0 &&
    character.level === null
  const failed = (from !== undefined && fromEvaluation.isError) || (to !== undefined && toEvaluation.isError)

  let body: React.ReactNode
  if (!both) {
    body = (
      <div className="board compare-stage">
        <p>{options.length < 2 ? t("compare.onlyOne") : t("compare.pickHint")}</p>
      </div>
    )
  } else if (failed) {
    body = (
      <p className="m-0 text-bad" role="alert">
        {t("compare.error")}
      </p>
    )
  } else if (!before || !after || !stats || !items) {
    body = (
      <div className="grid gap-3" aria-busy="true">
        <div className="skeleton h-16" />
        <div className="skeleton h-64" />
      </div>
    )
  } else if (identical) {
    body = (
      <div className="board compare-stage">
        <p role="status">{t("compare.identical")}</p>
      </div>
    )
  } else {
    body = (
      <>
        <ul className="compare-vitals" aria-label={t("compare.vitals")}>
          {vitalRows.map((row) => (
            <li key={row.stat.id} className="compare-vital">
              <span className="label">
                <StatName stat={row.stat} />
              </span>
              <Delta value={row.delta} />
              <span className="change">
                <span className="sr-only">{t("compare.from")} </span>
                {number(row.from)}
                <span aria-hidden="true"> → </span>
                <span className="sr-only"> {t("compare.to")} </span>
                {number(row.to)}
              </span>
            </li>
          ))}
        </ul>
        <Tabs
          value={section}
          onValueChange={(next) => setSection(next === "items" || next === "damage" ? next : "stats")}
          className="compare-tabs"
        >
          <TabsList aria-label={t("compare.sections")} className="slot-tabs">
            <TabsTab value="stats">
              {t("build.stats")}{" "}
              <span className="compare-count">{stats.gains.length + stats.losses.length}</span>
            </TabsTab>
            <TabsTab value="items">
              {t("build.tab.items")}{" "}
              <span className="compare-count">{changedItems}</span>
            </TabsTab>
            <TabsTab value="damage">{t("build.view.damage")}</TabsTab>
          </TabsList>
          <TabsPanel value="stats" className="compare-panel">
            <StatsTab names={names} gains={stats.gains} losses={stats.losses} unchanged={stats.unchanged} />
          </TabsPanel>
          <TabsPanel value="items" className="compare-panel">
            <ItemsTab
              meta={meta}
              names={names}
              from={{ option: from, evaluation: before }}
              to={{ option: to, evaluation: after }}
              rows={items}
            />
          </TabsPanel>
          <TabsPanel value="damage" className="compare-panel">
            <DamageTab
              meta={meta}
              names={names}
              breedId={breedId}
              mixedBreeds={from.breedId !== null && to.breedId !== null && from.breedId !== to.breedId}
              from={settled(fromDamage, true)}
              to={settled(toDamage, true)}
              failed={fromDamage.isError || toDamage.isError}
            />
          </TabsPanel>
        </Tabs>
      </>
    )
  }

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="grid min-w-0 gap-1">
          <DialogTitle>{t("compare.title")}</DialogTitle>
          <DialogDescription>{t("compare.hint")}</DialogDescription>
        </div>
        <DialogClose className="icon-btn" aria-label={t("about.close")}>
          <XIcon aria-hidden="true" />
        </DialogClose>
      </div>

      <div className="compare-sides">
        <div className="compare-side" data-side="from">
          <span className="field-label">{t("compare.fromLabel")}</span>
          <SearchSelect
            label={t("compare.fromLabel")}
            groups={groupsFor(toKey)}
            value={from?.key ?? null}
            placeholder={t("compare.pick")}
            emptyText={t("build.noMatch")}
            showLabel={t("compare.show")}
            onChange={setFromKey}
          />
          <SideNote meta={meta} option={from} evaluation={before} />
        </div>
        <button
          type="button"
          className="icon-btn compare-swap"
          aria-label={t("compare.swap")}
          title={t("compare.swap")}
          disabled={!both}
          onClick={() => {
            setFromKey(toKey)
            setToKey(fromKey)
          }}
        >
          <ArrowLeftRightIcon aria-hidden="true" />
        </button>
        <div className="compare-side" data-side="to">
          <span className="field-label">{t("compare.toLabel")}</span>
          <SearchSelect
            label={t("compare.toLabel")}
            groups={groupsFor(fromKey)}
            value={to?.key ?? null}
            placeholder={t("compare.pick")}
            emptyText={t("build.noMatch")}
            showLabel={t("compare.show")}
            onChange={setToKey}
          />
          <SideNote meta={meta} option={to} evaluation={after} />
        </div>
      </div>

      <div className="compare-scroll">{body}</div>
    </>
  )
}
