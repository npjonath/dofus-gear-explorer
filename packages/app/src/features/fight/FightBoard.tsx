import * as React from "react"
import type { FightAim, FightBoard, FightEntity, FightView } from "../../api/types"
import { useI18n } from "../../i18n"
import {
  blockFaces,
  boardFrame,
  cellCenter,
  cellPoints,
  cellXY,
  gridPoint,
  neighbour,
  pathTo,
  PILLAR,
  slabFaces,
  type Point,
} from "../../lib/fightBoard"
import { sceneAt, type Actor, type Facing, type Scene } from "../../lib/fightTimeline"
import { toneOfElement } from "../../lib/stats"
import { Figure, FIGURE_TOP, SILHOUETTE } from "./FightFigures"
import { defaultFacing, type Playback, type Run } from "./useFightPlayback"

interface Props {
  view: FightView
  aim: FightAim | undefined
  /** The spell in hand, if any. */
  spell: number | null
  cursor: number | null
  /** The entity picked up during placement. */
  picked: number | null
  /** The entity whose name and health are written out on the board. */
  pointed: number | null
  playback: Playback
  busy: boolean
  nameOf(entity: FightEntity): string
  onCursor(cell: number | null): void
  onAct(cell: number): void
  onCancel(): void
}

// Room above the back row: a figure, its readout and what a cast would cost it.
const HEADROOM = 92
const BAR = 30

// An eight-pointed burst: the mark of a critical hit, next to its number and where it lands.
const BURST = Array.from({ length: 16 }, (_, index) => {
  const radius = index % 2 === 0 ? 6.5 : 2.9
  const angle = (index * Math.PI) / 8 - Math.PI / 2
  return `${index === 0 ? "M" : "L"}${(Math.cos(angle) * radius).toFixed(2)} ${(Math.sin(angle) * radius).toFixed(2)}`
}).join(" ")
  .concat(" Z")

const easeOut = (value: number) => 1 - (1 - value) ** 3
const clamp = (value: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, value))

function span(lo: number, hi: number, format: (n: number) => string): string {
  return lo === hi ? format(lo) : `${format(lo)} – ${format(hi)}`
}

// A loss previewed on a target: what it would lose, or gain when negative.
function lossText(lo: number, hi: number, format: (n: number) => string): string {
  if (lo < 0 || hi < 0) return `+${span(Math.min(-lo, -hi), Math.max(-lo, -hi), format)}`
  return `−${span(lo, hi, format)}`
}

function useWidth(ref: React.RefObject<Element | null>): number {
  const [width, setWidth] = React.useState(0)
  React.useLayoutEffect(() => {
    const element = ref.current
    if (!element) return
    const observer = new ResizeObserver(([entry]) => setWidth(entry!.contentRect.width))
    observer.observe(element)
    return () => observer.disconnect()
  }, [ref])
  return width
}

// Milliseconds since the action being played started; past its end once the board has caught up.
function useElapsed(run: Run | null, playing: boolean, onEnd: (run: number) => void): number {
  const [now, setNow] = React.useState(0)
  React.useEffect(() => {
    if (!run || !playing) return
    let frame = requestAnimationFrame(function tick(time) {
      setNow(time)
      // A window that was hidden comes back late: the run ends on the first frame past its end.
      if (time - run.startedAt >= run.timeline.duration) onEnd(run.id)
      else frame = requestAnimationFrame(tick)
    })
    return () => cancelAnimationFrame(frame)
  }, [run, playing, onEnd])
  return run && playing ? Math.max(0, now - run.startedAt) : Infinity
}

// The arena itself: the slab's two front faces, then every cell. Nothing in it changes during a fight.
const Ground = React.memo(function Ground({ board }: { board: FightBoard }) {
  const slab = slabFaces(board)
  const walls = new Set(board.walls)
  return (
    <g>
      <polygon className="fight-slab" data-face="left" points={slab.left} />
      <polygon className="fight-slab" data-face="right" points={slab.right} />
      {Array.from({ length: board.width * board.height }, (_, cell) => {
        const { x, y } = cellXY(board, cell)
        return (
          <polygon
            key={cell}
            className="fight-cell"
            points={cellPoints(cellCenter(board, cell))}
            data-cell={cell}
            data-tone={(x + y) % 2 === 0 ? "a" : "b"}
            data-wall={walls.has(cell) || undefined}
          />
        )
      })}
      <polygon className="fight-rim" points={slab.top} />
    </g>
  )
})

function Pillar({ center }: { center: Point }) {
  const faces = blockFaces(center, PILLAR)
  return (
    <g className="fight-pillar">
      <polygon data-face="left" points={faces.left} />
      <polygon data-face="right" points={faces.right} />
      <polygon data-face="top" points={cellPoints({ x: center.x, y: center.y - PILLAR })} />
    </g>
  )
}

// How a figure is moved, tilted and faded by what it is doing, on top of where it stands.
function stance(actor: Actor, kind: FightEntity["kind"], at: Point) {
  let x = 0
  let y = 0
  let lean = 0
  let opacity = 1
  let squash = 1
  const { phase, dir } = actor
  const swell = Math.sin(Math.PI * phase)
  switch (actor.pose) {
    case "walk":
      y = -Math.abs(Math.sin(actor.stride * Math.PI)) * 2.6
      lean = actor.facing * 4
      break
    case "slide":
      lean = -dir * 10 * (1 - phase)
      break
    case "blink":
      opacity = Math.abs(phase * 2 - 1)
      squash = 0.55 + 0.45 * opacity
      break
    case "windup":
      lean = actor.facing * (phase < 0.55 ? -7 * Math.sin((Math.PI * phase) / 0.55) : 6 * Math.sin((Math.PI * (phase - 0.55)) / 0.45))
      break
    case "lunge": {
      const aim = actor.aim ? gridPoint(actor.aim.x, actor.aim.y) : at
      const length = Math.hypot(aim.x - at.x, aim.y - at.y) || 1
      x = ((aim.x - at.x) / length) * 12 * swell
      y = ((aim.y - at.y) / length) * 12 * swell
      lean = actor.facing * 8 * swell
      break
    }
    case "hit":
      if (kind === "dummy") {
        // A post springs back and forth before it settles.
        lean = dir * 20 * Math.sin(phase * Math.PI * 5) * Math.exp(-3.5 * phase)
      } else {
        x = dir * 3.5 * swell
        lean = dir * 9 * swell
      }
      break
    case "down": {
      const fall = phase < 0.22 ? (phase / 0.22) ** 2 : phase < 0.62 ? 1 : 1 - easeOut((phase - 0.62) / 0.38)
      lean = dir * 84 * fall
      break
    }
    default:
  }
  return { x, y, lean, opacity, squash }
}

interface StageProps {
  board: FightBoard
  entities: FightEntity[]
  playback: Playback
  picked: number | null
  pointed: number | null
  losses: Map<number, { min: number; max: number; critMin: number; critMax: number }>
  canCrit: boolean
  /** User units per pixel, for what is written on the board. */
  unit: number
  nameOf(entity: FightEntity): string
}

// Everything that stands or flies on the arena. It follows the clock of the action being played.
function Stage({ board, entities, playback, picked, pointed, losses, canCrit, unit, nameOf }: StageProps) {
  const { t, number } = useI18n()
  const { run, playing } = playback
  const elapsed = useElapsed(run, playing, playback.finish)
  const scene: Scene | null = run && playing ? sceneAt(run.timeline, elapsed, run.facing) : null

  const cast = entities.map((entity) => {
    const still = cellXY(board, entity.cell)
    const facing: Facing = playback.facing[entity.id] ?? defaultFacing(entity.kind)
    const actor: Actor = scene?.actors[entity.id] ?? { ...still, facing, pose: "idle", phase: 0, dir: 1, stride: 0 }
    return { entity, actor, at: gridPoint(actor.x, actor.y), depth: actor.x + actor.y }
  })
  // Whatever is nearer the bottom of the drawing is drawn over what stands behind.
  const standing = [
    ...board.walls.map((cell) => {
      const { x, y } = cellXY(board, cell)
      return { depth: x + y, pillar: cell, piece: undefined }
    }),
    ...cast.map((piece) => ({ depth: piece.depth + 0.01, pillar: undefined, piece })),
  ].sort((a, b) => a.depth - b.depth)
  const facingOf = new Map(cast.map((piece) => [piece.entity.id, piece.actor.facing]))

  return (
    <>
      {scene?.impacts.map(({ cue, progress }) => {
        const center = cellCenter(board, cue.center)
        const grow = 0.55 + 0.75 * easeOut(progress)
        return (
          <g key={cue.start} className="fight-impact">
            <g opacity={(1 - progress) ** 2}>
              {cue.cells.map((cell) => (
                <polygon key={cell} points={cellPoints(cellCenter(board, cell), 1.2)} />
              ))}
            </g>
            <polygon
              className="fight-impact-ring"
              points={cellPoints({ x: 0, y: 0 })}
              transform={`translate(${center.x} ${center.y}) scale(${grow})`}
              opacity={1 - progress}
            />
          </g>
        )
      })}

      {standing.map(({ pillar, piece }) => {
        if (pillar !== undefined) return <Pillar key={`pillar-${pillar}`} center={cellCenter(board, pillar)} />
        const { entity, actor, at } = piece
        const move = stance(actor, entity.kind, at)
        return (
          <g
            key={entity.id}
            className="fight-entity"
            transform={`translate(${at.x} ${at.y})`}
            data-kind={entity.kind}
            data-pose={actor.pose}
            data-picked={picked === entity.id || undefined}
            style={{ "--beat": `${(entity.id * 610) % 1900}ms` } as React.CSSProperties}
          >
            <ellipse className="fight-foot" rx={14} ry={5.6} />
            <g
              transform={`translate(${move.x} ${move.y}) rotate(${move.lean}) scale(${actor.facing} ${move.squash})`}
              opacity={move.opacity}
            >
              <Figure kind={entity.kind} pose={actor.pose} phase={actor.phase} stride={actor.stride} tone={actor.tone} />
            </g>
          </g>
        )
      })}

      {scene?.bolts.map(({ cue, progress }) => {
        const from = cellCenter(board, cue.from)
        const to = cellCenter(board, cue.to)
        const a = { x: from.x + (facingOf.get(cue.caster) ?? 1) * 10, y: from.y - 52 }
        const b = { x: to.x, y: to.y - (cue.onEntity ? 27 : 3) }
        const arc = Math.min(34, Math.hypot(b.x - a.x, b.y - a.y) * 0.16)
        const on = (part: number) => ({
          x: a.x + (b.x - a.x) * part,
          y: a.y + (b.y - a.y) * part - Math.sin(Math.PI * part) * arc,
        })
        const head = on(progress)
        const tail = on(Math.max(0, progress - 0.3))
        const half = cue.critical ? 5.4 : 4
        return (
          <g key={cue.start} className="fight-bolt" data-tone={cue.tone}>
            <line x1={tail.x} y1={tail.y} x2={head.x} y2={head.y} strokeWidth={cue.critical ? 5 : 3.4} />
            <rect
              x={head.x - half}
              y={head.y - half}
              width={half * 2}
              height={half * 2}
              rx={1.4}
              transform={`rotate(45 ${head.x} ${head.y})`}
            />
          </g>
        )
      })}

      {scene?.impacts.map(({ cue, progress }) => {
        if (!cue.critical) return null
        const center = cellCenter(board, cue.center)
        return (
          <path
            key={cue.start}
            className="fight-burst"
            d={BURST}
            transform={`translate(${center.x} ${center.y - 30}) scale(${1 + 1.6 * easeOut(progress)})`}
            opacity={1 - progress ** 2}
          />
        )
      })}

      {cast.map(({ entity, at }) => {
        const hp = playback.hp?.[entity.id] ?? entity.hp
        const health = clamp(hp / entity.maxHp, 0, 1)
        const loss = losses.get(entity.id)
        const after = loss ? clamp((entity.hp - loss.max) / entity.maxHp, 0, 1) : health
        // While a cast is previewed on it, what it would lose takes the place of its name.
        const named = pointed === entity.id && !loss
        const name = nameOf(entity)
        const figures = t("fight.entity.hp", { hp: number(hp), max: number(entity.maxHp) })
        const width = named ? Math.max(76, Math.max(name.length * 6.6, figures.length * 5.9) + 16) : BAR
        const bar = named ? width - 12 : BAR
        return (
          <g
            key={entity.id}
            className="fight-label"
            transform={`translate(${at.x} ${at.y - FIGURE_TOP[entity.kind] - 2}) scale(${unit})`}
          >
            {named && (
              <>
                <rect className="fight-readout" x={-width / 2} y={-34} width={width} height={36} rx={5} />
                <text className="fight-readout-name" x={-width / 2 + 6} y={-21}>
                  {name}
                </text>
                <text className="fight-readout-hp" x={-width / 2 + 6} y={-8.5} data-tone="vitality">
                  {figures}
                </text>
              </>
            )}
            <rect className="fight-hp-track" x={-bar / 2} y={-4.5} width={bar} height={4} rx={1} />
            <rect className="fight-hp" x={-bar / 2} y={-4.5} width={bar} height={4} rx={1} style={{ transform: `scaleX(${health})` }} />
            {loss && after < health && (
              <rect className="fight-hp-loss" x={-bar / 2 + bar * after} y={-4.5} width={bar * (health - after)} height={4} />
            )}
            {loss && (
              <text className="fight-preview" y={named ? -39 : -9}>
                <tspan x={0} dy={canCrit ? "-1.15em" : 0}>
                  {lossText(loss.min, loss.max, number)}
                </tspan>
                {canCrit && (
                  <tspan className="fight-preview-crit" x={0} dy="1.15em">
                    {lossText(loss.critMin, loss.critMax, number)}
                  </tspan>
                )}
              </text>
            )}
          </g>
        )
      })}
    </>
  )
}

export function FightBoardView({
  view,
  aim,
  spell,
  cursor,
  picked,
  pointed,
  playback,
  busy,
  nameOf,
  onCursor,
  onAct,
  onCancel,
}: Props) {
  const { t } = useI18n()
  const { state } = view
  const board = state.board
  const frame = boardFrame(board, HEADROOM)
  const fighting = state.phase === "fight"
  const root = React.useRef<HTMLDivElement>(null)
  const width = useWidth(root)
  // What is written on the board keeps a readable size when the board is drawn small.
  const unit = width > 0 ? clamp(frame.width / width, 0.9, 1.9) : 1

  const walls = new Set(board.walls)
  const reach = new Map(view.steps.map((step) => [step.cell, step]))
  // While an action plays, the board shows what happens, not what could be done next.
  const { settled } = playback
  const aiming = fighting && spell !== null
  const targets = new Map(aiming && settled ? aim?.targets.map((target) => [target.cell, target]) : [])
  const inRange = new Set(aiming && settled ? aim?.range : [])
  const target = cursor !== null ? targets.get(cursor) : undefined
  const impact = new Set(target?.zone)
  const path = fighting && !aiming && settled && cursor !== null ? pathTo(view.steps, cursor) : []
  const onPath = new Set(path)
  const losses = new Map(target?.losses.map((loss) => [loss.entity, loss]))
  const occupied = new Map(state.entities.map((entity) => [entity.cell, entity]))
  const kinds = new Map(state.entities.map((entity) => [entity.id, entity.kind]))

  function zoneOf(cell: number): string | undefined {
    if (walls.has(cell)) return undefined
    if (impact.has(cell)) return "impact"
    if (targets.has(cell)) return "range"
    if (inRange.has(cell)) return "blocked"
    if (onPath.has(cell)) return "path"
    if (!aiming && fighting && settled && reach.has(cell)) return "reach"
    if (!fighting && picked !== null && !occupied.has(cell)) return "reach"
    return undefined
  }

  // Whether a click on the pointed cell does something.
  function acts(cell: number): boolean {
    if (walls.has(cell)) return false
    if (!fighting) return occupied.has(cell) || picked !== null
    return aiming ? (aim?.targets.some((candidate) => candidate.cell === cell) ?? false) : reach.has(cell)
  }

  function cellOf(event: React.MouseEvent): number | null {
    const value = (event.target as Element).getAttribute("data-cell")
    return value === null ? null : Number(value)
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

  const zones = Array.from({ length: board.width * board.height }, (_, cell) => ({ cell, zone: zoneOf(cell) })).filter(
    (entry) => entry.zone !== undefined
  )
  const last = path[path.length - 1]
  const lastAt = last === undefined ? undefined : cellCenter(board, last)
  const pickedCell = state.entities.find((entity) => entity.id === picked)?.cell

  return (
    <div
      ref={root}
      className="fight-board"
      style={{ "--board-ratio": String(frame.width / frame.height) } as React.CSSProperties}
      role="application"
      tabIndex={0}
      aria-label={t("fight.board")}
      aria-describedby="fight-board-hint"
      aria-busy={busy}
      data-act={(cursor !== null && acts(cursor)) || undefined}
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
      <svg
        viewBox={`${frame.x} ${frame.y} ${frame.width} ${frame.height}`}
        aria-hidden="true"
        onMouseOver={(event) => {
          const cell = cellOf(event)
          if (cell !== null && cell !== cursor) onCursor(cell)
        }}
        onClick={(event) => {
          const cell = cellOf(event)
          if (cell !== null) onAct(cell)
        }}
      >
        <Ground board={board} />
        {zones.map(({ cell, zone }) => (
          <polygon key={cell} className="fight-zone" data-zone={zone} points={cellPoints(cellCenter(board, cell), 1.2)} />
        ))}
        {pickedCell !== undefined && <polygon className="fight-zone" data-zone="picked" points={cellPoints(cellCenter(board, pickedCell), 1.2)} />}
        {cursor !== null && !walls.has(cursor) && (
          <polygon className="fight-cursor" points={cellPoints(cellCenter(board, cursor), 0.6)} />
        )}
        {lastAt && (
          <g transform={`translate(${lastAt.x} ${lastAt.y}) scale(${unit})`}>
            <text className="fight-cost" dy="0.35em" data-tone="mp">
              {reach.get(last!)?.cost}
            </text>
          </g>
        )}
        {target?.shifts.map((shift) => {
          const center = cellCenter(board, shift.cell)
          const kind = kinds.get(shift.entity) ?? "dummy"
          const facing = playback.facing[shift.entity] ?? defaultFacing(kind)
          return (
            <path
              key={shift.entity}
              className="fight-ghost"
              d={SILHOUETTE[kind]}
              transform={`translate(${center.x} ${center.y}) scale(${facing} 1)`}
            />
          )
        })}
        <Stage
          board={board}
          entities={state.entities}
          playback={playback}
          picked={picked}
          pointed={pointed}
          losses={losses}
          canCrit={Boolean(aim?.canCrit)}
          unit={unit}
          nameOf={nameOf}
        />
        {/* A figure stands taller than its cell: pointing at its body points at the cell under its feet, not the one behind it. */}
        {state.entities.map((entity) => {
          const center = cellCenter(board, entity.cell)
          const height = FIGURE_TOP[entity.kind]
          return <rect key={entity.id} className="fight-hit" data-cell={entity.cell} x={center.x - 13} y={center.y - height} width={26} height={height} />
        })}
        {playback.floats.map((float) => {
          const center = cellCenter(board, float.cell)
          // The burst of a critical hit comes before its number; the two are centred together.
          const reads = float.text.length * 8.4 + (float.critical ? 15 : 0)
          // Numbers pile up over the health bar, or over the readout of the entity being pointed.
          const above = (pointed === float.entity ? 44 : 12) + (float.rank % 5) * 15
          return (
            <g
              key={float.id}
              transform={`translate(${center.x} ${center.y - FIGURE_TOP[float.kind] - above * unit}) scale(${unit * (float.critical ? 1.18 : 1)})`}
            >
              <g
                className="fight-float"
                data-tone={float.tone}
                data-critical={float.critical || undefined}
                style={{ animationDelay: `${float.delay}ms` }}
                onAnimationEnd={() => playback.endFloat(float.id)}
              >
                {float.critical && <path className="fight-float-mark" d={BURST} transform={`translate(${-reads / 2 + 6.5} -5.4)`} />}
                <text x={-reads / 2 + (float.critical ? 15 : 0)}>{float.text}</text>
              </g>
            </g>
          )
        })}
      </svg>
    </div>
  )
}

export function elementTone(element: number | undefined): string {
  return toneOfElement(element ?? 0)
}
