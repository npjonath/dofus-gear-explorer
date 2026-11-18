import type { Attack, DamageResult, Evaluation, ForgedLine, Item, Meta, Slot, Stat } from "../../api/types"

// A small game for the comparison tests: a few stats, six places, two stuffs' worth of items.

const stat = (id: number, key: string, label: string, unit = ""): Stat => ({
  id,
  key,
  label,
  unit,
  categoryId: 1,
  forgeMax: 100,
})

export const meta = {
  version: "test",
  languages: ["fr"],
  maxLevel: 200,
  slots: ["hat", "ring", "weapon", "dofus"],
  positions: [
    { key: "hat", slot: "hat" },
    { key: "ring1", slot: "ring" },
    { key: "ring2", slot: "ring" },
    { key: "weapon", slot: "weapon" },
    { key: "dofus1", slot: "dofus" },
    { key: "dofus2", slot: "dofus" },
  ],
  categories: [
    { id: 1, name: "Primaires", statIds: [0, 1, 23, 19, 11, 10, 13] },
    { id: 2, name: "Secondaires", statIds: [18, 25] },
  ],
  stats: [
    stat(0, "hitPoints", "Points de vie"),
    stat(1, "actionPoints", "PA"),
    stat(23, "movementPoints", "PM"),
    stat(19, "range", "Portée"),
    stat(11, "vitality", "Vitalité"),
    stat(10, "strength", "Force"),
    stat(13, "chance", "Chance"),
    stat(18, "criticalHit", "Critique", "%"),
    stat(25, "damageBonus", "Dommages"),
  ],
  types: [],
  primaryStats: [
    { key: "vitality", statId: 11 },
    { key: "wisdom", statId: 12 },
    { key: "strength", statId: 10 },
    { key: "intelligence", statId: 15 },
    { key: "chance", statId: 13 },
    { key: "agility", statId: 14 },
  ],
  itemCount: 0,
  setCount: 0,
  forge: {
    slots: [],
    limitedExos: [],
    runes: [],
    dracoRunes: [],
    elements: [],
    elementIds: {} as Meta["forge"]["elementIds"],
  },
  breeds: [{ id: 9, name: "Cra" }],
  spells: [],
  prysmaradite: { typeId: 0, position: "dofus1" },
} satisfies Meta as Meta

export function item(id: number, name: string, slot: Slot, set?: { id: number; name: string }): Item {
  return {
    id,
    name,
    level: 200,
    typeId: 1,
    typeName: slot,
    slot,
    image: `/images/${id}.png`,
    set: set ?? null,
    effects: [],
    condition: "",
    weapon: null,
    passive: null,
    forgeable: true,
  }
}

export function line(statId: number, value: number, origin: ForgedLine["origin"] = "base", rune = 0): ForgedLine {
  return { statId, min: 0, max: value, cap: value, value, origin, rune, natural: origin === "base", text: `${value}` }
}

export function evaluation(patch: Partial<Evaluation> = {}): Evaluation {
  return {
    level: 200,
    stats: {},
    forge: {},
    runes: {},
    forgeWeight: {},
    ignoredExos: [],
    capped: [],
    sets: [],
    setBonuses: 0,
    conditions: [],
    issues: [],
    points: { spent: 0, total: 995 },
    valid: true,
    items: {},
    ...patch,
  }
}

export function attack(id: number, name: string, mean: number, patch: Partial<Attack> = {}): Attack {
  return {
    id,
    name,
    grade: 1,
    apCost: 4,
    rangeMin: 1,
    rangeMax: 8,
    critChance: 10,
    melee: false,
    lines: [
      {
        element: 1,
        best: false,
        steal: false,
        delay: 0,
        targets: ["enemy"],
        counted: true,
        min: mean - 10,
        max: mean + 10,
        critMin: mean,
        critMax: mean + 40,
      },
    ],
    total: { min: mean - 10, max: mean + 10, critMin: mean, critMax: mean + 40 },
    critRate: 10,
    mean,
    breedId: 9,
    variantOf: 0,
    ...patch,
  }
}

export function damage(spells: Attack[], weapon: Attack | null = null): DamageResult {
  return { weapon, spells }
}
