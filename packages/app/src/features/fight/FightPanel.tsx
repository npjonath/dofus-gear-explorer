import { Dialog, DialogBackdrop, DialogContent, DialogTitle } from "@ui-sdk/react/dialog"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { loadFightKeys, keyLabel } from "../../lib/fightKeys"
import { GameActor } from "./GameActor"
import { useGameSpellCatalog } from "./GameSpellEffects"
import { entityAt } from "../../lib/fightBoard"
import { CombatInventory } from "./CombatInventory"
import { FightSpellBook } from "./FightSpellBook"
import { LightAnimations, useCombatPreferences, type CombatPreferences } from "./combatPreferences"
import { FightOptions } from "./FightOptions"
import { StatsPanel } from "../build/StatsPanel"
import { Popover, PopoverContent, PopoverDescription, PopoverTitle, PopoverTrigger } from "@ui-sdk/react/popover"
import {
  ArrowLeftIcon,
  BookOpenIcon,
  BackpackIcon,
  SettingsIcon,
  MaximizeIcon,
  UserRoundIcon,
  XIcon,
  RotateCcwIcon,
  SlidersHorizontalIcon,
} from "lucide-react"
import * as React from "react"
import { useEvaluation, useFightAction, useFightAim, useFightStart } from "../../api/queries"
import type {
  BuildInput,
  Evaluation,
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
import { AP_STAT, statIndex, toneOf } from "../../lib/stats"
import { useApi } from "../../platform"
import type { Build } from "../../store"
import { NumberField } from "../../ui/NumberField"
import type { FloatCue } from "../../lib/fightTimeline"
import { HitCells, LineLabel } from "../build/DamagePanel"
import { elementTone, FightBoardView } from "./FightBoard"
import { FigurePortrait } from "./FightFigures"
import { useFightPlayback } from "./useFightPlayback"
import { useFightAudio } from "./useFightAudio"

interface LogLine {
  id: number
  event: FightEvent
  /** Who it happened to and which spell, named when it happened. */
  who: string
  spell: string
  time: string
  hits?: FightEvent[]
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

function spellGroups(slots: FightSlot[]) {
  const groups = new Map<number, FightSlot[]>()
  for (const slot of slots) {
    if (slot.weapon) continue
    const base = slot.variantOf || slot.id
    groups.set(base, [...(groups.get(base) ?? []), slot])
  }
  const commonOrder = [18646, 370, 368, 369, 367, 373, 24006, 3506, 24015, 24011, 366, 364]
  const rank = (id: number) => {
    const i = commonOrder.indexOf(id)
    return i < 0 ? commonOrder.length + id : i
  }
  return [...groups.entries()].sort(([a, sa], [b, sb]) => {
    if (sa[0]!.breedId < 0 && sb[0]!.breedId < 0) return 0
    return Number(sb[0]!.breedId !== 0) - Number(sa[0]!.breedId !== 0) || (sa[0]!.breedId === 0 ? rank(a) - rank(b) : a - b)
  })
}

function SpellButton({
  slot,
  selected,
  onSelect,
  onPeek,
  shortcut,
}: {
  slot: FightSlot
  selected: boolean
  onSelect: (id: number) => void
  onPeek: (id: number | null) => void
  shortcut: string
}) {
  const { t } = useI18n()
  const reason = slot.blocked && slot.blocked !== "phase" ? t(`fight.blocked.${slot.blocked}`) : ""
  return (
    <button
      type="button"
      className="cell fight-spell"
      aria-label={`${slot.name}, ${t("item.apCost", { n: slot.apCost })}`}
      aria-pressed={selected}
      aria-disabled={Boolean(slot.blocked)}
      title={[slot.name, shortcut, reason].filter(Boolean).join(" · ")}
      onClick={() => onSelect(slot.id)}
      onMouseEnter={() => onPeek(slot.id)}
      onMouseLeave={() => onPeek(null)}
      onFocus={() => onPeek(slot.id)}
      onBlur={() => onPeek(null)}
    >
      <SpellIcon slot={slot} />
      {slot.cooldown > 0 && <span className="fight-spell-wait">{slot.cooldown}</span>}
    </button>
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
      case "finalHeal":
        return { label: t("fight.portal.heals", { value: signed(buff.value, number) }), tone: "vitality" }
      case "invisible":
        return { label: t("fight.state.invisible"), tone: "plain" }
      case "state":
        if (buff.ref === 3) return { label: t("fight.state.carrier"), tone: "plain" }
        if (buff.ref === 8) return { label: t("fight.state.carried"), tone: "plain" }
        return null
      case "armor":
      case "shield":
        return { label: t(buff.kind === "armor" ? "fight.armor" : "fight.shield", { n: number(buff.value) }), tone: "plain" }
      case "stat": {
        const stat = stats.get(buff.ref)
        if (!stat) return null
        return { label: `${signed(buff.value, number)}${stat.unit} ${stat.label}`, tone: toneOf(stat.key) }
      }
      case "base":
        return { label: t("fight.buff.base", { spell: names[buff.ref] ?? "", value: number(buff.value) }), tone: "plain" }
      case "range":
        return { label: t("fight.buff.range", { spell: names[buff.ref] ?? "", value: number(buff.value) }), tone: "range" }
      case "vitality":
        return { label: `${signed(buff.value, number)} ${stats.get(buff.ref)?.label ?? "PV"}`, tone: "vitality" }
      case "minimum":
      case "maximum":
        return { label: t(`fight.buff.${buff.kind}`), tone: "plain" }
      case "stateDisabled":
        return { label: t("fight.buff.stateDisabled", { n: buff.ref }), tone: "plain" }
      case "healBase":
        return {
          label: t("fight.buff.healBase", { spell: names[buff.ref] ?? "", value: signed(buff.value, number) }),
          tone: "plain",
        }
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

function LogEntry({ line, stats, inline = false }: { line: LogLine; stats: Map<number, Stat>; inline?: boolean }) {
  const { t, number } = useI18n()
  const { event, who, spell } = line
  const value = event.value ?? 0
  switch (event.type) {
    case "invisible":
      return (
        <li>
          {who} · {t("fight.state.invisible")} ({turnsText(t, event.turns ?? 1)})
        </li>
      )
    case "reveal":
      return <li>{t("fight.log.reveal", { who })}</li>
    case "armor":
    case "shield":
      return (
        <li>
          {who} · {t(event.type === "armor" ? "fight.armor" : "fight.shield", { n: signed(value, number) })}
        </li>
      )
    case "summon":
      return <li>{t("fight.log.summon", { who, spell })}</li>
    case "rune":
      return <li>{t("fight.log.rune", { who, spell })}</li>
    case "mark":
      return <li>{who ? t("fight.log.mark", { who, spell }) : t("fight.log.emptyMark", { spell })}</li>
    case "dodge": {
      const stat = event.stat === undefined ? "" : (stats.get(event.stat)?.label ?? "")
      return <li>{t("fight.log.dodge", { who, value: number(value), stat })}</li>
    }
    case "turn":
      return (
        <li className="fight-log-turn">
          <time>{line.time}</time> {t("fight.turn", { turn: value })}
        </li>
      )
    case "cast": {
      const Tag = inline ? "span" : "li"
      return (
        <Tag>
          <time>{line.time}</time> <b>{t("fight.log.cast", { who, spell })}</b>
          {event.critical && <strong className="text-ink"> {t("fight.log.critical")}</strong>}
        </Tag>
      )
    }
    case "damage":
      return (
        <li className="flex items-baseline gap-2 pl-3">
          <span className="pip" data-tone={event.push ? "plain" : elementTone(event.element)} />
          <span>
            {t(event.push ? "fight.log.push" : "fight.log.damage", {
              who,
              value: number(line.hits?.reduce((sum, hit) => sum + (hit.value ?? 0), 0) ?? value),
            })}
            {(line.hits?.length ?? 0) > 1 && (
              <>
                {" "}
                (
                {line.hits!.map((hit, i) => (
                  <React.Fragment key={i}>
                    {i > 0 ? " + " : ""}
                    <em data-tone={elementTone(hit.element)}>{number(hit.value ?? 0)}</em>
                  </React.Fragment>
                ))}
                )
              </>
            )}
          </span>
        </li>
      )
    case "heal":
      return <li className="pl-3">{t("fight.log.heal", { who, value: number(value) })}</li>
    case "buff": {
      if (event.action === 2971)
        return (
          <li>
            {t("fight.log.buff", {
              who,
              effect: t("fight.portal.heals", { value: signed(value, number) }),
              turns: turnsText(t, event.turns ?? 1),
            })}
          </li>
        )
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
      if (event.carrier && (event.action === 50 || event.action === 51)) {
        return <li>{t(event.action === 50 ? "fight.log.pickup" : "fight.log.throw", { who })}</li>
      }
      const cells = (event.path?.length ?? 1) - 1
      return (
        <li className="pl-3">{t(value > 0 ? "fight.log.walk" : "fight.log.moved", { who, count: value > 0 ? value : cells })}</li>
      )
    }
    case "eliminated":
      return <li>{t("fight.log.eliminated", { who })}</li>
    case "defeat":
      return <li className="pl-3">{t("fight.log.defeat", { who })}</li>
    case "skipped":
      return <li className="pl-3 text-ink-3">{t("fight.log.skipped", { spell })}</li>
    default:
      return null
  }
}

function FightJournal({ log, stats, fold }: { log: LogLine[]; stats: Map<number, Stat>; fold: boolean }) {
  if (!fold) return log.map((line) => <LogEntry key={line.id} line={line} stats={stats} />)
  const blocks: { head: LogLine; effects: LogLine[] }[] = []
  for (const line of log) {
    const last = blocks.at(-1)
    if (last?.head.event.type === "cast" && line.event.type !== "cast" && line.event.type !== "turn") last.effects.push(line)
    else blocks.push({ head: line, effects: [] })
  }
  return blocks.map(({ head, effects }) =>
    effects.length ? (
      <li key={head.id}>
        <details>
          <summary>
            <LogEntry line={head} stats={stats} inline />
          </summary>
          <ol>
            {effects.map((line) => (
              <LogEntry key={line.id} line={line} stats={stats} />
            ))}
          </ol>
        </details>
      </li>
    ) : (
      <LogEntry key={head.id} line={head} stats={stats} />
    )
  )
}

function ChatOptions({
  prefs,
  update,
  copy,
}: {
  prefs: CombatPreferences
  update: (v: Partial<CombatPreferences>) => void
  copy: () => Promise<void>
}) {
  const { t } = useI18n()
  const [copied, setCopied] = React.useState<"ok" | "failed" | null>(null)
  return (
    <section className="combat-chat-options">
      <button
        onClick={() =>
          void copy().then(
            () => setCopied("ok"),
            () => setCopied("failed")
          )
        }
      >
        {t("fight.chat.copy")}
      </button>
      {copied && <span role="status">{t(copied === "ok" ? "fight.chat.copied" : "fight.chat.copyFailed")}</span>}
      {(
        [
          { key: "chatSize", name: "size", min: 10, max: 22 },
          { key: "chatSpacing", name: "spacing", min: 100, max: 180 },
          { key: "chatOpacity", name: "opacity", min: 0, max: 100 },
        ] as const
      ).map(({ key, name, min, max }) => (
        <label className="setting-line" key={key}>
          {t(`fight.chat.${name}`)}
          <input
            type="range"
            min={min}
            max={max}
            value={prefs[key]}
            onChange={(e) => update({ [key]: e.target.valueAsNumber })}
          />
          <output>
            {prefs[key]}
            {key === "chatSize" ? " px" : " %"}
          </output>
        </label>
      ))}
      <label>
        <input type="checkbox" checked={prefs.foldEffects} onChange={(e) => update({ foldEffects: e.target.checked })} />{" "}
        {t("fight.chat.fold")}
      </label>
    </section>
  )
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
            onChange={(count) => onChange({ ...dummies, count, allies: Math.min(count - 1, dummies.allies ?? 0) })}
          />
          {dummies.count > 1 && (
            <>
              <span className="text-ink-2">{t("fight.allies")}</span>
              <NumberField
                label={t("fight.allies")}
                value={dummies.allies ?? 0}
                min={0}
                max={dummies.count - 1}
                onChange={(allies) => onChange({ ...dummies, allies })}
              />
            </>
          )}
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

type Panel = "chat" | "stats" | "spells" | "gear" | "settings" | "entity" | null

export function FightPanel({
  meta,
  build,
  input: savedInput,
  evaluation: savedEvaluation,
  onExit,
}: {
  meta: Meta
  build: Build
  input: BuildInput
  evaluation?: Evaluation
  onExit: () => void
}) {
  // Equipment changes are local trials. Only an explicit restart adopts a pending draft.
  const [original] = React.useState(savedInput)
  const [input, setInput] = React.useState(original)
  const [draft, setDraft] = React.useState(original)
  const applied = useEvaluation(input)
  const evaluation =
    applied.isPlaceholderData || !applied.data ? (input === savedInput ? savedEvaluation : undefined) : applied.data
  const api = useApi()
  const cache = useQueryClient()
  const assetQuery = useQuery({ queryKey: ["fight-assets", api.baseUrl], queryFn: () => api.fightAssets(), staleTime: 60000 })
  const maps = assetQuery.data?.maps
  const [mapChoice, setMapChoice] = React.useState(99222029)
  const mapId = maps?.includes(mapChoice) ? mapChoice : (maps?.[0] ?? 0)
  const [panel, setPanel] = React.useState<Panel>(null)
  const [keys, setKeys] = React.useState(loadFightKeys)
  const [variants, setVariants] = React.useState<Record<number, number>>({})
  const [highlightMarks, setHighlightMarks] = React.useState(false)
  const [showTip, setShowTip] = React.useState(false)
  const gameRef = React.useRef<HTMLDivElement>(null)
  const [fullscreen, setFullscreen] = React.useState(false)
  const { prefs, update } = useCombatPreferences()
  const [leaving, setLeaving] = React.useState(false)
  const exitTimer = React.useRef<number | undefined>(undefined)
  React.useEffect(() => () => clearTimeout(exitTimer.current), [])
  const chatContent = React.useRef<HTMLOListElement>(null)
  function returnToBuild() {
    if (leaving) return
    if (
      view?.state.phase === "fight" &&
      view.state.board.id &&
      !prefs.skipExit &&
      !window.matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      playback.skip()
      setLeaving(true)
      exitTimer.current = window.setTimeout(finishExit, 850)
    } else finishExit()
  }
  function finishExit() {
    if (document.fullscreenElement) void document.exitFullscreen().catch(() => {})
    onExit()
  }
  const toggleFullscreen = async () => {
    if (document.fullscreenElement) await document.exitFullscreen()
    else await document.documentElement.requestFullscreen()
  }
  const { t, number } = useI18n()
  const stats = React.useMemo(() => {
    // What only a fight changes has no line in a stuff's sheet: it is named here.
    const index = statIndex(meta)
    for (const { id, unit } of fightStats) {
      if (!index.has(id))
        index.set(id, { id, key: `fight.${id}`, label: t(`fight.stat.${id}`), unit, categoryId: 0, forgeMax: 0 })
    }
    return index
  }, [meta, t])
  const [dummies, setDummies] = React.useState<FightDummies>({ count: 1, hp: 50_000, stats: {} })
  const [round, setRound] = React.useState(0)
  const start = useFightStart(input, build.breedId, { ...dummies, mapId }, round)
  const action = useFightAction(input, build.breedId)

  // A fight belongs to the stuff, the class and the dummies it started with:
  // change one and a new fight starts.
  const key = JSON.stringify([input, build.breedId, dummies, mapId, round])
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
  const visualCatalog = useGameSpellCatalog(mapId ? api.imageUrl("/fight-assets") : undefined, assetQuery.data?.pack)
  const audio = useFightAudio(mapId ? api.imageUrl("/fight-assets") : undefined, assetQuery.data?.pack)
  const playback = useFightPlayback(key, visualCatalog)
  const { play: playSound, interfaceSound, stop: stopAudio } = audio
  React.useEffect(() => {
    if (playback.run && view && !document.hidden) {
      playSound(playback.run, view.state.entities, build.breedId ?? 0, input.items.weapon ?? 0)
    }
  }, [playback.run, view, playSound, build.breedId, input.items.weapon])
  const previousPanel = React.useRef(panel)
  React.useEffect(() => {
    if (previousPanel.current === panel) return
    if (panel === "spells") interfaceSound("open")
    else if (previousPanel.current === "spells") interfaceSound("close")
    else if (panel) interfaceSound("click")
    previousPanel.current = panel
  }, [panel, interfaceSound])
  React.useEffect(() => {
    const hide = () => {
      if (document.hidden) stopAudio()
    }
    document.addEventListener("visibilitychange", hide)
    return () => {
      document.removeEventListener("visibilitychange", hide)
      stopAudio()
    }
  }, [stopAudio, key])
  const logEnd = React.useRef<HTMLLIElement>(null)
  const pendingAction = React.useRef(false)
  const actionEpoch = React.useRef(0)

  const aim = useFightAim(input, build.breedId, view?.state, spell).data
  const breedName = meta.breeds.find((breed) => breed.id === build.breedId)?.name

  function nameOf(entity: FightEntity): string {
    if (entity.kind === "summon") return entity.name ?? t("fight.actors")
    if (entity.kind === "player") return breedName ?? t("fight.you")
    return entity.id === 2 ? "Poutch Ingball" : `Poutch Ingball ${entity.id - 1}`
  }

  function restart() {
    actionEpoch.current++
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
    if (!view || pendingAction.current || start.isPlaceholderData) return
    audio.unlock()
    pendingAction.current = true
    const epoch = actionEpoch.current
    const before = view.state
    action.mutate(
      { state: before, action: next },
      {
        onSettled: () => {
          pendingAction.current = false
        },
        onSuccess: (result) => {
          if (epoch !== actionEpoch.current) return
          if (!result.refused && !document.hidden && result.state.phase === "fight") {
            audio.start(mapId)
            if (next.type === "begin") interfaceSound("begin")
            else if (next.type === "end") interfaceSound("endTurn")
            else if (next.type === "move") interfaceSound("move")
            if (result.events.some((event) => event.type === "cast" && event.critical)) interfaceSound("critical")
          }
          const byId = new Map(result.state.entities.map((entity) => [entity.id, entity]))
          const time = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false })
          const lines: LogLine[] = []
          for (const event of result.events) {
            const entity = byId.get(event.entity ?? 0)
            const previous = lines.at(-1)
            if (
              event.type === "damage" &&
              previous?.event.type === "damage" &&
              event.entity === previous.event.entity &&
              event.push === previous.event.push
            ) {
              previous.hits = [...(previous.hits ?? [previous.event]), event]
            } else {
              lines.push({
                id: nextId(),
                event,
                time,
                who: entity ? nameOf(entity) : "",
                spell: result.names[event.spell ?? -1] ?? "",
              })
            }
          }
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
    const occupant = entityAt(view.state.entities, cell)
    if (view.state.phase === "placement") {
      if (occupant) setPicked(picked === occupant.id ? null : occupant.id)
      else {
        const camp = stateBoard?.blue?.includes(cell) ? "player" : stateBoard?.red?.includes(cell) ? "dummy" : null
        const entity =
          view.state.entities.find((e) => e.id === picked && (!camp || (e.ally ? "player" : e.kind) === camp)) ??
          view.state.entities.find((e) => (e.ally ? "player" : e.kind) === camp)
        if (entity) act({ type: "place", entity: entity.id, cell })
      }
      return
    }
    if (spell !== null) {
      const target = aim?.targets.find((candidate) => candidate.cell === cell)
      if (target) act({ type: "cast", spell, cell }, target.zone)
      return
    }
    if (view.steps.some((step) => step.cell === cell)) act({ type: "move", cell })
  }

  const stateBoard = view?.state.board
  const groups = spellGroups(view?.spells ?? [])
  const deck = groups.map(([base, slots]) => slots.find((slot) => slot.id === (variants[base] ?? base)) ?? slots[0]!)
  const weapon = view?.spells.find((slot) => slot.weapon)
  function endTurn() {
    if (!view || action.isPending || start.isPlaceholderData) return
    playback.skip()
    setSpell(null)
    setPicked(null)
    setPeek(null)
    act({ type: view.state.phase === "fight" ? "end" : "begin" })
  }
  const onShortcut = React.useEffectEvent((event: KeyboardEvent) => {
    if (!gameRef.current?.getClientRects().length || event.repeat || event.isComposing || event.metaKey || event.altKey) return
    if (
      event.target instanceof HTMLElement &&
      event.target.closest("input,select,textarea,[contenteditable=true],[role=dialog],[data-slot=popover-content]")
    )
      return
    if (panel || document.querySelector('[role="dialog"], [data-slot="popover-content"][data-open]')) return
    if (event.code === "Escape") {
      setSpell(null)
      setPicked(null)
      setPeek(null)
      setShowTip(false)
      return
    }
    const pos = keys.indexOf(event.code)
    if (pos < 0 || pos === 18 || leaving) return
    if (pos >= 12 && (event.ctrlKey || event.shiftKey)) return
    event.preventDefault()
    if (pos < 12) {
      const slot = deck[pos + (event.ctrlKey || event.shiftKey ? 12 : 0)]
      if (slot) toggleSpell(slot.id)
    } else if (pos === 12 && weapon) toggleSpell(weapon.id)
    else if (pos === 13) endTurn()
    else if (pos >= 14) setPanel((["stats", "spells", "gear", "settings"] as const)[pos - 14] ?? null)
  })
  React.useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      if (event.key === "Alt") setShowTip(true)
      if (
        event.code === keys[18] &&
        !event.ctrlKey &&
        !(event.target instanceof HTMLElement && event.target.closest("input,textarea,[role=dialog]"))
      )
        setHighlightMarks(true)
      onShortcut(event)
    }
    const clear = () => {
      setShowTip(false)
      setHighlightMarks(false)
    }
    const keyup = (event: KeyboardEvent) => {
      if (event.key === "Alt") setShowTip(false)
      if (event.code === keys[18]) setHighlightMarks(false)
    }
    const sync = () => setFullscreen(Boolean(document.fullscreenElement))
    window.addEventListener("keydown", keydown)
    window.addEventListener("keyup", keyup)
    window.addEventListener("blur", clear)
    document.addEventListener("fullscreenchange", sync)
    return () => {
      window.removeEventListener("keydown", keydown)
      window.removeEventListener("keyup", keyup)
      window.removeEventListener("blur", clear)
      document.removeEventListener("fullscreenchange", sync)
    }
  }, [keys])
  function changeMap(id: number) {
    setMapChoice(id)
    restart()
    setPanel(null)
  }
  if (!view)
    return (
      <div className="fight-screen combat-loading" ref={gameRef}>
        <p role={start.isError ? "alert" : "status"}>{t(start.isError ? "fight.failed" : "fight.loading")}</p>
        <button onClick={returnToBuild}>{t("fight.back")}</button>
      </div>
    )
  const { state } = view
  const fighting = state.phase === "fight"
  const player = state.entities.find((e) => e.id === state.activeId) ?? state.entities.find((e) => e.kind === "player")!
  const shownSlot = view.spells.find((slot) => slot.id === (peek ?? spell))
  const hovered = entityAt(state.entities, cursor)
  const pinnedEntity = state.entities.find((e) => e.id === pinned)
  const shownEntity = (panel === "entity" ? pinnedEntity : hovered) ?? hovered ?? pinnedEntity ?? player
  const hpOf = (entity: FightEntity) => playback.hp?.[entity.id] ?? entity.hp
  const art = Boolean(state.board.id)
  const hud = (id: string) => api.imageUrl(`/fight-assets/hud/${id}.png`)
  const imageStyle = (id: string) => ({ backgroundImage: `url("${hud(id)}")` })
  const menuTitle =
    panel === "chat"
      ? t("fight.chat")
      : panel === "settings"
        ? t("fight.settings")
        : panel === "stats"
          ? t("build.stats")
          : panel === "spells"
            ? t("fight.spells")
            : panel === "gear"
              ? t("fight.inventory.title")
              : nameOf(shownEntity)
  const tool = (id: "stats" | "spells" | "gear", asset: string, label: string, icon: React.ReactNode) => (
    <button title={label} aria-label={label} onClick={() => setPanel(id)}>
      {art ? <img src={hud(asset)} alt="" /> : icon}
    </button>
  )
  const button = (slot: FightSlot, index: number) => (
    <SpellButton
      key={slot.id}
      slot={slot}
      selected={spell === slot.id}
      onSelect={toggleSpell}
      onPeek={setPeek}
      shortcut={slot.weapon ? keyLabel(keys[12]!) : index < 24 ? `${index >= 12 ? "⇧ " : ""}${keyLabel(keys[index % 12]!)}` : ""}
    />
  )
  return (
    <LightAnimations.Provider value={prefs.light}>
      <div
        className="fight-screen"
        ref={gameRef}
        data-art={art || undefined}
        data-exiting={leaving || undefined}
        data-rows={prefs.rows}
        style={
          {
            ...(!prefs.autoScale && { "--hud-unit": prefs.scale / 100 + "px" }),
            "--chat-size": prefs.chatSize,
            "--chat-leading": prefs.chatSpacing / 100,
            "--chat-opacity": prefs.chatOpacity / 100,
          } as React.CSSProperties
        }
      >
        {art && (
          <style>{`@font-face{font-family:CombatLexend;src:url("${api.imageUrl("/fight-assets/hud/lexend-regular.ttf")}")}@font-face{font-family:CombatLexend;src:url("${api.imageUrl("/fight-assets/hud/lexend-bold.ttf")}");font-weight:700}@font-face{font-family:CombatRowdies;src:url("${api.imageUrl("/fight-assets/hud/rowdies-regular.ttf")}")}`}</style>
        )}
        <header className="combat-top">
          <div className="combat-summary">
            <b>{build.name}</b>
            {fighting && (
              <>
                <span>{t("fight.turn", { turn: state.turn })}</span>
                <span>{t("fight.crit", { n: view.currentStats?.["18"] ?? 0 })}</span>
                <span>{t("fight.damage", { turn: number(state.turnDamage), total: number(state.totalDamage) })}</span>
              </>
            )}
          </div>
          <nav aria-label={t("build.view.fight")}>
            <button className="compact-chat" onClick={() => setPanel("chat")}>
              {t("fight.chat")}
            </button>
            {tool("stats", "characteristic", t("build.stats"), <UserRoundIcon />)}
            {tool("spells", "spells", t("fight.spells"), <BookOpenIcon />)}
            {tool("gear", "inventory", t("fight.inventory.title"), <BackpackIcon />)}
            <DummySettings
              dummies={dummies}
              onChange={(next) => {
                actionEpoch.current++
                setDummies(next)
                setSpell(null)
                setPicked(null)
                playback.reset()
              }}
            />
            <button title={t("fight.restart")} onClick={restart}>
              <RotateCcwIcon />
              <span>{t("fight.restart")}</span>
            </button>
            <button title={t("fight.back")} onClick={returnToBuild}>
              <ArrowLeftIcon />
              <span>{t("fight.back")}</span>
            </button>
            <button
              aria-label={t("fight.fullscreen")}
              aria-pressed={fullscreen}
              onClick={() => void toggleFullscreen().catch(() => {})}
            >
              <MaximizeIcon />
            </button>
            <button aria-label={t("fight.settings")} title={t("fight.settings")} onClick={() => setPanel("settings")}>
              <SettingsIcon />
            </button>
          </nav>
        </header>
        <div className="fight-stage combat-stage">
          <FightBoardView
            showReach={prefs.showReach}
            leaving={leaving}
            highlightMarks={highlightMarks}
            view={view}
            aim={aim}
            spell={spell}
            cursor={cursor}
            picked={picked}
            pointed={hovered?.id ?? pinned}
            breedId={build.breedId}
            items={input.items}
            playback={playback}
            busy={action.isPending || start.isPlaceholderData}
            nameOf={nameOf}
            onCursor={setCursor}
            onAct={onCell}
            onCancel={() => {
              setSpell(null)
              setPicked(null)
            }}
          />
          <ol className="fight-order combat-order" aria-label={t("fight.order")}>
            <li className="combat-round">
              <svg width="16" height="17" viewBox="0 0 16 18" aria-hidden="true">
                <path d="M6 1h4M8 2v2m5 0-1.5 1.5" fill="none" stroke="currentColor" strokeWidth="1.6" />
                <circle cx="8" cy="10" r="5.3" fill="none" stroke="currentColor" strokeWidth="1.5" />
                <path d="M8 6v4l2 1" fill="none" stroke="currentColor" strokeWidth="1.4" />
              </svg>
              {state.turn || 1}
            </li>
            {[...state.entities]
              .sort((a, b) => Number(a.kind === "dummy" && !a.ally) - Number(b.kind === "dummy" && !b.ally))
              .map((entity) => (
                <li key={entity.id}>
                  <button
                    type="button"
                    className="cell fight-order-cell"
                    title={nameOf(entity)}
                    aria-label={`${nameOf(entity)}, ${number(hpOf(entity))} / ${number(entity.maxHp)} PV`}
                    data-kind={entity.ally ? "player" : entity.kind}
                    aria-current={(fighting && entity.id === player.id) || undefined}
                    onClick={() => {
                      setPinned(entity.id)
                      setPanel("entity")
                    }}
                    onMouseEnter={() => setCursor(entity.cell)}
                    onMouseLeave={() => setCursor(null)}
                  >
                    {art ? (
                      <svg
                        viewBox={
                          entity.kind === "player"
                            ? "-15 -58 30 33"
                            : entity.kind === "summon"
                              ? "-14 -28 28 29"
                              : "-20 -66 40 57"
                        }
                        aria-hidden="true"
                      >
                        <GameActor
                          kind={entity.kind}
                          breedId={build.breedId}
                          items={input.items}
                          monsterId={entity.monsterId}
                          direction={entity.kind === "player" ? 3 : 1}
                          portrait
                          placement={!fighting}
                          pose="idle"
                        />
                      </svg>
                    ) : (
                      <Portrait entity={entity} breedId={build.breedId} />
                    )}
                    <span className="fight-order-hp">
                      <span style={{ transform: `scaleY(${Math.max(0, Math.min(1, hpOf(entity) / entity.maxHp))})` }} />
                    </span>
                  </button>
                </li>
              ))}
          </ol>
          {state.entities.some((e) => e.kind === "summon") && (
            <aside className="combat-summons">
              <h3>{stats.get(26)?.label ?? t("fight.actors")}</h3>
              {state.entities
                .filter((e) => e.kind === "summon")
                .map((e) => (
                  <button
                    key={e.id}
                    onClick={() => {
                      setPinned(e.id)
                      setPanel("entity")
                    }}
                  >
                    <svg viewBox="-14 -28 28 29" aria-hidden="true">
                      <GameActor portrait kind="summon" monsterId={e.monsterId} pose="idle" direction={1} />
                    </svg>
                    <span>
                      <b>{nameOf(e)}</b>
                      <small>
                        {number(hpOf(e))} ♥　{view.state.activeId === e.id ? view.ap : (e.stats?.["1"] ?? 0)} PA　
                        {view.state.activeId === e.id ? view.mp : (e.stats?.["23"] ?? 0)} PM
                      </small>
                    </span>
                  </button>
                ))}
            </aside>
          )}
          {!fighting && (
            <div className="combat-placement">
              <span>{t("fight.placementHint")}</span>
              {maps?.length ? (
                <select aria-label={t("fight.arena")} value={mapId} onChange={(event) => changeMap(Number(event.target.value))}>
                  {maps.map((id) => (
                    <option key={id} value={id}>
                      {t(id === 99222029 ? "fight.village" : "fight.astrub")}
                    </option>
                  ))}
                </select>
              ) : (
                <button onClick={() => setPanel("settings")}>{t("fight.import")}</button>
              )}
            </div>
          )}
          {((showTip && shownSlot) || refused || action.isError) && (
            <div className="combat-tooltip" role="status">
              {showTip && shownSlot && <SpellSheet slot={shownSlot} />}
              {refused && t(`fight.blocked.${refused}` as "fight.blocked.ap")}
              {action.isError && t("fight.failed")}
            </div>
          )}
        </div>
        <footer className="combat-dock">
          <section className="combat-chat">
            <h3>
              {t("fight.chat")}
              <button title={t("fight.chat.options")} aria-label={t("fight.chat.options")} onClick={() => setPanel("chat")}>
                ⋮
              </button>
            </h3>
            <ol className="fight-log" ref={chatContent} aria-live="polite">
              <FightJournal log={log} stats={stats} fold={prefs.foldEffects} />
              <li ref={logEnd} aria-hidden="true" />
            </ol>
          </section>
          <div className="combat-vitals">
            <div className="combat-heart" title={t("fight.entity.hp", { hp: number(hpOf(player)), max: number(player.maxHp) })}>
              {art && (
                <>
                  <img src={hud("heart-background")} alt="" />
                  <img
                    src={hud("heart-fill")}
                    alt=""
                    style={{ clipPath: `inset(${100 - Math.max(0, hpOf(player) / player.maxHp) * 100}% 0 0)` }}
                  />
                  <img src={hud("heart-border")} alt="" />
                </>
              )}
              <strong>{hpOf(player)}</strong>
            </div>
            <div className="combat-points">
              <span title={stats.get(1)?.label} style={art ? imageStyle("ap") : undefined}>
                {view.ap}
              </span>
              <span title={stats.get(23)?.label} style={art ? imageStyle("mp") : undefined}>
                {view.mp}
              </span>
            </div>
          </div>
          <section className="combat-spells" aria-label={t("fight.spells")}>
            <h3>{t("fight.spells")}</h3>
            <div className="combat-spellgrid">
              <div className="combat-weapon">{weapon && button(weapon, -1)}</div>
              <div className="combat-deck">{deck.map(button)}</div>
            </div>
          </section>
          <div className="combat-endbox">
            <button
              className="combat-end"
              disabled={action.isPending || start.isPlaceholderData}
              onClick={endTurn}
              title={keyLabel(keys[13]!)}
              style={
                art
                  ? ({
                      "--end-mask": `url("${hud("end-idle")}")`,
                      "--end-bg": `url("${hud("end-background")}")`,
                    } as React.CSSProperties)
                  : undefined
              }
            >
              <span>{t(fighting ? "fight.endTurn" : "fight.ready")}</span>
            </button>
          </div>
        </footer>
        <Dialog
          open={panel !== null}
          onOpenChange={(open) => {
            if (!open) setPanel(null)
          }}
        >
          <DialogBackdrop className={panel === "gear" ? "combat-inventory-backdrop" : undefined} />
          <DialogContent
            className={panel === "gear" ? "combat-dialog combat-inventory-window" : "combat-dialog"}
            style={
              panel === "gear" && art
                ? ({
                    "--inventory-header": `url("${hud("inventory-header")}")`,
                    "--inventory-highlight": `url("${hud("inventory-highlight")}")`,
                  } as React.CSSProperties)
                : undefined
            }
          >
            <header>
              <DialogTitle>
                {panel === "gear" && art && <img src={hud("inventory")} alt="" />}
                {menuTitle}
              </DialogTitle>
              <button aria-label={t("about.close")} onClick={() => setPanel(null)}>
                <XIcon />
              </button>
            </header>
            {panel === "settings" && (
              <FightOptions
                prefs={prefs}
                update={update}
                audio={audio}
                maps={maps ?? []}
                map={mapId}
                onMap={changeMap}
                keys={keys}
                onKeys={setKeys}
                onImported={async () => {
                  await cache.invalidateQueries({ queryKey: ["fight-actors"] })
                  return assetQuery.refetch()
                }}
              />
            )}
            {panel === "chat" && (
              <>
                <ChatOptions
                  prefs={prefs}
                  update={update}
                  copy={async () => {
                    const nodes = chatContent.current?.querySelectorAll("summary,li:not(:has(details))")
                    await navigator.clipboard.writeText(
                      Array.from(nodes ?? [], (node) => node.textContent?.trim())
                        .filter(Boolean)
                        .join("\n")
                    )
                  }}
                />
                <ol
                  className="fight-log combat-popup-log"
                  style={{
                    fontSize: prefs.chatSize + "px",
                    lineHeight: prefs.chatSpacing / 100,
                    backgroundColor: `rgb(25 24 23 / ${prefs.chatOpacity / 100})`,
                  }}
                >
                  <FightJournal log={log} stats={stats} fold={prefs.foldEffects} />
                </ol>
              </>
            )}
            {panel === "stats" && (
              <StatsPanel
                meta={meta}
                evaluation={evaluation && view.currentStats ? { ...evaluation, stats: view.currentStats } : evaluation}
              />
            )}
            {panel === "entity" && (
              <EntitySheet entity={shownEntity} name={nameOf(shownEntity)} names={view.names} stats={stats} />
            )}
            {panel === "gear" && (
              <CombatInventory
                meta={meta}
                build={build}
                original={original}
                active={input}
                draft={draft}
                activeEvaluation={evaluation}
                art={art}
                busy={action.isPending}
                onDraft={setDraft}
                onRestart={() => {
                  if (pendingAction.current) return
                  setInput(draft)
                  setPanel(null)
                  restart()
                }}
              />
            )}
            {panel === "spells" &&
              (player.kind === "summon" ? (
                <div className="combat-book">
                  {deck.map((slot) => (
                    <div key={slot.id}>
                      <SpellSheet slot={slot} />
                    </div>
                  ))}
                </div>
              ) : (
                <FightSpellBook
                  breedId={build.breedId ?? 0}
                  breedName={breedName ?? t("fight.book.class")}
                  level={build.level}
                  slots={view.spells}
                  selected={new Set(deck.map((s) => s.id))}
                  pack={assetQuery.data?.pack}
                  onSelect={(slot) => {
                    setVariants({ ...variants, [slot.variantOf || slot.id]: slot.id })
                    setPeek(slot.id)
                    setSpell(null)
                  }}
                  sheet={(slot) => <SpellSheet slot={slot} />}
                />
              ))}
          </DialogContent>
        </Dialog>
      </div>
    </LightAnimations.Provider>
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
