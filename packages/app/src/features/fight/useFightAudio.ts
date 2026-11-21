import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import type { FightEntity } from "../../api/types"
import { defaultLevels, FightSoundPlayer, type InterfaceSound, type SoundCatalog, type SoundChannel, type VolumeKey } from "../../lib/fightAudio"
import type { Run } from "./useFightPlayback"

const preferenceKey = "dde-fight-sound-v1"

function saved() {
  const levels = { ...defaultLevels }
  let poutchHit = false
  try {
    const item = JSON.parse(localStorage.getItem(preferenceKey) ?? "{}")
    for (const k of Object.keys(levels) as VolumeKey[]) {
      const v = item?.levels?.[k]
      if (Number.isFinite(v)) levels[k] = Math.round(Math.max(0, Math.min(100, v)))
    }
    poutchHit = item?.poutchHit === true
  } catch { /* Storage can be unavailable in a private browser. */ }
  return { levels, poutchHit }
}

export function useFightAudio(assetRoot?: string, packId?: string) {
  const [{ levels, poutchHit }, setPreferences] = React.useState(saved)
  const catalog = useQuery<SoundCatalog>({
    queryKey: ["fight-audio", assetRoot, packId],
    enabled: Boolean(assetRoot && packId),
    queryFn: async ({ signal }) => {
      const res = await fetch(`${assetRoot}/audio/index.json`, { signal })
      if (!res.ok) throw new Error("The local combat sounds have not been imported.")
      const data = await res.json() as SoundCatalog
      if (!Array.isArray(data.categories) || !data.events || !data.maps || !data.animations) throw new Error("Invalid combat sound catalog.")
      return data
    },
    staleTime: Infinity,
    retry: false,
  }).data
  const player = React.useMemo(() => catalog && assetRoot ? new FightSoundPlayer(assetRoot, catalog) : undefined, [assetRoot, catalog])
  React.useEffect(() => () => player?.dispose(), [player])
  React.useEffect(() => {
    player?.volumes(levels)
    try { localStorage.setItem(preferenceKey, JSON.stringify({ levels, poutchHit })) } catch { /* Session-only settings. */ }
  }, [levels, poutchHit, player])
  const setLevel = (key: VolumeKey, value: number) => {
    if (!Number.isFinite(value)) return
    setPreferences((current) => ({ ...current, levels: { ...current.levels, [key]: Math.round(Math.max(0, Math.min(100, value))) } }))
  }
  const setPoutchHit = (value: boolean) => setPreferences((current) => ({ ...current, poutchHit: value }))
  return {
    levels, setLevel, poutchHit, setPoutchHit, available: catalog?.categories ?? [] as SoundChannel[],
    start: React.useCallback((mapId: number) => player?.start(mapId), [player]),
    stop: React.useCallback(() => player?.stop(), [player]),
    unlock: React.useCallback(() => player?.unlock(), [player]),
    interfaceSound: React.useCallback((key: InterfaceSound) => player?.interfaceSound(key), [player]),
    play: React.useCallback((run: Run, entities: FightEntity[], breed: number, weapon: number) =>
      player?.play(run.id, run.timeline, run.startedAt, entities, breed, weapon, poutchHit), [player, poutchHit]),
  }
}

export type FightAudio = ReturnType<typeof useFightAudio>
export type { SoundChannel, SoundLevels, VolumeKey } from "../../lib/fightAudio"
