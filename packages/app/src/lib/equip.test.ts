import { describe, expect, it } from "vitest"
import type { Meta } from "../api/types"
import { newBuild } from "../store"
import { targetPosition } from "./equip"

const meta = {
  positions: [
    { key: "amulet", slot: "amulet" },
    { key: "ring1", slot: "ring" },
    { key: "ring2", slot: "ring" },
    { key: "dofus1", slot: "dofus" },
    { key: "dofus2", slot: "dofus" },
  ],
  prysmaradite: { typeId: 217, position: "dofus1" },
} as Meta

describe("targetPosition", () => {
  it("fills the first free place of the item's kind", () => {
    const build = newBuild("test")
    expect(targetPosition(meta, build, { id: 1, slot: "ring", typeId: 9 })).toBe("ring1")
    build.items.ring1 = 1
    expect(targetPosition(meta, build, { id: 2, slot: "ring", typeId: 9 })).toBe("ring2")
  })

  it("keeps an item where it already is", () => {
    const build = newBuild("test")
    build.items = { ring1: 1, ring2: 2 }
    expect(targetPosition(meta, build, { id: 2, slot: "ring", typeId: 9 })).toBe("ring2")
  })

  it("always sends a prysmaradite to the first Dofus place", () => {
    const build = newBuild("test")
    build.items = { dofus1: 7754 }
    expect(targetPosition(meta, build, { id: 21997, slot: "dofus", typeId: 217 })).toBe("dofus1")
    expect(targetPosition(meta, build, { id: 7115, slot: "dofus", typeId: 23 })).toBe("dofus2")
  })

  it("replaces the first place when all are taken", () => {
    const build = newBuild("test")
    build.items = { ring1: 1, ring2: 2 }
    expect(targetPosition(meta, build, { id: 3, slot: "ring", typeId: 9 })).toBe("ring1")
  })

  it("takes the preferred place when it is free, and replaces it when none is", () => {
    const build = newBuild("test")
    expect(targetPosition(meta, build, { id: 3, slot: "ring", typeId: 9 }, "ring2")).toBe("ring2")
    build.items = { ring2: 2 }
    expect(targetPosition(meta, build, { id: 3, slot: "ring", typeId: 9 }, "ring2")).toBe("ring1")
    build.items = { ring1: 1, ring2: 2 }
    expect(targetPosition(meta, build, { id: 3, slot: "ring", typeId: 9 }, "ring2")).toBe("ring2")
    expect(targetPosition(meta, build, { id: 1, slot: "ring", typeId: 9 }, "ring2")).toBe("ring1")
  })
})
