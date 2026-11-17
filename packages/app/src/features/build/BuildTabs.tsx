import { Tabs, TabsList, TabsTab } from "@ui-sdk/react/tabs"
import { LayoutListIcon, PlusIcon, SearchIcon, XIcon } from "lucide-react"
import * as React from "react"
import type { Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { useStore } from "../../store"

// The name a new stuff gets: "Stuff 3" when two exist.
export function nextBuildName(template: (n: number) => string, taken: string[]): string {
  for (let n = taken.length + 1; ; n++) {
    const name = template(n)
    if (!taken.includes(name)) return name
  }
}

// The Stuff screen's tabs: the list of stuffs and the stuff search, which
// never close, then one tab per open stuff. Closing a tab keeps the stuff.
export function BuildTabs({ meta, onNew }: { meta: Meta; onNew(): void }) {
  const { t } = useI18n()
  const builds = useStore((s) => s.builds)
  const openIds = useStore((s) => s.openBuildIds)
  const activeId = useStore((s) => s.activeBuildId)
  const stuffTab = useStore((s) => s.stuffTab)
  const { selectBuild, closeBuild, showTab } = useStore.getState()
  const open = openIds.map((id) => builds.find((b) => b.id === id)).filter((b) => b !== undefined)
  const breedName = (id: number | null) => meta.breeds.find((b) => b.id === id)?.name

  // With more tabs than the bar holds, the list scrolls: keep the active one in view.
  const bar = React.useRef<HTMLDivElement>(null)
  React.useEffect(() => {
    bar.current
      ?.querySelector(".build-tab[data-selected], .build-tabs-list > [data-selected]")
      ?.scrollIntoView({ block: "nearest", inline: "nearest" })
  }, [stuffTab, activeId, openIds.length])

  return (
    <div className="build-tabs" ref={bar}>
      <Tabs
        value={stuffTab === "build" ? activeId : stuffTab}
        onValueChange={(id) => {
          if (id === "list" || id === "search") showTab(id)
          else if (typeof id === "string") selectBuild(id)
        }}
        className="min-w-0"
      >
        <TabsList
          aria-label={t("nav.build")}
          className="build-tabs-list"
          // A mouse wheel scrolls the tabs sideways, as it does in a browser.
          onWheel={(event) => {
            if (event.deltaY !== 0 && event.deltaX === 0) event.currentTarget.scrollLeft += event.deltaY
          }}
        >
          <TabsTab value="list">
            <LayoutListIcon aria-hidden="true" />
            {t("build.saved")}
          </TabsTab>
          <TabsTab value="search">
            <SearchIcon aria-hidden="true" />
            {t("nav.search")}
          </TabsTab>
          {open.map((b) => (
            <span
              key={b.id}
              className="build-tab"
              data-selected={stuffTab === "build" && b.id === activeId ? "" : undefined}
            >
              <TabsTab value={b.id} title={b.name}>
                <span className="truncate">{b.name || t("build.unnamed")}</span>
                {breedName(b.breedId) && <span className="build-tab-breed">{breedName(b.breedId)}</span>}
              </TabsTab>
              <button
                type="button"
                className="build-tab-close"
                aria-label={`${t("build.closeTab")} : ${b.name || t("build.unnamed")}`}
                title={t("build.closeTab")}
                onClick={() => closeBuild(b.id)}
              >
                <XIcon aria-hidden="true" />
              </button>
            </span>
          ))}
        </TabsList>
      </Tabs>
      <button type="button" className="build-tab-new" aria-label={t("build.new")} title={t("build.new")} onClick={onNew}>
        <PlusIcon aria-hidden="true" />
      </button>
    </div>
  )
}
