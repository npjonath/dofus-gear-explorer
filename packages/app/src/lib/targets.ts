import type { AttackTarget } from "../api/types"
import type { MessageKey } from "../i18n"

const summons: AttackTarget[] = ["allySummon", "enemySummon"]
const allies: AttackTarget[] = ["ally", "allyOther", "allyPlayer", "allyMonster", "allyCompanion", "allyPlayerCompanion", "allySummon"]

/**
 * The message keys that say who a line applies to, to join with " / ". The
 * common groups get one name: everybody, everybody but the caster, the
 * summons of both sides, or everybody but the summons.
 */
export function targetKeys(targets: AttackTarget[]): MessageKey[] {
  const has = (target: AttackTarget) => targets.includes(target)
  if (targets.length === 2 && has("ally") && has("enemy")) return ["target.all"]
  if (targets.length === 2 && has("allyOther") && has("enemy")) return ["target.allButCaster"]
  if (targets.length === 2 && summons.every(has)) return ["target.summons"]
  const bothSides = targets.some((t) => allies.includes(t)) && targets.some((t) => !allies.includes(t) && t !== "caster")
  if (targets.length > 2 && bothSides && !summons.some(has) && !has("caster")) return ["target.notSummons"]
  return targets.map((target): MessageKey => `target.${target}`)
}
