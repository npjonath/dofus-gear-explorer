import * as React from "react"
import type { FightBoard, FightEvent, FightPortal, FightPortalRoute } from "../../api/types"
import { cellCenter } from "../../lib/fightBoard"
import type { SpellVisualCatalog } from "../../lib/gameSpellEffects"
import { useI18n } from "../../i18n"
import { GameSprite } from "./GameActor"

interface Props {
  board: FightBoard
  portals?: FightPortal[]
  routes?: Record<number, FightPortalRoute>
  cursor: number | null
  catalog?: SpellVisualCatalog
  events: FightEvent[]
  assetRoot: string
  scale: number
}

function PortalSprite({
  portal,
  active,
  bone,
  events,
  assetRoot,
  scale,
  catalog,
}: {
  portal: FightPortal
  active: boolean
  bone: number
  events: FightEvent[]
  assetRoot: string
  scale: number
  catalog?: SpellVisualCatalog
}) {
  const [settled, setSettled] = React.useState<{ active: boolean; events: FightEvent[] } | null>(null)
  const transition = events.find((e) => e.portalPath?.includes(portal.cell))
  const travelled = transition && transition.type === "move"
  const entry = transition?.portalPath?.[0] === portal.cell
  const atExit = transition?.portalPath?.at(-1) === portal.cell
  const use = transition && (entry || atExit) && settled?.events !== events
  let animation = active ? "AnimStatiqueActif_1" : "AnimStatiqueInactif_1"
  if (use) animation = travelled ? (entry ? "AnimTPperso_1" : "AnimTPpersoOut_1") : "AnimTPsort_1"
  else if (!settled) animation = "AnimSpawn_1"
  else if (settled.active !== active) animation = active ? "AnimInactifToActif_1" : "AnimActifToInactif_1"
  const duration = catalog?.clips[bone]?.portalAnimations?.[animation] ?? 900
  React.useEffect(() => {
    if (settled?.active === active && settled.events === events) return
    const timer = window.setTimeout(() => setSettled({ active, events }), duration)
    return () => window.clearTimeout(timer)
  }, [active, duration, events, settled])
  return (
    <GameSprite
      assetRoot={assetRoot + "/actors/"}
      look={`{${bone}}`}
      bone={String(bone)}
      animation={animation}
      direction={1}
      scale={scale}
      effectColors
    />
  )
}

export function FightPortals({ board, portals, routes, cursor, catalog, events, assetRoot, scale }: Props) {
  const { t } = useI18n()
  if (!portals?.length) return null
  const route = cursor === null ? undefined : routes?.[cursor]
  const label = route ? t("fight.portal.link", { count: route.distance, bonus: route.bonus }) : ""
  const exit = route && cellCenter(board, route.cells[route.cells.length - 1]!)
  return (
    <g className="fight-portals" pointerEvents="none">
      {route && (
        <g>
          <polyline
            points={route.cells
              .map((c) => {
                const p = cellCenter(board, c)
                return `${p.x},${p.y}`
              })
              .join(" ")}
            fill="none"
            stroke="#36bc3c"
            strokeWidth={9}
            opacity={0.85}
          />
          <polyline
            points={route.cells
              .map((c) => {
                const p = cellCenter(board, c)
                return `${p.x},${p.y}`
              })
              .join(" ")}
            fill="none"
            stroke="white"
            strokeWidth={1.4}
            strokeDasharray="3 4"
          />
        </g>
      )}
      {portals.map((p, i) => {
        const at = cellCenter(board, p.cell)
        const active = Boolean(routes?.[p.cell])
        const gfx = catalog?.spells[p.spell]?.normal
          .flatMap((b) => b.sequences ?? [])
          .flatMap((s) => s.gfx)
          .find((g) => g.kind === "mark")
        return (
          <g key={p.cell} transform={`translate(${at.x} ${at.y})`}>
            {board.id && gfx ? (
              <PortalSprite
                portal={p}
                active={active}
                bone={gfx.bone}
                events={events}
                scale={scale}
                catalog={catalog}
                assetRoot={assetRoot}
              />
            ) : (
              <ellipse
                rx={18}
                ry={8}
                fill={active ? "#38baac" : "#4c595a"}
                stroke={active ? "#91ffee" : "#929d9d"}
                strokeWidth={3}
              />
            )}
            <text
              x={-15}
              y={-6}
              textAnchor="middle"
              fill="white"
              fontSize={9}
              fontWeight="bold"
              paintOrder="stroke"
              stroke="#31333a"
              strokeWidth={2}
            >
              {i + 1}
            </text>
          </g>
        )
      })}
      {exit && (
        <g transform={`translate(${exit.x + 9} ${exit.y - 8})`}>
          <rect x={0} y={-8} width={Math.max(66, label.length * 3.7)} height={15} rx={2.5} fill="#161510" fillOpacity={0.96} />
          <text x={4} y={2.5} fill="white" fontSize={6.7} fontWeight="bold">
            {label}
          </text>
        </g>
      )}
    </g>
  )
}
