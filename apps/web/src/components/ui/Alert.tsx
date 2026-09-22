import React from "react";
import { AlertCircle, AlertTriangle, CheckCircle2, Info } from "lucide-react";

export type AlertVariant = "info" | "success" | "warning" | "error";

export interface AlertProps {
  /** Semantik mengikuti token DESIGN.md, bukan nama warna generik. */
  variant?: AlertVariant;
  /** Judul opsional; teksnya tetap dari kamus i18n. */
  title?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

/**
 * Satu-satunya cara menampilkan umpan balik halaman/bagian, bukan galat field.
 * Peran ARIA mengikuti urgensinya: galat & peringatan `alert` (mendesak), info
 * & sukses `status` (sopan). Ikon hanya penanda keadaan; teks tetap yang
 * dibacakan pembaca layar, jadi ikonnya `aria-hidden`.
 */
const variantStyles: Record<
  AlertVariant,
  { container: string; icon: React.ElementType; role: "status" | "alert" }
> = {
  success: {
    container: "border-credit-700/30 bg-credit-50 text-credit-700",
    icon: CheckCircle2,
    role: "status",
  },
  error: {
    container: "border-debit-700/30 bg-debit-50 text-debit-700",
    icon: AlertCircle,
    role: "alert",
  },
  warning: {
    container: "border-accent-600/40 bg-accent-50 text-accent-600",
    icon: AlertTriangle,
    role: "alert",
  },
  info: {
    container: "border-border bg-canvas text-ink-600",
    icon: Info,
    role: "status",
  },
};

export const Alert: React.FC<AlertProps> = ({
  variant = "info",
  title,
  children,
  className = "",
}) => {
  const { container, icon: Icon, role } = variantStyles[variant];

  return (
    <div
      role={role}
      className={`flex items-start gap-2 rounded-md border px-4 py-3 text-body ${container} ${className}`}
    >
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
      <div className="min-w-0">
        {title && <p className="text-title font-medium">{title}</p>}
        <div className={title ? "mt-1" : ""}>{children}</div>
      </div>
    </div>
  );
};
