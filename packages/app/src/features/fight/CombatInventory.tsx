import { Dialog, DialogBackdrop, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/react/dialog"
import { ArrowLeftIcon, ArrowRightIcon, XIcon } from "lucide-react"
import * as React from "react"
import { useEvaluation } from "../../api/queries"
import type { BuildInput, Evaluation, Item, Meta, Position } from "../../api/types"
import { useI18n } from "../../i18n"
import { modsKey } from "../../lib/equip"
import { statIndex } from "../../lib/stats"
import { useApi } from "../../platform"
import { isEmptyMods, withEquipment, withForge, type Build } from "../../store"
import { ItemImage } from "../../ui/ItemCell"
import { ItemSummary } from "../../ui/ItemSummary"
import { SlotGlyph } from "../../ui/SlotGlyph"
import { ForgePanel } from "../build/ForgePanel"
import { ItemPicker } from "../build/ItemPicker"
import { GameActor, useGameActorCatalog, type GameDirection } from "./GameActor"
import "./CombatInventory.css"

const left = ["hat", "cloak", "weapon", "shield", "pet"]
const right = ["amulet", "ring1", "ring2", "belt", "boots"]
const bottom = ["dofus1", "dofus2", "dofus3", "dofus4", "dofus5", "dofus6"]
type Editor = { position: Position; mode: "items" | "forge" }

interface Props {
  meta: Meta
  build: Build
  original: BuildInput
  active: BuildInput
  draft: BuildInput
  activeEvaluation: Evaluation | undefined
  art: boolean
  busy: boolean
  onDraft(next: BuildInput): void
  onRestart(): void
}

function changed(position: string, before: BuildInput, after: BuildInput) {
  return before.items[position] !== after.items[position] || modsKey(before.mods[position]) !== modsKey(after.mods[position])
}

export function CombatInventory({
  meta,
  build,
  original,
  active,
  draft,
  activeEvaluation,
  art,
  busy,
  onDraft,
  onRestart,
}: Props) {
  const { t, slotName } = useI18n()
  const api = useApi()
  const [editor, setEditor] = React.useState<Editor | null>(null)
  const [confirm, setConfirm] = React.useState(false)
  const [facing, setFacing] = React.useState<GameDirection>(1)
  const evaluation = useEvaluation(draft)
  const preview = draft === active ? (activeEvaluation ?? evaluation.data) : evaluation.data
  const stats = React.useMemo(() => statIndex(meta), [meta])
  const pending = meta.positions.filter((p) => changed(p.key, active, draft))
  const customized = meta.positions.some((p) => changed(p.key, original, active))
  const differentFromSaved = meta.positions.some((p) => changed(p.key, original, draft))
  const hud = (id: string) => api.imageUrl(`/fight-assets/hud/${id}.png`)
  const closeEditor = () => setEditor(null)
  const itemAt = (position: string) => {
    const item = preview?.items[position]
    return item?.id === draft.items[position] ? item : undefined
  }
  const { data: appearances } = useGameActorCatalog(art)
  const missing = appearances
    ? ["hat", "cloak", "shield"]
        .filter((key) => draft.items[key] && !appearances.equipment[draft.items[key]!])
        .map((key) => itemAt(key)?.name)
        .filter(Boolean)
        .join(", ")
    : ""
  const selectedItem = editor ? itemAt(editor.position.key) : undefined
  const editingBuild: Build = { ...build, ...draft }

  function slot(key: string) {
    const position = meta.positions.find((p) => p.key === key)
    if (!position) return null
    const item = itemAt(key)
    const occupied = Boolean(draft.items[key])
    const label = `${slotName(position.slot)} · ${item?.name ?? (occupied ? "…" : t("fight.inventory.empty"))}`
    const forge = () => setEditor({ position, mode: occupied ? "forge" : "items" })
    return (
      <button
        key={key}
        type="button"
        className="combat-gear-slot"
        data-place={key}
        data-empty={!occupied || undefined}
        data-changed={changed(key, active, draft) || undefined}
        title={label}
        aria-label={label}
        onClick={() => setEditor({ position, mode: "items" })}
        onContextMenu={(event) => {
          event.preventDefault()
          forge()
        }}
        onKeyDown={(event) => {
          if (event.key === "ContextMenu" || (event.shiftKey && event.key === "F10")) {
            event.preventDefault()
            forge()
          }
        }}
        style={art && !occupied ? { backgroundImage: `url("${hud("slot-" + position.slot)}")` } : undefined}
      >
        {item ? <ItemImage key={item.id} item={item} /> : <SlotGlyph slot={position.slot} className="combat-empty-glyph" />}
        {!isEmptyMods(draft.mods[key]) && (
          <span className="combat-forged-mark" aria-label={t("forge.title")}>
            ✦
          </span>
        )}
      </button>
    )
  }

  function difference(position: Position) {
    const from = activeEvaluation?.items[position.key]?.name ?? t("fight.inventory.empty")
    const to = itemAt(position.key)?.name ?? (draft.items[position.key] ? "…" : t("fight.inventory.empty"))
    return (
      <li key={position.key}>
        <strong>{slotName(position.slot)}</strong> ·{" "}
        {active.items[position.key] === draft.items[position.key] ? `${to} · ${t("fight.inventory.forged")}` : `${from} → ${to}`}
      </li>
    )
  }

  function choose(item: Item | null) {
    if (editor) onDraft(withEquipment(draft, editor.position.key, item?.id ?? null))
  }

  const layers = art
    ? ({
        "--inventory-bottom": `url("${hud("inventory-bottom")}")`,
        "--inventory-top": `url("${hud("inventory-top")}")`,
        "--inventory-slot": `url("${hud("inventory-slot")}")`,
      } as React.CSSProperties)
    : undefined

  return (
    <div className="combat-paperdoll" style={layers} data-art={art || undefined}>
      <div className="combat-paperdoll-stage">
        <p className="combat-paperdoll-hint">{t("fight.inventory.hint")}</p>
        <div className="combat-paperdoll-model" aria-hidden="true">
          {art && (
            <img
              className="combat-pedestal"
              key={hud("inventory-pedestal-" + (build.breedId ?? 1))}
              onError={(e) => {
                e.currentTarget.style.display = "none"
              }}
              src={hud("inventory-pedestal-" + (build.breedId ?? 1))}
              alt=""
            />
          )}
          {art ? (
            <svg viewBox="-49 -92 98 102" className="combat-paperdoll-actor">
              <GameActor
                kind="player"
                breedId={build.breedId}
                items={draft.items}
                direction={facing}
                pose="idle"
                placement
                scale={0.8}
                resolution={3}
              />
            </svg>
          ) : (
            build.breedId && (
              <img className="combat-paperdoll-head" src={api.imageUrl(`/images/breeds/${build.breedId}.png`)} alt="" />
            )
          )}
        </div>
        <div className="combat-gear-stack combat-gear-left">{left.map(slot)}</div>
        <div className="combat-gear-stack combat-gear-right">{right.map(slot)}</div>
        {art && (
          <div className="combat-inventory-rotate">
            <button
              type="button"
              title={t("fight.inventory.left")}
              aria-label={t("fight.inventory.left")}
              onClick={() => setFacing(((facing + 7) % 8) as GameDirection)}
            >
              <ArrowLeftIcon />
            </button>
            <button
              type="button"
              title={t("fight.inventory.right")}
              aria-label={t("fight.inventory.right")}
              onClick={() => setFacing(((facing + 1) % 8) as GameDirection)}
            >
              <ArrowRightIcon />
            </button>
          </div>
        )}
      </div>
      <div className="combat-gear-dofus">{bottom.map(slot)}</div>
      {(pending.length > 0 || customized || missing) && (
        <div className="combat-gear-actions">
          {missing && <p>{t("fight.inventory.missing", { items: missing })}</p>}
          {pending.length > 0 && (
            <>
              <strong>{t("fight.inventory.pending")}</strong>
              <ul>{pending.map(difference)}</ul>
              <button type="button" onClick={() => onDraft(active)}>
                {t("build.cancel")}
              </button>
              <button type="button" onClick={() => setConfirm(true)} disabled={busy || evaluation.isError}>
                {t("fight.inventory.apply")}
              </button>
            </>
          )}
          {differentFromSaved && customized && (
            <button type="button" onClick={() => onDraft(original)}>
              {t("fight.inventory.restore")}
            </button>
          )}
        </div>
      )}

      <Dialog
        open={editor !== null}
        onOpenChange={(open) => {
          if (!open) closeEditor()
        }}
      >
        <DialogBackdrop />
        <DialogContent className="combat-dialog combat-gear-editor">
          {editor && (
            <>
              <header>
                <DialogTitle>
                  {slotName(editor.position.slot)}
                  {selectedItem ? ` · ${selectedItem.name}` : ""}
                </DialogTitle>
                <button type="button" aria-label={t("fight.inventory.back")} onClick={closeEditor}>
                  <XIcon />
                </button>
              </header>
              <DialogDescription className="combat-trial-hint">{t("fight.inventory.trial")}</DialogDescription>
              <div className="combat-editor-tools">
                <button
                  type="button"
                  aria-pressed={editor.mode === "items"}
                  onClick={() => setEditor({ ...editor, mode: "items" })}
                >
                  {t("fight.inventory.replace")}
                </button>
                <button
                  type="button"
                  aria-pressed={editor.mode === "forge"}
                  disabled={!selectedItem}
                  onClick={() => setEditor({ ...editor, mode: "forge" })}
                >
                  {t("forge.title")}
                </button>
              </div>
              {editor.mode === "items" ? (
                <ItemPicker
                  key={editor.position.key}
                  meta={meta}
                  build={editingBuild}
                  position={editor.position}
                  current={selectedItem}
                  onEquip={choose}
                />
              ) : (
                selectedItem && (
                  <div className="combat-forge-content">
                    {selectedItem.forgeable ? (
                      <ForgePanel
                        meta={meta}
                        item={selectedItem}
                        position={editor.position.key}
                        mods={draft.mods[editor.position.key]}
                        evaluation={preview}
                        onChange={(mods) => onDraft(withForge(draft, editor.position.key, mods))}
                      />
                    ) : (
                      <>
                        <p>{t("fight.inventory.noForge")}</p>
                        <ItemSummary item={selectedItem} stats={stats} />
                      </>
                    )}
                  </div>
                )
              )}
            </>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={confirm} onOpenChange={setConfirm}>
        <DialogBackdrop />
        <DialogContent className="combat-dialog combat-inventory-confirm">
          <DialogTitle>{t("fight.inventory.apply")}</DialogTitle>
          <DialogDescription>{t("fight.inventory.confirm")}</DialogDescription>
          <ul>{pending.map(difference)}</ul>
          <div className="combat-editor-tools">
            <button type="button" onClick={() => setConfirm(false)}>
              {t("build.cancel")}
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => {
                setConfirm(false)
                onRestart()
              }}
            >
              {t("fight.restart")}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
