export type Lang = "fr" | "en" | "es" | "pt"

export type Slot =
  | "amulet"
  | "weapon"
  | "ring"
  | "belt"
  | "boots"
  | "shield"
  | "hat"
  | "cloak"
  | "pet"
  | "dofus"

export interface Position {
  key: string
  slot: Slot
}

export interface Stat {
  id: number
  key: string
  label: string
  unit: string
  categoryId: number
  /** Highest value forgemagic can put on a line of this stat; 0 if it cannot be forged. */
  forgeMax: number
}

export type Element = "neutral" | "earth" | "fire" | "water" | "air"

export type DracoKey = "turquoise" | "emeraude" | "pourpre" | "ocre" | "ivoire" | "ebene"

export interface TranscendenceRune {
  id: number
  name: string
  statId: number
  value: number
  /** The weight the rune counts for on its line: 40, 60 or 80. */
  density: number
  text: string
}

export interface DracoRune {
  key: DracoKey
  kind: "element" | "resistance" | "power"
  target?: Element
}

export interface ForgeMeta {
  slots: Slot[]
  limitedExos: number[]
  runes: TranscendenceRune[]
  dracoRunes: DracoRune[]
  elements: Element[]
  elementIds: Record<Element, { statId: number; damageId: number; resFixedId: number; resPercentId: number }>
}

export interface Exo {
  statId: number
  value: number
}

export interface Draco {
  rune: DracoKey
  from?: Element
  to?: Element
}

/** The forgemagic a player puts on one worn item. */
export interface ItemMods {
  lines?: Record<string, number>
  exos?: Exo[]
  transcendence?: number
  draco?: Draco
  /** A weapon's elemental forgemagic: its Neutral damage hits in this element. */
  element?: WeaponElement
}

export type WeaponElement = "earth" | "fire" | "water" | "air"

export type LineOrigin = "base" | "over" | "exo" | "transcendence" | "draco"

export interface ForgedLine {
  statId: number
  min: number
  max: number
  cap: number
  value: number
  origin: LineOrigin
  /** The part of `value` that comes from the transcendence rune. */
  rune: number
  natural: boolean
  text: string
}

export interface Passive {
  name: string
  description: string
}

export interface StatCategory {
  id: number
  name: string
  statIds: number[]
}

export interface ItemType {
  id: number
  name: string
  slot: Slot
}

export interface Meta {
  version: string
  languages: string[]
  maxLevel: number
  slots: Slot[]
  positions: Position[]
  categories: StatCategory[]
  stats: Stat[]
  types: ItemType[]
  primaryStats: { key: PrimaryKey; statId: number }[]
  itemCount: number
  setCount: number
  forge: ForgeMeta
  /** Character classes, in the game's order. */
  breeds: Breed[]
  /** Every spell a character can cast: class by class, then the common ones (breed 0). */
  spells: SpellInfo[]
  /** Prysmaradites only fit one position, so a character wears at most one. */
  prysmaradite: { typeId: number; position: string }
}

export type PrimaryKey = "vitality" | "wisdom" | "strength" | "intelligence" | "chance" | "agility"

export interface Effect {
  kind: "stat" | "weapon"
  statId: number | null
  element: number | null
  min: number
  max: number
  text: string
}

export interface WeaponInfo {
  apCost: number
  rangeMin: number
  rangeMax: number
  critChance: number
  critBonus: number
  maxCastPerTurn: number
}

export interface Item {
  id: number
  name: string
  level: number
  typeId: number
  typeName: string
  slot: Slot
  image: string
  set: { id: number; name: string } | null
  effects: Effect[]
  condition: string
  weapon: WeaponInfo | null
  passive: Passive | null
  forgeable: boolean
  description?: string
}

export interface EffectLine {
  statId: number
  value: number
  text: string
}

export interface SetDetail {
  id: number
  name: string
  level: number
  items: Item[]
  bonuses: { count: number; lines: EffectLine[] }[]
}

export interface ItemDetail extends Item {
  setDetail: SetDetail | null
}

export interface ItemList {
  items: Item[]
  total: number
}

export interface StatFilter {
  statId: number
  op: "gte" | "lte"
  value: number
}

export interface ItemQuery {
  q?: string
  slots?: Slot[]
  types?: number[]
  excludeTypes?: number[]
  levelMin?: number
  levelMax?: number
  stats?: StatFilter[]
  passive?: boolean
  sort?: string
  order?: "asc" | "desc"
}

export interface Character {
  level: number
  base: Record<PrimaryKey, number>
  scrolls: Record<PrimaryKey, number>
}

export interface Breed {
  id: number
  name: string
}

export interface SpellInfo {
  id: number
  name: string
  /** The class the spell belongs to; 0 for a common spell. */
  breedId: number
  /** The spell this one replaces, or 0. */
  variantOf: number
  /** The spell deals direct damage at some grade. */
  damage: boolean
  /** The elements it hits in at once, at some grade; "best" follows the character's best element. */
  elements: (WeaponElement | "neutral" | "best" | "worst")[]
}

/** 0 neutral, 1 earth, 2 fire, 3 water, 4 air. */
export type ElementId = 0 | 1 | 2 | 3 | 4

export interface Hit {
  min: number
  max: number
  critMin: number
  critMax: number
}

/** Who a line of an attack applies to. Lower-case names are on the caster's side. */
export type AttackTarget =
  | "ally"
  | "enemy"
  | "allyOther"
  | "caster"
  | "allyPlayer"
  | "enemyPlayer"
  | "allyMonster"
  | "enemyMonster"
  | "allyCompanion"
  | "enemyCompanion"
  | "allyPlayerCompanion"
  | "enemyPlayerCompanion"
  | "allySummon"
  | "enemySummon"

export interface AttackLine extends Hit {
  element: ElementId
  /** The line follows the highest elemental characteristic, or the lowest one. */
  best: boolean
  worst?: boolean
  steal: boolean
  /** Turns before the line applies. */
  delay: number
  /** Damage when left out; a heal that scales with a characteristic, a heal in % of health points, or a pushback. */
  kind?: "heal" | "healPercent" | "push"
  /** Who the line applies to; empty for a weapon. */
  targets: AttackTarget[]
  /** The line applies later, when a condition is met. */
  conditional?: boolean
  /** When the line applies, if not as the spell is cast. */
  trigger?: "turnStart" | "turnEnd"
  /** The line is part of the attack's total: damage dealt to an enemy as the attack is made. */
  counted: boolean
  /** Cells a pushback pushes, on a normal and on a critical hit. */
  cells?: number
  critCells?: number
  /** What a steal gives back to the caster. */
  regen?: Hit
  /** The line for each MP the caster has used this turn, from one up, when it grows with them. */
  steps?: Hit[]
}

export interface Attack {
  /** The item id of a weapon or the spell id. */
  id: number
  name: string
  grade: number
  apCost: number
  rangeMin: number
  rangeMax: number
  critChance: number
  melee: boolean
  lines: AttackLine[]
  /** The counted lines, summed: what an enemy takes as the attack is made. */
  total: Hit
  /** Chance of a critical hit with this stuff, 0 to 100, and the average hit at that rate. */
  critRate: number
  mean: number
  breedId: number
  variantOf: number
}

export type AttackRange = "" | "melee" | "ranged"

export interface DamageResult {
  weapon: Attack | null
  spells: Attack[]
}

/** What a damage goal reads: a normal hit, a critical hit, or the average hit at the stuff's critical rate. */
export type DamageMeasure = "normal" | "crit" | "mean"

export interface DamageGoal {
  kind: "weapon" | "spell"
  id: number
  measure: DamageMeasure
  /** The least the measure must reach. */
  value: number
  /** A weapon goal's elemental forgemagic, if any. */
  element?: WeaponElement
}

/**
 * How the elements of a goal about several count together: the first one as
 * high as possible while each other keeps up with its share of it ("floor"),
 * or their average weighted by their shares ("sum").
 */
export type ElementMix = "floor" | "sum"

/** What an element of a goal is read on, instead of every spell of the class: one spell, or the weapon. */
export interface ElementTarget {
  kind: "spell" | "weapon"
  id: number
}

/** An element of a goal about several, with its share from 1 to 100. */
export interface ElementShare {
  element: WeaponElement
  share: number
  target?: ElementTarget
}

/** An element of a goal about several, as the search receives it. */
export interface ElementPart {
  element: WeaponElement
  share: number
  kind?: "spell" | "weapon"
  id?: number
}

/** The damage of the class's spells in an element, or in several at once. */
export interface ElementEntry {
  kind: "element"
  /** The first element; the search picks it itself when it is "auto". */
  element: WeaponElement | "auto"
  measure: DamageMeasure
  /** The elements pushed along with the first; none for a single element. */
  others?: ElementShare[]
  /** "floor" when left out. */
  mix?: ElementMix
  /** The first element's weight in a sum; 100 when left out. */
  share?: number
  /** What the first element is read on, when there are several. */
  target?: ElementTarget
}

/**
 * One thing the search pushes up: a stat, what a weapon or a spell hits for,
 * or the damage of the class's spells in one or several elements.
 */
export type MaximizeEntry =
  | { kind: "stat"; statId: number }
  | { kind: "weapon" | "spell"; id: number; measure: DamageMeasure; element?: WeaponElement }
  | ElementEntry

/** An attack to maximize, as the search receives it. */
export interface MaximizedAttack {
  kind: "weapon" | "spell" | "element" | "elements"
  id: number
  measure: DamageMeasure
  element?: WeaponElement
  /** The elements of an "elements" attack, the main one first, and how they count together. */
  parts?: ElementPart[]
  mix?: ElementMix
}

export interface BuildInput extends Character {
  items: Record<string, number>
  mods: Record<string, ItemMods>
}

export interface SetStatus {
  id: number
  name: string
  count: number
  size: number
  bonuses: EffectLine[]
}

export interface ConditionStatus {
  position: string
  itemId: number
  text: string
  ok: boolean
}

export interface Issue {
  code:
    | "unknown_item"
    | "wrong_slot"
    | "level"
    | "duplicate"
    | "prysmaradite_slot"
    | "points"
    | "forge_slot"
    | "forge_line"
    | "forge_exo"
    | "forge_weight"
    | "forge_transcendence"
    | "forge_draco"
  position?: string
  itemId?: number
}

export interface Evaluation {
  level: number
  stats: Record<string, number>
  forge: Record<string, ForgedLine[]>
  /** Per position, the transcendence runes the item can take as it stands. */
  runes: Record<string, number[]>
  /** Per position, the weight the item's overs and exos use out of the 101 they share. */
  forgeWeight: Record<string, number>
  /** Stats whose sum went over the game's cap: `raw - max` points are lost. */
  /** AP, MP and Range exos that do not count: a character counts one of each. */
  ignoredExos: { position: string; itemId: number; statId: number; value: number }[]
  capped: { statId: number; max: number; raw: number }[]
  sets: SetStatus[]
  setBonuses: number
  conditions: ConditionStatus[]
  issues: Issue[]
  points: { spent: number; total: number }
  valid: boolean
  items: Record<string, Item>
}

export interface SolverRequest {
  character: Character
  constraints: StatFilter[]
  /** By priority. An entry with `damage` is about an attack instead of a stat. */
  maximize: { statId: number; weight: number; damage?: MaximizedAttack }[]
  /** The class of the character; 0 when the stuff has none. */
  breedId: number
  /** Attacks that must hit for at least a value, on the average roll. */
  damage: DamageGoal[]
  locked: Record<string, number>
  /** Items every stuff must wear, wherever they fit. */
  required: number[]
  excluded: number[]
  maxResults: number
  forge: { exos: number[]; transcendence: boolean; draco: boolean; raiseCritical: boolean }
  /** Let the solver spend the characteristic points itself. */
  autoPoints: boolean
}

export interface Solution {
  /** The character the stuff was computed for, with the points the solver spent. */
  character: Character
  items: Record<string, number>
  mods?: Record<string, ItemMods>
  score: number
  feasible: boolean
  unmet: number[]
  /** Indexes of the damage goals the stuff misses, and what each goal measures. */
  unmetDamage: number[]
  damage: number[]
  /** For an attack about several elements, what each of them measures; null for the others. */
  damageParts?: (number[] | null)[]
  /** The element the stuff was searched for, when the search chose it. */
  element?: WeaponElement
  evaluation: Evaluation
}

export interface SolverResponse {
  /** Names these results on the server, to ask for their next pages. */
  id: string
  /** The first page of stuffs; the others come from `results`. */
  solutions: Solution[]
  /** How many stuffs the search lists, and how many of them meet every condition. */
  total: number
  feasible: number
  explored: number
  candidates: number
  elapsedMs: number
  /** Required items left out: every place they fit held another required item. */
  unplaced: number[]
}

export interface SolverPage {
  solutions: Solution[]
  total: number
}

export interface DataStatus {
  state: "idle" | "checking" | "updating"
  ready: boolean
  installedVersion: string
  importedAt: string
  latestVersion: string
  latestPublishedAt: string
  lastCheckedAt: string
  updateAvailable: boolean
  autoUpdate: boolean
  progress: { step: "download" | "parse" | "import" | "images"; percent: number; target: string } | null
  error: string
}
