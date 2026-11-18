import { describe, expect, it } from "vitest"
import type { ElementEntry } from "../api/types"
import { addElement, elementsOf, maximizedAttack, spellHitsIn } from "./maximize"

describe("what the search receives for an entry to maximize", () => {
  it("keeps a weapon or a spell as it is", () => {
    expect(maximizedAttack({ kind: "weapon", id: 7, measure: "crit", element: "fire" })).toEqual({
      kind: "weapon",
      id: 7,
      measure: "crit",
      element: "fire",
    })
  })

  it("leaves the element out when the search picks it", () => {
    expect(maximizedAttack({ kind: "element", element: "auto", measure: "mean" })).toEqual({
      kind: "element",
      id: 0,
      measure: "mean",
      element: undefined,
    })
    expect(maximizedAttack({ kind: "element", element: "water", measure: "mean", others: [] }).element).toBe("water")
  })

  it("sends several elements with the main one first, held to a floor by default", () => {
    const entry: ElementEntry = { kind: "element", element: "water", measure: "mean", others: [{ element: "fire", share: 70 }] }
    expect(maximizedAttack(entry)).toEqual({
      kind: "elements",
      id: 0,
      measure: "mean",
      mix: "floor",
      parts: [
        { element: "water", share: 100 },
        { element: "fire", share: 70 },
      ],
    })
    const targeted: ElementEntry = {
      ...entry,
      target: { kind: "spell", id: 12 },
      others: [{ element: "fire", share: 70, target: { kind: "weapon", id: 34 } }],
    }
    expect(maximizedAttack(targeted).parts).toEqual([
      { element: "water", share: 100, kind: "spell", id: 12 },
      { element: "fire", share: 70, kind: "weapon", id: 34 },
    ])
    // An element alone keeps the spell or the weapon it names.
    const alone: ElementEntry = { kind: "element", element: "fire", measure: "mean", target: { kind: "weapon", id: 34 } }
    expect(maximizedAttack(alone)).toEqual({
      kind: "elements",
      id: 0,
      measure: "mean",
      mix: "floor",
      parts: [{ element: "fire", share: 100, kind: "weapon", id: 34 }],
    })
    const summed: ElementEntry = { ...entry, mix: "sum", share: 60 }
    expect(maximizedAttack(summed).parts?.[0]).toEqual({ element: "water", share: 60 })
  })
})

describe("the spells offered for an element", () => {
  it("keeps those with a line in it or in the best element", () => {
    const spell = { id: 1, name: "", breedId: 9, variantOf: 0, damage: true }
    expect(spellHitsIn({ ...spell, elements: ["earth"] }, "earth")).toBe(true)
    expect(spellHitsIn({ ...spell, elements: ["earth"] }, "water")).toBe(false)
    expect(spellHitsIn({ ...spell, elements: ["best"] }, "water")).toBe(true)
  })
})

describe("adding an element to an entry", () => {
  it("names the first element when the search was picking it", () => {
    const entry = addElement({ kind: "element", element: "auto", measure: "mean" })
    expect(entry.element).toBe("earth")
    expect(entry.others).toEqual([{ element: "fire", share: 70 }])
  })

  it("adds the first element not named yet, up to the four there are", () => {
    let entry: ElementEntry = { kind: "element", element: "fire", measure: "mean" }
    entry = addElement(entry)
    expect(elementsOf(entry)).toEqual(["fire", "earth"])
    entry = addElement(addElement(entry))
    expect(elementsOf(entry)).toEqual(["fire", "earth", "water", "air"])
    expect(addElement(entry)).toBe(entry)
  })
})
