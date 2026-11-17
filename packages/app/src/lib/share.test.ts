import { describe, expect, it } from "vitest"
import { newBuild } from "../store"
import { decodeBuild, encodeBuild, readShared } from "./share"

describe("share links", () => {
  it("round-trips a build through its URL form", () => {
    const build = newBuild("Féca terre – PvM")
    build.level = 186
    build.base = { vitality: 300, wisdom: 0, strength: 250, intelligence: 0, chance: 0, agility: 12 }
    build.scrolls = { vitality: 100, wisdom: 100, strength: 100, intelligence: 0, chance: 0, agility: 0 }
    build.items = { amulet: 8262, ring1: 2469, dofus3: 7754 }
    build.mods = {
      amulet: { lines: { "11": 320 }, exos: [{ statId: 23, value: 1 }], draco: { rune: "emeraude", from: "earth" } },
      ring1: { transcendence: 20569 },
    }

    const encoded = encodeBuild(build)
    expect(encoded).toMatch(/^[A-Za-z0-9_-]+$/)

    const decoded = decodeBuild(encoded)
    expect(decoded).toMatchObject({
      name: build.name,
      level: 186,
      base: build.base,
      scrolls: build.scrolls,
      items: build.items,
      mods: build.mods,
    })
    expect(decoded?.id).not.toBe(build.id)
  })

  it("still opens links made before forgemagic existed", () => {
    const payload = [1, "Ancien lien", 200, [0, 0, 0, 0, 0, 0], [0, 0, 0, 0, 0, 0], [["ring1", 2469]]]
    const decoded = decodeBuild(btoa(JSON.stringify(payload)))
    expect(decoded?.items).toEqual({ ring1: 2469 })
    expect(decoded?.mods).toEqual({})
  })

  it("drops forgemagic that points at an empty slot", () => {
    const payload = [2, "x", 200, [0, 0, 0, 0, 0, 0], [0, 0, 0, 0, 0, 0], [["ring1", 2469]], { hat: { transcendence: 1 } }]
    expect(decodeBuild(btoa(JSON.stringify(payload)))?.mods).toEqual({})
  })

  it("rejects links that are not builds", () => {
    expect(decodeBuild("not-base64!")).toBeNull()
    expect(decodeBuild(btoa(JSON.stringify({ hello: "world" })))).toBeNull()
    expect(decodeBuild(btoa(JSON.stringify([3, "future format"])))).toBeNull()
  })

  it("clamps values a hand-edited link could carry", () => {
    const payload = [1, "x", 999, [-5, 0, 0, 0, 0, 0], [500, 0, 0, 0, 0, 0], [["ring1", 1.5], ["ring2", 10]]]
    const decoded = decodeBuild(btoa(JSON.stringify(payload)))
    expect(decoded?.level).toBe(200)
    expect(decoded?.base.vitality).toBe(0)
    expect(decoded?.scrolls.vitality).toBe(100)
    expect(decoded?.items).toEqual({ ring2: 10 })
  })

  it("imports a pasted code or a pasted link", () => {
    const build = newBuild("Cra eau")
    build.items = { amulet: 8262 }
    const code = encodeBuild(build)

    expect(readShared(code)?.items).toEqual({ amulet: 8262 })
    expect(readShared(`  ${code.slice(0, 10)}\n${code.slice(10)}  `)?.name).toBe("Cra eau")
    expect(readShared(`https://example.org/#/build?b=${code}`)?.name).toBe("Cra eau")
    expect(readShared(`file:///C:/app/index.html#/build?b=${code}&x=1`)?.name).toBe("Cra eau")
  })

  it("refuses pasted text that is not a stuff", () => {
    expect(readShared("")).toBeNull()
    expect(readShared("bonjour, voici mon stuff !")).toBeNull()
    expect(readShared("https://example.org/#/build")).toBeNull()
  })
})
