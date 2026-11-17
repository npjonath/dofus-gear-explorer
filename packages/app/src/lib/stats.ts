import type { Meta, Stat } from "../api/types"

export type Tone =
  | "earth"
  | "fire"
  | "water"
  | "air"
  | "neutral"
  | "vitality"
  | "wisdom"
  | "ap"
  | "mp"
  | "range"
  | "critical"
  | "prospecting"
  | "plain"

const exact: Record<string, Tone> = {
  strength: "earth",
  intelligence: "fire",
  chance: "water",
  agility: "air",
  vitality: "vitality",
  hitPoints: "vitality",
  wisdom: "wisdom",
  actionPoints: "ap",
  movementPoints: "mp",
  range: "range",
  criticalHit: "critical",
  magicFind: "prospecting",
}

const prefixes: [string, Tone][] = [
  ["earth", "earth"],
  ["fire", "fire"],
  ["water", "water"],
  ["air", "air"],
  ["neutral", "neutral"],
]

// The colour a stat is read by in the game: its element, or its own for AP, MP…
export function toneOf(key: string | undefined): Tone {
  if (!key) return "plain"
  const direct = exact[key]
  if (direct) return direct
  return prefixes.find(([prefix]) => key.startsWith(prefix))?.[1] ?? "plain"
}

const elementTones: Tone[] = ["neutral", "earth", "fire", "water", "air"]

export function toneOfElement(element: number | null): Tone {
  return element === null ? "plain" : (elementTones[element] ?? "plain")
}

export function statIndex(meta: Meta | undefined): Map<number, Stat> {
  return new Map(meta?.stats.map((s) => [s.id, s]))
}

export function statLabel(stat: Stat | undefined): string {
  if (!stat) return ""
  return stat.unit ? `${stat.label} (${stat.unit})` : stat.label
}

export function formatValue(value: number, stat: Stat | undefined): string {
  return `${value}${stat?.unit ?? ""}`
}

export const HP_STAT = 0
export const AP_STAT = 1
export const MP_STAT = 23
export const RANGE_STAT = 19
