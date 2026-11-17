import { Button } from "@ui-sdk/react/button"
import { useStartUpdate } from "../api/queries"
import type { DataStatus } from "../api/types"
import { useI18n } from "../i18n"
import { usePlatform } from "../platform"
import { UpdateProgress } from "./DataStatus"

function Stage({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <main className="board grid h-full place-items-center p-6">
      <section className="grid w-full max-w-md gap-4 rounded-lg border border-line bg-panel p-6">
        <h1 className="m-0 text-xl font-bold">{title}</h1>
        {children}
      </section>
    </main>
  )
}

export function Offline() {
  const { t } = useI18n()
  const { apiBaseUrl } = usePlatform()
  return (
    <Stage title={t("setup.offline.title")}>
      <p className="m-0 text-ink-2">{t("setup.offline.body", { url: apiBaseUrl })}</p>
    </Stage>
  )
}

export function Setup({ status }: { status: DataStatus }) {
  const { t } = useI18n()
  const update = useStartUpdate()
  if (status.state !== "updating" && status.error) {
    return (
      <Stage title={t("setup.failed.title")}>
        <p className="m-0 text-ink-2" role="alert">
          {status.error}
        </p>
        <div>
          <Button data-variant="solid" onClick={() => update.mutateAsync()}>
            {t("data.retry")}
          </Button>
        </div>
      </Stage>
    )
  }
  return (
    <Stage title={t("setup.title")}>
      <p className="m-0 text-ink-2">{t("setup.body")}</p>
      <UpdateProgress status={status} />
    </Stage>
  )
}
