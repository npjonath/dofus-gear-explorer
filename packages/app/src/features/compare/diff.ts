import type {
  Attack,
  Character,
  DamageMeasure,
  DamageResult,
  Evaluation,
  ForgedLine,
  Item,
  ItemMods,
  LineOrigin,
  Meta,
  PrimaryKey,
  Slot,
  Stat,
  WeaponElement,
} from "../../api/types"
import { isEmptyMods } from "../../store"

// What changes from one stuff to another. Nothing here knows a game rule:
// each stuff comes evaluated by the API, and these functions only set the
// two answers side by side.

export interface StatDelta {
  stat: Stat
  from: number
  to: number
  delta: number
}

export interface StatDiff {
  gains: StatDelta[]
  losses: StatDelta[]
  /** Stats both stuffs have at the same value; a stat neither has is left out. */
  unchanged: StatDelta[]
}

/**
 * The stats of two stuffs, split by which way they move. They come in the
 * game's order, after the ones in `lead`, which are read first.
 */
export function diffStats(meta: Meta, from: Evaluation, to: Evaluation, lead: readonly number[] = []): StatDiff {
  const byId = new Map(meta.stats.map((stat) => [stat.id, stat]))
  const ordered: Stat[] = []
  const seen = new Set<number>()
  const take = (id: number) => {
    const stat = byId.get(id)
    if (!stat || seen.has(id)) return
    seen.add(id)
    ordered.push(stat)
  }
  lead.forEach(take)
  for (const category of meta.categories) category.statIds.forEach(take)
  for (const stat of meta.stats) take(stat.id)

  const diff: StatDiff = { gains: [], losses: [], unchanged: [] }
  for (const stat of ordered) {
    const row = statDelta(stat, from, to)
    if (row.delta > 0) diff.gains.push(row)
    else if (row.delta < 0) diff.losses.push(row)
    else if (row.to !== 0) diff.unchanged.push(row)
  }
  return diff
}

export function statDelta(stat: Stat, from: Evaluation, to: Evaluation): StatDelta {
  const a = from.stats[String(stat.id)] ?? 0
  const b = to.stats[String(stat.id)] ?? 0
  return { stat, from: a, to: b, delta: b - a }
}

/** An item as one stuff wears it. */
export interface Worn {
  position: string
  item: Item
  mods?: ItemMods
  /** The item's lines as they count, forgemagic included. */
  forged: ForgedLine[]
}

export interface LineDelta {
  statId: number
  from: number
  to: number
  /** Where the line's value comes from on the side that holds the most. */
  origin: LineOrigin
  /** The part a transcendence rune adds, on each side. */
  runeFrom: number
  runeTo: number
}

/**
 * - `same`: the same item with the same forgemagic
 * - `forge`: the same item, forged differently
 * - `swap`: another item takes the place
 * - `added` / `removed`: the place was empty on one side
 */
export type SlotChange = "same" | "forge" | "swap" | "added" | "removed"

export interface SlotDiff {
  slot: Slot
  change: SlotChange
  from?: Worn
  to?: Worn
  /** For `forge`: the lines that differ. */
  lines: LineDelta[]
  /** For `forge` on a weapon: its elemental forgemagic on each side, when it differs. */
  element?: { from?: WeaponElement; to?: WeaponElement }
}

/** A stuff, as far as its items go. */
export interface Gear {
  mods: Record<string, ItemMods>
  evaluation: Evaluation
}

function wornAt(gear: Gear, position: string): Worn | undefined {
  const item = gear.evaluation.items[position]
  if (!item) return undefined
  const mods = gear.mods[position]
  return {
    position,
    item,
    mods: isEmptyMods(mods) ? undefined : mods,
    forged: gear.evaluation.forge[position] ?? [],
  }
}

// Forgemagic as text that only depends on what was forged, not on the order
// it was written in.
function modsKey(mods: ItemMods | undefined): string {
  if (!mods) return ""
  const lines = Object.entries(mods.lines ?? {}).sort(([a], [b]) => a.localeCompare(b))
  const exos = (mods.exos ?? []).map((exo) => [exo.statId, exo.value]).sort((a, b) => a[0]! - b[0]! || a[1]! - b[1]!)
  const draco = mods.draco ? [mods.draco.rune, mods.draco.from ?? "", mods.draco.to ?? ""] : null
  return JSON.stringify([lines, exos, mods.transcendence ?? 0, draco, mods.element ?? ""])
}

/** The lines of an item that do not count the same on both sides. */
export function diffLines(from: readonly ForgedLine[], to: readonly ForgedLine[]): LineDelta[] {
  const after = new Map(to.map((line) => [line.statId, line]))
  const before = new Map(from.map((line) => [line.statId, line]))
  const ids = [...from.map((line) => line.statId), ...to.filter((line) => !before.has(line.statId)).map((line) => line.statId)]
  const deltas: LineDelta[] = []
  for (const statId of ids) {
    const a = before.get(statId)
    const b = after.get(statId)
    const delta: LineDelta = {
      statId,
      from: a?.value ?? 0,
      to: b?.value ?? 0,
      origin: (b && b.origin !== "base" ? b : (a ?? b))!.origin,
      runeFrom: a?.rune ?? 0,
      runeTo: b?.rune ?? 0,
    }
    if (delta.from !== delta.to || delta.runeFrom !== delta.runeTo) deltas.push(delta)
  }
  return deltas
}

/**
 * The items of two stuffs, place by place. Places of the same kind are
 * interchangeable (the two rings, the six Dofus), so an item that only moved
 * from one to the other is not a difference.
 */
export function diffItems(meta: Meta, from: Gear, to: Gear): SlotDiff[] {
  const order = new Map(meta.positions.map((position, index) => [position.key, index]))
  const slots: Slot[] = []
  for (const position of meta.positions) if (!slots.includes(position.slot)) slots.push(position.slot)

  const rows: SlotDiff[] = []
  for (const slot of slots) {
    const keys = meta.positions.filter((position) => position.slot === slot).map((position) => position.key)
    const left = keys.map((key) => wornAt(from, key)).filter((worn) => worn !== undefined)
    const right = keys.map((key) => wornAt(to, key)).filter((worn) => worn !== undefined)
    const group: SlotDiff[] = []

    // Pairs each item of the first stuff with the first one left in the second that matches.
    const pair = (matches: (a: Worn, b: Worn) => boolean, change: SlotChange) => {
      for (let i = 0; i < left.length; i++) {
        const a = left[i]!
        const at = right.findIndex((b) => matches(a, b))
        if (at < 0) continue
        const b = right[at]!
        left.splice(i--, 1)
        right.splice(at, 1)
        const row: SlotDiff = { slot, change, from: a, to: b, lines: [] }
        if (change === "forge") {
          row.lines = diffLines(a.forged, b.forged)
          if (a.mods?.element !== b.mods?.element) row.element = { from: a.mods?.element, to: b.mods?.element }
        }
        group.push(row)
      }
    }
    pair((a, b) => a.item.id === b.item.id && modsKey(a.mods) === modsKey(b.mods), "same")
    pair((a, b) => a.item.id === b.item.id, "forge")
    pair(() => true, "swap")
    for (const a of left) group.push({ slot, change: "removed", from: a, lines: [] })
    for (const b of right) group.push({ slot, change: "added", to: b, lines: [] })

    const rank = (row: SlotDiff) => order.get((row.to ?? row.from)!.position) ?? 0
    rows.push(...group.sort((a, b) => rank(a) - rank(b)))
  }
  return rows
}

export interface SetDelta {
  id: number
  name: string
  size: number
  from: number
  to: number
}

/** The sets whose count of worn pieces differs, as long as one side earns a bonus. */
export function diffSets(from: Evaluation, to: Evaluation): SetDelta[] {
  const before = new Map(from.sets.map((set) => [set.id, set]))
  const after = new Map(to.sets.map((set) => [set.id, set]))
  const deltas: SetDelta[] = []
  for (const set of [...from.sets, ...to.sets.filter((other) => !before.has(other.id))]) {
    const a = before.get(set.id)?.count ?? 0
    const b = after.get(set.id)?.count ?? 0
    if (a !== b && Math.max(a, b) > 1) deltas.push({ id: set.id, name: set.name, size: set.size, from: a, to: b })
  }
  return deltas
}

export interface PointDelta {
  key: PrimaryKey
  from: number
  to: number
}

export interface CharacterDiff {
  level: { from: number; to: number } | null
  base: PointDelta[]
  scrolls: PointDelta[]
}

/** What differs in the characters themselves: level, points spent and scrolls. */
export function diffCharacter(meta: Meta, from: Character, to: Character): CharacterDiff {
  const points = (field: "base" | "scrolls") =>
    meta.primaryStats
      .map(({ key }) => ({ key, from: from[field][key] ?? 0, to: to[field][key] ?? 0 }))
      .filter((row) => row.from !== row.to)
  return {
    level: from.level === to.level ? null : { from: from.level, to: to.level },
    base: points("base"),
    scrolls: points("scrolls"),
  }
}

export interface AttackDelta {
  key: string
  from?: Attack
  to?: Attack
}

export interface DamageDiff {
  /** The weapon of each stuff, whichever they are; null when neither holds one. */
  weapon: AttackDelta | null
  classSpells: AttackDelta[]
  commonSpells: AttackDelta[]
}

/** The attacks of two stuffs, spell by spell, in the order the API lists them. */
export function diffDamage(from: DamageResult, to: DamageResult): DamageDiff {
  const before = new Map(from.spells.map((spell) => [spell.id, spell]))
  const after = new Map(to.spells.map((spell) => [spell.id, spell]))
  const spells = [...to.spells, ...from.spells.filter((spell) => !after.has(spell.id))].map(
    (spell): AttackDelta => ({ key: `spell-${spell.id}`, from: before.get(spell.id), to: after.get(spell.id) })
  )
  const isCommon = (row: AttackDelta) => (row.to ?? row.from)!.breedId === 0
  return {
    weapon:
      from.weapon || to.weapon ? { key: "weapon", from: from.weapon ?? undefined, to: to.weapon ?? undefined } : null,
    classSpells: spells.filter((row) => !isCommon(row)),
    commonSpells: spells.filter(isCommon),
  }
}

/**
 * What an attack hits for, read the way the search reads it: the average
 * roll of a normal or of a critical hit, or the average hit at the stuff's
 * critical rate. Null when the attack cannot land a critical hit.
 */
export function hitValue(attack: Attack, measure: DamageMeasure): number | null {
  if (measure === "mean") return attack.mean
  if (measure === "normal") return (attack.total.min + attack.total.max) / 2
  const canCrit = attack.critChance > 0 && attack.lines.some((line) => line.critMax > 0)
  return canCrit ? (attack.total.critMin + attack.total.critMax) / 2 : null
}

/** The change in percent, or null when there is nothing to compare it with. */
export function percentChange(from: number, to: number): number | null {
  if (from <= 0) return null
  return ((to - from) / from) * 100
}
