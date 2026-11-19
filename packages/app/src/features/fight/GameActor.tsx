import { LightAnimations } from "./combatPreferences"
import * as React from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import type { DofusSprite } from "d3-ts-renderer"
import { useApi } from "../../platform"
import type { Pose } from "../../lib/fightTimeline"

interface ActorCatalog {
  breeds: Record<string, string>
  dummy: string
  creatures?: Record<string, string>
  equipment: Record<string, number>
  weapons: Record<string, number>
  animations: Record<string, string[]>
}

export type GameDirection = 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7

export interface GameSpriteProps {
  assetRoot: string
  pack?: string
  look: string
  bone: string
  animation: string
  direction: GameDirection
  scale?: number
  resolution?: number
  phase?: number
  effectColors?: boolean
  portrait?: boolean
  fallback?: React.ReactNode
}

interface Surface {
  canvas: HTMLCanvasElement
  wrapper: SVGForeignObjectElement
  sprite?: DofusSprite
  look?: string
  bone?: string
  queue: Promise<void>
  frame: number
  closed: boolean
}

function usePack() {
  const api = useApi()
  return useQueryClient().getQueryData<{ pack?: string }>(["fight-assets", api.baseUrl])?.pack ?? ""
}

function resolveClip(sprite: DofusSprite, animation: string, direction: GameDirection): readonly [string, boolean] {
  if (sprite.animations.has(animation)) return [animation, false]
  const directions = sprite.availableDirections()[animation] ?? []
  // Some casts only have diagonal drawings. An axial target still plays the same cast.
  for (const offset of [0, 1, -1, 2, -2, 3, -3, 4]) {
    const candidate = ((direction + offset + 8) % 8) as GameDirection
    if (directions.includes(candidate)) return sprite.getAnimName(candidate, animation)
  }
  return sprite.getAnimName(direction, animation, false)
}

function freeSprite(sprite: DofusSprite | undefined) {
  if (!sprite) return
  const context = sprite.openGl
  context.assetStore.clear()
  context.unloadAllTextures()
  context.gl.getExtension("WEBGL_lose_context")?.loseContext()
}

let nextPortrait: Promise<unknown> = Promise.resolve()

function StillPortrait({ assetRoot, pack = "", look, bone, animation, direction, scale = 0.58, fallback }: GameSpriteProps) {
  const { data } = useQuery({
    queryKey: ["fight-portraits", assetRoot, pack, look, bone, animation, direction],
    queryFn: ({ signal }) => {
      const render = nextPortrait.then(async () => {
        signal.throwIfAborted()
        const { gameSpriteRenderer } = await import("./gameSpriteRenderer")
        const engine = gameSpriteRenderer(assetRoot, pack)
        const model = engine.Look.fromString(look)
        await model.injectColor()
        signal.throwIfAborted()
        const canvas = document.createElement("canvas")
        const sprite = await engine.DofusSprite.create(model, canvas, { boneName: bone })
        try {
          signal.throwIfAborted()
          const [name, flip] = resolveClip(sprite, animation, direction)
          await sprite.prepareAnimation(name, 2, true, flip)
          sprite.renderFrame(0)
          return {
            href: canvas.toDataURL(),
            x: -sprite.openGl.offset.x / 2,
            y: -sprite.openGl.offset.y / 2,
            width: canvas.width / 2,
            height: canvas.height / 2,
          }
        } finally {
          freeSprite(sprite)
        }
      })
      nextPortrait = render.catch(() => {})
      return render
    },
    staleTime: Infinity,
    retry: false,
  })
  if (!data) return fallback
  return (
    <image
      href={data.href}
      x={scale * data.x}
      y={scale * data.y}
      width={scale * data.width}
      height={scale * data.height}
      aria-hidden="true"
      pointerEvents="none"
    />
  )
}

export function GameSprite(props: GameSpriteProps) {
  const current = usePack()
  const pack = props.pack ?? current
  if (props.portrait) return <StillPortrait {...props} pack={pack} />
  return <SpriteSurface key={[props.assetRoot, pack, props.effectColors].join("#")} {...props} pack={pack} />
}

function SpriteSurface({
  assetRoot,
  pack = "",
  look,
  bone,
  animation,
  direction,
  scale = 0.58,
  resolution = 1,
  phase,
  effectColors,
  fallback,
}: GameSpriteProps) {
  const light = React.useContext(LightAnimations)
  const holder = React.useRef<SVGForeignObjectElement>(null)
  const surface = React.useRef<Surface | null>(null)
  const progress = React.useRef(phase)
  const [ready, setReady] = React.useState(false)

  React.useLayoutEffect(() => {
    progress.current = phase
  }, [phase])

  React.useLayoutEffect(() => {
    const wrapper = holder.current
    if (!wrapper) return
    const canvas = document.createElement("canvas")
    canvas.style.cssText = "display:block;width:100%;height:100%;pointer-events:none"
    wrapper.appendChild(canvas)
    const target: Surface = { wrapper, canvas, queue: Promise.resolve(), frame: 0, closed: false }
    surface.current = target
    return () => {
      target.closed = true
      cancelAnimationFrame(target.frame)
      canvas.remove()
      void target.queue.finally(() => freeSprite(target.sprite))
      surface.current = null
    }
  }, [])

  React.useEffect(() => {
    const target = surface.current
    if (!target) return
    let cancelled = false
    cancelAnimationFrame(target.frame)
    // Changing attachments and bones mutates a sprite. Finish the previous operation first.
    target.queue = target.queue
      .then(async () => {
        if (cancelled || target.closed) return
        const { gameSpriteRenderer } = await import("./gameSpriteRenderer")
        if (cancelled || target.closed) return
        const engine = gameSpriteRenderer(assetRoot, pack)
        if (!target.sprite || target.look !== look || target.bone !== bone) {
          const appearance = engine.Look.fromString(look)
          await appearance.injectColor()
          if (cancelled || target.closed) return
          if (!target.sprite) {
            target.sprite = await engine.DofusSprite.create(appearance, target.canvas, { boneName: bone })
            if (effectColors) {
              // Spell FX encode additive colour in BGRA order.
              const context = target.sprite.openGl
              const apply = context.setRenderUniforms.bind(context)
              context.setRenderUniforms = (mul, add, custom, matrix) =>
                apply(mul, [add[2], add[1], add[0], add[3]], custom, matrix)
            }
          } else {
            await target.sprite.changeLook(appearance, bone)
          }
          target.look = look
          target.bone = bone
        }
        if (cancelled || target.closed) return
        const sprite = target.sprite
        const [name, flip] = resolveClip(sprite, animation, direction)
        const density = Math.min(8, Math.max(0.5, (window.devicePixelRatio || 1) * resolution * (light ? 0.5 : 1)))
        const frames = await sprite.prepareAnimation(name, density, true, flip)
        if (cancelled || target.closed) return

        // The renderer crops every clip differently. Its pivot is the floor contact point.
        const factor = scale / density
        target.wrapper.setAttribute("x", String(-sprite.openGl.offset.x * factor))
        target.wrapper.setAttribute("y", String(-sprite.openGl.offset.y * factor))
        target.wrapper.setAttribute("width", String(target.canvas.width * factor))
        target.wrapper.setAttribute("height", String(target.canvas.height * factor))
        let drawn = -1
        const start = performance.now()
        const fps = sprite.data.defaultFrameRate || 24
        const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches
        const paint = (now: number) => {
          if (cancelled || target.closed) return
          let current = 0
          if (!reduced) {
            current =
              progress.current === undefined
                ? Math.floor(((now - start) * fps) / 1000) % Math.max(1, frames)
                : Math.round(Math.max(0, Math.min(1, progress.current)) * Math.max(0, frames - 1))
          }
          if (current !== drawn) {
            sprite.renderFrame(current)
            drawn = current
          }
          if (!reduced) target.frame = requestAnimationFrame(paint)
        }
        paint(start)
        setReady(true)
      })
      .catch((error: unknown) => {
        if (!cancelled && !target.closed) {
          if (import.meta.env.DEV) console.warn("Game sprite could not be displayed", bone, animation, error)
          setReady(false)
        }
      })
    return () => {
      cancelled = true
      cancelAnimationFrame(target.frame)
    }
  }, [assetRoot, pack, look, bone, animation, direction, scale, resolution, effectColors, light])

  return (
    <>
      {!ready && fallback}
      <foreignObject
        ref={holder}
        aria-hidden="true"
        pointerEvents="none"
        style={{ opacity: ready ? 1 : 0, overflow: "visible" }}
      />
    </>
  )
}

export interface GameActorProps {
  kind: "player" | "dummy" | "summon"
  placement?: boolean
  carrying?: boolean
  portrait?: boolean
  pack?: string
  monsterId?: number
  breedId?: number | null
  items?: Record<string, number>
  pose: Pose
  direction?: GameDirection
  animation?: string
  spellId?: number
  phase?: number
  scale?: number
  resolution?: number
  fallback?: React.ReactNode
}

function attachmentLook(base: string, items: Record<string, number> | undefined, catalog: ActorCatalog) {
  const skins = ["cloak", "hat", "shield"].map((slot) => catalog.equipment[items?.[slot] ?? 0]).filter(Boolean)
  if (!skins.length) return base
  const fields = base.slice(1, -1).split("|")
  fields[1] = [fields[1], ...skins].filter(Boolean).join(",")
  return "{" + fields.join("|") + "}"
}

function findClip(catalog: ActorCatalog, bones: string[], names: string[]) {
  for (const name of names) {
    for (const bone of bones) {
      if (catalog.animations[bone]?.includes(name)) return { bone, animation: name }
    }
  }
  return null
}

export function useGameActorCatalog(enabled: boolean, version?: string) {
  const api = useApi()
  const root = api.imageUrl("/fight-assets/actors/")
  const current = usePack()
  const pack = version ?? current
  const { data } = useQuery<ActorCatalog>({
    enabled,
    queryKey: ["fight-actors", root, pack],
    queryFn: async ({ signal }) => {
      const url = new URL("index.json", new URL(root, document.baseURI))
      if (pack) url.searchParams.set("v", pack)
      const response = await fetch(url, { signal })
      if (!response.ok) throw new Error("The fight actor catalog is unavailable")
      return await response.json()
    },
    staleTime: Infinity,
    retry: false,
  })
  return { root, pack, data }
}

export function GameActor(props: GameActorProps) {
  const { root, pack, data } = useGameActorCatalog(true, props.pack)
  if (!data) return props.fallback
  const breed = props.breedId || 1
  const player = props.kind === "player"
  let look = data.dummy
  if (player) look = attachmentLook(data.breeds[breed] ?? "", props.items, data)
  if (props.kind === "summon") look = data.creatures?.[props.monsterId ?? 0] ?? ""
  if (!look) return props.fallback
  const creatureBone = look.slice(1, -1).split("|")[0]!
  const idle = player ? "AnimStatiqueCombat0" : "AnimStatique"
  const bones = player
    ? [`1-${breed}-combat`, "1-combat-armes", "1-combat", `1-${breed}-static`, "1-movement", "1-static"]
    : [creatureBone]
  const candidates: string[] = []
  switch (props.pose) {
    case "walk":
      if (props.carrying) candidates.push("AnimCourseCarrying", "AnimMarcheCarrying")
      candidates.push("AnimCourse", "AnimMarche")
      break
    case "down":
      candidates.push("AnimMort")
      break
    case "hit":
      candidates.push("AnimHit")
      break
    case "windup":
    case "lunge":
      if (props.animation) candidates.push(props.animation)
      if (props.spellId === 0) candidates.push("AnimArme" + (data.weapons[props.items?.weapon ?? 0] ?? 0), "AnimArme0")
      candidates.push("AnimAttaque0", "AnimAttaque1")
      break
  }
  if (props.carrying) candidates.push("AnimStatiqueCarrying")
  if (player && !props.placement) candidates.push(`AnimStatiqueCombat${breed}a`)
  candidates.push(idle)
  const clip = findClip(data, bones, candidates)
  if (!clip) return props.fallback
  return (
    <GameSprite
      assetRoot={root}
      pack={pack}
      portrait={props.portrait}
      look={look}
      {...clip}
      direction={props.direction ?? (player ? 3 : 1)}
      scale={props.scale}
      resolution={props.resolution}
      phase={props.pose === "idle" || props.pose === "walk" ? undefined : props.phase}
      fallback={props.fallback}
    />
  )
}
