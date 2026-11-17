import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import { XIcon } from "lucide-react"
import type { Meta, StatFilter } from "../api/types"
import { useI18n } from "../i18n"
import { statLabel } from "../lib/stats"
import { NumberField } from "./NumberField"
import { StatSelect } from "./StatSelect"

interface Props {
  meta: Meta
  filter: StatFilter
  exclude: number[]
  onChange(filter: StatFilter): void
  onRemove(): void
  /** The lowest value: item filters accept a malus, stuff conditions start at 0. */
  min?: number
}

export function FilterRow({ meta, filter, exclude, onChange, onRemove, min = -9999 }: Props) {
  const { t } = useI18n()
  const stat = meta.stats.find((s) => s.id === filter.statId)
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_3.4rem_4.4rem_2rem] items-center gap-1.5">
      <StatSelect
        meta={meta}
        label={t("filter.stat")}
        value={filter.statId}
        exclude={exclude}
        onChange={(statId) => onChange({ ...filter, statId })}
      />
      <Select
        value={filter.op}
        onValueChange={(op) => {
          if (op === "gte" || op === "lte") onChange({ ...filter, op })
        }}
      >
        <SelectTrigger aria-label={t("filter.op")} className="w-full min-w-0 justify-between px-2">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="gte" label="≥">
            ≥ <span className="text-ink-3">{t("op.gte")}</span>
          </SelectItem>
          <SelectItem value="lte" label="≤">
            ≤ <span className="text-ink-3">{t("op.lte")}</span>
          </SelectItem>
        </SelectContent>
      </Select>
      <NumberField
        label={t("filter.value")}
        value={filter.value}
        min={min}
        max={99999}
        onChange={(value) => onChange({ ...filter, value })}
      />
      <button
        type="button"
        className="icon-btn"
        aria-label={t("items.removeFilter", { stat: statLabel(stat) })}
        onClick={onRemove}
      >
        <XIcon aria-hidden="true" />
      </button>
    </div>
  )
}

// A sensible first threshold when a condition is added, by characteristic.
export function defaultThreshold(key: string | undefined): number {
  switch (key) {
    case "actionPoints":
    case "movementPoints":
    case "range":
    case "maxSummonedCreaturesBoost":
      return 1
    case "vitality":
      return 200
    case "criticalHit":
      return 5
    default:
      return 30
  }
}
