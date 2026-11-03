import type { ReactNode } from "react";

export default function Keycap({ children }: { children: ReactNode }) {
  return (
    <kbd className="keycap justify-center min-w-[1.5rem] whitespace-nowrap">
      {children}
    </kbd>
  );
}
