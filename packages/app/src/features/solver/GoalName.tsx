import { useItem } from "../../api/queries"
import type { Meta } from "../../api/types"

export function ItemName({ id }: { id: number }) {
  return <>{useItem(id).data?.name ?? "…"}</>
}

// The weapon or spell a damage goal is about.
export function GoalName({ meta, goal }: { meta: Meta; goal: { kind: "weapon" | "spell"; id: number } }) {
  if (goal.kind === "weapon") return <ItemName id={goal.id} />
  return <>{meta.spells.find((spell) => spell.id === goal.id)?.name ?? "…"}</>
}
