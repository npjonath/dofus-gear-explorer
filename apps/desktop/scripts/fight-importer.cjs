// electron-builder calls this once per target architecture.
const { createHash } = require("node:crypto")
const { existsSync, readdirSync, readFileSync } = require("node:fs")
const { join } = require("node:path")
const { execFileSync } = require("node:child_process")
const archNames = ["ia32", "x64", "armv7l", "arm64", "universal"]

module.exports = function prepareImporter(context) {
  const os = { darwin: "mac", win32: "win", linux: "linux" }[context.electronPlatformName]
  const arch = archNames[context.arch]
  const desktop = join(__dirname, "..")
  const binary = join(desktop, "fight-import", os + "-" + arch, os === "win" ? "fight-import.exe" : "fight-import")
  const source = join(desktop, "..", "..", "scripts", "fight-assets")
  const hash = createHash("sha256")
  const fingerprint = join(desktop, "fight-import", os + "-" + arch, "source.sha256")
  const files = readdirSync(source)
    .filter((name) => (name.endsWith(".py") && !name.startsWith("test_")) || name === "requirements.txt")
    .sort()
  for (const name of files) hash.update(name + "\0" + readFileSync(join(source, name), "utf8").replace(/\r\n/g, "\n") + "\0")
  if (existsSync(binary) && existsSync(fingerprint) && readFileSync(fingerprint, "ascii").trim() === hash.digest("hex")) return
  if (context.electronPlatformName !== process.platform || arch !== process.arch) {
    throw new Error("Build or download the native fight-import artifact for " + os + "-" + arch + " before cross-packaging.")
  }
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
