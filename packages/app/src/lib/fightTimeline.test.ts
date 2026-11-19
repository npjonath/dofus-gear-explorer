import { describe, expect, it } from "vitest"
import type { FightEntity, FightEvent, FightState } from "../api/types"
import { groundGraphics, type SpellVisualCatalog } from "./gameSpellEffects"
import { buildTimeline, cellsAfter, facingAfter, HIT_HOLD_MS, HIT_MS, HIT_STAGGER_MS, sceneAt, TOPPLE_MS, WALK_MS } from "./fightTimeline"

const WIDTH = 13
const cell = (x: number, y: number) => y * WIDTH + x

function entity(id: number, kind: FightEntity["kind"], at: number, hp = 1000): FightEntity {
  return { id, kind, cell: at, hp, maxHp: 1000, buffs: [] }
}

function fight(...entities: FightEntity[]): FightState {
  return {
    board: { width: WIDTH, height: WIDTH, walls: [] },
    phase: "fight",
    turn: 1,
    entities,
    apUsed: 0,
    mpUsed: 0,
    uses: {},
    turnDamage: 0,
    totalDamage: 0,
  }
}

const PLAYER = 1
const DUMMY = 2
const looks = { [PLAYER]: 1, [DUMMY]: -1 } as const

describe("installed spell animations", () => {
  const before = fight(entity(PLAYER, "player", cell(2, 2)), entity(DUMMY, "dummy", cell(1, 1)))
  const catalog: SpellVisualCatalog = {
    spells: { 42: { normal: [
      { cast: "self-cast", targetMask: "c" },
      { cast: "attack", targetMask: "g,A,x", sequences: [{ end: "waitForAll", gfx: [{ kind: "target", bone: 8 }] }] },
    ] } },
    casts: { attack: { duration: 600, shot: 200 }, "self-cast": { duration: 700, shot: 350 } },
    clips: { 8: { animation: "FX", duration: 900, shot: 80, directions: [0] } },
  }

  it("starts the native target effect at the caster's SHOT and lands damage on the effect's SHOT", () => {
    const timeline = buildTimeline(before, [
      { type: "cast", spell: 42, entity: PLAYER, cell: cell(1, 1), critical: true },
      { type: "damage", entity: DUMMY, value: 100 },
    ], undefined, catalog)
    expect(timeline.poses[0]).toMatchObject({ animation: "attack", start: 0, end: 600 })
    expect(timeline.graphics[0]).toMatchObject({ bone: 8, start: 200, end: 1100, to: cell(1, 1) })
    expect(timeline.health[0]?.at).toBe(280)
    expect(sceneAt(timeline, 650, looks).graphics[0]?.progress).toBeCloseTo(0.5)
    expect(sceneAt(timeline, timeline.duration, looks).actors[PLAYER]?.direction).toBe(6)
    expect(timeline.bolts).toHaveLength(0)
  })

  it("selects the caster-only script on self-cast", () => {
    const timeline = buildTimeline(before, [{ type: "cast", spell: 42, entity: PLAYER, cell: cell(2, 2) }], undefined, catalog)
    expect(timeline.poses[0]?.animation).toBe("self-cast")
    expect(timeline.graphics).toHaveLength(0)
  })

  it("plays a triggered glyph on its victim, without making its owner cast again", () => {
    const timeline = buildTimeline(before, [
      { type: "mark", spell: 19, triggerSpell: 42, cell: cell(1, 2), entity: DUMMY },
      { type: "damage", entity: DUMMY, value: 100 },
    ], undefined, catalog)
    expect(timeline.graphics[0]).toMatchObject({ bone: 8, from: cell(1, 2), to: cell(1, 1) })
    expect(timeline.poses.map((pose) => pose.pose)).toEqual(["hit"])
  })

  it("plays a rune's spawn, idle, and conditional detonation at its empty tile", () => {
    const runeCatalog: SpellVisualCatalog = {
      spells: {
        10: { normal: [{ sequences: [{ end: "dontWait", gfx: [{ kind: "mark", bone: 1 }] }] }] },
        11: { normal: [{ casterMask: "C,*E123", sequences: [{ end: "dontWait", gfx: [{ kind: "target", bone: 2 }] }] }] },
      },
      casts: {},
      clips: {
        1: { animation: "spawn", idle: "idle", duration: 300, directions: [0] },
        2: { animation: "hit", duration: 200, directions: [0] },
      },
    }
    const at = cell(1, 2)
    const mark = { action: 2022, spell: 10, from: 9, index: 0, grade: 1, id: 3, cell: at, cells: [at], turns: 2, color: 0xff0000, caster: PLAYER }
    expect(groundGraphics(runeCatalog, mark)[0]?.idle).toBe("idle")
    const spawn = buildTimeline(before, [{ type: "rune", spell: 10, entity: PLAYER, cell: at }], undefined, runeCatalog)
    expect(spawn.graphics[0]).toMatchObject({ bone: 1, animation: "spawn", to: at })
    const activation = buildTimeline({ ...before, marks: [mark] }, [
      { type: "mark", spell: 10, triggerSpell: 11, action: 2022, cell: at, casterStates: [123] },
    ], undefined, runeCatalog)
    expect(activation.graphics[0]).toMatchObject({ bone: 2, animation: "hit", from: at, to: at })
  })
})

describe("carrying another fighter", () => {
  const p = cell(3,4), r = cell(3,5)
  it("raises the rider and keeps the rider on the carrier's path", () => {
    const st = fight(entity(PLAYER, "player", p), entity(DUMMY, "dummy", r))
    const lift = buildTimeline(st, [{ type:"move", entity:DUMMY, carrier:PLAYER, action:50, path:[r,p] }])
    expect(sceneAt(lift, lift.duration, looks).actors[DUMMY]).toMatchObject({x:3,y:4,carriedBy:PLAYER,lift:1})
    st.entities[1]!.cell=p
    st.entities[1]!.carriedBy=PLAYER
    const walk = buildTimeline(st,[{type:"move",entity:PLAYER,value:1,path:[p,cell(4,4)]}])
    const frame = sceneAt(walk, walk.duration/2, looks)
    expect(frame.actors[DUMMY]?.x).toBe(frame.actors[PLAYER]?.x)
    expect(cellsAfter(walk)[DUMMY]).toBe(cell(4,4))
  })
  it("throws from the carrier's hands and lands on the destination", () => {
    const st = fight(entity(PLAYER, "player", p), {...entity(DUMMY,"dummy",p),carriedBy:PLAYER})
    const tl = buildTimeline(st,[{type:"move",entity:DUMMY,carrier:PLAYER,action:51,path:[p,cell(7,4)]}])
    expect(sceneAt(tl, tl.duration/2, looks).actors[DUMMY]!.lift).toBeGreaterThan(.5)
    expect(sceneAt(tl, tl.duration, looks).actors[DUMMY]).toMatchObject({x:7,y:4})
    expect(sceneAt(tl, tl.duration, looks).actors[DUMMY]?.carriedBy).toBeUndefined()
  })
})

describe("a walk", () => {
  const before = fight(entity(PLAYER, "player", cell(2, 2)))
  // Two cells along the row, then one down the column.
  const path = [cell(2, 2), cell(3, 2), cell(4, 2), cell(4, 3)]
  const timeline = buildTimeline(before, [{ type: "move", entity: PLAYER, path, value: 3 }])

  it("takes the same time for each cell", () => {
    expect(timeline.duration).toBe(3 * WALK_MS)
    expect(timeline.free).toBe(timeline.duration)
    expect(timeline.moves).toEqual([{ entity: PLAYER, cells: path, gait: "walk", start: 0, end: 3 * WALK_MS }])
  })

  it("starts where the entity stood and ends on the last cell", () => {
    expect(sceneAt(timeline, 0, looks).actors[PLAYER]).toMatchObject({ x: 2, y: 2, pose: "walk" })
    expect(sceneAt(timeline, timeline.duration, looks).actors[PLAYER]).toMatchObject({ x: 4, y: 3, pose: "idle" })
    expect(cellsAfter(timeline)).toEqual({ [PLAYER]: cell(4, 3) })
  })

  it("goes through each cell in turn, never jumping", () => {
    let last = sceneAt(timeline, 0, looks).actors[PLAYER]!
    for (let time = 10; time <= timeline.duration; time += 10) {
      const actor = sceneAt(timeline, time, looks).actors[PLAYER]!
      expect(Math.abs(actor.x - last.x) + Math.abs(actor.y - last.y)).toBeLessThan(0.2)
      // It stays on the path: on the row until its end, then on the column.
      expect(actor.y === 2 || actor.x === 4).toBe(true)
      last = actor
    }
    expect(sceneAt(timeline, timeline.duration / 2, looks).actors[PLAYER]).toMatchObject({ x: 3.5, y: 2 })
  })

  it("faces the way it goes", () => {
    expect(sceneAt(timeline, WALK_MS, looks).actors[PLAYER]!.facing).toBe(1)
    // Down a column is to the left of the drawing.
    expect(sceneAt(timeline, 2.5 * WALK_MS, looks).actors[PLAYER]!.facing).toBe(-1)
    expect(facingAfter(timeline, looks)).toEqual({ [PLAYER]: -1, [DUMMY]: -1 })
  })
})

describe("a cast from afar", () => {
  const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(7, 6)))
  const events: FightEvent[] = [
    { type: "cast", entity: PLAYER, spell: 9, cell: cell(7, 6), critical: true },
    { type: "damage", entity: DUMMY, value: 300, element: 2, critical: true },
    { type: "damage", entity: DUMMY, value: 50, element: 3, critical: true },
    { type: "move", entity: DUMMY, path: [cell(7, 6), cell(8, 6), cell(9, 6)] },
    { type: "damage", entity: DUMMY, value: 40, push: true },
  ]
  const zone = [cell(7, 6), cell(7, 5), cell(7, 7)]
  const timeline = buildTimeline(before, events, zone)
  const [bolt] = timeline.bolts
  const [impact] = timeline.impacts
  const [slide] = timeline.moves

  it("winds up, then sends a projectile in the tone of the first loss", () => {
    expect(timeline.poses[0]).toMatchObject({ entity: PLAYER, pose: "windup", start: 0, cell: cell(7, 6), tone: "fire" })
    expect(bolt).toMatchObject({ from: cell(2, 6), to: cell(7, 6), tone: "fire", critical: true, onEntity: true })
    expect(bolt!.start).toBeGreaterThan(0)
    expect(bolt!.start).toBeLessThan(timeline.poses[0]!.end)
  })

  it("flashes the cells the cast covered when the projectile lands", () => {
    expect(impact).toMatchObject({ cells: zone, center: cell(7, 6), critical: true })
    expect(impact!.start).toBe(bolt!.end)
  })

  it("lands the losses one after the other, each in its element's tone", () => {
    expect(timeline.floats.map((float) => [float.at, float.amount, float.tone, float.critical, float.rank])).toEqual([
      [impact!.start, -300, "fire", true, 0],
      [impact!.start + HIT_STAGGER_MS, -50, "water", true, 1],
      [slide!.end, -40, "plain", false, 2],
    ])
  })

  it("shoves the target after the losses, faster than a walk and without turning it", () => {
    expect(slide).toMatchObject({ entity: DUMMY, gait: "slide" })
    expect(slide!.start).toBe(impact!.start + 2 * HIT_STAGGER_MS)
    expect((slide!.end - slide!.start) / 2).toBeLessThan(WALK_MS)
    const actor = sceneAt(timeline, (slide!.start + slide!.end) / 2, looks).actors[DUMMY]!
    expect(actor.x).toBeGreaterThan(7)
    expect(actor.x).toBeLessThan(9)
    expect(actor.facing).toBe(-1)
  })

  it("takes health down as each blow lands", () => {
    expect(timeline.health).toEqual([
      { entity: DUMMY, at: impact!.start, hp: 700 },
      { entity: DUMMY, at: impact!.start + HIT_STAGGER_MS, hp: 650 },
      { entity: DUMMY, at: slide!.end, hp: 610 },
    ])
    // The number rises from where the target stands at that moment.
    expect(timeline.floats.map((float) => float.cell)).toEqual([cell(7, 6), cell(7, 6), cell(9, 6)])
  })

  it("lasts until the last recoil is over, and ends on the engine's state", () => {
    expect(timeline.duration).toBe(slide!.end + HIT_MS)
    // The board is the player's again while the target still recoils.
    expect(timeline.free).toBe(slide!.end + HIT_HOLD_MS)
    const end = sceneAt(timeline, timeline.duration, looks)
    expect(end.actors[DUMMY]).toMatchObject({ x: 9, y: 6, pose: "idle" })
    expect(end.actors[PLAYER]).toMatchObject({ x: 2, y: 6, pose: "idle", facing: 1 })
    expect(end.bolts).toEqual([])
    expect(end.impacts).toEqual([])
    expect(cellsAfter(timeline)).toEqual({ [PLAYER]: cell(2, 6), [DUMMY]: cell(9, 6) })
  })

  it("shows each thing in its time", () => {
    const flying = sceneAt(timeline, (bolt!.start + bolt!.end) / 2, looks)
    expect(flying.bolts).toHaveLength(1)
    expect(flying.bolts[0]!.progress).toBeCloseTo(0.5)
    expect(flying.impacts).toEqual([])
    expect(flying.actors[DUMMY]!.pose).toBe("idle")
    const struck = sceneAt(timeline, impact!.start + 10, looks)
    expect(struck.bolts).toEqual([])
    expect(struck.impacts).toHaveLength(1)
    // The blow came from the left of the drawing: the target is thrown to the right.
    expect(struck.actors[DUMMY]).toMatchObject({ pose: "hit", dir: 1 })
  })

  it("turns the caster towards its target", () => {
    const back = buildTimeline(fight(entity(PLAYER, "player", cell(7, 6)), entity(DUMMY, "dummy", cell(2, 6))), [
      { type: "cast", entity: PLAYER, cell: cell(2, 6) },
    ])
    expect(sceneAt(back, 1, looks).actors[PLAYER]!.facing).toBe(-1)
    expect(facingAfter(back, looks)[PLAYER]).toBe(-1)
  })
})

describe("other casts", () => {
  it("strikes a neighbour without a projectile", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(3, 6)))
    const timeline = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(3, 6) },
      { type: "damage", entity: DUMMY, value: 10 },
    ])
    expect(timeline.bolts).toEqual([])
    expect(timeline.poses[0]).toMatchObject({ pose: "lunge", tone: "neutral" })
    expect(sceneAt(timeline, 20, looks).actors[PLAYER]!.aim).toEqual({ x: 3, y: 6 })
    // Without the zone the preview showed, the aimed cell alone flashes.
    expect(timeline.impacts[0]!.cells).toEqual([cell(3, 6)])
  })

  it("claims no element when nothing is lost, and the Vitality tone for a heal", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6), 400), entity(DUMMY, "dummy", cell(6, 6)))
    const boost = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(6, 6) },
      { type: "buff", entity: DUMMY, stat: 1, value: -2, turns: 2 },
    ])
    expect(boost.bolts[0]).toMatchObject({ tone: "plain", critical: false })
    expect(boost.floats).toEqual([])

    const heal = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(2, 6) },
      { type: "heal", entity: PLAYER, value: 900 },
      { type: "ap", entity: PLAYER, value: 2 },
    ])
    // On its own cell nothing flies; health never goes over its maximum.
    expect(heal.bolts).toEqual([])
    expect(heal.poses[0]).toMatchObject({ pose: "windup", tone: "vitality" })
    expect(heal.health).toEqual([{ entity: PLAYER, at: heal.impacts[0]!.start, hp: 1000 }])
    expect(heal.floats.map((float) => [float.kind, float.amount, float.tone])).toEqual([
      ["hp", 900, "vitality"],
      ["ap", 2, "ap"],
    ])
  })

  it("moves everything one effect shoves at the same time", () => {
    const other = 3
    const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(5, 6)), entity(other, "dummy", cell(5, 7)))
    const timeline = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(5, 6) },
      { type: "move", entity: DUMMY, path: [cell(5, 6), cell(6, 6)] },
      { type: "move", entity: other, path: [cell(5, 7), cell(6, 7), cell(7, 7)] },
    ])
    expect(timeline.moves[0]!.start).toBe(timeline.moves[1]!.start)
    expect(timeline.duration).toBe(Math.max(timeline.moves[1]!.end, timeline.impacts[0]!.end))
  })

  it("carries an entity to a cell that is not next to it without sliding it across", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(8, 6)))
    const timeline = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(8, 6) },
      { type: "move", entity: PLAYER, path: [cell(2, 6), cell(8, 6)] },
      { type: "move", entity: DUMMY, path: [cell(8, 6), cell(2, 6)] },
    ])
    const [first, second] = timeline.moves
    expect(first).toMatchObject({ gait: "blink" })
    expect(second!.start).toBe(first!.start)
    const early = sceneAt(timeline, first!.start + 10, looks).actors[PLAYER]!
    const late = sceneAt(timeline, first!.end - 10, looks).actors[PLAYER]!
    expect([early.x, late.x]).toEqual([2, 8])
    expect(cellsAfter(timeline)).toEqual({ [PLAYER]: cell(8, 6), [DUMMY]: cell(2, 6) })
  })

  it("topples a defeated dummy, then stands it back up with all its health", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(6, 6), 120))
    const timeline = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(6, 6) },
      { type: "damage", entity: DUMMY, value: 500, element: 4 },
      { type: "defeat", entity: DUMMY },
    ])
    const fall = timeline.poses.find((pose) => pose.pose === "down")!
    expect(fall.start).toBe(timeline.impacts[0]!.start + HIT_STAGGER_MS)
    expect(timeline.health.map((cue) => cue.hp)).toEqual([0, 1000])
    expect(timeline.health[1]!.at).toBeGreaterThan(fall.start)
    expect(timeline.duration).toBe(fall.start + TOPPLE_MS)
    expect(timeline.free).toBe(timeline.duration)
    // The fall is not cut short by the recoil of the blow that caused it.
    expect(sceneAt(timeline, fall.start + 10, looks).actors[DUMMY]!.pose).toBe("down")
    expect(sceneAt(timeline, timeline.duration, looks).actors[DUMMY]!.pose).toBe("idle")
  })

  it("plays no gesture for what a fight opens with, which is aimed at nothing", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6), 600), entity(DUMMY, "dummy", cell(6, 6)))
    const timeline = buildTimeline(before, [
      { type: "turn", value: 1 },
      { type: "cast", entity: PLAYER, spell: 7 },
      { type: "buff", entity: PLAYER, stat: 1, value: 1, turns: -1 },
      { type: "cast", entity: PLAYER, spell: 8 },
      { type: "heal", entity: PLAYER, value: 100 },
    ])
    expect(timeline.poses).toEqual([])
    expect(timeline.impacts).toEqual([])
    expect(timeline.floats.map((float) => [float.at, float.amount])).toEqual([[0, 100]])
    expect(timeline.health).toEqual([{ entity: PLAYER, at: 0, hp: 700 }])
  })

  it("lands what waited for the turn without a cast", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(6, 6)))
    const timeline = buildTimeline(before, [
      { type: "damage", entity: DUMMY, value: 80, element: 3 },
      { type: "turn", value: 2 },
    ])
    expect(timeline.bolts).toEqual([])
    expect(timeline.floats.map((float) => [float.at, float.amount, float.tone])).toEqual([[0, -80, "water"]])
    expect(timeline.duration).toBe(HIT_MS)
  })

  it("shows nothing for a loss of zero, and nothing at all without events", () => {
    const before = fight(entity(PLAYER, "player", cell(2, 6)), entity(DUMMY, "dummy", cell(3, 6)))
    const none = buildTimeline(before, [{ type: "turn", value: 2 }])
    expect(none.duration).toBe(0)
    const resisted = buildTimeline(before, [
      { type: "cast", entity: PLAYER, cell: cell(3, 6) },
      { type: "damage", entity: DUMMY, value: 0, element: 1 },
    ])
    expect(resisted.floats).toEqual([])
    expect(resisted.poses.filter((pose) => pose.pose === "hit")).toEqual([])
  })
})
