import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, expect, it, vi } from "vitest"
import type { SolverRequest, SolverResponse } from "../../api/types"
import { PlatformProvider } from "../../platform"
import { newBuild, useStore, type SolverDraft } from "../../store"
import { BuildPickerHost } from "../../ui/BuildPicker"
import { evaluation, item, meta } from "../compare/fixtures"
import { BuildPage } from "./BuildPage"

afterEach(() => vi.unstubAllGlobals())

it("restores the applied result's original search from its destination tab after reloading", async () => {
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))
  window.location.hash = "#/build"
  const hat = item(1, "Coiffe de départ", "hat")
  const ring = item(2, "Anneau du résultat", "ring")
  const source = { ...newBuild("Stuff de départ"), level: 150, breedId: 9, items: { hat: hat.id } }
  const target = newBuild("Mon stuff")
  const criteria: SolverDraft = {
    ...useStore.getInitialState().solver,
    constraints: [{ statId: 1, op: "gte", value: 11 }],
    maximize: [{ kind: "stat", statId: 10 }],
    required: [ring.id],
    excluded: [3],
    keepEquipped: true,
    fullScrolls: false,
    exoMp: true,
  }
  useStore.setState({
    lang: "fr",
    builds: [source, target],
    activeBuildId: source.id,
    openBuildIds: [source.id, target.id],
    stuffTab: "search",
    solver: criteria,
    solverSource: null,
    searchRevision: 0,
  })
  const requests: SolverRequest[] = []
  vi.stubGlobal("fetch", async (url: string, init?: RequestInit) => {
    const requestUrl = new URL(url)
    if (requestUrl.pathname === "/api/solver/search") {
      const request = JSON.parse(String(init?.body)) as SolverRequest
      requests.push(request)
      const result: SolverResponse = {
        id: `results-${requests.length}`,
        total: 1,
        feasible: 1,
        explored: 100,
        candidates: 4,
        elapsedMs: 100,
        unplaced: [],
        solutions: [
          {
            character: { ...request.character, base: { ...request.character.base, strength: 100 } },
            items: { hat: hat.id, ring1: ring.id },
            score: 100,
            feasible: true,
            unmet: [],
            unmetDamage: [],
            damage: [],
            evaluation: evaluation({ items: { hat, ring1: ring }, stats: { 1: 11, 10: 500 } }),
          },
        ],
      }
      return Response.json(result)
    }
    if (requestUrl.pathname === "/api/data/status")
      return Response.json({ ready: true, state: "idle", installedVersion: "test", importedAt: "now" })
    if (requestUrl.pathname === "/api/builds/evaluate") return Response.json(evaluation())
    if (requestUrl.pathname === "/api/items") return Response.json({ items: [], total: 0 })
    if (requestUrl.pathname === "/api/items/2") return Response.json({ ...ring, setDetail: null })
    return new Response(null, { status: 404 })
  })
  const openApp = () =>
    render(
      <PlatformProvider platform={{ kind: "web", apiBaseUrl: "http://test.local", version: "1" }}>
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <BuildPage meta={meta} />
          <BuildPickerHost meta={meta} />
        </QueryClientProvider>
      </PlatformProvider>
    )

  const app = openApp()
  fireEvent.click(screen.getByRole("button", { name: "Rechercher" }))
  const apply = await screen.findByRole("button", { name: "Appliquer à…" })
  // Editing the live form after the result arrives must not change its saved criteria.
  act(() => useStore.getState().updateSolver({ constraints: [{ statId: 23, op: "gte", value: 6 }], keepEquipped: false }))
  fireEvent.click(apply)
  const dialog = await screen.findByRole("dialog", { name: "Appliquer à quel stuff ?" })
  fireEvent.click(within(dialog).getByRole("radio", { name: /Mon stuff/ }))
  fireEvent.click(within(dialog).getByRole("button", { name: "Appliquer" }))
  expect(await screen.findByRole("button", { name: "Rouvrir la recherche" })).toBeDefined()
  expect(useStore.getState().activeBuildId).toBe(target.id)

  app.unmount()
  const storageKey = useStore.persist.getOptions().name!
  const saved = localStorage.getItem(storageKey)!
  useStore.setState(useStore.getInitialState(), true)
  localStorage.setItem(storageKey, saved)
  await useStore.persist.rehydrate()
  openApp()
  fireEvent.click(screen.getByRole("button", { name: "Rouvrir la recherche" }))
  await waitFor(() => expect(document.activeElement?.id).toBe("solver-title"))
  expect(screen.getByRole("tab", { name: "Recherche de stuffs" }).getAttribute("aria-selected")).toBe("true")
  expect((screen.getByRole("textbox", { name: "Niveau de la recherche" }) as HTMLInputElement).value).toBe("150")
  expect(useStore.getState().solver).toEqual(criteria)
  expect(requests).toHaveLength(1)

  fireEvent.click(screen.getByRole("button", { name: "Rechercher" }))
  await waitFor(() => expect(requests).toHaveLength(2))
  expect(requests[1]).toEqual(requests[0])
  expect(requests[1]?.locked).toEqual({ hat: hat.id })
  expect(useStore.getState().builds.find((build) => build.id === target.id)).toMatchObject({
    level: 200,
    breedId: null,
    items: { hat: hat.id, ring1: ring.id },
  })
})
