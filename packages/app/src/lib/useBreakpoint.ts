import * as React from "react"

export type Breakpoint = "sm" | "lg" | "xl"

const queries = { lg: "(min-width: 64rem)", xl: "(min-width: 80rem)" }

function read(): Breakpoint {
  if (window.matchMedia(queries.xl).matches) return "xl"
  return window.matchMedia(queries.lg).matches ? "lg" : "sm"
}

function subscribe(onChange: () => void) {
  const lists = Object.values(queries).map((query) => window.matchMedia(query))
  for (const list of lists) list.addEventListener("change", onChange)
  return () => {
    for (const list of lists) list.removeEventListener("change", onChange)
  }
}

// Layout changes shape, not only size, between these widths: side panels
// become resizable columns from lg, and the third column appears at xl.
export function useBreakpoint(): Breakpoint {
  return React.useSyncExternalStore(subscribe, read)
}
