import { useQuery } from "@tanstack/react-query"
import { CheckIcon, LockKeyholeIcon, SearchIcon } from "lucide-react"
import * as React from "react"
import type { FightBookSpell, FightSlot } from "../../api/types"
import { useI18n } from "../../i18n"
import { useApi } from "../../platform"

const elements = ["neutral", "earth", "fire", "water", "air", "variable"] as const
const roles = ["damage", "heal", "bonus", "penalty", "protection", "placement", "summon", "special"] as const
const tags = [...roles.slice(0, 2), ...elements, ...roles.slice(2)]
const normalize = (value: string) => value.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase()

interface Props {
  breedId: number
  breedName: string
  level: number
  slots: FightSlot[]
  selected: Set<number>
  onSelect(slot: FightSlot): void
  sheet(slot: FightSlot): React.ReactNode
  pack?: string
}

export function FightSpellBook({ breedId, breedName, level, slots, selected, onSelect, sheet, pack }: Props) {
  const api = useApi()
  const { lang, t } = useI18n()
  const book = useQuery({
    queryKey: ["fight-book", api.baseUrl, lang, breedId, pack],
    queryFn: ({ signal }) => api.fightBook(lang, breedId, signal),
    staleTime: 60_000,
  })
  const [common, setCommon] = React.useState(!breedId)
  const [query, setQuery] = React.useState("")
  const [filters, setFilters] = React.useState<string[]>([])
  const [showLevels, setShowLevels] = React.useState(true)
  const [detail, setDetail] = React.useState<number | null>(null)
  const byId = new Map(slots.map((slot) => [slot.id, slot]))
  const pairs = new Map<number, FightBookSpell[]>()
  for (const spell of book.data ?? []) {
    if ((spell.breedId === 0) !== common) continue
    const id = spell.variantOf || spell.id
    pairs.set(id, [...(pairs.get(id) ?? []), spell])
  }
  const term = normalize(query.trim())
  const matches = (spell: FightBookSpell) =>
    normalize(spell.name).includes(term) &&
    [elements, roles].every((group) => {
      const wanted = group.filter((tag) => filters.includes(tag))
      return wanted.length === 0 || wanted.some((tag) => spell.tags.includes(tag))
    })
  const visiblePairs = [...pairs].filter(([, pair]) => pair.some(matches))
  const visibleSpells = visiblePairs.flatMap(([, pair]) => pair)
  const focus = visibleSpells.find((spell) => spell.id === detail) ?? visibleSpells.find(matches)
  const slot = focus && byId.get(focus.id)
  const error = book.isError
  const lockedLabel = (spell: FightBookSpell) => t("fight.book.locked", { n: spell.levels[0] ?? 0 })

  function switchGroup(next: boolean) {
    setCommon(next)
    setDetail(null)
  }

  return (
    <div className="fight-book-window">
      <nav aria-label={t("fight.spells")}>
        <button aria-pressed={!common} disabled={!breedId} onClick={() => switchGroup(false)}>
          {breedName}
        </button>
        <button aria-pressed={common} onClick={() => switchGroup(true)}>
          {t("fight.book.others")}
        </button>
      </nav>
      <div className="fight-book-controls">
        <label className="fight-book-search">
          <SearchIcon aria-hidden="true" />
          <input
            type="search"
            aria-label={t("fight.book.search")}
            placeholder={t("fight.book.search")}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value)
              setDetail(null)
            }}
          />
        </label>
        <div className="fight-book-filters">
          {tags.map((tag) => (
            <button
              key={tag}
              aria-pressed={filters.includes(tag)}
              onClick={() => {
                setFilters((list) => (list.includes(tag) ? list.filter((value) => value !== tag) : [...list, tag]))
                setDetail(null)
              }}
            >
              {t(`fight.book.${tag}`)}
            </button>
          ))}
        </div>
      </div>
      {error && (
        <p role="alert">
          {t("fight.failed")} <button onClick={() => void book.refetch()}>{t("fight.restart")}</button>
        </p>
      )}
      {book.isPending && <p role="status">{t("fight.loading")}</p>}
      {book.isSuccess && !visiblePairs.length && <p role="status">{t("fight.book.empty")}</p>}
      {visiblePairs.length > 0 && (
        <div className="fight-book-body">
          <div className="fight-book-pairs">
            {visiblePairs.map(([id, pair]) => (
              <div key={id} className="fight-book-pair">
                {pair.map((spell) => {
                  const usable = byId.get(spell.id)
                  const equipped = selected.has(spell.id)
                  return (
                    <button
                      key={spell.id}
                      className="fight-book-choice"
                      data-locked={!usable || undefined}
                      aria-label={usable ? spell.name : `${spell.name} · ${lockedLabel(spell)}`}
                      aria-pressed={equipped}
                      title={spell.name}
                      onClick={() => {
                        setDetail(spell.id)
                        if (usable) onSelect(usable)
                      }}
                      onFocus={() => setDetail(spell.id)}
                      onMouseEnter={() => setDetail(spell.id)}
                    >
                      {showLevels && (
                        <span className="fight-book-levels" aria-hidden="true">
                          {spell.levels.map((rank) => (
                            <small key={rank} data-future={rank > level || undefined}>
                              {rank}
                            </small>
                          ))}
                        </span>
                      )}
                      <img src={api.imageUrl(`/images/spells/${spell.iconId}.png`)} alt="" loading="lazy" />
                      <span>{spell.name}</span>
                      {!usable ? <LockKeyholeIcon aria-hidden="true" /> : equipped && <CheckIcon aria-hidden="true" />}
                    </button>
                  )
                })}
              </div>
            ))}
          </div>
          {focus && (
            <section className="fight-book-detail" aria-label={focus.name} key={focus.id}>
              {slot ? (
                sheet(slot)
              ) : (
                <strong>
                  {focus.name} · {lockedLabel(focus)}
                </strong>
              )}
              {focus.description && <p>{focus.description}</p>}
            </section>
          )}
        </div>
      )}
      <label className="fight-book-check">
        <input type="checkbox" checked={showLevels} onChange={(event) => setShowLevels(event.target.checked)} />
        {t("fight.book.levels")}
      </label>
    </div>
  )
}
