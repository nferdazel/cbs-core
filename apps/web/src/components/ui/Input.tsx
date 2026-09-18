import React from "react";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
  isMono?: boolean;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, isMono = false, className = "", ...props }, ref) => {
    return (
      <div className="w-full space-y-1">
        {label && (
          <label className="block text-meta font-medium text-ink-600">{label}</label>
        )}
        <input
          ref={ref}
          className={`h-9 w-full rounded-md border bg-surface px-3 text-body text-ink-900 placeholder:text-ink-400 transition-colors duration-fast focus:border-navy-600 focus:outline-none focus:ring-1 focus:ring-navy-600 disabled:cursor-not-allowed disabled:bg-canvas disabled:text-ink-400 ${
            isMono ? "font-mono" : "font-sans"
          } ${error ? "border-debit-700 focus:border-debit-700 focus:ring-debit-700" : "border-border-strong"} ${className}`}
          {...props}
        />
        {error ? (
          <p className="text-meta text-debit-700">{error}</p>
        ) : helperText ? (
          <p className="text-meta text-ink-600">{helperText}</p>
        ) : null}
      </div>
    );
  }
);

Input.displayName = "Input";
