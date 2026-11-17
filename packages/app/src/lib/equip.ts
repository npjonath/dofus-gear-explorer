import type { Item, Meta } from "../api/types"
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
