// End-to-end test of the app updating itself, on the system it runs on.
//
// It packages the app twice, as 1.0.0 and 1.0.1, both pointing at an update
// feed served from this machine. It installs and starts 1.0.0, waits until the
// app has downloaded 1.0.1 and offers to restart, presses restart, and checks
// that 1.0.1 is what is installed afterwards.
//
// Run it from apps/desktop after `pnpm --filter @dde/api build:targets <os>-<arch>`
// and `electron-vite build`: node scripts/update-test.mjs
// On Linux it needs a display (xvfb-run) and FUSE for the AppImage.
import { execFileSync, spawn } from "node:child_process"
import { createHash } from "node:crypto"
import { cpSync, createReadStream, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs"
import { createServer } from "node:http"
import { homedir } from "node:os"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const desktop = join(dirname(fileURLToPath(import.meta.url)), "..")
const work = join(desktop, "release", "update-test")
const feedPort = 8899
const debugPort = 9455
const oldVersion = "1.0.0"
const newVersion = "1.0.1"
const os = { darwin: "mac", win32: "win", linux: "linux" }[process.platform]
const arch = process.arch
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
const log = (message) => console.log(`[update-test] ${message}`)

function fail(message) {
  console.error(`[update-test] FAILED: ${message}`)
  process.exit(1)
}

async function until(what, check, seconds) {
  const end = Date.now() + seconds * 1000
  while (Date.now() < end) {
    const value = await check().catch(() => null)
    if (value) return value
    await sleep(1000)
  }
  fail(`timed out after ${seconds}s waiting for ${what}`)
}

// Packages one version with a local update feed and only this system's target.
function packageVersion(version) {
  const build = structuredClone(JSON.parse(readFileSync(join(desktop, "package.json"), "utf8")).build)
  build.extraMetadata = { version }
  build.directories = { output: join(work, version) }
  build.publish = [{ provider: "generic", url: `http://127.0.0.1:${feedPort}/` }]
  build.mac.target = [{ target: "zip", arch: [arch] }]
  build.win.target = [{ target: "nsis", arch: [arch] }]
  build.linux.target = [{ target: "AppImage", arch: [arch] }]
  const config = join(work, `config-${version}.json`)
  writeFileSync(config, JSON.stringify(build))
  log(`packaging ${version}`)
  execFileSync("npx", ["electron-builder", `--${os}`, "--publish", "never", "--config", config], {
    cwd: desktop,
    stdio: "inherit",
    shell: process.platform === "win32",
  })
  return join(work, version)
}

const find = (dir, test) => {
  const name = readdirSync(dir).find(test)
  if (!name) fail(`nothing matching in ${dir}`)
  return join(dir, name)
}

function serve(dir) {
  const server = createServer((request, response) => {
    const file = join(dir, decodeURIComponent(new URL(request.url, "http://x").pathname))
    if (!file.startsWith(dir) || !existsSync(file) || statSync(file).isDirectory()) {
      response.writeHead(404).end()
      return
    }
    log(`feed: ${request.method} ${request.url}`)
    response.writeHead(200, { "Content-Length": statSync(file).size })
    createReadStream(file).pipe(response)
  })
  server.listen(feedPort, "127.0.0.1")
  return server
}

async function evaluate(expression) {
  const targets = await (await fetch(`http://127.0.0.1:${debugPort}/json/list`)).json()
  const page = targets.find((t) => t.type === "page")
  if (!page) return null
  const ws = new WebSocket(page.webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    ws.addEventListener("open", resolve)
    ws.addEventListener("error", reject)
  })
  const result = await new Promise((resolve) => {
    ws.addEventListener("message", (event) => {
      const message = JSON.parse(event.data)
      if (message.id === 1) resolve(message.result?.result?.value ?? null)
    })
    ws.send(JSON.stringify({ id: 1, method: "Runtime.evaluate", params: { expression, awaitPromise: true, returnByValue: true } }))
  })
  ws.close()
  return result
}

const sha512 = (file) => createHash("sha512").update(readFileSync(file)).digest("base64")

// Installs the old version and returns how to start it and how to read the installed version.
function install(oldDir, newDir) {
  if (os === "mac") {
    const apps = join(work, "Applications")
    mkdirSync(apps, { recursive: true })
    execFileSync("ditto", ["-x", "-k", find(oldDir, (n) => n.endsWith(".zip")), apps])
    const bundle = join(apps, "Dofus Gear Explorer.app")
    return {
      executable: join(bundle, "Contents", "MacOS", "Dofus Gear Explorer"),
      installed: () =>
        execFileSync("plutil", ["-extract", "CFBundleShortVersionString", "raw", "-o", "-", join(bundle, "Contents", "Info.plist")], {
          encoding: "utf8",
        }).trim(),
    }
  }
  if (os === "win") {
    execFileSync(find(oldDir, (n) => n.endsWith("-setup.exe")), ["/S"], { stdio: "inherit" })
    const programs = join(process.env.LOCALAPPDATA ?? join(homedir(), "AppData", "Local"), "Programs")
    const folder = readdirSync(programs).find((n) => existsSync(join(programs, n, "Dofus Gear Explorer.exe")))
    if (!folder) fail(`the installer put no "Dofus Gear Explorer.exe" in ${programs}`)
    const executable = join(programs, folder, "Dofus Gear Explorer.exe")
    return {
      executable,
      installed: () =>
        execFileSync("powershell", ["-NoProfile", "-Command", `(Get-Item '${executable}').VersionInfo.ProductVersion`], {
          encoding: "utf8",
        })
          .trim()
          .replace(/^(\d+\.\d+\.\d+)\.0$/, "$1"),
    }
  }
  const appImage = join(work, "Dofus-Gear-Explorer.AppImage")
  cpSync(find(oldDir, (n) => n.endsWith(".AppImage")), appImage)
  execFileSync("chmod", ["+x", appImage])
  const oldHash = sha512(appImage)
  const newHash = sha512(find(newDir, (n) => n.endsWith(".AppImage")))
  // An AppImage carries no version the system can read: compare it with the two builds.
  return {
    executable: appImage,
    args: ["--no-sandbox"],
    installed: () => ({ [oldHash]: oldVersion, [newHash]: newVersion })[sha512(appImage)] ?? "unknown",
  }
}

if (!os) fail(`unsupported platform ${process.platform}`)
rmSync(work, { recursive: true, force: true })
mkdirSync(work, { recursive: true })
const oldDir = packageVersion(oldVersion)
const newDir = packageVersion(newVersion)
const server = serve(newDir)

const app = install(oldDir, newDir)
if (app.installed() !== oldVersion) fail(`installed version is ${app.installed()}, expected ${oldVersion}`)
log(`installed ${oldVersion}: ${app.executable}`)

const child = spawn(app.executable, [...(app.args ?? []), `--remote-debugging-port=${debugPort}`], {
  stdio: "inherit",
  env: { ...process.env, DDE_DATA_DIR: join(work, "data") },
})
child.on("exit", (code) => log(`${oldVersion} exited (${code})`))

const version = await until("the app window", () => evaluate("document.title && window.dde.config().then((c) => c.version)"), 90)
if (version !== oldVersion) fail(`the running app says ${version}, expected ${oldVersion}`)
log(`${oldVersion} is running`)

const update = await until(
  "the update to be downloaded",
  () => evaluate("window.dde.updates.last()").then((u) => (u?.state === "ready" ? u : null)),
  300
)
if (update.version !== newVersion) fail(`the app got ${update.version}, expected ${newVersion}`)
log(`update ${update.version} ready: restarting`)
await evaluate("window.dde.updates.install(), true").catch(() => {})

await until(`${newVersion} to be installed`, async () => app.installed() === newVersion, 180)
log(`${newVersion} is installed`)
server.close()
// The relaunched app runs on; the job's end stops it.
process.exit(0)
