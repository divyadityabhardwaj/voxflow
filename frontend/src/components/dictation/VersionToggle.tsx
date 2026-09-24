import { onRadioGroupKeyDown } from "../../lib/radioGroup";

export type Version = "cleaned" | "raw";

const OPTIONS: [Version, string][] = [
  ["cleaned", "Cleaned up"],
  ["raw", "As you said it"],
];
const VALUES = OPTIONS.map(([v]) => v);

export default function VersionToggle({
  value,
  onChange,
}: {
  value: Version;
  onChange: (v: Version) => void;
}) {
  return (
    <div
      role="radiogroup"
      aria-label="Show version"
      className="inline-flex p-0.5 rounded-lg bg-secondary border border-border"
      onKeyDown={(e) => onRadioGroupKeyDown(e, VALUES, value, onChange)}
    >
      {OPTIONS.map(([v, label]) => (
        <button
          key={v}
          type="button"
          role="radio"
          aria-checked={value === v}
          tabIndex={value === v ? 0 : -1}
          onClick={() => onChange(v)}
          className={`px-2.5 py-0.5 rounded-md text-xs font-medium transition-colors ${
            value === v ? "bg-surface text-text shadow-soft-sm" : "text-secondary hover:text-text"
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
