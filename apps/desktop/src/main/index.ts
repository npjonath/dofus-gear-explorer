import { app, BrowserWindow, dialog, ipcMain, shell } from "electron"
import { spawn, type ChildProcess } from "node:child_process"
import { existsSync, renameSync } from "node:fs"
import { join, resolve } from "node:path"
import { createInterface } from "node:readline"
import { registerFightAssets } from "./fightAssets"
import { startUpdates } from "./updates"

const appName = "Dofus Gear Explorer"
app.setName(appName)
// Two apps sharing one folder cannot both open its storage: the second one would
// silently forget the stuffs and settings it saves. Development runs get their own.
const profile = app.commandLine.getSwitchValue("user-data-dir")
const userData = profile ? resolve(profile) : join(app.getPath("appData"), app.isPackaged ? appName : `${appName} Dev`)

// The app was called Dofus Database Explorer before 0.1.0: keep its stuffs and game data.
const legacyData = join(app.getPath("appData"), "Dofus Database Explorer")
if (!profile && app.isPackaged && existsSync(legacyData) && !existsSync(userData)) {
  try {
    renameSync(legacyData, userData)
  } catch {
    // A fresh folder works too: the data downloads again and the stuffs start empty.
  }
}
app.setPath("userData", userData)

// For the same reason, opening the app again brings back the window already open.
const primary = app.requestSingleInstanceLock()
if (!primary) app.exit(0)
app.on("second-instance", () => {
  const window = BrowserWindow.getAllWindows()[0]
  if (!window) return
  if (window.isMinimized()) window.restore()
  window.focus()
})

// Ctrl-C in a terminal and the development watcher end the app with a signal; quitting
// properly lets Chromium write the storage to disk first.
for (const signal of ["SIGINT", "SIGTERM"] as const) process.on(signal, () => app.quit())

app.setAboutPanelOptions({
  applicationName: appName,
  applicationVersion: app.getVersion(),
  version: "",
  copyright: "Unofficial tool, not affiliated with Ankama. Dofus is a trademark of Ankama.",
})

const binaryName = process.platform === "win32" ? "dde-api.exe" : "dde-api"
let api: ChildProcess | null = null
let apiBaseUrl = ""
// Development runs load the interface from electron-vite's server instead of the bundled files.
const devUrl = app.isPackaged ? undefined : process.env.ELECTRON_RENDERER_URL

function apiBinary(): string {
  if (app.isPackaged) return join(process.resourcesPath, "bin", binaryName)
  return resolve(app.getAppPath(), "..", "api", "bin", binaryName)
}

// Starts the Go API on a free local port and resolves with its URL, which
// the API prints on stdout once it is listening.
function startApi(): Promise<string> {
  const external = process.env.DDE_API_URL
  if (external) return Promise.resolve(external)

  const binary = apiBinary()
  if (!existsSync(binary)) {
    return Promise.reject(new Error(`API binary not found at ${binary}. Run "pnpm --filter @dde/api build" first.`))
  }
  const dataDir = process.env.DDE_DATA_DIR ?? join(app.getPath("userData"), "data")
  return new Promise((resolveUrl, reject) => {
    const child = spawn(
      binary,
      [
        "serve",
        "--addr",
        "127.0.0.1:0",
        "--data-dir",
        dataDir,
        "--exit-with-stdin",
        // The bundled file:// pages send no Origin; only the dev server's pages need allowing.
        ...(devUrl ? ["--cors-origin", new URL(devUrl).origin] : []),
      ],
      // The API watches its stdin and stops when this process goes away, however it ends.
      { stdio: ["pipe", "pipe", "inherit"], windowsHide: true }
    )
    api = child
    const lines = createInterface({ input: child.stdout! })
    lines.on("line", (line) => {
      const match = /^DDE_API_LISTENING (\S+)/.exec(line)
      if (match) resolveUrl(match[1]!)
    })
    child.once("error", reject)
    child.once("exit", (code) => {
      api = null
      reject(new Error(`The API stopped unexpectedly (exit code ${code}).`))
    })
  })
}

function createWindow() {
  const window = new BrowserWindow({
    width: 1440,
    height: 900,
    minWidth: 960,
    minHeight: 640,
    show: false,
    title: appName,
    backgroundColor: "#eef0f4",
    webPreferences: {
      preload: join(__dirname, "../preload/index.js"),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      // On Windows and Linux the spellchecker downloads dictionaries from Google; the app only goes online for game data and updates.
      spellcheck: false,
    },
  })
  window.once("ready-to-show", () => window.show())

  // Links leave the app; the window only ever shows the bundled interface.
  window.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith("https://")) void shell.openExternal(url)
    return { action: "deny" }
  })

  if (devUrl) void window.loadURL(devUrl)
  else void window.loadFile(join(__dirname, "../renderer/index.html"))
}

ipcMain.handle("dde:config", () => ({ apiBaseUrl, version: app.getVersion() }))

app.whenReady().then(async () => {
  if (!primary) return
  try {
    apiBaseUrl = await startApi()
  } catch (error) {
    dialog.showErrorBox(appName, error instanceof Error ? error.message : String(error))
    app.quit()
    return
  }
  registerFightAssets(process.env.DDE_DATA_DIR ?? join(app.getPath("userData"), "data"))
  createWindow()
  startUpdates(() => BrowserWindow.getAllWindows()[0] ?? null)
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit()
})

app.on("will-quit", () => {
  api?.removeAllListeners("exit")
  api?.kill()
})
