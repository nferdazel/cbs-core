import React from "react";
import { Loader2 } from "lucide-react";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md";
  loading?: boolean;
  icon?: React.ReactNode;
}

export const Button: React.FC<ButtonProps> = ({
  children,
  variant = "primary",
  size = "md",
  loading = false,
  icon,
  className = "",
  disabled,
  ...props
}) => {
  const baseStyles =
    "inline-flex items-center justify-center font-medium rounded-md border transition-colors duration-fast disabled:opacity-50 disabled:cursor-not-allowed";

  const variantStyles = {
    primary:
      "bg-navy-700 text-white border-navy-700 hover:bg-navy-600 hover:border-navy-600",
    secondary: "bg-surface text-ink-900 border-border-strong hover:bg-canvas",
    ghost:
      "bg-transparent text-ink-600 border-transparent hover:bg-canvas hover:text-ink-900",
    danger: "bg-debit-700 text-white border-debit-700 hover:opacity-90",
  };

  const sizeStyles = {
    sm: "h-8 px-3 text-meta gap-1.5",
    md: "h-9 px-3 text-body gap-2",
  };

  return (
    <button
      className={`${baseStyles} ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? (
        <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
      ) : icon ? (
        <span className="shrink-0">{icon}</span>
      ) : null}
      <span>{children}</span>
    </button>
  );
};
