// Cross-compiles the API for every platform the desktop app is packaged for.
// The folder names are electron-builder's ${os}-${arch}, which is how the
// desktop package picks the right binary.
import { spawnSync } from "node:child_process"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const targets = {
  "mac-arm64": ["darwin", "arm64"],
  "mac-x64": ["darwin", "amd64"],
  "win-x64": ["windows", "amd64"],
  "win-arm64": ["windows", "arm64"],
  "linux-x64": ["linux", "amd64"],
  "linux-arm64": ["linux", "arm64"],
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..")
const wanted = process.argv.slice(2)
const unknown = wanted.filter((name) => !(name in targets))
if (unknown.length > 0) {
  console.error(`Unknown target: ${unknown.join(", ")}. Known targets: ${Object.keys(targets).join(", ")}`)
  process.exit(1)
}

for (const name of wanted.length > 0 ? wanted : Object.keys(targets)) {
  const [goos, goarch] = targets[name]
  const output = join("bin", name, goos === "windows" ? "dde-api.exe" : "dde-api")
  const built = spawnSync("go", ["build", "-trimpath", "-ldflags", "-s -w", "-o", output, "./cmd/dde-api"], {
    cwd: root,
    stdio: "inherit",
    env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0" },
  })
  if (built.status !== 0) process.exit(built.status ?? 1)
  console.log(`${name}: ${output}`)
}
