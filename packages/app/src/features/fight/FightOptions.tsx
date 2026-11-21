import * as React from "react"
import { useI18n } from "../../i18n"
import { assignable, bindFightKey, combatCodes, keyLabel, saveFightKeys } from "../../lib/fightKeys"
import { usePlatform } from "../../platform"

import { defaultLevels } from "../../lib/fightAudio"
import type { FightAudio, VolumeKey } from "./useFightAudio"
import { initialPreferences, type CombatPreferences } from "./combatPreferences"

export function FightOptions({
  prefs,
  update,
  audio,
  maps,
  map,
  onMap,
  keys,
  onKeys,
  stage,
  onImported,
}: {
  prefs: CombatPreferences
  update: (change: Partial<CombatPreferences>) => void
  audio: FightAudio
  maps: number[]
  map: number
  onMap: (id: number) => void
  keys: string[]
  onKeys: (keys: string[]) => void
  stage: string
  onImported: () => Promise<unknown>
}) {
  const { t } = useI18n()
  const { fightAssets } = usePlatform()
  const [tab, setTab] = React.useState<"general" | "keys">("general")
  const [search, setSearch] = React.useState("")
  const previousVolumes = React.useRef({ ...defaultLevels })
  const norm = (value: string) => value.toLocaleLowerCase().normalize("NFD").replace(/\p{M}/gu, "")
  const matches = (...labels: string[]) => labels.some((label) => norm(label).includes(norm(search.trim())))
  const channels = Object.keys(defaultLevels) as VolumeKey[]
  const [binding, setBinding] = React.useState<number | null>(null)
  const [requesting, setRequesting] = React.useState(false)
  const [failed, setFailed] = React.useState("")
  const running = requesting || Boolean(stage)
  const importLabel = t(maps.length ? "fight.reimport" : "fight.import")
  function change(next: string[]) {
    onKeys(next)
    saveFightKeys(next)
  }
  async function importArt(pick: boolean) {
    if (!fightAssets || running) return
    setRequesting(true)
    setFailed("")
    try {
      if (await fightAssets.import(pick)) await onImported()
    } catch (error) {
      setFailed(error instanceof Error ? error.message : String(error))
    } finally {
      setRequesting(false)
    }
  }
  const commands = [
    t("fight.weapon"),
    t("fight.keys.turn"),
    t("build.stats"),
    t("fight.spells"),
    t("build.view.gear"),
    t("fight.settings"),
    t("fight.opt.marks"),
  ]
  const sections = {
    scale: matches(t("fight.opt.scale"), t("fight.opt.auto"), t("fight.interface")),
    quality: matches(t("fight.opt.quality"), t("fight.interface")),
    rows: matches(t("fight.opt.rows")),
    showReach: matches(t("fight.opt.reach")),
    skipExit: matches(t("fight.opt.exit")),
    sound: matches(t("fight.opt.sound"), t("fight.opt.poutch"), ...channels.map((key) => t(`fight.opt.${key}`))),
    arena: matches(t("fight.arena"), t("fight.import"), importLabel),
  }
  const keyName = (i: number) => (i < 12 ? t("fight.keyslot", { n: i + 1 }) : (commands[i - 12] ?? ""))
  const visibleKeys = keys.map((code, i) => matches(keyName(i), keyLabel(code)))
  const hasMatches = tab === "general" ? Object.values(sections).some(Boolean) : visibleKeys.some(Boolean)
  return (
    <div className="combat-settings">
      <nav className="combat-setting-tabs">
        <button
          aria-pressed={tab === "general"}
          onClick={() => {
            setTab("general")
            setSearch("")
          }}
        >
          {t("fight.opt.general")}
        </button>
        <button
          aria-pressed={tab === "keys"}
          onClick={() => {
            setTab("keys")
            setSearch("")
          }}
        >
          {t("fight.keys")}
        </button>
      </nav>
      <input
        type="search"
        aria-label={t("fight.opt.search")}
        placeholder={t("fight.opt.search")}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      {!hasMatches && <p role="status">{t("fight.opt.empty")}</p>}
      {hasMatches && tab === "general" && (
        <>
          {sections.scale && (
            <section>
              <h3>{t("fight.opt.scale")}</h3>
              <label className="setting-line">
                {t("fight.opt.scale")}{" "}
                <input
                  type="range"
                  min="45"
                  max="135"
                  step="5"
                  value={prefs.scale}
                  onChange={(e) => update({ scale: e.target.valueAsNumber, autoScale: false })}
                />
                <output>{prefs.autoScale ? "Auto" : prefs.scale + " %"}</output>
              </label>
              <label>
                <input type="checkbox" checked={prefs.autoScale} onChange={(e) => update({ autoScale: e.target.checked })} />{" "}
                {t("fight.opt.auto")}
              </label>
            </section>
          )}
          {sections.quality && (
            <section>
              <h3>{t("fight.opt.quality")}</h3>
              {([false, true] as const).map((light) => (
                <button key={String(light)} aria-pressed={prefs.light === light} onClick={() => update({ light })}>
                  {t(light ? "fight.opt.light" : "fight.opt.normal")}
                </button>
              ))}
            </section>
          )}
          {sections.rows && (
            <section>
              <h3>{t("fight.opt.rows")}</h3>
              {[2, 3].map((rows) => (
                <button key={rows} aria-pressed={prefs.rows === rows} onClick={() => update({ rows })}>
                  {rows}
                </button>
              ))}
            </section>
          )}
          {(["showReach", "skipExit"] as const).map((key) => {
            const label = t(key === "showReach" ? "fight.opt.reach" : "fight.opt.exit")
            return (
              sections[key] && (
                <section key={key}>
                  <label>
                    <input type="checkbox" checked={prefs[key]} onChange={(e) => update({ [key]: e.target.checked })} /> {label}
                  </label>
                </section>
              )
            )
          })}
          {sections.sound && (
            <section>
              <h3>{t("fight.opt.sound")}</h3>
              {!audio.available.length && <p>{t("fight.opt.importSound")}</p>}
              {channels.map((key) => (
                <div className="setting-volume" key={key}>
                  <label>
                    {t(`fight.opt.${key}`)}{" "}
                    <input
                      type="range"
                      min={0}
                      max={100}
                      value={audio.levels[key]}
                      onChange={(e) => audio.setLevel(key, e.target.valueAsNumber)}
                    />
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    aria-label={t(`fight.opt.${key}`)}
                    value={audio.levels[key]}
                    onChange={(e) => audio.setLevel(key, e.target.valueAsNumber)}
                  />
                  <button
                    onClick={() => {
                      if (audio.levels[key]) {
                        previousVolumes.current[key] = audio.levels[key]
                        audio.setLevel(key, 0)
                      } else audio.setLevel(key, previousVolumes.current[key] || defaultLevels[key])
                    }}
                  >
                    {t(audio.levels[key] ? "fight.opt.mute" : "fight.opt.unmute")}
                  </button>
                </div>
              ))}
              <label>
                <input type="checkbox" checked={audio.poutchHit} onChange={(e) => audio.setPoutchHit(e.target.checked)} />{" "}
                {t("fight.opt.poutch")}
              </label>
            </section>
          )}
          {sections.arena && (
            <section>
              <h3>{t("fight.arena")}</h3>
              {maps.length > 0 && !running && <p role="status">{t("fight.assetsReady")}</p>}
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
                      {importLabel}
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
                            : stage === "audio"
                              ? t("fight.opt.sound")
                              : stage === "effects"
                                ? t("fight.opt.effects")
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
          )}
          <button
            onClick={() => {
              update(initialPreferences)
              for (const key of channels) audio.setLevel(key, defaultLevels[key])
              audio.setPoutchHit(false)
            }}
          >
            {t("fight.opt.reset")}
          </button>
        </>
      )}
      {hasMatches && tab === "keys" && (
        <section>
          <h3>{t("fight.keys")}</h3>
          <p>{t("fight.keys.help")}</p>
          <div className="combat-keygrid">
            {keys.map(
              (code, i) =>
                visibleKeys[i] && (
                  <label key={i}>
                    {keyName(i)}
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
                )
            )}
          </div>
          <button type="button" onClick={() => change([...combatCodes])}>
            {t("fight.resetKeys")}
          </button>
        </section>
      )}
    </div>
  )
}
