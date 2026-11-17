import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxGroup,
  ComboboxGroupLabel,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from "@ui-sdk/react/combobox"
import { ChevronDownIcon } from "lucide-react"
import * as React from "react"
import { fold } from "../lib/text"

export interface SearchOption {
  value: string
  label: string
  /** Extra words the option answers to, besides its label. */
  keywords?: string[]
  tone?: string
}

export interface SearchGroup {
  label: string
  options: SearchOption[]
}

interface Props {
  label: string
  groups: SearchGroup[]
  value: string | null
  placeholder?: string
  emptyText: string
  showLabel: string
  onChange(value: string): void
}

// A select the player can type into: "res" narrows the list to resistances.
export function SearchSelect({ label, groups, value, placeholder, emptyText, showLabel, onChange }: Props) {
  // With no value this is an "add" field: it empties itself after each pick.
  const [resetKey, setResetKey] = React.useState(0)
  const labels = React.useMemo(
    () => new Map(groups.flatMap((group) => group.options.map((option) => [option.value, option.label]))),
    [groups]
  )

  return (
    <Combobox
      key={value === null ? resetKey : "selected"}
      value={value}
      itemToLabel={(item: string) => labels.get(item) ?? item}
      onValueChange={(next) => {
        if (next === null) return
        onChange(next)
        if (value === null) setResetKey((key) => key + 1)
      }}
    >
      <div className="search-select">
        <ComboboxInput
          aria-label={label}
          placeholder={placeholder}
          onFocus={(event) => event.currentTarget.select()}
        />
        <ComboboxTrigger aria-label={showLabel}>
          <ChevronDownIcon aria-hidden="true" />
        </ComboboxTrigger>
      </div>
      <ComboboxContent className="max-h-80 min-w-56">
        <ComboboxEmpty>{emptyText}</ComboboxEmpty>
        <ComboboxList>
          {groups.map((group) =>
            group.options.length === 0 ? null : (
              <ComboboxGroup key={group.label}>
                {group.label && <ComboboxGroupLabel>{group.label}</ComboboxGroupLabel>}
                {group.options.map((option) => (
                  <ComboboxItem
                    key={option.value}
                    value={option.value}
                    label={option.label}
                    keywords={[fold(option.label), ...(option.keywords ?? [])]}
                  >
                    {option.tone && <span className="pip" data-tone={option.tone} />}
                    {option.label}
                  </ComboboxItem>
                ))}
              </ComboboxGroup>
            )
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}
