import { describe, expect, it } from "vitest"
import {
  diffCharacter,
  diffDamage,
  diffItems,
  diffLines,
  diffSets,
  diffStats,
  hitValue,
  percentChange,
} from "./diff"
import { attack, damage, evaluation, item, line, meta } from "./fixtures"

const zero = { vitality: 0, wisdom: 0, strength: 0, intelligence: 0, chance: 0, agility: 0 }

describe("diffStats", () => {
  const from = evaluation({ stats: { "0": 4000, "1": 11, "10": 500, "18": 30, "25": 20 } })
  const to = evaluation({ stats: { "0": 4300, "1": 11, "10": 420, "18": 30, "11": 300 } })

  it("splits the stats into gains and losses, in the game's order", () => {
    const diff = diffStats(meta, from, to)
    expect(diff.gains.map((row) => [row.stat.label, row.from, row.to, row.delta])).toEqual([
      ["Points de vie", 4000, 4300, 300],
      ["Vitalité", 0, 300, 300],
    ])
    expect(diff.losses.map((row) => [row.stat.label, row.delta])).toEqual([
      ["Force", -80],
      ["Dommages", -20],
    ])
  })

  it("keeps the stats that do not move apart, and drops the ones neither stuff has", () => {
    const diff = diffStats(meta, from, to)
    expect(diff.unchanged.map((row) => row.stat.label)).toEqual(["PA", "Critique"])
  })

  it("puts the stats to read first at the head of each list", () => {
    const diff = diffStats(meta, from, to, [25, 11])
    expect(diff.gains.map((row) => row.stat.label)).toEqual(["Vitalité", "Points de vie"])
    expect(diff.losses.map((row) => row.stat.label)).toEqual(["Dommages", "Force"])
  })

  it("finds nothing between a stuff and itself", () => {
    const diff = diffStats(meta, from, from)
    expect(diff.gains).toEqual([])
    expect(diff.losses).toEqual([])
  })
})

describe("diffItems", () => {
  const hat = item(1, "Coiffe", "hat")
  const otherHat = item(2, "Casque", "hat")
  const gelano = item(3, "Gelano", "ring")
  const band = item(4, "Alliance", "ring")
  const bow = item(5, "Arc", "weapon")
  const ochre = item(6, "Dofus Ocre", "dofus")
  const emerald = item(7, "Dofus Émeraude", "dofus")

  const changes = (rows: ReturnType<typeof diffItems>) =>
    rows.map((row) => [row.slot, row.change, row.from?.item.name ?? null, row.to?.item.name ?? null])

  it("tells a swapped item, an added one and a removed one", () => {
    const from = { mods: {}, evaluation: evaluation({ items: { hat, ring1: gelano, dofus1: ochre } }) }
    const to = { mods: {}, evaluation: evaluation({ items: { hat: otherHat, ring1: gelano, weapon: bow } }) }
    expect(changes(diffItems(meta, from, to))).toEqual([
      ["hat", "swap", "Coiffe", "Casque"],
      ["ring", "same", "Gelano", "Gelano"],
      ["weapon", "added", null, "Arc"],
      ["dofus", "removed", "Dofus Ocre", null],
    ])
  })

  it("does not count an item that moved between two places of the same kind", () => {
    const from = { mods: {}, evaluation: evaluation({ items: { ring1: gelano, ring2: band, dofus1: ochre, dofus2: emerald } }) }
    const to = { mods: {}, evaluation: evaluation({ items: { ring1: band, ring2: gelano, dofus1: emerald, dofus2: ochre } }) }
    expect(diffItems(meta, from, to).every((row) => row.change === "same")).toBe(true)
  })

  it("pairs the item kept before the one replaced", () => {
    const from = { mods: {}, evaluation: evaluation({ items: { ring1: gelano, ring2: band } }) }
    const to = { mods: {}, evaluation: evaluation({ items: { ring1: band } }) }
    expect(changes(diffItems(meta, from, to))).toEqual([
      ["ring", "same", "Alliance", "Alliance"],
      ["ring", "removed", "Gelano", null],
    ])
  })

  it("sees the same item forged differently, with the lines that differ", () => {
    const from = {
      mods: {},
      evaluation: evaluation({ items: { ring1: gelano }, forge: { ring1: [line(10, 80), line(11, 200)] } }),
    }
    const to = {
      mods: { ring1: { lines: { "10": 101 }, exos: [{ statId: 1, value: 1 }] } },
      evaluation: evaluation({
        items: { ring1: gelano },
        forge: { ring1: [line(10, 101, "over"), line(11, 200), line(1, 1, "exo")] },
      }),
    }
    const [row] = diffItems(meta, from, to)
    expect(row?.change).toBe("forge")
    expect(row?.lines).toEqual([
      { statId: 10, from: 80, to: 101, origin: "over", runeFrom: 0, runeTo: 0 },
      { statId: 1, from: 0, to: 1, origin: "exo", runeFrom: 0, runeTo: 0 },
    ])
  })

  it("reads the same forgemagic written in another order as the same", () => {
    const exos = [
      { statId: 1, value: 1 },
      { statId: 23, value: 1 },
    ]
    const from = { mods: { ring1: { exos, lines: { "10": 90, "11": 250 } } }, evaluation: evaluation({ items: { ring1: gelano } }) }
    const to = {
      mods: { ring1: { lines: { "11": 250, "10": 90 }, exos: exos.toReversed() }, hat: {} },
      evaluation: evaluation({ items: { ring1: gelano } }),
    }
    expect(changes(diffItems(meta, from, to))).toEqual([["ring", "same", "Gelano", "Gelano"]])
  })

  it("notes a weapon's elemental forgemagic", () => {
    const from = { mods: {}, evaluation: evaluation({ items: { weapon: bow } }) }
    const to = { mods: { weapon: { element: "fire" as const } }, evaluation: evaluation({ items: { weapon: bow } }) }
    const [row] = diffItems(meta, from, to)
    expect(row?.change).toBe("forge")
    expect(row?.element).toEqual({ from: undefined, to: "fire" })
  })
})

describe("diffLines", () => {
  it("keeps a line that only one side has, and a rune that moved", () => {
    expect(diffLines([line(10, 80), line(18, 5, "exo")], [line(10, 80, "base", 0), line(11, 40, "transcendence", 40)])).toEqual([
      { statId: 18, from: 5, to: 0, origin: "exo", runeFrom: 0, runeTo: 0 },
      { statId: 11, from: 0, to: 40, origin: "transcendence", runeFrom: 0, runeTo: 40 },
    ])
  })
})

describe("diffSets", () => {
  const set = (id: number, count: number) => ({ id, name: `Panoplie ${id}`, count, size: 8, bonuses: [] })

  it("lists the sets whose bonus changes", () => {
    const from = evaluation({ sets: [set(1, 3), set(2, 2), set(3, 1)] })
    const to = evaluation({ sets: [set(1, 5), set(2, 2), set(4, 2), set(5, 1)] })
    expect(diffSets(from, to)).toEqual([
      { id: 1, name: "Panoplie 1", size: 8, from: 3, to: 5 },
      { id: 4, name: "Panoplie 4", size: 8, from: 0, to: 2 },
    ])
  })
})

describe("diffCharacter", () => {
  it("lists the level, points and scrolls that differ", () => {
    const from = { level: 200, base: { ...zero, strength: 300 }, scrolls: { ...zero, strength: 100 } }
    const to = { level: 199, base: { ...zero, chance: 300 }, scrolls: { ...zero, strength: 100 } }
    expect(diffCharacter(meta, from, to)).toEqual({
      level: { from: 200, to: 199 },
      base: [
        { key: "strength", from: 300, to: 0 },
        { key: "chance", from: 0, to: 300 },
      ],
      scrolls: [],
    })
  })

  it("finds nothing between two characters alike", () => {
    const character = { level: 200, base: zero, scrolls: zero }
    expect(diffCharacter(meta, character, character)).toEqual({ level: null, base: [], scrolls: [] })
  })
})

describe("diffDamage", () => {
  it("matches the spells by id and sets the two weapons side by side", () => {
    const from = damage([attack(1, "Flèche", 400), attack(2, "Tir", 300), attack(9, "Coup", 100, { breedId: 0 })], attack(50, "Arc", 500))
    const to = damage([attack(1, "Flèche", 460), attack(3, "Salve", 250), attack(9, "Coup", 100, { breedId: 0 })], attack(51, "Arc long", 650))
    const diff = diffDamage(from, to)
    expect(diff.weapon?.from?.name).toBe("Arc")
    expect(diff.weapon?.to?.name).toBe("Arc long")
    expect(diff.classSpells.map((row) => [row.key, row.from?.mean ?? null, row.to?.mean ?? null])).toEqual([
      ["spell-1", 400, 460],
      ["spell-3", null, 250],
      ["spell-2", 300, null],
    ])
    expect(diff.commonSpells.map((row) => row.key)).toEqual(["spell-9"])
  })

  it("has no weapon row when neither stuff holds one", () => {
    expect(diffDamage(damage([]), damage([])).weapon).toBeNull()
  })
})

describe("hitValue", () => {
  const spell = attack(1, "Flèche", 400)

  it("reads the average roll of a normal and of a critical hit", () => {
    expect(hitValue(spell, "mean")).toBe(400)
    expect(hitValue(spell, "normal")).toBe(400)
    expect(hitValue(spell, "crit")).toBe(420)
  })

  it("has no critical value for an attack that cannot land one", () => {
    expect(hitValue(attack(1, "Flèche", 400, { critChance: 0 }), "crit")).toBeNull()
  })
})

describe("percentChange", () => {
  it("is relative to the first stuff", () => {
    expect(percentChange(400, 460)).toBeCloseTo(15)
    expect(percentChange(400, 300)).toBeCloseTo(-25)
    expect(percentChange(0, 300)).toBeNull()
  })
})
