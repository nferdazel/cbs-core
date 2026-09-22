import React, { useId } from "react";

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
  helperText?: string;
  isMono?: boolean;
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ label, error, helperText, isMono = false, className = "", ...props }, ref) => {
    // Label harus terhubung ke textarea lewat htmlFor/id agar pembaca layar
    // membacakan nama field, bukan sekadar teks visual di sebelahnya. id pemanggil dipakai bila ada.
    const generatedId = useId();
    const textareaId = props.id ?? generatedId;

    return (
      <div className="w-full space-y-1">
        {label && (
          <label
            htmlFor={textareaId}
            className="block text-meta font-medium text-ink-600"
          >
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          id={textareaId}
          className={`w-full rounded-md border bg-surface px-3 py-2 text-body text-ink-900 placeholder:text-ink-400 transition-colors duration-fast focus:border-navy-600 focus:outline-none focus:ring-1 focus:ring-navy-600 disabled:cursor-not-allowed disabled:bg-canvas disabled:text-ink-400 ${
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

Textarea.displayName = "Textarea";
