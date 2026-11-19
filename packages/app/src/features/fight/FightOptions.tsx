import * as React from "react"
import { useI18n } from "../../i18n"
import { assignable, bindFightKey, combatCodes, keyLabel, saveFightKeys } from "../../lib/fightKeys"
import { usePlatform } from "../../platform"

export function FightOptions({
  maps,
  map,
  onMap,
  keys,
  onKeys,
  onImported,
}: {
  maps: number[]
  map: number
  onMap: (id: number) => void
  keys: string[]
  onKeys: (keys: string[]) => void
  onImported: () => Promise<unknown>
}) {
  const { t } = useI18n()
  const { fightAssets } = usePlatform()
  const [binding, setBinding] = React.useState<number | null>(null)
  const [running, setRunning] = React.useState(false)
  const [failed, setFailed] = React.useState("")
  const [stage, setStage] = React.useState("")
  React.useEffect(() => {
    if (!running || !fightAssets) return
    const id = window.setInterval(() => {
      void fightAssets
        .stage()
        .then(setStage)
        .catch(() => {})
    }, 700)
    return () => window.clearInterval(id)
  }, [running, fightAssets])
  function change(next: string[]) {
    onKeys(next)
    saveFightKeys(next)
  }
  async function importArt(pick: boolean) {
    if (!fightAssets || running) return
    setRunning(true)
    setFailed("")
    setStage("")
    try {
      if (await fightAssets.import(pick)) await onImported()
    } catch (error) {
      setFailed(error instanceof Error ? error.message : String(error))
    } finally {
      setRunning(false)
    }
  }
  const commands = [
    t("fight.weapon"),
    t("fight.keys.turn"),
    t("build.stats"),
    t("fight.spells"),
    t("build.view.gear"),
    t("fight.settings"),
  ]
  return (
    <div className="combat-settings">
      <section>
        <h3>{t("fight.arena")}</h3>
        {maps.length > 0 && (
          <div className="combat-map-picker">
            {maps.map((id) => (
              <button type="button" key={id} aria-pressed={map === id} onClick={() => onMap(id)}>
                {t(id === 99222029 ? "fight.village" : "fight.astrub")}
              </button>
            ))}
          </div>
        )}
        {fightAssets ? (
          <>
            <div className="combat-import">
              <button type="button" disabled={running} onClick={() => void importArt(false)}>
                {t("fight.import")}
              </button>
              <button type="button" disabled={running} onClick={() => void importArt(true)}>
                {t("fight.directory")}
              </button>
            </div>
            {running && (
              <p role="status">
                {t("fight.importing")}{" "}
                {stage === "maps"
                  ? t("fight.arena")
                  : stage === "actors"
                    ? t("fight.actors")
                    : stage === "hud"
                      ? t("fight.interface")
                      : ""}
              </p>
            )}
            {failed && (
              <p role="alert">
                {t("fight.importFailed")} <small>{failed}</small>
              </p>
            )}
          </>
        ) : (
          maps.length === 0 && <p>{t("fight.webAssets")}</p>
        )}
      </section>
      <section>
        <h3>{t("fight.keys")}</h3>
        <p>{t("fight.keys.help")}</p>
        <div className="combat-keygrid">
          {keys.map((code, i) => (
            <label key={i}>
              {i < 12 ? t("fight.keyslot", { n: i + 1 }) : commands[i - 12]}
              <button
                type="button"
                aria-pressed={binding === i}
                onBlur={() => setBinding(null)}
                onClick={() => setBinding(i)}
                onKeyDown={(event) => {
                  if (binding !== i) return
                  event.stopPropagation()
                  if (event.code === "Escape") {
                    event.preventDefault()
                    setBinding(null)
                    return
                  }
                  if (!assignable(event.code) || event.altKey || event.metaKey || event.ctrlKey) return
                  event.preventDefault()
                  change(bindFightKey(keys, i, event.code))
                  setBinding(null)
                }}
              >
                {binding === i ? "…" : keyLabel(code)}
              </button>
            </label>
          ))}
        </div>
        <button type="button" onClick={() => change([...combatCodes])}>
          {t("fight.resetKeys")}
        </button>
      </section>
    </div>
  )
}
