import React, { useId } from "react";
import { InputProps } from "./Input";

export interface CurrencyInputProps extends Omit<
  InputProps,
  "onChange" | "value"
> {
  value: number | string;
  /**
   * `value` adalah angka terurai (0 saat kosong), `raw` adalah digit mentah yang
   * baru diketik ("", "0", "1500", "1500.5"). Pemanggil yang perlu membedakan
   * "belum diketik" dari "diketik 0" harus memakai `raw`, bukan `value`/tampilan
   * terformat.
   */
  onChange: (value: number, raw: string) => void;
  currencyPrefix?: string;
  /**
   * Bila true, pemisah desimal tidak lagi disaring sehingga nilai tersimpan yang
   * berdesimal bisa disunting dan `raw` tetap memuat desimalnya. Bawaan false
   * mempertahankan perilaku lama: hanya digit.
   */
  allowDecimals?: boolean;
}

/**
 * Nilai mentah dari isi input terformat id-ID. "." adalah pemisah ribuan dan ","
 * pemisah desimal; titik desimal papan angka (mis. "1500.55") tetap diterima
 * selama bukan kelompok ribuan tiga digit. Hasil selalu memakai "." sebagai
 * pemisah desimal agar bisa dibaca Number().
 */
function decimalRaw(input: string): string {
  const cleaned = input.replace(/[^0-9.,]/g, "");
  if (!/\d/.test(cleaned)) return "";
  if (cleaned.includes(",")) {
    const [head, ...rest] = cleaned.replace(/\./g, "").split(",");
    return rest.length > 0 ? `${head}.${rest.join("")}` : head;
  }
  const lastDot = cleaned.lastIndexOf(".");
  if (lastDot === -1) return cleaned;
  const after = cleaned.slice(lastDot + 1);
  // Titik yang diikuti tepat tiga digit adalah pemisah ribuan.
  if (/^\d{3}$/.test(after)) return cleaned.replace(/\./g, "");
  return `${cleaned.slice(0, lastDot).replace(/\./g, "")}.${after}`;
}

/**
 * Input nominal. Menampilkan pemisah ribuan; nilai mentah disertakan lewat
 * hidden input ber-`name` agar bisa dikirim sebagai angka tanpa format. Nilai
 * string kosong berarti belum diisi dan tampil kosong, bukan "0".
 */
export const CurrencyInput: React.FC<CurrencyInputProps> = ({
  value,
  onChange,
  currencyPrefix = "Rp",
  label,
  error,
  helperText,
  className = "",
  name,
  allowDecimals = false,
  ...props
}) => {
  const isEmpty = value === "";
  const numericValue = isEmpty
    ? 0
    : typeof value === "string"
      ? Number(value) || 0
      : value;

  // Label harus terhubung ke input lewat htmlFor/id agar pembaca layar
  // membacakan nama field, bukan sekadar teks visual di sebelahnya.
  const generatedId = useId();
  const inputId = props.id ?? generatedId;

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = allowDecimals
      ? decimalRaw(e.target.value)
      : e.target.value.replace(/[^0-9]/g, "");
    if (!allowDecimals) {
      onChange(raw ? parseInt(raw, 10) : 0, raw);
      return;
    }
    const parsed = raw ? Number(raw) : 0;
    onChange(Number.isFinite(parsed) ? parsed : 0, raw);
  };

  const formatted = isEmpty
    ? ""
    : new Intl.NumberFormat(
        "id-ID",
        // Bawaan (false) tetap seperti semula: maksimum tiga angka pecahan.
        allowDecimals ? { maximumFractionDigits: 20 } : undefined,
      ).format(numericValue);

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
      <div className="flex h-9 items-center rounded-md border border-border-strong bg-surface focus-within:border-navy-600 focus-within:ring-1 focus-within:ring-navy-600">
        <span className="pl-3 font-mono text-body text-ink-600">
          {currencyPrefix}
        </span>
        <input
          id={inputId}
          type="text"
          inputMode={allowDecimals ? "decimal" : "numeric"}
          value={formatted}
          onChange={handleChange}
          className={`h-full w-full rounded-md bg-transparent px-2 text-right font-mono text-body text-ink-900 focus:outline-none ${className}`}
          {...props}
        />
      </div>
      {name && <input type="hidden" name={name} value={numericValue} />}
      {error ? (
        <p className="text-meta text-debit-700">{error}</p>
      ) : helperText ? (
        <p className="text-meta text-ink-600">{helperText}</p>
      ) : null}
    </div>
  );
};
