import type { FightBoard, FightEvent, FightState } from "../api/types"
import { cellDistance, cellXY, heading } from "./fightBoard"
import { castVisual, gameDirection, groundGraphics, type Direction, type GraphicCue, type SpellVisualCatalog } from "./gameSpellEffects"
import { toneOfElement } from "./stats"

/*
  What one action of a fight looks like, as cues on a clock. The engine
  answers with the fight as it now stands and the events that led there;
  the board plays them in order (cast, projectile, impact, losses, shoves)
  and always ends on the state the engine gave.
*/

/** Milliseconds a walked cell takes, and a cell an entity is shoved through. */
export const WALK_MS = 125
export const SLIDE_MS = 55
const SLIDE_BASE_MS = 90
const BLINK_MS = 280
const WINDUP_MS = 300
const LUNGE_MS = 280
const BOLT_MS_PER_CELL = 36
const BOLT_MIN_MS = 150
const BOLT_MAX_MS = 380
const IMPACT_MS = 380
export const HIT_MS = 440
/** Losses of one cast land one after the other, this far apart. */
export const HIT_STAGGER_MS = 120
export const TOPPLE_MS = 1150
/** How long after a blow lands the board is the player's again, while the target still recoils. */
export const HIT_HOLD_MS = 160

/** Which way a figure looks on the drawing: 1 to the right, -1 to the left. */
export type Facing = 1 | -1
export type Gait = "walk" | "slide" | "blink"
export type Pose = "idle" | Gait | "windup" | "lunge" | "hit" | "down"

export interface MoveCue {
  entity: number
  /** The cells gone through, from where the entity stood. */
  cells: number[]
  gait: Gait
  start: number
  end: number
}

export interface PoseCue {
  entity: number
  pose: "windup" | "lunge" | "hit" | "down"
  start: number
  end: number
  /** The way the blow travels on the drawing. */
  dir: Facing
  /** The cell a caster aims at. */
  cell?: number
  /** The tone of what is being cast. */
  tone?: string
  animation?: string
  spell?: number
}

export interface BoltCue {
  caster: number
  from: number
  to: number
  /** Whether it ends on an entity or on the ground. */
  onEntity: boolean
  start: number
  end: number
  tone: string
  critical: boolean
}

export interface ImpactCue {
  cells: number[]
  center: number
  start: number
  end: number
  critical: boolean
}

export interface HealthCue {
  entity: number
  at: number
  hp: number
}

export interface FloatCue {
  entity: number
  /** Where the entity stands when the number rises. */
  cell: number
  at: number
  kind: "hp" | "ap"
  /** Points gained, or lost when negative. */
  amount: number
  tone: string
  critical: boolean
  /** How many numbers rose from this entity before this one, in the same action. */
  rank: number
}

export interface TurnCue {
  entity: number
  at: number
  facing?: Facing
  direction: Direction
}

export interface Timeline {
  board: FightBoard
  /** Where each entity stands when the action starts. */
  start: Record<number, number>
  duration: number
  /** When the board is the player's again: what still plays after it (a recoil, a number rising) hides nothing. */
  free: number
  moves: MoveCue[]
  poses: PoseCue[]
  bolts: BoltCue[]
  impacts: ImpactCue[]
  health: HealthCue[]
  floats: FloatCue[]
  turns: TurnCue[]
  graphics: GraphicCue[]
  carriers: Record<number, number>
  lifts: { rider: number; carrier: number; pickup: boolean; start: number; end: number }[]
}

function contiguous(board: FightBoard, cells: number[]): boolean {
  return cells.every((cell, index) => index === 0 || cellDistance(board, cells[index - 1]!, cell) === 1)
}

// The tone of a cast: the element of the first loss it deals, the Vitality tone when it only heals.
function toneOfCast(events: FightEvent[], cast: number): string {
  for (let index = cast + 1; index < events.length; index++) {
    const event = events[index]!
    if (event.type === "cast") break
    if (event.type === "damage" && !event.push) return toneOfElement(event.element ?? 0)
    if (event.type === "heal") return "vitality"
  }
  return "plain"
}

/**
 * Lays the events of one action on a clock. `zone` is the cells the cast was
 * shown to cover when it was aimed; without it the aimed cell alone flashes.
 */
export function buildTimeline(before: FightState, events: FightEvent[], zone?: number[], visuals?: SpellVisualCatalog): Timeline {
  const board = before.board
  const cellOf = new Map(before.entities.map((entity) => [entity.id, entity.cell]))
  const carrying = new Map(before.entities.filter((entity) => entity.carriedBy).map((entity) => [entity.id, entity.carriedBy!]))
  const hpOf = new Map(before.entities.map((entity) => [entity.id, entity.hp]))
  const maxOf = new Map(before.entities.map((entity) => [entity.id, entity.maxHp]))
  const busy = new Map<number, number>()
  const ranks = new Map<number, number>()
  const timeline: Timeline = {
    board,
    start: Object.fromEntries(cellOf),
    duration: 0,
    free: 0,
    moves: [],
    poses: [],
    bolts: [],
    impacts: [],
    health: [],
    floats: [],
    turns: [],
    graphics: [],
    carriers: Object.fromEntries(carrying),
    lifts: [],
  }

  // When the next beat may start, and when everything laid so far is over.
  let now = 0
  let end = 0
  let free = 0
  // Losses of one beat fan out; shoves of one beat happen together.
  let hits = 0
  let hitsEnd = 0
  let shove: { start: number; end: number; last: number } | null = null
  // Where the blow comes from, to know which way a target is thrown.
  let source: number | null = null
  let zoneLeft = zone

  const closeHits = () => {
    if (hits > 0) now = Math.max(now, hitsEnd)
    hits = 0
  }
  const closeShove = () => {
    if (shove) now = Math.max(now, shove.end)
    shove = null
  }
  const float = (entity: number, at: number, cue: Pick<FloatCue, "kind" | "amount" | "tone" | "critical">) => {
    const rank = ranks.get(entity) ?? 0
    ranks.set(entity, rank + 1)
    timeline.floats.push({ entity, cell: cellOf.get(entity)!, at, rank, ...cue })
    end = Math.max(end, at)
  }
  const strike = (entity: number, at: number, event: FightEvent) => {
    const value = event.value ?? 0
    const hp = Math.max(0, (hpOf.get(entity) ?? 0) - value)
    hpOf.set(entity, hp)
    timeline.health.push({ entity, at, hp })
    if (value === 0) return
    const from = source ?? cellOf.get(entity)!
    timeline.poses.push({ entity, pose: "hit", start: at, end: at + HIT_MS, dir: heading(board, from, cellOf.get(entity)!) || 1 })
    float(entity, at, {
      kind: "hp",
      amount: -value,
      tone: event.push ? "plain" : toneOfElement(event.element ?? 0),
      critical: Boolean(event.critical) && !event.push,
    })
    end = Math.max(end, at + HIT_MS)
    free = Math.max(free, at + HIT_HOLD_MS)
  }

  events.forEach((event, index) => {
    const entity = event.entity ?? 0
    if (event.type === "rune") {
      if (event.cell === undefined || event.spell === undefined) return
      const caster = before.entities.find((fighter) => fighter.id === entity)
      if (!caster) return
      const target = before.entities.find((fighter) => cellOf.get(fighter.id) === event.cell)
      const visual = castVisual(visuals, event.spell, Boolean(event.critical), caster, target,
        board, cellOf.get(entity) ?? caster.cell, event.cell, now)
      for (const cue of visual?.graphics ?? []) {
        if (cue.kind !== "mark" || timeline.graphics.some((other) =>
          other.kind === "mark" && other.to === cue.to && other.bone === cue.bone && other.end >= now)) continue
        timeline.graphics.push(cue)
        end = Math.max(end, cue.end)
      }
      return
    }
    if (event.type === "release") {
      carrying.delete(entity)
      timeline.lifts.push({ rider: entity, carrier: event.carrier ?? 0, pickup: false, start: now, end: now })
      return
    }
    if (event.type === "mark") {
      if (!visuals || event.cell === undefined || event.triggerSpell === undefined) return
      const mark = before.marks?.find((entry) => entry.cell === event.cell && (entry.action === 2022 ? entry.spell : entry.from) === event.spell && entry.action === event.action)
      const owner = mark ? before.entities.find((fighter) => fighter.id === mark.caster) : before.entities.find((fighter) => fighter.kind === "player")
      const target = before.entities.find((fighter) => fighter.id === entity)
      if (!owner) return
      closeHits()
      closeShove()
      if (mark) {
        for (const { gfx, trigger } of groundGraphics(visuals, mark)) {
          if (!trigger) continue
          const finish = now + trigger.duration
          timeline.graphics.push({ ...gfx, from: event.cell, to: event.cell, direction: 0,
            start: now, end: finish, animation: trigger.animation })
          end = Math.max(end, finish)
        }
      }
      // A rune's activating spell can grant a state and remove it again
      // within the same action. Its FX binding needs the state at detonation.
      const caster = event.casterStates ? {
        ...owner,
        buffs: event.casterStates.map((ref) => ({kind: "state" as const, spell: 0, from: 0, value: 0, ref, turns: 1})),
      } : owner
      const visual = castVisual(visuals, event.triggerSpell, Boolean(event.critical ?? mark?.critical),
        caster, target, board, event.cell, cellOf.get(entity) ?? target?.cell ?? event.cell, now)
      if (visual) {
        timeline.graphics.push(...visual.graphics)
        source = event.cell
        now = visual.hit
        end = Math.max(visual.end, end)
      }
      return
    }
    if (event.type === "cast") {
      if (!cellOf.has(entity)) return
      closeHits()
      closeShove()
      const from = cellOf.get(entity)!
      source = from
      // What a fight opens with is aimed at nothing: its effects show, the gesture does not.
      if (event.cell === undefined) return
      const to = event.cell
      const dir = heading(board, from, to)
      const tone = toneOfCast(events, index)
      const distance = cellDistance(board, from, to)
      const direction = gameDirection(board, from, to)
      if (direction !== undefined) timeline.turns.push({ entity, at: now, facing: dir || undefined, direction })
      const caster = before.entities.find((fighter) => fighter.id === entity)
      const target = before.entities.find((fighter) => cellOf.get(fighter.id) === to)
      const visual = caster && castVisual(visuals, event.spell ?? 0, Boolean(event.critical), caster, target, board, from, to, now)
      if (visual) {
        timeline.poses.push({
          entity, pose: "windup", start: now, end: visual.poseEnd, dir: dir || 1,
          cell: to, tone, animation: visual.animation, spell: event.spell,
        })
        timeline.graphics.push(...visual.graphics)
        zoneLeft = undefined
        end = Math.max(end, visual.end)
        free = Math.max(free, visual.hit + HIT_HOLD_MS)
        now = visual.hit
        return
      }
      let impact: number
      if (distance === 1) {
        // Next to the target: the caster strikes it, nothing flies.
        timeline.poses.push({ entity, pose: "lunge", start: now, end: now + LUNGE_MS, dir: dir || 1, cell: to, tone, spell: event.spell })
        impact = now + LUNGE_MS * 0.45
      } else {
        timeline.poses.push({ entity, pose: "windup", start: now, end: now + WINDUP_MS, dir: dir || 1, cell: to, tone, spell: event.spell })
        impact = now + WINDUP_MS * 0.7
        if (distance > 1) {
          const flight = Math.max(BOLT_MIN_MS, Math.min(BOLT_MAX_MS, distance * BOLT_MS_PER_CELL))
          const onEntity = [...cellOf.values()].includes(to)
          timeline.bolts.push({ caster: entity, from, to, onEntity, start: impact, end: impact + flight, tone, critical: Boolean(event.critical) })
          impact += flight
        }
      }
      timeline.impacts.push({ cells: zoneLeft ?? [to], center: to, start: impact, end: impact + IMPACT_MS, critical: Boolean(event.critical) })
      zoneLeft = undefined
      end = Math.max(end, now + WINDUP_MS, impact + IMPACT_MS)
      free = Math.max(free, impact + HIT_HOLD_MS)
      now = impact
      return
    }
    if (event.type === "move") {
      const cells = event.path ?? []
      if (cells.length < 2 || !cellOf.has(entity)) return
      closeHits()
      let cue: MoveCue
      if (event.carrier && (event.action === 50 || event.action === 51)) {
        closeShove()
        const pickup = event.action === 50
        const duration = pickup ? 250 : Math.min(500, 180 + cellDistance(board, cells[0]!, cells[cells.length-1]!) * 45)
        cue = { entity, cells, gait: "slide", start: now, end: now + duration }
        timeline.lifts.push({ rider: entity, carrier: event.carrier, pickup, start: now, end: cue.end })
        if (pickup) carrying.set(entity, event.carrier)
        else carrying.delete(entity)
        now = cue.end
      } else if ((event.value ?? 0) > 0) {
        closeShove()
        const start = Math.max(now, busy.get(entity) ?? 0)
        cue = { entity, cells, gait: "walk", start, end: start + (cells.length - 1) * WALK_MS }
        now = cue.end
      } else {
        // Pushed, pulled or carried elsewhere: everything one effect shoves moves at once.
        shove ??= { start: now, end: now, last: now }
        const start = Math.max(shove.start, busy.get(entity) ?? 0)
        const slides = contiguous(board, cells)
        const length = slides ? SLIDE_BASE_MS + (cells.length - 1) * SLIDE_MS : BLINK_MS
        cue = { entity, cells, gait: slides ? "slide" : "blink", start, end: start + length }
        shove.end = Math.max(shove.end, cue.end)
        shove.last = cue.end
      }
      timeline.moves.push(cue)
      busy.set(entity, cue.end)
      cellOf.set(entity, cells[cells.length - 1]!)
      for (const [rider, owner] of carrying) {
        if (owner === entity) cellOf.set(rider, cells[cells.length - 1]!)
      }
      end = Math.max(end, cue.end)
      free = Math.max(free, cue.end)
      return
    }
    if (event.type === "damage") {
      if (!cellOf.has(entity)) return
      if (event.push) {
        // What a shove costs lands when the shove stops.
        closeHits()
        shove ??= { start: now, end: now, last: now }
        strike(entity, shove.last, event)
        shove.end = Math.max(shove.end, shove.last + HIT_STAGGER_MS)
        return
      }
      closeShove()
      const at = now + hits * HIT_STAGGER_MS
      hits++
      hitsEnd = at + HIT_STAGGER_MS
      strike(entity, at, event)
      return
    }
    if (event.type === "heal") {
      if (!cellOf.has(entity)) return
      closeShove()
      const at = now + hits * HIT_STAGGER_MS
      hits++
      hitsEnd = at + HIT_STAGGER_MS
      const hp = Math.min(maxOf.get(entity) ?? 0, (hpOf.get(entity) ?? 0) + (event.value ?? 0))
      hpOf.set(entity, hp)
      timeline.health.push({ entity, at, hp })
      if (event.value) float(entity, at, { kind: "hp", amount: event.value, tone: "vitality", critical: false })
      return
    }
    if (event.type === "ap") {
      if (!cellOf.has(entity) || !event.value) return
      const at = now + hits * HIT_STAGGER_MS
      hits++
      hitsEnd = at + HIT_STAGGER_MS
      float(entity, at, { kind: "ap", amount: event.value, tone: "ap", critical: false })
      return
    }
    if (event.type === "defeat") {
      if (!cellOf.has(entity)) return
      // A training dummy falls, then stands back up with all its health.
      closeHits()
      closeShove()
      const from = source ?? cellOf.get(entity)!
      timeline.poses.push({ entity, pose: "down", start: now, end: now + TOPPLE_MS, dir: heading(board, from, cellOf.get(entity)!) || 1 })
      const hp = maxOf.get(entity) ?? 0
      hpOf.set(entity, hp)
      timeline.health.push({ entity, at: now + TOPPLE_MS * 0.8, hp })
      end = Math.max(end, now + TOPPLE_MS)
      free = Math.max(free, now + TOPPLE_MS)
    }
  })

  closeHits()
  closeShove()
  timeline.duration = Math.max(end, now)
  timeline.free = Math.min(timeline.duration, free)
  return timeline
}

export interface Actor {
  /** Where the entity is on the grid; fractional between two cells. */
  x: number
  y: number
  facing: Facing
  pose: Pose
  /** How far into its pose the entity is, from 0 to 1. */
  phase: number
  /** The way the blow or the shove travels. */
  dir: Facing
  /** Cells walked so far: the legs follow it. */
  stride: number
  /** The grid point a caster aims at. */
  aim?: { x: number; y: number }
  tone?: string
  direction?: Direction
  animation?: string
  spell?: number
  carriedBy?: number
  lift?: number
}

export interface Scene {
  actors: Record<number, Actor>
  bolts: { cue: BoltCue; progress: number }[]
  impacts: { cue: ImpactCue; progress: number }[]
  graphics: { cue: GraphicCue; progress: number; index: number }[]
}

// A walk picks up speed over its first half cell and slows over its last.
function glide(progress: number, cells: number): number {
  const ramp = Math.min(0.5, 0.5 / cells)
  const speed = 1 / (1 - ramp)
  if (progress < ramp) return (speed * progress * progress) / (2 * ramp)
  if (progress > 1 - ramp) return 1 - (speed * (1 - progress) * (1 - progress)) / (2 * ramp)
  return speed * (progress - ramp / 2)
}

/** The board at a moment of the timeline. `facing` is the way each entity looked when it started. */
export function sceneAt(timeline: Timeline, time: number, facing: Record<number, Facing>, directions: Record<number, Direction> = {}): Scene {
  const { board } = timeline
  const actors: Record<number, Actor> = {}
  for (const [id, cell] of Object.entries(timeline.start)) {
    const at = cellXY(board, cell)
    actors[Number(id)] = { x: at.x, y: at.y, facing: facing[Number(id)] ?? 1, direction: directions[Number(id)], pose: "idle", phase: 0, dir: 1, stride: 0 }
  }

  const beats = [
    ...timeline.turns.map((turn) => ({ at: turn.at, turn, move: undefined })),
    ...timeline.moves.map((move) => ({ at: move.start, turn: undefined, move })),
  ].sort((a, b) => a.at - b.at)
  for (const { at, turn, move } of beats) {
    if (at > time) break
    if (turn) {
      if (turn.facing !== undefined) actors[turn.entity]!.facing = turn.facing
      actors[turn.entity]!.direction = turn.direction
      continue
    }
    const actor = actors[move.entity]!
    const { cells } = move
    const steps = cells.length - 1
    const progress = time >= move.end ? 1 : (time - move.start) / (move.end - move.start)
    let walked = steps
    if (progress < 1) {
      actor.pose = move.gait
      actor.phase = progress
      if (move.gait === "walk") walked = glide(progress, steps) * steps
      else if (move.gait === "slide") walked = (1 - (1 - progress) ** 3) * steps
      else walked = progress < 0.5 ? 0 : steps
    }
    const step = Math.min(steps - 1, Math.floor(walked))
    const from = cellXY(board, cells[step]!)
    const to = cellXY(board, cells[step + 1]!)
    const part = walked - step
    actor.x = from.x + (to.x - from.x) * part
    actor.y = from.y + (to.y - from.y) * part
    actor.stride = walked
    const turned = heading(board, cells[step]!, cells[step + 1]!)
    if (move.gait === "walk") actor.direction = gameDirection(board, cells[step]!, cells[step + 1]!)
    if (move.gait === "walk" && turned !== 0) actor.facing = turned
    actor.dir = heading(board, cells[0]!, cells[steps]!) || 1
  }

  for (const cue of timeline.poses) {
    const actor = actors[cue.entity]
    if (!actor || time < cue.start || time >= cue.end) continue
    // A fall is not interrupted by the hit that caused it.
    if (actor.pose === "down") continue
    actor.pose = cue.pose
    actor.phase = (time - cue.start) / (cue.end - cue.start)
    actor.dir = cue.dir
    actor.tone = cue.tone
    actor.animation = cue.animation
    actor.spell = cue.spell
    actor.aim = cue.cell === undefined ? undefined : cellXY(board, cue.cell)
  }

  const carriers = { ...timeline.carriers }
  for (const lift of timeline.lifts) {
    if (time < lift.start) continue
    const actor = actors[lift.rider]
    if (!actor) continue
    if (lift.pickup && time >= lift.end) carriers[lift.rider] = lift.carrier
    else delete carriers[lift.rider]
    if (time < lift.end) {
      const phase = (time - lift.start) / (lift.end - lift.start)
      actor.lift = lift.pickup ? phase : 1-phase + 0.7 * Math.sin(Math.PI * phase)
    }
  }
  for (const [riderId, carrierId] of Object.entries(carriers)) {
    const rider = actors[Number(riderId)]
    const carrier = actors[carrierId]
    if (!rider || !carrier) continue
    rider.x = carrier.x
    rider.y = carrier.y
    rider.carriedBy = carrierId
    rider.lift = 1
  }

  const running = <Cue extends { start: number; end: number }>(cues: Cue[]) =>
    cues
      .filter((cue) => time >= cue.start && time < cue.end)
      .map((cue) => ({ cue, progress: (time - cue.start) / (cue.end - cue.start) }))
  const graphics = timeline.graphics.flatMap((cue, index) =>
    time < cue.start || time >= cue.end ? [] : [{ cue, progress: (time - cue.start) / (cue.end - cue.start), index }],
  )
  return { actors, bolts: running(timeline.bolts), impacts: running(timeline.impacts), graphics }
}

/** The way each entity looks once the timeline is over. */
export function facingAfter(timeline: Timeline, facing: Record<number, Facing>): Record<number, Facing> {
  const { actors } = sceneAt(timeline, timeline.duration, facing)
  return { ...facing, ...Object.fromEntries(Object.entries(actors).map(([id, actor]) => [id, actor.facing])) }
}

/** The cell each entity stands on once the timeline is over. */
export function cellsAfter(timeline: Timeline): Record<number, number> {
  const { actors } = sceneAt(timeline, timeline.duration, {})
  return Object.fromEntries(Object.entries(actors).map(([id, actor]) => [id, Math.round(actor.y) * timeline.board.width + Math.round(actor.x)]))
}
