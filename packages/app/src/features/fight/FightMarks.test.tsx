import { render } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import type { FightBoard, FightMark } from "../../api/types"
import { FightMarks } from "./FightMarks"

const board: FightBoard = { width: 4, height: 4, walls: [] }
const glyph: FightMark = {
  id: 1, action: 401, caster: 1, from: 101, spell: 101, grade: 1,
  index: 0, cell: 5, cells: [5, 6], color: 0xca1330, turns: 2,
}
const names = { "101": "Fire", "102": "Earth" }

describe("ground marks", () => {
  it("draws one perimeter for a glyph and its aura and cycles overlapping spells", () => {
    const marks = [
      glyph,
      { ...glyph, id: 2, action: 1091 },
      { ...glyph, id: 3, from: 102, action: 400, cells: [5], turns: -1 },
    ]
    const frame = (cycle: number, cursor: number | null) =>
      <svg><FightMarks board={board} marks={marks} cursor={cursor} names={names} cycle={cycle} /></svg>
    const { container, rerender } = render(frame(0, 5))
    expect(container.querySelectorAll("title")).toHaveLength(2)
    expect(container.querySelector("text")?.textContent).toContain("1/2 · Fire")
    rerender(frame(1, 5))
    expect(container.querySelector("text")?.textContent).toContain("2/2 · Earth")
    rerender(frame(2, 5))
    expect(container.querySelector("text")?.textContent).toContain("1/2 · Fire")
    rerender(frame(0, null))
    expect(container.querySelector("text")).toBeNull()
  })
})
