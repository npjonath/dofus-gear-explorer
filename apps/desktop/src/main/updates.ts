import { app, ipcMain, shell, type BrowserWindow } from "electron"
import { autoUpdater } from "electron-updater"
import { readFileSync } from "node:fs"
import { join } from "node:path"

export interface AppUpdate {
  state: "available" | "ready"
  version: string
}

const checkEvery = 6 * 60 * 60 * 1000

// Squirrel.Mac only installs an update signed by the same Apple Developer ID.
// The macOS app is signed ad hoc, so there it can only say a new version exists.
const canInstall = process.platform !== "darwin"

// electron-builder writes the GitHub repository the app updates from into app-update.yml.
function releasesPage(): string | null {
  try {
    const config = readFileSync(join(process.resourcesPath, "app-update.yml"), "utf8")
    const owner = /^owner:\s*(\S+)/m.exec(config)?.[1]
    const repo = /^repo:\s*(\S+)/m.exec(config)?.[1]
    return owner && repo ? `https://github.com/${owner}/${repo}/releases/latest` : null
  } catch {
    return null
  }
}

export function startUpdates(window: () => BrowserWindow | null) {
  let last: AppUpdate | null = null
  ipcMain.handle("dde:update:last", () => last)
  ipcMain.on("dde:update:install", () => {
    if (canInstall) {
      autoUpdater.quitAndInstall()
      return
    }
    const page = releasesPage()
    if (page) void shell.openExternal(page)
  })
  if (!app.isPackaged) return

  const send = (update: AppUpdate) => {
    last = update
    window()?.webContents.send("dde:update", update)
  }
  autoUpdater.logger = null
  autoUpdater.autoDownload = canInstall
  autoUpdater.autoInstallOnAppQuit = canInstall
  autoUpdater.on("update-available", (info) => {
    if (!canInstall) send({ state: "available", version: info.version })
  })
  autoUpdater.on("update-downloaded", (info) => send({ state: "ready", version: info.version }))
  // Offline, or no release published yet: the app works the same without updates.
  autoUpdater.on("error", () => {})

  const check = () => void autoUpdater.checkForUpdates().catch(() => {})
  check()
  setInterval(check, checkEvery)
}
