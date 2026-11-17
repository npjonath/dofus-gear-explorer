// pnpm release X.Y.Z: prepares main for the Release workflow.
//
// 1. Sets X.Y.Z in every package.json, on a branch from develop, and merges
//    that pull request into develop once CI passes.
// 2. Merges develop into main through a pull request (merge commit).
// 3. Leaves one step: press "Run workflow" on the Release workflow, on main.
//
// It runs as you, through the GitHub CLI (gh), and changes nothing locally
// besides switching back to an up-to-date develop.
import { execFileSync } from "node:child_process"
import { readFileSync, writeFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const root = join(dirname(fileURLToPath(import.meta.url)), "..")
const packages = ["package.json", "apps/api/package.json", "apps/desktop/package.json", "apps/web/package.json", "packages/app/package.json"]
const pollSeconds = 15
const timeoutMinutes = 30

function run(command, args, { quiet = false } = {}) {
  return execFileSync(command, args, {
    cwd: root,
    encoding: "utf8",
    stdio: quiet ? ["ignore", "pipe", "pipe"] : ["ignore", "pipe", "inherit"],
    shell: process.platform === "win32",
  }).trim()
}

const git = (...args) => run("git", args)
const gh = (...args) => run("gh", args)
const step = (message) => console.log(`\n▸ ${message}`)

function fail(message) {
  console.error(`\n✗ ${message}`)
  process.exit(1)
}

function parse(version) {
  return version.split(".").map(Number)
}

function newer(a, b) {
  const [x, y] = [parse(a), parse(b)]
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] > y[i]
  return false
}

const sleep = (seconds) => new Promise((resolve) => setTimeout(resolve, seconds * 1000))

// Waits for the required checks, then merges. A failed check stops the release.
async function mergeWhenGreen(number, method) {
  const deadline = Date.now() + timeoutMinutes * 60_000
  let last = ""
  while (Date.now() < deadline) {
    const pr = JSON.parse(gh("pr", "view", String(number), "--json", "state,mergeStateStatus,statusCheckRollup"))
    if (pr.state === "MERGED") return
    const checks = pr.statusCheckRollup ?? []
    const failed = checks.filter((c) => ["FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "ERROR"].includes(c.conclusion ?? c.state))
    if (failed.length > 0) fail(`Checks failed on #${number}: ${failed.map((c) => c.name ?? c.context).join(", ")}. Fix them, then run the command again.`)
    if (pr.mergeStateStatus === "CLEAN") {
      gh("pr", "merge", String(number), `--${method}`)
      return
    }
    if (pr.mergeStateStatus === "DIRTY") fail(`#${number} has conflicts to resolve first.`)
    if (pr.mergeStateStatus === "BEHIND") {
      if (method === "merge") {
        fail(`#${number} is blocked because main requires develop to contain every commit of main, and each release adds a merge commit there. Untick "Require branches to be up to date before merging" in the "Protect main" ruleset (Settings → Rules → Rulesets), then run the command again.`)
      }
      console.log(`  #${number} is behind its base branch: updating it`)
      gh("pr", "update-branch", String(number))
    }
    const pending = checks.filter((c) => !c.conclusion && c.state !== "SUCCESS").length
    const status = `  waiting for #${number}: ${pending} check(s) running (${pr.mergeStateStatus.toLowerCase()})`
    if (status !== last) console.log(status)
    last = status
    await sleep(pollSeconds)
  }
  fail(`#${number} could not be merged within ${timeoutMinutes} minutes. Open it on GitHub to see what blocks it.`)
}

process.on("uncaughtException", (error) => fail(error.message))

const version = process.argv[2]?.replace(/^v/, "")
if (!version || !/^\d+\.\d+\.\d+$/.test(version)) fail("Usage: pnpm release X.Y.Z")
const tag = `v${version}`

step("Checking the repository")
try {
  run("gh", ["auth", "status"], { quiet: true })
} catch {
  fail("The GitHub CLI is not signed in. Run: gh auth login")
}
if (git("status", "--porcelain")) fail("Commit or stash your changes first.")
git("fetch", "--quiet", "--prune", "--tags", "origin")
const current = JSON.parse(git("show", "origin/develop:package.json")).version
if (!newer(version, current)) fail(`${version} must be newer than the version on develop, ${current}.`)
if (git("ls-remote", "--tags", "origin", `refs/tags/${tag}`)) fail(`${tag} is already released.`)
const repo = gh("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
console.log(`  ${repo}: ${current} → ${version}`)

const branch = `release/${tag}`
step(`Setting ${version} in every package.json (${branch})`)
const openBump = gh("pr", "list", "--head", branch, "--base", "develop", "--state", "open", "--json", "number", "--jq", ".[0].number")
let bump = openBump ? Number(openBump) : 0
if (!bump) {
  git("switch", "--quiet", "--create", branch, "origin/develop")
  for (const file of packages) {
    const path = join(root, file)
    writeFileSync(path, readFileSync(path, "utf8").replace(/"version": "[^"]+"/, `"version": "${version}"`))
  }
  run("node", ["scripts/check-version.mjs", version])
  git("commit", "--quiet", "--all", "--message", `chore: release ${version}`)
  git("push", "--quiet", "--set-upstream", "origin", branch)
  const url = gh(
    "pr", "create", "--base", "develop", "--head", branch, "--label", "skip-changelog",
    "--title", `chore: release ${version}`,
    "--body", `Sets ${version} in every \`package.json\`. Opened by \`pnpm release ${version}\`.`
  )
  bump = Number(url.split("/").pop())
  console.log(`  ${url}`)
}
await mergeWhenGreen(bump, "squash")
console.log(`  #${bump} merged into develop`)

step("Merging develop into main")
git("fetch", "--quiet", "origin")
const openRelease = gh("pr", "list", "--head", "develop", "--base", "main", "--state", "open", "--json", "number", "--jq", ".[0].number")
let release = openRelease ? Number(openRelease) : 0
if (!release) {
  const log = git("log", "--format=- %s", "origin/main..origin/develop")
  const url = gh(
    "pr", "create", "--base", "main", "--head", "develop", "--label", "skip-changelog",
    "--title", `Release ${version}`,
    "--body", `Brings develop into main for ${tag}. Opened by \`pnpm release ${version}\`.\n\n${log}`
  )
  release = Number(url.split("/").pop())
  console.log(`  ${url}`)
}
await mergeWhenGreen(release, "merge")
console.log(`  #${release} merged into main`)

step("Back to develop")
git("switch", "--quiet", "develop")
git("pull", "--quiet", "--ff-only", "origin", "develop")
try {
  git("branch", "--quiet", "--delete", "--force", branch)
} catch {
  // Already gone.
}

console.log(`
✓ main is ready for ${tag}.

Last step: open https://github.com/${repo}/actions/workflows/release.yml,
press "Run workflow", keep "main" and leave "Dry run" unticked.
It builds the installers and publishes the release (about 15 minutes).`)
