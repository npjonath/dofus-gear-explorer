import type { FightBoard, FightMark } from "../../api/types"
import { useI18n } from "../../i18n"
import { cellCenter, HALF_H, HALF_W } from "../../lib/fightBoard"

interface Props {
  board: FightBoard
  marks?: FightMark[]
  cursor: number | null
  names: Record<string, string>
  highlight?: boolean
  cycle?: number
}

// Tile fills share a path. Only the perimeter is stroked, so adjacent cells
// form a single area on the ground, even when there is a pillar or a hole.
function outline(board: FightBoard, cells: number[]) {
  const cover = new Set(cells)
  let fill = ""
  let border = ""
  for (const cell of cells) {
    const { x, y } = cellCenter(board, cell)
    const top = `${x},${y - HALF_H}`
    const right = `${x + HALF_W},${y}`
    const bottom = `${x},${y + HALF_H}`
    const left = `${x - HALF_W},${y}`
    fill += `M${top}L${right}L${bottom}L${left}Z`
    if (!cover.has(cell - board.width)) border += `M${top}L${right}`
    if (cell % board.width === board.width - 1 || !cover.has(cell + 1)) border += `M${right}L${bottom}`
    if (!cover.has(cell + board.width)) border += `M${bottom}L${left}`
    if (cell % board.width === 0 || !cover.has(cell - 1)) border += `M${left}L${top}`
  }
  return { fill, border }
}

export function FightMarks({ board, marks, cursor, names, highlight = false, cycle = 0 }: Props) {
  const { t } = useI18n()
  // Elemental glyphs have a start-of-turn hit and an aura at the same
  // position. Represent the single visible mark only once.
  const unique = new Map<string, FightMark>()
  for (const mark of marks ?? []) {
    unique.set([mark.from, mark.cell, mark.color, mark.cells.join(",")].join("/"), mark)
  }
  const visible = [...unique.values()]
  const pointed = cursor === null ? [] : visible.filter((mark) => mark.cells.includes(cursor))
  const index = pointed.length ? ((cycle % pointed.length) + pointed.length) % pointed.length : 0
  const selected = pointed[index]
  let label = ""
  const shapes = visible.map((mark) => {
    const { fill, border } = outline(board, mark.cells)
    const color = `#${mark.color.toString(16).padStart(6, "0")}`
    const hovered = selected?.id === mark.id
    const turns = mark.turns < 0 ? t("fight.buff.forever") : t("fight.buff.turns", { count: mark.turns })
    const title = [names[mark.action === 2022 ? mark.spell : mark.from], turns].filter(Boolean).join(" · ")
    if (hovered) label = pointed.length > 1 ? `${index + 1}/${pointed.length} · ${title}` : title
    return (
      <g key={mark.id} fill={color} stroke={color}>
        <title>{title}</title>
        <path d={fill} opacity={highlight || hovered ? 0.67 : 0.43} stroke="none" />
        <path d={border} fill="none" strokeWidth={highlight || hovered ? 2 : 1} />
      </g>
    )
  })
  const at = cursor === null ? null : cellCenter(board, cursor)
  return (
    <g pointerEvents="none">
      {shapes}
      {at && selected && (
        <text x={at.x} y={at.y + HALF_H + 12} textAnchor="middle" fontSize={10}
          fontWeight="bold" fill="white" stroke="#17140f" strokeWidth={3} paintOrder="stroke">
          {label}
        </text>
      )}
    </g>
  )
}
