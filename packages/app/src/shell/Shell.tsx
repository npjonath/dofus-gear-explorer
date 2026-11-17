import { SegmentedControl, SegmentedControlIndicator, SegmentedControlItem } from "@ui-sdk/react/segmented-control"
import { Toast, ToastAction, ToastClose, ToastDescription, ToastRegion, ToastTitle } from "@ui-sdk/react/toast"
import { OverlayHost } from "@ui-sdk/react/overlays"
import { MoonIcon, SunIcon } from "lucide-react"
import * as React from "react"
import { useDataStatus, useMeta } from "../api/queries"
import { BuildPage } from "../features/build/BuildPage"
import { ExplorerPage } from "../features/explorer/ExplorerPage"
import { useI18n } from "../i18n"
import { hrefFor, navigate, useLocation, type Route } from "../lib/router"
import { decodeBuild } from "../lib/share"
import { useStore } from "../store"
import { BuildPickerHost } from "../ui/BuildPicker"
import { ItemHoverCard } from "../ui/ItemHover"
import { toaster } from "../ui/toaster"
import { AboutMenu } from "./AboutMenu"
import { DataBanner, DataStatusButton } from "./DataStatus"
import { ErrorBoundary } from "./ErrorBoundary"
import { Offline, Setup } from "./Setup"
import { useAppUpdates } from "./useAppUpdates"

function useResolvedTheme(): "light" | "dark" {
  const theme = useStore((s) => s.theme)
  const prefersDark = React.useSyncExternalStore(
    (onChange) => {
      const media = window.matchMedia("(prefers-color-scheme: dark)")
      media.addEventListener("change", onChange)
      return () => media.removeEventListener("change", onChange)
    },
    () => window.matchMedia("(prefers-color-scheme: dark)").matches
  )
  return theme === "system" ? (prefersDark ? "dark" : "light") : theme
}

const routes: { route: Route; label: "nav.items" | "nav.build" }[] = [
  { route: "items", label: "nav.items" },
  { route: "build", label: "nav.build" },
]

export function Shell() {
  const { t, lang } = useI18n()
  const location = useLocation()
  const status = useDataStatus()
  const meta = useMeta()
  const theme = useResolvedTheme()
  const setTheme = useStore((s) => s.setTheme)
  const setLang = useStore((s) => s.setLang)
  const addBuild = useStore((s) => s.addBuild)
  useAppUpdates()

  const [visited, setVisited] = React.useState(() => new Set<Route>([location.route]))
  if (!visited.has(location.route)) setVisited(new Set(visited).add(location.route))

  React.useEffect(() => {
    document.documentElement.dataset.theme = theme
    document.documentElement.lang = lang
  }, [theme, lang])

  // A shared link carries a whole build in ?b=; import it once, then clean the URL.
  const shared = location.route === "build" ? location.params.get("b") : null
  React.useEffect(() => {
    if (!shared) return
    const build = decodeBuild(shared)
    if (build) {
      addBuild(build)
      toaster(t("build.imported", { name: build.name }))
    }
    navigate("build")
  }, [shared, addBuild, t])

  // The search used to be a screen of its own: its old address opens its tab.
  React.useEffect(() => {
    if (window.location.hash.replace(/^#\/?/, "").startsWith("search")) {
      useStore.getState().showTab("search")
      navigate("build")
    }
  }, [])

  const data = status.data
  let body: React.ReactNode
  if (!data) {
    body = status.isError ? <Offline /> : <div className="board h-full" />
  } else if (!data.ready) {
    body = <Setup status={data} />
  } else if (!meta.data) {
    body = <div className="board h-full" />
  } else {
    // A screen stays mounted once visited, hidden while another is shown, so
    // coming back finds the same filters, results and scroll position.
    const metaData = meta.data
    const screens: [Route, () => React.ReactNode][] = [
      ["items", () => <ExplorerPage meta={metaData} />],
      ["build", () => <BuildPage meta={metaData} />],
    ]
    body = screens.map(([route, render]) =>
      visited.has(route) ? (
        <div key={route} className="h-full" hidden={location.route !== route}>
          {render()}
        </div>
      ) : null
    )
  }

  return (
    <div className="flex h-full flex-col">
      <header className="topbar">
        <span className="brand">
          <span className="brand-mark" aria-hidden="true">
            <i />
            <i />
            <i />
            <i />
          </span>
          <span className="max-sm:hidden">{t("app.name")}</span>
        </span>
        <nav className="nav" aria-label={t("nav.main")}>
          {routes.map(({ route, label }) => (
            <a key={route} href={hrefFor(route)} aria-current={location.route === route ? "page" : undefined}>
              {t(label)}
            </a>
          ))}
        </nav>
        <span className="flex-1" />
        {data?.ready && (
          <DataStatusButton status={data} />
        )}
        <SegmentedControl
          aria-label={t("lang.label")}
          value={lang}
          onValueChange={(next) => {
            if (next === "fr" || next === "en") setLang(next)
          }}
        >
          <SegmentedControlIndicator />
          <SegmentedControlItem value="fr">FR</SegmentedControlItem>
          <SegmentedControlItem value="en">EN</SegmentedControlItem>
        </SegmentedControl>
        <button
          type="button"
          className="icon-btn"
          aria-label={`${t("theme.label")} : ${theme === "dark" ? t("theme.light") : t("theme.dark")}`}
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
        >
          {theme === "dark" ? <SunIcon aria-hidden="true" /> : <MoonIcon aria-hidden="true" />}
        </button>
        <AboutMenu />
      </header>
      {data?.ready && <DataBanner status={data} />}
      <div className="min-h-0 flex-1">
        <ErrorBoundary title={t("app.crashed")} action={t("app.reload")}>
          {body}
        </ErrorBoundary>
      </div>
      <ToastRegion toaster={toaster}>
        {() => (
          <Toast>
            <div className="grid gap-0.5">
              <ToastTitle />
              <ToastDescription />
            </div>
            <ToastAction />
            <ToastClose />
          </Toast>
        )}
      </ToastRegion>
      <ItemHoverCard />
      {meta.data && <BuildPickerHost meta={meta.data} />}
      <OverlayHost />
    </div>
  )
}
