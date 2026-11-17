import { Button } from "@ui-sdk/react/button"
import {
  createHoverCardController,
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@ui-sdk/react/hover-card"
import { PlusIcon } from "lucide-react"
import * as React from "react"
import type { ForgedLine, Item, Stat } from "../api/types"
import { ItemSummary } from "./ItemSummary"

type Side = "top" | "right" | "bottom" | "left"

export interface ItemAction {
  label: string
  /** The main action shows a plus sign; the others are plain. */
  primary?: boolean
  run(): void
}

interface HoverPayload {
  item: Item
  stats: Map<number, Stat>
  forged?: ForgedLine[]
  actions?: ItemAction[]
  side: Side
}

// One card serves every item on the page. With a card per item, moving the
// pointer quickly left the previous card on screen under the next one.
const itemCard = createHoverCardController<HoverPayload>()

interface Props {
  item: Item
  stats: Map<number, Stat>
  /** The item's lines after forgemagic; without them the natural lines show. */
  forged?: ForgedLine[]
  /** Buttons at the foot of the card, such as applying the item to a stuff. */
  actions?: ItemAction[]
  /** Where the card opens; it flips when there is no room. */
  side?: Side
  /** The element the pointer rests on; it receives the trigger's props. */
  children: React.ReactElement<Record<string, unknown>>
}

// Resting the pointer on an item shows its card: name, lines, condition, passive.
export function ItemHover({ item, stats, forged, actions, side = "bottom", children }: Props) {
  const payload = React.useMemo(() => ({ item, stats, forged, actions, side }), [item, stats, forged, actions, side])
  return <HoverCardTrigger controller={itemCard} payload={payload} render={children} />
}

// The shared card itself; rendered once, near the root of the app.
export function ItemHoverCard() {
  const [side, setSide] = React.useState<Side>("bottom")
  return (
    <HoverCard
      controller={itemCard}
      delay={120}
      closeDelay={60}
      onPayloadChange={(payload) => {
        const next = (payload as HoverPayload | undefined)?.side
        if (next) setSide(next)
      }}
    >
      <HoverCardContent<HoverPayload> side={side} className="item-pop">
        {(state) =>
          state.payload ? (
            <>
              <ItemSummary item={state.payload.item} stats={state.payload.stats} forged={state.payload.forged} />
              {state.payload.actions && state.payload.actions.length > 0 && (
                <div className="grid gap-1.5">
                  {state.payload.actions.map((action) => (
                    <Button key={action.label} className="w-full" onClick={action.run}>
                      {action.primary && <PlusIcon aria-hidden="true" className="size-4" />}
                      {action.label}
                    </Button>
                  ))}
                </div>
              )}
            </>
          ) : null
        }
      </HoverCardContent>
    </HoverCard>
  )
}
