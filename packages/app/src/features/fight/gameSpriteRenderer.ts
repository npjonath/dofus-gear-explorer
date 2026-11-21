import { createDataLoader, DofusSprite, Look, setLoader, type DataLoader } from "d3-ts-renderer"

// Portraits, the board and overlapping spell effects can all request the same atlas.
class Recent<T> {
  private readonly entries = new Map<string, Promise<T>>()

  constructor(private readonly limit: number) {}

  get(key: string, load: () => Promise<T>): Promise<T> {
    const old = this.entries.get(key)
    if (old) {
      this.entries.delete(key)
      this.entries.set(key, old)
      return old
    }
    const pending = load()
    this.entries.set(key, pending)
    if (this.entries.size > this.limit) this.entries.delete(this.entries.keys().next().value!)
    void pending.catch(() => {
      if (this.entries.get(key) === pending) this.entries.delete(key)
    })
    return pending
  }
}

function packLoader(root: string) {
  const loader = createDataLoader({ strategy: "url", basePath: root, ImageExtension: "webp" })
  const skins = new Recent<Awaited<ReturnType<DataLoader["loadSkin"]>>>(12)
  const bones = new Recent<Awaited<ReturnType<DataLoader["loadBone"]>>>(16)
  const clips = new Recent<ArrayBuffer>(32)
  const loadSkin = loader.loadSkin.bind(loader)
  const loadBone = loader.loadBone.bind(loader)
  const loadClip = loader.loadAnimationData.bind(loader)
  loader.loadSkin = id => skins.get(String(id), () => loadSkin(id))
  loader.loadBone = (id, prop) => bones.get([id, prop].join(":"), () => loadBone(id, prop))
  loader.loadAnimationData = (id, name, prop) =>
    clips.get([id, name, prop].join(":"), () => loadClip(id, name, prop))
  return loader
}

let source: { root: string; pack: string } | undefined

export function gameSpriteRenderer(root: string, pack: string) {
  if (source?.root !== root || source.pack !== pack) {
    setLoader(packLoader(root))
    source = { root, pack }
  }
  return { DofusSprite, Look }
}
