/** Pemformatan nilai untuk UI. Semua uang harus lewat formatMoney/MoneyText. */

export type Numeric = string | number | null | undefined;

function toNumber(value: Numeric): number | null {
  if (value === null || value === undefined || value === "") return null;
  const num = typeof value === "number" ? value : Number(value);
  return Number.isFinite(num) ? num : null;
}

/**
 * Pemisah ribuan id-ID. Tanpa desimal bila nilai bulat; dua desimal bila tidak.
 * Contoh: 5000000 -> "5.000.000"; 1500.5 -> "1.500,50".
 */
export function formatMoney(value: Numeric): string {
  const num = toNumber(value);
  if (num === null) return "-";
  const isInteger = Number.isInteger(num);
  return new Intl.NumberFormat("id-ID", {
    minimumFractionDigits: isInteger ? 0 : 2,
    maximumFractionDigits: isInteger ? 0 : 2,
  }).format(num);
}

/**
 * Persentase id-ID dengan maksimal dua desimal, diakhiri tanda persen.
 * Nilai yang tidak bisa dibaca ditampilkan apa adanya agar tidak
 * menyamarkan data yang mencurigakan menjadi angka yang terlihat normal.
 */
export function formatRate(value: Numeric): string {
  const num = Number(value);
  if (!Number.isFinite(num)) return value ? String(value) : "-";
  return `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 }).format(num)}%`;
}

/** dd MMM yyyy menurut locale id-ID. Contoh: 02 Sep 2026. */
export function formatDate(value: string | Date | null | undefined): string {
  if (!value) return "-";
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  }).format(date);
}

/** dd MMM yyyy HH:mm menurut locale id-ID. */
export function formatDateTime(value: string | Date | null | undefined): string {
  if (!value) return "-";
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
