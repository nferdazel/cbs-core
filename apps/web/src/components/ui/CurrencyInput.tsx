import React from "react";
import { Input, InputProps } from "./Input";

export interface CurrencyInputProps extends Omit<InputProps, "onChange" | "value"> {
  value: number | string;
  onChange: (value: number) => void;
  currencyPrefix?: string;
}

export const CurrencyInput: React.FC<CurrencyInputProps> = ({
  value,
  onChange,
  currencyPrefix = "Rp ",
  label,
  error,
  helperText,
  className = "",
  ...props
}) => {
  const numericValue = typeof value === "string" ? parseFloat(value) || 0 : value;

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const rawVal = e.target.value.replace(/[^0-9]/g, "");
    const parsed = parseInt(rawVal, 10) || 0;
    onChange(parsed);
  };

  const formattedDisplay = numericValue.toLocaleString("id-ID");

  return (
    <div className="w-full space-y-1">
      {label && (
        <label className="block text-xs font-semibold uppercase tracking-wider text-slate-600">
          {label}
        </label>
      )}
      <div className="relative rounded-lg shadow-sm">
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
          <span className="text-slate-500 font-mono text-xs font-bold">{currencyPrefix}</span>
        </div>
        <input
          type="text"
          value={formattedDisplay}
          onChange={handleChange}
          className={`w-full bg-white border border-slate-300 rounded-lg pl-10 pr-3.5 py-2 text-sm font-bold font-mono text-slate-900 focus:outline-none focus:border-blue-700 focus:ring-1 focus:ring-blue-700 transition-colors ${
            error ? "border-red-500" : ""
          } ${className}`}
          {...props}
        />
      </div>
      {error ? (
        <p className="text-[11px] font-medium text-red-600">{error}</p>
      ) : helperText ? (
        <p className="text-[11px] text-slate-500">{helperText}</p>
      ) : null}
    </div>
  );
};
