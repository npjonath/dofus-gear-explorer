import { describe, expect, it } from "vitest"
import { statLabel, toneOf, toneOfElement } from "./stats"

describe("stat colours", () => {
  it("gives each characteristic the colour of its element", () => {
    expect(toneOf("strength")).toBe("earth")
    expect(toneOf("earthElementResistPercent")).toBe("earth")
    expect(toneOf("fireDamageBonus")).toBe("fire")
    expect(toneOf("chance")).toBe("water")
    expect(toneOf("airElementReduction")).toBe("air")
    expect(toneOf("actionPoints")).toBe("ap")
    expect(toneOf("pushDamageBonus")).toBe("plain")
    expect(toneOf(undefined)).toBe("plain")
  })

  it("maps weapon damage elements", () => {
    expect(toneOfElement(0)).toBe("neutral")
    expect(toneOfElement(2)).toBe("fire")
    expect(toneOfElement(null)).toBe("plain")
  })

  it("adds the unit to a label", () => {
    expect(statLabel({ id: 18, key: "criticalHit", label: "Critique", unit: "%", categoryId: 2, forgeMax: 10 })).toBe("Critique (%)")
    expect(statLabel({ id: 10, key: "strength", label: "Force", unit: "", categoryId: 2, forgeMax: 101 })).toBe("Force")
  })
})
