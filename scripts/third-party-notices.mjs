// Writes THIRD_PARTY_NOTICES.md: the licence of every package the app ships,
// from pnpm's production dependency list and the Go modules of the API.
// Run it after changing dependencies: node scripts/third-party-notices.mjs
import { execFileSync } from "node:child_process"
import { readdirSync, readFileSync, writeFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const root = join(dirname(fileURLToPath(import.meta.url)), "..")
const licenceFile = /^(licen[cs]e|copying|notice)(\.(md|txt))?$/i

function licenceTexts(dir) {
  try {
    return readdirSync(dir)
      .filter((name) => licenceFile.test(name))
      .sort()
      .map((name) => readFileSync(join(dir, name), "utf8").trim())
  } catch {
    return []
  }
}

function guessLicence(text) {
  if (/Apache License/i.test(text)) return "Apache-2.0"
  if (/Permission is hereby granted, free of charge/i.test(text)) return "MIT"
  if (/Neither the name/i.test(text)) return "BSD-3-Clause"
  if (/Redistribution and use in source and binary forms/i.test(text)) return "BSD-2-Clause"
  return text ? "see text" : "unknown"
}

const entries = []

const js = JSON.parse(
  execFileSync("pnpm", ["licenses", "list", "--prod", "--json", "--recursive"], {
    cwd: root,
    encoding: "utf8",
    shell: process.platform === "win32",
  })
)
for (const [licence, packages] of Object.entries(js)) {
  for (const pkg of packages) {
    if (pkg.name.startsWith("@types/")) continue
    entries.push({
      name: pkg.name,
      version: pkg.versions.join(", "),
      licence,
      texts: licenceTexts(pkg.paths[0]),
    })
  }
}

const api = join(root, "apps/api")
execFileSync("go", ["mod", "download"], { cwd: api })
// Every system the API is built for, so the file is the same whichever system writes it.
const modules = new Set()
for (const goos of ["darwin", "linux", "windows"]) {
  const listed = execFileSync("go", ["list", "-deps", "-f", "{{with .Module}}{{.Path}} {{.Version}} {{.Dir}}{{end}}", "./cmd/dde-api"], {
    cwd: api,
    encoding: "utf8",
    env: { ...process.env, GOOS: goos, GOARCH: "amd64", CGO_ENABLED: "0" },
  })
  for (const line of listed.split("\n")) if (line) modules.add(line)
}
for (const line of modules) {
  const [name, version, dir] = line.split(" ")
  if (!version) continue
  const texts = licenceTexts(dir)
  entries.push({ name, version, licence: guessLicence(texts.join("\n")), texts })
}

entries.sort((a, b) => a.name.localeCompare(b.name))
const missing = entries.filter((entry) => entry.texts.length === 0).map((entry) => entry.name)

const out = [
  "# Third-party notices",
  "",
  "Dofus Gear Explorer ships the packages below. Their licences follow, as their authors wrote them.",
  "",
  "The desktop app is built on Electron; the licences of Electron and Chromium are in `LICENSE.electron.txt` and `LICENSES.chromium.html` next to the app.",
  "",
  "Dofus game data and artwork are not part of the app and are not covered here: they belong to Ankama.",
  "",
  "| Package | Version | Licence |",
  "| --- | --- | --- |",
  ...entries.map((entry) => `| ${entry.name} | ${entry.version} | ${entry.licence} |`),
  "",
  ...entries.flatMap((entry) =>
    entry.texts.length === 0
      ? []
      : [`## ${entry.name} ${entry.version}`, "", ...entry.texts.flatMap((text) => ["```text", text, "```", ""])]
  ),
]
writeFileSync(join(root, "THIRD_PARTY_NOTICES.md"), out.join("\n"))
console.log(`THIRD_PARTY_NOTICES.md: ${entries.length} packages`)
if (missing.length) console.log(`No licence file found for: ${missing.join(", ")}`)
