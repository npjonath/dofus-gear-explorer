import type { ForgedLine, Stat } from "../api/types"
import { useI18n } from "../i18n"
import { toneOf } from "../lib/stats"

interface Props {
  lines: ForgedLine[]
  stats: Map<number, Stat>
}

// An item's lines after forgemagic, each tagged with what changed it.
export function ForgedLines({ lines, stats }: Props) {
  const { t } = useI18n()
  return (
    <ul className="effects">
      {lines.map((line) => (
        <li key={line.statId} data-negative={line.value < 0 ? "" : undefined}>
          <span className="pip" data-tone={toneOf(stats.get(line.statId)?.key)} />
          <span className="line-body">
            <span>{line.text}</span>
            {line.origin !== "base" && line.origin !== "transcendence" && (
              <span className="tag" data-origin={line.origin}>
                {t(`forge.origin.${line.origin}`)}
              </span>
            )}
            {line.rune > 0 && (
              <span className="tag" data-origin="transcendence">
                {line.rune === line.value
                  ? t("forge.origin.transcendence")
                  : t("forge.runeShare", { value: line.rune })}
              </span>
            )}
          </span>
        </li>
      ))}
    </ul>
  )
}
