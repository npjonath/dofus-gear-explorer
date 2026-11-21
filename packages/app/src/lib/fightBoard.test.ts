import { describe, expect, it } from "vitest"
import { blockFaces, boardFrame, cellCenter, cellDistance, cellPoints, gridPoint, heading, neighbour, pathTo, slabFaces } from "./fightBoard"

const board = { width: 13, height: 13, walls: [] }

describe("the fight board's drawing", () => {
  it("puts each cell where a square seen at an angle falls", () => {
    expect(cellCenter(board, 0)).toEqual({ x: 0, y: 0 })
    // One cell along the row goes right and down, one along the column left and down.
    expect(cellCenter(board, 1)).toEqual({ x: 30, y: 15 })
    expect(cellCenter(board, 13)).toEqual({ x: -30, y: 15 })
    expect(cellPoints({ x: 0, y: 0 })).toBe("0,-15 30,0 0,15 -30,0")
  })

  it("draws a tile smaller than its cell when asked", () => {
    expect(cellPoints({ x: 0, y: 0 }, 1)).toBe("0,-14 28,0 0,14 -28,0")
  })

  it("places an entity between two cells while it travels", () => {
    expect(gridPoint(1, 0)).toEqual(cellCenter(board, 1))
    expect(gridPoint(0.5, 0)).toEqual({ x: 15, y: 7.5 })
  })

  it("frames every cell and the slab under them, with room above the back row", () => {
    const frame = boardFrame(board, 40)
    expect(frame).toEqual({ x: -390, y: -55, width: 780, height: 440 })
  })

  it("gives the arena the two front faces of a slab", () => {
    const slab = slabFaces(board)
    // The rim goes round the four outer corners of the cells.
    expect(slab.top).toBe("0,-15 390,180 0,375 -390,180")
    // Each face hangs from a front edge of the rim, ten units deep.
    expect(slab.left).toBe("-390,180 0,375 0,385 -390,190")
    expect(slab.right).toBe("0,375 390,180 390,190 0,385")
  })

  it("follows a board that is not square", () => {
    const slab = slabFaces({ width: 4, height: 2, walls: [] })
    expect(slab.top).toBe("0,-15 120,45 60,75 -60,15")
  })

  it("raises a pillar as a block with a left and a right face", () => {
    const faces = blockFaces({ x: 0, y: 0 }, 20)
    expect(faces.left).toBe("-30,-20 0,-5 0,15 -30,0")
    expect(faces.right).toBe("0,-5 30,-20 30,0 0,15")
  })
})

describe("which way a figure turns", () => {
  it("looks right along a row and left along a column", () => {
    expect(heading(board, 14, 15)).toBe(1)
    expect(heading(board, 14, 27)).toBe(-1)
    expect(heading(board, 15, 14)).toBe(-1)
    expect(heading(board, 27, 14)).toBe(1)
  })

  it("keeps its side when the other cell is straight above or below on the drawing", () => {
    expect(heading(board, 14, 28)).toBe(0)
    expect(heading(board, 14, 14)).toBe(0)
  })

  it("counts cells along rows and columns", () => {
    expect(cellDistance(board, 14, 14)).toBe(0)
    expect(cellDistance(board, 14, 15)).toBe(1)
    expect(cellDistance(board, 0, 28)).toBe(4)
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
