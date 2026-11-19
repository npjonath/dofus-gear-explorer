import { afterEach, expect, it } from "vitest"
import { combatCodes, loadFightKeys, saveFightKeys } from "../../lib/fightKeys"
import { initialPreferences, readPreferences } from "./combatPreferences"

afterEach(() => localStorage.clear())

it("recovers invalid display settings and constrains numeric values", () => {
  localStorage.setItem("dde.fight.display.v1", "null")
  expect(readPreferences()).toEqual(initialPreferences)
  localStorage.setItem(
    "dde.fight.display.v1",
    JSON.stringify({ rows: 9, scale: 65, showReach: true, light: "false", chatSize: -5 })
  )
  expect(readPreferences()).toEqual({ ...initialPreferences, scale: 65, showReach: true, chatSize: 10 })
})

it("adds the terrain shortcut without taking a previously remapped key", () => {
  const old = combatCodes.slice(0, -1)
  old[0] = "KeyY"
  saveFightKeys(old)
  const restored = loadFightKeys()
  expect(restored.slice(0, -1)).toEqual(old)
  expect(restored.at(-1)).toBe("Digit1")
  expect(new Set(restored).size).toBe(restored.length)
})
