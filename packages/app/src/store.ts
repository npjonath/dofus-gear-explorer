import { create } from "zustand"
import { persist } from "zustand/middleware"
import type {
  BuildInput,
  DamageGoal,
  ItemMods,
  Lang,
  MaximizeEntry,
  PrimaryKey,
  StatFilter,
} from "./api/types"
import { locales, pickLang } from "./locales"

export type ThemeChoice = "system" | "light" | "dark"

export type StuffTab = "list" | "search" | "build"

export interface Build {
  id: string
  name: string
  /** The character's class, which gives its spells; null until chosen. */
  breedId: number | null
  level: number
  base: Record<PrimaryKey, number>
  scrolls: Record<PrimaryKey, number>
  items: Record<string, number>
  /** Forgemagic of the worn items, by position. */
  mods: Record<string, ItemMods>
  updatedAt: number
}

export interface SolverDraft {
  constraints: StatFilter[]
  /** What to push up, by priority: stats, weapons and spells. */
  maximize: MaximizeEntry[]
  keepEquipped: boolean
  exoAp: boolean
  exoMp: boolean
  exoRange: boolean
  transcendence: boolean
  draco: boolean
  /** A negative Critical line may be forged up to -1 instead of taking a rune. */
  raiseCritical: boolean
  /** The solver spends the characteristic points itself. */
  autoPoints: boolean
  /** Every characteristic counts as scrolled to 100. */
  fullScrolls: boolean
  /** Items every stuff found must wear, by id. */
  required: number[]
  /** Items no stuff found may wear, by id. */
  excluded: number[]
  /** Attacks that must hit for at least a value. */
  damage: DamageGoal[]
}

// Brings a saved draft to today's shape. Before version 5 it carried a
// minimum item level; before version 9 damage goals only knew "critical";
// before version 10 the list to maximize held stat ids only, and a damage
// goal said by itself whether it was maximized.
function upgradeDraft(saved: SolverDraft & { levelMin?: number }): SolverDraft {
  const { levelMin: _dropped, ...rest } = saved
  type OldGoal = DamageGoal & { crit?: boolean; maximize?: boolean }
  const goals = ((rest.damage ?? []) as OldGoal[]).map((old) => ({
    goal: {
      kind: old.kind,
      id: old.id,
      measure: old.measure ?? (old.crit ? ("crit" as const) : ("normal" as const)),
      value: old.value ?? 0,
      element: old.element,
    },
    maximize: old.maximize ?? false,
  }))
  const stats = ((rest.maximize ?? []) as (MaximizeEntry | number)[]).map(
    (entry): MaximizeEntry => (typeof entry === "number" ? { kind: "stat", statId: entry } : entry)
  )
  const attacks = goals
    .filter((g) => g.maximize)
    .map(({ goal }): MaximizeEntry => ({ kind: goal.kind, id: goal.id, measure: goal.measure, element: goal.element }))
  return {
    ...rest,
    damage: goals.filter((g) => g.goal.value > 0).map((g) => g.goal),
    maximize: [...attacks, ...stats],
  }
}

const defaultSolver: SolverDraft = {
  constraints: [],
  maximize: [],
  keepEquipped: false,
  exoAp: false,
  exoMp: false,
  exoRange: false,
  transcendence: false,
  draco: false,
  raiseCritical: false,
  autoPoints: true,
  fullScrolls: true,
  required: [],
  excluded: [],
  damage: [],
}

export function isEmptyMods(mods: ItemMods | undefined): boolean {
  if (!mods) return true
  return (
    Object.keys(mods.lines ?? {}).length === 0 &&
    (mods.exos ?? []).length === 0 &&
    !mods.transcendence &&
    !mods.draco &&
    !mods.element
  )
}

const zeroStats = (): Record<PrimaryKey, number> => ({
  vitality: 0,
  wisdom: 0,
  strength: 0,
  intelligence: 0,
  chance: 0,
  agility: 0,
})

export function newBuild(name: string): Build {
  return {
    id: crypto.randomUUID(),
    name,
    breedId: null,
    level: 200,
    base: zeroStats(),
    scrolls: zeroStats(),
    items: {},
    mods: {},
    updatedAt: Date.now(),
  }
}

export function toBuildInput(build: Build): BuildInput {
  return { level: build.level, base: build.base, scrolls: build.scrolls, items: build.items, mods: build.mods }
}

// PA, PM, Portée, Vitalité, Sagesse, Force, Intelligence, Chance, Agilité
export const defaultStatColumns = [1, 23, 19, 11, 12, 10, 15, 13, 14]

/** What undo restores: the items of a stuff and their forgemagic. */
interface Snapshot {
  items: Record<string, number>
  mods: Record<string, ItemMods>
}

const HISTORY_LIMIT = 50

interface State {
  lang: Lang
  theme: ThemeChoice
  builds: Build[]
  /** The stuff the app works on: the one shown, and the one searches and "add to stuff" target. */
  activeBuildId: string
  /** The stuffs open as tabs, in tab order. */
  openBuildIds: string[]
  /** What the Stuff screen shows: the list of stuffs, the stuff search, or the active stuff. */
  stuffTab: StuffTab
  statColumns: number[]
  solver: SolverDraft
  /** Side panel widths in pixels, per screen. */
  panelWidths: Record<string, { left?: number; right?: number }>

  setLang(lang: Lang): void
  setPanelWidths(id: string, widths: { left?: number; right?: number }): void
  setTheme(theme: ThemeChoice): void
  setStatColumns(ids: number[]): void
  updateSolver(patch: Partial<SolverDraft>): void

  /** Saves a new stuff and opens it in a tab. */
  addBuild(build: Build): void
  /** Opens a stuff in a tab, or goes to its tab. */
  selectBuild(id: string): void
  /** Makes a stuff the one searches work on, in a tab, without leaving the current tab. */
  targetBuild(id: string): void
  /** Closes a tab; the stuff stays in the list. */
  closeBuild(id: string): void
  /** Shows one of the tabs that never close. */
  showTab(tab: "list" | "search"): void
  updateBuild(id: string, patch: Partial<Omit<Build, "id">>): void
  removeBuild(id: string, fallbackName: string): void
  /** Puts an item at a position, with its forgemagic when it comes with some. */
  equip(id: string, position: string, itemId: number | null, mods?: ItemMods): void
  /** Replaces the gear of a stuff, and its points and scrolls, in one undoable step. */
  replaceGear(id: string, gear: Pick<Build, "items" | "mods" | "base" | "scrolls">): void
  setMods(id: string, position: string, mods: ItemMods | null): void

  /** Earlier and undone states of each stuff's items, for this session only. */
  past: Record<string, Snapshot[]>
  future: Record<string, Snapshot[]>
  /** Steps a stuff's items back or forward; false when there is nothing to restore. */
  undo(id: string): boolean
  redo(id: string): boolean
}

function browserLang(): Lang {
  if (typeof navigator === "undefined") return "en"
  return pickLang(navigator.languages?.length ? navigator.languages : [navigator.language])
}

export const useStore = create<State>()(
  persist(
    (set, get) => {
      const first = newBuild(locales[browserLang()].messages["build.first"])
      const patchBuild = (state: State, id: string, change: (build: Build) => Build) => ({
        builds: state.builds.map((b) => (b.id === id ? { ...change(b), updatedAt: Date.now() } : b)),
      })
      // Changes to items go through here so they can be undone.
      const patchItems = (state: State, id: string, change: (build: Build) => Build) => {
        const current = state.builds.find((b) => b.id === id)
        if (!current) return {}
        const snapshot: Snapshot = { items: current.items, mods: current.mods }
        return {
          ...patchBuild(state, id, change),
          past: { ...state.past, [id]: [...(state.past[id] ?? []), snapshot].slice(-HISTORY_LIMIT) },
          future: { ...state.future, [id]: [] },
        }
      }
      const step = (from: "past" | "future", to: "past" | "future") => (id: string) => {
        const state = get()
        const stack = state[from][id] ?? []
        const current = state.builds.find((b) => b.id === id)
        const target = stack[stack.length - 1]
        if (!current || !target) return false
        set({
          ...patchBuild(state, id, (b) => ({ ...b, ...target })),
          [from]: { ...state[from], [id]: stack.slice(0, -1) },
          [to]: { ...state[to], [id]: [...(state[to][id] ?? []), { items: current.items, mods: current.mods }] },
        } as Partial<State>)
        return true
      }
      return {
        lang: browserLang(),
        theme: "system",
        builds: [first],
        activeBuildId: first.id,
        openBuildIds: [first.id],
        stuffTab: "build",
        statColumns: defaultStatColumns,
        solver: defaultSolver,
        panelWidths: {},

        setLang: (lang) => set({ lang }),
        setPanelWidths: (id, widths) => set((state) => ({ panelWidths: { ...state.panelWidths, [id]: widths } })),
        setTheme: (theme) => set({ theme }),
        setStatColumns: (statColumns) => set({ statColumns }),
        updateSolver: (patch) => set((state) => ({ solver: { ...state.solver, ...patch } })),

        addBuild: (build) =>
          set((state) => ({
            builds: [...state.builds, build],
            activeBuildId: build.id,
            openBuildIds: [...state.openBuildIds, build.id],
            stuffTab: "build",
          })),
        selectBuild: (id) =>
          set((state) =>
            state.builds.some((b) => b.id === id)
              ? {
                  activeBuildId: id,
                  openBuildIds: state.openBuildIds.includes(id) ? state.openBuildIds : [...state.openBuildIds, id],
                  stuffTab: "build",
                }
              : {}
          ),
        targetBuild: (id) =>
          set((state) =>
            state.builds.some((b) => b.id === id)
              ? {
                  activeBuildId: id,
                  openBuildIds: state.openBuildIds.includes(id) ? state.openBuildIds : [...state.openBuildIds, id],
                }
              : {}
          ),
        closeBuild: (id) =>
          set((state) => {
            const at = state.openBuildIds.indexOf(id)
            if (at < 0) return {}
            const openBuildIds = state.openBuildIds.filter((other) => other !== id)
            if (state.activeBuildId !== id) return { openBuildIds }
            // The neighbour takes over; with no tab left the list shows.
            const next = openBuildIds[Math.max(0, at - 1)]
            if (next) return { openBuildIds, activeBuildId: next }
            return { openBuildIds, stuffTab: state.stuffTab === "build" ? "list" : state.stuffTab }
          }),
        showTab: (stuffTab) => set({ stuffTab }),
        updateBuild: (id, patch) => set((state) => patchBuild(state, id, (b) => ({ ...b, ...patch }))),
        removeBuild: (id, fallbackName) =>
          set((state) => {
            const builds = state.builds.filter((b) => b.id !== id)
            if (builds.length === 0) builds.push(newBuild(fallbackName))
            const openBuildIds = state.openBuildIds.filter((other) => other !== id)
            if (state.activeBuildId !== id) return { builds, openBuildIds }
            const next = openBuildIds[0]
            return {
              builds,
              openBuildIds,
              activeBuildId: next ?? builds[0]!.id,
              stuffTab: next || state.stuffTab !== "build" ? state.stuffTab : "list",
            }
          }),
        past: {},
        future: {},
        undo: step("past", "future"),
        redo: step("future", "past"),

        equip: (id, position, itemId, forged) =>
          set((state) =>
            patchItems(state, id, (b) => {
              const items = { ...b.items }
              const mods = { ...b.mods }
              // Forgemagic belongs to the item it was put on.
              if (items[position] !== itemId || forged) delete mods[position]
              if (itemId === null) delete items[position]
              else items[position] = itemId
              if (itemId !== null && forged && !isEmptyMods(forged)) mods[position] = forged
              return { ...b, items, mods }
            })
          ),
        replaceGear: (id, gear) => set((state) => patchItems(state, id, (b) => ({ ...b, ...gear }))),
        setMods: (id, position, next) =>
          set((state) =>
            patchItems(state, id, (b) => {
              const mods = { ...b.mods }
              if (next === null || isEmptyMods(next)) delete mods[position]
              else mods[position] = next
              return { ...b, mods }
            })
          ),
      }
    },
    {
      name: "dofus-database-explorer",
      version: 11,
      // Version 2 added forgemagic to builds and to the search options;
      // version 3 the characteristic point options of the search; version 4
      // its required items; version 5 dropped its minimum item level;
      // version 6 added the class of a stuff and the damage goals; version 7
      // the tabs, which start with the stuff that was active; version 8 made
      // the search a tab of the Stuff screen; version 9 gave damage goals a
      // measure; version 10 put weapons and spells in the list to maximize;
      // version 11 added the forbidden items.
      migrate: (persisted) => {
        const state = persisted as State
        return {
          ...state,
          builds: (state.builds ?? []).map((b) => ({ ...b, mods: b.mods ?? {}, breedId: b.breedId ?? null })),
          solver: upgradeDraft({ ...defaultSolver, ...state.solver }),
          panelWidths: state.panelWidths ?? {},
          openBuildIds: state.openBuildIds ?? (state.activeBuildId ? [state.activeBuildId] : []),
          stuffTab: state.stuffTab ?? ((state as { listOpen?: boolean }).listOpen ? "list" : "build"),
        }
      },
      partialize: ({
        lang,
        theme,
        builds,
        activeBuildId,
        openBuildIds,
        stuffTab,
        statColumns,
        solver,
        panelWidths,
      }) => ({
        lang,
        theme,
        builds,
        activeBuildId,
        openBuildIds,
        stuffTab,
        statColumns,
        solver,
        panelWidths,
      }),
    }
  )
)

export function useActiveBuild(): Build {
  return useStore((state) => state.builds.find((b) => b.id === state.activeBuildId) ?? state.builds[0]!)
}
