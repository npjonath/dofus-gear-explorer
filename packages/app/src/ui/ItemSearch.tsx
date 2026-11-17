import { useAsyncOptions } from "@ui-sdk/react/async-options"
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@ui-sdk/react/combobox"
import * as React from "react"
import type { Item, Slot } from "../api/types"
import { useI18n } from "../i18n"
import { useApi } from "../platform"
import { useStore } from "../store"
import { ItemCell } from "./ItemCell"

interface Props {
  label: string
  placeholder: string
  /** Items left out of the results, by id. */
  exclude?: number[]
  /** Keep only the items of these slots. */
  slots?: Slot[]
  onPick(item: Item): void
}

// Finds an item by name, across every slot. It empties itself after each pick.
export function ItemSearch({ label, placeholder, exclude = [], slots, onPick }: Props) {
  const { t } = useI18n()
  const api = useApi()
  const lang = useStore((s) => s.lang)
  const [resetKey, setResetKey] = React.useState(0)
  const found = React.useRef(new Map<string, Item>())

  const results = useAsyncOptions<{ value: string; label: string }>(async (query, { signal }) => {
    const q = query.trim()
    if (q.length < 2) return []
    const page = await api.items(lang, { q, slots, sort: "level", order: "desc" }, 20, 0, signal)
    for (const item of page.items) found.current.set(String(item.id), item)
    return page.items.map((item) => ({ value: String(item.id), label: item.name }))
  })

  return (
    <Combobox<string>
      key={resetKey}
      value={null}
      items={results.items}
      filter={false}
      status={results.status}
      onQueryChange={(query) => results.setQuery(query)}
      onValueChange={(id) => {
        const item = id === null ? undefined : found.current.get(id)
        if (!item) return
        onPick(item)
        results.setQuery("")
        setResetKey((key) => key + 1)
      }}
    >
      <div className="search-select">
        <ComboboxInput aria-label={label} placeholder={placeholder} />
      </div>
      <ComboboxContent className="max-h-80 min-w-64">
        <ComboboxEmpty>
          {(state) =>
            !state.empty
              ? null
              : state.loading
                ? t("common.loading")
                : state.query.trim().length < 2
                  ? t("itemSearch.hint")
                  : t("itemSearch.none")
          }
        </ComboboxEmpty>
        <ComboboxList>
          {(state) =>
            (state.items as { value: string; label: string }[])
              .filter((option) => !exclude.includes(Number(option.value)))
              .map((option) => {
                const item = found.current.get(option.value)
                return (
                  <ComboboxItem key={option.value} value={option.value} label={option.label}>
                    {item && <ItemCell item={item} size="1.75rem" />}
                    <span className="grid min-w-0">
                      <span className="truncate">{option.label}</span>
                      {item && (
                        <span className="truncate text-[0.86rem] text-ink-3">
                          {item.typeName} · {t("item.level", { level: item.level })}
                        </span>
                      )}
                    </span>
                  </ComboboxItem>
                )
              })
          }
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
