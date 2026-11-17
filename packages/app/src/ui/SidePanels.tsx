import { Resizable, ResizableHandle, ResizablePanel } from "@ui-sdk/react/resizable"
import * as React from "react"
import { useI18n } from "../i18n"
import { useStore } from "../store"

interface Props {
  /** Names the saved widths; each screen keeps its own. */
  id: string
  left?: React.ReactNode
  right?: React.ReactNode
  /** Default widths, in rem. */
  leftSize?: number
  rightSize?: number
  children: React.ReactNode
}

const MIN_SIDE_REM = 15
const MAX_SIDE_REM = 44
// The work area never gets narrower than this share of the screen.
const MIN_MAIN_PERCENT = 25

function useWidth(ref: React.RefObject<HTMLElement | null>): number {
  const [width, setWidth] = React.useState(0)
  React.useLayoutEffect(() => {
    const element = ref.current
    if (!element) return
    // A hidden screen measures zero: keep the last real width until it shows again.
    const update = (next: number) => {
      if (next > 0) setWidth(next)
    }
    update(element.getBoundingClientRect().width)
    const observer = new ResizeObserver(([entry]) => update(entry!.contentRect.width))
    observer.observe(element)
    return () => observer.disconnect()
  }, [ref])
  return width
}

// The screen's columns: optional side panels the player drags wider or
// narrower, around the work area. A side panel keeps its width in pixels
// when the window changes size; only the work area stretches.
export function SidePanels({ id, left, right, leftSize = 22, rightSize = 24, children }: Props) {
  const { t } = useI18n()
  const ref = React.useRef<HTMLDivElement>(null)
  const width = useWidth(ref)
  const saved = useStore((s) => s.panelWidths[id])
  const setPanelWidths = useStore((s) => s.setPanelWidths)

  const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16
  const hasLeft = Boolean(left)
  const hasRight = Boolean(right)

  // Side widths in pixels, kept within their bounds and the room available.
  const sides = hasLeft && hasRight ? 2 : 1
  const roomPerSide = width > 0 ? (width * (100 - MIN_MAIN_PERCENT)) / 100 / sides : Infinity
  const clamp = (px: number) => Math.max(MIN_SIDE_REM * rem, Math.min(px, MAX_SIDE_REM * rem, roomPerSide))
  const leftPx = hasLeft ? clamp(saved?.left ?? leftSize * rem) : 0
  const rightPx = hasRight ? clamp(saved?.right ?? rightSize * rem) : 0

  const percent = (px: number) => (width > 0 ? (px / width) * 100 : 0)
  const layout: number[] = []
  if (hasLeft) layout.push(percent(leftPx))
  layout.push(100 - percent(leftPx) - percent(rightPx))
  if (hasRight) layout.push(percent(rightPx))

  return (
    <div ref={ref} className="h-full min-h-0">
      {width > 0 && (
        <Resizable
          className="side-panels"
          layout={layout}
          onLayoutChange={(next, details) => {
            // The window resizing is not the player choosing a width.
            if (details.reason === "resize") return
            const px = (share: number | undefined) => ((share ?? 0) / 100) * width
            setPanelWidths(id, {
              left: hasLeft ? px(next[0]) : saved?.left,
              right: hasRight ? px(next[next.length - 1]) : saved?.right,
            })
          }}
        >
          {left && (
            <>
              <ResizablePanel id={`${id}-left`}>{left}</ResizablePanel>
              <ResizableHandle aria-label={t("panels.resizeLeft")} />
            </>
          )}
          <ResizablePanel id={`${id}-main`}>{children}</ResizablePanel>
          {right && (
            <>
              <ResizableHandle aria-label={t("panels.resizeRight")} />
              <ResizablePanel id={`${id}-right`}>{right}</ResizablePanel>
            </>
          )}
        </Resizable>
      )}
    </div>
  )
}
