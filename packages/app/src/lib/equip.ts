import type { Item, ItemMods, Meta } from "../api/types"
import type { Build } from "../store"

// Where an item goes when equipped without choosing a position: where it
// already is, else a free place of its kind (the preferred one first), else
// the preferred or first place, whose item it replaces.
export function targetPosition(
  meta: Meta,
  build: Build,
  item: Pick<Item, "slot" | "id" | "typeId">,
  preferred?: string
): string {
  // A prysmaradite has one place only; it takes it even when it is occupied.
  if (item.typeId === meta.prysmaradite.typeId) return meta.prysmaradite.position
  const places = meta.positions.filter((p) => p.slot === item.slot)
  const already = places.find((p) => build.items[p.key] === item.id)
  if (already) return already.key
  const wanted = places.find((p) => p.key === preferred)
  if (wanted && !build.items[wanted.key]) return wanted.key
  return (places.find((p) => !build.items[p.key]) ?? wanted ?? places[0]!).key
}

// Forgemagic as text that only depends on what was forged, not on the order
// it was written in.
export function modsKey(mods: ItemMods | undefined): string {
  if (!mods) return ""
  const lines = Object.entries(mods.lines ?? {}).sort(([a], [b]) => a.localeCompare(b))
  const exos = (mods.exos ?? []).map((exo) => [exo.statId, exo.value]).sort((a, b) => a[0]! - b[0]! || a[1]! - b[1]!)
  const draco = mods.draco ? [mods.draco.rune, mods.draco.from ?? "", mods.draco.to ?? ""] : null
  return JSON.stringify([lines, exos, mods.transcendence ?? 0, draco, mods.element ?? ""])
}
