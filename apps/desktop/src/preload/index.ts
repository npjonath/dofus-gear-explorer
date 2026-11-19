import { contextBridge, ipcRenderer, type IpcRendererEvent } from "electron"

interface AppUpdate {
  state: "downloading" | "available" | "ready"
  version: string
}

contextBridge.exposeInMainWorld("dde", {
  config: (): Promise<{ apiBaseUrl: string; version: string }> => ipcRenderer.invoke("dde:config"),
  fightAssets: {
    import: (pick: boolean): Promise<boolean> => ipcRenderer.invoke("dde:fight-assets:import", pick),
    stage: (): Promise<string> => ipcRenderer.invoke("dde:fight-assets:stage"),
  },
  updates: {
    last: (): Promise<AppUpdate | null> => ipcRenderer.invoke("dde:update:last"),
    subscribe(listener: (update: AppUpdate | null) => void): () => void {
      const handler = (_event: IpcRendererEvent, update: AppUpdate | null) => listener(update)
      ipcRenderer.on("dde:update", handler)
      return () => ipcRenderer.removeListener("dde:update", handler)
    },
    check: (): Promise<AppUpdate | null> => ipcRenderer.invoke("dde:update:check"),
    install: () => ipcRenderer.send("dde:update:install"),
  },
})
