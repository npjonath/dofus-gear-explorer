import type {
  AttackRange,
  BuildInput,
  DamageResult,
  DataStatus,
  Evaluation,
  FightAction,
  FightAim,
  FightDummies,
  FightState,
  FightView,
  ItemDetail,
  ItemList,
  ItemQuery,
  Lang,
  Meta,
  SolverPage,
  SolverRequest,
  SolverResponse,
} from "./types"

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string
  ) {
    super(message)
  }
}

export interface ApiClient {
  baseUrl: string
  imageUrl(path: string): string
  fightAssets(): Promise<{ maps: number[]; pack:string }>
  status(): Promise<DataStatus>
  checkForUpdate(): Promise<DataStatus>
  startUpdate(): Promise<DataStatus>
  meta(lang: Lang): Promise<Meta>
  items(lang: Lang, query: ItemQuery, limit: number, offset: number, signal?: AbortSignal): Promise<ItemList>
  item(lang: Lang, id: number): Promise<ItemDetail>
  evaluate(lang: Lang, build: BuildInput, signal?: AbortSignal): Promise<Evaluation>
  damage(
    lang: Lang,
    build: BuildInput,
    breedId: number | null,
    range: AttackRange,
    mastery: boolean,
    signal?: AbortSignal
  ): Promise<DamageResult>
  /** Starts a fight (no state) or plays one action of it. */
  fight(
    lang: Lang,
    build: BuildInput,
    breedId: number | null,
    body: { dummies: FightDummies } | { state: FightState; action: FightAction },
    signal?: AbortSignal
  ): Promise<FightView>
  /** Lists where a spell can be aimed, with what each cast would do. */
  fightAim(
    lang: Lang,
    build: BuildInput,
    breedId: number | null,
    state: FightState,
    spell: number,
    signal?: AbortSignal
  ): Promise<FightAim>
  /** Runs a search and returns its first page of stuffs. */
  solve(lang: Lang, request: SolverRequest, limit: number, signal?: AbortSignal): Promise<SolverResponse>
  /** Reads another page of a search's stuffs; fails with `results_expired` once the server dropped them. */
  results(id: string, offset: number, limit: number): Promise<SolverPage>
}

export function createApiClient(baseUrl: string): ApiClient {
  const root = baseUrl.replace(/\/$/, "")

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    let response: Response
    try {
      response = await fetch(root + path, init)
    } catch (cause) {
      if (cause instanceof DOMException && cause.name === "AbortError") throw cause
      throw new ApiError(0, "unreachable", "API unreachable")
    }
    if (!response.ok) {
      const body = (await response.json().catch(() => null)) as { error?: string; message?: string } | null
      throw new ApiError(response.status, body?.error ?? "error", body?.message ?? response.statusText)
    }
    return (await response.json()) as T
  }

  const post = <T>(path: string, body: unknown, signal?: AbortSignal) =>
    request<T>(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    })

  return {
    baseUrl: root,
    imageUrl: (path) => root + path,
    fightAssets: () => request("/api/fight/assets"),
    status: () => request("/api/data/status"),
    checkForUpdate: () => post("/api/data/check", {}),
    startUpdate: () => post("/api/data/update", {}),
    meta: (lang) => request(`/api/meta?lang=${lang}`),
    items: (lang, query, limit, offset, signal) => {
      const params = new URLSearchParams({ lang, limit: String(limit), offset: String(offset) })
      if (query.q) params.set("q", query.q)
      if (query.slots?.length) params.set("slots", query.slots.join(","))
      if (query.types?.length) params.set("types", query.types.join(","))
      if (query.excludeTypes?.length) params.set("excludeTypes", query.excludeTypes.join(","))
      if (query.levelMin) params.set("levelMin", String(query.levelMin))
      if (query.levelMax) params.set("levelMax", String(query.levelMax))
      if (query.passive) params.set("passive", "true")
      if (query.sort) params.set("sort", query.sort)
      if (query.order) params.set("order", query.order)
      for (const f of query.stats ?? []) params.append("stat", `${f.statId}:${f.op}:${f.value}`)
      return request(`/api/items?${params}`, { signal })
    },
    item: (lang, id) => request(`/api/items/${id}?lang=${lang}`),
    evaluate: (lang, build, signal) => post(`/api/builds/evaluate?lang=${lang}`, build, signal),
    damage: (lang, build, breedId, range, mastery, signal) =>
      post(`/api/builds/damage?lang=${lang}`, { ...build, breedId: breedId ?? 0, range, mastery }, signal),
    fight: (lang, build, breedId, body, signal) =>
      post(`/api/fight?lang=${lang}`, { ...build, breedId: breedId ?? 0, ...body }, signal),
    fightAim: (lang, build, breedId, state, spell, signal) =>
      post(`/api/fight/aim?lang=${lang}`, { ...build, breedId: breedId ?? 0, state, spell }, signal),
    solve: (lang, body, limit, signal) => post(`/api/solver/search?lang=${lang}&limit=${limit}`, body, signal),
    results: (id, offset, limit) =>
      request(`/api/solver/results/${encodeURIComponent(id)}?offset=${offset}&limit=${limit}`),
  }
}
