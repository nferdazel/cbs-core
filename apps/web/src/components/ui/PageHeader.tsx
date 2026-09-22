import React from "react";

export interface PageHeaderProps {
  title: string;
  description?: string;
  actions?: React.ReactNode;
}

export const PageHeader: React.FC<PageHeaderProps> = ({
  title,
  description,
  actions,
}) => (
  <div className="mb-4 flex items-start justify-between gap-4">
    <div className="min-w-0">
      <h1 className="text-page font-semibold text-ink-900">{title}</h1>
      {description && (
        <p className="mt-1 text-body text-ink-600">{description}</p>
      )}
    </div>
    {actions && (
      <div className="flex shrink-0 items-center gap-2">{actions}</div>
    )}
  </div>
);
