import { beforeEach, describe, expect, it } from "vitest"
import { newBuild, useStore } from "./store"

function reset() {
  const first = newBuild("A")
  useStore.setState({ builds: [first], activeBuildId: first.id, openBuildIds: [first.id], stuffTab: "build" })
  return first
}

describe("stuff tabs", () => {
  let a: ReturnType<typeof newBuild>
  beforeEach(() => {
    a = reset()
  })

  it("opens a new stuff in a tab and leaves the list", () => {
    useStore.getState().showTab("list")
    const b = newBuild("B")
    useStore.getState().addBuild(b)
    const state = useStore.getState()
    expect(state.openBuildIds).toEqual([a.id, b.id])
    expect(state.activeBuildId).toBe(b.id)
    expect(state.stuffTab).toBe("build")
  })

  it("closes a tab without deleting the stuff, and hands over to its neighbour", () => {
    const b = newBuild("B")
    const c = newBuild("C")
    useStore.getState().addBuild(b)
    useStore.getState().addBuild(c)
    useStore.getState().closeBuild(c.id)
    expect(useStore.getState().activeBuildId).toBe(b.id)
    // Closing a tab that is not the active one changes nothing else.
    useStore.getState().closeBuild(a.id)
    expect(useStore.getState().openBuildIds).toEqual([b.id])
    expect(useStore.getState().activeBuildId).toBe(b.id)
    expect(useStore.getState().builds).toHaveLength(3)
  })

  it("shows the list when the last tab closes, and reopens a stuff from it", () => {
    useStore.getState().closeBuild(a.id)
    expect(useStore.getState().openBuildIds).toEqual([])
    expect(useStore.getState().stuffTab).toBe("list")
    useStore.getState().selectBuild(a.id)
    expect(useStore.getState().openBuildIds).toEqual([a.id])
    expect(useStore.getState().stuffTab).toBe("build")
    useStore.getState().selectBuild("missing")
    expect(useStore.getState().activeBuildId).toBe(a.id)
  })

  it("deletes a stuff with its tab; deleting the only one leaves a fresh stuff and the list", () => {
    const b = newBuild("B")
    useStore.getState().addBuild(b)
    useStore.getState().removeBuild(b.id, "New")
    expect(useStore.getState().openBuildIds).toEqual([a.id])
    expect(useStore.getState().activeBuildId).toBe(a.id)
    useStore.getState().removeBuild(a.id, "New")
    expect(useStore.getState().builds).toHaveLength(1)
    expect(useStore.getState().builds[0]!.name).toBe("New")
    expect(useStore.getState().stuffTab).toBe("list")
  })

  it("targets a stuff for the search without leaving the search tab", () => {
    const b = newBuild("B")
    useStore.getState().addBuild(b)
    useStore.getState().closeBuild(b.id)
    useStore.getState().showTab("search")
    useStore.getState().targetBuild(b.id)
    const state = useStore.getState()
    expect(state.stuffTab).toBe("search")
    expect(state.activeBuildId).toBe(b.id)
    expect(state.openBuildIds).toEqual([a.id, b.id])
  })

  it("keeps the search tab when a stuff's tab closes behind it", () => {
    useStore.getState().showTab("search")
    useStore.getState().closeBuild(a.id)
    expect(useStore.getState().stuffTab).toBe("search")
    expect(useStore.getState().builds).toHaveLength(1)
  })

  it("replaces the gear of a stuff in one undoable step", () => {
    useStore.getState().equip(a.id, "hat", 1)
    useStore.getState().replaceGear(a.id, { items: { ring1: 2 }, mods: {}, base: a.base, scrolls: a.scrolls })
    expect(useStore.getState().builds[0]!.items).toEqual({ ring1: 2 })
    useStore.getState().undo(a.id)
    expect(useStore.getState().builds[0]!.items).toEqual({ hat: 1 })
  })

  it("undoes and redoes the search and character changes together with its equipment", () => {
    const { level, breedId, base, scrolls, items } = a
    const savedSearch = {
      draft: { ...useStore.getInitialState().solver, required: [2] },
      source: { level, breedId, base, scrolls, items },
    }
    useStore.getState().replaceGear(a.id, { items: { ring1: 2 }, mods: {}, base: { ...base, strength: 100 }, scrolls }, savedSearch)
    useStore.getState().undo(a.id)
    expect(useStore.getState().builds[0]).toMatchObject({ items, base, scrolls })
    expect(useStore.getState().builds[0]?.savedSearch).toBeUndefined()
    useStore.getState().redo(a.id)
    expect(useStore.getState().builds[0]).toMatchObject({ items: { ring1: 2 }, base: { strength: 100 }, savedSearch })
  })
})

describe("saved state", () => {
  it("raises a negative search condition to 0", () => {
    const migrate = useStore.persist.getOptions().migrate!
    const saved = {
      builds: [],
      solver: { constraints: [{ statId: 1, op: "gte", value: -5 }, { statId: 23, op: "gte", value: 6 }] },
    }
    const state = migrate(saved, 11) as { solver: { constraints: { value: number }[] } }
    expect(state.solver.constraints.map((c) => c.value)).toEqual([0, 6])
  })
})
