import { app, net } from "electron"
import type { UpdateInfo } from "electron-updater"
import { execFile, spawn } from "node:child_process"
import { createHash } from "node:crypto"
import { accessSync, constants, createWriteStream, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { Readable, Transform } from "node:stream"
import { pipeline } from "node:stream/promises"
import { promisify } from "node:util"

// Squirrel.Mac, which electron-updater uses on macOS, only installs an update
// signed with the same Apple Developer ID. The app is signed ad hoc, so on
// macOS the update is installed here instead: the zip of the release is
// downloaded, checked against the sha512 the release publishes, unpacked and
// checked, and the app bundle is swapped once the app has quit.

const run = promisify(execFile)

export interface PreparedUpdate {
  version: string
  app: string
  dir: string
}

/** The running bundle: …/Dofus Gear Explorer.app/Contents/MacOS/<executable>. */
function bundlePath(): string {
  return resolve(app.getPath("exe"), "../../..")
}

/**
 * Whether the bundle can be replaced: not when the app runs from the disk
 * image, from a folder the user can't write to, or from the read-only copy
 * Gatekeeper makes of an app that was never moved (App Translocation).
 */
export function canReplaceBundle(): boolean {
  const bundle = bundlePath()
  if (!bundle.endsWith(".app") || bundle.startsWith("/Volumes/") || bundle.includes("/AppTranslocation/")) return false
  try {
    accessSync(dirname(bundle), constants.W_OK)
    accessSync(bundle, constants.W_OK)
    return true
  } catch {
    return false
  }
}

/** Where the release files are, from the app-update.yml electron-builder writes. */
function releaseFilesUrl(version: string): string {
  const config = readFileSync(join(process.resourcesPath, "app-update.yml"), "utf8")
  const value = (key: string) => new RegExp(`^${key}:\\s*(\\S+)`, "m").exec(config)?.[1]
  if (value("provider") === "generic") return value("url")!.replace(/\/?$/, "/")
  const owner = value("owner")
  const repo = value("repo")
  if (!owner || !repo) throw new Error("app-update.yml names no repository")
  return `https://github.com/${owner}/${repo}/releases/download/v${version}/`
}

async function download(url: string, file: string, sha512: string) {
  const response = await net.fetch(url)
  if (!response.ok || !response.body) throw new Error(`download failed: ${response.status}`)
  const hash = createHash("sha512")
  const tap = new Transform({
    transform(chunk: Buffer, _encoding, done) {
      hash.update(chunk)
      done(null, chunk)
    },
  })
  await pipeline(Readable.fromWeb(response.body as import("node:stream/web").ReadableStream), tap, createWriteStream(file))
  if (hash.digest("base64") !== sha512) throw new Error("the download does not match the release's checksum")
}

const plist = async (bundle: string, key: string) =>
  (await run("/usr/bin/plutil", ["-extract", key, "raw", "-o", "-", join(bundle, "Contents", "Info.plist")])).stdout.trim()

/** Downloads and checks the new version; the current app keeps running. */
export async function prepareMacUpdate(info: UpdateInfo): Promise<PreparedUpdate> {
  const zip = info.files.find((f) => f.url.endsWith(`-mac-${process.arch}.zip`))
  if (!zip) throw new Error(`the release has no macOS ${process.arch} zip`)
  const dir = mkdtempSync(join(app.getPath("temp"), "dofus-gear-explorer-update-"))
  try {
    const archive = join(dir, "update.zip")
    await download(releaseFilesUrl(info.version) + encodeURIComponent(zip.url), archive, zip.sha512)
    const unpacked = join(dir, "unpacked")
    await run("/usr/bin/ditto", ["-x", "-k", archive, unpacked])
    rmSync(archive)
    const name = readdirSync(unpacked).find((entry) => entry.endsWith(".app"))
    if (!name) throw new Error("the zip holds no app")
    const next = join(unpacked, name)
    const current = bundlePath()
    if ((await plist(next, "CFBundleIdentifier")) !== (await plist(current, "CFBundleIdentifier"))) throw new Error("the zip holds another app")
    if ((await plist(next, "CFBundleShortVersionString")) !== info.version) throw new Error("the zip holds another version")
    // The bundle must be intact: every file matches the signature made when it was built.
    await run("/usr/bin/codesign", ["--verify", "--deep", "--strict", next])
    return { version: info.version, app: next, dir }
  } catch (error) {
    rmSync(dir, { recursive: true, force: true })
    throw error
  }
}

// Runs once the app has quit: the old bundle is set aside, the new one moved
// in, and the old one deleted, or put back if the move failed.
const swapScript = `#!/bin/sh
pid="$1"; old="$2"; new="$3"; dir="$4"; relaunch="$5"
while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done
backup="$old.previous"
rm -rf "$backup"
if mv "$old" "$backup"; then
  if mv "$new" "$old"; then
    rm -rf "$backup"
    xattr -dr com.apple.quarantine "$old" 2>/dev/null
  else
    mv "$backup" "$old"
  fi
fi
rm -rf "$dir"
if [ "$relaunch" = 1 ]; then open "$old"; fi
`

/** Swaps the bundle once this process exits; the caller quits the app. */
export function installMacUpdate(update: PreparedUpdate, relaunch: boolean) {
  const script = join(update.dir, "install.sh")
  writeFileSync(script, swapScript, { mode: 0o755 })
  spawn("/bin/sh", [script, String(process.pid), bundlePath(), update.app, update.dir, relaunch ? "1" : "0"], {
    detached: true,
    stdio: "ignore",
  }).unref()
}
