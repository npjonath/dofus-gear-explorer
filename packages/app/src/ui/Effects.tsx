import type { Effect, Stat } from "../api/types"
import { toneOf, toneOfElement } from "../lib/stats"

export function Effects({ effects, stats }: { effects: Effect[]; stats: Map<number, Stat> }) {
  // A weapon's attack lines lead, as they do in the game's tooltips.
  const ordered = [...effects].sort((a, b) => Number(b.kind === "weapon") - Number(a.kind === "weapon"))
  return (
    <ul className="effects">
      {ordered.map((effect, index) => {
        const tone =
          effect.kind === "weapon"
            ? toneOfElement(effect.element)
            : toneOf(effect.statId === null ? undefined : stats.get(effect.statId)?.key)
        return (
          <li
            key={`${index}-${effect.text}`}
            data-kind={effect.kind}
            data-negative={effect.kind === "stat" && effect.max < 0 ? "" : undefined}
          >
            <span className="pip" data-tone={tone} />
            <span>{effect.text}</span>
          </li>
        )
      })}
    </ul>
  )
}
