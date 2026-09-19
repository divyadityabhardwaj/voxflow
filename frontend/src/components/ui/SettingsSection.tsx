import { ReactNode } from "react";

interface SettingsSectionProps {
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
}

export default function SettingsSection({
  title,
  description,
  children,
  className = "",
}: SettingsSectionProps) {
  return (
    <section className={`settings-card ${className}`}>
      <div className="mb-4">
        <h3 className="text-[15px] font-semibold tracking-tight text-text">
          {title}
        </h3>
        {description && (
          <p className="text-[13px] text-secondary mt-1 leading-relaxed max-w-prose">
            {description}
          </p>
        )}
      </div>
      {children}
    </section>
  );
}
