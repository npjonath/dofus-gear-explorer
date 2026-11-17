import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApi } from "../platform"
import { useStore } from "../store"
import type { AttackRange, BuildInput, DataStatus, ItemQuery, SolverRequest } from "./types"

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
  const version = useDataStatus().data?.installedVersion
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
  const version = useDataStatus().data?.installedVersion
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
  const version = useDataStatus().data?.installedVersion
  return useQuery({
    queryKey: ["item", lang, version, id],
    queryFn: () => api.item(lang, id!),
    enabled: id !== null && version !== undefined,
    staleTime: 5 * 60_000,
  })
}

export function useEvaluation(build: BuildInput) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataStatus().data?.installedVersion
  return useQuery({
    queryKey: ["evaluate", lang, version, build],
    queryFn: ({ signal }) => api.evaluate(lang, build, signal),
    enabled: version !== undefined,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

export function useDamage(build: BuildInput, breedId: number | null, range: AttackRange, mastery: boolean) {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const version = useDataStatus().data?.installedVersion
  return useQuery({
    queryKey: ["damage", lang, version, build, breedId, range, mastery],
    queryFn: ({ signal }) => api.damage(lang, build, breedId, range, mastery, signal),
    enabled: version !== undefined,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

/** Stuffs a search shows at once; the server keeps the rest. */
export const RESULTS_PAGE = 5

export function useSolver() {
  const api = useApi()
  const lang = useStore((s) => s.lang)
  return useMutation({
    mutationFn: (request: SolverRequest) => api.solve(lang, request, RESULTS_PAGE),
  })
}
