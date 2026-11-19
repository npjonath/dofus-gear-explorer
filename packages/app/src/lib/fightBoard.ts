import type { FightBoard, FightEntity, FightStep } from "../api/types"

export function entityAt(entities: FightEntity[], cell: number | null) {
  return entities.find((entity) => entity.carriedBy && entity.cell === cell) ?? entities.find((entity) => entity.cell === cell)
}

/** Half the width and half the height of a cell as drawn: a square seen from above, at an angle. */
export const HALF_W = 30
export const HALF_H = 15

export interface Point {
  x: number
  y: number
}

export function cellXY(board: FightBoard, cell: number): Point {
  return { x: cell % board.width, y: Math.floor(cell / board.width) }
}

/** The centre of a cell on the drawing. */
export function cellCenter(board: FightBoard, cell: number): Point {
  const { x, y } = cellXY(board, cell)
  return { x: (x - y) * HALF_W, y: (x + y) * HALF_H }
}

/** Where a point of the grid falls on the drawing; an entity between two cells has a fractional place. */
export function gridPoint(x: number, y: number): Point {
  return { x: (x - y) * HALF_W, y: (x + y) * HALF_H }
}

/** The four corners of a cell, as an SVG points list; an inset draws a tile smaller than its cell. */
export function cellPoints(center: Point, inset = 0): string {
  const { x, y } = center
  const w = HALF_W - inset * 2
  const h = HALF_H - inset
  return `${x},${y - h} ${x + w},${y} ${x},${y + h} ${x - w},${y}`
}

/** The thickness of the slab the arena is cut from, and the height of a pillar standing on it. */
export const SLAB = 10
export const PILLAR = 22

/** The two faces of a block that the eye sees: the left one and the right one, under a top raised by `height`. */
export function blockFaces(center: Point, height: number): { left: string; right: string } {
  const { x, y } = center
  return {
    left: `${x - HALF_W},${y - height} ${x},${y + HALF_H - height} ${x},${y + HALF_H} ${x - HALF_W},${y}`,
    right: `${x},${y + HALF_H - height} ${x + HALF_W},${y - height} ${x + HALF_W},${y} ${x},${y + HALF_H}`,
  }
}

/** The arena as one block: its top outline and the two front faces of its slab. */
export function slabFaces(board: FightBoard): { top: string; left: string; right: string } {
  const right = { x: board.width * HALF_W, y: (board.width - 1) * HALF_H }
  const bottom = { x: (board.width - board.height) * HALF_W, y: (board.width + board.height - 1) * HALF_H }
  const left = { x: -board.height * HALF_W, y: (board.height - 1) * HALF_H }
  const at = (point: Point, drop = 0) => `${point.x},${point.y + drop}`
  return {
    top: `0,${-HALF_H} ${at(right)} ${at(bottom)} ${at(left)}`,
    left: `${at(left)} ${at(bottom)} ${at(bottom, SLAB)} ${at(left, SLAB)}`,
    right: `${at(bottom)} ${at(right)} ${at(right, SLAB)} ${at(bottom, SLAB)}`,
  }
}

/** The drawing's frame: every cell and the slab under them, with room above for what stands on the back row. */
export function boardFrame(board: FightBoard, headroom: number): { x: number; y: number; width: number; height: number } {
  const left = -board.height * HALF_W
  const right = board.width * HALF_W
  const bottom = (board.width + board.height - 1) * HALF_H
  return { x: left, y: -HALF_H - headroom, width: right - left, height: bottom + HALF_H + headroom + SLAB }
}

/** Which way an entity looks when it turns from one cell to another: 1 to the right of the drawing, -1 to the left, 0 straight up or down. */
export function heading(board: FightBoard, from: number, to: number): 1 | -1 | 0 {
  const a = cellXY(board, from)
  const b = cellXY(board, to)
  return Math.sign(b.x - b.y - (a.x - a.y)) as 1 | -1 | 0
}

/** The cells between two cells, counted along rows and columns. */
export function cellDistance(board: FightBoard, from: number, to: number): number {
  const a = cellXY(board, from)
  const b = cellXY(board, to)
  return Math.abs(a.x - b.x) + Math.abs(a.y - b.y)
}

/** The cells walked to reach a cell, from the first step to the cell itself; empty when it is out of reach. */
export function pathTo(steps: FightStep[], cell: number): number[] {
  const byCell = new Map(steps.map((step) => [step.cell, step]))
  const path: number[] = []
  for (let step = byCell.get(cell); step; step = byCell.get(step.from)) path.unshift(step.cell)
  return path
}

const moves: Record<string, [number, number]> = {
  ArrowRight: [1, 0],
  ArrowLeft: [-1, 0],
  ArrowDown: [0, 1],
  ArrowUp: [0, -1],
}

/** The cell an arrow key leads to from another one, staying on the board; null for any other key. */
export function neighbour(board: FightBoard, cell: number, key: string): number | null {
  const move = moves[key]
  if (!move) return null
  const { x, y } = cellXY(board, cell)
  const nx = Math.max(0, Math.min(board.width - 1, x + move[0]))
  const ny = Math.max(0, Math.min(board.height - 1, y + move[1]))
  return ny * board.width + nx
}
