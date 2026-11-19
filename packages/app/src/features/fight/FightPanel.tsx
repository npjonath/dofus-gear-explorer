import { Button } from "@ui-sdk/react/button"
import { Popover, PopoverContent, PopoverDescription, PopoverTitle, PopoverTrigger } from "@ui-sdk/react/popover"
import { RotateCcwIcon, SlidersHorizontalIcon } from "lucide-react"
import * as React from "react"
import { useFightAction, useFightAim, useFightStart } from "../../api/queries"
import type {
  BuildInput,
  FightAction,
  FightBuff,
  FightDummies,
  FightEntity,
  FightEvent,
  FightSlot,
  FightView,
  Meta,
  Stat,
} from "../../api/types"
import { useI18n } from "../../i18n"
import { AP_STAT, HP_STAT, MP_STAT, statIndex, toneOf } from "../../lib/stats"
import { useApi } from "../../platform"
import type { Build } from "../../store"
import { NumberField } from "../../ui/NumberField"
import type { FloatCue } from "../../lib/fightTimeline"
import { HitCells, LineLabel } from "../build/DamagePanel"
import { elementTone, FightBoardView } from "./FightBoard"
import { FigurePortrait } from "./FightFigures"
import { useFightPlayback } from "./useFightPlayback"

interface LogLine {
  id: number
  event: FightEvent
  /** Who it happened to and which spell, named when it happened. */
  who: string
  spell: string
}

interface Played {
  key: string
  view: FightView
  log: LogLine[]
}

const elements = ["neutral", "earth", "fire", "water", "air"] as const
// The characteristics of a dummy's resistances, per element in the order above.
const percentStats = [37, 33, 34, 35, 36]
const fixedStats = [58, 54, 55, 56, 57]
const CRITICAL_RESIST = 87
// Characteristics the effects of a fight raise or lower, which no item carries.
const fightStats = [
  { id: 107, unit: "%" },
  { id: 101, unit: "%" },
  { id: 98, unit: "" },
  { id: 103, unit: "" },
] as const

let sequence = 0
const nextId = () => ++sequence

function SpellIcon({ slot }: { slot: FightSlot }) {
  const api = useApi()
  const [failed, setFailed] = React.useState(false)
  if (failed) {
    const initials = slot.name
      .split(/\s+/)
      .filter((word) => word.length > 2 || /^\p{Lu}/u.test(word))
      .slice(0, 2)
      .map((word) => word[0])
      .join("")
    return <span className="fight-spell-initials">{initials.toUpperCase()}</span>
  }
  const path = slot.weapon ? `/images/items/${slot.iconId}.png` : `/images/spells/${slot.iconId}.png`
  return <img src={api.imageUrl(path)} alt="" loading="lazy" draggable={false} onError={() => setFailed(true)} />
}

type SpellGroup = "class" | "common"

// Two rows whatever the class: a spell and its variant share a column, as in
// the game's own spell list, and the common spells wait behind their tab.
function SpellBar({
  slots,
  selected,
  onSelect,
  onPeek,
}: {
  slots: FightSlot[]
  selected: number | null
  onSelect(id: number): void
  onPeek(id: number | null): void
}) {
  const { t } = useI18n()
  const [wanted, setWanted] = React.useState<SpellGroup>("class")
  const groups: Record<SpellGroup, FightSlot[]> = {
    class: slots.filter((slot) => slot.weapon || slot.breedId !== 0),
    common: slots.filter((slot) => !slot.weapon && slot.breedId === 0),
  }
  const names: Record<SpellGroup, { short: string; full: string }> = {
    class: { short: t("fight.spells.class"), full: t("fight.classSpells") },
    common: { short: t("fight.spells.common"), full: t("fight.commonSpells") },
  }
  const tabs = (["class", "common"] as const).filter((name) => groups[name].length > 0)
  const tab = tabs.includes(wanted) ? wanted : (tabs[0] ?? wanted)
  const columns = new Map<string, FightSlot[]>()
  for (const slot of groups[tab]) {
    const column = slot.weapon ? "weapon" : String(slot.variantOf || slot.id)
    columns.set(column, [...(columns.get(column) ?? []), slot])
  }
  return (
    <div className="flex min-w-0 flex-1 items-start gap-2">
      {tabs.length > 1 && (
        <div className="grid flex-none gap-1" role="group" aria-label={t("fight.spells")}>
          {tabs.map((name) => (
            <button
              key={name}
              type="button"
              className="chip fight-tab"
              aria-pressed={tab === name}
              aria-label={names[name].full}
              title={names[name].full}
              onClick={() => setWanted(name)}
            >
              {names[name].short}
            </button>
          ))}
        </div>
      )}
      <div
        className="fight-bar"
        role="group"
        aria-label={names[tab].full}
        style={{ "--cols": String(columns.size) } as React.CSSProperties}
      >
        <div className="fight-bar-row">
          {[...columns].map(([column, pair]) => (
            <div key={column} className="fight-pair">
              {pair.map((slot) => {
                // Before the fight starts nothing can be cast, and nothing is wrong with any spell.
                const blocked = slot.blocked && slot.blocked !== "phase" ? slot.blocked : null
                const state = blocked ? t(`fight.blocked.${blocked}`) : ""
                return (
                  <button
                    key={slot.id}
                    type="button"
                    className="cell fight-spell"
                    aria-pressed={selected === slot.id}
                    aria-disabled={Boolean(blocked)}
                    aria-label={[slot.name, t("item.apCost", { n: slot.apCost }), state].filter(Boolean).join(", ")}
                    onClick={() => onSelect(slot.id)}
                    onMouseEnter={() => onPeek(slot.id)}
                    onMouseLeave={() => onPeek(null)}
                    onFocus={() => onPeek(slot.id)}
                    onBlur={() => onPeek(null)}
                  >
                    <SpellIcon slot={slot} />
                    <span className="fight-spell-cost" data-tone="ap">
                      {slot.apCost}
                    </span>
                    {slot.cooldown > 0 && <span className="fight-spell-wait">{slot.cooldown}</span>}
                  </button>
                )
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// The class portrait when the app has it, the board's own figure otherwise.
function Portrait({ entity, breedId }: { entity: FightEntity; breedId: number | null }) {
  const api = useApi()
  const [failed, setFailed] = React.useState<number | null>(null)
  if (entity.kind === "player" && breedId !== null && failed !== breedId) {
    return <img src={api.imageUrl(`/images/breeds/${breedId}.png`)} alt="" draggable={false} onError={() => setFailed(breedId)} />
  }
  return <FigurePortrait kind={entity.kind} />
}

function SpellSheet({ slot }: { slot: FightSlot }) {
  const { t } = useI18n()
  const reach = slot.rangeMin === slot.rangeMax ? String(slot.rangeMax) : `${slot.rangeMin} – ${slot.rangeMax}`
  const rules = [
    t("item.apCost", { n: slot.apCost }),
    t("fight.rule.range", { reach }),
    slot.rules.boostRange ? t("fight.rule.boost") : null,
    slot.rules.line && slot.rules.diagonal
      ? t("fight.rule.lineOrDiagonal")
      : slot.rules.line
        ? t("fight.rule.line")
        : slot.rules.diagonal
          ? t("fight.rule.diagonal")
          : null,
    slot.rangeMax > 0 ? t(slot.rules.sight ? "fight.rule.sight" : "fight.rule.noSight") : null,
    slot.rules.perTurn ? t("fight.rule.perTurn", { count: slot.rules.perTurn }) : null,
    slot.rules.perTarget ? t("fight.rule.perTarget", { count: slot.rules.perTarget }) : null,
    slot.rules.interval ? t("fight.rule.interval", { count: slot.rules.interval }) : null,
    slot.critRate > 0 ? t("damage.critChance", { chance: slot.critRate }) : null,
  ].filter(Boolean)
  const lines = slot.attack?.lines ?? []
  const canCrit = slot.critRate > 0
  return (
    <div className="grid gap-2">
      <div className="flex items-center gap-2">
        <span className="cell">
          <SpellIcon slot={slot} />
        </span>
        <span className="min-w-0">
          <span className="block truncate font-bold">{slot.name}</span>
          <span className="block text-[0.86rem] text-ink-2">{rules.join(" · ")}</span>
        </span>
      </div>
      {slot.blocked && slot.blocked !== "phase" && (
        <p className="m-0 text-[0.93rem] text-ink-2">
          {slot.cooldown > 0 ? t("fight.cooldown", { count: slot.cooldown }) : t(`fight.blocked.${slot.blocked}`)}
        </p>
      )}
      {lines.length > 0 ? (
        <div className="grid gap-1">
          <div className="damage-line fight-lines text-[0.86rem] text-ink-2">
            <span />
            <span className="damage-value font-semibold!">{t("damage.normal")}</span>
            <span className="damage-value font-semibold!">{t("damage.critical")}</span>
          </div>
          {lines.map((line, index) => (
            <div key={index} className="damage-line fight-lines">
              <LineLabel line={line} />
              <HitCells hit={line} canCrit={canCrit && line.critMax > 0} />
            </div>
          ))}
        </div>
      ) : (
        <p className="m-0 text-[0.93rem] text-ink-2">{t("fight.sheet.noLines")}</p>
      )}
    </div>
  )
}

function turnsText(t: ReturnType<typeof useI18n>["t"], turns: number): string {
  return turns < 0 ? t("fight.buff.forever") : t("fight.buff.turns", { count: turns })
}

function signed(value: number, format: (n: number) => string): string {
  return value > 0 ? `+${format(value)}` : `−${format(-value)}`
}

function EntitySheet({
  entity,
  name,
  names,
  stats,
}: {
  entity: FightEntity
  name: string
  names: Record<string, string>
  stats: Map<number, Stat>
}) {
  const { t, number } = useI18n()
  function describe(buff: FightBuff): { label: string; tone: string } | null {
    switch (buff.kind) {
      case "stat": {
        const stat = stats.get(buff.ref)
        if (!stat) return null
        return { label: `${signed(buff.value, number)}${stat.unit} ${stat.label}`, tone: toneOf(stat.key) }
      }
      case "base":
        return { label: t("fight.buff.base", { spell: names[buff.ref] ?? "", value: number(buff.value) }), tone: "plain" }
      case "range":
        return { label: t("fight.buff.range", { spell: names[buff.ref] ?? "", value: number(buff.value) }), tone: "range" }
      case "suffered":
        return { label: t("fight.buff.suffered", { value: number(buff.value) }), tone: "plain" }
      case "healed":
        return { label: t("fight.buff.healed", { value: number(buff.value) }), tone: "plain" }
      default:
        return null
    }
  }
  const shown = entity.buffs.flatMap((buff, index) => {
    const text = describe(buff)
    return text ? [{ ...text, index, buff }] : []
  })
  return (
    <div className="grid gap-2">
      <div className="flex items-baseline justify-between gap-2">
        <span className="truncate font-bold">{name}</span>
        <span className="flex-none font-bold tabular-nums" data-tone="vitality" style={{ color: "var(--tone)" }}>
          {t("fight.entity.hp", { hp: number(entity.hp), max: number(entity.maxHp) })}
        </span>
      </div>
      {shown.length > 0 ? (
        <ul className="effects m-0 grid list-none gap-1 p-0">
          {shown.map(({ label, tone, index, buff }) => (
            <li key={index} className="flex items-baseline gap-2">
              <span className="pip" data-tone={tone} />
              <span className="min-w-0 flex-1">
                {label}
                <span className="text-ink-3"> · {names[buff.from] ?? ""}</span>
              </span>
              <span className="flex-none text-[0.86rem] text-ink-2">{turnsText(t, buff.turns)}</span>
            </li>
          ))}
        </ul>
      ) : (
        <p className="m-0 text-[0.93rem] text-ink-2">{t("fight.entity.noEffects")}</p>
      )}
    </div>
  )
}

function LogEntry({ line, stats }: { line: LogLine; stats: Map<number, Stat> }) {
  const { t, number } = useI18n()
  const { event, who, spell } = line
  const value = event.value ?? 0
  switch (event.type) {
    case "turn":
      return <li className="fight-log-turn">{t("fight.turn", { turn: value })}</li>
    case "cast":
      return (
        <li>
          {t("fight.log.cast", { who, spell })}
          {event.critical && <strong className="text-ink"> {t("fight.log.critical")}</strong>}
        </li>
      )
    case "damage":
      return (
        <li className="flex items-baseline gap-2 pl-3">
          <span className="pip" data-tone={event.push ? "plain" : elementTone(event.element)} />
          <span>{t(event.push ? "fight.log.push" : "fight.log.damage", { who, value: number(value) })}</span>
        </li>
      )
    case "heal":
      return <li className="pl-3">{t("fight.log.heal", { who, value: number(value) })}</li>
    case "buff": {
      const stat = stats.get(event.stat ?? -1)
      if (!stat) return null
      return (
        <li className="flex items-baseline gap-2 pl-3">
          <span className="pip" data-tone={toneOf(stat.key)} />
          <span>
            {t("fight.log.buff", {
              who,
              effect: `${signed(value, number)}${stat.unit} ${stat.label}`,
              turns: turnsText(t, event.turns ?? 1),
            })}
          </span>
        </li>
      )
    }
    case "ap":
      return <li className="pl-3">{t("fight.log.ap", { who, count: value })}</li>
    case "move": {
      const cells = (event.path?.length ?? 1) - 1
      return <li className="pl-3">{t(value > 0 ? "fight.log.walk" : "fight.log.moved", { who, count: value > 0 ? value : cells })}</li>
    }
    case "defeat":
      return <li className="pl-3">{t("fight.log.defeat", { who })}</li>
    case "skipped":
      return <li className="pl-3 text-ink-3">{t("fight.log.skipped", { spell })}</li>
    default:
      return null
  }
}

function DummySettings({ dummies, onChange }: { dummies: FightDummies; onChange(next: FightDummies): void }) {
  const { t } = useI18n()
  const setStat = (id: number, value: number) => onChange({ ...dummies, stats: { ...dummies.stats, [id]: value } })
  return (
    <Popover>
      <PopoverTrigger className="btn-ghost gap-2">
        <SlidersHorizontalIcon aria-hidden="true" className="size-4" />
        {t("fight.dummies")}
      </PopoverTrigger>
      <PopoverContent align="end" className="grid w-80 gap-3">
        <PopoverTitle className="section-title">{t("fight.dummies")}</PopoverTitle>
        <div className="grid grid-cols-[1fr_auto] items-center gap-x-3 gap-y-2">
          <span className="text-ink-2">{t("fight.dummies.count")}</span>
          <NumberField
            label={t("fight.dummies.count")}
            value={dummies.count}
            min={1}
            max={4}
            onChange={(count) => onChange({ ...dummies, count })}
          />
          <span className="text-ink-2">{t("fight.dummies.hp")}</span>
          <NumberField
            label={t("fight.dummies.hp")}
            value={dummies.hp}
            min={1}
            max={1_000_000}
            onChange={(hp) => onChange({ ...dummies, hp })}
          />
        </div>
        <div className="grid grid-cols-[1fr_auto_auto] items-center gap-x-3 gap-y-2">
          <span />
          <span className="text-right text-[0.86rem] text-ink-2">{t("fight.dummies.percent")}</span>
          <span className="text-right text-[0.86rem] text-ink-2">{t("fight.dummies.fixed")}</span>
          {elements.map((element, index) => {
            const name = t(`element.${element}`)
            return (
              <React.Fragment key={element}>
                <span className="flex items-center gap-2">
                  <span className="pip" data-tone={element} />
                  {name}
                </span>
                <NumberField
                  label={`${name}, ${t("fight.dummies.percent")}`}
                  value={dummies.stats[percentStats[index]!] ?? 0}
                  min={-200}
                  max={100}
                  onChange={(value) => setStat(percentStats[index]!, value)}
                />
                <NumberField
                  label={`${name}, ${t("fight.dummies.fixed")}`}
                  value={dummies.stats[fixedStats[index]!] ?? 0}
                  min={-1000}
                  max={1000}
                  onChange={(value) => setStat(fixedStats[index]!, value)}
                />
              </React.Fragment>
            )
          })}
          <span className="col-span-2">{t("fight.dummies.critical")}</span>
          <NumberField
            label={t("fight.dummies.critical")}
            value={dummies.stats[CRITICAL_RESIST] ?? 0}
            min={0}
            max={1000}
            onChange={(value) => setStat(CRITICAL_RESIST, value)}
          />
        </div>
        <PopoverDescription className="m-0 text-[0.86rem] text-ink-2">{t("fight.dummies.hint")}</PopoverDescription>
      </PopoverContent>
    </Popover>
  )
}

export function FightPanel({ meta, build, input }: { meta: Meta; build: Build; input: BuildInput }) {
  const { t, number } = useI18n()
  const stats = React.useMemo(() => {
    // What only a fight changes has no line in a stuff's sheet: it is named here.
    const index = statIndex(meta)
    for (const { id, unit } of fightStats) {
      if (!index.has(id)) index.set(id, { id, key: `fight.${id}`, label: t(`fight.stat.${id}`), unit, categoryId: 0, forgeMax: 0 })
    }
    return index
  }, [meta, t])
  const [dummies, setDummies] = React.useState<FightDummies>({ count: 1, hp: 50_000, stats: {} })
  const [round, setRound] = React.useState(0)
  const start = useFightStart(input, build.breedId, dummies, round)
  const action = useFightAction(input, build.breedId)

  // A fight belongs to the stuff, the class and the dummies it started with:
  // change one and a new fight starts.
  const key = JSON.stringify([input, build.breedId, dummies, round])
  const [played, setPlayed] = React.useState<Played | null>(null)
  const live = played?.key === key ? played : null
  const view = live?.view ?? start.data
  const log = live?.log ?? []

  const [spell, setSpell] = React.useState<number | null>(null)
  const [peek, setPeek] = React.useState<number | null>(null)
  const [cursor, setCursor] = React.useState<number | null>(null)
  const [picked, setPicked] = React.useState<number | null>(null)
  // The entity the player clicked in the turn order: it stays pointed until another is.
  const [pinned, setPinned] = React.useState<number | null>(null)
  const [refused, setRefused] = React.useState<string | null>(null)
  const playback = useFightPlayback(key)
  const logEnd = React.useRef<HTMLLIElement>(null)

  const aim = useFightAim(input, build.breedId, view?.state, spell).data
  const breedName = meta.breeds.find((breed) => breed.id === build.breedId)?.name

  function nameOf(entity: FightEntity): string {
    if (entity.kind === "player") return breedName ?? t("fight.you")
    return t("fight.dummy", { n: entity.id - 1 })
  }

  function restart() {
    setRound(round + 1)
    setSpell(null)
    setPicked(null)
    setPinned(null)
    setRefused(null)
    playback.reset()
  }

  // What a number rising from an entity says.
  function floatText(cue: FloatCue): string {
    if (cue.kind === "ap") return `+${number(cue.amount)} ${stats.get(AP_STAT)?.label ?? ""}`.trim()
    return cue.amount < 0 ? `−${number(-cue.amount)}` : `+${number(cue.amount)}`
  }

  // `zone` is what the cast was shown to cover when it was aimed: the events do not say it.
  function act(next: FightAction, zone?: number[]) {
    if (!view || action.isPending) return
    const before = view.state
    action.mutate(
      { state: before, action: next },
      {
        onSuccess: (result) => {
          const byId = new Map(result.state.entities.map((entity) => [entity.id, entity]))
          const lines: LogLine[] = result.events.map((event) => {
            const entity = byId.get(event.entity ?? 0)
            return { id: nextId(), event, who: entity ? nameOf(entity) : "", spell: result.names[event.spell ?? -1] ?? "" }
          })
          setPlayed({ key, view: result, log: [...log, ...lines].slice(-400) })
          // Placing an entity is not an event of the fight: it is carried to its cell all the same.
          const carried = before.entities.find((entity) => next.type === "place" && entity.id === next.entity)
          const moved = result.state.entities.find((entity) => entity.id === carried?.id)
          const events: FightEvent[] =
            carried && moved && moved.cell !== carried.cell
              ? [{ type: "move", entity: carried.id, path: [carried.cell, moved.cell] }]
              : result.events
          playback.play(before, events, floatText, zone)
          setRefused(result.refused ?? null)
          if (result.spells.find((slot) => slot.id === spell)?.blocked) setSpell(null)
          requestAnimationFrame(() => logEnd.current?.scrollIntoView({ block: "nearest" }))
        },
      }
    )
  }

  function onCell(cell: number) {
    if (!view) return
    // A click does not wait for the board: what was playing jumps to its end.
    playback.skip()
    const occupant = view.state.entities.find((entity) => entity.cell === cell)
    if (view.state.phase === "placement") {
      if (occupant) setPicked(picked === occupant.id ? null : occupant.id)
      else if (picked !== null) act({ type: "place", entity: picked, cell })
      return
    }
    if (spell !== null) {
      const target = aim?.targets.find((candidate) => candidate.cell === cell)
      if (target) act({ type: "cast", spell, cell }, target.zone)
      return
    }
    if (view.steps.some((step) => step.cell === cell)) act({ type: "move", cell })
  }

  if (!view) {
    return (
      <div className="board grid flex-1 place-items-center p-6">
        <p className="m-0 rounded-lg border border-line bg-panel p-6 text-ink-2" role={start.isError ? "alert" : "status"}>
          {start.isError ? t("fight.failed") : t("fight.loading")}
        </p>
      </div>
    )
  }

  const { state } = view
  const fighting = state.phase === "fight"
  const player = state.entities.find((entity) => entity.kind === "player")!
  const shownSlot = view.spells.find((slot) => slot.id === (peek ?? spell))
  const hovered = cursor !== null ? state.entities.find((entity) => entity.cell === cursor) : undefined
  const shownEntity = hovered ?? state.entities.find((entity) => entity.id === pinned) ?? player
  // Health follows the board: a loss shows when the blow lands.
  const hpOf = (entity: FightEntity) => playback.hp?.[entity.id] ?? entity.hp
  const vitals = [
    { stat: stats.get(HP_STAT), tone: "vitality", value: number(hpOf(player)) },
    { stat: stats.get(AP_STAT), tone: "ap", value: number(view.ap) },
    { stat: stats.get(MP_STAT), tone: "mp", value: number(view.mp) },
  ]

  return (
    <div className="fight flex min-h-0 flex-1 flex-col lg:flex-row">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-x-4 border-b border-line bg-panel px-4 py-2">
          <strong className="flex-none">{fighting ? t("fight.turn", { turn: state.turn }) : t("fight.placement")}</strong>
          <span className="min-w-0 flex-1 truncate text-ink-2" title={fighting ? undefined : t("fight.placementHint")}>
            {fighting
              ? t("fight.damage", { turn: number(state.turnDamage), total: number(state.totalDamage) })
              : t("fight.placementHint")}
          </span>
          <DummySettings dummies={dummies} onChange={setDummies} />
          <Button onClick={restart}>
            <RotateCcwIcon aria-hidden="true" className="size-4" />
            {t("fight.restart")}
          </Button>
        </div>

        <div className="fight-stage min-h-0 flex-1">
          <FightBoardView
            view={view}
            aim={aim}
            spell={spell}
            cursor={cursor}
            picked={picked}
            pointed={hovered?.id ?? pinned}
            playback={playback}
            busy={action.isPending}
            nameOf={nameOf}
            onCursor={setCursor}
            onAct={onCell}
            onCancel={() => {
              setSpell(null)
              setPicked(null)
            }}
          />

          <ol className="fight-order" aria-label={t("fight.order")}>
            {state.entities.map((entity) => {
              const name = nameOf(entity)
              const active = fighting && entity.kind === "player"
              const hp = hpOf(entity)
              return (
                <li key={entity.id}>
                  <button
                    type="button"
                    className="cell fight-order-cell"
                    title={name}
                    aria-label={[name, t("fight.entity.hp", { hp: number(hp), max: number(entity.maxHp) }), active ? t("fight.order.playing") : ""]
                      .filter(Boolean)
                      .join(", ")}
                    aria-pressed={pinned === entity.id}
                    aria-current={active || undefined}
                    data-pointed={shownEntity.id === entity.id || undefined}
                    onClick={() => setPinned(pinned === entity.id ? null : entity.id)}
                    onMouseEnter={() => setCursor(entity.cell)}
                    onMouseLeave={() => setCursor(null)}
                    onFocus={() => setCursor(entity.cell)}
                    onBlur={() => setCursor(null)}
                  >
                    <Portrait entity={entity} breedId={build.breedId} />
                    <span className="fight-order-hp">
                      <span style={{ transform: `scaleX(${Math.max(0, Math.min(1, hp / entity.maxHp))})` }} />
                    </span>
                  </button>
                </li>
              )
            })}
          </ol>

          <div className="fight-vitals">
            {vitals.map(({ stat, tone, value }) => (
              <div key={tone} className="vital fight-vital" data-tone={tone}>
                <span className="label">{stat?.label}</span>
                <span className="value">{value}</span>
              </div>
            ))}
          </div>

          <Button
            data-variant="solid"
            className="fight-end h-10!"
            disabled={action.isPending}
            onClick={() => {
              playback.skip()
              setSpell(null)
              setPicked(null)
              act({ type: fighting ? "end" : "begin" })
            }}
          >
            {fighting ? t("fight.endTurn") : t("fight.ready")}
          </Button>
        </div>

        <div className="fight-dock flex items-start gap-3 border-t border-line bg-panel px-4 py-2">
          {build.breedId === null && view.spells.every((slot) => slot.weapon || slot.breedId === 0) && (
            <p className="m-0 max-w-56 flex-none text-ink-2">{t("fight.noBreed")}</p>
          )}
          <SpellBar slots={view.spells} selected={spell} onSelect={toggleSpell} onPeek={setPeek} />
        </div>
      </div>

      <aside className="fight-side flex min-h-0 flex-none flex-col border-line bg-panel max-lg:border-t lg:w-[22rem] lg:border-l">
        <section className="grid gap-2 border-b border-line p-4" aria-live="polite">
          {shownSlot ? <SpellSheet slot={shownSlot} /> : <p className="m-0 text-ink-2">{t("fight.sheet.empty")}</p>}
          {refused && refused !== "phase" && (
            <p className="m-0 text-bad" role="alert">
              {t(`fight.blocked.${refused}` as "fight.blocked.ap")}
            </p>
          )}
        </section>
        <section className="grid gap-2 border-b border-line p-4">
          <h3 className="section-title">{t("fight.entity.effects")}</h3>
          <EntitySheet entity={shownEntity} name={nameOf(shownEntity)} names={view.names} stats={stats} />
        </section>
        <section className="flex min-h-40 flex-1 flex-col">
          <h3 className="section-title px-4 pt-4 pb-2">{t("fight.log")}</h3>
          <ol className="fight-log m-0 min-h-0 flex-1 list-none overflow-y-auto px-4 pb-4" aria-live="polite">
            {log.length === 0 && <li className="text-ink-2">{t("fight.log.empty")}</li>}
            {log.map((line) => (
              <LogEntry key={line.id} line={line} stats={stats} />
            ))}
            <li ref={logEnd} aria-hidden="true" />
          </ol>
          <p className="m-0 border-t border-line px-4 py-3 text-[0.86rem] text-ink-3">{t("fight.limits")}</p>
        </section>
      </aside>
    </div>
  )

  function toggleSpell(id: number) {
    if (!view) return
    const slot = view.spells.find((candidate) => candidate.id === id)
    playback.skip()
    setRefused(null)
    if (!slot || slot.blocked) {
      setSpell(null)
      return
    }
    setSpell(spell === id ? null : id)
  }
}
