import React, { useId } from "react";

export interface DateInputProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "type"> {
  label?: string;
  error?: string;
  helperText?: string;
}

/** Input tanggal (YYYY-MM-DD). Nilai diteruskan apa adanya ke API. */
export const DateInput = React.forwardRef<HTMLInputElement, DateInputProps>(
  ({ label, error, helperText, className = "", ...props }, ref) => {
    // Label harus terhubung ke input lewat htmlFor/id agar pembaca layar
    // membacakan nama field, bukan sekadar teks visual di sebelahnya.
    const generatedId = useId();
    const inputId = props.id ?? generatedId;

    return (
      <div className="w-full space-y-1">
        {label && (
          <label
            htmlFor={inputId}
            className="block text-meta font-medium text-ink-600"
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          type="date"
          className={`h-9 w-full rounded-md border bg-surface px-3 font-mono text-body text-ink-900 transition-colors duration-fast focus:border-navy-600 focus:outline-none focus:ring-1 focus:ring-navy-600 disabled:cursor-not-allowed disabled:bg-canvas ${
            error ? "border-debit-700" : "border-border-strong"
          } ${className}`}
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

DateInput.displayName = "DateInput";
