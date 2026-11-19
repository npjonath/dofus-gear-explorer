import { app, BrowserWindow, dialog, ipcMain } from "electron"
import { spawn, type ChildProcess } from "node:child_process"
import { existsSync } from "node:fs"
import { homedir } from "node:os"
import { dirname, join, resolve } from "node:path"

function installedGame(): string | undefined {
  const home = homedir()
  const local = process.env.LOCALAPPDATA ?? join(home, "AppData", "Local")
  const roots = [
    process.env.DDE_DOFUS_DIR,
    ...["Dofus-dofus3", "Dofus", "dofus"].flatMap((name) => [
      join("/Applications", "Ankama", name),
      join(local, "Ankama", name),
      join(home, ".local", "share", "Ankama", name),
      join(home, ".config", "Ankama", name),
      join(home, "Games", "Ankama", name),
    ]),
  ]
  return roots.find(
    (root) =>
      root &&
      ["DofusContent/Content", "Dofus_Data/StreamingAssets/Content", "Content", ""].some((dir) =>
        existsSync(join(root, dir, "Characters", "Bones"))
      )
  )
}

export function registerFightAssets(dataDir: string) {
  let job: Promise<boolean> | undefined
  let child: ChildProcess | undefined
  let stage = ""
  const repository = resolve(app.getAppPath(), "../..")
  const script = join(repository, "scripts", "fight-assets", "import_assets.py")
  const binary = join(process.resourcesPath, "fight-import", process.platform === "win32" ? "fight-import.exe" : "fight-import")

  async function run(pick: boolean, owner: BrowserWindow): Promise<boolean> {
    let source = pick ? undefined : installedGame()
    if (!source) {
      const selected = await dialog.showOpenDialog(owner, { properties: ["openDirectory"] })
      if (selected.canceled || !selected.filePaths[0]) return false
      source = selected.filePaths[0]
    }
    return new Promise((done, fail) => {
      const args = ["--source", source, "--output", join(dataDir, "fight-assets")]
      const python = process.env.DDE_ASSET_PYTHON
      const command = app.isPackaged ? binary : (python ?? "uv")
      const devArgs = python
        ? [script]
        : ["run", "--isolated", "--python", "3.13", "--with-requirements", join(dirname(script), "requirements.txt"), script]
      child = spawn(command, app.isPackaged ? args : [...devArgs, ...args], {
        windowsHide: true,
        stdio: ["ignore", "pipe", "pipe"],
      })
      let diagnostic = ""
      stage = "maps"
      child.stdout?.on("data", (chunk: Buffer) => {
        for (const line of chunk.toString().trim().split(/\r?\n/)) {
          if (["maps", "hud", "actors", "effects"].includes(line)) stage = line
        }
      })
      child.stderr?.on("data", (chunk: Buffer) => {
        diagnostic = (diagnostic + chunk.toString()).slice(-3000)
      })
      child.once("error", fail)
      child.once("close", (code) => {
        child = undefined
        if (code === 0) done(true)
        else fail(new Error(diagnostic.trim() || `DOFUS import exited with code ${code}`))
      })
    })
  }

  ipcMain.handle("dde:fight-assets:stage", () => stage)
  ipcMain.handle("dde:fight-assets:import", (event, pick: unknown) => {
    const owner = BrowserWindow.fromWebContents(event.sender)
    if (!owner || event.senderFrame !== event.sender.mainFrame) throw new Error("Missing application window")
    job ??= run(pick === true, owner).finally(() => {
      job = undefined
      stage = ""
    })
    return job
  })
  app.on("will-quit", () => child?.kill())
}
