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
    <section className={`settings-card p-5 ${className}`}>
      <div className="mb-4">
        <h3 className="text-[15px] font-semibold text-text">{title}</h3>
        {description && (
          <p className="text-sm text-secondary mt-1 leading-relaxed">
            {description}
          </p>
        )}
      </div>
      {children}
    </section>
  );
}
