import React from "react";
import { AlertCircle, Loader2, Inbox } from "lucide-react";
import { useTranslation } from "@/i18n/context";

export interface StateMessageProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

const shell =
  "flex flex-col items-center justify-center gap-2 rounded-md border border-border bg-surface px-6 py-12 text-center";

/** Data belum ada, tetapi bukan kesalahan. */
export const EmptyState: React.FC<StateMessageProps> = ({
  title,
  description,
  action,
  className = "",
}) => (
  <div className={`${shell} ${className}`}>
    <Inbox className="h-6 w-6 text-ink-400" aria-hidden />
    <p className="text-title font-medium text-ink-900">{title}</p>
    {description && <p className="max-w-md text-body text-ink-600">{description}</p>}
    {action && <div className="mt-2">{action}</div>}
  </div>
);

/** Sedang memuat. */
export const LoadingState: React.FC<{ label?: string; className?: string }> = ({
  label,
  className = "",
}) => {
  const { t } = useTranslation();
  return (
    <div className={`${shell} ${className}`} role="status">
      <Loader2 className="h-6 w-6 animate-spin text-ink-600" aria-hidden />
      <p className="text-body text-ink-600">{label ?? t.states.loading}</p>
    </div>
  );
};

/** Gagal memuat; jelaskan sebab dan sediakan tindakan. */
export const ErrorState: React.FC<StateMessageProps> = ({
  title,
  description,
  action,
  className = "",
}) => (
  <div className={`${shell} ${className}`} role="alert">
    <AlertCircle className="h-6 w-6 text-debit-700" aria-hidden />
    <p className="text-title font-medium text-ink-900">{title}</p>
    {description && <p className="max-w-md text-body text-ink-600">{description}</p>}
    {action && <div className="mt-2">{action}</div>}
  </div>
);
