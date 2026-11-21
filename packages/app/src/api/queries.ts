import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApi } from "../platform"
import { useStore, type SavedSearch } from "../store"
import type {
  AttackRange,
  BuildInput,
  DataStatus,
  FightAction,
  FightDummies,
  FightState,
  ItemQuery,
  SolverRequest,
} from "./types"

const PAGE_SIZE = 60

export function useDataStatus() {
  const api = useApi()
  return useQuery({
    queryKey: ["status"],
    queryFn: api.status,
    refetchInterval: (query) => {
      const status = query.state.data
      if (query.state.error) return 3000
      return !status || !status.ready || status.state !== "idle" ? 700 : 60_000
    },
    retry: false,
  })
}

/**
 * Identifies the imported data in query keys. It changes with every import,
 * including one of the same release that adds a language.
 */
function useDataKey(): string | undefined {
  const status = useDataStatus().data
  return status && `${status.installedVersion}@${status.importedAt}`
}

export function useCheckForUpdate() {
  const api = useApi()
  const client = useQueryClient()
  return useMutation({
    mutationFn: api.checkForUpdate,
    onSuccess: (status: DataStatus) => client.setQueryData(["status"], status),
  })
}

export function useStartUpdate() {
  const api = useApi()
  const client = useQueryClient()
  return useMutation({
    mutationFn: api.startUpdate,
    onSuccess: (status: DataStatus) => client.setQueryData(["status"], status),
  })
}

export function useMeta() {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useQuery({
    queryKey: ["meta", lang, version],
    queryFn: () => api.meta(lang),
    enabled: version !== undefined,
    staleTime: Infinity,
    placeholderData: keepPreviousData,
  })
}

export function useItems(query: ItemQuery) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useInfiniteQuery({
    queryKey: ["items", lang, version, query],
    queryFn: ({ pageParam, signal }) => api.items(lang, query, PAGE_SIZE, pageParam, signal),
    initialPageParam: 0,
    enabled: version !== undefined,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, page) => n + page.items.length, 0)
      return loaded < last.total ? loaded : undefined
    },
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

export function useItem(id: number | null) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useQuery({
    queryKey: ["item", lang, version, id],
    queryFn: () => api.item(lang, id!),
    enabled: id !== null && version !== undefined,
    staleTime: 5 * 60_000,
  })
}

export function useEvaluation(build: BuildInput, enabled = true) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useQuery({
    queryKey: ["evaluate", lang, version, build],
    queryFn: ({ signal }) => api.evaluate(lang, build, signal),
    enabled: enabled && version !== undefined,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

export function useDamage(
  build: BuildInput,
  breedId: number | null,
  range: AttackRange,
  mastery: boolean,
  enabled = true
) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useQuery({
    queryKey: ["damage", lang, version, build, breedId, range, mastery],
    queryFn: ({ signal }) => api.damage(lang, build, breedId, range, mastery, signal),
    enabled: enabled && version !== undefined,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

/** A new fight for a stuff; `round` asks for another one with the same settings. */
export function useFightStart(build: BuildInput, breedId: number | null, dummies: FightDummies, round: number, assetsChecked: boolean) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useQuery({
    queryKey: ["fight", "start", lang, version, build, breedId, dummies, round],
    queryFn: ({ signal }) => api.fight(lang, build, breedId, { dummies }, signal),
    placeholderData: keepPreviousData,
    enabled: version !== undefined && assetsChecked,
    staleTime: Infinity,
  })
}

/** Plays one action of a fight and returns the fight as it then stands. */
export function useFightAction(build: BuildInput, breedId: number | null) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  return useMutation({
    mutationFn: ({ state, action }: { state: FightState; action: FightAction }) =>
      api.fight(lang, build, breedId, { state, action }),
  })
}

/** Where the spell in hand can be aimed, and what each cast would do. */
export function useFightAim(build: BuildInput, breedId: number | null, state: FightState | undefined, spell: number | null) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataKey()
  return useQuery({
    queryKey: ["fight", "aim", lang, version, build, breedId, state, spell],
    queryFn: ({ signal }) => api.fightAim(lang, build, breedId, state!, spell!, signal),
    enabled: version !== undefined && state !== undefined && spell !== null && state.phase === "fight",
    staleTime: Infinity,
    gcTime: 30_000,
  })
}

/** Stuffs a search shows at once; the server keeps the rest. */
export const RESULTS_PAGE = 5

export function useSolver() {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  return useMutation({
    mutationFn: ({ request }: { request: SolverRequest; search: SavedSearch }) => api.solve(lang, request, RESULTS_PAGE),
  })
}
