import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import * as React from "react"
import { PlatformProvider, type Platform } from "./platform"
import { Shell } from "./shell/Shell"

export function App({ platform }: { platform: Platform }) {
  const [queryClient] = React.useState(
    () => new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } } })
  )
  return (
    <PlatformProvider platform={platform}>
      <QueryClientProvider client={queryClient}>
        <Shell />
      </QueryClientProvider>
    </PlatformProvider>
  )
}
