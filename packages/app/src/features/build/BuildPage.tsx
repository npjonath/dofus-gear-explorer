import { confirm } from "@ui-sdk/react/alert-dialog"
import { Menu, MenuContent, MenuContextTrigger, MenuItem } from "@ui-sdk/react/menu"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@ui-sdk/react/select"
import { SegmentedControl, SegmentedControlIndicator, SegmentedControlItem } from "@ui-sdk/react/segmented-control"
import { Tabs, TabsList, TabsPanel, TabsTab } from "@ui-sdk/react/tabs"
import {
  CheckIcon,
  CopyIcon,
  LinkIcon,
  SparklesIcon,
  Trash2Icon,
  TriangleAlertIcon,
  XIcon,
} from "lucide-react"
import * as React from "react"
import { useEvaluation } from "../../api/queries"
import type { Evaluation, Item, Meta, Position, PrimaryKey, Stat } from "../../api/types"
import { useI18n } from "../../i18n"
import { hrefFor, useLocation } from "../../lib/router"
import { encodeBuild } from "../../lib/share"
import { statIndex } from "../../lib/stats"
import { useBreakpoint } from "../../lib/useBreakpoint"
import { usePlatform } from "../../platform"
import { isEmptyMods, newBuild, toBuildInput, useActiveBuild, useStore, type Build } from "../../store"
import { ItemCell, ItemImage } from "../../ui/ItemCell"
import { SolverPage } from "../solver/SolverPage"
import { ItemHover } from "../../ui/ItemHover"
import { ItemSummary } from "../../ui/ItemSummary"
import { NumberField } from "../../ui/NumberField"
import { SidePanels } from "../../ui/SidePanels"
import { SlotGlyph } from "../../ui/SlotGlyph"
import { toaster } from "../../ui/toaster"
import { BuildList } from "./BuildList"
import { BuildTabs } from "./BuildTabs"
import { DamagePanel } from "./DamagePanel"
import { ForgePanel } from "./ForgePanel"
import { ItemPicker } from "./ItemPicker"
import { NewBuildDialog } from "./NewBuildDialog"
import { StatsPanel } from "./StatsPanel"

type SlotTab = "items" | "forge"

const leftColumn = ["hat", "cloak", "amulet", "ring1", "ring2"]
const rightColumn = ["weapon", "shield", "belt", "boots", "pet"]
const bottomRow = ["dofus1", "dofus2", "dofus3", "dofus4", "dofus5", "dofus6"]

// Set markers stay clear of the blue, green and red that mean selected, valid and failing.
const setHues = [305, 85, 195, 340, 60, 280]

type SlotAction = "pick" | "forge" | "remove"

const removeKeys = ["Delete", "Backspace"]

// Wraps a control that stands for a worn item: a right-click opens the
// quick actions and Delete removes the item. An empty slot keeps the
// browser's own menu.
function ItemMenu({
  item,
  onAction,
  trigger,
  children,
}: {
  item: Item | undefined
  onAction(action: SlotAction): void
  trigger: React.ReactElement<React.ButtonHTMLAttributes<HTMLButtonElement>>
  children: React.ReactNode
}) {
  const { t } = useI18n()
  return (
    <Menu onAction={(value) => onAction(value as SlotAction)}>
      <MenuContextTrigger
        disabled={!item}
        render={React.cloneElement(trigger, {
          onKeyDown: (event: React.KeyboardEvent<HTMLButtonElement>) => {
            if (item && removeKeys.includes(event.key)) {
              event.preventDefault()
              onAction("remove")
            }
          },
        })}
      >
        {children}
      </MenuContextTrigger>
      <MenuContent>
        <MenuItem value="pick">{t("build.menu.change")}</MenuItem>
        {item?.forgeable && <MenuItem value="forge">{t("forge.title")}</MenuItem>}
        <MenuItem value="remove">
          {t("build.unequip")}
          <span className="ml-auto pl-4 text-ink-3">{t("build.menu.removeKey")}</span>
        </MenuItem>
      </MenuContent>
    </Menu>
  )
}

function SlotButton({
  position,
  evaluation,
  selected,
  forged,
  stats,
  onSelect,
  onAction,
}: {
  position: Position
  evaluation: Evaluation | undefined
  selected: boolean
  forged: boolean
  stats: Map<number, Stat>
  onSelect(): void
  onAction(action: SlotAction): void
}) {
  const { t, slotName } = useI18n()
  const item = evaluation?.items[position.key]
  const invalid =
    evaluation?.conditions.some((c) => c.position === position.key && !c.ok) ||
    evaluation?.issues.some((i) => i.position === position.key)
  const setIndex = item?.set ? evaluation!.sets.findIndex((s) => s.id === item.set!.id && s.count > 1) : -1

  const menu = (
    <ItemMenu
      item={item}
      onAction={onAction}
      trigger={
        <button
          type="button"
          className="slot"
          aria-pressed={selected}
          aria-label={
            item
              ? t("build.filledSlot", { slot: slotName(position.slot), item: item.name })
              : t("build.emptySlot", { slot: slotName(position.slot) })
          }
          title={item ? undefined : slotName(position.slot)}
          data-invalid={invalid ? "" : undefined}
          data-filled={item ? "" : undefined}
          onClick={onSelect}
        />
      }
    >
      {item ? (
        <>
          <ItemImage key={item.id} item={item} />
          <span className="slot-level">{item.level}</span>
          {setIndex >= 0 && (
            <span
              className="slot-set"
              style={{ "--set-color": `oklch(0.6 0.15 ${setHues[setIndex % setHues.length]})` } as React.CSSProperties}
            />
          )}
          {forged && <SparklesIcon className="slot-forged" aria-label={t("forge.title")} />}
        </>
      ) : (
        <SlotGlyph slot={position.slot} />
      )}
    </ItemMenu>
  )
  if (!item) return menu
  return (
    <ItemHover item={item} stats={stats} forged={evaluation?.forge[position.key]} side="right">
      <span className="inline-flex">{menu}</span>
    </ItemHover>
  )
}

function CharacterPanel({ meta, build, evaluation }: { meta: Meta; build: Build; evaluation: Evaluation | undefined }) {
  const { t, primaryName } = useI18n()
  const updateBuild = useStore((s) => s.updateBuild)
  const points = evaluation?.points
  const left = points ? points.total - points.spent : 0

  const passives = meta.positions
    .map((position) => ({ position: position.key, item: evaluation?.items[position.key] }))
    .filter((entry) => entry.item?.passive)
    .map((entry) => ({ position: entry.position, item: entry.item! }))

  const setStat = (field: "base" | "scrolls", key: PrimaryKey, value: number) =>
    updateBuild(build.id, { [field]: { ...build[field], [key]: value } })

  return (
    <div className="character-panel">
      <div className="flex items-center justify-between gap-3">
        <span className="font-semibold">{t("build.level")}</span>
        <NumberField
          label={t("build.level")}
          value={build.level}
          min={1}
          max={meta.maxLevel}
          onChange={(level) => updateBuild(build.id, { level })}
        />
      </div>

      <div>
        <div className="mb-1.5 flex items-baseline justify-between gap-2">
          <h3 className="section-title">{t("build.points")}</h3>
          {points && (
            <span className={left < 0 ? "font-semibold text-bad" : "text-ink-2"}>
              {left < 0 ? t("build.pointsOver", { over: -left }) : t("build.pointsLeft", { left, total: points.total })}
            </span>
          )}
        </div>
        <table className="w-full border-separate border-spacing-y-1">
          <thead>
            <tr className="text-[0.86rem] text-ink-2">
              <th scope="col" className="text-left font-semibold" />
              <th scope="col" className="w-[4.9rem] text-right font-semibold">
                {t("build.base")}
              </th>
              <th scope="col" className="w-[4.9rem] text-right font-semibold">
                {t("build.scroll")}
              </th>
            </tr>
          </thead>
          <tbody>
            {meta.primaryStats.map(({ key }) => (
              <tr key={key}>
                <th scope="row" className="text-left font-medium text-ink-2">
                  {primaryName(key)}
                </th>
                <td className="w-[4.9rem]">
                  <div className="flex justify-end">
                    <NumberField
                      label={`${primaryName(key)}, ${t("build.base")}`}
                      value={build.base[key]}
                      max={995}
                      invalid={left < 0}
                      onChange={(value) => setStat("base", key, value)}
                    />
                  </div>
                </td>
                <td className="w-[4.9rem]">
                  <div className="flex justify-end">
                    <NumberField
                      label={`${primaryName(key)}, ${t("build.scroll")}`}
                      value={build.scrolls[key]}
                      max={100}
                      onChange={(value) => setStat("scrolls", key, value)}
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div>
        <h3 className="section-title mb-1.5">{t("build.sets")}</h3>
        {evaluation && evaluation.sets.some((s) => s.count > 1) ? (
          <ul className="m-0 grid list-none gap-2 p-0">
            {evaluation.sets
              .filter((s) => s.count > 1)
              .map((set, index) => (
                <li key={set.id}>
                  <span className="flex items-center gap-2 font-semibold">
                    <span
                      className="pip"
                      style={{ background: `oklch(0.6 0.15 ${setHues[index % setHues.length]})` }}
                    />
                    {set.name}
                    <span className="font-medium text-ink-3">
                      {set.count}/{set.size}
                    </span>
                  </span>
                  <span className="text-ink-2">{set.bonuses.map((b) => b.text).join(" · ")}</span>
                </li>
              ))}
          </ul>
        ) : (
          <p className="m-0 text-ink-2">{t("build.noSets")}</p>
        )}
      </div>

      {passives.length > 0 && (
        <div>
          <h3 className="section-title mb-1.5">{t("build.passives")}</h3>
          <ul className="m-0 grid list-none gap-2.5 p-0">
            {passives.map(({ position, item }) => (
              <li key={position} className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-2.5">
                <ItemCell item={item} size="2.25rem" />
                <span>
                  <span className="block font-semibold">{item.passive!.name}</span>
                  <span className="text-ink-2">{item.passive!.description}</span>
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

// Every worn item with its lines as they count in the stuff, forgemagic
// included, so the whole stuff reads without opening each slot.
function WornItems({
  meta,
  evaluation,
  selectedKey,
  onSelect,
  onAction,
}: {
  meta: Meta
  evaluation: Evaluation
  selectedKey: string | null
  onSelect(key: string): void
  onAction(key: string, action: SlotAction): void
}) {
  const { t } = useI18n()
  const stats = React.useMemo(() => statIndex(meta), [meta])
  const worn = meta.positions.filter((position) => evaluation.items[position.key])
  if (worn.length === 0) return null

  return (
    <section className="worn-items" aria-label={t("build.worn")}>
      {worn.map((position) => {
        const item = evaluation.items[position.key]!
        return (
          <ItemMenu
            key={position.key}
            item={item}
            onAction={(action) => onAction(position.key, action)}
            trigger={
              <button
                type="button"
                className="worn-item"
                aria-pressed={selectedKey === position.key}
                onClick={() => onSelect(position.key)}
              />
            }
          >
            <ItemSummary item={item} stats={stats} forged={evaluation.forge[position.key]} />
          </ItemMenu>
        )
      })}
    </section>
  )
}

function Problems({ evaluation }: { evaluation: Evaluation }) {
  const { t } = useI18n()
  const failed = evaluation.conditions.filter((c) => !c.ok)
  if (failed.length === 0 && evaluation.issues.length === 0) return null
  return (
    <ul className="m-0 grid w-full max-w-(--board-center,25rem) list-none gap-1 rounded-lg border border-bad bg-bad-soft p-3" role="alert">
      {evaluation.issues.map((issue, index) => (
        <li key={`issue-${index}`}>
          {t(`build.issue.${issue.code}`, { item: evaluation.items[issue.position ?? ""]?.name ?? "" })}
        </li>
      ))}
      {failed.map((condition) => (
        <li key={`${condition.position}-${condition.itemId}`}>
          <strong>{evaluation.items[condition.position]?.name}</strong> : {condition.text}
        </li>
      ))}
    </ul>
  )
}

export function BuildPage({ meta }: { meta: Meta }) {
  const { t, slotName } = useI18n()
  const build = useActiveBuild()
  const platform = usePlatform()
  const shareByLink = platform.kind === "web" && window.location.protocol.startsWith("http")
  const stuffTab = useStore((s) => s.stuffTab)
  const [creating, setCreating] = React.useState(false)
  // The search keeps its form and results while other tabs show.
  const [searchSeen, setSearchSeen] = React.useState(stuffTab === "search")
  if (stuffTab === "search" && !searchSeen) setSearchSeen(true)
  const { addBuild, updateBuild, removeBuild, equip, setMods } = useStore.getState()
  const breakpoint = useBreakpoint()
  const [selectedKey, setSelectedKey] = React.useState<string | null>(null)
  const [selectedIn, setSelectedIn] = React.useState(build.id)
  const [tab, setTab] = React.useState<SlotTab>("items")
  const [view, setView] = React.useState<"gear" | "damage">("gear")
  // A slot chosen in one stuff says nothing about the next one.
  if (selectedIn !== build.id) {
    setSelectedIn(build.id)
    setSelectedKey(null)
  }

  const input = React.useMemo(() => toBuildInput(build), [build])
  const evaluation = useEvaluation(input).data
  const positions = new Map(meta.positions.map((p) => [p.key, p]))
  const stats = React.useMemo(() => statIndex(meta), [meta])
  const selected = selectedKey ? (positions.get(selectedKey) ?? null) : null
  const selectedItem = selectedKey ? evaluation?.items[selectedKey] : undefined
  const canForge = Boolean(selectedItem?.forgeable)
  const activeTab: SlotTab = tab === "forge" && canForge ? "forge" : "items"

  // Ctrl+Z steps the items back, Ctrl+Shift+Z or Ctrl+Y forward. Text
  // fields keep their own undo.
  const visible = useLocation().route === "build"
  React.useEffect(() => {
    if (!visible) return
    const onKeyDown = (event: KeyboardEvent) => {
      const key = event.key.toLowerCase()
      if (!(event.ctrlKey || event.metaKey) || (key !== "z" && key !== "y")) return
      const target = event.target as HTMLElement | null
      if (target?.closest("input, textarea, [contenteditable]")) return
      event.preventDefault()
      const redo = key === "y" || event.shiftKey
      const done = redo ? useStore.getState().redo(build.id) : useStore.getState().undo(build.id)
      if (done) toaster(t(redo ? "build.redone" : "build.undone"))
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [visible, build.id, t])

  const act = (key: string, action: SlotAction) => {
    if (action === "remove") {
      const removed = evaluation?.items[key]
      equip(build.id, key, null)
      if (removed) {
        toaster(t("build.removed", { item: removed.name }), {
          action: { label: t("build.undo"), onAction: () => void useStore.getState().undo(build.id) },
        })
      }
      return
    }
    setSelectedKey(key)
    setTab(action === "forge" ? "forge" : "items")
  }

  const slot = (key: string) => {
    const position = positions.get(key)
    if (!position) return null
    return (
      <SlotButton
        key={key}
        position={position}
        evaluation={evaluation}
        selected={selectedKey === key}
        forged={!isEmptyMods(build.mods[key])}
        stats={stats}
        onSelect={() => {
          setSelectedKey(selectedKey === key ? null : key)
          setTab("items")
        }}
        onAction={(action) => act(key, action)}
      />
    )
  }

  // The web version's address opens the stuff for anyone; the desktop app's
  // address is a local file, so it shares the code that "Import" reads.
  async function share() {
    const code = encodeBuild(build)
    try {
      if (shareByLink) {
        await navigator.clipboard.writeText(`${window.location.origin}${window.location.pathname}${hrefFor("build", { b: code })}`)
        toaster(t("build.shared"))
      } else {
        await navigator.clipboard.writeText(code)
        toaster(t("build.sharedCode"), { description: t("build.sharedCodeHint") })
      }
    } catch {
      toaster(t("build.shareFailed"))
    }
  }

  async function remove() {
    const yes = await confirm({
      title: t("build.deleteTitle", { name: build.name }),
      description: t("build.deleteBody"),
      confirmLabel: t("build.delete"),
      cancelLabel: t("build.cancel"),
    })
    if (yes) removeBuild(build.id, t("build.defaultName"))
  }

  const board = (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col lg:h-full lg:overflow-y-auto">
      <div className="flex flex-wrap items-center gap-2 border-b border-line bg-panel px-4 py-2">
        <input
          className="text-input min-w-40 flex-1 font-semibold"
          aria-label={t("build.name")}
          value={build.name}
          maxLength={80}
          onChange={(event) => updateBuild(build.id, { name: event.target.value })}
        />
        <Select
          value={build.breedId === null ? null : String(build.breedId)}
          placeholder={t("build.breed")}
          onValueChange={(id) => updateBuild(build.id, { breedId: id ? Number(id) : null })}
        >
          <SelectTrigger aria-label={t("build.breed")} className="w-auto! min-w-32 justify-between">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {meta.breeds.map((breed) => (
              <SelectItem key={breed.id} value={String(breed.id)} label={breed.name}>
                {breed.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {evaluation && (
          <span className="status px-1" data-state={evaluation.valid ? "ok" : "bad"}>
            {evaluation.valid ? <CheckIcon aria-hidden="true" /> : <TriangleAlertIcon aria-hidden="true" />}
            {evaluation.valid ? t("build.valid") : t("build.invalid")}
          </span>
        )}
        <SegmentedControl
          aria-label={t("build.view")}
          value={view}
          onValueChange={(next) => setView(next === "damage" ? "damage" : "gear")}
        >
          <SegmentedControlIndicator />
          <SegmentedControlItem value="gear">{t("build.view.gear")}</SegmentedControlItem>
          <SegmentedControlItem value="damage">{t("build.view.damage")}</SegmentedControlItem>
        </SegmentedControl>
        <button
          type="button"
          className="icon-btn"
          aria-label={t("build.duplicate")}
          title={t("build.duplicate")}
          onClick={() =>
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
        >
          <CopyIcon aria-hidden="true" />
        </button>
        <button
          type="button"
          className="icon-btn"
          aria-label={t(shareByLink ? "build.share" : "build.shareCode")}
          title={t(shareByLink ? "build.share" : "build.shareCode")}
          onClick={share}
        >
          <LinkIcon aria-hidden="true" />
        </button>
        <button type="button" className="icon-btn" aria-label={t("build.delete")} title={t("build.delete")} onClick={remove}>
          <Trash2Icon aria-hidden="true" />
        </button>
      </div>

      {view === "damage" ? (
        <DamagePanel meta={meta} build={build} input={input} />
      ) : (
        <div className="board grid flex-1 grid-cols-[minmax(0,1fr)] content-start justify-items-center gap-4 p-4 sm:p-6">
          <div className="board-grid w-full">
            <div className="board-col">{leftColumn.map(slot)}</div>
            <div className="board-center grid justify-items-center gap-3">
              <CharacterPanel meta={meta} build={build} evaluation={evaluation} />
              {evaluation && <Problems evaluation={evaluation} />}
            </div>
            <div className="board-col">{rightColumn.map(slot)}</div>
          </div>
          <div className="flex flex-wrap justify-center gap-2">{bottomRow.map(slot)}</div>
          {evaluation && (
            <WornItems
              meta={meta}
              evaluation={evaluation}
              selectedKey={selectedKey}
              onSelect={(key) => {
                setSelectedKey(key)
                setTab("items")
              }}
              onAction={act}
            />
          )}
        </div>
      )}
    </section>
  )

  const statsPanel = (
    <div className="h-full overflow-y-auto bg-panel">
      <StatsPanel meta={meta} evaluation={evaluation} />
    </div>
  )

  const slotPanel = selected ? (
    <Tabs
      value={activeTab}
      onValueChange={(next) => setTab(next === "forge" ? "forge" : "items")}
      className="flex h-full min-h-0 flex-col bg-panel"
    >
      <div className="flex items-center gap-2 border-b border-line px-4 pt-2">
        <TabsList aria-label={slotName(selected.slot)} className="slot-tabs flex-1">
          <TabsTab value="items">{t("build.tab.items")}</TabsTab>
          <TabsTab value="forge" disabled={!canForge}>
            {t("forge.title")}
          </TabsTab>
        </TabsList>
        <button
          type="button"
          className="icon-btn"
          aria-label={t("build.closePicker")}
          onClick={() => setSelectedKey(null)}
        >
          <XIcon aria-hidden="true" />
        </button>
      </div>
      <TabsPanel value="items" className="min-h-0 flex-1 pt-0!">
        <ItemPicker
          meta={meta}
          build={build}
          position={selected}
          current={selectedItem}
          onEquip={(item) => equip(build.id, selected.key, item ? item.id : null)}
        />
      </TabsPanel>
      <TabsPanel value="forge" className="min-h-0 flex-1 overflow-y-auto pt-0!">
        {selectedItem && canForge && (
          <>
            <header className="flex items-center gap-2.5 border-b border-line p-4">
              <ItemCell item={selectedItem} size="2.5rem" />
              <span className="min-w-0">
                <span className="block truncate font-semibold">{selectedItem.name}</span>
                <span className="text-[0.86rem] text-ink-3">
                  {selectedItem.typeName} · {t("item.level", { level: selectedItem.level })}
                </span>
              </span>
            </header>
            <ForgePanel
              meta={meta}
              item={selectedItem}
              position={selected.key}
              mods={build.mods[selected.key]}
              evaluation={evaluation}
              onChange={(mods) => setMods(build.id, selected.key, mods)}
            />
          </>
        )}
      </TabsPanel>
    </Tabs>
  ) : undefined

  // Side panels only show when they have something to say: the
  // characteristics beside a stuff, the picker once a slot is chosen.
  const showSlot = selected !== null && view === "gear"
  let workspace: React.ReactNode
  if (stuffTab === "list") {
    workspace = <BuildList meta={meta} onNew={() => setCreating(true)} />
  } else if (stuffTab === "search") {
    workspace = null
  } else if (breakpoint === "sm") {
    workspace = (
      <div className="h-full overflow-y-auto">
        {board}
        <div className="border-t border-line">{statsPanel}</div>
        {showSlot && (
          <div className="fixed inset-x-0 bottom-0 z-20 h-[72dvh] border-t border-line bg-panel shadow-[var(--shadow-pop)]">
            {slotPanel}
          </div>
        )}
      </div>
    )
  } else {
    workspace = (
      <SidePanels
        id="build"
        left={breakpoint === "xl" ? statsPanel : undefined}
        leftSize={20}
        right={showSlot ? slotPanel : undefined}
      >
        {breakpoint === "xl" ? (
          board
        ) : (
          <div className="flex h-full min-h-0 flex-col">
            {board}
            <div className="max-h-[32%] min-h-0 flex-none overflow-hidden border-t border-line">{statsPanel}</div>
          </div>
        )}
      </SidePanels>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <BuildTabs meta={meta} onNew={() => setCreating(true)} />
      <div className="min-h-0 flex-1" hidden={stuffTab === "search"}>
        {workspace}
      </div>
      {searchSeen && (
        <div className="min-h-0 flex-1" hidden={stuffTab !== "search"}>
          <SolverPage meta={meta} />
        </div>
      )}
      <NewBuildDialog meta={meta} open={creating} onOpenChange={setCreating} />
    </div>
  )
}
