import { describe, expect, it } from "vitest"
import type { FightState } from "../api/types"
import { buildTimeline } from "./fightTimeline"
import { fightSoundCues, type SoundCatalog } from "./fightAudio"

const before: FightState = {
  board: { width: 13, height: 13, walls: [] }, phase: "fight", turn: 1, apUsed: 0, mpUsed: 0, uses: {}, turnDamage: 0, totalDamage: 0,
  entities: [{ id: 1, kind: "player", cell: 0, hp: 100, maxHp: 100, buffs: [] }, { id: 2, kind: "dummy", cell: 3, hp: 100, maxHp: 100, buffs: [] }],
}
const sounds: SoundCatalog = {
  maps: {}, voices: {}, ui: {}, dummy: "431", creatures: {}, weapons: {}, categories: ["effects"],
  poutchHit: ["straw"], animations: { "8975": { FX: [[36,"fire"]] } },
  events: { fire: { channel: "effects", layers: [[0,["clip"]]] } },
}
const timeline = buildTimeline(before, [{ type:"cast",spell:1,cell:3,entity:1 },{type:"damage",entity:2,value:15}])
timeline.graphics.push({kind:"target",bone:8975,animation:"FX",from:0,to:3,start:350,end:1200,direction:0})
describe("local fight audio", () => {
  it("starts the sound at the frame named by the game and mutes the straw hit independently", () => {
    expect(fightSoundCues(sounds,timeline,before.entities,4,0,false)).toEqual([{at:386,event:"fire"}])
    expect(fightSoundCues(sounds,timeline,before.entities,4,0,true)).toContainEqual({at:timeline.health[0]!.at,take:["straw"]})
  })
})
