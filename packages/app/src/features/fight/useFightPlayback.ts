import * as React from "react"
import type { FightEvent, FightState } from "../../api/types"
import { buildTimeline, cellsAfter, sceneAt, type Facing, type FloatCue, type Timeline } from "../../lib/fightTimeline"
import type { Direction, SpellVisualCatalog } from "../../lib/gameSpellEffects"

/** A number rising from an entity: health lost or gained, points given back. */
export interface Float {
  id: number
  run: number
  entity: number
  cell: number
  kind: "player" | "dummy" | "summon"
  text: string
  /** A stat tone, or "plain". */
  tone: string
  critical: boolean
  /** How many numbers rose from the same entity before it. */
  rank: number
  /** Milliseconds before it shows, counted from when the action was played. */
  delay: number
}

/** One action being played on the board. */
export interface Run {
  id: number
  key: string
  timeline: Timeline
  /** On the clock of `performance.now()`. */
  startedAt: number
  /** The way each entity looked when the action started. */
  facing: Record<number, Facing>
  directions: Record<number, Direction>
}

export interface Playback {
  visualCatalog?: SpellVisualCatalog
  run: Run | null
  /** True while the board is still catching up with the fight. */
  playing: boolean
  /** True once the board is the player's again: only a recoil or a number may still be playing. */
  settled: boolean
  /** Health to show instead of the true one, while the losses have not landed yet. */
  hp: Record<number, number> | null
  facing: Record<number, Facing>
  directions: Record<number, Direction>
  floats: Float[]
  /** Plays the events that led from `before`; `zone` is what the cast was shown to cover. */
  play(before: FightState, events: FightEvent[], text: (cue: FloatCue) => string, zone?: number[]): void
  /** Jumps to the end of what is playing. */
  skip(): void
  /** The board reached the end of a run by itself. */
  finish(run: number): void
  /** Forgets everything: a new fight starts. */
  reset(): void
  endFloat(id: number): void
}

let sequence = 0

/** The way a piece looks before anything turned it: the player to the right, the dummies back at it. */
export function defaultFacing(kind: "player" | "dummy" | "summon"): Facing {
  return kind === "player" ? 1 : -1
}

function reducedMotion(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches
}

// Numbers that were still waiting when the board jumps ahead show at once, one after the other.
function hurry(floats: Float[], run: number, elapsed: number): Float[] {
  let late = 0
  return floats.map((float) => {
    if (float.run !== run || float.delay <= elapsed) return float
    return { ...float, delay: elapsed + late++ * 90 }
  })
}

/**
 * Plays each action of a fight on the board. The fight itself is always the
 * state the engine returned; this only holds what the eye is shown on the way
 * there. `key` names the fight: what belongs to another fight is ignored.
 */
export function useFightPlayback(key: string, visuals?: SpellVisualCatalog): Playback {
  const [run, setRun] = React.useState<Run | null>(null)
  const [done, setDone] = React.useState(0)
  const [freed, setFreed] = React.useState(0)
  const [health, setHealth] = React.useState<{ run: number; values: Record<number, number> } | null>(null)
  const [looks, setLooks] = React.useState<{ key: string; values: Record<number, Facing>; directions: Record<number, Direction> }>({ key, values: {}, directions: {} })
  const [floats, setFloats] = React.useState<Float[]>([])

  const live = run?.key === key ? run : null
  const playing = live !== null && done !== live.id
  const facing = looks.key === key ? looks.values : {}
  const directions = looks.key === key ? looks.directions : {}

  React.useEffect(() => {
    if (!live || !playing) return
    const wait = (at: number) => Math.max(0, live.startedAt + at - performance.now())
    const timers = live.timeline.health.map((cue) =>
      window.setTimeout(() => {
        setHealth((current) => (current?.run === live.id ? { run: live.id, values: { ...current.values, [cue.entity]: cue.hp } } : current))
      }, wait(cue.at))
    )
    timers.push(window.setTimeout(() => setFreed(live.id), wait(live.timeline.free)))
    timers.push(window.setTimeout(() => setDone(live.id), wait(live.timeline.duration)))
    return () => timers.forEach((timer) => window.clearTimeout(timer))
  }, [live, playing])

  function skip() {
    if (!live || !playing) return
    setDone(live.id)
    setFloats((current) => hurry(current, live.id, performance.now() - live.startedAt))
  }

  function play(before: FightState, events: FightEvent[], text: (cue: FloatCue) => string, zone?: number[]) {
    const timeline = buildTimeline(before, events, zone, visuals)
    const id = ++sequence
    const still = reducedMotion()
    const kinds = new Map(before.entities.map((entity) => [entity.id, entity.kind]))
    const looked = Object.fromEntries(before.entities.map((entity) => [entity.id, facing[entity.id] ?? defaultFacing(entity.kind)]))
    const ends = cellsAfter(timeline)
    const risen: Float[] = timeline.floats.map((cue, index) => ({
      id: ++sequence,
      run: id,
      entity: cue.entity,
      cell: still ? (ends[cue.entity] ?? cue.cell) : cue.cell,
      kind: kinds.get(cue.entity) ?? "dummy",
      text: text(cue),
      tone: cue.tone,
      critical: cue.critical,
      rank: cue.rank,
      // Without motion nothing is waited for: the numbers follow each other at once.
      delay: still ? index * 140 : cue.at,
    }))
    const elapsed = live && playing ? performance.now() - live.startedAt : 0
    setFloats((current) => [...(live && playing ? hurry(current, live.id, elapsed) : current), ...risen])
    const final = sceneAt(timeline, timeline.duration, looked, directions).actors
    const nextDirections = { ...directions }
    const nextFacing = { ...looked }
    for (const [entity, actor] of Object.entries(final)) {
      nextFacing[Number(entity)] = actor.facing
      if (actor.direction !== undefined) nextDirections[Number(entity)] = actor.direction
    }
    setLooks({ key, values: nextFacing, directions: nextDirections })
    if (timeline.duration === 0 || still) {
      // Nothing to watch: the board shows the fight as it stands.
      setRun({ id, key, timeline, startedAt: performance.now(), facing: looked, directions })
      setDone(id)
      setHealth(null)
      return
    }
    // Health waits for the blow: entities that lose or gain some keep what they had.
    const held: Record<number, number> = {}
    for (const cue of timeline.health) held[cue.entity] ??= before.entities.find((entity) => entity.id === cue.entity)?.hp ?? cue.hp
    setRun({ id, key, timeline, startedAt: performance.now(), facing: looked, directions })
    setHealth({ run: id, values: held })
  }

  function reset() {
    setRun(null)
    setHealth(null)
    setFloats([])
    setLooks({ key, values: {}, directions: {} })
  }

  return {
    visualCatalog: visuals,
    run: live,
    playing,
    settled: !playing || freed === live.id,
    finish: setDone,
    hp: playing && health?.run === live.id ? health.values : null,
    facing,
    directions,
    floats,
    play,
    skip,
    reset,
    endFloat: (id) => setFloats((current) => current.filter((float) => float.id !== id)),
  }
}
