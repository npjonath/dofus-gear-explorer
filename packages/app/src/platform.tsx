import * as React from "react"
import { createApiClient, type ApiClient } from "./api/client"

export interface AppUpdate {
  state: "downloading" | "available" | "ready"
  version: string
}

// The desktop shell updates itself: "downloading" while the new version is on
// its way, "ready" once it installs on restart, "available" when the app can
// only point to the download page. `subscribe` gets null when a download
// failed and there is nothing to install any more. `check` looks for a release
// now: it resolves with the update found, or null when the app is up to date,
// and rejects when it could not look.
export interface UpdateBridge {
  last(): Promise<AppUpdate | null>
  subscribe(listener: (update: AppUpdate | null) => void): () => void
  check(): Promise<AppUpdate | null>
  install(): void
}

export interface Platform {
  kind: "web" | "desktop"
  apiBaseUrl: string
  version: string
  updates?: UpdateBridge
}

interface PlatformValue extends Platform {
  api: ApiClient
}

const PlatformContext = React.createContext<PlatformValue | null>(null)

export function PlatformProvider({ platform, children }: { platform: Platform; children: React.ReactNode }) {
  const value = React.useMemo(
    () => ({ ...platform, api: createApiClient(platform.apiBaseUrl) }),
    [platform]
  )
  return <PlatformContext.Provider value={value}>{children}</PlatformContext.Provider>
}

export function usePlatform(): PlatformValue {
  const value = React.useContext(PlatformContext)
  if (!value) throw new Error("usePlatform must be used inside PlatformProvider")
  return value
}

export function useApi(): ApiClient {
  return usePlatform().api
}
