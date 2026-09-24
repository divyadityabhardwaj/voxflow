import { Component, type ErrorInfo, type ReactNode } from "react";
import { LogFrontendError } from "../../wailsjs/go/main/App";

interface State {
  crashed: boolean;
}

export default class ErrorBoundary extends Component<
  { children: ReactNode },
  State
> {
  state: State = { crashed: false };

  static getDerivedStateFromError(): State {
    return { crashed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[VoxFlow] UI crashed:", error, info.componentStack);
    LogFrontendError(error.message, `${error.stack}\n${info.componentStack}`).catch(() => {});
  }

  render() {
    if (!this.state.crashed) return this.props.children;

    // The mini pill window is only ~52×32, so drop everything but the button there.
    return (
      <div
        role="alert"
        className="h-full flex flex-col items-center justify-center gap-3 p-4 [@media(max-height:120px)]:p-0 text-center bg-background text-text"
      >
        <p className="text-sm [@media(max-height:120px)]:hidden">
          Something went wrong in VoxFlow.
        </p>
        <button
          type="button"
          className="btn-primary [@media(max-height:120px)]:!p-1 [@media(max-height:120px)]:!text-[10px]"
          onClick={() => window.location.reload()}
        >
          Reload
        </button>
      </div>
    );
  }
}
