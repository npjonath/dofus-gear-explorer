import { describe, expect, it } from "vitest"
import { boardFrame, cellCenter, cellPoints, neighbour, pathTo } from "./fightBoard"

const board = { width: 13, height: 13, walls: [] }

describe("the fight board's drawing", () => {
  it("puts each cell where a square seen at an angle falls", () => {
    expect(cellCenter(board, 0)).toEqual({ x: 0, y: 0 })
    // One cell along the row goes right and down, one along the column left and down.
    expect(cellCenter(board, 1)).toEqual({ x: 30, y: 15 })
    expect(cellCenter(board, 13)).toEqual({ x: -30, y: 15 })
    expect(cellPoints({ x: 0, y: 0 })).toBe("0,-15 30,0 0,15 -30,0")
  })

  it("frames every cell, with room above the back row", () => {
    const frame = boardFrame(board, 40)
    expect(frame).toEqual({ x: -390, y: -55, width: 780, height: 430 })
  })
})

describe("the way to a cell", () => {
  const steps = [
    { cell: 1, cost: 1, from: 0 },
    { cell: 2, cost: 2, from: 1 },
    { cell: 15, cost: 3, from: 2 },
  ]

  it("lists the cells walked, the last one included", () => {
    expect(pathTo(steps, 15)).toEqual([1, 2, 15])
  })

  it("is empty for a cell out of reach", () => {
    expect(pathTo(steps, 40)).toEqual([])
  })
})

describe("moving the cursor with the arrow keys", () => {
  it("steps along a row or a column", () => {
    expect(neighbour(board, 14, "ArrowRight")).toBe(15)
    expect(neighbour(board, 14, "ArrowUp")).toBe(1)
  })

  it("stays on the board", () => {
    expect(neighbour(board, 0, "ArrowLeft")).toBe(0)
    expect(neighbour(board, 168, "ArrowDown")).toBe(168)
  })

  it("ignores other keys", () => {
    expect(neighbour(board, 14, "a")).toBeNull()
  })
})
