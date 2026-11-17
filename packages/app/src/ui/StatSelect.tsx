import * as React from "react"
import type { Meta } from "../api/types"
import { useI18n } from "../i18n"
import { statLabel, toneOf } from "../lib/stats"
import { SearchSelect, type SearchGroup } from "./SearchSelect"

interface Props {
  meta: Meta
  label: string
  value: number | null
  placeholder?: string
  exclude?: number[]
  /** Keep only the stats forgemagic can add. */
  forgeable?: boolean
  onChange(statId: number): void
}

export function StatSelect({ meta, label, value, placeholder, exclude = [], forgeable, onChange }: Props) {
  const { t } = useI18n()
  const groups = React.useMemo<SearchGroup[]>(() => {
    const byId = new Map(meta.stats.map((s) => [s.id, s]))
    const hidden = new Set(exclude.filter((id) => id !== value))
    return meta.categories.map((category) => ({
      label: category.name,
      options: category.statIds
        .map((id) => byId.get(id))
        .filter((s) => s !== undefined)
        .filter((s) => !hidden.has(s.id) && (!forgeable || s.forgeMax > 0))
        .map((s) => ({ value: String(s.id), label: statLabel(s), tone: toneOf(s.key) })),
    }))
    // `exclude` is rebuilt on every render; its content is what matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [meta, value, forgeable, exclude.join(",")])

  return (
    <SearchSelect
      label={label}
      groups={groups}
      value={value === null ? null : String(value)}
      placeholder={placeholder}
      emptyText={t("filter.noStat")}
      showLabel={t("filter.showStats")}
      onChange={(next) => onChange(Number(next))}
    />
  )
}
