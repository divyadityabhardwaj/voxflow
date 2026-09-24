import type { ReactNode } from "react";

export default function Keycap({ children }: { children: ReactNode }) {
  return (
    <kbd className="inline-flex items-center justify-center min-w-[1.5rem] px-1.5 py-0.5 rounded-md bg-surface border border-border border-b-2 text-xs font-medium font-sans text-text whitespace-nowrap">
      {children}
    </kbd>
  );
}
