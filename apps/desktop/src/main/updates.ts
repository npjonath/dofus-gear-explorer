import { app, ipcMain, shell, type BrowserWindow } from "electron"
import { autoUpdater } from "electron-updater"
import { readFileSync, rmSync } from "node:fs"
import { join } from "node:path"
import { canReplaceBundle, installMacUpdate, prepareMacUpdate, type PreparedUpdate } from "./macUpdate"

export interface AppUpdate {
  state: "available" | "ready"
  version: string
}

const checkEvery = 6 * 60 * 60 * 1000
const mac = process.platform === "darwin"

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

// Every system downloads the new version in the background and installs it on
// restart, or when the app quits. On Windows and Linux electron-updater does
// it all; on macOS it only finds the release, and macUpdate.ts installs it.
// When the macOS bundle can't be replaced (the app runs from the disk image or
// a read-only folder), the notice links to the release page instead.
export function startUpdates(window: () => BrowserWindow | null) {
  let last: AppUpdate | null = null
  let prepared: PreparedUpdate | null = null
  let preparing: string | null = null
  let installing = false

  ipcMain.handle("dde:update:last", () => last)
  ipcMain.on("dde:update:install", () => {
    if (!mac) {
      autoUpdater.quitAndInstall()
    } else if (prepared) {
      installing = true
      installMacUpdate(prepared, true)
      app.quit()
    } else {
      const page = releasesPage()
      if (page) void shell.openExternal(page)
    }
  })
  if (!app.isPackaged) return

  const send = (update: AppUpdate) => {
    last = update
    window()?.webContents.send("dde:update", update)
  }
  autoUpdater.logger = null
  autoUpdater.autoDownload = !mac
  autoUpdater.autoInstallOnAppQuit = !mac
  autoUpdater.on("update-downloaded", (info) => send({ state: "ready", version: info.version }))
  if (mac) {
    autoUpdater.on("update-available", (info) => {
      if (prepared?.version === info.version || preparing === info.version) return
      if (!canReplaceBundle()) {
        send({ state: "available", version: info.version })
        return
      }
      preparing = info.version
      prepareMacUpdate(info)
        .then((update) => {
          if (prepared) rmSync(prepared.dir, { recursive: true, force: true })
          prepared = update
          send({ state: "ready", version: update.version })
        })
        // Offline or a broken download: try again at the next check.
        .catch((error: unknown) => {
          const detail = (error as { stderr?: string }).stderr?.trim()
          console.error(`update ${info.version} not installed:`, error instanceof Error ? error.message : error, detail ?? "")
        })
        .finally(() => {
          preparing = null
        })
    })
    app.on("will-quit", () => {
      if (prepared && !installing) {
        installing = true
        installMacUpdate(prepared, false)
      }
    })
  }
  // Offline, or no release published yet: the app works the same without updates.
  autoUpdater.on("error", () => {})

  const check = () => void autoUpdater.checkForUpdates().catch(() => {})
  check()
  setInterval(check, checkEvery)
}
