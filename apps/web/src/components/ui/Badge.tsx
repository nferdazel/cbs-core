import React from "react";

export interface BadgeProps {
  children: React.ReactNode;
  /** Varian mengikuti semantik DESIGN.md, bukan nama warna generik. */
  variant?: "neutral" | "outline" | "accent" | "credit" | "debit" | "info";
  className?: string;
}

export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = "neutral",
  className = "",
}) => {
  const variantStyles = {
    neutral: "bg-canvas text-ink-900 border-border",
    outline: "bg-transparent text-ink-600 border-border-strong",
    accent: "bg-accent-50 text-accent-600 border-accent-600/30",
    credit: "bg-credit-50 text-credit-700 border-credit-700/30",
    debit: "bg-debit-50 text-debit-700 border-debit-700/30",
    info: "bg-surface text-info-700 border-info-700/40",
  };

  return (
    <span
      className={`inline-flex items-center rounded-sm border px-1.5 py-0.5 text-meta font-medium ${variantStyles[variant]} ${className}`}
    >
      {children}
    </span>
  );
};
