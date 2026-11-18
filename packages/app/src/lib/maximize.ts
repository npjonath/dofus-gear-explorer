import type { ElementEntry, MaximizeEntry, MaximizedAttack, WeaponElement } from "../api/types"

export const weaponElements: WeaponElement[] = ["earth", "fire", "water", "air"]

/** The most elements a goal can name. */
export const MAX_ELEMENTS = weaponElements.length

const DEFAULT_SHARE = 70

/** The elements an entry names, the first one included unless the search picks it. */
export function elementsOf(entry: ElementEntry): WeaponElement[] {
  const others = (entry.others ?? []).map((other) => other.element)
  return entry.element === "auto" ? others : [entry.element, ...others]
}

/**
 * Adds an element to an entry: the first one the entry does not name yet. A
 * goal about several elements names them all, so a first element left to the
 * search becomes a named one.
 */
export function addElement(entry: ElementEntry): ElementEntry {
  const free = weaponElements.filter((element) => !elementsOf(entry).includes(element))
  const first = entry.element === "auto" ? free.shift() : entry.element
  const next = free.shift()
  if (!first || !next) return entry
  return { ...entry, element: first, others: [...(entry.others ?? []), { element: next, share: DEFAULT_SHARE }] }
}

/** The attack the search receives for an entry about a weapon, a spell or elements. */
export function maximizedAttack(entry: Exclude<MaximizeEntry, { kind: "stat" }>): MaximizedAttack {
  if (entry.kind !== "element") {
    return { kind: entry.kind, id: entry.id, measure: entry.measure, element: entry.element }
  }
  const others = entry.others ?? []
  if (entry.element === "auto" || others.length === 0) {
    return { kind: "element", id: 0, measure: entry.measure, element: entry.element === "auto" ? undefined : entry.element }
  }
  return {
    kind: "elements",
    id: 0,
    measure: entry.measure,
    mix: entry.mix ?? "floor",
    parts: [{ element: entry.element, share: entry.share ?? 100 }, ...others],
  }
}
