import { render } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { FightPortals } from "./FightPortals"
import { buildTimeline } from "../../lib/fightTimeline"
import type { FightState } from "../../api/types"
import type { SpellVisualCatalog } from "../../lib/gameSpellEffects"

describe("Eliotrope portals", () => {
  const board = { width: 10, height: 15, walls: [] }

  it("shows placement order and the measured distance and bonus only while pointing at a connected portal", () => {
    const props = {
      board, portals: [106, 86].map((cell) => ({ caster: 1, cell, spell: 14574, grade: 3 })),
      routes: { 106: { cells: [106, 86], distance: 2, bonus: 4 } }, events: [], assetRoot: "", scale: 1,
    }
    const { container, rerender } = render(<svg><FightPortals {...props} cursor={106} /></svg>)
    const texts = () => [...container.querySelectorAll("text")].map((el) => el.textContent)
    expect(texts()).toEqual(["1","2",expect.stringMatching(/2.*4/)])
    expect(container.querySelectorAll("polyline")).toHaveLength(2)
    rerender(<svg><FightPortals {...props} cursor={100} /></svg>)
    expect(texts()).toEqual(["1","2"])
    expect(container.querySelector("polyline")).toBeNull()
  })

  it("aims the caster at the entrance and splits a native projectile at the exit", () => {
    const before: FightState = {
      board, phase: "fight", turn: 1, entities: [
        { id: 1, kind: "player", hp: 100, maxHp: 100, cell: 116, buffs: [] },
        { id: 2, kind: "dummy", hp: 100, maxHp: 100, cell: 124, buffs: [] },
      ], apUsed: 0, mpUsed: 0, turnDamage: 0, totalDamage: 0, uses: {},
    }
    const catalog: SpellVisualCatalog = {
      spells: { 10: { normal: [{ cast: "shot", sequences: [{end:"waitForAll",gfx:[{bone:4,kind:"projectile"}]}]}] } },
      clips: { 4: { animation: "FX", directions: [0], duration: 400 } }, casts: { shot: {duration:100} },
    }
    const result = buildTimeline(before, [
      { type:"cast",entity:1,cell:124,spell:10,portalPath:[106,134] },
      { type:"damage",entity:2,value:40 },
    ],undefined,catalog)
    expect(result.poses[0]?.cell).toBe(106)
    expect(result.graphics.map(({from,to})=>[from,to])).toEqual([[116,106],[134,124]])
    expect(result.graphics[0]?.end).toBe(result.graphics[1]?.start)
  })
})
