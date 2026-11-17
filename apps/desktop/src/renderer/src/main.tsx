import { App, type Platform, type UpdateBridge } from "@dde/app"
import "@dde/app/styles.css"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

declare global {
  interface Window {
    dde: { config(): Promise<{ apiBaseUrl: string; version: string }>; updates: UpdateBridge }
  }
}

const { apiBaseUrl, version } = await window.dde.config()
const platform: Platform = { kind: "desktop", apiBaseUrl, version, updates: window.dde.updates }

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App platform={platform} />
  </StrictMode>
)
