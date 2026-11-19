export const combatCodes = [
  "Digit1",
  "Digit2",
  "Digit3",
  "Digit4",
  "Digit5",
  "Digit6",
  "Digit7",
  "Digit8",
  "Digit9",
  "Digit0",
  "Minus",
  "Equal",
  "Backquote",
  "F1",
  "KeyC",
  "KeyS",
  "KeyI",
  "KeyH",
  "KeyY",
]
const storageKey = "dde.fight.keys.v1"

export function loadFightKeys(): string[] {
  try {
    let saved: unknown = JSON.parse(localStorage.getItem(storageKey) ?? "null")
    // Append the terrain binding to previously saved layouts without taking an occupied key.
    if (Array.isArray(saved) && saved.length === combatCodes.length - 1) {
      const occupied = new Set(saved)
      const free = ["KeyY", ...combatCodes, "KeyJ"].find((code) => !occupied.has(code))
      saved = [...saved, free]
    }
    if (
      Array.isArray(saved) &&
      saved.length === combatCodes.length &&
      new Set(saved).size === saved.length &&
      saved.every((code) => typeof code === "string" && assignable(code))
    )
      return saved
  } catch {
    /* Browser storage can be disabled. */
  }
  return [...combatCodes]
}

export function assignable(code: string): boolean {
  return /^(?:Key[A-Z]|Digit\d|F(?:[1-9]|1[0-2])|Backquote|Minus|Equal|BracketLeft|BracketRight|Semicolon|Quote|Comma|Period|Slash|Backslash|IntlBackslash|Space|Numpad\d)$/.test(
    code
  )
}

export function bindFightKey(current: string[], index: number, code: string): string[] {
  if (!assignable(code)) return current
  const next = [...current]
  const old = next.indexOf(code)
  if (old !== -1) next[old] = next[index]!
  next[index] = code
  return next
}

export function saveFightKeys(keys: string[]) {
  try {
    localStorage.setItem(storageKey, JSON.stringify(keys))
  } catch {
    /* Keep bindings in memory. */
  }
}

export function keyLabel(code: string) {
  const symbols: Record<string, string> = {
    Backquote: "²",
    Minus: "-",
    Equal: "=",
    BracketLeft: "[",
    BracketRight: "]",
    Semicolon: ";",
    Quote: "'",
    Comma: ",",
    Period: ".",
    Slash: "/",
    Backslash: "\\",
    IntlBackslash: "<",
    Space: "␣",
  }
  return symbols[code] ?? code.replace(/^(Digit|Key)/, "")
}
