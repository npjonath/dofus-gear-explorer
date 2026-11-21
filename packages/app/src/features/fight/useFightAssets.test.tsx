import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, cleanup, renderHook, waitFor } from "@testing-library/react"
import type { ReactNode } from "react"
import { afterEach, expect, it, vi } from "vitest"
import { PlatformProvider, type Platform } from "../../platform"
import { useFightAssets } from "./useFightAssets"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

it("rediscovers a completed pack and picks up an ongoing import after a renderer reload", async () => {
  const firstPack = { maps: [192419850], pack: "pack-first" }
  const updatedPack = { maps: [99222029], pack: "pack-updated" }
  let available = firstPack
  let stage = ""
  const fightAssets = { stage: vi.fn(async () => stage), import: vi.fn(async () => true) }
  const platform: Platform = { kind: "desktop", apiBaseUrl: "http://app.test", version: "test", fightAssets }
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(available))))

  function openFight() {
    const cache = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={cache}>
        <PlatformProvider platform={platform}>{children}</PlatformProvider>
      </QueryClientProvider>
    )
    return { cache, ...renderHook(useFightAssets, { wrapper }) }
  }

  const first = openFight()
  await waitFor(() => expect(first.result.current.assets.data).toEqual(firstPack))
  first.unmount()

  stage = "actors"
  const reloaded = openFight()
  await waitFor(() => {
    expect(reloaded.result.current.stage).toBe("actors")
    expect(reloaded.result.current.assets.data).toEqual(firstPack)
  })

  available = updatedPack
  stage = ""
  await act(() => reloaded.cache.refetchQueries({ queryKey: ["fight-import-stage"] }))
  await waitFor(() => expect(reloaded.result.current.assets.data).toEqual(updatedPack))
  reloaded.unmount()

  const restarted = openFight()
  await waitFor(() => expect(restarted.result.current.assets.data).toEqual(updatedPack))
  expect(fightAssets.import).not.toHaveBeenCalled()
})
