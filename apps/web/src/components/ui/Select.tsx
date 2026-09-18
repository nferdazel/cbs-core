import React from "react";

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps
  extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, "children"> {
  label?: string;
  error?: string;
  helperText?: string;
  options: SelectOption[];
  placeholder?: string;
}

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  (
    { label, error, helperText, options, placeholder, className = "", ...props },
    ref
  ) => (
    <div className="w-full space-y-1">
      {label && (
        <label className="block text-meta font-medium text-ink-600">{label}</label>
      )}
      <select
        ref={ref}
        className={`h-9 w-full rounded-md border bg-surface px-3 text-body text-ink-900 transition-colors duration-fast focus:border-navy-600 focus:outline-none focus:ring-1 focus:ring-navy-600 disabled:cursor-not-allowed disabled:bg-canvas ${
          error ? "border-debit-700" : "border-border-strong"
        } ${className}`}
        {...props}
      >
        {placeholder && <option value="">{placeholder}</option>}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {error ? (
        <p className="text-meta text-debit-700">{error}</p>
      ) : helperText ? (
        <p className="text-meta text-ink-600">{helperText}</p>
      ) : null}
    </div>
  )
);

Select.displayName = "Select";
