import { ReactNode, useId } from "react";

interface SettingsSectionProps {
  title: string;
  description?: ReactNode;
  children: ReactNode;
  className?: string;
}

// A titled, rounded group of rows, like a System Settings pane.
export default function SettingsSection({
  title,
  description,
  children,
  className = "",
}: SettingsSectionProps) {
  const titleId = useId();
  return (
    <section aria-labelledby={titleId} className={className}>
      <h3 id={titleId} className="text-[13px] font-semibold text-text mb-1 px-1">
        {title}
      </h3>
      {description && (
        <p className="text-[13px] text-secondary mb-2 px-1 leading-relaxed max-w-prose">
          {description}
        </p>
      )}
      <div className="settings-group">{children}</div>
    </section>
  );
}

export function SettingRow({
  label,
  description,
  htmlFor,
  labelId,
  children,
}: {
  label: ReactNode;
  description?: ReactNode;
  htmlFor?: string;
  labelId?: string;
  children?: ReactNode;
}) {
  const Label = htmlFor ? "label" : "p";
  return (
    <div className="settings-row">
      <div className="min-w-0">
        <Label id={labelId} htmlFor={htmlFor} className="block text-[13px] text-text">
          {label}
        </Label>
        {description && (
          <div className="text-xs text-secondary mt-0.5 leading-relaxed">
            {description}
          </div>
        )}
      </div>
      {children && (
        <div className="shrink-0 flex items-center gap-2">{children}</div>
      )}
    </div>
  );
}

export function Toggle({
  checked,
  onChange,
  disabled,
  labelledBy,
  describedBy,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
  labelledBy: string;
  describedBy?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      aria-disabled={disabled}
      className="toggle"
      data-on={checked ? "true" : "false"}
      onClick={() => !disabled && onChange(!checked)}
    >
      <span className="toggle-thumb" />
    </button>
  );
}

export function Disclosure({
  summary,
  children,
}: {
  summary: string;
  children: ReactNode;
}) {
  return (
    <details className="disclosure">
      <summary>{summary}</summary>
      <div className="mt-3 space-y-6">{children}</div>
    </details>
  );
}

export function StatusText({
  tone,
  children,
}: {
  tone: "success" | "danger" | "muted";
  children: ReactNode;
}) {
  const color =
    tone === "success"
      ? "text-[var(--success)]"
      : tone === "danger"
        ? "text-danger"
        : "text-tertiary";
  return (
    <span role="status" className={`text-xs ${color}`}>
      {children}
    </span>
  );
}
