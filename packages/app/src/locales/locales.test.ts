import { describe, expect, it } from "vitest"
import { createI18n } from "../i18n"
import { languages, locales, pickLang, type MessageKey } from "."

const placeholders = (text: string) => [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()
const forms = (message: string | { one: string; other: string }) =>
  typeof message === "string" ? [message] : [message.one, message.other]

describe("locales", () => {
  const keys = Object.keys(locales.fr.messages) as MessageKey[]

  it.each(languages)("%s uses the placeholders of the French messages", (lang) => {
    for (const key of keys) {
      const expected = placeholders(forms(locales.fr.messages[key]).at(-1) ?? "")
      for (const form of forms(locales[lang].messages[key])) {
        expect(placeholders(form), `${lang} ${key}`).toEqual(expected)
      }
    }
  })

  it.each(languages)("%s has no empty message", (lang) => {
    for (const key of keys) {
      for (const form of forms(locales[lang].messages[key])) expect(form.trim(), `${lang} ${key}`).not.toBe("")
    }
  })
})

describe("plural forms", () => {
  it("follow each language's rules", () => {
    const fr = createI18n("fr")
    const en = createI18n("en")
    // French counts zero as singular, English as plural.
    expect(fr.t("items.count", { count: 0 })).toBe("0 objet")
    expect(en.t("items.count", { count: 0 })).toBe("0 items")
    expect(createI18n("es").t("items.count", { count: 1 })).toBe("1 objeto")
    expect(createI18n("pt").t("items.count", { count: 2 })).toBe("2 itens")
  })

  it("format the count", () => {
    expect(createI18n("en").t("items.count", { count: 1234 })).toBe("1,234 items")
    expect(createI18n("es").t("solver.results", { count: 20 })).toBe("20 equipamientos encontrados")
  })
})

describe("pickLang", () => {
  it("takes the first supported browser language", () => {
    expect(pickLang(["pt-BR", "en-US"])).toBe("pt")
    expect(pickLang(["de-DE", "es-419", "fr"])).toBe("es")
    expect(pickLang(["FR-ca"])).toBe("fr")
    expect(pickLang(["de", "it"])).toBe("en")
    expect(pickLang([])).toBe("en")
  })
})
