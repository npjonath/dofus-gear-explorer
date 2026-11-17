import type { ItemMods, PrimaryKey } from "../api/types"
import { newBuild, type Build } from "../store"

const keys: PrimaryKey[] = ["vitality", "wisdom", "strength", "intelligence", "chance", "agility"]

// A shared build travels in the URL as base64url JSON, kept short with
// positional arrays: [version, name, level, base[], scrolls[], [position, id][], mods, class].
// Version 1 links have no mods, version 2 links no class.
type Payload = [
  1 | 2 | 3,
  string,
  number,
  number[],
  number[],
  [string, number][],
  Record<string, ItemMods>?,
  (number | null)?,
]

function toBase64Url(text: string): string {
  const bytes = new TextEncoder().encode(text)
  let binary = ""
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}

function fromBase64Url(encoded: string): string {
  const binary = atob(encoded.replace(/-/g, "+").replace(/_/g, "/"))
  return new TextDecoder().decode(Uint8Array.from(binary, (c) => c.charCodeAt(0)))
}

export function encodeBuild(build: Build): string {
  const payload: Payload = [
    3,
    build.name,
    build.level,
    keys.map((k) => build.base[k]),
    keys.map((k) => build.scrolls[k]),
    Object.entries(build.items),
    build.mods,
    build.breedId,
  ]
  return toBase64Url(JSON.stringify(payload))
}

export function decodeBuild(encoded: string): Build | null {
  try {
    const payload = JSON.parse(fromBase64Url(encoded)) as Payload
    if (!Array.isArray(payload) || ![1, 2, 3].includes(payload[0])) return null
    const [, name, level, base, scrolls, items, mods, breedId] = payload
    const build = newBuild(String(name).slice(0, 80))
    build.level = Math.min(200, Math.max(1, Number(level) || 200))
    if (Number.isInteger(breedId) && Number(breedId) > 0) build.breedId = Number(breedId)
    keys.forEach((k, i) => {
      build.base[k] = Math.max(0, Number(base[i]) || 0)
      build.scrolls[k] = Math.min(100, Math.max(0, Number(scrolls[i]) || 0))
    })
    for (const [position, id] of items) {
      if (typeof position === "string" && Number.isInteger(id)) build.items[position] = id
    }
    // The API validates mods; here only their shape and their item matter.
    if (mods && typeof mods === "object") {
      for (const [position, value] of Object.entries(mods)) {
        if (position in build.items && value && typeof value === "object") build.mods[position] = value
      }
    }
    return build
  } catch {
    return null
  }
}

// What a player pastes to import a stuff: a share link (the stuff is its
// `b` parameter) or the code alone, possibly wrapped in spaces or line breaks.
export function readShared(text: string): Build | null {
  const fromLink = /[?&]b=([A-Za-z0-9_-]+)/.exec(text)
  const code = fromLink ? fromLink[1]! : text.replace(/\s+/g, "")
  if (!/^[A-Za-z0-9_-]+$/.test(code)) return null
  return decodeBuild(code)
}
