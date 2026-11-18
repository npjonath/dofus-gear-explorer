import { SegmentedControl, SegmentedControlIndicator, SegmentedControlItem } from "@ui-sdk/react/segmented-control"
import { Switch, SwitchThumb } from "@ui-sdk/react/switch"
import * as React from "react"
import { useDamage } from "../../api/queries"
import type { Attack, AttackLine, AttackRange, BuildInput, Hit, Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { targetKeys } from "../../lib/targets"
import type { Build } from "../../store"

export const elementKeys = ["neutral", "earth", "fire", "water", "air"] as const

function range(lo: number, hi: number, format: (n: number) => string): string {
  return lo === hi ? format(lo) : `${format(lo)} – ${format(hi)}`
}

function HitCells({ hit, canCrit }: { hit: Hit; canCrit: boolean }) {
  const { number } = useI18n()
  return (
    <>
      <span className="damage-value">{range(hit.min, hit.max, number)}</span>
      <span className="damage-value damage-crit">{canCrit ? range(hit.critMin, hit.critMax, number) : "–"}</span>
    </>
  )
}

// What a line does, then who it applies to and when.
function LineLabel({ line }: { line: AttackLine }) {
  const { t } = useI18n()
  const element = elementKeys[line.element] ?? "neutral"
  const name =
    line.kind === "push"
      ? t("damage.push", { count: line.cells ?? 0 })
      : line.kind === "healPercent"
        ? t("damage.healPercent")
        : t(line.kind === "heal" ? "damage.heal" : line.steal ? "damage.steal" : "damage.hit", { element: t(`element.${element}`) })
  const damage = !line.kind
  const notes = [
    targetKeys(line.targets)
      .map((key) => t(key))
      .join(" / "),
    line.best ? t("damage.best") : "",
    line.worst ? t("damage.worst") : "",
    line.conditional ? t("damage.conditional") : "",
    line.trigger ? t(`damage.trigger.${line.trigger}`) : "",
    line.delay > 0 ? t("damage.delay", { count: line.delay }) : "",
    damage && !line.counted && !line.conditional && line.delay === 0 ? t("damage.notCounted") : "",
  ].filter(Boolean)
  return (
    <span className="grid min-w-0">
      <span className="flex min-w-0 items-center gap-2">
        {line.kind === "healPercent" || line.kind === "push" ? null : <span className="pip" data-tone={element} />}
        <span className="truncate">{name}</span>
      </span>
      {notes.length > 0 && <span className="truncate text-[0.86rem] text-ink-3">{notes.join(" · ")}</span>}
    </span>
  )
}

// One weapon or spell: its cost and reach, then each line and their total.
function AttackRow({ attack, variant }: { attack: Attack; variant: boolean }) {
  const { t, number } = useI18n()
  const canCrit = attack.critChance > 0 && attack.lines.some((line) => line.critMax > 0)
  const reach =
    attack.rangeMin === attack.rangeMax ? String(attack.rangeMax) : `${attack.rangeMin} – ${attack.rangeMax}`
  const counted = attack.lines.filter((line) => line.counted)
  return (
    <li className="damage-attack">
      <div className="damage-head">
        <span className="min-w-0">
          <span className="block truncate font-semibold">{attack.name}</span>
          <span className="block truncate text-[0.86rem] text-ink-3">
            {[
              variant ? t("damage.variant") : null,
              t("item.apCost", { n: attack.apCost }),
              t("damage.reach", { reach }),
              canCrit ? t("damage.critChance", { chance: attack.critRate }) : null,
              attack.melee ? t("damage.melee") : t("damage.ranged"),
            ]
              .filter(Boolean)
              .join(" · ")}
          </span>
        </span>
      </div>
      {attack.lines.map((line, index) => (
        <React.Fragment key={index}>
          <div className="damage-line">
            <LineLabel line={line} />
            <HitCells hit={line} canCrit={canCrit && line.critMax > 0} />
          </div>
          {line.regen && (
            <div className="damage-line damage-sub">
              <span className="truncate">{t("damage.regen")}</span>
              <HitCells hit={line.regen} canCrit={canCrit && line.critMax > 0} />
            </div>
          )}
          {line.steps?.map((step, used) => (
            <div key={used} className="damage-line damage-sub">
              <span className="truncate">{t("damage.mpUsed", { count: used + 1 })}</span>
              <HitCells hit={step} canCrit={canCrit && line.critMax > 0} />
            </div>
          ))}
        </React.Fragment>
      ))}
      {counted.length > 1 && (
        <div className="damage-line damage-total">
          <span>{t("damage.total")}</span>
          <HitCells hit={attack.total} canCrit={canCrit} />
        </div>
      )}
      {counted.length > 0 && (
        <div className="damage-mean">
          <span>{canCrit ? t("damage.meanAt", { chance: attack.critRate }) : t("damage.mean")}</span>
          <strong>{number(attack.mean)}</strong>
        </div>
      )}
    </li>
  )
}

function AttackList({ title, attacks, empty }: { title: string; attacks: Attack[]; empty?: string }) {
  const { t } = useI18n()
  return (
    <section className="grid gap-2">
      <div className="damage-line damage-columns">
        <h3 className="section-title">{title}</h3>
        <span className="damage-value">{t("damage.normal")}</span>
        <span className="damage-value">{t("damage.critical")}</span>
      </div>
      {attacks.length > 0 ? (
        <ul className="m-0 grid list-none gap-2 p-0">
          {attacks.map((attack) => (
            <AttackRow key={attack.id} attack={attack} variant={attack.variantOf > 0} />
          ))}
        </ul>
      ) : (
        <p className="m-0 text-ink-2">{empty}</p>
      )}
    </section>
  )
}

// What the stuff's weapon and the spells of its class hit for.
export function DamagePanel({ meta, build, input }: { meta: Meta; build: Build; input: BuildInput }) {
  const { t } = useI18n()
  const [mode, setMode] = React.useState<AttackRange>("")
  const [mastery, setMastery] = React.useState(true)
  const damage = useDamage(input, build.breedId, mode, mastery).data
  const breed = meta.breeds.find((b) => b.id === build.breedId)
  const classSpells = damage?.spells.filter((spell) => spell.breedId !== 0) ?? []
  const commonSpells = damage?.spells.filter((spell) => spell.breedId === 0) ?? []

  return (
    <div className="damage-panel">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="m-0 max-w-[60ch] text-ink-2">{t("damage.hint")}</p>
        <SegmentedControl
          aria-label={t("damage.rangeLabel")}
          value={mode === "" ? "auto" : mode}
          onValueChange={(next) => setMode(next === "melee" || next === "ranged" ? next : "")}
        >
          <SegmentedControlIndicator />
          <SegmentedControlItem value="auto">{t("damage.auto")}</SegmentedControlItem>
          <SegmentedControlItem value="melee">{t("damage.melee")}</SegmentedControlItem>
          <SegmentedControlItem value="ranged">{t("damage.ranged")}</SegmentedControlItem>
        </SegmentedControl>
      </div>

      {!damage ? (
        <div className="skeleton h-40" />
      ) : (
        <>
          <label className="flex cursor-pointer items-center gap-2.5">
            <Switch checked={mastery} onCheckedChange={setMastery}>
              <SwitchThumb />
            </Switch>
            <span>
              <span className="font-semibold">{t("damage.mastery")}</span>
              <span className="text-ink-2"> · {t("damage.masteryHint")}</span>
            </span>
          </label>
          <AttackList
            title={t("damage.weapon")}
            attacks={damage.weapon ? [damage.weapon] : []}
            empty={t("damage.noWeapon")}
          />
          <AttackList
            title={breed ? t("damage.spellsOf", { breed: breed.name }) : t("damage.spells")}
            attacks={classSpells}
            empty={breed ? t("damage.noSpells") : t("damage.pickClass")}
          />
          <AttackList title={t("damage.common")} attacks={commonSpells} empty={t("damage.noSpells")} />
        </>
      )}
    </div>
  )
}
