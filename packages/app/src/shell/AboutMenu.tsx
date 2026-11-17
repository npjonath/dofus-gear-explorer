import { Dialog, DialogBackdrop, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/react/dialog"
import { Menu, MenuContent, MenuGroup, MenuGroupLabel, MenuItem, MenuItemIcon, MenuTrigger } from "@ui-sdk/react/menu"
import { ExternalLinkIcon, InfoIcon, ScaleIcon } from "lucide-react"
import * as React from "react"
import { useDataStatus } from "../api/queries"
import { useI18n, type MessageKey } from "../i18n"
import { noticesUrl, repositoryUrl } from "../lib/project"
import { usePlatform } from "../platform"

type Panel = "about" | "legal"

const legalSections: [MessageKey, MessageKey][] = [
  ["legal.unofficial.title", "legal.unofficial.body"],
  ["legal.brand.title", "legal.brand.body"],
  ["legal.data.title", "legal.data.body"],
  ["legal.calc.title", "legal.calc.body"],
  ["legal.contact.title", "legal.contact.body"],
  ["legal.licence.title", "legal.licence.body"],
]

export function AboutMenu() {
  const { t } = useI18n()
  const { version } = usePlatform()
  const status = useDataStatus()
  const [panel, setPanel] = React.useState<Panel | null>(null)

  return (
    <>
      <Menu onAction={(value) => setPanel(value as Panel)}>
        <MenuTrigger className="icon-btn" aria-label={t("about.menu")}>
          <InfoIcon aria-hidden="true" />
        </MenuTrigger>
        <MenuContent className="about-menu">
          <MenuGroup>
            <MenuGroupLabel>
              {t("app.name")}
              <span className="about-menu-version">{t("about.version", { version })}</span>
            </MenuGroupLabel>
            <MenuItem value="about">
              <MenuItemIcon>
                <InfoIcon />
              </MenuItemIcon>
              {t("about.title")}
            </MenuItem>
            <MenuItem value="legal">
              <MenuItemIcon>
                <ScaleIcon />
              </MenuItemIcon>
              {t("legal.title")}
            </MenuItem>
          </MenuGroup>
        </MenuContent>
      </Menu>

      <Dialog open={panel !== null} onOpenChange={(open) => !open && setPanel(null)}>
        <DialogBackdrop />
        <DialogContent className="about-dialog" initialFocus="content">
          {panel === "legal" ? (
            <>
              <div className="grid gap-1">
                <DialogTitle>{t("legal.title")}</DialogTitle>
                <DialogDescription>{t("about.unofficial")}</DialogDescription>
              </div>
              <div className="about-scroll">
                {legalSections.map(([title, body]) => (
                  <section key={title} className="grid gap-1">
                    <h3 className="m-0 text-[0.93rem] font-bold">{t(title)}</h3>
                    <p className="m-0 text-ink-2">{t(body)}</p>
                  </section>
                ))}
              </div>
            </>
          ) : (
            <>
              <div className="grid gap-1">
                <DialogTitle>{t("app.name")}</DialogTitle>
                <DialogDescription>{t("about.summary")}</DialogDescription>
              </div>
              <dl className="about-facts">
                <dt>{t("about.appVersion")}</dt>
                <dd>{version}</dd>
                <dt>{t("about.dataVersion")}</dt>
                <dd>{status.data?.installedVersion || t("about.noData")}</dd>
                <dt>{t("about.licence")}</dt>
                <dd>MIT</dd>
              </dl>
              <p className="m-0 text-ink-2">{t("about.credits")}</p>
              <p className="m-0 text-ink-2">{t("about.unofficial")}</p>
              <ul className="about-links">
                <li>
                  <a href={repositoryUrl} target="_blank" rel="noreferrer">
                    {t("about.source")}
                    <ExternalLinkIcon aria-hidden="true" />
                  </a>
                  <span className="about-link-url">{repositoryUrl.replace("https://", "")}</span>
                </li>
                <li>
                  <a href={noticesUrl} target="_blank" rel="noreferrer">
                    {t("about.notices")}
                    <ExternalLinkIcon aria-hidden="true" />
                  </a>
                </li>
              </ul>
            </>
          )}
          <div className="flex justify-end">
            <DialogClose data-variant="solid">{t("about.close")}</DialogClose>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
