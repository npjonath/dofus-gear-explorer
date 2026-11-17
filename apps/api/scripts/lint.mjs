// Lints the API: gofmt formatting, then staticcheck (pinned as a Go tool in go.mod).
import { execFileSync, spawnSync } from "node:child_process"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const root = join(dirname(fileURLToPath(import.meta.url)), "..")

const unformatted = execFileSync("gofmt", ["-l", "."], { cwd: root, encoding: "utf8" }).trim()
if (unformatted) {
  console.error(`Not gofmt-formatted (run gofmt -w .):\n${unformatted}`)
  process.exit(1)
}

const staticcheck = spawnSync("go", ["tool", "staticcheck", "./..."], { cwd: root, stdio: "inherit" })
process.exit(staticcheck.status ?? 1)
