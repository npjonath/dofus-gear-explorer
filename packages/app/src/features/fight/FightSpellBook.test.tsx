import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, expect, it, vi } from "vitest"
import type { FightBookSpell } from "../../api/types"
import { PlatformProvider } from "../../platform"
import { useStore } from "../../store"
import { FightSpellBook } from "./FightSpellBook"

afterEach(cleanup)

it("keeps variant partners visible and previews the spell matching a new search", () => {
  useStore.setState({ lang: "fr" })
  const query = new QueryClient()
  const spells: FightBookSpell[] = [
    { id: 1, variantOf: 0, name: "Réflexes", description: "Fiche Réflexes", tags: ["air"], breedId: 6, levels: [1], iconId: 1 },
    {
      id: 2,
      variantOf: 1,
      name: "Lapement",
      description: "Fiche Lapement",
      tags: ["earth"],
      breedId: 6,
      levels: [95],
      iconId: 2,
    },
  ]
  query.setQueryData(["fight-book", "http://api.test", "fr", 6, "test"], spells)
  const onSelect = vi.fn()
  render(
    <PlatformProvider platform={{ kind: "web", apiBaseUrl: "http://api.test", version: "1" }}>
      <QueryClientProvider client={query}>
        <FightSpellBook
          breedId={6}
          breedName="Ecaflip"
          level={1}
          slots={[]}
          selected={new Set()}
          onSelect={onSelect}
          sheet={() => null}
          pack="test"
        />
      </QueryClientProvider>
    </PlatformProvider>
  )
  fireEvent.click(screen.getByRole("button", { name: /Lapement/ }))
  expect(screen.getByRole("region", { name: "Lapement" })).toBeDefined()
  expect(onSelect).not.toHaveBeenCalled()
  const search = screen.getByRole("searchbox", { name: "Rechercher un sort" })
  fireEvent.change(search, { target: { value: "reflexes" } })
  expect(screen.getByRole("button", { name: /Lapement/ })).toBeDefined()
  expect(screen.getByRole("region", { name: "Réflexes" })).toBeDefined()
  fireEvent.change(search, { target: { value: "introuvable" } })
  expect(screen.getByRole("status").textContent).toBe("Aucun sort ne correspond.")
  expect(screen.queryByRole("region")).toBeNull()
})
