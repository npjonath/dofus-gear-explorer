import { useQuery } from "@tanstack/react-query"
import type { FightBoard, FightMark } from "../../api/types"
import { cellCenter } from "../../lib/fightBoard"
import type { Scene } from "../../lib/fightTimeline"
import { groundGraphics, type SpellVisualCatalog } from "../../lib/gameSpellEffects"
import { GameSprite } from "./GameActor"

export function useGameSpellCatalog(assetRoot?: string, packId?: string) {
  return useQuery<SpellVisualCatalog>({
    queryKey: ["fight-spell-visuals", assetRoot, packId],
    queryFn: async ({ signal }) => {
      const response = await fetch(`${assetRoot}/effects/spells.json`, { signal })
      if (!response.ok) throw new Error("The local spell animation catalog is unavailable.")
      const catalog = await response.json() as Partial<SpellVisualCatalog> | null
      if (!catalog?.spells || !catalog.clips || !catalog.casts) throw new Error("Unsupported spell animation catalog.")
      return { spells: catalog.spells, clips: catalog.clips, casts: catalog.casts }
    },
    enabled: Boolean(assetRoot),
    staleTime: Infinity,
    retry: false,
  }).data
}

interface Props {
  assetRoot: string
  board: FightBoard
  graphics: Scene["graphics"]
  layer: "ground" | "front"
  scale?: number
}

export function GameSpellEffects({ assetRoot, board, graphics, layer, scale = 0.58 }: Props) {
  return graphics.map(({ cue, progress, index }) => {
    const behind = cue.kind === "mark" || cue.strata === "background"
    if (behind !== (layer === "ground")) return null
    const from = cellCenter(board, cue.from)
    const to = cellCenter(board, cue.to)
    const moving = cue.kind === "projectile" || cue.kind === "trail"
    const fraction = moving ? progress : cue.kind === "caster" ? 0 : 1
    const x = from.x + (to.x - from.x) * fraction
    const arc = moving ? Math.sin(Math.PI * fraction) * (cue.curvature ?? 0) : 0
    const y = from.y + (to.y - from.y) * fraction - ((cue.offsetY ?? 0) + arc) * scale
    return (
      <g key={index} aria-hidden="true" pointerEvents="none" transform={`translate(${x} ${y}) scale(${cue.scaleX ?? 1} ${cue.scaleY ?? 1})`}>
        <GameSprite
          assetRoot={`${assetRoot}/actors/`}
          look={`{${cue.bone}}`}
          bone={String(cue.bone)}
          animation={cue.animation}
          direction={cue.direction}
          phase={progress}
          scale={scale}
          effectColors
        />
      </g>
    )
  })
}

interface GroundMarkProps {
  assetRoot: string
  board: FightBoard
  marks: FightMark[]
  catalog?: SpellVisualCatalog
  scale?: number
  animating: Set<string>
}

export function GameGroundMarks({ assetRoot, board, marks, catalog, scale = 0.58, animating }: GroundMarkProps) {
  const drawn = new Set(animating)
  return marks.flatMap((mark) => {
    const { x, y } = cellCenter(board, mark.cell)
    return groundGraphics(catalog, mark).flatMap(({ gfx, idle }) => {
      const key = [mark.cell, gfx.bone].join("-")
      if (drawn.has(key)) return []
      drawn.add(key)
      return [
        <g key={key} aria-hidden="true" pointerEvents="none" transform={`translate(${x} ${y}) scale(${gfx.scaleX ?? 1} ${gfx.scaleY ?? 1})`}>
          <GameSprite
            assetRoot={`${assetRoot}/actors/`}
            look={`{${gfx.bone}}`}
            bone={String(gfx.bone)}
            animation={idle}
            direction={0}
            scale={scale}
            effectColors
          />
        </g>,
      ]
    })
  })
}
