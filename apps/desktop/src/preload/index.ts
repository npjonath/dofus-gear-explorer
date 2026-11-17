import { contextBridge, ipcRenderer, type IpcRendererEvent } from "electron"

interface AppUpdate {
  state: "available" | "ready"
  version: string
}

contextBridge.exposeInMainWorld("dde", {
  config: (): Promise<{ apiBaseUrl: string; version: string }> => ipcRenderer.invoke("dde:config"),
  updates: {
    last: (): Promise<AppUpdate | null> => ipcRenderer.invoke("dde:update:last"),
    subscribe(listener: (update: AppUpdate) => void): () => void {
      const handler = (_event: IpcRendererEvent, update: AppUpdate) => listener(update)
      ipcRenderer.on("dde:update", handler)
      return () => ipcRenderer.removeListener("dde:update", handler)
    },
    install: () => ipcRenderer.send("dde:update:install"),
  },
})
