import { Button } from "@ui-sdk/react/button"
import { XIcon } from "lucide-react"
import { useItem } from "../../api/queries"
import type { Item, Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { statIndex } from "../../lib/stats"
import { useAddToBuild } from "../../lib/useAddToBuild"
import { Effects } from "../../ui/Effects"
import { ItemCell } from "../../ui/ItemCell"
import { ItemHover } from "../../ui/ItemHover"

export function ItemInspector({ meta, item, onClose }: { meta: Meta; item: Item; onClose(): void }) {
  const { t } = useI18n()
  const detail = useItem(item.id)
  const addToBuild = useAddToBuild(meta)
  const stats = statIndex(meta)
  const full = detail.data ?? item
  const set = detail.data?.setDetail

  return (
    <aside className="flex h-full flex-col bg-panel" aria-label={full.name}>
      <header className="flex items-start gap-3 border-b border-line p-4">
        <ItemCell item={full} size="4rem" />
        <div className="min-w-0 flex-1">
          <h2 className="m-0 text-[1.15rem] leading-tight font-bold">{full.name}</h2>
          <p className="m-0 mt-1 text-ink-2">
            {full.typeName} · {t("item.level", { level: full.level })}
          </p>
        </div>
        <button type="button" className="icon-btn" aria-label={t("item.close")} onClick={onClose}>
          <XIcon aria-hidden="true" />
        </button>
      </header>

      <div className="grid flex-1 content-start gap-5 overflow-y-auto p-4">
        <section className="grid gap-2">
          <h3 className="section-title">{t("item.effects")}</h3>
          <Effects effects={full.effects} stats={stats} />
        </section>

        {full.passive && (
          <section className="grid gap-1">
            <h3 className="section-title">
              {t("item.passive")} : {full.passive.name}
            </h3>
            <p className="m-0 max-w-[65ch]">{full.passive.description}</p>
          </section>
        )}

        {full.weapon && (
          <section className="grid gap-1">
            <h3 className="section-title">{t("item.weapon")}</h3>
            <p className="m-0 text-ink-2">
              {[
                t("item.apCost", { n: full.weapon.apCost }),
                t("item.range", { min: full.weapon.rangeMin, max: full.weapon.rangeMax }),
                t("item.crit", { chance: full.weapon.critChance, bonus: full.weapon.critBonus }),
                t("item.casts", { n: full.weapon.maxCastPerTurn }),
              ].join(" · ")}
            </p>
          </section>
        )}

        {full.condition && (
          <section className="grid gap-1">
            <h3 className="section-title">{t("item.conditions")}</h3>
            <p className="m-0">{full.condition}</p>
          </section>
        )}

        {full.set && (
          <section className="grid gap-2">
            <h3 className="section-title">
              {t("item.set")} : {full.set.name}
            </h3>
            {set ? (
              <>
                <div className="flex flex-wrap gap-1.5">
                  {set.items.map((piece) => (
                    <ItemHover
                      key={piece.id}
                      item={piece}
                      stats={stats}
                      actions={[{ label: t("item.add"), primary: true, run: () => void addToBuild(piece) }]}
                    >
                      <span role="img" aria-label={piece.name} tabIndex={0} className="rounded-md">
                        <ItemCell item={piece} size="2.5rem" />
                      </span>
                    </ItemHover>
                  ))}
                </div>
                <dl className="m-0 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5">
                  {set.bonuses
                    .filter((bonus) => bonus.lines.length > 0)
                    .map((bonus) => (
                      <div key={bonus.count} className="contents">
                        <dt className="font-semibold whitespace-nowrap text-ink-2">
                          {t("item.setBonus", { count: bonus.count })}
                        </dt>
                        <dd className="m-0">{bonus.lines.map((line) => line.text).join(" · ")}</dd>
                      </div>
                    ))}
                </dl>
              </>
            ) : (
              <div className="skeleton h-16" />
            )}
          </section>
        )}

        {detail.data?.description && <p className="m-0 max-w-[65ch] text-ink-2">{detail.data.description}</p>}
      </div>

      <footer className="border-t border-line p-4">
        <Button
          data-variant="solid"
          className="w-full"
          onClick={() => void addToBuild(item)}
        >
          {t("item.add")}
        </Button>
      </footer>
    </aside>
  )
}
