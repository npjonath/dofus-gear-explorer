import type { Lang } from "../api/types"
import { en } from "./en"
import { es } from "./es"
import { fr } from "./fr"
import { pt } from "./pt"
import type { LocaleData } from "./types"

export type { MessageKey, Messages } from "./fr"
export type { LocaleData, Plural } from "./types"

/** The interface languages, in the order of the language switch. */
export const languages: readonly Lang[] = ["fr", "en", "es", "pt"]

export const locales: Record<Lang, LocaleData> = { fr, en, es, pt }

export function isLang(value: string | null | undefined): value is Lang {
  return value != null && (languages as readonly string[]).includes(value)
}

/** The first supported language among the browser's, English otherwise. */
export function pickLang(tags: readonly string[]): Lang {
  for (const tag of tags) {
    const base = tag.toLowerCase().split("-")[0] ?? ""
    if (isLang(base)) return base
  }
  return "en"
}
