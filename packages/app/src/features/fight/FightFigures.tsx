import type { Pose } from "../../lib/fightTimeline"

/*
  The pieces that stand on the fight board, drawn here and nowhere else: an
  adventurer for the player, a padded post for a training dummy. Both are
  flat shapes in the ink and cell tokens, drawn looking right with their
  feet at the origin; the board mirrors them to look left.
*/

export interface FigureProps {
  pose: Pose
  /** How far into the pose, from 0 to 1. */
  phase: number
  /** Cells walked so far. */
  stride: number
  /** The tone of the spell being cast, shown on the staff. */
  tone?: string
}

/** How far above its feet a figure rises: what is written over it starts there. */
export const FIGURE_TOP = { player: 60, dummy: 51, summon: 40 } as const

/** The outline of each figure, for the place a shove would leave it. */
export const SILHOUETTE = {
  summon: "M-12 0 Q-15 -18 -8 -25 Q0 -32 8 -25 Q15 -18 12 0Z",
  player:
    "M-7 -35 C-9.5 -44 -5 -50.5 1 -50.5 C6.5 -50.5 8.5 -45 7.5 -35 L9 -33 L10.5 -14 L6 -14 L6 0 L-6 0 L-6 -14 L-10.5 -14 L-9 -33 Z",
  dummy:
    "M-5.5 -37 L-5.5 -45 Q-5.5 -48.5 0 -48.5 Q5.5 -48.5 5.5 -45 L5.5 -37 L9 -36 L15.5 -35 L15.5 -28 L9 -29 L9 -17 Q9 -14 5 -14 L2 -14 L2 -3 L7 -3 L9.5 0 L-9.5 0 L-7 -3 L-2 -3 L-2 -14 L-5 -14 Q-9 -14 -9 -17 L-9 -29 L-15.5 -28 L-15.5 -35 L-9 -36 Z",
} as const

const easeOut = (value: number) => 1 - (1 - value) ** 3

// The staff arm of a cast: drawn back, then thrust at the target.
function staffAngle(pose: Pose, phase: number): number {
  if (pose === "windup") return phase < 0.55 ? -46 * easeOut(phase / 0.55) : -46 + 74 * easeOut((phase - 0.55) / 0.45)
  if (pose === "lunge") return 44 * Math.sin(Math.PI * phase)
  if (pose === "walk") return 5
  return 0
}

export function Hero({ pose, phase, stride, tone }: FigureProps) {
  const swing = pose === "walk" ? Math.sin(stride * Math.PI) * 27 : 0
  const casting = pose === "windup" || pose === "lunge"
  return (
    <g className="fight-fig">
      <path className="fig-ink" d="M-5 -35.5 C-11.5 -31 -14.5 -20 -13.5 -8.5 L-6.5 -12.5 L-3 -34 Z" />
      <rect className="fig-ink" x={-5.8} y={-16} width={5.2} height={16} rx={2} transform={`rotate(${swing} -3.2 -15)`} />
      <rect className="fig-ink" x={0.6} y={-16} width={5.2} height={16} rx={2} transform={`rotate(${-swing} 3.2 -15)`} />
      <g className="fig-breath">
        <path
          className="fig-body"
          d="M-6.5 -35 H6.5 A2.5 2.5 0 0 1 9 -32.6 L10.4 -15.8 A1.6 1.6 0 0 1 8.8 -14 H-8.8 A1.6 1.6 0 0 1 -10.4 -15.8 L-9 -32.6 A2.5 2.5 0 0 1 -6.5 -35 Z"
        />
        <rect className="fig-trim" x={-9.8} y={-22.6} width={19.6} height={2.4} />
        <rect className="fig-face" x={-6} y={-47.5} width={13} height={12.5} rx={4.5} />
        <path
          className="fig-ink"
          d="M-7.6 -35.4 C-9.6 -43 -6.6 -50.6 1 -50.6 C5.6 -50.6 8.2 -48 8.6 -44.6 L2.4 -45.4 C-0.6 -45.4 -2.2 -43.4 -2.2 -40.4 L-2.2 -35.4 Z"
        />
        <rect className="fig-ink" x={3.1} y={-42.8} width={2.1} height={3.2} rx={1} />
        <g transform={`rotate(${staffAngle(pose, phase)} 5 -31)`}>
          <rect className="fig-wood" x={9.6} y={-53} width={2.2} height={42} rx={1.1} />
          <rect
            className="fig-pip"
            data-tone={casting ? tone : undefined}
            x={7.4}
            y={-59.6}
            width={6.6}
            height={6.6}
            rx={1.2}
            transform="rotate(45 10.7 -56.3)"
          />
          <path className="fig-arm" d="M4.6 -31 L10.6 -24.4" />
        </g>
      </g>
    </g>
  )
}

export function Dummy() {
  return (
    <g className="fight-fig">
      <path className="fig-wood" d="M-9.5 0 L9.5 0 L7 -3.2 L-7 -3.2 Z" />
      <g className="fig-sway">
        <rect className="fig-wood" x={-1.8} y={-32} width={3.6} height={29.5} />
        <rect className="fig-wood" x={-13} y={-34} width={26} height={3.4} rx={1.6} />
        <rect className="fig-pad" x={-15.5} y={-35.6} width={5.4} height={7.6} rx={2.2} />
        <rect className="fig-pad" x={10.1} y={-35.6} width={5.4} height={7.6} rx={2.2} />
        <rect className="fig-pad" x={-9} y={-37} width={18} height={23} rx={5} />
        <path className="fig-stitch" d="M-9 -19.5 H9" />
        <circle className="fig-ring" cx={0} cy={-27.5} r={5.2} />
        <circle className="fig-ink" cx={0} cy={-27.5} r={1.9} />
        <rect className="fig-pad" x={-5.5} y={-48.5} width={11} height={11} rx={4} />
        <rect className="fig-wood" x={-3.4} y={-38.4} width={6.8} height={2} rx={1} />
      </g>
    </g>
  )
}

export function Figure({ kind, ...props }: FigureProps & { kind: "player" | "dummy" | "summon" }) {
  return kind === "player" ? <Hero {...props} /> : <Dummy />
}

const STILL: FigureProps = { pose: "idle", phase: 0, stride: 0 }

/** A figure standing still in a small frame of its own: the piece as a portrait. */
export function FigurePortrait({ kind }: { kind: "player" | "dummy" | "summon" }) {
  return (
    <svg className="fight-portrait" viewBox="-19 -62 38 65" aria-hidden="true">
      <Figure kind={kind} {...STILL} />
    </svg>
  )
}
