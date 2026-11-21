import { useQuery, useQueryClient } from "@tanstack/react-query"
import * as React from "react"
import { usePlatform } from "../../platform"

export function useFightAssets() {
  const { api, fightAssets } = usePlatform()
  const cache = useQueryClient()
  const progress = useQuery({
    queryKey: ["fight-import-stage", api.baseUrl],
    queryFn: () => fightAssets?.stage() ?? "",
    enabled: Boolean(fightAssets),
    retry: false,
    refetchInterval: (query) => (query.state.data ? 700 : 1500),
  })
  const assets = useQuery({
    queryKey: ["fight-assets", api.baseUrl],
    queryFn: () => api.fightAssets(),
    // A new renderer first reconnects to any import still owned by Electron.
    enabled: !fightAssets || !progress.isPending,
  })
  const refresh = React.useCallback(async () => {
    await cache.invalidateQueries({ queryKey: ["fight-assets", api.baseUrl] })
    await cache.invalidateQueries({ queryKey: ["fight-actors", api.imageUrl("/fight-assets/actors/")] })
  }, [cache, api])
  const previousStage = React.useRef("")
  const stage = progress.data ?? ""
  React.useEffect(() => {
    if (previousStage.current && !stage) void refresh()
    previousStage.current = stage
  }, [stage, refresh])

  return { assets, stage, refresh }
}
