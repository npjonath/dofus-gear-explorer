import * as React from "react"
import type { Item } from "../api/types"
import { useApi } from "../platform"
import { SlotGlyph } from "./SlotGlyph"

export function ItemImage({ item }: { item: Pick<Item, "image" | "slot"> }) {
  const api = useApi()
  const [failed, setFailed] = React.useState(false)
  if (failed) return <SlotGlyph slot={item.slot} />
  return <img src={api.imageUrl(item.image)} alt="" loading="lazy" draggable={false} onError={() => setFailed(true)} />
}

export function ItemCell({ item, size }: { item: Pick<Item, "image" | "slot">; size?: string }) {
  return (
    <span className="cell text-ink-3" style={size ? ({ "--cell": size } as React.CSSProperties) : undefined}>
      <ItemImage item={item} />
    </span>
  )
}
