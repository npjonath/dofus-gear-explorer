import { Button } from "@ui-sdk/react/button"
import { Select, SelectContent, SelectGroup, SelectGroupLabel, SelectItem, SelectTrigger } from "@ui-sdk/react/select"
import {
  columnVisibilityFeature,
  rowSortingFeature,
  tableFeatures,
  useTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table"
import { useVirtualizer } from "@tanstack/react-virtual"
import {
  ArrowDownIcon,
  ArrowUpDownIcon,
  ArrowUpIcon,
  Columns3Icon,
  PlusIcon,
  SlidersHorizontalIcon,
  SparkleIcon,
} from "lucide-react"
import * as React from "react"
import { useItems } from "../../api/queries"
import type { Item, ItemQuery, Meta, Slot, StatFilter } from "../../api/types"
import { useI18n } from "../../i18n"
import { statLabel, toneOf } from "../../lib/stats"
import { useBreakpoint } from "../../lib/useBreakpoint"
import { useStore } from "../../store"
import { defaultThreshold, FilterRow } from "../../ui/FilterRow"
import { ItemCell } from "../../ui/ItemCell"
import { NumberField } from "../../ui/NumberField"
import { SidePanels } from "../../ui/SidePanels"
import { SlotGlyph } from "../../ui/SlotGlyph"
import { ItemInspector } from "./ItemInspector"

const features = tableFeatures({ rowSortingFeature, columnVisibilityFeature })
const ROW_HEIGHT = 48

function statRange(item: Item, statId: number): { min: number; max: number } | null {
  let found = false
  let min = 0
  let max = 0
  for (const effect of item.effects) {
    if (effect.kind === "stat" && effect.statId === statId) {
      found = true
      min += effect.min
      max += effect.max
    }
  }
  return found ? { min, max } : null
}

function useDebounced<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = React.useState(value)
  React.useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}

export function ExplorerPage({ meta }: { meta: Meta }) {
  const { t, slotName, number } = useI18n()
  const statColumns = useStore((s) => s.statColumns)
  const setStatColumns = useStore((s) => s.setStatColumns)

  const [search, setSearch] = React.useState("")
  const [slots, setSlots] = React.useState<Slot[]>([])
  const [levelMin, setLevelMin] = React.useState(1)
  const [levelMax, setLevelMax] = React.useState(meta.maxLevel)
  const [filters, setFilters] = React.useState<StatFilter[]>([])
  const [sorting, setSorting] = React.useState<SortingState>([{ id: "level", desc: true }])
  const [selected, setSelected] = React.useState<Item | null>(null)
  const [filtersOpen, setFiltersOpen] = React.useState(false)
  const [passiveOnly, setPassiveOnly] = React.useState(false)
  const breakpoint = useBreakpoint()

  const sort = sorting[0]
  const query = useDebounced<ItemQuery>(
    React.useMemo(
      () => ({
        q: search.trim() || undefined,
        slots: slots.length ? slots : undefined,
        levelMin: levelMin > 1 ? levelMin : undefined,
        levelMax: levelMax < meta.maxLevel ? levelMax : undefined,
        stats: filters.length ? filters : undefined,
        passive: passiveOnly || undefined,
        sort: sort ? (sort.id.startsWith("stat-") ? `stat:${sort.id.slice(5)}` : sort.id) : undefined,
        order: sort?.desc ? "desc" : "asc",
      }),
      [search, slots, levelMin, levelMax, filters, passiveOnly, sort, meta.maxLevel]
    ),
    180
  )
  const result = useItems(query)
  const items = React.useMemo(() => result.data?.pages.flatMap((page) => page.items) ?? [], [result.data])
  const total = result.data?.pages[0]?.total ?? 0

  // Filtered stats always get a column, so the reason a row matches is visible.
  const visibleStats = React.useMemo(() => {
    const ids = [...statColumns]
    for (const filter of filters) if (!ids.includes(filter.statId)) ids.push(filter.statId)
    return ids.map((id) => meta.stats.find((s) => s.id === id)).filter((s) => s !== undefined)
  }, [statColumns, filters, meta.stats])

  const columns = React.useMemo<ColumnDef<typeof features, Item>[]>(
    () => [
      {
        id: "name",
        header: t("items.col.name"),
        cell: ({ row }) => (
          <span className="flex min-w-0 items-center gap-2.5">
            <ItemCell item={row.original} size="2.5rem" />
            <span className="grid min-w-0">
              <span className="truncate font-semibold">{row.original.name}</span>
              <span className="truncate text-[0.86rem] text-ink-3">
                {row.original.typeName}
                {row.original.set ? ` · ${row.original.set.name}` : ""}
                {row.original.passive ? ` · ${row.original.passive.name}` : ""}
              </span>
            </span>
          </span>
        ),
      },
      {
        id: "level",
        header: t("items.col.level"),
        cell: ({ row }) => <span className="font-semibold">{row.original.level}</span>,
      },
      ...visibleStats.map<ColumnDef<typeof features, Item>>((stat) => ({
        id: `stat-${stat.id}`,
        header: statLabel(stat),
        sortDescFirst: true,
        cell: ({ row }) => {
          const range = statRange(row.original, stat.id)
          if (!range) return null
          return (
            <span title={range.min === range.max ? undefined : `${range.min} – ${range.max}${stat.unit}`}>
              <span className="stat-value" data-tone={toneOf(stat.key)} data-negative={range.max < 0 ? "" : undefined}>
                {range.max}
                {stat.unit}
              </span>
            </span>
          )
        },
      })),
    ],
    [t, visibleStats]
  )

  const table = useTable<typeof features, Item>({
    features,
    columns,
    data: items,
    getRowId: (item) => String(item.id),
    manualSorting: true,
    enableSortingRemoval: false,
    state: { sorting },
    onSortingChange: setSorting,
  })

  const scrollRef = React.useRef<HTMLDivElement>(null)
  const rows = table.getRowModel().rows
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  })
  const virtualRows = virtualizer.getVirtualItems()
  const before = virtualRows[0]?.start ?? 0
  const after = virtualizer.getTotalSize() - (virtualRows.at(-1)?.end ?? 0)

  const lastIndex = virtualRows.at(-1)?.index ?? 0
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = result
  React.useEffect(() => {
    if (hasNextPage && !isFetchingNextPage && lastIndex >= rows.length - 15) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, lastIndex, rows.length])

  React.useEffect(() => {
    scrollRef.current?.scrollTo({ top: 0 })
  }, [query])

  const usedStats = filters.map((f) => f.statId)
  const firstFreeStat = meta.stats.find((s) => !usedStats.includes(s.id) && s.id !== 0)
  const hasFilters =
    search !== "" || slots.length > 0 || filters.length > 0 || passiveOnly || levelMin > 1 || levelMax < meta.maxLevel

  const filterPanel = (
    <div className="grid content-start gap-5 p-4">
      <div className="max-lg:hidden">
        <label className="field-label" htmlFor="item-search">
          {t("items.search")}
        </label>
        <input
          id="item-search"
          type="search"
          className="text-input"
          placeholder={t("items.searchPlaceholder")}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      <fieldset className="m-0 min-w-0 border-0 p-0">
        <legend className="field-label p-0">{t("items.slots")}</legend>
        <div className="flex flex-wrap gap-1.5">
          {meta.slots.map((slot) => {
            const pressed = slots.includes(slot)
            return (
              <button
                key={slot}
                type="button"
                className="chip"
                aria-pressed={pressed}
                onClick={() => setSlots(pressed ? slots.filter((s) => s !== slot) : [...slots, slot])}
              >
                <SlotGlyph slot={slot} className="size-4" />
                {slotName(slot)}
              </button>
            )
          })}
        </div>
      </fieldset>

      <div>
        <button type="button" className="chip" aria-pressed={passiveOnly} onClick={() => setPassiveOnly(!passiveOnly)}>
          <SparkleIcon aria-hidden="true" className="size-4" />
          {t("items.passiveOnly")}
        </button>
      </div>

      <fieldset className="m-0 min-w-0 border-0 p-0">
        <legend className="field-label p-0">{t("items.level")}</legend>
        <div className="flex items-center gap-2">
          <NumberField
            label={t("items.levelMin")}
            value={levelMin}
            min={1}
            max={meta.maxLevel}
            onChange={(value) => setLevelMin(Math.min(value, levelMax))}
          />
          <span className="text-ink-3">–</span>
          <NumberField
            label={t("items.levelMax")}
            value={levelMax}
            min={1}
            max={meta.maxLevel}
            onChange={(value) => setLevelMax(Math.max(value, levelMin))}
          />
        </div>
      </fieldset>

      <fieldset className="m-0 grid min-w-0 gap-2 border-0 p-0">
        <legend className="field-label p-0">{t("items.statFilters")}</legend>
        {filters.map((filter, index) => (
          <FilterRow
            key={filter.statId}
            meta={meta}
            filter={filter}
            exclude={usedStats}
            onChange={(next) => setFilters(filters.map((f, i) => (i === index ? next : f)))}
            onRemove={() => setFilters(filters.filter((_, i) => i !== index))}
          />
        ))}
        <div>
          <Button
            disabled={!firstFreeStat}
            onClick={() => {
              if (!firstFreeStat) return
              setFilters([
                ...filters,
                { statId: firstFreeStat.id, op: "gte", value: defaultThreshold(firstFreeStat.key) },
              ])
            }}
          >
            <PlusIcon aria-hidden="true" className="size-4" />
            {t("items.addFilter")}
          </Button>
        </div>
      </fieldset>

      {hasFilters && (
        <div>
          <Button
            className="btn-ghost"
            onClick={() => {
              setSearch("")
              setSlots([])
              setFilters([])
              setPassiveOnly(false)
              setLevelMin(1)
              setLevelMax(meta.maxLevel)
            }}
          >
            {t("items.reset")}
          </Button>
        </div>
      )}
    </div>
  )

  const tableSection = (
    <section className="flex h-full min-h-0 min-w-0 flex-col bg-panel">
        <div className="flex items-center gap-2 border-b border-line px-4 py-2">
          <Button
            className="lg:hidden"
            aria-expanded={filtersOpen}
            onClick={() => setFiltersOpen(!filtersOpen)}
          >
            <SlidersHorizontalIcon aria-hidden="true" className="size-4" />
            {t("items.filters")}
          </Button>
          <input
            type="search"
            className="text-input min-w-0 flex-1 lg:hidden"
            aria-label={t("items.search")}
            placeholder={t("items.searchPlaceholder")}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <p className="m-0 font-semibold whitespace-nowrap lg:flex-1" role="status">
            {result.isPending ? t("common.loading") : total === 1 ? t("items.count.one") : t("items.count", { count: number(total) })}
          </p>
          <Select
            multiple
            value={statColumns.map(String)}
            onValueChange={(next) => setStatColumns(next.map(Number))}
          >
            <SelectTrigger aria-label={t("items.columnsHint")} className="w-auto! min-w-0! flex-none justify-start! gap-2">
              <Columns3Icon aria-hidden="true" className="size-4" />
              <span className="max-sm:sr-only">{t("items.columns")}</span>
            </SelectTrigger>
            <SelectContent align="end" className="max-h-96">
              {meta.categories.map((category) => (
                <SelectGroup key={category.id}>
                  <SelectGroupLabel>{category.name}</SelectGroupLabel>
                  {category.statIds.map((id) => {
                    const stat = meta.stats.find((s) => s.id === id)
                    if (!stat || stat.id === 0) return null
                    return (
                      <SelectItem key={id} value={String(id)} label={statLabel(stat)}>
                        {statLabel(stat)}
                      </SelectItem>
                    )
                  })}
                </SelectGroup>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div ref={scrollRef} className="min-h-0 flex-1 overflow-auto">
          {result.isError ? (
            <p className="m-0 p-6 text-bad" role="alert">
              {t("items.error")}
            </p>
          ) : !result.isPending && total === 0 ? (
            <div className="grid justify-items-start gap-1 p-6">
              <h2 className="m-0 text-base font-bold">{t("items.empty.title")}</h2>
              <p className="m-0 text-ink-2">{t("items.empty.body")}</p>
            </div>
          ) : (
            <table className="grid-table">
              <thead>
                {table.getHeaderGroups().map((group) => (
                  <tr key={group.id}>
                    {group.headers.map((header) => {
                      // Sorting happens on the server: the header only says which column and which way.
                      const columnId = header.column.id
                      const sorted = sort?.id === columnId ? (sort.desc ? "desc" : "asc") : false
                      const label = String(header.column.columnDef.header)
                      return (
                        <th
                          key={header.id}
                          scope="col"
                          aria-sort={sorted === "asc" ? "ascending" : sorted === "desc" ? "descending" : undefined}
                        >
                          <button
                            type="button"
                            className="th-sort"
                            title={t("items.sortBy", { column: label })}
                            onClick={() =>
                              setSorting([
                                {
                                  id: columnId,
                                  // A second click flips the order; numbers start with the highest.
                                  desc: sort?.id === columnId ? !sort.desc : columnId !== "name",
                                },
                              ])
                            }
                          >
                            {label}
                            {sorted === "asc" ? (
                              <ArrowUpIcon aria-hidden="true" />
                            ) : sorted === "desc" ? (
                              <ArrowDownIcon aria-hidden="true" />
                            ) : (
                              <ArrowUpDownIcon aria-hidden="true" />
                            )}
                          </button>
                        </th>
                      )
                    })}
                  </tr>
                ))}
              </thead>
              <tbody>
                {before > 0 && (
                  <tr aria-hidden="true">
                    <td colSpan={columns.length} style={{ height: before, padding: 0, border: 0 }} />
                  </tr>
                )}
                {result.isPending &&
                  Array.from({ length: 14 }, (_, i) => (
                    <tr key={i} aria-hidden="true">
                      <td colSpan={columns.length}>
                        <div className="skeleton h-6 w-full" />
                      </td>
                    </tr>
                  ))}
                {virtualRows.map((virtualRow) => {
                  const row = rows[virtualRow.index]!
                  return (
                    <tr
                      key={row.id}
                      tabIndex={0}
                      aria-selected={selected?.id === row.original.id}
                      onClick={() => setSelected(row.original)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault()
                          setSelected(row.original)
                        }
                      }}
                    >
                      {row.getAllCells().map((cell) => (
                        <td key={cell.id}>
                          <table.FlexRender cell={cell} />
                        </td>
                      ))}
                    </tr>
                  )
                })}
                {after > 0 && (
                  <tr aria-hidden="true">
                    <td colSpan={columns.length} style={{ height: after, padding: 0, border: 0 }} />
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </section>
  )

  const sheet = selected ? (
    <ItemInspector meta={meta} item={selected} onClose={() => setSelected(null)} />
  ) : (
    <div className="board grid h-full place-items-center p-8">
      <p className="m-0 max-w-[26ch] rounded-lg border border-line bg-panel p-5 text-center text-ink-2">
        {t("item.noSelection")}
      </p>
    </div>
  )

  // Below xl the sheet has no column of its own: it slides over the table.
  const overlay = selected && breakpoint !== "xl" && (
    <div className="fixed inset-y-0 right-0 z-20 w-[min(24rem,100vw)] border-l border-line shadow-[var(--shadow-pop)]">
      {sheet}
    </div>
  )

  if (breakpoint === "sm") {
    return (
      <div className="flex h-full flex-col">
        {filtersOpen && (
          <div className="max-h-[60dvh] flex-none overflow-y-auto border-b border-line bg-ground">{filterPanel}</div>
        )}
        <div className="min-h-0 flex-1">{tableSection}</div>
        {overlay}
      </div>
    )
  }

  return (
    <>
      <SidePanels
        id="items"
        left={<div className="h-full overflow-y-auto bg-ground">{filterPanel}</div>}
        right={breakpoint === "xl" ? sheet : undefined}
      >
        {tableSection}
      </SidePanels>
      {overlay}
    </>
  )
}
