import type { FightBoard, FightStep } from "../api/types"

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

/** The four corners of a cell, as an SVG points list. */
export function cellPoints(center: Point): string {
  const { x, y } = center
  return `${x},${y - HALF_H} ${x + HALF_W},${y} ${x},${y + HALF_H} ${x - HALF_W},${y}`
}

/** The drawing's frame: every cell, with room above for what stands on the back row. */
export function boardFrame(board: FightBoard, headroom: number): { x: number; y: number; width: number; height: number } {
  const left = -board.height * HALF_W
  const right = board.width * HALF_W
  const bottom = (board.width + board.height - 1) * HALF_H
  return { x: left, y: -HALF_H - headroom, width: right - left, height: bottom + HALF_H + headroom }
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
