import React from "react";
import { formatMoney } from "@/lib/format";

export interface MoneyTextProps {
  value: string | number | null | undefined;
  /** Warna semantik sesuai arah nilai. Default netral. */
  tone?: "default" | "debit" | "credit";
  /** Tampilkan awalan "Rp". Default true. */
  withCurrency?: boolean;
  className?: string;
}

const toneStyles = {
  default: "text-ink-900",
  debit: "text-debit-700",
  credit: "text-credit-700",
};

/**
 * Satu-satunya cara menampilkan uang di UI: font mono, tabular-nums,
 * pemisah ribuan. Jangan format uang inline di halaman.
 */
export const MoneyText: React.FC<MoneyTextProps> = ({
  value,
  tone = "default",
  withCurrency = true,
  className = "",
}) => (
  <span className={`font-mono ${toneStyles[tone]} ${className}`}>
    {withCurrency ? `Rp ${formatMoney(value)}` : formatMoney(value)}
  </span>
);
