import type { FightEntity } from "../api/types"
import type { Timeline } from "./fightTimeline"

export type SoundChannel = "music" | "ambience" | "effects" | "voices" | "ui"
export type VolumeKey = SoundChannel | "master"
export type SoundLevels = Record<VolumeKey, number>
export type InterfaceSound = "begin" | "endTurn" | "turn" | "click" | "open" | "close" | "critical" | "move"
export const defaultLevels: SoundLevels = { master: 30, music: 35, ambience: 70, effects: 70, voices: 70, ui: 70 }

type Layers = [number, string[]][]
interface CombatSound {
  channel: SoundChannel
  layers: Layers
}
export interface SoundCatalog {
  categories: SoundChannel[]
  events: Record<string, CombatSound>
  voices: Record<number, Record<string, string[]>>
  animations: Record<string, Record<string, [number, string][]>>
  maps: Record<number, Partial<Record<"music" | "ambience", string>>>
  ui: Partial<Record<InterfaceSound, string>>
  poutchHit: string[]
  creatures: Record<number, string>
  dummy: string
  weapons: Record<number, number>
}
export interface SoundCue {
  at: number
  event?: string
  take?: string[]
}

export function fightSoundCues(catalog: SoundCatalog, timeline: Timeline, entities: FightEntity[], breedId: number, weaponId: number, poutchHit: boolean): SoundCue[] {
  const cues: SoundCue[] = []
  const actors = new Map(entities.map((entity) => [entity.id, entity]))
  const sounds = (bones: string[], names: string[], start: number) => {
    for (const name of names) {
      for (const bone of bones) {
        const clip = catalog.animations[bone]?.[name]
        if (!clip) continue
        for (const [offset, event] of clip) {
          if (catalog.events[event]) cues.push({ at: start + offset, event })
        }
        return
      }
    }
  }
  function actorBones(entity: FightEntity) {
    if (entity.kind === "dummy") return [catalog.dummy]
    if (entity.kind === "summon") return [catalog.creatures[entity.monsterId ?? 0] ?? ""]
    return [`1-${breedId}-combat`, "1-combat-armes", "1-combat", "1-movement", "1-static"]
  }
  for (const pose of timeline.poses) {
    const entity = actors.get(pose.entity)
    if (!entity) continue
    if (entity.kind === "dummy" && pose.pose === "hit") {
      if (poutchHit && catalog.poutchHit.length) cues.push({ at: pose.start, take: catalog.poutchHit })
      continue
    }
    const name = pose.animation ?? (pose.pose === "down" ? "AnimMort" : pose.pose === "hit" ? "AnimHit"
      : pose.spell === 0 ? `AnimArme${catalog.weapons[weaponId] ?? 0}` : undefined)
    if (name) sounds(actorBones(entity), [name], pose.start)
  }
  for (const move of timeline.moves) {
    const entity = actors.get(move.entity)
    if (entity && move.gait === "walk") sounds(actorBones(entity), ["AnimCourse", "AnimMarche"], move.start)
  }
  for (const effect of timeline.graphics) sounds([String(effect.bone)], [effect.animation], effect.start)
  return cues
}

const pick = (choices: string[]) => choices[Math.floor(Math.random() * choices.length)]

export class FightSoundPlayer {
  private context?: AudioContext
  private channels = new Map<SoundChannel, GainNode>()
  private samples = new Map<string, Promise<AudioBuffer>>()
  private activeSounds = new Map<AudioBufferSourceNode, boolean>()
  private background: { element: HTMLAudioElement; channel: SoundChannel; end?: () => void }[] = []
  private timers: ReturnType<typeof setTimeout>[] = []
  private generation = 0
  private map = 0
  private backgroundStarted = false
  private lastRun = -1
  private levels = { ...defaultLevels }

  constructor(private readonly root: string, private readonly catalog: SoundCatalog) {}

  unlock() {
    try {
      this.context ??= new AudioContext()
      if (this.context.state === "suspended") void this.context.resume().catch(() => {})
    } catch { /* A device can have no audio output. */ }
  }

  volumes(levels: SoundLevels) {
    this.levels = levels
    for (const [channel, node] of this.channels) node.gain.value = this.volume(channel)
    for (const { element, channel } of this.background) element.volume = this.volume(channel)
  }

  private volume(channel: SoundChannel) {
    return (this.levels.master * this.levels[channel]) / 10000
  }

  private url(sample: string) {
    return `${this.root}/audio/samples/${sample}.ogg`
  }

  private playSample(sample: string, channel: SoundChannel, clock: number, action: boolean) {
    const ctx = this.context
    if (!ctx || !this.volume(channel)) return
    const generation = this.generation
    let decoded = this.samples.get(sample)
    if (!decoded) {
      decoded = fetch(this.url(sample)).then(async (response) => {
        if (!response.ok) throw new Error("Combat sound is unavailable.")
        return ctx.decodeAudioData(await response.arrayBuffer())
      })
      this.samples.set(sample, decoded)
      if (this.samples.size > 80) this.samples.delete(this.samples.keys().next().value!)
    }
    void decoded.then((buffer) => {
      if (this.context !== ctx || ctx.state === "closed" || action && this.generation !== generation) return
      const offset = Math.max(0, ctx.currentTime - clock)
      if (offset >= buffer.duration) return
      let gain = this.channels.get(channel)
      if (!gain) {
        gain = ctx.createGain()
        gain.connect(ctx.destination)
        this.channels.set(channel, gain)
      }
      gain.gain.value = this.volume(channel)
      const source = ctx.createBufferSource()
      source.buffer = buffer
      source.connect(gain)
      this.activeSounds.set(source, action)
      source.addEventListener("ended", () => { this.activeSounds.delete(source); source.disconnect() }, { once: true })
      source.start(Math.max(ctx.currentTime, clock), offset)
    }).catch(() => { if (this.samples.get(sample) === decoded) this.samples.delete(sample) })
  }

  private event(guid: string, at: number, breed: number, action: boolean) {
    const ctx = this.context
    const event = this.catalog.events[guid]
    if (!ctx || !event) return
    if (event.channel === "voices") {
      const chosen = pick(this.catalog.voices[breed]?.[guid] ?? [])
      if (chosen) this.playSample(chosen, event.channel, at, action)
      return
    }
    for (const [offset, choices] of event.layers) {
      const chosen = pick(choices)
      if (chosen) this.playSample(chosen, event.channel, at + offset / 1000, action)
    }
  }

  interfaceSound(id: InterfaceSound) {
    this.unlock()
    const guid = this.catalog.ui[id]
    if (guid && this.context) this.event(guid, this.context.currentTime, 0, false)
  }

  play(id: number, timeline: Timeline, startedAt: number, entities: FightEntity[], breed: number, weaponId: number, poutchHit: boolean) {
    if (this.lastRun === id || !this.context) return
    this.stopAction()
    this.lastRun = id
    const origin = this.context.currentTime - Math.max(0, performance.now() - startedAt) / 1000
    for (const cue of fightSoundCues(this.catalog, timeline, entities, breed, weaponId, poutchHit)) {
      if (cue.event) this.event(cue.event, origin + cue.at / 1000, breed, true)
      else if (cue.take) {
        const chosen = pick(cue.take)
        if (chosen) this.playSample(chosen, "effects", origin + cue.at / 1000, true)
      }
    }
  }

  stopAction() {
    this.generation++
    for (const [source, action] of this.activeSounds) if (action) source.stop()
  }

  start(mapId: number) {
    this.unlock()
    if (this.map === mapId && this.backgroundStarted) return
    this.stopBackground()
    this.map = mapId
    const map = this.catalog.maps[mapId]
    if (!map) return
    this.backgroundStarted = true
    for (const channel of ["music", "ambience"] as const) {
      const guid = map[channel]
      const event = guid ? this.catalog.events[guid] : undefined
      if (!event?.layers?.length) continue
      const phases = [...new Set(event.layers.map(([at]) => at))].sort((a, b) => a - b)
      const playPhase = (index: number) => {
        if (this.map !== mapId || !this.backgroundStarted) return
        this.removeTracks(channel)
        const at = phases[index]!
        const group = event.layers.filter(([start]) => start === at)
        for (const [, choices] of group) {
          const sample = pick(choices)
          if (!sample) continue
          const element = new Audio(this.url(sample))
          element.volume = this.volume(channel)
          element.loop = phases.length === 1
          this.background.push({element, channel})
          void element.play().catch(() => {})
        }
        if (phases.length === 1) return
        const next = phases[index + 1]
        if (next !== undefined) this.timers.push(setTimeout(() => playPhase(index + 1), next - at))
        else {
          const final = this.background.find((track) => track.channel === channel)
          if (final) {
            final.end = () => playPhase(0)
            final.element.addEventListener("ended", final.end, { once: true })
          }
        }
      }
      playPhase(0)
    }
  }

  private removeTracks(channel?: SoundChannel) {
    this.background = this.background.filter((track) => {
      if (channel && track.channel !== channel) return true
      if (track.end) track.element.removeEventListener("ended", track.end)
      track.element.pause()
      track.element.removeAttribute("src")
      track.element.load()
      return false
    })
  }

  private stopBackground() {
    this.backgroundStarted = false
    for (const timer of this.timers) clearTimeout(timer)
    this.timers = []
    this.removeTracks()
  }

  stop() {
    this.stopBackground()
    this.stopAction()
    for (const source of this.activeSounds.keys()) source.stop()
    this.map = 0
  }

  dispose() {
    this.stop()
    const ctx = this.context
    this.context = undefined
    if (ctx && ctx.state !== "closed") void ctx.close().catch(() => {})
    this.samples.clear()
    this.channels.clear()
    this.activeSounds.clear()
  }
}
