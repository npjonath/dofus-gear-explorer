import { describe, expect, it } from "vitest"
import type { ElementEntry } from "../api/types"
import { addElement, elementsOf, maximizedAttack } from "./maximize"

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
    const summed: ElementEntry = { ...entry, mix: "sum", share: 60 }
    expect(maximizedAttack(summed).parts?.[0]).toEqual({ element: "water", share: 60 })
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
