import { app, ipcMain, powerMonitor, shell, type BrowserWindow } from "electron"
import { autoUpdater } from "electron-updater"
import { readFileSync, rmSync } from "node:fs"
import { join } from "node:path"
import { canReplaceBundle, installMacUpdate, prepareMacUpdate, type PreparedUpdate } from "./macUpdate"

// "downloading": the new version is on its way; "ready": it installs on
// restart; "available": the app can't replace itself and links to the release.
export interface AppUpdate {
  state: "downloading" | "available" | "ready"
  version: string
}

const checkEvery = 60 * 60 * 1000
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
//
// The app looks for a release when it starts, every hour, when its window
// comes back to the front after an hour without a check (timers stand still
// while the computer sleeps), and when the interface asks.
export function startUpdates(window: () => BrowserWindow | null) {
  let last: AppUpdate | null = null
  let prepared: PreparedUpdate | null = null
  let preparing: string | null = null
  let installing = false
  let checkedAt = 0

  const send = (update: AppUpdate | null) => {
    last = update
    window()?.webContents.send("dde:update", update)
  }
  // Offline or a broken download: nothing to install until the next check tries again.
  const downloadFailed = (version: string) => {
    if (last?.state === "downloading" && last.version === version) send(null)
  }
  // Resolves with the update found, or null when this version is the latest; rejects when the release can't be read.
  const check = async (): Promise<AppUpdate | null> => {
    const result = await autoUpdater.checkForUpdates()
    if (!result) throw new Error("this build does not update itself")
    checkedAt = Date.now()
    result.downloadPromise?.catch(() => downloadFailed(result.updateInfo.version))
    return result.isUpdateAvailable ? last : null
  }

  ipcMain.handle("dde:update:last", () => last)
  ipcMain.handle("dde:update:check", () => {
    if (!app.isPackaged) throw new Error("a development build does not update itself")
    return check()
  })
  ipcMain.on("dde:update:install", () => {
    if (last?.state === "available") {
      const page = releasesPage()
      if (page) void shell.openExternal(page)
    } else if (last?.state !== "ready") {
      return
    } else if (!mac) {
      autoUpdater.quitAndInstall()
    } else if (prepared) {
      installing = true
      installMacUpdate(prepared, true)
      app.quit()
    }
  })
  if (!app.isPackaged) return

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
      // A version that is already prepared stays installable while a newer one downloads.
      if (!prepared) send({ state: "downloading", version: info.version })
      prepareMacUpdate(info)
        .then((update) => {
          if (prepared) rmSync(prepared.dir, { recursive: true, force: true })
          prepared = update
          send({ state: "ready", version: update.version })
        })
        .catch((error: unknown) => {
          const detail = (error as { stderr?: string }).stderr?.trim()
          console.error(`update ${info.version} not installed:`, error instanceof Error ? error.message : error, detail ?? "")
          downloadFailed(info.version)
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
  } else {
    // Every check finds the release again: one that is already downloaded stays ready.
    autoUpdater.on("update-available", (info) => {
      if (last?.state === "ready" && last.version === info.version) return
      send({ state: "downloading", version: info.version })
    })
  }
  // Offline, or no release published yet: the app works the same without updates.
  autoUpdater.on("error", () => {})

  const checkNow = () => void check().catch(() => {})
  const checkIfDue = () => {
    if (Date.now() - checkedAt >= checkEvery) checkNow()
  }
  checkNow()
  setInterval(checkNow, checkEvery)
  app.on("browser-window-focus", checkIfDue)
  powerMonitor.on("resume", checkIfDue)
}
