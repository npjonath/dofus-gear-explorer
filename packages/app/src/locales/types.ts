import type { PrimaryKey, Slot } from "../api/types"
import type { Messages } from "./fr"

/** A message that depends on a count, chosen with Intl.PluralRules. */
export interface Plural {
  one: string
  other: string
}

export interface LocaleData {
  /** The language's own name, for the language switch. */
  name: string
  /** The BCP 47 tag for number, date and plural formats. */
  intl: string
  /** What goes between a label and its value: " : " in French, ": " elsewhere. */
  colon: string
  messages: Messages
  slots: Record<Slot, string>
  primaries: Record<PrimaryKey, string>
}
