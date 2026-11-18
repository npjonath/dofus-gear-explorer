import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Toast, ToastAction, ToastDescription, ToastRegion, ToastTitle } from "@ui-sdk/react/toast"
import { act, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type * as React from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { PlatformProvider, type AppUpdate, type Platform, type UpdateBridge } from "../platform"
import { useStore } from "../store"
import { toaster } from "../ui/toaster"
import { AboutMenu } from "./AboutMenu"
import { UpdateButton } from "./UpdateButton"
import { useAppUpdates } from "./useAppUpdates"

// The desktop shell's bridge, with the update the shell already knew at start and a way to speak as the shell later.
function fakeUpdates(first: AppUpdate | null = null) {
  const listeners = new Set<(update: AppUpdate | null) => void>()
  const bridge = {
    last: vi.fn(async () => first),
    subscribe: vi.fn((listener: (update: AppUpdate | null) => void) => {
      listeners.add(listener)
      return () => void listeners.delete(listener)
    }),
    check: vi.fn(async (): Promise<AppUpdate | null> => null),
    install: vi.fn(),
  } satisfies UpdateBridge
  const tell = (update: AppUpdate | null) => act(() => listeners.forEach((listener) => listener(update)))
  return { bridge, tell }
}

function TopBar() {
  const update = useAppUpdates()
  return (
    <>
      <UpdateButton update={update} />
      <AboutMenu />
      <ToastRegion toaster={toaster}>
        {() => (
          <Toast>
            <ToastTitle />
            <ToastDescription />
            <ToastAction />
          </Toast>
        )}
      </ToastRegion>
    </>
  )
}

function show(updates?: UpdateBridge, ui: React.ReactNode = <TopBar />) {
  const platform: Platform = { kind: updates ? "desktop" : "web", apiBaseUrl: "http://api.test", version: "1.0.0", updates }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <PlatformProvider platform={platform}>
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    </PlatformProvider>
  )
}

const updateButton = () => screen.queryByRole("button", { name: /version 1\.1\.0/i })

beforeEach(() => {
  useStore.setState({ lang: "en" })
  // The About menu reads the data status; these tests have no API.
  vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("no API in tests"))))
})

afterEach(() => {
  toaster.dismiss()
  vi.unstubAllGlobals()
})

describe("the update button", () => {
  it("is absent on the web, where nothing installs updates", async () => {
    show()
    await screen.findByRole("button", { name: "About and legal notice" })
    expect(screen.getAllByRole("button")).toHaveLength(1)
  })

  it("is absent while there is nothing to install", async () => {
    const { bridge } = fakeUpdates()
    show(bridge)
    await waitFor(() => expect(bridge.last).toHaveBeenCalled())
    expect(screen.getAllByRole("button")).toHaveLength(1)
  })

  it("installs and restarts in one click when the update found at start is ready", async () => {
    const { bridge } = fakeUpdates({ state: "ready", version: "1.1.0" })
    show(bridge)
    const button = await screen.findByRole("button", { name: "Install version 1.1.0 and restart" })
    expect(button.dataset.state).toBe("ready")
    await userEvent.click(button)
    expect(bridge.install).toHaveBeenCalledTimes(1)
  })

  it("appears in a running app, waits for the download, then installs", async () => {
    const { bridge, tell } = fakeUpdates()
    show(bridge)
    await waitFor(() => expect(bridge.last).toHaveBeenCalled())
    expect(updateButton()).toBeNull()

    tell({ state: "downloading", version: "1.1.0" })
    const downloading = await screen.findByRole("button", { name: "Downloading version 1.1.0…" })
    expect(downloading.getAttribute("aria-disabled")).toBe("true")
    await userEvent.click(downloading)
    expect(bridge.install).not.toHaveBeenCalled()

    tell({ state: "ready", version: "1.1.0" })
    const ready = await screen.findByRole("button", { name: "Install version 1.1.0 and restart" })
    expect(ready.getAttribute("aria-disabled")).toBeNull()
    await userEvent.click(ready)
    expect(bridge.install).toHaveBeenCalledTimes(1)
  })

  it("opens the release page when the app can't replace itself", async () => {
    const { bridge } = fakeUpdates({ state: "available", version: "1.1.0" })
    show(bridge)
    const button = await screen.findByRole("button", { name: "Download version 1.1.0 from GitHub" })
    expect(button.dataset.state).toBe("available")
    await userEvent.click(button)
    expect(bridge.install).toHaveBeenCalledTimes(1)
  })

  it("goes away when the download fails", async () => {
    const { bridge, tell } = fakeUpdates({ state: "downloading", version: "1.1.0" })
    show(bridge)
    await screen.findByRole("button", { name: "Downloading version 1.1.0…" })
    tell(null)
    await waitFor(() => expect(updateButton()).toBeNull())
  })

  it("keeps what the shell said last when the first answer arrives late", async () => {
    const { bridge, tell } = fakeUpdates()
    let answer: (update: AppUpdate | null) => void = () => {}
    bridge.last.mockReturnValue(new Promise((resolve) => (answer = resolve)))
    show(bridge)
    tell({ state: "ready", version: "1.1.0" })
    await act(async () => answer(null))
    expect(screen.getByRole("button", { name: "Install version 1.1.0 and restart" })).toBeTruthy()
  })

  it("names the version in the interface language", async () => {
    useStore.setState({ lang: "fr" })
    const { bridge } = fakeUpdates({ state: "ready", version: "1.1.0" })
    show(bridge)
    expect(await screen.findByRole("button", { name: "Installer la version 1.1.0 et redémarrer" })).toBeTruthy()
  })
})

describe("the update notice", () => {
  it("is shown once an update is ready, not while it downloads", async () => {
    const { bridge, tell } = fakeUpdates()
    show(bridge)
    await waitFor(() => expect(bridge.last).toHaveBeenCalled())
    tell({ state: "downloading", version: "1.1.0" })
    await screen.findByRole("button", { name: "Downloading version 1.1.0…" })
    expect(screen.queryByText("Version 1.1.0 is ready")).toBeNull()

    tell({ state: "ready", version: "1.1.0" })
    expect(await screen.findByText("Version 1.1.0 is ready")).toBeTruthy()
    await userEvent.click(screen.getByRole("button", { name: "Restart" }))
    expect(bridge.install).toHaveBeenCalledTimes(1)
  })
})

describe("checking for an update from the About menu", () => {
  async function check() {
    await userEvent.click(screen.getByRole("button", { name: "About and legal notice" }))
    await userEvent.click(await screen.findByRole("menuitem", { name: "Check for updates" }))
  }

  it("is not offered on the web", async () => {
    show()
    await userEvent.click(screen.getByRole("button", { name: "About and legal notice" }))
    await screen.findByRole("menuitem", { name: "About" })
    expect(screen.queryByRole("menuitem", { name: "Check for updates" })).toBeNull()
  })

  it("says the app is up to date", async () => {
    const { bridge } = fakeUpdates()
    show(bridge)
    await check()
    expect(bridge.check).toHaveBeenCalledTimes(1)
    expect(await screen.findByText("Dofus Gear Explorer is up to date")).toBeTruthy()
    expect(screen.queryByRole("dialog")).toBeNull()
  })

  it("shows the update it found", async () => {
    const { bridge, tell } = fakeUpdates()
    bridge.check.mockImplementation(async () => {
      const found: AppUpdate = { state: "ready", version: "1.1.0" }
      tell(found)
      return found
    })
    show(bridge)
    await check()
    expect(await screen.findByRole("button", { name: "Install version 1.1.0 and restart" })).toBeTruthy()
    expect(await screen.findAllByText("Version 1.1.0 is ready")).toHaveLength(1)
  })

  it("says so when it could not look", async () => {
    const { bridge } = fakeUpdates()
    bridge.check.mockRejectedValue(new Error("offline"))
    show(bridge)
    await check()
    expect(await screen.findByText("Could not check for updates")).toBeTruthy()
  })
})
