import type { Slot } from "../api/types"

const paths: Record<Slot, React.ReactNode> = {
  amulet: (
    <>
      <path d="M6 3c0 6 2.5 9 6 11 3.5-2 6-5 6-11" />
      <path d="M12 14l3 3.5-3 3.5-3-3.5z" />
    </>
  ),
  hat: (
    <>
      <path d="M5 16c0-6 2.5-10 7-10s7 4 7 10" />
      <path d="M2.5 16h19c0 1.7-1.3 3-3 3h-13c-1.7 0-3-1.3-3-3z" />
    </>
  ),
  ring: (
    <>
      <circle cx="12" cy="14.5" r="6" />
      <path d="M9.5 6l2.5-3 2.5 3-2.5 2.5z" />
    </>
  ),
  weapon: (
    <>
      <path d="M19.5 3.5l-10 10M19.5 3.5h-4l-9 9 4 4 9-9z" />
      <path d="M5 13l6 6M7 17l-3.5 3.5" />
    </>
  ),
  shield: <path d="M12 3l7.5 2.5v6c0 4.5-3 7.8-7.5 9.5-4.5-1.7-7.5-5-7.5-9.5v-6z" />,
  belt: (
    <>
      <path d="M2.5 9h19v6h-19z" />
      <rect x="9" y="7.5" width="6" height="9" rx="1" />
      <path d="M12 11v2" />
    </>
  ),
  cloak: (
    <>
      <path d="M8.5 4h7l4 16h-15z" />
      <path d="M8.5 4c0 2 1.5 3.5 3.5 3.5s3.5-1.5 3.5-3.5" />
    </>
  ),
  boots: (
    <>
      <path d="M8 3h6v9l5.5 3.5c1 .6 1.5 1.5 1.5 2.5v2h-13z" />
      <path d="M8 16.5h13" />
    </>
  ),
  pet: (
    <>
      <ellipse cx="12" cy="16" rx="4.5" ry="3.8" />
      <circle cx="6" cy="11" r="1.8" />
      <circle cx="10" cy="6.5" r="1.8" />
      <circle cx="14.5" cy="6.5" r="1.8" />
      <circle cx="18.5" cy="11" r="1.8" />
    </>
  ),
  dofus: (
    <>
      <path d="M12 3c4 0 7 5 7 10a7 7 0 0 1-14 0c0-5 3-10 7-10z" />
      <path d="M9 9.5c.5-1.5 1.5-2.5 3-3" />
    </>
  ),
}

export function SlotGlyph({ slot, className = "glyph" }: { slot: Slot; className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[slot]}
    </svg>
  )
}
