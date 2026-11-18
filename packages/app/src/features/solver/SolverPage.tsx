import { Button } from "@ui-sdk/react/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import { Switch, SwitchThumb } from "@ui-sdk/react/switch"
import { CheckIcon, ChevronDownIcon, ChevronUpIcon, PlusIcon, SparklesIcon, XIcon } from "lucide-react"
import * as React from "react"
import { ApiError } from "../../api/client"
import { RESULTS_PAGE, useItem, useSolver } from "../../api/queries"
import type {
  DamageGoal,
  DamageMeasure,
  Item,
  ItemMods,
  MaximizeEntry,
  Meta,
  Solution,
  SolverRequest,
  Stat,
  StatFilter,
  WeaponElement,
} from "../../api/types"
import { useI18n } from "../../i18n"
import { useApi } from "../../platform"
import { useAddToBuild } from "../../lib/useAddToBuild"
import { useBreakpoint } from "../../lib/useBreakpoint"
import { maximizedAttack, weaponElements } from "../../lib/maximize"
import { AP_STAT, HP_STAT, MP_STAT, RANGE_STAT, statLabel, toneOf } from "../../lib/stats"
import { newBuild, useActiveBuild, useStore } from "../../store"
import { nextBuildName } from "../build/BuildTabs"
import { CompareDialog, savedOption, type CompareOption } from "../compare/CompareDialog"
import { ElementGoalOptions } from "./ElementGoal"
import { defaultThreshold, FilterRow } from "../../ui/FilterRow"
import { ItemCell } from "../../ui/ItemCell"
import { ItemHover } from "../../ui/ItemHover"
import { pickBuild } from "../../ui/BuildPicker"
import { ItemSearch } from "../../ui/ItemSearch"
import { SearchSelect } from "../../ui/SearchSelect"
import { NumberField } from "../../ui/NumberField"
import { SidePanels } from "../../ui/SidePanels"
import { StatSelect } from "../../ui/StatSelect"
import { toaster } from "../../ui/toaster"

// The search returns up to 20 stuffs, shown five at a time.
const MAX_RESULTS = 20

function entryKey(entry: MaximizeEntry): string {
  if (entry.kind === "stat") return `stat-${entry.statId}`
  return entry.kind === "element" ? "element" : `${entry.kind}-${entry.id}`
}

const WISDOM = 12
const STRENGTH = 10

const example = {
  constraints: [
    { statId: AP_STAT, op: "gte", value: 11 },
    { statId: MP_STAT, op: "gte", value: 6 },
    { statId: WISDOM, op: "gte", value: 300 },
  ] satisfies StatFilter[],
  maximize: [{ kind: "stat", statId: STRENGTH }] satisfies MaximizeEntry[],
}

// An item the search must wear, as a removable row.
function RequiredItem({ id, unplaced, onRemove }: { id: number; unplaced: boolean; onRemove(): void }) {
  const { t, colon } = useI18n()
  const item = useItem(id).data
  return (
    <li className="flex items-center gap-2">
      {item ? <ItemCell item={item} size="2rem" /> : <span className="skeleton size-8" />}
      <span className="grid min-w-0 flex-1">
        <span className="truncate font-semibold">{item?.name ?? t("common.loading")}</span>
        {item && (
          <span className="truncate text-[0.86rem] text-ink-3">
            {unplaced ? <span className="text-bad">{t("solver.requiredUnplaced")}</span> : item.typeName}
          </span>
        )}
      </span>
      <button
        type="button"
        className="icon-btn"
        aria-label={`${t("common.remove")}${colon}${item?.name ?? ""}`}
        onClick={onRemove}
      >
        <XIcon aria-hidden="true" />
      </button>
    </li>
  )
}

function ItemName({ id }: { id: number }) {
  return <>{useItem(id).data?.name ?? "…"}</>
}

// The weapon or spell a damage goal is about.
function GoalName({ meta, goal }: { meta: Meta; goal: { kind: "weapon" | "spell"; id: number } }) {
  if (goal.kind === "weapon") return <ItemName id={goal.id} />
  return <>{meta.spells.find((spell) => spell.id === goal.id)?.name ?? "…"}</>
}

// How an attack is read: which hit, and for a weapon its elemental forgemagic.
function AttackOptions({
  attack,
  onChange,
}: {
  attack: { kind: "weapon" | "spell"; measure: DamageMeasure; element?: WeaponElement }
  onChange(patch: { measure?: DamageMeasure; element?: WeaponElement | undefined }): void
}) {
  const { t } = useI18n()
  return (
    <>
      <Select
        value={attack.measure}
        onValueChange={(next) => onChange({ measure: next === "crit" || next === "mean" ? next : "normal" })}
      >
        <SelectTrigger aria-label={t("solver.damage.hit")} className="min-w-28 flex-1 justify-between">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(["normal", "crit", "mean"] as const).map((measure) => (
            <SelectItem key={measure} value={measure} label={t(`solver.damage.measure.${measure}`)}>
              {t(`solver.damage.measure.${measure}`)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {attack.kind === "weapon" && (
        <Select
          value={attack.element ?? "none"}
          onValueChange={(next) => onChange({ element: next && next !== "none" ? (next as WeaponElement) : undefined })}
        >
          <SelectTrigger aria-label={t("forge.weaponElement")} className="min-w-28 flex-1 justify-between">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none" label={t("forge.weaponElement.none")}>
              {t("forge.weaponElement.none")}
            </SelectItem>
            {weaponElements.map((element) => (
              <SelectItem
                key={element}
                value={element}
                label={t("forge.weaponElement.to", { element: t(`element.${element}`) })}
              >
                <span className="pip" data-tone={element} />
                {t("forge.weaponElement.to", { element: t(`element.${element}`) })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </>
  )
}

// One damage condition: which attack, how it is read, and the value to reach.
function GoalRow({
  meta,
  goal,
  onChange,
  onRemove,
}: {
  meta: Meta
  goal: DamageGoal
  onChange(goal: DamageGoal): void
  onRemove(): void
}) {
  const { t } = useI18n()
  return (
    <li className="grid gap-1 rounded-md border border-line-strong py-1.5 pr-1 pl-2.5">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-semibold">
          <GoalName meta={meta} goal={goal} />
          <span className="font-normal text-ink-3"> · {t(`solver.damage.${goal.kind}`)}</span>
        </span>
        <button type="button" className="icon-btn size-6!" aria-label={t("common.remove")} onClick={onRemove}>
          <XIcon aria-hidden="true" />
        </button>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <AttackOptions attack={goal} onChange={(patch) => onChange({ ...goal, ...patch })} />
        <span className="flex items-center gap-2">
          <span className="text-ink-3">≥</span>
          <NumberField
            label={t("filter.value")}
            value={goal.value}
            min={1}
            max={99999}
            onChange={(value) => onChange({ ...goal, value })}
          />
        </span>
      </div>
    </li>
  )
}

function ResultRow({
  meta,
  rank,
  solution,
  request,
  onApply,
  onOpen,
  onCompare,
  onAdd,
  onExclude,
}: {
  meta: Meta
  rank: number
  solution: Solution
  request: SolverRequest
  onApply(): void
  onOpen(): void
  onCompare(): void
  onAdd(item: Item, from: { position: string; mods?: ItemMods }): void
  onExclude(item: Item): void
}) {
  const { t, number, primaryName, colon } = useI18n()
  const byId = new Map(meta.stats.map((s) => [s.id, s]))
  const evaluation = solution.evaluation
  const value = (id: number) => evaluation.stats[String(id)] ?? 0

  // Constraints first, then what was maximized, then the vitals not shown yet.
  const shown = new Set(request.constraints.map((c) => c.statId))
  const extras: Stat[] = []
  const maximized = request.maximize.filter((m) => !m.damage).map((m) => m.statId)
  // Every attack measured on the stuff: the conditions, then the maximized ones.
  const attacks = [
    ...request.damage.map((goal) => ({ goal, required: true })),
    ...request.maximize.flatMap((m) => (m.damage ? [{ goal: m.damage, required: false }] : [])),
  ]
  for (const id of [...maximized, HP_STAT, AP_STAT, MP_STAT, RANGE_STAT]) {
    const stat = byId.get(id)
    if (stat && !shown.has(id)) {
      shown.add(id)
      extras.push(stat)
    }
  }
  const sets = evaluation.sets.filter((s) => s.count > 1)

  const pointNotes = meta.primaryStats
    .filter(({ key }) => solution.character.base[key] > 0)
    .map(({ key }) => `${primaryName(key)} ${number(solution.character.base[key])}`)

  // What the solver forged, in a line: who holds each exo, then rune and dracorune counts.
  const forgeNotes: string[] = []
  const runeCounts = new Map<string, number>()
  const dracoCounts = new Map<string, number>()
  for (const [position, mods] of Object.entries(solution.mods ?? {})) {
    const item = evaluation.items[position]
    for (const exo of mods.exos ?? []) {
      forgeNotes.push(`${t("forge.origin.exo")} ${byId.get(exo.statId)?.label ?? ""}${colon}${item?.name ?? ""}`)
    }
    const rune = meta.forge.runes.find((r) => r.id === mods.transcendence)
    if (rune) runeCounts.set(rune.name, (runeCounts.get(rune.name) ?? 0) + 1)
    if (mods.draco) {
      const name = `${t("forge.draco")} ${t(`draco.${mods.draco.rune}`)}`
      dracoCounts.set(name, (dracoCounts.get(name) ?? 0) + 1)
    }
  }
  for (const [name, count] of [...runeCounts, ...dracoCounts]) forgeNotes.push(`${name} × ${count}`)

  return (
    <li className="grid gap-3 border-b border-line px-5 py-4">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span className="text-[1.15rem] font-extrabold text-ink-3">{rank}</span>
        <div className="flex flex-1 flex-wrap gap-1.5">
          {request.constraints.map((constraint, index) => {
            const stat = byId.get(constraint.statId)
            const met = !solution.unmet.includes(index)
            return (
              <span
                key={constraint.statId}
                className="goal"
                data-state={met ? "ok" : "bad"}
                title={`${met ? t("solver.met") : t("solver.unmet")}${colon}${statLabel(stat)} ${constraint.op === "gte" ? "≥" : "≤"} ${constraint.value}`}
              >
                {met ? <CheckIcon aria-hidden="true" /> : <XIcon aria-hidden="true" />}
                <span className="text-ink-2">{stat?.label}</span>
                <strong>
                  {number(value(constraint.statId))}
                  {stat?.unit}
                </strong>
                <span className="sr-only">{met ? t("solver.met") : t("solver.unmet")}</span>
              </span>
            )
          })}
          {attacks.map(({ goal, required }, index) => {
            const met = !solution.unmetDamage.includes(index)
            const measure = t(`solver.damage.measure.${goal.measure}`)
            if (goal.kind === "elements") {
              const parts = solution.damageParts?.[index] ?? []
              return (goal.parts ?? []).map((part, at) => {
                const share = at > 0 && parts[0] ? Math.round(((parts[at] ?? 0) * 100) / parts[0]) : null
                return (
                  <span key={`${index}-${part.element}`} className="goal">
                    <span className="pip" data-tone={part.element} />
                    <span className="text-ink-2">
                      {t("solver.elementDamage", { element: t(`element.${part.element}`) })} · {measure}
                    </span>
                    <strong>{number(parts[at] ?? 0)}</strong>
                    {share !== null && (
                      <span className="text-ink-3" title={t("solver.elements.ratio", { percent: share })}>
                        <span aria-hidden="true">{t("solver.elements.percent", { percent: share })}</span>
                        <span className="sr-only">{t("solver.elements.ratio", { percent: share })}</span>
                      </span>
                    )}
                  </span>
                )
              })
            }
            return (
              <span
                key={`${index}-${goal.kind}-${goal.id}-${goal.measure}`}
                className="goal"
                data-state={!required ? undefined : met ? "ok" : "bad"}
              >
                {required && (met ? <CheckIcon aria-hidden="true" /> : <XIcon aria-hidden="true" />)}
                <span className="text-ink-2">
                  {goal.kind === "element" ? (
                    t("solver.elementDamage", { element: t(`element.${goal.element ?? solution.element ?? "earth"}`) })
                  ) : (
                    <GoalName meta={meta} goal={{ kind: goal.kind, id: goal.id }} />
                  )}{" "}
                  · {measure}
                </span>
                <strong>{number(solution.damage[index] ?? 0)}</strong>
                {required && <span className="sr-only">{met ? t("solver.met") : t("solver.unmet")}</span>}
              </span>
            )
          })}
          {extras.map((stat) => (
            <span key={stat.id} className="inline-flex h-[1.7rem] items-center gap-1.5 px-1 text-[0.93rem]">
              <span className="pip" data-tone={toneOf(stat.key)} />
              <span className="text-ink-2">{stat.id === HP_STAT ? t("stat.hp") : stat.label}</span>
              <strong>
                {number(value(stat.id))}
                {stat.unit}
              </strong>
            </span>
          ))}
        </div>
        <div className="flex flex-wrap gap-1.5">
          <Button onClick={onApply}>{t("solver.apply")}</Button>
          <Button onClick={onOpen}>{t("solver.openTab")}</Button>
          <Button onClick={onCompare}>{t("compare.open")}</Button>
        </div>
      </div>

      <div className="flex flex-wrap gap-1">
        {meta.positions.map((position) => {
          const item = evaluation.items[position.key]
          if (!item) return null
          return (
            <ItemHover
              key={position.key}
              item={item}
              stats={byId}
              forged={evaluation.forge[position.key]}
              actions={[
                {
                  label: t("item.add"),
                  primary: true,
                  run: () => onAdd(item, { position: position.key, mods: solution.mods?.[position.key] }),
                },
                { label: t("solver.exclude"), run: () => onExclude(item) },
              ]}
            >
              <span role="img" aria-label={item.name} tabIndex={0} className="relative rounded-md">
                <ItemCell item={item} size="5.5rem" />
                {solution.mods?.[position.key] && <SparklesIcon className="slot-forged" aria-hidden="true" />}
              </span>
            </ItemHover>
          )
        })}
      </div>

      {sets.length > 0 && (
        <p className="m-0 text-ink-2">
          {sets.map((set) => `${set.name} ${set.count}/${set.size}`).join(" · ")}
        </p>
      )}

      {request.autoPoints && pointNotes.length > 0 && (
        <p className="m-0 text-ink-2">
          <strong className="text-ink">{t("solver.points")}</strong>{colon}{pointNotes.join(" · ")}
        </p>
      )}

      {forgeNotes.length > 0 && (
        <p className="m-0 text-ink-2">
          <strong className="text-ink">{t("forge.title")}</strong>{colon}{forgeNotes.join(" · ")}
        </p>
      )}
    </li>
  )
}

export function SolverPage({ meta }: { meta: Meta }) {
  const { t, number, colon } = useI18n()
  const build = useActiveBuild()
  const draft = useStore((s) => s.solver)
  const updateSolver = useStore((s) => s.updateSolver)
  const builds = useStore((s) => s.builds)
  const addBuild = useStore((s) => s.addBuild)
  const targetBuild = useStore((s) => s.targetBuild)
  const updateBuild = useStore((s) => s.updateBuild)
  const solver = useSolver()
  const addToBuild = useAddToBuild(meta)
  const breakpoint = useBreakpoint()
  const [lastRequest, setLastRequest] = React.useState<SolverRequest | null>(null)
  // The pages of stuffs read after the first one, which comes with the search.
  const [morePages, setMorePages] = React.useState<Solution[]>([])
  const [loadingMore, setLoadingMore] = React.useState(false)
  // The result being compared, by its place in the list.
  const [comparing, setComparing] = React.useState<number | null>(null)
  const api = useApi()
  const resultsRef = React.useRef<HTMLElement>(null)

  const byId = new Map(meta.stats.map((s) => [s.id, s]))
  const usedStats = draft.constraints.map((c) => c.statId)
  const firstFree = meta.stats.find((s) => !usedStats.includes(s.id) && s.id !== HP_STAT)
  const equippedCount = Object.keys(build.items).length
  const canRun = draft.constraints.length > 0 || draft.maximize.length > 0 || draft.damage.length > 0
  const spellOptions = React.useCallback(
    (taken: Set<number>) => {
      const options = (breedId: number) =>
        meta.spells
          .filter((spell) => spell.breedId === breedId && spell.damage && !taken.has(spell.id))
          .map((spell) => ({ value: String(spell.id), label: spell.name }))
      const breed = meta.breeds.find((b) => b.id === build.breedId)
      return [
        ...(breed ? [{ label: breed.name, options: options(breed.id) }] : []),
        { label: t("damage.common"), options: options(0) },
      ]
    },
    [meta, build.breedId, t]
  )
  const breedOfBuild = meta.breeds.find((b) => b.id === build.breedId)

  // A forbidden item cannot also be required.
  function exclude(item: Item) {
    const current = useStore.getState().solver
    if (current.excluded.includes(item.id)) return
    updateSolver({
      excluded: [...current.excluded, item.id],
      required: current.required.filter((id) => id !== item.id),
    })
  }
  const maximizeSpellGroups = React.useMemo(
    () => spellOptions(new Set(draft.maximize.flatMap((entry) => (entry.kind === "spell" ? [entry.id] : [])))),
    [spellOptions, draft.maximize]
  )
  const spellGroups = React.useMemo(() => {
    const taken = new Set(draft.damage.filter((g) => g.kind === "spell").map((g) => g.id))
    const options = (breedId: number) =>
      meta.spells
        .filter((spell) => spell.breedId === breedId && spell.damage && !taken.has(spell.id))
        .map((spell) => ({ value: String(spell.id), label: spell.name }))
    const breed = meta.breeds.find((b) => b.id === build.breedId)
    return [
      ...(breed ? [{ label: breed.name, options: options(breed.id) }] : []),
      { label: t("damage.common"), options: options(0) },
    ]
  }, [meta, build.breedId, draft.damage, t])

  function run() {
    const request: SolverRequest = {
      character: {
        level: build.level,
        base: build.base,
        scrolls: draft.fullScrolls
          ? { vitality: 100, wisdom: 100, strength: 100, intelligence: 100, chance: 100, agility: 100 }
          : build.scrolls,
      },
      autoPoints: draft.autoPoints,
      constraints: draft.constraints,
      maximize: draft.maximize.map((entry) =>
        entry.kind === "stat" ? { statId: entry.statId, weight: 1 } : { statId: 0, weight: 1, damage: maximizedAttack(entry) }
      ),
      breedId: build.breedId ?? 0,
      damage: draft.damage,
      locked: draft.keepEquipped ? build.items : {},
      required: draft.required,
      excluded: draft.excluded,
      maxResults: MAX_RESULTS,
      forge: {
        exos: [
          ...(draft.exoAp ? [AP_STAT] : []),
          ...(draft.exoMp ? [MP_STAT] : []),
          ...(draft.exoRange ? [RANGE_STAT] : []),
        ],
        transcendence: draft.transcendence,
        draco: draft.draco,
        raiseCritical: draft.raiseCritical,
      },
    }
    setLastRequest(request)
    setMorePages([])
    solver.mutate(request, {
      // On narrow layouts the results sit below the form: bring them into view.
      onSuccess: () => {
        if (window.matchMedia("(max-width: 63.99rem)").matches) {
          requestAnimationFrame(() => {
            resultsRef.current?.scrollIntoView({ block: "start" })
            resultsRef.current?.querySelector("h2")?.focus()
          })
        }
      },
    })
  }

  // The stuff picked takes the points and scrolls the search counted, so its
  // stats are the ones shown in the result.
  async function apply(solution: Solution) {
    const target = await pickBuild({
      title: t("solver.applyTitle"),
      hint: t("solver.applyHint"),
      confirmLabel: t("solver.applyConfirm"),
    })
    if (!target) return
    const { replaceGear, selectBuild } = useStore.getState()
    replaceGear(target.id, {
      items: solution.items,
      mods: solution.mods ?? {},
      base: solution.character.base,
      scrolls: solution.character.scrolls,
    })
    toaster(t("solver.applied", { name: target.name }))
    selectBuild(target.id)
  }

  function open(solution: Solution) {
    const created = newBuild(
      nextBuildName(
        (n) => t("build.numbered", { n }),
        builds.map((b) => b.name)
      )
    )
    created.breedId = build.breedId
    created.level = build.level
    created.base = solution.character.base
    created.scrolls = solution.character.scrolls
    created.items = solution.items
    created.mods = solution.mods ?? {}
    addBuild(created)
    toaster(t("solver.opened", { name: created.name }))
  }

  const response = solver.data
  const list = [...(response?.solutions ?? []), ...morePages]

  // A result is compared with the stuff the search targets, then with any
  // saved stuff or any other result the player picks.
  const compareOptions: CompareOption[] = [
    ...builds.map((b) => savedOption(b, t("build.unnamed"))),
    ...list.map(
      (solution, index): CompareOption => ({
        key: `result-${index}`,
        label: t("compare.result", { n: index + 1 }),
        group: "results",
        breedId: lastRequest?.breedId || null,
        input: {
          level: solution.character.level,
          base: solution.character.base,
          scrolls: solution.character.scrolls,
          items: solution.items,
          mods: solution.mods ?? {},
        },
      })
    ),
  ]

  async function loadMore() {
    if (!response || loadingMore) return
    setLoadingMore(true)
    try {
      const page = await api.results(response.id, list.length, RESULTS_PAGE)
      setMorePages((pages) => [...pages, ...page.solutions])
    } catch (error) {
      toaster(error instanceof ApiError && error.code === "results_expired" ? t("solver.expired") : t("solver.error"))
    } finally {
      setLoadingMore(false)
    }
  }

  const form = (
      <form
        className="grid content-start gap-6 bg-panel p-5 max-lg:border-b max-lg:border-line lg:h-full lg:overflow-y-auto"
        onSubmit={(event) => {
          event.preventDefault()
          if (canRun) run()
        }}
      >
        <div>
          <h1 className="m-0 text-xl font-bold">{t("solver.title")}</h1>
          <p className="m-0 mt-1.5 max-w-[60ch] text-ink-2">{t("solver.intro")}</p>
        </div>

        <fieldset className="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend className="section-title mb-2 p-0">{t("solver.constraints")}</legend>
          {draft.constraints.map((constraint, index) => (
            <FilterRow
              key={constraint.statId}
              meta={meta}
              filter={constraint}
              min={0}
              exclude={usedStats}
              onChange={(next) =>
                updateSolver({ constraints: draft.constraints.map((c, i) => (i === index ? next : c)) })
              }
              onRemove={() => updateSolver({ constraints: draft.constraints.filter((_, i) => i !== index) })}
            />
          ))}
          <div>
            <Button
              disabled={!firstFree}
              onClick={() => {
                if (!firstFree) return
                updateSolver({
                  constraints: [
                    ...draft.constraints,
                    { statId: firstFree.id, op: "gte", value: defaultThreshold(firstFree.key) },
                  ],
                })
              }}
            >
              <PlusIcon aria-hidden="true" className="size-4" />
              {t("solver.addConstraint")}
            </Button>
          </div>
        </fieldset>

        <fieldset className="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend className="section-title mb-1 p-0">{t("solver.damage")}</legend>
          <p className="m-0 text-ink-2">{t("solver.damageHint")}</p>
          {draft.damage.length > 0 && (
            <ul className="m-0 grid list-none gap-1.5 p-0">
              {draft.damage.map((goal, index) => (
                <GoalRow
                  key={`${goal.kind}-${goal.id}`}
                  meta={meta}
                  goal={goal}
                  onChange={(next) => updateSolver({ damage: draft.damage.map((g, i) => (i === index ? next : g)) })}
                  onRemove={() => updateSolver({ damage: draft.damage.filter((_, i) => i !== index) })}
                />
              ))}
            </ul>
          )}
          <div className="grid gap-1.5 sm:grid-cols-2">
            <ItemSearch
              label={t("solver.damage.addWeapon")}
              placeholder={t("solver.damage.addWeapon")}
              slots={["weapon"]}
              exclude={draft.damage.filter((g) => g.kind === "weapon").map((g) => g.id)}
              onPick={(item) =>
                updateSolver({ damage: [...draft.damage, { kind: "weapon", id: item.id, measure: "normal", value: 1000 }] })
              }
            />
            <SearchSelect
              label={t("solver.damage.addSpell")}
              groups={spellGroups}
              value={null}
              placeholder={t("solver.damage.addSpell")}
              emptyText={t("solver.damage.noSpell")}
              showLabel={t("solver.damage.addSpell")}
              onChange={(id) =>
                updateSolver({ damage: [...draft.damage, { kind: "spell", id: Number(id), measure: "normal", value: 500 }] })
              }
            />
          </div>
        </fieldset>

        <fieldset className="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend className="section-title mb-1 p-0">{t("solver.maximize")}</legend>
          <p className="m-0 text-ink-2">{t("solver.maximizeHint")}</p>
          {draft.maximize.length > 0 && (
            <ol className="m-0 grid list-none gap-1 p-0">
              {draft.maximize.map((entry, index) => {
                const stat = entry.kind === "stat" ? byId.get(entry.statId) : undefined
                const key = entryKey(entry)
                const name = entry.kind === "stat" ? statLabel(stat) : undefined
                const move = (to: number) => {
                  const next = draft.maximize.filter((_, i) => i !== index)
                  next.splice(to, 0, entry)
                  updateSolver({ maximize: next })
                }
                const replace = (next: MaximizeEntry) =>
                  updateSolver({ maximize: draft.maximize.map((e, i) => (i === index ? next : e)) })
                return (
                  <li key={key} className="grid min-w-0 gap-1 rounded-md border border-line-strong py-1 pr-0.5 pl-2">
                    <div className="flex min-h-[1.9rem] items-center gap-2">
                      <span className="w-4 flex-none text-center font-extrabold text-ink-3">{index + 1}</span>
                      {entry.kind === "stat" ? (
                        <>
                          <span className="pip" data-tone={toneOf(stat?.key)} />
                          <span className="min-w-0 flex-1 truncate font-semibold">{name}</span>
                        </>
                      ) : entry.kind === "element" ? (
                        <span className="min-w-0 flex-1 truncate font-semibold">
                          {t("solver.finalDamage")}
                          <span className="font-normal text-ink-3">
                            {" · "}
                            {breedOfBuild ? t("solver.finalDamage.of", { breed: breedOfBuild.name }) : t("solver.finalDamage.reference")}
                          </span>
                        </span>
                      ) : (
                        <span className="min-w-0 flex-1 truncate font-semibold">
                          <GoalName meta={meta} goal={entry} />
                          <span className="font-normal text-ink-3"> · {t(`solver.damage.${entry.kind}`)}</span>
                        </span>
                      )}
                      <button
                        type="button"
                        className="icon-btn size-6!"
                        aria-label={`${t("solver.moveUp")}${colon}${name ?? index + 1}`}
                        disabled={index === 0}
                        onClick={() => move(index - 1)}
                      >
                        <ChevronUpIcon aria-hidden="true" />
                      </button>
                      <button
                        type="button"
                        className="icon-btn size-6!"
                        aria-label={`${t("solver.moveDown")}${colon}${name ?? index + 1}`}
                        disabled={index === draft.maximize.length - 1}
                        onClick={() => move(index + 1)}
                      >
                        <ChevronDownIcon aria-hidden="true" />
                      </button>
                      <button
                        type="button"
                        className="icon-btn size-6!"
                        aria-label={`${t("common.remove")}${colon}${name ?? index + 1}`}
                        onClick={() => updateSolver({ maximize: draft.maximize.filter((_, i) => i !== index) })}
                      >
                        <XIcon aria-hidden="true" />
                      </button>
                    </div>
                    {(entry.kind === "weapon" || entry.kind === "spell") && (
                      <div className="flex flex-wrap items-center gap-2 pr-1.5 pb-1">
                        <AttackOptions attack={entry} onChange={(patch) => replace({ ...entry, ...patch })} />
                      </div>
                    )}
                    {entry.kind === "element" && <ElementGoalOptions entry={entry} onChange={replace} />}
                  </li>
                )
              })}
            </ol>
          )}
          {!draft.maximize.some((entry) => entry.kind === "element") && (
            <div>
              <Button
                onClick={() =>
                  updateSolver({ maximize: [{ kind: "element", element: "auto", measure: "mean" }, ...draft.maximize] })
                }
              >
                <PlusIcon aria-hidden="true" className="size-4" />
                {t("solver.finalDamage.add")}
              </Button>
            </div>
          )}
          <StatSelect
            meta={meta}
            label={t("solver.addMaximize")}
            value={null}
            placeholder={t("solver.addMaximize")}
            exclude={draft.maximize.flatMap((entry) => (entry.kind === "stat" ? [entry.statId] : []))}
            onChange={(statId) => updateSolver({ maximize: [...draft.maximize, { kind: "stat", statId }] })}
          />
          <div className="grid gap-1.5 sm:grid-cols-2">
            <ItemSearch
              label={t("solver.maximizeWeapon")}
              placeholder={t("solver.damage.addWeapon")}
              slots={["weapon"]}
              exclude={draft.maximize.flatMap((entry) => (entry.kind === "weapon" ? [entry.id] : []))}
              onPick={(item) =>
                updateSolver({ maximize: [...draft.maximize, { kind: "weapon", id: item.id, measure: "mean" }] })
              }
            />
            <SearchSelect
              label={t("solver.maximizeSpell")}
              groups={maximizeSpellGroups}
              value={null}
              placeholder={t("solver.damage.addSpell")}
              emptyText={t("solver.damage.noSpell")}
              showLabel={t("solver.maximizeSpell")}
              onChange={(id) =>
                updateSolver({ maximize: [...draft.maximize, { kind: "spell", id: Number(id), measure: "mean" }] })
              }
            />
          </div>
        </fieldset>

        <fieldset className="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend className="section-title mb-1 p-0">{t("solver.required")}</legend>
          <p className="m-0 text-ink-2">{t("solver.requiredHint")}</p>
          {draft.required.length > 0 && (
            <ul className="m-0 grid list-none gap-1.5 p-0">
              {draft.required.map((id) => (
                <RequiredItem
                  key={id}
                  id={id}
                  unplaced={solver.data?.unplaced.includes(id) ?? false}
                  onRemove={() => updateSolver({ required: draft.required.filter((other) => other !== id) })}
                />
              ))}
            </ul>
          )}
          <div className="flex">
            <ItemSearch
              label={t("solver.addRequired")}
              placeholder={t("solver.addRequired")}
              exclude={draft.required}
              onPick={(item) =>
                updateSolver({
                  required: [...draft.required, item.id],
                  excluded: draft.excluded.filter((id) => id !== item.id),
                })
              }
            />
          </div>
        </fieldset>

        <fieldset className="m-0 grid min-w-0 gap-2 border-0 p-0">
          <legend className="section-title mb-1 p-0">{t("solver.excluded")}</legend>
          <p className="m-0 text-ink-2">{t("solver.excludedHint")}</p>
          {draft.excluded.length > 0 && (
            <ul className="m-0 grid list-none gap-1.5 p-0">
              {draft.excluded.map((id) => (
                <RequiredItem
                  key={id}
                  id={id}
                  unplaced={false}
                  onRemove={() => updateSolver({ excluded: draft.excluded.filter((other) => other !== id) })}
                />
              ))}
            </ul>
          )}
          <div className="flex">
            <ItemSearch
              label={t("solver.addExcluded")}
              placeholder={t("solver.addExcluded")}
              exclude={draft.excluded}
              onPick={exclude}
            />
          </div>
        </fieldset>

        <div className="grid gap-3">
          <div className="grid gap-1.5">
            <h2 className="section-title">{t("solver.target")}</h2>
            <Select value={build.id} onValueChange={(id) => typeof id === "string" && targetBuild(id)}>
              <SelectTrigger aria-label={t("solver.target")} className="justify-between">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {builds.map((b) => (
                  <SelectItem key={b.id} value={b.id} label={b.name}>
                    {b.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="m-0 text-ink-2">{t("solver.targetHint")}</p>
          </div>
          <div className="flex items-center justify-between gap-3">
            <span>
              <span className="block font-semibold">{t("solver.level")}</span>
              <span className="text-ink-2">{t("solver.levelHint")}</span>
            </span>
            <NumberField
              label={t("solver.level")}
              value={build.level}
              min={1}
              max={meta.maxLevel}
              onChange={(level) => updateBuild(build.id, { level })}
            />
          </div>
          <label className="flex cursor-pointer items-start gap-3">
            <Switch checked={draft.autoPoints} onCheckedChange={(checked) => updateSolver({ autoPoints: checked })}>
              <SwitchThumb />
            </Switch>
            <span>
              <span className="block font-semibold">{t("solver.autoPoints")}</span>
              <span className="text-ink-2">
                {draft.autoPoints ? t("solver.autoPointsHint", { total: (build.level - 1) * 5 }) : t("solver.manualPointsHint")}
              </span>
            </span>
          </label>
          <label className="flex cursor-pointer items-start gap-3">
            <Switch checked={draft.fullScrolls} onCheckedChange={(checked) => updateSolver({ fullScrolls: checked })}>
              <SwitchThumb />
            </Switch>
            <span>
              <span className="block font-semibold">{t("solver.fullScrolls")}</span>
              <span className="text-ink-2">
                {draft.fullScrolls ? t("solver.fullScrollsHint") : t("solver.manualScrollsHint")}
              </span>
            </span>
          </label>
          <label className="flex cursor-pointer items-start gap-3">
            <Switch
              checked={draft.keepEquipped}
              disabled={equippedCount === 0}
              onCheckedChange={(checked) => updateSolver({ keepEquipped: checked })}
            >
              <SwitchThumb />
            </Switch>
            <span>
              <span className="block font-semibold">{t("solver.keep")}</span>
              <span className="text-ink-2">{t("solver.keepHint", { count: equippedCount })}</span>
            </span>
          </label>
        </div>

        <fieldset className="m-0 grid min-w-0 gap-2.5 border-0 p-0">
          <legend className="section-title mb-1 p-0">{t("solver.forge")}</legend>
          <p className="m-0 text-ink-2">{t("solver.forgeHint")}</p>
          {(
            [
              ["exoAp", "solver.forge.exoAp"],
              ["exoMp", "solver.forge.exoMp"],
              ["exoRange", "solver.forge.exoRange"],
              ["transcendence", "solver.forge.transcendence"],
              ["draco", "solver.forge.draco"],
              ["raiseCritical", "solver.forge.raiseCritical"],
            ] as const
          ).map(([key, label]) => (
            <label key={key} className="flex cursor-pointer items-center gap-3">
              <Switch checked={draft[key]} onCheckedChange={(checked) => updateSolver({ [key]: checked })}>
                <SwitchThumb />
              </Switch>
              <span className="font-semibold">{t(label)}</span>
            </label>
          ))}
        </fieldset>

        <div className="sticky bottom-0 -mx-5 -mb-5 grid gap-2 border-t border-line bg-panel px-5 py-3">
          <Button type="submit" data-variant="solid" disabled={!canRun || solver.isPending} className="h-10!">
            {solver.isPending ? t("solver.running") : t("solver.run")}
          </Button>
          {!canRun && <p className="m-0 text-ink-2">{t("solver.needCriteria")}</p>}
        </div>
      </form>
  )

  const results = (
      <section ref={resultsRef} className="min-w-0 bg-panel lg:h-full lg:overflow-y-auto" aria-live="polite">
        {solver.isError ? (
          <p className="m-0 p-6 text-bad" role="alert">
            {t("solver.error")}
          </p>
        ) : solver.isPending ? (
          <div className="grid gap-3 p-5">
            {Array.from({ length: 5 }, (_, i) => (
              <div key={i} className="skeleton h-24" />
            ))}
          </div>
        ) : response && lastRequest ? (
          <>
            <header className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-line bg-panel-2 px-5 py-3">
              {response.feasible > 0 ? (
                <h2 className="m-0 text-base font-bold" tabIndex={-1}>
                  {t("solver.results", { count: response.feasible })}
                </h2>
              ) : (
                <div>
                  <h2 className="m-0 text-base font-bold text-bad" tabIndex={-1}>
                    {t("solver.none.title")}
                  </h2>
                  <p className="m-0 text-ink-2">{t("solver.none.body")}</p>
                </div>
              )}
              <span className="text-ink-2">
                {t("solver.summary", { explored: number(response.explored), ms: number(response.elapsedMs) })}
              </span>
            </header>
            {response.unplaced.length > 0 && (
              <p className="m-0 border-b border-line bg-bad-soft px-5 py-3" role="alert">
                {t("solver.unplaced")}{" "}
                {response.unplaced.map((id, index) => (
                  <React.Fragment key={id}>
                    {index > 0 && ", "}
                    <strong>
                      <ItemName id={id} />
                    </strong>
                  </React.Fragment>
                ))}
              </p>
            )}
            <ol className="m-0 list-none p-0">
              {list.map((solution, index) => (
                <ResultRow
                  key={Object.values(solution.items).join("-")}
                  meta={meta}
                  rank={index + 1}
                  solution={solution}
                  request={lastRequest}
                  onApply={() => void apply(solution)}
                  onOpen={() => open(solution)}
                  onCompare={() => setComparing(index)}
                  onAdd={(item, from) => void addToBuild(item, from)}
                  onExclude={(item) => {
                    exclude(item)
                    toaster(t("solver.excludedToast", { item: item.name }))
                  }}
                />
              ))}
            </ol>
            {response.total > list.length && (
              <div className="flex flex-wrap items-center justify-center gap-3 p-4">
                <span className="text-ink-2">{t("build.shown", { count: list.length, total: response.total })}</span>
                <Button disabled={loadingMore} onClick={() => void loadMore()}>
                  {loadingMore
                    ? t("common.loading")
                    : t("build.showMore", { count: Math.min(RESULTS_PAGE, response.total - list.length) })}
                </Button>
              </div>
            )}
          </>
        ) : (
          <div className="board grid h-full place-items-center p-6">
            <div className="grid max-w-md justify-items-start gap-3 rounded-lg border border-line bg-panel p-6">
              <h2 className="m-0 text-base font-bold">{t("solver.idle.title")}</h2>
              <p className="m-0 text-ink-2">{t("solver.idle.body")}</p>
              <Button onClick={() => updateSolver({ constraints: example.constraints, maximize: example.maximize })}>
                {t("solver.example")}
              </Button>
            </div>
          </div>
        )}
      </section>
  )

  const compare = (
    <CompareDialog
      meta={meta}
      open={comparing !== null}
      onOpenChange={(next) => !next && setComparing(null)}
      options={compareOptions}
      initial={{ from: build.id, to: comparing === null ? undefined : `result-${comparing}` }}
    />
  )

  if (breakpoint === "sm") {
    return (
      <div className="h-full overflow-y-auto">
        {form}
        {results}
        {compare}
      </div>
    )
  }
  return (
    <>
      <SidePanels id="search" left={form} leftSize={26}>
        {results}
      </SidePanels>
      {compare}
    </>
  )
}
