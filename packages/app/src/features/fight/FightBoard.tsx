import * as React from "react"
import type { FightAim, FightEntity, FightView } from "../../api/types"
import { useI18n } from "../../i18n"
import { boardFrame, cellCenter, cellPoints, cellXY, neighbour, pathTo } from "../../lib/fightBoard"
import { toneOfElement } from "../../lib/stats"
import { useApi } from "../../platform"

/** A number rising from an entity: health lost or gained, points spent. */
export interface Float {
  id: number
  cell: number
  text: string
  /** A stat tone, or "plain". */
  tone: string
}

interface Props {
  view: FightView
  aim: FightAim | undefined
  /** The spell in hand, if any. */
  spell: number | null
  cursor: number | null
  /** The entity picked up during placement. */
  picked: number | null
  floats: Float[]
  breedId: number | null
  busy: boolean
  nameOf(entity: FightEntity): string
  onCursor(cell: number | null): void
  onAct(cell: number): void
  onCancel(): void
  onFloatEnd(id: number): void
}

const TOKEN = 28
const HEADROOM = 64

function span(lo: number, hi: number, format: (n: number) => string): string {
  return lo === hi ? format(lo) : `${format(lo)} – ${format(hi)}`
}

// A loss previewed on a target: what it would lose, or gain when negative.
function lossText(lo: number, hi: number, format: (n: number) => string): string {
  if (lo < 0 || hi < 0) return `+${span(Math.min(-lo, -hi), Math.max(-lo, -hi), format)}`
  return `−${span(lo, hi, format)}`
}

function Face({ entity, breedId, x, y }: { entity: FightEntity; breedId: number | null; x: number; y: number }) {
  const api = useApi()
  const [failed, setFailed] = React.useState<number | null>(null)
  if (entity.kind === "player" && breedId !== null && failed !== breedId) {
    return (
      <image
        href={api.imageUrl(`/images/breeds/${breedId}.png`)}
        x={x + 2}
        y={y + 2}
        width={TOKEN - 4}
        height={TOKEN - 4}
        onError={() => setFailed(breedId)}
      />
    )
  }
  const cx = x + TOKEN / 2
  const cy = y + TOKEN / 2
  if (entity.kind === "player") {
    // A figure, when the class has no portrait on disk.
    return (
      <g className="fight-glyph">
        <circle cx={cx} cy={cy - 5} r={4} />
        <path d={`M${cx - 7} ${cy + 9} a7 7 0 0 1 14 0`} />
      </g>
    )
  }
  return (
    <g className="fight-glyph">
      <circle cx={cx} cy={cy} r={8} />
      <circle cx={cx} cy={cy} r={3} />
    </g>
  )
}

export function FightBoardView({
  view,
  aim,
  spell,
  cursor,
  picked,
  floats,
  breedId,
  busy,
  nameOf,
  onCursor,
  onAct,
  onCancel,
  onFloatEnd,
}: Props) {
  const { t, number } = useI18n()
  const { state } = view
  const board = state.board
  const frame = boardFrame(board, HEADROOM)
  const fighting = state.phase === "fight"

  const walls = new Set(board.walls)
  const reach = new Map(view.steps.map((step) => [step.cell, step]))
  const aiming = fighting && spell !== null
  const targets = new Map(aiming ? aim?.targets.map((target) => [target.cell, target]) : [])
  const inRange = new Set(aiming ? aim?.range : [])
  const target = cursor !== null ? targets.get(cursor) : undefined
  const impact = new Set(target?.zone)
  const path = fighting && !aiming && cursor !== null ? pathTo(view.steps, cursor) : []
  const onPath = new Set(path)
  const losses = new Map(target?.losses.map((loss) => [loss.entity, loss]))
  const occupied = new Map(state.entities.map((entity) => [entity.cell, entity]))

  function zoneOf(cell: number): string | undefined {
    if (walls.has(cell)) return undefined
    if (impact.has(cell)) return "impact"
    if (targets.has(cell)) return "range"
    if (inRange.has(cell)) return "blocked"
    if (onPath.has(cell)) return "path"
    if (!aiming && fighting && reach.has(cell)) return "reach"
    if (!fighting && picked !== null && !occupied.has(cell)) return "reach"
    return undefined
  }

  function onKeyDown(event: React.KeyboardEvent) {
    if (event.key === "Escape") {
      onCancel()
      return
    }
    const from = cursor ?? state.entities[0]!.cell
    const next = neighbour(board, from, event.key)
    if (next !== null) {
      event.preventDefault()
      onCursor(next)
    } else if ((event.key === "Enter" || event.key === " ") && cursor !== null) {
      event.preventDefault()
      onAct(cursor)
    }
  }

  const cells = Array.from({ length: board.width * board.height }, (_, cell) => cell)
  // Entities nearer the bottom of the drawing are drawn over those behind.
  const entities = [...state.entities].sort((a, b) => {
    const pa = cellXY(board, a.cell)
    const pb = cellXY(board, b.cell)
    return pa.x + pa.y - (pb.x + pb.y)
  })
  const last = path[path.length - 1]

  return (
    <div
      className="fight-board"
      style={{ "--board-ratio": String(frame.width / frame.height) } as React.CSSProperties}
      role="application"
      tabIndex={0}
      aria-label={t("fight.board")}
      aria-describedby="fight-board-hint"
      aria-busy={busy}
      onKeyDown={onKeyDown}
      onMouseLeave={() => onCursor(null)}
      onContextMenu={(event) => {
        event.preventDefault()
        onCancel()
      }}
    >
      <span id="fight-board-hint" className="sr-only">
        {t("fight.boardHint")}
      </span>
      <svg viewBox={`${frame.x} ${frame.y} ${frame.width} ${frame.height}`} aria-hidden="true">
        {cells.map((cell) => {
          const { x, y } = cellXY(board, cell)
          const center = cellCenter(board, cell)
          return (
            <polygon
              key={cell}
              className="fight-cell"
              points={cellPoints(center)}
              data-tone={(x + y) % 2 === 0 ? "a" : "b"}
              data-wall={walls.has(cell) || undefined}
              data-zone={zoneOf(cell)}
              onMouseEnter={() => onCursor(cell)}
              onClick={() => onAct(cell)}
            />
          )
        })}
        {board.walls.map((cell) => {
          const center = cellCenter(board, cell)
          return <polygon key={cell} className="fight-pillar" points={cellPoints({ x: center.x, y: center.y - 9 })} />
        })}
        {cursor !== null && !walls.has(cursor) && (
          <polygon className="fight-cursor" points={cellPoints(cellCenter(board, cursor))} />
        )}
        {last !== undefined && (
          <text className="fight-cost" {...cellCenter(board, last)} dy="0.35em" data-tone="mp">
            {reach.get(last)?.cost}
          </text>
        )}
        {target?.shifts.map((shift) => {
          const center = cellCenter(board, shift.cell)
          return (
            <rect
              key={shift.entity}
              className="fight-ghost"
              x={center.x - TOKEN / 2}
              y={center.y - TOKEN - 4}
              width={TOKEN}
              height={TOKEN}
              rx={5}
            />
          )
        })}
        {entities.map((entity) => {
          const center = cellCenter(board, entity.cell)
          const x = center.x - TOKEN / 2
          const y = center.y - TOKEN - 4
          const loss = losses.get(entity.id)
          const health = Math.max(0, Math.min(1, entity.hp / entity.maxHp))
          const after = loss ? Math.max(0, Math.min(1, (entity.hp - loss.max) / entity.maxHp)) : health
          return (
            <g key={entity.id} className="fight-entity" data-kind={entity.kind} data-picked={picked === entity.id || undefined}>
              <title>{nameOf(entity)}</title>
              <rect className="fight-token" x={x} y={y} width={TOKEN} height={TOKEN} rx={5} />
              <Face entity={entity} breedId={breedId} x={x} y={y} />
              <rect className="fight-hp-track" x={x} y={y - 7} width={TOKEN} height={4} rx={1} />
              <rect className="fight-hp" x={x} y={y - 7} width={TOKEN * health} height={4} rx={1} />
              {loss && after < health && (
                <rect
                  className="fight-hp-loss"
                  x={x + TOKEN * after}
                  y={y - 7}
                  width={TOKEN * (health - after)}
                  height={4}
                />
              )}
              {loss && (
                <text className="fight-preview" x={center.x} y={y - 12}>
                  <tspan x={center.x} dy={aim?.canCrit ? "-1.15em" : 0}>
                    {lossText(loss.min, loss.max, number)}
                  </tspan>
                  {aim?.canCrit && (
                    <tspan className="fight-preview-crit" x={center.x} dy="1.15em">
                      {lossText(loss.critMin, loss.critMax, number)}
                    </tspan>
                  )}
                </text>
              )}
            </g>
          )
        })}
        {floats.map((float, index) => {
          const center = cellCenter(board, float.cell)
          // Numbers born of the same action fan out instead of piling up.
          const stagger = floats.slice(0, index).filter((other) => other.cell === float.cell).length
          return (
            <text
              key={float.id}
              className="fight-float"
              data-tone={float.tone}
              x={center.x}
              y={center.y - TOKEN - 18}
              style={{ animationDelay: `${stagger * 140}ms` }}
              onAnimationEnd={() => onFloatEnd(float.id)}
            >
              {float.text}
            </text>
          )
        })}
      </svg>
    </div>
  )
}

export function elementTone(element: number | undefined): string {
  return toneOfElement(element ?? 0)
}
