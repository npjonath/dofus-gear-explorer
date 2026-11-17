import type { Lang, PrimaryKey, Slot } from "./api/types"
import { locales, type MessageKey } from "./locales"
import { useStore } from "./store"

// The dictionaries live in ./locales, one file per language. The French one
// defines the keys; the others are typed against it, so a missing or extra
// key, or a plural form where French has none, fails the typecheck.
export type { MessageKey } from "./locales"

/**
 * Fills a message. A message with plural forms picks one from the `count`
 * parameter, which is also formatted as a number.
 */
export type Translate = (key: MessageKey, params?: Record<string, string | number>) => string

export interface I18n {
  lang: Lang
  /** The BCP 47 tag for Intl formats. */
  locale: string
  t: Translate
  /** Separates a label from its value: "Label : value" in French, "Label: value" elsewhere. */
  colon: string
  slotName(slot: Slot): string
  primaryName(key: PrimaryKey): string
  number(value: number): string
  date(iso: string): string
}

export function createI18n(lang: Lang): I18n {
  const data = locales[lang]
  const numberFormat = new Intl.NumberFormat(data.intl)
  const dateFormat = new Intl.DateTimeFormat(data.intl, { dateStyle: "medium", timeStyle: "short" })
  const plurals = new Intl.PluralRules(data.intl)
  return {
    lang,
    locale: data.intl,
    colon: data.colon,
    t: (key, params) => {
      const message = data.messages[key]
      // Keys built at run time (an issue code from the API, say) may be unknown.
      if (message === undefined) return key
      const count = params?.count
      let text =
        typeof message === "string"
          ? message
          : typeof count === "number" && plurals.select(count) === "one"
            ? message.one
            : message.other
      if (params) {
        for (const [name, value] of Object.entries(params)) {
          const shown = name === "count" && typeof value === "number" ? numberFormat.format(value) : String(value)
          text = text.replaceAll(`{${name}}`, shown)
        }
      }
      return text
    },
    slotName: (slot) => data.slots[slot],
    primaryName: (key) => data.primaries[key],
    number: (value) => numberFormat.format(value),
    date: (iso) => {
      const date = new Date(iso)
      return Number.isNaN(date.getTime()) ? "" : dateFormat.format(date)
    },
  }
}

const cache = new Map<Lang, I18n>()

export function useI18n(): I18n {
  const lang = useStore((s) => s.lang)
  let i18n = cache.get(lang)
  if (!i18n) {
    i18n = createI18n(lang)
    cache.set(lang, i18n)
  }
  return i18n
}
