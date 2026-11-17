import type { ForgedLine, Item, Stat } from "../api/types"
import { useI18n } from "../i18n"
import { Effects } from "./Effects"
import { ForgedLines } from "./ForgedLines"
import { ItemCell } from "./ItemCell"

interface Props {
  item: Item
  stats: Map<number, Stat>
  /** The item's lines after forgemagic; without them the natural lines show. */
  forged?: ForgedLine[]
}

// An item at a glance: who it is, then every line as it counts.
export function ItemSummary({ item, stats, forged }: Props) {
  const { t, colon } = useI18n()
  const weaponLines = item.effects.filter((effect) => effect.kind === "weapon")
  return (
    <>
      <span className="flex items-center gap-2.5">
        <ItemCell item={item} size="2.5rem" />
        <span className="grid min-w-0">
          <span className="truncate font-semibold">{item.name}</span>
          <span className="truncate text-[0.86rem] text-ink-3">
            {item.typeName} · {t("item.level", { level: item.level })}
            {item.set ? ` · ${item.set.name}` : ""}
          </span>
        </span>
      </span>
      {forged ? (
        <>
          {weaponLines.length > 0 && <Effects effects={weaponLines} stats={stats} />}
          <ForgedLines lines={forged} stats={stats} />
        </>
      ) : (
        <Effects effects={item.effects} stats={stats} />
      )}
      {item.condition && <span className="text-ink-2">{item.condition}</span>}
      {item.passive && (
        <span className="text-ink-2">
          <strong className="text-ink">{item.passive.name}</strong>{colon}{item.passive.description}
        </span>
      )}
    </>
  )
}
