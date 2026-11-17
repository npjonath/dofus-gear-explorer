import { confirm } from "@ui-sdk/react/alert-dialog"
import { Button } from "@ui-sdk/react/button"
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon, ClipboardPasteIcon, CopyIcon, PlusIcon, Trash2Icon } from "lucide-react"
import * as React from "react"
import type { Meta } from "../../api/types"
import { useI18n } from "../../i18n"
import { fold } from "../../lib/text"
import { newBuild, useStore, type Build } from "../../store"
import { ImportBuildDialog } from "./ImportBuildDialog"

type SortKey = "name" | "breed" | "level" | "items" | "updated"

// Rows shown at once; the rest come 50 at a time.
const PAGE = 50

// Every saved stuff, in a table the player searches and sorts. Opening one
// puts it in a tab.
export function BuildList({ meta, onNew }: { meta: Meta; onNew(): void }) {
  const { t, colon, locale } = useI18n()
  const builds = useStore((s) => s.builds)
  const openIds = useStore((s) => s.openBuildIds)
  const { selectBuild, addBuild, removeBuild } = useStore.getState()
  const [query, setQuery] = React.useState("")
  const [sort, setSort] = React.useState<{ key: SortKey; desc: boolean }>({ key: "updated", desc: true })
  const [limit, setLimit] = React.useState(PAGE)
  const [importing, setImporting] = React.useState(false)

  const breedName = React.useCallback(
    (build: Build) => meta.breeds.find((b) => b.id === build.breedId)?.name ?? "",
    [meta]
  )
  const date = React.useMemo(() => new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }), [locale])

  const rows = React.useMemo(() => {
    const needle = fold(query.trim())
    const value = (build: Build): string | number => {
      switch (sort.key) {
        case "name":
          return fold(build.name)
        case "breed":
          return fold(breedName(build))
        case "level":
          return build.level
        case "items":
          return Object.keys(build.items).length
        default:
          return build.updatedAt
      }
    }
    return builds
      .filter((build) => !needle || fold(build.name).includes(needle) || fold(breedName(build)).includes(needle))
      .sort((a, b) => {
        const va = value(a)
        const vb = value(b)
        const order = typeof va === "number" && typeof vb === "number" ? va - vb : String(va).localeCompare(String(vb))
        return sort.desc ? -order : order
      })
  }, [builds, query, sort, breedName])

  function duplicate(build: Build) {
    addBuild({
      ...newBuild(t("build.copyOf", { name: build.name })),
      breedId: build.breedId,
      level: build.level,
      base: build.base,
      scrolls: build.scrolls,
      items: build.items,
      mods: build.mods,
    })
  }

  async function remove(build: Build) {
    const yes = await confirm({
      title: t("build.deleteTitle", { name: build.name }),
      description: t("build.deleteBody"),
      confirmLabel: t("build.delete"),
      cancelLabel: t("build.cancel"),
    })
    if (yes) removeBuild(build.id, t("build.defaultName"))
  }

  const header = (key: SortKey, label: string, align?: "left") => {
    const active = sort.key === key
    const Icon = !active ? ArrowUpDownIcon : sort.desc ? ArrowDownIcon : ArrowUpIcon
    return (
      <th data-align={align} aria-sort={active ? (sort.desc ? "descending" : "ascending") : "none"}>
        <button
          type="button"
          className="th-sort"
          onClick={() => {
            setSort({ key, desc: active ? !sort.desc : key === "updated" || key === "level" })
            setLimit(PAGE)
          }}
        >
          {label}
          <Icon aria-hidden="true" />
        </button>
      </th>
    )
  }

  return (
    <section className="flex h-full min-h-0 flex-col bg-panel" aria-label={t("build.saved")}>
      <div className="flex flex-wrap items-center gap-2 border-b border-line px-4 py-2.5">
        <input
          type="search"
          className="text-input max-w-80 min-w-40 flex-1"
          aria-label={t("build.search")}
          placeholder={t("build.search")}
          value={query}
          onChange={(event) => {
            setQuery(event.target.value)
            setLimit(PAGE)
          }}
        />
        <span className="flex-1 text-ink-2">{t("build.count", { count: rows.length, total: builds.length })}</span>
        <Button onClick={() => setImporting(true)}>
          <ClipboardPasteIcon aria-hidden="true" className="size-4" />
          {t("build.import")}
        </Button>
        <Button data-variant="solid" onClick={onNew}>
          <PlusIcon aria-hidden="true" className="size-4" />
          {t("build.new")}
        </Button>
      </div>
      <ImportBuildDialog meta={meta} open={importing} onOpenChange={setImporting} />
      <div className="min-h-0 flex-1 overflow-auto">
        <table className="grid-table build-list">
          <thead>
            <tr>
              {header("name", t("build.name"), "left")}
              {header("breed", t("build.breed"), "left")}
              {header("level", t("build.col.level"))}
              {header("items", t("build.col.items"))}
              {header("updated", t("build.col.updated"))}
              <th>
                <span className="sr-only">{t("build.col.actions")}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.slice(0, limit).map((build) => (
              <tr key={build.id} onClick={() => selectBuild(build.id)}>
                <td>
                  <button
                    type="button"
                    className="build-list-open"
                    onClick={(event) => {
                      event.stopPropagation()
                      selectBuild(build.id)
                    }}
                  >
                    <span className="truncate">{build.name || t("build.unnamed")}</span>
                    {openIds.includes(build.id) && <span className="tag">{t("build.isOpen")}</span>}
                  </button>
                </td>
                <td data-align="left">{breedName(build) || <span className="text-ink-3">–</span>}</td>
                <td>{build.level}</td>
                <td>
                  {Object.keys(build.items).length}
                  <span className="text-ink-3"> / {meta.positions.length}</span>
                </td>
                <td className="text-ink-2">{date.format(build.updatedAt)}</td>
                <td>
                  <span className="inline-flex gap-0.5">
                    <button
                      type="button"
                      className="icon-btn"
                      aria-label={`${t("build.duplicate")}${colon}${build.name}`}
                      title={t("build.duplicate")}
                      onClick={(event) => {
                        event.stopPropagation()
                        duplicate(build)
                      }}
                    >
                      <CopyIcon aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      className="icon-btn"
                      aria-label={`${t("build.delete")}${colon}${build.name}`}
                      title={t("build.delete")}
                      onClick={(event) => {
                        event.stopPropagation()
                        void remove(build)
                      }}
                    >
                      <Trash2Icon aria-hidden="true" />
                    </button>
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && <p className="m-0 p-6 text-ink-2">{t("build.noMatch")}</p>}
        {rows.length > limit && (
          <div className="flex flex-wrap items-center justify-center gap-3 p-4">
            <span className="text-ink-2">{t("build.shown", { count: limit, total: rows.length })}</span>
            <Button onClick={() => setLimit(limit + PAGE)}>{t("build.showMore", { count: PAGE })}</Button>
          </div>
        )}
      </div>
    </section>
  )
}
