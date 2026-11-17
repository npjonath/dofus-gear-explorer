import { NumberInput, NumberInputControl, NumberInputGroup } from "@ui-sdk/react/number-input"

interface Props {
  label: string
  value: number
  onChange(value: number): void
  min?: number
  max?: number
  invalid?: boolean
  className?: string
}

export function NumberField({ label, value, onChange, min = 0, max, invalid, className }: Props) {
  return (
    <NumberInput value={value} min={min} max={max} onValueChange={(next) => onChange(next ?? min)}>
      <NumberInputGroup className="contents">
        <NumberInputControl
          aria-label={label}
          aria-invalid={invalid || undefined}
          className={className ?? "num-input"}
          onFocus={(event) => event.currentTarget.select()}
        />
      </NumberInputGroup>
    </NumberInput>
  )
}
