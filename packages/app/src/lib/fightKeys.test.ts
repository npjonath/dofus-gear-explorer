import { describe, expect, it } from "vitest"
import { bindFightKey, combatCodes, loadFightKeys, saveFightKeys } from "./fightKeys"

describe("combat shortcut bindings", () => {
  it("swaps an occupied slot and restores it on reopening the fight", () => {
    const swapped = bindFightKey(combatCodes, 0, "Digit2")
    expect(swapped.slice(0, 2)).toEqual(["Digit2", "Digit1"])
    expect(combatCodes.slice(0, 2)).toEqual(["Digit1", "Digit2"])
    saveFightKeys(swapped)
    expect(loadFightKeys()).toEqual(swapped)
    saveFightKeys(combatCodes)
  })

  it("keeps Escape and Tab available to cancel and inspect terrain", () => {
    for (const reserved of ["Escape", "Tab"]) expect(bindFightKey(combatCodes, 0, reserved)).toBe(combatCodes)
  })
})
