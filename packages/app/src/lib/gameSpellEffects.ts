import type { FightBoard, FightEntity, FightMark } from "../api/types"
import { cellXY } from "./fightBoard"

export type Direction = 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7
type GraphicKind = "caster" | "target" | "mark" | "trail" | "projectile"

interface Graphic {
  kind: GraphicKind
  bone: number
  strata?: string
  isOriented?: boolean
  scaleX?: number
  scaleY?: number
  offsetY?: number
  curvature?: number
}

interface BoundScript {
  cast?: string
  targetMask?: string
  casterMask?: string
  activationMask?: string
  criterion?: string
  random?: number
  spellLevels?: number[]
  sequences?: { gfx: Graphic[]; end: string }[]
}

interface Timing {
  duration: number
  shot?: number
}

export interface SpellVisualCatalog {
  spells: Record<string, { normal: BoundScript[]; critical?: BoundScript[] }>
  casts: Record<string, Timing>
  clips: Record<string, Timing & { animation: string; directions: Direction[]; idle?: string; trigger?: Timing & { animation: string } }>
}

export interface GraphicCue extends Graphic {
  start: number
  end: number
  animation: string
  from: number
  to: number
  direction: Direction
}

export function groundGraphics(catalog: SpellVisualCatalog | undefined, mark: FightMark) {
  const entry = catalog?.spells[mark.from]
  if (!entry || !catalog) return []
  const seen = new Set<number>()
  return (mark.critical ? entry.critical ?? entry.normal : entry.normal).flatMap((binding) =>
    (binding.sequences ?? []).flatMap((step) =>
      step.gfx.flatMap((gfx) => {
        const clip = catalog.clips[gfx.bone]
        if (gfx.kind !== "mark" || !clip?.idle || seen.has(gfx.bone)) return []
        seen.add(gfx.bone)
        return [{ gfx, idle: clip.idle, trigger: clip.trigger }]
      }),
    ),
  )
}

export function gameDirection(board: FightBoard, from: number, to: number): Direction | undefined {
  if (from === to) return undefined
  const start = cellXY(board, from)
  const end = cellXY(board, to)
  const dx = end.x - start.x
  const dy = end.y - start.y
  return ((Math.round((Math.atan2(dx + dy, dx - dy) * 4) / Math.PI) + 8) % 8) as Direction
}

function maskMatches(mask: string | undefined, caster: FightEntity, target: FightEntity | undefined): boolean {
  if (!mask) return true
  const names = new Set<string>()
  const monsterIds: number[] = []
  for (const token of mask.split(",")) {
    const condition = /^(\*?)([EeFfVv])(\d+)$/.exec(token)
    if (condition) {
      const fighter = condition[1] ? caster : target
      const value = Number(condition[3])
      switch (condition[2]) {
        case "E":
        case "e": {
          const has = fighter?.buffs.some((buff) => buff.kind === "state" && buff.ref === value) ?? false
          if (has !== (condition[2] === "E")) return false
          break
        }
        case "F":
          monsterIds.push(value)
          break
        case "f":
          if (fighter?.monsterId === value) return false
          break
        case "V":
        case "v":
          if (!fighter || (100 * fighter.hp <= value * fighter.maxHp) !== (condition[2] === "V")) return false
          break
      }
    } else if (/^[CcAaHhLlMmxgJjIiSsDd]$/.test(token)) {
      names.add(token)
    } else if (token === "p" || token === "u") {
      continue
    } else {
      // A conditional graphic can be omitted when the scene cannot establish its condition.
      return false
    }
  }
  if (monsterIds.length && (!target?.monsterId || !monsterIds.includes(target.monsterId))) return false
  if (names.size === 0) return true
  if (!target) return names.has("x")
  if (caster.id === target.id) return ["c", "C", "a", "h", "l"].some((token) => names.has(token))
  const ally = (caster.kind === "dummy") === (target.kind === "dummy")
  const allowed = ally ? ["a", "h", "l", "g"] : ["A", "H", "L", "M"]
  if (target.kind === "summon") allowed.push(...(ally ? ["i", "j", "s"] : ["I", "J", "S"]))
  return allowed.some((token) => names.has(token))
}

interface CastVisual {
  animation?: string
  poseEnd: number
  hit: number
  end: number
  graphics: GraphicCue[]
}

export function castVisual(
  catalog: SpellVisualCatalog | undefined,
  id: number,
  critical: boolean,
  caster: FightEntity,
  target: FightEntity | undefined,
  board: FightBoard,
  from: number,
  to: number,
  start: number,
): CastVisual | undefined {
  const entry = catalog?.spells[id]
  if (!catalog || !entry) return undefined
  const bindings = (critical ? entry.critical ?? entry.normal : entry.normal).filter((binding) =>
    !binding.criterion && !binding.spellLevels?.length && !binding.random &&
    maskMatches(binding.casterMask, caster, caster) && maskMatches(binding.targetMask, caster, target) &&
    maskMatches(binding.activationMask, caster, target),
  )
  if (!bindings.length) return undefined
  const animation = bindings.find((binding) => binding.cast)?.cast
  const cast = animation ? catalog.casts[animation] : undefined
  const poseEnd = start + (cast?.duration ?? 300)
  let time = start + (cast?.shot ?? 0)
  let hit: number | undefined
  let end = poseEnd
  const graphics: GraphicCue[] = []
  const facing = gameDirection(board, from, to) ?? 1

  for (const binding of bindings) {
    for (const step of binding.sequences ?? []) {
      const advances: number[] = []
      for (const gfx of step.gfx) {
        const clip = catalog.clips[gfx.bone]
        if (!clip) continue
        const direction = gfx.isOriented === false || clip.directions.length === 1 ? clip.directions[0] ?? 0 : facing
        const endTime = time + clip.duration
        graphics.push({ ...gfx, from, to, direction, animation: clip.animation, start: time, end: endTime })
        end = Math.max(end, endTime)
        if (gfx.kind === "target") hit = time + (clip.shot ?? 0)
        advances.push(clip.shot ?? clip.duration)
      }
      if (advances.length && step.end !== "dontWait") {
        time += step.end === "waitForFirstOne" ? Math.min(...advances) : Math.max(...advances)
      }
    }
  }
  if (!animation && !graphics.length) return undefined
  return { animation, poseEnd, graphics, hit: hit ?? time, end: Math.max(time, end) }
}
