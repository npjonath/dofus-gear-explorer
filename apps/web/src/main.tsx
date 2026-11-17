import { App, type Platform } from "@dde/app"
import "@dde/app/styles.css"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

const platform: Platform = {
  kind: "web",
  apiBaseUrl: import.meta.env.VITE_API_URL ?? "http://127.0.0.1:8787",
  version: __APP_VERSION__,
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App platform={platform} />
  </StrictMode>
)
