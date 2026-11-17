import * as React from "react"
import { createApiClient, type ApiClient } from "./api/client"

export interface AppUpdate {
  state: "available" | "ready"
  version: string
}

// The desktop shell updates itself; "available" means it can only point to the
// download page, "ready" that the new version installs on restart.
export interface UpdateBridge {
  last(): Promise<AppUpdate | null>
  subscribe(listener: (update: AppUpdate) => void): () => void
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
