import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, waitFor } from "@testing-library/react"
import { afterEach, expect, it, vi } from "vitest"
import { PlatformProvider } from "../../platform"
import { GameSprite, type GameSpriteProps } from "./GameActor"

const gpu = vi.hoisted(() => ({ create: vi.fn(), release: vi.fn(), frame: vi.fn(), changed: vi.fn() }))
vi.mock("./gameSpriteRenderer", () => ({
  gameSpriteRenderer: () => ({
    Look: { fromString: (value: string) => ({ value, injectColor: async () => {} }) },
    DofusSprite: {
      create: async (_look: unknown, canvas: HTMLCanvasElement) => {
        gpu.create()
        canvas.width = 140
        canvas.height = 240
        return {
          changeLook: gpu.changed,
          animations: new Map(),
          availableDirections: () => ({}),
          getAnimName: (_d: number, name: string) => [name, false],
          prepareAnimation: async () => 24,
          renderFrame: gpu.frame,
          data: { defaultFrameRate: 24 },
          openGl: {
            offset: { x: 20, y: 100 },
            assetStore: { clear() {} },
            unloadAllTextures() {},
            gl: { getExtension: () => ({ loseContext: gpu.release }) },
          },
        }
      },
    },
  }),
}))

afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

it("reuses a canvas through cast, walk and turn, and releases its textures when a new pack replaces it", async () => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }))
  const cache = new QueryClient()
  const initial: GameSpriteProps = {
    assetRoot: "https://localhost/fight-assets/actors/", pack: "one",
    look: "{1|60,2092||58}", bone: "1-static", animation: "AnimStatiqueCombat0", direction: 3,
  }
  function surface(props: Partial<GameSpriteProps>) {
    return (
      <QueryClientProvider client={cache}>
        <PlatformProvider platform={{ kind: "web", apiBaseUrl: "https://localhost", version: "test" }}>
          <svg><GameSprite {...initial} {...props} /></svg>
        </PlatformProvider>
      </QueryClientProvider>
    )
  }
  const view = render(surface({}))
  await waitFor(() => expect(gpu.frame).toHaveBeenCalledTimes(1))
  view.rerender(surface({ bone: "1-movement", animation: "AnimCourse", direction: 5 }))
  await waitFor(() => expect(gpu.frame).toHaveBeenCalledTimes(2))
  view.rerender(surface({ bone: "1-combat", animation: "AnimAttaque5", direction: 7 }))
  await waitFor(() => expect(gpu.frame).toHaveBeenCalledTimes(3))
  expect(gpu.create).toHaveBeenCalledTimes(1)
  expect(gpu.changed).toHaveBeenCalledTimes(2)
  expect(gpu.release).not.toHaveBeenCalled()
  view.rerender(surface({ pack: "two" }))
  await waitFor(() => expect(gpu.create).toHaveBeenCalledTimes(2))
  expect(gpu.release).toHaveBeenCalledTimes(1)
  view.unmount()
  await waitFor(() => expect(gpu.release).toHaveBeenCalledTimes(2))
})

it("bakes identical portraits once and releases their temporary WebGL context", async () => {
  const png = vi.spyOn(HTMLCanvasElement.prototype, "toDataURL").mockReturnValue("data:image/png;base64,AA==")
  const cache = new QueryClient()
  const props: GameSpriteProps = {
    assetRoot: "https://localhost/fight-assets/actors/", pack: "portraits",
    look: "{431}", bone: "431", animation: "AnimStatique", direction: 1, portrait: true,
  }
  try {
    const view = render(
      <QueryClientProvider client={cache}>
        <PlatformProvider platform={{ kind: "web", apiBaseUrl: "https://localhost", version: "test" }}>
          <svg><GameSprite {...props} /><GameSprite {...props} /></svg>
        </PlatformProvider>
      </QueryClientProvider>
    )
    await waitFor(() => expect(view.container.querySelectorAll("image")).toHaveLength(2))
    expect(gpu.create).toHaveBeenCalledTimes(1)
    expect(gpu.release).toHaveBeenCalledTimes(1)
    expect(view.container.querySelector("canvas")).toBeNull()
  } finally {
    png.mockRestore()
  }
})
