import { describe, expect, it } from "vitest"
import { targetKeys } from "./targets"

describe("who a line of a spell applies to", () => {
  it("names the common groups in one word", () => {
    expect(targetKeys(["ally", "enemy"])).toEqual(["target.all"])
    expect(targetKeys(["allyOther", "enemy"])).toEqual(["target.allButCaster"])
    expect(targetKeys(["enemySummon", "allySummon"])).toEqual(["target.summons"])
    expect(targetKeys(["allyPlayer", "allyMonster", "allyCompanion", "enemyPlayer", "enemyMonster", "enemyCompanion"])).toEqual([
      "target.notSummons",
    ])
    expect(targetKeys(["allyPlayerCompanion", "allyMonster", "enemyPlayerCompanion", "enemyMonster"])).toEqual(["target.notSummons"])
  })

  it("lists the others one by one", () => {
    expect(targetKeys(["enemy"])).toEqual(["target.enemy"])
    expect(targetKeys(["caster"])).toEqual(["target.caster"])
    expect(targetKeys(["enemyPlayer", "enemyMonster", "enemySummon"])).toEqual([
      "target.enemyPlayer",
      "target.enemyMonster",
      "target.enemySummon",
    ])
    expect(targetKeys([])).toEqual([])
  })
})
