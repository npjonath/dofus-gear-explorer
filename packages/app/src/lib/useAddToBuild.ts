import * as React from "react"
import type { Item, ItemMods, Meta } from "../api/types"
import { useI18n } from "../i18n"
import { useStore } from "../store"
import { pickBuild } from "../ui/BuildPicker"
import { toaster } from "../ui/toaster"
import { targetPosition } from "./equip"
import { navigate } from "./router"

// Adds an item to a stuff the player picks, replacing what holds its place.
export function useAddToBuild(meta: Meta) {
  const { t } = useI18n()
  return React.useCallback(
    async (item: Item, from?: { position?: string; mods?: ItemMods }) => {
      const picked = await pickBuild({
        title: t("item.addTitle"),
        hint: t("item.addHint", { item: item.name }),
        confirmLabel: t("item.addConfirm"),
      })
      if (!picked) return
      // The stuff is read again: it may have changed while the dialog was open.
      const state = useStore.getState()
      const build = state.builds.find((b) => b.id === picked.id)
      if (!build) return
      state.equip(build.id, targetPosition(meta, build, item, from?.position), item.id, from?.mods)
      toaster(t("item.added", { item: item.name, build: build.name }), {
        action: {
          label: t("nav.build"),
          onAction: () => {
            useStore.getState().selectBuild(build.id)
            navigate("build")
          },
        },
      })
    },
    [meta, t]
  )
}
