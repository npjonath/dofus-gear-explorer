import * as React from "react"

export const initialPreferences = {
  autoScale: true,
  scale: 75,
  light: false,
  rows: 3,
  showReach: false,
  skipExit: false,
  chatSize: 14,
  chatSpacing: 125,
  chatOpacity: 92,
  foldEffects: false,
}
export type CombatPreferences = typeof initialPreferences
const limits = { scale: [45, 135], rows: [2, 3], chatSize: [10, 22], chatSpacing: [100, 180], chatOpacity: [0, 100] } as const
const storage = "dde.fight.display.v1"
export function readPreferences(): CombatPreferences {
  const prefs = { ...initialPreferences }
  try {
    const data = JSON.parse(localStorage.getItem(storage) ?? "{}")
    for (const key of ["autoScale", "light", "showReach", "skipExit", "foldEffects"] as const) {
      if (typeof data?.[key] === "boolean") prefs[key] = data[key]
    }
    for (const key of Object.keys(limits) as (keyof typeof limits)[]) {
      if (Number.isFinite(data?.[key])) prefs[key] = Math.min(limits[key][1], Math.max(limits[key][0], Math.round(data[key])))
    }
  } catch {
    /* Use defaults when storage is unavailable. */
  }
  return prefs
}
export function useCombatPreferences() {
  const [prefs, set] = React.useState(readPreferences)
  function update(change: Partial<CombatPreferences>) {
    set((current) => ({ ...current, ...change }))
  }
  React.useEffect(() => {
    try {
      localStorage.setItem(storage, JSON.stringify(prefs))
    } catch {
      /* Session-only preferences. */
    }
  }, [prefs])
  return { prefs, update }
}
export const LightAnimations = React.createContext(false)
