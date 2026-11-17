import { Button } from "@ui-sdk/react/button"
import * as React from "react"
import { useItems } from "../../api/queries"
import type { Item, Meta, Position } from "../../api/types"
import { useI18n } from "../../i18n"
import { statIndex } from "../../lib/stats"
import type { Build } from "../../store"
import { Effects } from "../../ui/Effects"
import { ItemCell } from "../../ui/ItemCell"
import { StatSelect } from "../../ui/StatSelect"

interface Props {
  meta: Meta
  build: Build
  position: Position
  current: Item | undefined
  onEquip(item: Item | null): void
}

export function ItemPicker({ meta, build, position, current, onEquip }: Props) {
  const { t, slotName } = useI18n()
  const [search, setSearch] = React.useState("")
  const [sortStat, setSortStat] = React.useState<number | null>(null)
  const stats = statIndex(meta)
  const listRef = React.useRef<HTMLDivElement>(null)

  const result = useItems({
    q: search.trim() || undefined,
    slots: [position.slot],
    // Prysmaradites are only offered where they can be worn.
    excludeTypes: position.key === meta.prysmaradite.position ? undefined : [meta.prysmaradite.typeId],
    levelMax: build.level,
    sort: sortStat === null ? "level" : `stat:${sortStat}`,
    order: "desc",
  })
  const items = result.data?.pages.flatMap((page) => page.items) ?? []

  React.useEffect(() => {
    listRef.current?.scrollTo({ top: 0 })
  }, [position.key, search, sortStat])

  return (
    <aside className="flex h-full flex-col bg-panel" aria-label={t("build.pick", { slot: slotName(position.slot) })}>
      <header className="grid gap-3 border-b border-line p-4">
        <h2 className="m-0 text-base font-bold">{t("build.pick", { slot: slotName(position.slot) })}</h2>
        {current && (
          <div className="flex items-center gap-2.5">
            <ItemCell item={current} size="2.5rem" />
            <span className="min-w-0 flex-1 truncate font-semibold">{current.name}</span>
            <Button onClick={() => onEquip(null)}>{t("build.unequip")}</Button>
          </div>
        )}
        <input
          type="search"
          className="text-input"
          aria-label={t("items.search")}
          placeholder={t("items.searchPlaceholder")}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div className="flex items-center gap-2">
          <span className="text-ink-2">{t("build.pickerSort")}</span>
          <StatSelect
            meta={meta}
            label={t("build.pickerSort")}
            value={sortStat}
            placeholder={t("build.pickerLevel")}
            exclude={[0]}
            onChange={setSortStat}
          />
          {sortStat !== null && (
            <Button className="btn-ghost" onClick={() => setSortStat(null)}>
              {t("build.pickerLevel")}
            </Button>
          )}
        </div>
      </header>

      <div
        ref={listRef}
        className="min-h-0 flex-1 overflow-y-auto"
        onScroll={(event) => {
          const el = event.currentTarget
          if (result.hasNextPage && !result.isFetchingNextPage && el.scrollHeight - el.scrollTop - el.clientHeight < 600) {
            void result.fetchNextPage()
          }
        }}
      >
        {result.isPending ? (
          <div className="grid gap-2 p-4">
            {Array.from({ length: 8 }, (_, i) => (
              <div key={i} className="skeleton h-14" />
            ))}
          </div>
        ) : items.length === 0 ? (
          <p className="m-0 p-4 text-ink-2">{t("build.pickerEmpty")}</p>
        ) : (
          <ul className="m-0 list-none p-0">
            {items.map((item) => {
              const equipped = current?.id === item.id
              return (
                <li key={item.id} className="border-b border-line">
                  <button
                    type="button"
                    className="grid w-full cursor-pointer grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 border-0 bg-transparent px-4 py-2.5 text-left text-ink transition-colors hover:bg-panel-2 aria-pressed:bg-accent-soft"
                    aria-pressed={equipped}
                    onClick={() => onEquip(item)}
                  >
                    <ItemCell item={item} size="2.75rem" />
                    <span className="grid min-w-0 content-center">
                      <span className="truncate font-semibold">{item.name}</span>
                      <span className="truncate text-[0.86rem] text-ink-3">
                        {t("item.level", { level: item.level })}
                        {item.set ? ` · ${item.set.name}` : ""}
                      </span>
                    </span>
                    <span className="effects-cols col-span-2 text-[0.93rem]">
                      <Effects effects={item.effects} stats={stats} />
                      {item.condition && <span className="mt-1 block text-ink-2">{item.condition}</span>}
                      {item.passive && (
                        <span className="mt-1 block text-ink-2">
                          <strong className="text-ink">{item.passive.name}</strong> : {item.passive.description}
                        </span>
                      )}
                    </span>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </aside>
  )
}
