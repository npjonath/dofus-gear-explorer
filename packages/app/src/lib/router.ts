import * as React from "react"

export type Route = "items" | "build"

const routes: Route[] = ["items", "build"]

export interface Location {
  route: Route
  params: URLSearchParams
}

function parse(hash: string): Location {
  const [path = "", query = ""] = hash.replace(/^#\/?/, "").split("?")
  const route = routes.find((r) => r === path) ?? "items"
  return { route, params: new URLSearchParams(query) }
}

function subscribe(onChange: () => void) {
  window.addEventListener("hashchange", onChange)
  return () => window.removeEventListener("hashchange", onChange)
}

// Hash routing works the same from a web server and from Electron's file:// pages.
export function useLocation(): Location {
  const hash = React.useSyncExternalStore(subscribe, () => window.location.hash)
  return React.useMemo(() => parse(hash), [hash])
}

export function hrefFor(route: Route, params?: Record<string, string>): string {
  const query = params ? "?" + new URLSearchParams(params).toString() : ""
  return `#/${route}${query}`
}

export function navigate(route: Route, params?: Record<string, string>) {
  window.location.hash = hrefFor(route, params)
}
