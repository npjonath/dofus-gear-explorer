// Fails unless every package.json carries the given version (the release tag without its "v").
import { readFileSync } from "node:fs"

const expected = process.argv[2]
const files = ["package.json", "apps/api/package.json", "apps/desktop/package.json", "apps/web/package.json", "packages/app/package.json"]
const wrong = files
  .map((file) => [file, JSON.parse(readFileSync(file, "utf8")).version])
  .filter(([, version]) => version !== expected)
if (!expected || wrong.length > 0) {
  for (const [file, version] of wrong) console.error(`${file} is ${version}, the tag says ${expected}`)
  process.exit(1)
}
console.log(`Every package.json is ${expected}`)
