import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, configure, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import type { BuildInput } from "../../api/types"
import { PlatformProvider, type Platform } from "../../platform"
import { useStore } from "../../store"
import { CompareDialog, type CompareOption } from "./CompareDialog"
import { attack, damage, evaluation, item, line, meta } from "./fixtures"

// The API of a tiny game: each item gives a few stats, and spells hit for the Strength of the stuff.
const items = {
  1: { item: item(1, "Coiffe", "hat"), stats: { "0": 500, "10": 300 } },
  2: { item: item(2, "Casque", "hat"), stats: { "0": 300, "13": 300, "1": 1 } },
  3: { item: item(3, "Gelano", "ring"), stats: { "1": 1, "18": 10 } },
} as const

function evaluate(input: BuildInput) {
  const stats: Record<string, number> = { "1": 10 }
  const worn = Object.entries(input.items).map(([position, id]) => [position, items[id as keyof typeof items]] as const)
  for (const [, entry] of worn) {
    for (const [stat, value] of Object.entries(entry.stats)) stats[stat] = (stats[stat] ?? 0) + value
  }
  return evaluation({
    stats,
    items: Object.fromEntries(worn.map(([position, entry]) => [position, entry.item])),
    forge: Object.fromEntries(
      worn.map(([position]) => [position, input.mods[position]?.exos?.map((exo) => line(exo.statId, exo.value, "exo")) ?? []])
    ),
  })
}

const calls: string[] = []

function fakeFetch(url: string | URL | Request, init?: RequestInit): Promise<Response> {
  const path = new URL(String(url)).pathname
  calls.push(path)
  const body = init?.body ? (JSON.parse(String(init.body)) as BuildInput) : null
  const json = (data: unknown) => Promise.resolve(new Response(JSON.stringify(data), { status: 200 }))
  if (path === "/api/data/status") return json({ state: "idle", ready: true, installedVersion: "1", importedAt: "now" })
  if (path === "/api/builds/evaluate") return json(evaluate(body!))
  if (path === "/api/builds/damage") {
    const strength = evaluate(body!).stats["10"] ?? 0
    return json(damage([attack(7, "Flèche", 400 + strength / 5)]))
  }
  return Promise.resolve(new Response("{}", { status: 404 }))
}

const base = { vitality: 0, wisdom: 0, strength: 0, intelligence: 0, chance: 0, agility: 0 }

function option(key: string, label: string, worn: Record<string, number>, patch: Partial<BuildInput> = {}): CompareOption {
  return {
    key,
    label,
    group: "saved",
    breedId: 9,
    input: { level: 200, base, scrolls: base, items: worn, mods: {}, ...patch },
  }
}

const earth = option("earth", "Terre", { hat: 1, ring1: 3 })
const water = option("water", "Eau", { hat: 2, ring1: 3 })

const platform: Platform = { kind: "web", apiBaseUrl: "http://api.test", version: "0" }

function open(options: CompareOption[], initial: { from: string; to?: string }) {
  const onOpenChange = vi.fn()
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <PlatformProvider platform={platform}>
      <QueryClientProvider client={client}>
        <CompareDialog meta={meta} open onOpenChange={onOpenChange} options={options} initial={initial} />
      </QueryClientProvider>
    </PlatformProvider>
  )
  return { onOpenChange, user: userEvent.setup() }
}

// The dialog opens as a popover, which jsdom never shows as open: to its
// style sheet the dialog stays hidden, so the queries must look there too.
configure({ defaultHidden: true })

const row = (name: string) => screen.getByRole("row", { name: new RegExp(`^${name}`) })

beforeEach(() => {
  // jsdom has no Web Animations; the tabs ask for them when the selection moves.
  Element.prototype.getAnimations ??= () => []
  calls.length = 0
  useStore.setState({ lang: "fr" })
  vi.stubGlobal("fetch", vi.fn(fakeFetch))
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("CompareDialog", () => {
  it("is a named dialog that opens on the two stuffs it was asked for", async () => {
    open([earth, water], { from: "earth", to: "water" })
    const dialog = await screen.findByRole("dialog")
    const named = (attribute: string) => document.getElementById(dialog.getAttribute(attribute) ?? "")?.textContent
    expect(named("aria-labelledby")).toBe("Comparer deux stuffs")
    expect(named("aria-describedby")).toContain("Ce que l'on gagne et ce que l'on perd")
    expect(within(dialog).getByRole("combobox", { name: "Stuff de départ" })).toHaveProperty("value", "Terre")
    expect(within(dialog).getByRole("combobox", { name: "Stuff d'arrivée" })).toHaveProperty("value", "Eau")
    expect(within(dialog).getByRole("button", { name: "Fermer" })).toBeDefined()
  })

  it("splits the stats into gains and losses, with the vitals first", async () => {
    open([earth, water], { from: "earth", to: "water" })
    const vitals = await screen.findByRole("list", { name: "Points de vie, PA, PM et portée" })
    const [hp, ap] = within(vitals).getAllByRole("listitem")
    expect(hp?.textContent).toContain("−200")
    expect(ap?.textContent).toContain("+1")
    expect(ap?.textContent).toContain("11")
    expect(ap?.textContent).toContain("12")

    // The vitals lead each list, so the counts hold every change.
    expect(screen.getByRole("heading", { name: "2 gains" })).toBeDefined()
    expect(screen.getByRole("heading", { name: "2 pertes" })).toBeDefined()
    const names = screen.getAllByRole("rowheader").map((cell) => cell.textContent)
    expect(names).toEqual(["PA", "Chance", "PV", "Force"])
    expect(row("Chance").textContent).toContain("+300")
    expect(row("Force").textContent).toContain("−300")
    // Critical does not move: it waits out of the way.
    expect(screen.getByText("1 caractéristique inchangée")).toBeDefined()
    expect(screen.queryByRole("row", { name: /^Critique/ })).toBeNull()
  })

  it("asks the API for each stuff instead of computing anything", async () => {
    open([earth, water], { from: "earth", to: "water" })
    await screen.findByRole("heading", { name: "2 gains" })
    expect(calls.filter((path) => path === "/api/builds/evaluate")).toHaveLength(2)
  })

  it("turns the gains into losses when the two stuffs are swapped", async () => {
    const { user } = open([earth, water], { from: "earth", to: "water" })
    await screen.findByRole("heading", { name: "2 gains" })
    await user.click(screen.getByRole("button", { name: "Inverser le départ et l'arrivée" }))
    expect(screen.getByRole("combobox", { name: "Stuff de départ" })).toHaveProperty("value", "Eau")
    expect(row("Force").textContent).toContain("+300")
    expect(row("Chance").textContent).toContain("−300")
  })

  it("shows the items that differ, place by place", async () => {
    const { user } = open([earth, water], { from: "earth", to: "water" })
    await user.click(await screen.findByRole("tab", { name: /^Objets/ }))
    const places = within(screen.getByRole("tabpanel")).getAllByRole("listitem")
    expect(places).toHaveLength(1)
    expect(places[0]?.textContent).toMatch(/Chapeau.*Coiffe.*remplacé par.*Casque/)
    expect(screen.getByText("1 objet identique dans les deux stuffs.")).toBeDefined()
  })

  it("shows the forgemagic of an item worn on both sides", async () => {
    const forged = option("forged", "Terre exo", { hat: 1, ring1: 3 }, { mods: { ring1: { exos: [{ statId: 23, value: 1 }] } } })
    const { user } = open([earth, forged], { from: "earth", to: "forged" })
    await user.click(await screen.findByRole("tab", { name: /^Objets/ }))
    const panel = within(screen.getByRole("tabpanel"))
    expect(panel.getByText("Même objet, forgemagie différente")).toBeDefined()
    expect(panel.getByRole("row", { name: /^PM/ }).textContent).toMatch(/Exo.*0.*1.*\+1/)
  })

  it("compares what the spells hit for", async () => {
    const { user } = open([earth, water], { from: "earth", to: "water" })
    await user.click(await screen.findByRole("tab", { name: "Dommages" }))
    const spell = await screen.findByRole("row", { name: /^Flèche/ })
    expect(spell.textContent).toMatch(/460.*400.*−60.*−13 %/)
    expect(screen.getByRole("columnheader", { name: /Départ.*Terre/ })).toBeDefined()
  })

  it("says so when the two stuffs are the same", async () => {
    open([earth, option("copy", "Copie", { hat: 1, ring2: 3 })], { from: "earth", to: "copy" })
    expect((await screen.findByRole("status")).textContent).toContain("identiques")
  })

  it("explains that a second stuff is needed when there is only one", async () => {
    open([earth], { from: "earth" })
    expect(await screen.findByText(/Il faut un deuxième stuff/)).toBeDefined()
    expect(screen.getByRole("button", { name: "Inverser le départ et l'arrivée" })).toHaveProperty("disabled", true)
    expect(calls).not.toContain("/api/builds/damage")
  })

  it("closes from its close button", async () => {
    const { user, onOpenChange } = open([earth, water], { from: "earth", to: "water" })
    await user.click(await screen.findByRole("button", { name: "Fermer" }))
    expect(onOpenChange).toHaveBeenCalled()
    expect(onOpenChange.mock.calls[0]?.[0]).toBe(false)
  })
})
