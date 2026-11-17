import * as React from "react"

interface Props {
  title: string
  action: string
  children: React.ReactNode
}

// Keeps one failing screen from blanking the whole app: the top bar stays,
// and the player can reload.
export class ErrorBoundary extends React.Component<Props, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: unknown) {
    console.error(error)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className="board grid h-full place-items-center p-6">
        <div className="grid max-w-md justify-items-start gap-3 rounded-lg border border-line bg-panel p-6" role="alert">
          <h1 className="m-0 text-base font-bold">{this.props.title}</h1>
          <button type="button" className="chip" onClick={() => window.location.reload()}>
            {this.props.action}
          </button>
        </div>
      </div>
    )
  }
}
