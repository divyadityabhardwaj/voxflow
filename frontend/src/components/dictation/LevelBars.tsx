import { useEffect, useState } from "react";

// Middle bars react most, like a voice meter.
const WEIGHTS = [0.55, 0.8, 1, 0.8, 0.55];

const reducedMotionQuery = () =>
  window.matchMedia?.("(prefers-reduced-motion: reduce)");

function useReducedMotion() {
  const [reduced, setReduced] = useState(() => !!reducedMotionQuery()?.matches);
  useEffect(() => {
    const q = reducedMotionQuery();
    if (!q) return;
    const onChange = () => setReduced(q.matches);
    q.addEventListener("change", onChange);
    return () => q.removeEventListener("change", onChange);
  }, []);
  return reduced;
}

interface Props {
  level: number; // 0..1
  maxHeight?: number;
  barWidth?: number;
  color?: string;
  className?: string;
}

// With reduced motion the bars move together and without transitions, so the
// level still reads but nothing wobbles.
export default function LevelBars({
  level,
  maxHeight = 14,
  barWidth = 2,
  color = "currentColor",
  className = "",
}: Props) {
  const reduced = useReducedMotion();
  // Speech RMS rarely passes ~0.3; stretch it so normal talking fills the bars.
  const l = Math.min(1, Math.sqrt(level) * 1.4);
  const min = Math.max(2, maxHeight * 0.14);
  return (
    <div
      className={`flex items-center justify-center ${className}`}
      style={{ height: maxHeight, gap: barWidth }}
      aria-hidden="true"
    >
      {WEIGHTS.map((w, i) => (
        <span
          key={i}
          className="rounded-full"
          style={{
            width: barWidth,
            height: min + (maxHeight - min) * l * (reduced ? 1 : w),
            background: color,
            transition: reduced ? "none" : "height 80ms linear",
          }}
        />
      ))}
    </div>
  );
}
