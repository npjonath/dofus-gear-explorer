// electron-builder calls this once per target architecture.
const { existsSync } = require("node:fs")
const { join } = require("node:path")
const { execFileSync } = require("node:child_process")
const archNames = ["ia32", "x64", "armv7l", "arm64", "universal"]

module.exports = function prepareImporter(context) {
  const os = { darwin: "mac", win32: "win", linux: "linux" }[context.electronPlatformName]
  const arch = archNames[context.arch]
  const desktop = join(__dirname, "..")
  const binary = join(desktop, "fight-import", os + "-" + arch, os === "win" ? "fight-import.exe" : "fight-import")
  if (existsSync(binary)) return
  if (context.electronPlatformName !== process.platform || arch !== process.arch) {
    throw new Error("Build or download the native fight-import artifact for " + os + "-" + arch + " before cross-packaging.")
  }
  const source = join(desktop, "..", "..", "scripts", "fight-assets")
  execFileSync(
    "uv",
    [
      "run",
      "--isolated",
      "--python",
      "3.13",
      "--with-requirements",
      join(source, "requirements.txt"),
      "--with",
      "pyinstaller==6.22.3",
      join(source, "build.py"),
    ],
    { stdio: "inherit" }
  )
}
