import type { Evaluation, Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { AP_STAT, HP_STAT, MP_STAT, RANGE_STAT, statLabel, toneOf } from "../../lib/stats"

const vitals = [HP_STAT, AP_STAT, MP_STAT, RANGE_STAT]

export function StatsPanel({ meta, evaluation }: { meta: Meta; evaluation: Evaluation | undefined }) {
  const { t, number } = useI18n()
  const byId = new Map(meta.stats.map((s) => [s.id, s]))
  const value = (id: number) => evaluation?.stats[String(id)] ?? 0

  return (
    <section className="grid content-start gap-4 p-4" aria-label={t("build.stats")}>
      <div className="grid grid-cols-2 gap-1.5">
        {vitals.map((id) => {
          const stat = byId.get(id)
          if (!stat) return null
          return (
            <div key={id} className="vital" data-tone={toneOf(stat.key)}>
              <span className="value">{evaluation ? number(value(id)) : "–"}</span>
              <span className="label truncate">{id === HP_STAT ? t("stat.hp") : stat.label}</span>
            </div>
          )
        })}
      </div>

      {evaluation && evaluation.capped.length > 0 && (
        <ul className="m-0 grid list-none gap-1 p-0 text-ink-2">
          {evaluation.capped.map((cap) => (
            <li key={cap.statId} className="flex items-baseline gap-2">
              <span className="pip" data-tone={toneOf(byId.get(cap.statId)?.key)} />
              {t("build.capped", {
                stat: byId.get(cap.statId)?.label ?? "",
                max: cap.max,
                lost: cap.raw - cap.max,
              })}
            </li>
          ))}
        </ul>
      )}

      {meta.categories.map((category) => {
        const stats = category.statIds
          .filter((id) => !vitals.includes(id))
          .map((id) => byId.get(id))
          .filter((s) => s !== undefined)
        if (stats.length === 0) return null
        return (
          <div key={category.id}>
            <h3 className="section-title mb-1 px-1">{category.name}</h3>
            {stats.map((stat) => {
              const v = value(stat.id)
              return (
                <div key={stat.id} className="sheet-row">
                  <span className="pip" data-tone={toneOf(stat.key)} />
                  <span className="label">{statLabel(stat)}</span>
                  <span
                    className="value"
                    data-zero={v === 0 ? "" : undefined}
                    data-negative={v < 0 ? "" : undefined}
                  >
                    {number(v)}
                    {stat.unit}
                  </span>
                </div>
              )
            })}
          </div>
        )
      })}
    </section>
  )
}
