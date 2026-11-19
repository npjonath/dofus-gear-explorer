import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, beforeEach, expect, it, vi } from "vitest"
import type { BuildInput, FightAction, FightState, FightView } from "../../api/types"
import { PlatformProvider } from "../../platform"
import { newBuild, toBuildInput, useStore } from "../../store"
import { evaluation, item, meta } from "../compare/fixtures"
import { FightPanel } from "./FightPanel"

const hat = item(17, "Coiffe d’essai", "hat")
hat.effects = [{ kind: "stat", statId: 11, element: null, min: 0, max: 100, text: "100 Vitalité" }]
const replacement = item(23, "Coiffe légère", "hat")
const sword = item(24, "Épée neutre", "weapon")
sword.weapon = { apCost: 4, rangeMin: 1, rangeMax: 1, critChance: 10, critBonus: 5, maxCastPerTurn: 1 }
sword.effects = [{ kind: "weapon", statId: null, element: 0, min: 12, max: 18, text: "12 à 18 dommages Neutre" }]
const callInputs: { gear: BuildInput; action?: string }[] = []
type RequestBody = BuildInput & { state?: FightState; action?: FightAction }

function testEvaluation(gear: BuildInput) {
  const worn = gear.items.hat === hat.id ? hat : gear.items.hat === replacement.id ? replacement : undefined
  const hp = 900 + (worn === hat ? (gear.mods.hat?.lines?.["11"] ?? 100) : 0)
  return evaluation({
    items: { ...(worn ? { hat: worn } : {}), ...(gear.items.weapon === sword.id ? { weapon: sword } : {}) },
    stats: { 0: hp, 1: 6, 23: 3 },
  })
}

function simulate(gear: RequestBody): FightView {
  callInputs.push({ gear, action: gear.action?.type })
  const hp = testEvaluation(gear).stats["0"]!
  let state: FightState = gear.state ?? {
    board: { width: 4, height: 4, walls: [] },
    phase: "placement",
    turn: 0,
    apUsed: 0,
    mpUsed: 0,
    uses: {},
    totalDamage: 0,
    turnDamage: 0,
    entities: [{ id: 1, kind: "player", cell: 5, hp, maxHp: hp, buffs: [] }],
  }
  if (gear.action) state = { ...state, phase: "fight", turn: state.turn + 1 }
  return { state, spells: [], names: {}, events: [], steps: [], ap: 6, maxAp: 6, mp: 3, maxMp: 3 }
}

function openCombat(build = { ...newBuild("Essai"), breedId: 9, items: { hat: hat.id, weapon: sword.id } }) {
  const input = toBuildInput(build)
  const view = render(
    <PlatformProvider platform={{ kind: "web", apiBaseUrl: "http://api.test", version: "1" }}>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <FightPanel meta={meta} build={build} input={input} evaluation={testEvaluation(input)} onExit={() => {}} />
      </QueryClientProvider>
    </PlatformProvider>
  )
  return { build, input, view }
}

beforeEach(() => {
  useStore.setState({ lang: "fr" })
  callInputs.length = 0
  Element.prototype.scrollTo ??= () => {}
  vi.stubGlobal("fetch", async (url: string | URL | Request, init?: RequestInit) => {
    const route = new URL(String(url)).pathname
    const body = init?.body && (JSON.parse(String(init.body)) as RequestBody)
    let result: unknown
    if (route === "/api/data/status") result = { state: "idle", ready: true, installedVersion: "1", importedAt: "now" }
    else if (route === "/api/fight/assets") result = { maps: [], pack: "" }
    else if (route === "/api/items") result = { items: [hat, replacement], total: 2 }
    else if (route === "/api/builds/evaluate" && body) result = testEvaluation(body)
    else if (route === "/api/fight" && body) result = simulate(body)
    else return new Response("{}", { status: 404 })
    return new Response(JSON.stringify(result), { status: 200 })
  })
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const click = async (name: string) => fireEvent.click(await screen.findByRole("button", { name }))

it("stages a change while playing, cancels without spending a turn, and restores the original stuff after a confirmed trial", async () => {
  const { input } = openCombat()
  await click("Prêt")
  await click("Fin de tour")
  await click("Inventaire")
  await click("Chapeau · Coiffe d’essai")
  fireEvent.click(await screen.findByRole("button", { name: /Coiffe légère/ }))
  await click("Revenir à l’inventaire")
  const starts = () => callInputs.filter((c) => !c.action)
  expect(starts()).toHaveLength(1)
  await click("Redémarrer avec ces changements")
  let confirm = await screen.findByRole("dialog", { name: "Redémarrer avec ces changements" })
  fireEvent.click(within(confirm).getByRole("button", { name: "Annuler" }))
  await click("Fermer")
  await click("Fin de tour")
  expect(callInputs.at(-1)?.gear.items.hat).toBe(hat.id)
  expect(callInputs.at(-1)?.gear).toMatchObject({ state: { turn: 2 } })
  await click("Inventaire")
  await click("Redémarrer avec ces changements")
  confirm = await screen.findByRole("dialog", { name: "Redémarrer avec ces changements" })
  fireEvent.click(within(confirm).getByRole("button", { name: "Recommencer" }))
  await screen.findByRole("button", { name: "Prêt" })
  expect(starts().at(-1)?.gear.items.hat).toBe(replacement.id)
  expect(await screen.findByRole("button", { name: /Cra, 900 \/ 900 PV/ })).toBeDefined()
  await click("Inventaire")
  await click("Reprendre le stuff initial")
  await click("Redémarrer avec ces changements")
  confirm = await screen.findByRole("dialog", { name: "Redémarrer avec ces changements" })
  fireEvent.click(within(confirm).getByRole("button", { name: "Recommencer" }))
  await waitFor(() => expect(starts().at(-1)?.gear.items.hat).toBe(hat.id))
  expect(input.items).toEqual({ hat: hat.id, weapon: sword.id })
})

it("edits a natural roll on right-click without touching the active fight or the saved build", async () => {
  const { input } = openCombat()
  await click("Prêt")
  await click("Inventaire")
  fireEvent.contextMenu(await screen.findByRole("button", { name: "Chapeau · Coiffe d’essai" }))
  const field = await screen.findByRole("textbox", { name: "Vitalité, Valeur" })
  fireEvent.change(field, { target: { value: "60" } })
  fireEvent.blur(field)
  await click("Revenir à l’inventaire")
  expect(await screen.findByText(/Coiffe d’essai · Jets modifiés/)).toBeDefined()
  await click("Annuler")
  expect(screen.queryByText("Changements en attente")).toBeNull()
  expect(input.mods).toEqual({})
  expect(callInputs.every((c) => !c.gear.mods.hat)).toBe(true)
})

it("applies the weapon element only to a confirmed trial and restores the saved build on leaving combat", async () => {
  const { build, input, view } = openCombat()
  await click("Prêt")
  await click("Inventaire")
  fireEvent.contextMenu(await screen.findByRole("button", { name: "Arme · Épée neutre" }))
  await click("Eau")
  await click("Revenir à l’inventaire")
  expect(callInputs.at(-1)?.gear.mods.weapon).toBeUndefined()
  await click("Redémarrer avec ces changements")
  const confirm = await screen.findByRole("dialog", { name: "Redémarrer avec ces changements" })
  fireEvent.click(within(confirm).getByRole("button", { name: "Recommencer" }))
  await waitFor(() => expect(callInputs.at(-1)?.gear.mods.weapon?.element).toBe("water"))
  expect(input.mods).toEqual({})
  view.unmount()
  openCombat(build)
  await screen.findByRole("button", { name: "Prêt" })
  expect(callInputs.at(-1)?.gear.mods.weapon).toBeUndefined()
})
